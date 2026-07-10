package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"wsfs-core/internal/server/config"
	"wsfs-core/internal/server/wsfs"

	"github.com/rs/zerolog/log"
)

type instance struct {
	server *Server
}

type Hub struct {
	GetConfig func() (config.Server, error)

	inst atomic.Pointer[instance]

	listenerConfig config.Listener
	listener       net.Listener
	httpServer     *http.Server
	exitErrorChan  chan error

	lock            sync.Mutex
	reloadReentrant atomic.Bool
	wsfsRegistry    atomic.Pointer[wsfs.SessionRegistry]
}

func NewHub() (h *Hub, err error) {
	h = new(Hub)
	h.exitErrorChan = make(chan error, 1)
	return
}

func (h *Hub) ServeHTTP(rsp http.ResponseWriter, req *http.Request) {
	inst := h.inst.Load()
	if inst == nil || inst.server == nil {
		http.Error(rsp, "server unavailable", http.StatusServiceUnavailable)
		return
	}
	inst.server.ServeHTTP(rsp, req)
}

func (h *Hub) Run(c config.Server) error {
	registry := h.ensureWSFSRegistry(c.WSFS)

	server, err := NewServer(c, registry)
	if err != nil {
		return err
	}
	h.inst.Store(&instance{server: server})

	listener, tlsConfig, err := listen(c.Listener)
	if err != nil {
		return err
	}
	h.listener = listener
	h.listenerConfig = c.Listener

	httpServer := &http.Server{Handler: h}
	if c.Listener.TLS.Enable {
		httpServer.TLSConfig = tlsConfig
	}
	h.httpServer = httpServer

	log.Warn().Str("Net", c.Listener.Network).Str("Addr", c.Listener.Address).Msg("Listening")

	go func() {
		var serveErr error
		if c.Listener.TLS.Enable {
			serveErr = httpServer.ServeTLS(listener, "", "")
		} else {
			serveErr = httpServer.Serve(listener)
		}
		if serveErr != nil {
			h.exitErrorChan <- serveErr
		}
	}()

	err = <-h.exitErrorChan
	cleanListen(c.Listener)
	if registry := h.wsfsRegistry.Load(); registry != nil {
		registry.Stop()
	}
	return err
}

func (h *Hub) IssueReload() {
	if !h.lock.TryLock() {
		log.Warn().Msg("Reload has been postponed: Server is in reloading")
		h.reloadReentrant.Store(true)
		return
	}
	log.Warn().Msg("Reloading")

	go h.doReload()
}

func (h *Hub) doReload() {
	defer func() {
		err := recover()
		if err != nil {
			log.Error().Any("Error", err).Msg("Panic during reloading")
		}

		h.lock.Unlock()
		if h.reloadReentrant.CompareAndSwap(true, false) {
			h.IssueReload()
		}
	}()

	conf, err := h.GetConfig()
	if err != nil {
		log.Error().Err(err).Msg("Reload failed: Unable to decode new config")
		return
	}

	registry := h.ensureWSFSRegistry(conf.WSFS)

	server, err := NewServer(conf, registry)
	if err != nil {
		log.Error().Err(err).Msg("Reload failed: Unable to new server")
		return
	}

	if listenerEquals(h.listenerConfig, conf.Listener) {
		h.inst.Store(&instance{server: server})
		log.Warn().Msg("Reloaded")
		return
	}

	listener, tlsConfig, err := listen(conf.Listener)
	if err != nil {
		log.Error().Err(err).Msg("Reload failed: Unable to listen on new config")
		return
	}

	oldHTTPServer := h.httpServer
	oldListenerConfig := h.listenerConfig

	newHTTPServer := &http.Server{Handler: h}
	if conf.Listener.TLS.Enable {
		newHTTPServer.TLSConfig = tlsConfig
	}

	h.inst.Store(&instance{server: server})
	h.listener = listener
	h.listenerConfig = conf.Listener
	h.httpServer = newHTTPServer

	log.Warn().Str("Net", conf.Listener.Network).Str("Addr", conf.Listener.Address).Msg("Listening")
	go func() {
		var serveErr error
		if conf.Listener.TLS.Enable {
			serveErr = newHTTPServer.ServeTLS(listener, "", "")
		} else {
			serveErr = newHTTPServer.Serve(listener)
		}
		if serveErr != nil {
			h.exitErrorChan <- serveErr
		}
	}()

	// Intentionally wait without a deadline so in-flight requests can finish and
	// long-lived wsfs sessions can survive a listener reload.
	// TODO: Emit a warning if shutdown remains blocked for too long.
	shutdownErr := oldHTTPServer.Shutdown(context.Background())
	if shutdownErr != nil && !errors.Is(shutdownErr, http.ErrServerClosed) {
		log.Error().Err(shutdownErr).Msg("Old server shutdown failed")
	}
	cleanListen(oldListenerConfig)

	log.Warn().Msg("Reloaded")
}

func (h *Hub) IssueShutdown() {
	log.Warn().Msg("Shutting down")
	if h.httpServer != nil {
		// Intentionally wait without a deadline so active wsfs sessions are not
		// force-terminated during hub shutdown.
		// TODO: Emit a warning if shutdown remains blocked for too long.
		err := h.httpServer.Shutdown(context.Background())
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error().Err(err).Msg("Server shutdown failed")
		}
	}
	if registry := h.wsfsRegistry.Load(); registry != nil {
		registry.Stop()
	}
}

func (h *Hub) ensureWSFSRegistry(c config.WSFS) *wsfs.SessionRegistry {
	if !c.Enable {
		return nil
	}

	if registry := h.wsfsRegistry.Load(); registry != nil {
		registry.Reconfigure(c)
		return registry
	}

	registry := wsfs.NewSessionRegistry(c)
	if h.wsfsRegistry.CompareAndSwap(nil, registry) {
		go registry.CollectInactiveSessions()
		return registry
	}

	registry = h.wsfsRegistry.Load()
	registry.Reconfigure(c)
	return registry
}

func listenerEquals(a, b config.Listener) bool {
	return a.Network == b.Network &&
		a.Address == b.Address &&
		a.TLS.Enable == b.TLS.Enable &&
		a.TLS.CertFile == b.TLS.CertFile &&
		a.TLS.KeyFile == b.TLS.KeyFile
}

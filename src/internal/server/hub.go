package server

import (
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"wsfs-core/internal/server/config"

	"github.com/rs/zerolog/log"
)

type instance struct {
	server *Server
	olded  atomic.Bool
}

type Hub struct {
	GetConfig func() (config.Server, error)

	inst *instance

	exitErrorChan chan error

	lock            sync.Mutex
	reloadReentrant atomic.Bool
}

func NewHub() (h *Hub, err error) {
	h = new(Hub)

	return
}

func (h *Hub) runInst(listener config.Listener, inst *instance) {
	err := inst.server.Run(listener)
	if inst.olded.Load() {
		if !errors.Is(err, http.ErrServerClosed) {
			log.Error().Err(err).Msg("Old server exit with error")
		}
	} else {
		h.exitErrorChan <- err
	}
}

func (h *Hub) Run(c config.Server) error {
	server, err := NewServer(c)
	if err != nil {
		return err
	}
	inst := &instance{
		server: server,
	}
	h.inst = inst

	go h.runInst(c.Listener, inst)

	return <-h.exitErrorChan
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
		log.Error().Err(err).Msg("Reload failed: Uable to decode new config")
		return
	}

	server, err := NewServer(conf)
	if err != nil {
		log.Error().Err(err).Msg("Reload failed: Uable to new server")
		return
	}

	oldinst := h.inst
	oldinst.olded.Store(true)
	oldinst.server.Shutdown(func(err error) {
		if err != nil {
			log.Error().Err(err).Msg("Old server shutdown failed")
		}
	})

	newinst := &instance{
		server: server,
	}
	go h.runInst(conf.Listener, newinst)
	h.inst = newinst

	log.Warn().Msg("Reloaded")
}

func (h *Hub) IssueShutdown() {
	log.Warn().Msg("Shutdowning")
	h.inst.server.Shutdown(nil)
}

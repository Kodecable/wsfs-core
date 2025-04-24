package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"wsfs-core/internal/server/config"
	internalerror "wsfs-core/internal/server/internalError"
	"wsfs-core/internal/server/storage"
	"wsfs-core/internal/server/webdav"
	"wsfs-core/internal/server/webui"
	"wsfs-core/internal/server/wsfs"
	"wsfs-core/internal/util"
	"wsfs-core/version"

	"github.com/rs/zerolog/log"
)

var cacheIdRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

var _ = (internalerror.ErrorHandler)((*Server)(nil))

type Server struct {
	httpServer http.Server
	errorChan  chan error
	cacheId    string

	webuiHandler  *webui.Handler
	webdavHandler *webdav.Handler
	wsfsHandler   *wsfs.Handler

	users     storage.Users
	anonymous *storage.User
}

func NewServer(c config.Server) (s *Server, err error) {
	s = &Server{
		httpServer: http.Server{},
		errorChan:  make(chan error),
		cacheId:    util.RandomString(8, cacheIdRunes),
	}

	s.users, s.anonymous, err = storage.NewUsers(c, anonymousUsername)
	if err != nil {
		return
	}

	if c.Webdav.Webui.Enable && !c.Webdav.Enable {
		err = errors.New("webui enabled but webdav disabled")
		return
	}

	if c.Webdav.Enable {
		s.webdavHandler, err = webdav.NewHandler(c.Webdav, s)
		if err != nil {
			return
		}
	}

	if c.Webdav.Webui.Enable {
		s.webuiHandler, err = webui.NewHandler(c.Webdav.Webui, s.cacheId)
		if err != nil {
			return
		}
	}

	if c.WSFS.Enable {
		s.wsfsHandler, err = wsfs.NewHandler(s, c.WSFS)
		if err != nil {
			return
		}
		go s.wsfsHandler.CollecteInactivedSession()
	}

	return
}

func (s *Server) Run(c config.Listener) error {
	listener, tlsConfig, err := listen(c)
	if err != nil {
		return err
	}

	defer cleanListen(c)

	s.httpServer = http.Server{
		Handler: s,
	}

	log.Warn().Str("Net", c.Network).Str("Addr", c.Address).Msg("Listening")
	if c.TLS.Enable {
		s.httpServer.TLSConfig = tlsConfig
		err = s.httpServer.ServeTLS(listener, "", "")
	} else {
		err = s.httpServer.Serve(listener)
	}

	return err
}

// Async shutdown
func (s *Server) Shutdown(callback func(error)) {
	done := make(chan any)

	go func() {
		s.wsfsHandler.Stop()
		s.httpServer.RegisterOnShutdown(func() {
			close(done)
		})
		err := s.httpServer.Shutdown(context.Background())
		if callback != nil {
			callback(err)
		}
	}()

	<-done
}

func (s *Server) ServeErrorPage(rsp http.ResponseWriter, req *http.Request, status int, msg string) {
	if s.webuiHandler != nil && (req.Method == "GET" || req.Method == "HEAD") {
		s.webuiHandler.ServeErrorPage(rsp, req, status, msg)
	} else {
		rsp.WriteHeader(status)
		rsp.Write([]byte(msg))
	}
}

func (s *Server) ServeError(rsp http.ResponseWriter, req *http.Request, err error) {
	if s.webuiHandler != nil && (req.Method == "GET" || req.Method == "HEAD") {
		s.webuiHandler.ServeError(rsp, req, err)
	} else {
		if errors.Is(err, internalerror.ErrInternalForbidden) {
			s.ServeErrorPage(rsp, req, http.StatusNotFound, "Not Found")
		} else if errors.Is(err, internalerror.ErrInternalNotFound) {
			s.ServeErrorPage(rsp, req, http.StatusForbidden, "Forbidden")
		} else {
			s.ServeErrorPage(rsp, req, http.StatusInternalServerError, err.Error())
		}
	}
}

func (s *Server) ServeErrorMessage(rsp http.ResponseWriter, req *http.Request, status int, msg string) {
	rsp.Header().Set("Content-Type", "text/plain")
	rsp.WriteHeader(status)
	rsp.Write([]byte(msg))
}

// Modified from gin's RecoveryFunc.
// Original copyright: Copyright 2014 Manu Martinez-Almeida. All rights reserved.
// Original license: MIT (https://raw.githubusercontent.com/gin-gonic/gin/master/LICENSE)
func (s *Server) serveRecover(rsp *responseWriter, req *http.Request, err any) {
	// Check for a broken connection
	var brokenPipe bool
	if ne, ok := err.(*net.OpError); ok {
		var se *os.SyscallError
		if errors.As(ne, &se) {
			seStr := strings.ToLower(se.Error())
			if strings.Contains(seStr, "broken pipe") ||
				strings.Contains(seStr, "connection reset by peer") {
				brokenPipe = true
			}
		}
	}

	if brokenPipe {
		log.Warn().Str("From", req.RemoteAddr).Msg("Connection reset")
		// If the connection is dead, we can do nothing
		return
	}

	log.Error().Str("From", req.RemoteAddr).Str("Err", fmt.Sprint(err)).Msg("Panic")

	if rsp.status == statusUnwrited {
		func() {
			defer func() {
				if err := recover(); err != nil {
					log.Warn().Str("From", req.RemoteAddr).Any("Err", err).Msg("Write response failed")
				}
			}()
			s.ServeError(rsp, req, internalerror.Warp(err))
		}()
	}
}

func (s *Server) writeAuthRsp(rsp http.ResponseWriter) {
	rsp.Header().Set("WWW-Authenticate", `Basic charset="UTF-8"`)
	rsp.WriteHeader(http.StatusUnauthorized)
}

func (s *Server) writeMethodNotAllow(rsp http.ResponseWriter, allow string) {
	rsp.Header().Set("Allow", allow)
	rsp.WriteHeader(http.StatusMethodNotAllowed)
}

func (s *Server) tryAuth(rsp http.ResponseWriter, req *http.Request) (user *storage.User) {
	var err error

	user, err = httpBasciAuth(s.users, req)
	switch err {
	case nil:
		break
	case ErrAuthHeaderNotExists, ErrAnonymous:
		if s.anonymous != nil && !req.URL.Query().Has("must-login") {
			user = s.anonymous
			break
		}
		fallthrough
	case ErrBadHttpAuthHeader, ErrUserNotExists, ErrHashMismatch:
		s.writeAuthRsp(rsp)
	default:
		log.Error().Err(err).Msg("Uable to auth user")
		s.ServeError(rsp, req, internalerror.Warp(err))
	}
	return
}

func (s *Server) ServeHTTP(rsp_ http.ResponseWriter, req *http.Request) {
	rsp := newResponseWriter(rsp_)

	defer func() {
		if err := recover(); err != nil {
			s.serveRecover(rsp, req, err)
		} else {
			if rsp.status == -1 {
				// conn hijacked
				return
			}
			log.Info().Str("Path", req.RequestURI).Str("From", req.RemoteAddr).Int("Code", rsp.status).Msg(req.Method)
		}
	}()
	rsp.Header().Set("Server", "WSFS/"+version.Version)

	if !util.IsUrlValid(req.URL.Path) {
		s.ServeErrorPage(rsp, req, http.StatusBadRequest, "invalid URL path")
		return
	}

	querys := req.URL.Query()
	if querys.Has("webui-assets") {
		if req.Method != "GET" && req.Method != "HEAD" {
			s.writeMethodNotAllow(rsp, "GET, HEAD")
		} else {
			s.webuiHandler.ServeAssets(rsp, req)
		}
		return
	}

	user := s.tryAuth(rsp, req)
	if user == nil {
		return
	}

	if s.wsfsHandler != nil {
		if s.wsfsHandler.TryServerHTTP(rsp, req, user, querys.Has("wsfs")) {
			return
		}
	} else if querys.Has("wsfs") {
		s.writeMethodNotAllow(rsp, "")
		return
	}
	if s.webuiHandler != nil &&
		strings.HasSuffix(req.URL.Path, "/") &&
		(req.Method == "GET" || req.Method == "HEAD") {
		s.webuiHandler.ServeList(rsp, req, user)
	} else {
		if s.webdavHandler != nil {
			s.webdavHandler.ServeHTTP(rsp, req, user)
		} else {
			s.writeMethodNotAllow(rsp, "")
		}
	}

}

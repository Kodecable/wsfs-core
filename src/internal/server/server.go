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

	users map[string]User

	anonymous *storage.Storage
}

func NewServer(c config.Server) (s *Server, err error) {
	s = &Server{
		httpServer: http.Server{},
		errorChan:  make(chan error),
		cacheId:    util.RandomString(8, cacheIdRunes),
	}

	storages := map[string]*storage.Storage{}
	for _, st := range c.Storages {
		if _, ok := storages[st.Id]; ok {
			if st.Id == "" {
				err = fmt.Errorf("default storage repeated")
			} else {
				err = fmt.Errorf("storage id '%s' repeated", st.Id)
			}
			return
		}

		storages[st.Id], err = storage.NewStorage(&st)
		if err != nil {
			return
		}
	}

	users := map[string]User{}
	for _, us := range c.Users {
		if _, ok := users[us.Name]; ok {
			err = fmt.Errorf("user '%s' repeated", us.Name)
			return
		}

		if us.Name == "" {
			err = fmt.Errorf("username can not be empty")
			return
		}

		if _, ok := storages[us.Storage]; !ok {
			err = fmt.Errorf("user '%s' referenced a storage that does not exist", us.Name)
			return
		}

		users[us.Name] = User{
			Name:     us.Name,
			Password: us.SecretHash,
			Storage:  storages[us.Storage],
		}
	}
	s.users = users

	if c.Anonymous.Enable {
		if _, ok := storages[c.Anonymous.Storage]; !ok {
			err = fmt.Errorf("anonymous user referenced a storage that does not exist")
			return
		}
		s.anonymous = storages[c.Anonymous.Storage]

		if _, ok := s.users[anonymousUsername]; ok {
			log.Warn().Msg("anonymousUsername used; it will not be considered anonymous")
		}
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

func (s *Server) tryAuth(rsp http.ResponseWriter, req *http.Request) (st *storage.Storage) {
	user, err := httpBasciAuth(s.users, req)
	switch err {
	case nil:
		st = user.Storage
	case ErrAuthHeaderNotExists, ErrAnonymous:
		// if webui-login in query, make sure login
		if s.anonymous != nil && !req.URL.Query().Has("webui-login") {
			st = s.anonymous
		} else {
			s.writeAuthRsp(rsp)
		}
	case ErrBadHttpAuthHeader, ErrUserNotExists, ErrHashMismatch:
		s.writeAuthRsp(rsp)
	default:
		log.Error().Err(err).Msg("Auth error")
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

	st := s.tryAuth(rsp, req)
	if st == nil {
		return
	}

	if querys.Has("wsfs") {
		if s.wsfsHandler != nil {
			s.wsfsHandler.ServeHTTP(rsp, req, st)
		} else {
			s.writeMethodNotAllow(rsp, "")
		}
	} else {
		if s.webuiHandler != nil &&
			strings.HasSuffix(req.URL.Path, "/") &&
			(req.Method == "GET" || req.Method == "HEAD") {
			s.webuiHandler.ServeList(rsp, req, st)
		} else {
			if s.webdavHandler != nil {
				s.webdavHandler.ServeHTTP(rsp, req, st)
			} else {
				s.writeMethodNotAllow(rsp, "")
			}
		}
	}
}

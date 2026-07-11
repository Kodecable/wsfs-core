package server

import (
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
	cacheId string

	webuiHandler  *webui.Handler
	webdavHandler *webdav.Handler
	wsfsHandler   *wsfs.Handler

	users     storage.Users
	anonymous *storage.User

	realIpHeader string
	serverHeader string
}

func NewServer(c config.Server, wsfsRegistry *wsfs.SessionRegistry) (s *Server, err error) {
	s = &Server{
		cacheId:      util.RandomString(8, cacheIdRunes),
		realIpHeader: c.RealIpHeader,
		serverHeader: c.ServerHeader,
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
		fsIds, resolveErr := c.FsIds.Resolve()
		if resolveErr != nil {
			err = resolveErr
			return
		}
		s.wsfsHandler = wsfs.NewHandler(s, fsIds, wsfsRegistry)
	}

	return
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
			s.ServeErrorPage(rsp, req, http.StatusForbidden, "Forbidden")
		} else if errors.Is(err, internalerror.ErrInternalNotFound) {
			s.ServeErrorPage(rsp, req, http.StatusNotFound, "Not Found")
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

func (s *Server) serveRecover(rsp *responseWriter, req *http.Request, err any) {
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
		return
	}

	log.Error().Str("From", req.RemoteAddr).Str("Err", fmt.Sprint(err)).Msg("Panic")

	if rsp.status == statusUnwritten {
		func() {
			defer func() {
				if err := recover(); err != nil {
					log.Warn().Str("From", req.RemoteAddr).Any("Err", err).Msg("Write response failed")
				}
			}()
			s.ServeError(rsp, req, internalerror.Wrap(err))
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

// return nil for auth fail
func (s *Server) tryAuth(rsp http.ResponseWriter, req *http.Request) (user *storage.User) {
	var err error

	user, err = httpBasicAuth(s.users, req)
	switch err {
	case nil: // pass
	case ErrAuthHeaderNotExists, ErrAnonymous:
		if s.anonymous != nil && !req.URL.Query().Has("must-login") {
			user = s.anonymous
			break
		}
		fallthrough
	case ErrBadHttpAuthHeader, ErrUserNotExists, ErrHashMismatch:
		s.writeAuthRsp(rsp)
		user = nil
	default:
		user = nil
		log.Error().Err(err).Msg("Unable to auth user")
		s.ServeError(rsp, req, internalerror.Wrap(err))
	}
	return
}

func rewriteRemoteAddr(req *http.Request, realIpHeader string) {
	if realIpHeader == "" {
		return
	}

	if addr := req.Header.Get(realIpHeader); addr != "" {
		addr, _, _ = strings.Cut(addr, ",")
		req.RemoteAddr = strings.TrimSpace(addr)
	}
}

func (s *Server) ServeHTTP(rsp_ http.ResponseWriter, req *http.Request) {
	rsp := newResponseWriter(rsp_)
	defer func() {
		if err := recover(); err != nil {
			s.serveRecover(rsp, req, err)
		} else {
			if rsp.status == -1 || rsp.status == http.StatusSwitchingProtocols {
				return
			}
			log.Info().Str("Path", req.RequestURI).Str("From", req.RemoteAddr).Int("Code", rsp.status).Msg("HTTP " + req.Method)
		}
	}()

	rewriteRemoteAddr(req, s.realIpHeader)
	if s.serverHeader == "" {
		rsp.Header().Set("Server", "WSFS/"+version.Version)
	} else {
		rsp.Header().Set("Server", s.serverHeader)
	}

	if !util.IsUrlValid(req.URL.Path) {
		s.ServeErrorPage(rsp, req, http.StatusBadRequest, "invalid URL path")
		return
	}

	querys := req.URL.Query()
	if querys.Has("webui-assets") && s.webuiHandler != nil {
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

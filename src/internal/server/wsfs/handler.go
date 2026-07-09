package wsfs

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
	"wsfs-core/internal/server/config"
	internalerror "wsfs-core/internal/server/internalError"
	"wsfs-core/internal/server/storage"
	"wsfs-core/internal/util"

	"github.com/rs/zerolog/log"
)

var (
	ErrBadSubprotocol = errors.New("wsfs: bad websocket sub protocol")
)

const (
	sessionInactiveScanPeriod = 3 * time.Minute
	sessionInactiveMaxCount   = 5
)

type Handler struct {
	idSource     sessionIdSource
	errorHandler internalerror.ErrorHandler
	sessions     sync.Map
	fsIds        util.FsIds
	ctx          context.Context

	Stop context.CancelFunc
}

func NewHandler(errorHandler internalerror.ErrorHandler, fsIds util.FsIds, c config.WSFS) (h *Handler, err error) {
	h = &Handler{
		errorHandler: errorHandler,
		fsIds:        fsIds,
		idSource:     setupSessionIdSource(c),
	}
	h.ctx, h.Stop = context.WithCancel(context.Background())
	return h, nil
}

func (h *Handler) CollectInactiveSessions() {
	for {
		time.Sleep(sessionInactiveScanPeriod)
		existsSession := false
		h.sessions.Range(func(key, value any) bool {
			existsSession = true

			s := value.(*session)
			if s == nil {
				return true // continue
			}

			if s.Lock.TryLock() {
				s.inactiveCount += 1

				if s.inactiveCount >= sessionInactiveMaxCount {
					h.delSession(key.(string))
					return true // continue
				}

				s.Lock.Unlock()
			}
			return true
		})
		if h.ctx.Err() != nil && !existsSession {
			return
		}
	}
}

func (h *Handler) TryServerHTTP(rsp http.ResponseWriter, req *http.Request, user *storage.User, forced bool) (handled bool) {
	if !isWebSocketUpgrade(req) {
		if forced {
			h.errorHandler.ServeErrorMessage(rsp, req, http.StatusBadRequest, "Bad WSFS handshake: Not a upgrade request")
			return true
		}
		return false
	}
	if user.ReadOnly {
		h.errorHandler.ServeErrorMessage(rsp, req, http.StatusForbidden, "Bad WSFS handshake: Access Denied")
		return true
	}
	h.ServeHTTP(rsp, req, user)
	return true
}

func isWebSocketUpgrade(req *http.Request) bool {
	return headerContainsToken(req.Header, "Connection", "Upgrade") &&
		headerContainsToken(req.Header, "Upgrade", "websocket")
}

func headerContainsToken(header http.Header, key, want string) bool {
	for _, value := range header.Values(key) {
		for _, token := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(token), want) {
				return true
			}
		}
	}
	return false
}

func (h *Handler) ServeHTTP(rsp http.ResponseWriter, req *http.Request, user *storage.User) {
	id := req.Header.Get("X-Wsfs-Resume")
	if id == "" {
		var err error
		id, err = h.newSession(user.Name, user.Storage)
		if err != nil {
			log.Error().Err(err).Msg("Generate session id failed")
			rsp.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	session := h.getSession(id)
	if session == nil {
		rsp.WriteHeader(http.StatusBadRequest)
		return
	}
	if session.Username != user.Name {
		// lie as session not found
		rsp.WriteHeader(http.StatusBadRequest)
		return
	}
	if !session.Lock.TryLock() {
		rsp.WriteHeader(http.StatusPreconditionFailed)
		return
	}

	// in case of panic
	var succeeded = false
	defer func() {
		if !succeeded {
			session.Lock.Unlock()
		}
	}()

	conn, err := h.upgrade(rsp, req, id)
	if err != nil {
		log.Error().Err(err).Msg("Upgrade websocket connection failed")
		return
	}
	succeeded = true

	log.Info().Str("From", req.RemoteAddr).Str("User", user.Name).Str("Id", id).Msg("Session running")
	session.takeConn(conn, req.RemoteAddr)
}

func (h *Handler) getSession(id string) *session {
	v, ok := h.sessions.Load(id)
	if !ok {
		return nil
	}
	return v.(*session)
}

func (h *Handler) newSession(username string, storage *storage.Storage) (string, error) {
	for {
		id, err := h.idSource.New()
		if err != nil {
			return "", err
		}
		if _, loaded := h.sessions.LoadOrStore(id, (*session)(nil)); loaded {
			continue
		}
		h.sessions.Store(id, newSession(h, id, username, storage))
		log.Info().Str("Id", id).Msg("Session created")
		return id, nil
	}
}

func (h *Handler) delSession(id string) {
	if session := h.getSession(id); session != nil {
		session.clearFDs()
	}
	log.Info().Str("Id", id).Msg("Session destroyed")
	h.sessions.Delete(id)
}

package wsfs

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"wsfs-core/internal/server/config"
	internalerror "wsfs-core/internal/server/internalError"
	"wsfs-core/internal/server/storage"
	"wsfs-core/internal/util"

	"github.com/rs/zerolog/log"
	"github.com/sqids/sqids-go"
)

var (
	ErrBadSubprotocol = errors.New("wsfs: bad websocket sub protocol")
)

const (
	sessionInactiveScanPeriod = 3 * time.Minute
	sessionInactiveMaxCount   = 5
)

type Handler struct {
	ider         *sqids.Sqids
	errorHandler internalerror.ErrorHandler
	sessions     sync.Map
	sessionLast  atomic.Uint64
	fsIds        util.FsIds
	ctx          context.Context

	Stop context.CancelFunc
}

func NewHandler(errorHandler internalerror.ErrorHandler, fsIds util.FsIds, c config.WSFS) (h *Handler, err error) {
	h = &Handler{
		errorHandler: errorHandler,
		fsIds:        fsIds,
	}
	h.ctx, h.Stop = context.WithCancel(context.Background())

	h.ider, err = setupIder()
	return
}

func (h *Handler) CollecteInactivedSession() {
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
					h.delSession(key.(uint64))
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
	var id uint64
	var idstr = ""
	if resumeHeader := req.Header.Get("X-Wsfs-Resume"); resumeHeader != "" {
		if result := h.ider.Decode(resumeHeader); len(result) == 1 {
			id = result[0]
		} else {
			rsp.WriteHeader(http.StatusBadRequest)
			return
		}
	} else {
		var err error
		id = h.newSession(user.Name, user.Storage)
		if idstr, err = h.ider.Encode([]uint64{id}); err != nil {
			log.Error().Err(err).Msg("Ider encode failed")
			h.delSession(id)
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
	var sucessed = false
	defer func() {
		if !sucessed {
			session.Lock.Unlock()
		}
	}()

	conn, err := h.upgrade(rsp, req, idstr)
	if err != nil {
		log.Error().Err(err).Msg("Upgrade websocket connection failed")
		return
	}
	sucessed = true

	log.Info().Str("From", req.RemoteAddr).Str("User", user.Name).Uint64("Id", id).Msg("Session running")
	session.takeConn(conn, req.RemoteAddr)
}

func (h *Handler) getSession(id uint64) *session {
	v, ok := h.sessions.Load(id)
	if !ok {
		return nil
	}
	return v.(*session)
}

func (h *Handler) newSession(username string, storage *storage.Storage) (id uint64) {
	for {
		id = h.sessionLast.Add(1)
		if _, loaded := h.sessions.LoadOrStore(id, (*session)(nil)); !loaded {
			break
		}
	}
	h.sessions.Store(id, newSession(h, username, storage))
	log.Info().Uint64("Id", id).Msg("Seesion created")
	return
}

func (h *Handler) delSession(id uint64) {
	if session := h.getSession(id); session != nil {
		session.clearFDs()
	}
	log.Info().Uint64("Id", id).Msg("Session destroyed")
	h.sessions.Delete(id)
}

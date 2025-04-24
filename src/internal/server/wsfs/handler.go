package wsfs

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
	"wsfs-core/internal/server/config"
	internalerror "wsfs-core/internal/server/internalError"
	"wsfs-core/internal/server/storage"

	"github.com/gorilla/websocket"
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

type Suser struct {
	Uid      uint32
	Gid      uint32
	OtherUid uint32
	OtherGid uint32
}

type Handler struct {
	ider         *sqids.Sqids
	upgrader     websocket.Upgrader
	errorHandler internalerror.ErrorHandler
	sessions     sync.Map
	sessionLast  atomic.Uint64
	suser        Suser

	Stop context.CancelFunc
}

func NewHandler(errorHandler internalerror.ErrorHandler, c config.WSFS) (h *Handler, err error) {
	h = &Handler{
		errorHandler: errorHandler,
	}

	h.setupUpgrader()
	h.ider, err = setupIder()
	h.suser.Uid = uint32(c.Uid)
	h.suser.Gid = uint32(c.Gid)
	h.suser.OtherUid = uint32(c.OtherUid)
	h.suser.OtherGid = uint32(c.OtherGid)
	return
}

func (h *Handler) CollecteInactivedSession() {
	var ctx context.Context
	ctx, h.Stop = context.WithCancel(context.Background())

	for {
		time.Sleep(sessionInactiveScanPeriod)
		existsSession := false
		h.sessions.Range(func(key, value any) bool {
			existsSession = true

			s := value.(*session)
			if s == nil {
				return true // continue
			}

			if s.ConnLock.TryLock() {
				s.inactiveCount += 1

				if s.inactiveCount >= sessionInactiveMaxCount {
					h.delSession(key.(uint64))
					return true // continue
				}

				s.ConnLock.Unlock()
			}
			return true
		})
		if ctx.Err() != nil && !existsSession {
			return
		}
	}
}

func (h *Handler) TryServerHTTP(rsp http.ResponseWriter, req *http.Request, user *storage.User, forced bool) (handled bool) {
	if !websocket.IsWebSocketUpgrade(req) {
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
		id = h.newSession(user.Storage)
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
	if !session.ConnLock.TryLock() {
		rsp.WriteHeader(http.StatusPreconditionFailed)
		return
	}

	// in case of panic
	var sucessed = false
	defer func() {
		if !sucessed {
			session.ConnLock.Unlock()
		}
	}()

	conn, err := h.upgrade(rsp, req, idstr)
	if err != nil {
		log.Error().Err(err).Msg("Upgrade websocket connection failed")
		return
	}
	sucessed = true

	log.Info().Str("From", req.RemoteAddr).Str("User", user.Name).Uint64("Id", id).Msg("Session running")
	session.takeConn(conn)
}

func (h *Handler) getSession(id uint64) *session {
	v, ok := h.sessions.Load(id)
	if !ok {
		return nil
	}
	return v.(*session)
}

func (h *Handler) newSession(storage *storage.Storage) (id uint64) {
	for {
		id = h.sessionLast.Add(1)
		if _, loaded := h.sessions.LoadOrStore(id, (*session)(nil)); !loaded {
			break
		}
	}
	h.sessions.Store(id, newSession(h, storage))
	log.Info().Uint64("Id", id).Msg("Seesion created")
	return
}

func (h *Handler) delSession(id uint64) {
	log.Info().Uint64("Id", id).Msg("Session destroyed")
	h.sessions.Delete(id)
}

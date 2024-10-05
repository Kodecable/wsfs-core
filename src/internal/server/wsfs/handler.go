package wsfs

import (
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
	"wsfs-core/internal/server/config"
	"wsfs-core/internal/server/storage"
	"wsfs-core/internal/util"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
	"github.com/sqids/sqids-go"
)

var (
	ErrBadSubprotocol = errors.New("wsfs: bad websocket sub protocol")
)

const (
	sessionIdMinLength        = 13
	sessionIdAlphabet         = "Lb1VKhJAxiFuNezc2fvPMns7TakWrCqmUDj4R5twBpH9oQyZXdEg863SGY"
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
	ider        *sqids.Sqids
	upgrader    websocket.Upgrader
	errorPage   util.ErrorPageFunc
	sessions    sync.Map
	sessionLast atomic.Uint64
	suser       Suser
}

func NewHandler(errorPage util.ErrorPageFunc, c *config.Server) (h Handler, err error) {
	h.setupUpgrader()
	h.errorPage = errorPage
	h.ider, err = sqids.New(sqids.Options{
		MinLength: sessionIdMinLength,
		Alphabet:  sessionIdAlphabet,
	})
	h.suser.Uid = uint32(c.Uid)
	h.suser.Gid = uint32(c.Gid)
	h.suser.OtherUid = uint32(c.OtherUid)
	h.suser.OtherGid = uint32(c.OtherGid)
	return
}

func (h *Handler) CollecteInactivedSession() {
	for {
		time.Sleep(sessionInactiveScanPeriod)
		h.sessions.Range(func(key, value any) bool {
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
	}
}

func (h *Handler) ServeHTTP(rsp http.ResponseWriter, req *http.Request, st *storage.Storage) {
	if !websocket.IsWebSocketUpgrade(req) {
		if req.Method == "GET" {
			h.errorPage(rsp, req, http.StatusBadRequest, "This is WSFS endpoint.")
		} else {
			rsp.WriteHeader(http.StatusBadRequest)
		}
		return
	}

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
		id = h.newSession(st)
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

	log.Info().Str("From", conn.RemoteAddr().String()).Uint64("Id", id).Msg("Session running")
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

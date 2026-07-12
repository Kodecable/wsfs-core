package wsfs

import (
	"errors"
	"net/http"
	"strings"
	internalerror "wsfs-core/internal/server/internalError"
	"wsfs-core/internal/server/storage"
	"wsfs-core/internal/util"

	"github.com/rs/zerolog/log"
)

var (
	ErrBadSubprotocol = errors.New("wsfs: bad websocket sub protocol")
)

type FeatureOptions struct {
	EnableLink bool
}

type Handler struct {
	errorHandler internalerror.ErrorHandler
	registry     *SessionRegistry
	fsIds        util.FsIds
	featureOpts  FeatureOptions
}

func NewHandler(errorHandler internalerror.ErrorHandler, fsIds util.FsIds, featureOpts FeatureOptions, registry *SessionRegistry) *Handler {
	return &Handler{
		errorHandler: errorHandler,
		registry:     registry,
		fsIds:        fsIds,
		featureOpts:  featureOpts,
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
		id, err = h.registry.newSession(user.Name, user.Storage, h.fsIds, h.featureOpts)
		if err != nil {
			log.Error().Err(err).Msg("Generate session id failed")
			rsp.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	session := h.registry.getSession(id)
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

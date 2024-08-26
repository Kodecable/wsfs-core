package wsfs

import (
	"net/http"

	"github.com/gorilla/websocket"
)

func (h *Handler) setupUpgrader() {
	h.upgrader.Subprotocols = []string{"WSFS/draft.1"}
	h.upgrader.EnableCompression = false
}

func (h *Handler) upgrade(rsp http.ResponseWriter, req *http.Request, resumeId string) (*websocket.Conn, error) {
	header := http.Header{}
	if len(resumeId) != 0 {
		header.Set("X-Wsfs-Resume", resumeId)
	}
	conn, err := h.upgrader.Upgrade(rsp, req, header)
	if err != nil {
		return nil, err
	}
	if conn.Subprotocol() != "WSFS/draft.1" {
		return nil, ErrBadSubprotocol
	}
	return conn, nil
}

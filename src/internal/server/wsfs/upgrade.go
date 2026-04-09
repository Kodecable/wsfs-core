package wsfs

import (
	"net/http"

	"github.com/coder/websocket"
)

func (h *Handler) upgrade(rsp http.ResponseWriter, req *http.Request, resumeId string) (*websocket.Conn, error) {
	if len(resumeId) != 0 {
		rsp.Header().Set("X-Wsfs-Resume", resumeId)
	}
	conn, err := websocket.Accept(rsp, req, &websocket.AcceptOptions{
		Subprotocols: []string{"WSFS/draft.2"},
	})
	if err != nil {
		return nil, err
	}
	if conn.Subprotocol() != "WSFS/draft.2" {
		return nil, ErrBadSubprotocol
	}
	return conn, nil
}

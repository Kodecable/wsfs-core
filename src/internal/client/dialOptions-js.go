//go:build js

package client

import (
	"net/http"

	"github.com/coder/websocket"
)

func httpDialOptions(_ *http.Client, _ http.Header) *websocket.DialOptions {
	return &websocket.DialOptions{}
}

//go:build !js

package client

import (
	"net/http"

	"github.com/coder/websocket"
)

func httpDialOptions(hc *http.Client, hh http.Header) *websocket.DialOptions {
	return &websocket.DialOptions{HTTPClient: hc, HTTPHeader: hh}
}

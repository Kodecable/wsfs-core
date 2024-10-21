package client

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/websocket"
)

func unixSocketUrl(urlStr string) (isSocket bool, socketPath, httpUrl string, err error) {
	parsedUrl, err := url.Parse(urlStr)
	if err != nil {
		return false, "", "", err
	}

	scheme := ""
	switch parsedUrl.Scheme {
	case "unix+wsfs":
		scheme = "ws"
	case "unix+wsfss":
		scheme = "wss"
	default:
		return false, "", urlStr, nil
	}
	parsedUrl.Scheme = scheme
	isSocket = true

	ok := false
	hostpath := ""
	socketPath, hostpath, ok = strings.Cut(parsedUrl.Path, "/./")
	if !ok {
		return false, "", "", errors.New("bad unix socket url, '/./' not found")
	}
	if hostpath == "" {
		hostpath = "/"
	}

	// It's needed to pass a scheme to url.Parse()
	// '//'      + '/' => path: '///'
	// 'http://' + '/' => path: '/'
	// Which scheme is not important
	hostpathUrl, err := url.Parse("http://" + hostpath)
	if !ok {
		return false, "", "", err
	}

	parsedUrl.Host = hostpathUrl.Host
	parsedUrl.Path = hostpathUrl.Path
	httpUrl = parsedUrl.String()
	return
}

func wsdial(urlStr string, requestHeader http.Header) (*websocket.Conn, *http.Response, error) {
	isSocket, socketPath, httpUrl, err := unixSocketUrl(urlStr)
	if err != nil {
		return nil, nil, err
	}

	dialer := websocket.Dialer{
		Subprotocols:      []string{"WSFS/draft.1"},
		EnableCompression: false,
		NetDialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			if isSocket {
				return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
			} else {
				return (&net.Dialer{}).DialContext(ctx, network, address)
			}
		},
	}

	return dialer.Dial(httpUrl, requestHeader)
}

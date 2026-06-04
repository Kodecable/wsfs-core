package client

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"wsfs-core/internal/share/wsfsprotocol"

	"github.com/coder/websocket"
)

func unixSocketUrl(urlStr string) (isSocket bool, socketPath, httpUrl string, err error) {
	parsedUrl, err := url.Parse(urlStr)
	if err != nil {
		return false, "", "", err
	}

	scheme := ""
	switch parsedUrl.Scheme {
	case "wsfs+unix":
		scheme = "ws"
	case "wsfss+unix":
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
	if err != nil {
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

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		if isSocket {
			return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
		}
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}

	return websocket.Dial(context.Background(), httpUrl, &websocket.DialOptions{
		HTTPClient: &http.Client{
			Transport: transport,
		},
		HTTPHeader:   requestHeader,
		Subprotocols: []string{wsfsprotocol.WSSubprotocol},
	})
}

package client

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"wsfs-core/internal/share/wsfsprotocol"

	"github.com/coder/websocket"
	"github.com/rs/zerolog/log"
)

type certHashMismatchError struct {
	Expected string
	Actual   string
}

func (e *certHashMismatchError) Error() string {
	return fmt.Sprintf("unmatched cert hash: expected %s, got %s", e.Expected, e.Actual)
}

func x509CertHash(cert *x509.Certificate) string {
	hash := sha256.Sum256(cert.Raw)
	return "SHA256:" + hex.EncodeToString(hash[:])
}

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

func tlsConfig(expectedCertHash string) *tls.Config {
	if expectedCertHash == "" {
		return &tls.Config{}
	}

	return &tls.Config{
		InsecureSkipVerify: true,
		VerifyConnection: func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) != 0 {
				actualHash := x509CertHash(cs.PeerCertificates[0])
				if actualHash == expectedCertHash {
					return nil
				}
				return &certHashMismatchError{Expected: expectedCertHash, Actual: actualHash}
			}
			return fmt.Errorf("unmatched cert hash")
		},
	}
}

func logServerCertHash(rsp *http.Response, err error) {
	if rsp != nil && rsp.TLS != nil && len(rsp.TLS.PeerCertificates) != 0 {
		log.Warn().Str("Hash", x509CertHash(rsp.TLS.PeerCertificates[0])).Msg("Server cert received")
		return
	}

	var verificationErr *tls.CertificateVerificationError
	if errors.As(err, &verificationErr) && len(verificationErr.UnverifiedCertificates) != 0 {
		log.Warn().Str("Hash", x509CertHash(verificationErr.UnverifiedCertificates[0])).Msg("Server cert received")
		return
	}

	var hashMismatchErr *certHashMismatchError
	if errors.As(err, &hashMismatchErr) {
		log.Warn().Str("Hash", hashMismatchErr.Actual).Msg("Server cert received")
	}
}

func wsdial(urlStr string, requestHeader http.Header, expectedCertHash string) (*websocket.Conn, *http.Response, error) {
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
	transport.TLSClientConfig = tlsConfig(expectedCertHash)

	conn, rsp, err := websocket.Dial(context.Background(), httpUrl, &websocket.DialOptions{
		HTTPClient: &http.Client{
			Transport: transport,
		},
		HTTPHeader:   requestHeader,
		Subprotocols: []string{wsfsprotocol.WSSubprotocol},
	})
	logServerCertHash(rsp, err)
	return conn, rsp, err
}

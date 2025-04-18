package server

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"wsfs-core/internal/server/config"

	"github.com/rs/zerolog/log"
)

func readTLSKeyPair(tlsConfig config.TLS) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(tlsConfig.CertFile, tlsConfig.KeyFile)

	if err != nil {
		return nil, err
	} else {
		return &tls.Config{
			GetCertificate: func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
				return &cert, nil
			},
		}, nil
	}
}

func listen(c config.Listener) (listener net.Listener, tlsConfig *tls.Config, err error) {
	if c.Network == "unix" {
		var fi os.FileInfo
		fi, err = os.Stat(c.Address)
		if err == nil {
			if fi.Mode()&os.ModeSocket != 0 {
				err = os.Remove(c.Address)
				if err != nil {
					err = fmt.Errorf("unable to remove old sock file: %e", err)
					return
				}
			} else {
				err = fmt.Errorf("sock file exists and not a unix socket")
				return
			}
		} else if !os.IsNotExist(err) {
			log.Warn().Err(err).Msg("unable to check sock file status")
		}
	}

	if c.TLS.Enable {
		tlsConfig, err = readTLSKeyPair(c.TLS)
		if err != nil {
			return
		}
	}

	listener, err = net.Listen(c.Network, c.Address)
	return
}

func cleanListen(c config.Listener) {
	if c.Network == "unix" {
		err := os.Remove(c.Address)
		if err != nil {
			log.Error().Err(err).Msg("Unable to remove sock file")
		}
	}
}

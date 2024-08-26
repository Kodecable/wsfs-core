package server

import (
	"crypto/tls"
	"wsfs-core/internal/server/config"
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

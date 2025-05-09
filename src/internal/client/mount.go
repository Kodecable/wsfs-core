//go:build windows || unix

package client

import (
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
	"wsfs-core/internal/client/session"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

const (
	sessionRecoveryRetryMaxCount = 30
	sessionRecoveryRetrySeconds  = 5
)

type MountOption struct {
	AttrTimeout      time.Duration
	EntryTimeout     time.Duration
	NegativeTimeout  time.Duration
	UseFusemount     bool
	VolumeLabel      string
	MasqueradeAsNtfs bool
	EnableFuseLog    bool
	FuseFsName       string
	Uid              uint32
	Gid              uint32
	NobodyUid        uint32
	NobodyGid        uint32
}

func dial(url, username, password, resumeId string) (conn *websocket.Conn, rsp *http.Response, err error) {
	header := http.Header{}
	if username != "" {
		header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(username+":"+password)))
	}
	if resumeId != "" {
		header.Set("X-Wsfs-Resume", resumeId)
	}

	conn, rsp, err = wsdial(url, header)
	if err != nil {
		return
	}
	if conn.Subprotocol() != "WSFS/draft.2" {
		log.Error().Str("Subprotocol", conn.Subprotocol()).Msg("Subprotocol mismatch")
		err = errors.New("subprotocol mismatch")
		return
	}
	return
}

func reDialFunc(url, username, password, resumeId string) func() (*websocket.Conn, error) {
	if resumeId == "" {
		return func() (*websocket.Conn, error) { return nil, errors.New("server do not support session resume") }
	}
	return func() (*websocket.Conn, error) {
		for range sessionRecoveryRetryMaxCount {
			conn, rsp, err := dial(url, username, password, resumeId)
			if err == nil {
				return conn, nil
			}

			if rsp != nil {
				switch rsp.StatusCode {
				case http.StatusUnauthorized:
					return nil, errors.New("http unauthorized")
				case http.StatusBadRequest:
					return nil, errors.New("this session can not be resumed")
				case http.StatusPreconditionFailed:
					log.Info().Msg("Waiting for session to be resumable")
				default:
					log.Error().Err(err).Msg("Uable to connect to server")
				}
			} else {
				log.Error().Err(err).Msg("Uable to connect to server")
			}

			time.Sleep(sessionRecoveryRetrySeconds * time.Second)
		}
		return nil, errors.New("too many retries")
	}
}

func logDialError(rsp *http.Response, err error) {
	if errors.Is(err, websocket.ErrBadHandshake) &&
		rsp != nil {
		if rsp.StatusCode == http.StatusUnauthorized {
			log.Error().Err(err).Msg("Bad WSFS handshake: Authorize failed")
			return
		}
		if rsp.Header.Get("Content-Type") == "text/plain" {
			msg, err_ := io.ReadAll(io.LimitReader(rsp.Body, 64))
			if err_ == nil {
				msg := string(msg)
				msg = strings.TrimSpace(msg)
				msg = strings.TrimPrefix(msg, "Bad WSFS handshake: ")
				msg = strings.TrimSpace(msg)
				log.Error().Str("Message", msg).Msg("Bad WSFS handshake")
				return
			}
		}
	}
	log.Error().Err(err).Msg("Uable to connect to server")
}

func Mount(mountpoint string, url string, username, password string, opt MountOption) error {
	conn, rsp, err := dial(url, username, password, "")
	if err != nil {
		logDialError(rsp, err)
		return err
	}

	resumeId := rsp.Header.Get("X-Wsfs-Resume")
	if resumeId == "" {
		log.Warn().Msg("Server do not support session resume")
	}

	s, err := session.NewSession(reDialFunc(url, username, password, resumeId))
	if err != nil {
		log.Error().Err(err).Msg("Uable to create session")
		return err
	}

	s.Start(conn)

	err = fuseMount(mountpoint, s, opt)
	if err != nil {
		log.Error().Err(err).Msg("Mount failed")
		return err
	}

	return nil
}

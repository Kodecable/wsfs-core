package client

import (
	"encoding/base64"
	"errors"
	"net/http"
	"time"
	"wsfs-core/internal/client/session"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

type MountOption struct {
	AttrTimeout  time.Duration
	EntryTimeout time.Duration
	//EnoentTimeout    time.Duration
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
	dialer := websocket.Dialer{
		Subprotocols:      []string{"WSFS/draft.1"},
		EnableCompression: false,
	}

	header := http.Header{}
	if username != "" {
		header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(username+":"+password)))
	}
	if resumeId != "" {
		header.Set("X-Wsfs-Resume", resumeId)
	}

	conn, rsp, err = dialer.Dial(url, header)
	if err != nil {
		log.Error().Err(err).Msg("Uable to connect to server")
		return
	}
	if conn.Subprotocol() != "WSFS/draft.1" {
		log.Error().Str("Subprotocol", conn.Subprotocol()).Msg("Subprotocol mismatch")
		err = errors.New("subprotocol mismatch")
		return
	}
	return
}

func Mount(mountpoint string, url string, username, password string, opt MountOption) error {
	conn, rsp, err := dial(url, username, password, "")
	if err != nil {
		return err
	}

	resumeId := rsp.Header.Get("X-Wsfs-Resume")
	s, err := session.NewSession(resumeId, func() *websocket.Conn {
		for {
			conn, _, err := dial(url, username, password, resumeId)
			if err == nil {
				return conn
			}
			time.Sleep(10 * time.Second)
		}
	})
	if err != nil {
		log.Error().Err(err).Msg("Uable to create session")
		return err
	}

	s.TakeConn(conn)

	err = fuseMount(mountpoint, s, opt)
	if err != nil {
		log.Error().Err(err).Msg("Mount failed")
		return err
	}

	return nil
}

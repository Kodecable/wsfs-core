package wsfs

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"wsfs-core/internal/server/storage"
	"wsfs-core/internal/util"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

var (
	ErrSessionBusy = errors.New("wsfs: session busy")
)

type session struct {
	inactiveCount uint32 // to mark unactived session
	ConnLock      sync.Mutex
	handler       *Handler
	storage       *storage.Storage
	fds           sync.Map
	fdLast        atomic.Uint32
	wg            sync.WaitGroup
}

func newSession(handler *Handler, storage *storage.Storage) *session {
	return &session{handler: handler,
		storage: storage,
		wg:      sync.WaitGroup{},
	}
}

func (s *session) newFD(sfd sfd_t) uint32 {
	var fd uint32
	for {
		fd = s.fdLast.Add(1)
		if _, loaded := s.fds.LoadOrStore(fd, sfd); !loaded {
			break
		}
	}
	return fd
}

func (s *session) takeConn(conn *websocket.Conn) {
	ctx, cancel := context.WithCancel(context.Background())
	writeCh := make(chan *util.Buffer)

	go s.writeLoop(conn, ctx, cancel, writeCh)
	go s.readLoop(conn, ctx, cancel, writeCh)
}

func (s *session) onLoopExit() {
	log.Info().Msg("Session hibernated")
	s.inactiveCount = 0
	s.ConnLock.Unlock()
}

func (s *session) readLoop(conn *websocket.Conn, ctx context.Context, cancel context.CancelFunc, writeCh chan<- *util.Buffer) {
	defer func() {
		// Cancel() can be called multiple times safely.
		cancel()

		/*
			if err := recover(); err != nil {
				if terr, ok := err.(error); ok {
					log.Error().Err(terr).Msg("Read loop panic")
				} else {
					log.Error().Any("Error", err).Msg("Read loop panic")
				}
			}*/

		s.wg.Wait()
		// Goruntine may write new response to writeCh. In case of write data
		// to a closed chan and panic, writeCh should be close after wait group
		// done.
		close(writeCh)
	}()

	for {
		msgType, reader, err := conn.NextReader()

		if err != nil {
			// If the context is cancelled, errors are already logged in the write loop.
			if ctx.Err() == nil {
				if websocket.IsUnexpectedCloseError(err) {
					log.Info().Str("From", conn.RemoteAddr().String()).Msg("Disconnected")
				} else {
					log.Error().Str("From", conn.RemoteAddr().String()).Err(err).Msg("Failed to get a reader")
				}
			}
			return
		}
		if msgType != websocket.BinaryMessage {
			log.Warn().Str("From", conn.RemoteAddr().String()).Msg("Message type is not binary")
		}

		err = s.readAndExec(reader, writeCh)
		if err != nil {
			log.Error().Str("From", conn.RemoteAddr().String()).Err(err).Msg("Failed to execute cmd")
			return
		}
	}
}

func (s *session) writeLoop(conn *websocket.Conn, ctx context.Context, cancel context.CancelFunc, writeCh <-chan *util.Buffer) {
	var err error = nil
	defer func() {
		// Cancel() can be called multiple times safely.
		cancel()
		// Close() will interrupt the read call in the read loop, allowing the
		// read loop to exit normally, and the documentation does not confirm
		// whether it is safe to call twice, so it is the job of the write loop
		// to call. The read loop will test whether the context is canceled to
		// decide whether to log a warning, so this call should be after the
		// cancel call.
		_ = conn.Close()

		if err := recover(); err != nil {
			if terr, ok := err.(error); ok {
				log.Error().Err(terr).Msg("Write loop panic")
			} else {
				log.Error().Any("Error", err).Msg("Write loop panic")
			}
		}
		// we are not sure it is possible to send msg now, so just drop datas
		// from writeCh untill it's closed.
		for {
			_, ok := <-writeCh
			if !ok {
				break
			}
		}

		s.onLoopExit()
	}()

	for {
		select {
		case buf, ok := <-writeCh:
			if !ok {
				return
			}
			//T := buf.Done()
			//log.Debug().Uint8("Cm", T[0]).Uint8("Sc", T[1]).Msg("Send response")
			err = conn.WriteMessage(websocket.BinaryMessage, buf.Done())
			bufPool.Put(buf)
			if err != nil {
				// If the context is cancelled, errors are already logged in the read loop.
				if ctx.Err() == nil {
					if websocket.IsUnexpectedCloseError(err) {
						log.Info().Str("From", conn.RemoteAddr().String()).Msg("Disconnected")
					} else {
						log.Error().Str("From", conn.RemoteAddr().String()).Err(err).Msg("Failed to write message")
					}
				}
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

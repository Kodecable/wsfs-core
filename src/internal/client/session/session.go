package session

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"wsfs-core/internal/share/wsfsprotocol"
	"wsfs-core/internal/util"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

type ReDialFunc func() (*websocket.Conn, error)

const (
	dataPerMsg = 4096

	// In cmd writeAt call:
	//  clientMark  1
	//  cmdCode     1
	//  fd          4
	//  offset      8
	//  data        dataPerMsg
	bufSize = dataPerMsg + 1 + 1 + 4 + 8
)

var (
	bufPool = sync.Pool{
		New: func() any {
			return util.NewBuffer(bufSize)
		},
	}
)

func msg(vals ...any) *util.Buffer {
	buf := bufPool.Get().(*util.Buffer)

	for _, val := range vals {
		buf.Put(val)
	}

	return buf
}

type Session struct {
	exitErr      error
	exitWg       sync.WaitGroup
	reDial       ReDialFunc
	writeRequest chan *util.Buffer
	readRequests [256]chan *util.Buffer
	clientMarks  [256]sync.Mutex
	lastMark     atomic.Uint32
}

func NewSession(reDial ReDialFunc) (*Session, error) {
	s := &Session{reDial: reDial}

	s.writeRequest = make(chan *util.Buffer)
	for i := range s.readRequests {
		s.readRequests[i] = make(chan *util.Buffer)
	}

	return s, nil
}

func (s *Session) exit(err error) {
	s.exitErr = err
	s.exitWg.Done()
}

func (s *Session) newClientMark() uint8 {
	var v uint8
	for {
		v = uint8(s.lastMark.Add(1))
		if s.clientMarks[v].TryLock() {
			break
		}
	}
	return v
}

func (s *Session) Start(conn *websocket.Conn) {
	s.exitErr = nil
	s.exitWg.Add(1)
	s.takeConn(conn)
}

func (s *Session) Wait() error {
	s.exitWg.Wait()
	return s.exitErr
}

func (s *Session) takeConn(conn *websocket.Conn) {
	ctx, cancel := context.WithCancel(context.Background())

	go s.writeLoop(conn, ctx, cancel)
	go s.readLoop(conn, ctx, cancel)
	//s.readLoop(conn, ctx, cancel)
}

func (s *Session) readLoop(conn *websocket.Conn, ctx context.Context, cancel context.CancelFunc) {
	defer func() {
		// Cancel() can be called multiple times safely.
		cancel()

		if err := recover(); err != nil {
			if terr, ok := err.(error); ok {
				log.Error().Err(terr).Msg("Read loop panic")
			} else {
				log.Error().Any("Error", err).Msg("Read loop panic")
			}
		}
	}()

	for {
		msgType, reader, err := conn.NextReader()

		if err != nil {
			// If the context is cancelled, errors are already logged in the write loop.
			if ctx.Err() == nil {
				if websocket.IsUnexpectedCloseError(err) {
					log.Error().Msg("Disconnected")
				} else {
					log.Error().Err(err).Msg("Failed to get a reader")
				}
			}
			return
		}
		if msgType != websocket.BinaryMessage {
			log.Warn().Msg("Message type is not binary")
		}

		buf := bufPool.Get().(*util.Buffer)
		_, err = io.Copy(buf, reader)
		if err != nil {
			log.Error().Err(err).Msg("Failed to read message")
			break
		}
		if !buf.Ensure(1) {
			log.Error().Msg("Bad message, too short")
			continue
		}

		//log.Debug().Uint8("Cm", buf.ReadByteAt(0)).Uint8("Sc", buf.ReadByteAt(1)).Msg("Reviced response")
		clientMark := buf.ReadByteAt(0)
		s.readRequests[clientMark] <- buf
	}
}

func (s *Session) writeLoop(conn *websocket.Conn, ctx context.Context, cancel context.CancelFunc) {
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

		s.errorMode()
	}()

	for {
		select {
		case buf := <-s.writeRequest:
			//T := buf.Done()
			//log.Debug().Bytes("data", T).Uint8("Cm", buf.ReadByteAt(0)).Uint8("Op", buf.ReadByteAt(1)).Msg("Send commnad")
			err = conn.WriteMessage(websocket.BinaryMessage, buf.Done())
			bufPool.Put(buf)
			if err != nil {
				// If the context is cancelled, errors are already logged in the read loop.
				if ctx.Err() == nil {
					if websocket.IsUnexpectedCloseError(err) {
						log.Error().Msg("Disconnected")
					} else {
						log.Error().Err(err).Msg("Failed to write message")
					}
				}
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Session) execCommand(buf *util.Buffer) *util.Buffer {
	clientMark := buf.ReadByteAt(0)
	s.writeRequest <- buf
	rsp := <-s.readRequests[clientMark]
	s.clientMarks[clientMark].Unlock()
	return rsp
}

//func (s *Session) allRequestError() {
//	for i := range s.clientMarks {
//		if s.clientMarks[i].TryLock() {
//			s.clientMarks[i].Unlock()
//			continue
//		}
//
//	}
//}

func (s *Session) errorModeWriteLoop(ctx context.Context, wg *sync.WaitGroup) {
	for {
		select {
		case buf := <-s.writeRequest:
			clientMark := buf.Done()[0]
			bufPool.Put(buf)
			s.readRequests[clientMark] <- msg(uint8(clientMark), wsfsprotocol.ErrorIO, "Session error mode")
		case <-ctx.Done():
			//s.allRequestError()
			wg.Done()
			return
		}
	}
}

func (s *Session) errorMode() {
	log.Warn().Msg("Error mode activated")
	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}

	//go s.errorModeReadLoop(ctx, &wg)
	wg.Add(1)
	go s.errorModeWriteLoop(ctx, &wg)

	if s.reDial == nil {
		util.Unused(cancel)
		log.Warn().Msg("Connection redial not configed")
		s.exit(errors.New("recovery failed"))
		return
	}
	log.Info().Msg("Try recovery session")
	conn, err := s.reDial()
	if err != nil {
		util.Unused(cancel)
		s.exit(err)
		return
	}
	log.Info().Msg("Reconnected to server")
	cancel()
	wg.Wait()
	log.Warn().Msg("Error mode deactivated")

	s.takeConn(conn)
}

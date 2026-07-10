package session

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"
	"wsfs-core/internal/share/wsfsprotocol"
	"wsfs-core/internal/util"

	"github.com/coder/websocket"
	"github.com/rs/zerolog/log"
)

type ReDialFunc func() (*websocket.Conn, error)

const (
	maxFrameSize = wsfsprotocol.MaxMsgSize
	pingTimeout  = 10 * time.Second
)

type sessionState uint8

const (
	sessionStateRunning sessionState = iota
	sessionStateRecovering
	sessionStateClosing
	sessionStateClosed
)

var (
	bufPool = sync.Pool{
		New: func() any {
			return util.NewBuffer(maxFrameSize)
		},
	}
)

type Session struct {
	exitErr error
	exitWg  sync.WaitGroup
	reDial  ReDialFunc

	pingInterval time.Duration
	exitOnce     sync.Once

	lifecycleLock sync.Mutex
	lifecycleCond *sync.Cond
	state         sessionState
	activeReqs    int

	conn      *websocket.Conn
	writer    io.WriteCloser
	writeLock sync.Mutex

	connCtx       context.Context
	connCtxCancel context.CancelFunc
	connErr       error
	connErrLock   sync.Mutex

	responses [256]chan *util.Buffer
	lastMark  atomic.Uint32
	marks     [256]sync.Mutex
}

func NewSession(reDial ReDialFunc, pingInterval time.Duration) (*Session, error) {
	s := &Session{
		reDial:       reDial,
		pingInterval: pingInterval,
		state:        sessionStateRunning,
	}
	s.lifecycleCond = sync.NewCond(&s.lifecycleLock)
	for i := range s.responses {
		s.responses[i] = make(chan *util.Buffer, 1)
	}
	return s, nil
}

func (s *Session) exit(err error) {
	s.exitOnce.Do(func() {
		s.lifecycleLock.Lock()
		s.exitErr = err
		s.state = sessionStateClosed
		s.lifecycleCond.Broadcast()
		s.lifecycleLock.Unlock()
		s.exitWg.Done()
	})
}

func (s *Session) newClientMark() (uint8, bool) {
	s.lifecycleLock.Lock()
	if s.state == sessionStateClosing || s.state == sessionStateClosed {
		s.lifecycleLock.Unlock()
		return 0, false
	}
	s.activeReqs += 1
	s.lifecycleLock.Unlock()

	var v uint8
	for {
		v = uint8(s.lastMark.Add(1))
		if s.marks[v].TryLock() {
			break
		}
	}
	return v, true
}

func (s *Session) releaseClientMark(clientMark uint8) {
	s.marks[clientMark].Unlock()

	s.lifecycleLock.Lock()
	s.activeReqs -= 1
	s.lifecycleCond.Broadcast()
	s.lifecycleLock.Unlock()
}

func (s *Session) Start(conn *websocket.Conn) {
	s.exitErr = nil
	s.exitWg.Add(1)
	s.lifecycleLock.Lock()
	s.state = sessionStateRunning
	s.lifecycleCond.Broadcast()
	s.lifecycleLock.Unlock()
	s.takeConn(conn)
}

func (s *Session) Wait() error {
	s.exitWg.Wait()
	return s.exitErr
}

func (s *Session) takeConn(conn *websocket.Conn) {
	conn.SetReadLimit(int64(maxFrameSize))
	s.conn = conn
	s.connCtx, s.connCtxCancel = context.WithCancel(context.Background())
	go s.readLoop(conn)
	if s.pingInterval > 0 {
		go s.pingLoop(conn, s.connCtx)
	}
}

func (s *Session) pingLoop(conn *websocket.Conn, ctx context.Context) {
	ticker := time.NewTicker(s.pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
		err := conn.Ping(pingCtx)
		cancel()
		if err == nil {
			continue
		}

		if s.connErrLock.TryLock() {
			s.connErr = err
		}
		s.connCtxCancel()
		return
	}
}

func (s *Session) Close() error {
	s.lifecycleLock.Lock()
	for {
		switch s.state {
		case sessionStateClosed, sessionStateClosing:
			s.lifecycleLock.Unlock()
			return s.Wait()
		case sessionStateRecovering:
			s.lifecycleCond.Wait()
		case sessionStateRunning:
			if s.activeReqs != 0 {
				s.lifecycleCond.Wait()
				continue
			}
			s.state = sessionStateClosing
			s.lifecycleCond.Broadcast()
			conn := s.conn
			s.lifecycleLock.Unlock()

			if conn == nil {
				s.exit(nil)
				return s.Wait()
			}

			if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil {
				if s.connErrLock.TryLock() {
					s.connErr = err
				}
				s.connCtxCancel()
			}
			return s.Wait()
		}
	}
}

func (s *Session) stopConn() {
	s.lifecycleLock.Lock()
	gracefulClose := s.state == sessionStateClosing
	if gracefulClose {
		s.lifecycleCond.Broadcast()
	} else if s.state != sessionStateClosed {
		s.state = sessionStateRecovering
		s.lifecycleCond.Broadcast()
	}
	s.lifecycleLock.Unlock()

	s.connCtxCancel()

	s.writeLock.Lock()
	conn := s.conn
	s.conn = nil
	s.writeLock.Unlock()
	if conn != nil && !gracefulClose {
		_ = conn.CloseNow()
	}

	if cs := websocket.CloseStatus(s.connErr); cs != -1 {
		log.Info().Int("CloseStatus", int(cs)).Msg("Disconnected")
	} else {
		log.Error().Err(s.connErr).Msg("Failed to read/write message")
	}
	s.connErrLock.Unlock()

	if gracefulClose {
		s.notifyAllMarksClosed()
		if websocket.CloseStatus(s.connErr) == websocket.StatusNormalClosure || websocket.CloseStatus(s.connErr) == websocket.StatusGoingAway {
			s.exit(nil)
		} else {
			s.exit(s.connErr)
		}
		return
	}

	s.notifyAllMarksError()

	s.errorMode()
}

func (s *Session) notifyAllMarksError() {
	s.notifyAllMarksWithDesc("Session error mode")
}

func (s *Session) notifyAllMarksClosed() {
	s.notifyAllMarksWithDesc("Session closed")
}

func (s *Session) notifyAllMarksWithDesc(desc string) {
	if len(desc) > wsfsprotocol.MaxErrorDescLength {
		desc = desc[:wsfsprotocol.MaxErrorDescLength]
	}
	for i := range 256 {
		if s.marks[i].TryLock() {
			s.marks[i].Unlock()
			continue
		}

		buf := bufPool.Get().(*util.Buffer)
		buf.Reset()
		buf.Write([]byte{uint8(i), wsfsprotocol.ErrorIO})
		if err := wsfsprotocol.WriteRspErrorToWriter(wsfsprotocol.RspError{Desc: desc}, buf); err != nil {
			buf.Reset()
			buf.Write([]byte{uint8(i), wsfsprotocol.ErrorIO})
			_ = wsfsprotocol.WriteRspErrorToWriter(wsfsprotocol.RspError{Desc: "bad synthetic error response"}, buf)
		}
		s.responses[i] <- buf
	}
}

func (s *Session) requireWrite() (ok bool) {
	s.writeLock.Lock()

	if s.conn == nil {
		s.writeLock.Unlock()
		return false
	}

	var err error
	s.writer, err = s.conn.Writer(s.connCtx, websocket.MessageBinary)
	if err != nil {
		if s.connErrLock.TryLock() {
			s.connErr = err
		}
		s.conn = nil
		s.connCtxCancel()
		s.writeLock.Unlock()
		return false
	}

	return true
}

func (s *Session) writeDone(err error) {
	if s.writer != nil {
		closeErr := s.writer.Close()
		if err == nil {
			err = closeErr
		}
		s.writer = nil
	}
	if err != nil {
		if s.connErrLock.TryLock() {
			s.connErr = err
		}
		s.conn = nil
		s.connCtxCancel()
	}
	s.writeLock.Unlock()
}

func (s *Session) beginRequest(clientMark uint8, cmd uint8) (ok bool) {
	if !s.requireWrite() {
		return false
	}
	_, err := s.writer.Write([]byte{clientMark, cmd})
	if err != nil {
		s.writeDone(err)
		return false
	}
	return true
}

func (s *Session) readLoop(conn *websocket.Conn) {
	defer func() {
		if err := util.RecoverValue(recover()); err != nil {
			log.Error().Err(err).Msg("Read loop panic")
			if s.connErrLock.TryLock() {
				s.connErr = err
			}
		}
		s.stopConn()
	}()

	for {
		msgType, reader, err := conn.Reader(s.connCtx)

		if err != nil {
			if s.connErrLock.TryLock() {
				s.connErr = err
			}
			return
		}
		if msgType != websocket.MessageBinary {
			log.Warn().Msg("Message type is not binary")
		}

		buf := bufPool.Get().(*util.Buffer)
		buf.Reset()
		_, err = io.Copy(buf, reader)
		if err != nil {
			bufPool.Put(buf)
			log.Error().Err(err).Msg("Failed to read message")
			if s.connErrLock.TryLock() {
				s.connErr = err
			}
			return
		}

		if buf.Written() < 2 {
			bufPool.Put(buf)
			log.Error().Msg("Bad message, too short")
			continue
		}
		//log.Debug().Uint8("Cm", buf.Bytes[0]).Uint8("Ec", buf.Bytes[1]).Msg("Recived response")

		clientMark := buf.Bytes[0]
		s.responses[clientMark] <- buf
	}
}

func (s *Session) errorMode() {
	log.Warn().Msg("Error mode activated")

	if s.reDial == nil {
		log.Warn().Msg("Connection redial not configed")
		s.exit(errors.New("recovery failed"))
		return
	}

	log.Info().Msg("Try recovery session")
	conn, err := s.reDial()
	if err != nil {
		s.exit(err)
		return
	}
	log.Info().Msg("Reconnected to server")

	s.takeConn(conn)
	s.lifecycleLock.Lock()
	if s.state == sessionStateRecovering {
		s.state = sessionStateRunning
		s.lifecycleCond.Broadcast()
	}
	s.lifecycleLock.Unlock()
}

package wsfs

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"wsfs-core/internal/server/storage"
	"wsfs-core/internal/share/wsfsprotocol"
	"wsfs-core/internal/util"

	"github.com/coder/websocket"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

var (
	ErrSessionBusy = errors.New("wsfs: session busy")
)

type session struct {
	// Lock is an external indicator of whether a session is running.
	Lock sync.Mutex

	Id       string
	Username string
	registry *SessionRegistry
	storage  *storage.Storage
	fsIds    util.FsIds

	inactiveCount uint32

	// this should only be read by write caller
	// read caller take another copy of conn to make sure independent
	// so writeLock's holder can set conn to nil to stop incoming write
	conn      *websocket.Conn
	writer    io.WriteCloser
	writeLock sync.Mutex

	remoteAddr string

	connCtx       context.Context
	connCtxCancel context.CancelFunc
	connErr       error
	connErrLock   sync.Mutex

	fds          sync.Map
	fdLast       atomic.Uint32
	writeStreams sync.Map

	cmdGroup    errgroup.Group
	fastBuffers chan []byte
}

func newSession(registry *SessionRegistry, id string, username string, storage *storage.Storage, fsIds util.FsIds) *session {
	s := &session{
		Id:          id,
		Username:    username,
		registry:    registry,
		storage:     storage,
		fsIds:       fsIds,
		fastBuffers: make(chan []byte, 16),
	}
	s.cmdGroup.SetLimit(64)
	for range cap(s.fastBuffers) {
		s.fastBuffers <- make([]byte, wsfsprotocol.MaxCommandLength)
	}
	return s
}

func (s *session) acquireFastBuffer() []byte {
	return <-s.fastBuffers
}

func (s *session) releaseFastBuffer(buf []byte) {
	s.fastBuffers <- buf[:cap(buf)]
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

func (s *session) takeConn(conn *websocket.Conn, remoteAddr string) {
	conn.SetReadLimit(int64(wsfsprotocol.MaxCommandLength))
	s.conn = conn
	s.remoteAddr = remoteAddr
	s.connCtx, s.connCtxCancel = context.WithCancel(context.Background())
	go s.readLoop(conn)
}

func (s *session) stopConn() {
	s.connCtxCancel()

	s.writeLock.Lock()
	conn := s.conn
	s.conn = nil
	s.writeLock.Unlock()

	connErr := s.connErr
	closeStatus := websocket.CloseStatus(connErr)
	gracefulClose := closeStatus == websocket.StatusNormalClosure || closeStatus == websocket.StatusGoingAway
	if conn != nil && closeStatus == -1 {
		_ = conn.CloseNow()
	}
	_ = s.cmdGroup.Wait()
	s.clearWriteStreams()
	s.connErrLock.Unlock()

	if gracefulClose {
		log.Info().Str("From", s.remoteAddr).Str("Id", s.Id).Msg("Session closed")
		s.registry.delSession(s.Id)
		s.Lock.Unlock()
		return
	}

	log.Error().Str("From", s.remoteAddr).Str("Id", s.Id).Err(connErr).Msg("Failed to read/write message")
	log.Info().Str("From", s.remoteAddr).Str("Id", s.Id).Msg("Session hibernated")

	s.inactiveCount = 0
	s.Lock.Unlock()
}

func (s *session) clearWriteStreams() {
	s.writeStreams.Range(func(key, _ any) bool {
		s.writeStreams.Delete(key)
		return true
	})
}

func (s *session) clearFDs() {
	s.fds.Range(func(key, value any) bool {
		s.fds.Delete(key)
		closeSFD(value.(sfd_t))
		return true
	})
}

type writeStreamState struct {
	fd           sfd_t
	offset       uint64
	written      uint64
	writeErrSent bool
}

func (s *session) loadWriteStream(clientMark uint8) (*writeStreamState, bool) {
	v, ok := s.writeStreams.Load(clientMark)
	if !ok {
		return nil, false
	}
	return v.(*writeStreamState), true
}

func (s *session) readLoop(conn *websocket.Conn) {
	defer func() {
		var err error
		if err = util.RecoverValue(recover()); err != nil {
			log.Error().Err(err).Msg("Read loop panic")
			if s.connErrLock.TryLock() {
				s.connErr = err
			}
		}
		s.stopConn()

		_ = s.cmdGroup.Wait()
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
			log.Warn().Str("From", s.remoteAddr).Msg("Message type is not binary")
		}

		err = s.dispatchCommand(reader)
		if err != nil {
			if s.connErrLock.TryLock() {
				s.connErr = err
			}
			return
		}
	}
}

func (s *session) requireWrite() (ok bool) {
	var err error

	s.writeLock.Lock()
	if s.conn == nil {
		s.writeLock.Unlock()
		return false
	}

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

func (s *session) writeDone(err error) {
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

func (s *session) write(d []byte) {
	if !s.requireWrite() {
		return
	}
	_, err := s.writer.Write(d)
	s.writeDone(err)
}

func (s *session) beginRsp(clientMark uint8, ec uint8) bool {
	if !s.requireWrite() {
		return false
	}
	_, err := s.writer.Write([]byte{clientMark, ec})
	if err != nil {
		s.writeDone(err)
		return false
	}
	return true
}

func (s *session) writeRspOK(clientMark uint8) {
	if s.beginRsp(clientMark, wsfsprotocol.ErrorOK) {
		s.writeDone(nil)
	}
}

func (s *session) writeRspError(clientMark uint8, ec uint8, desc string) {
	if !s.beginRsp(clientMark, ec) {
		return
	}
	err := wsfsprotocol.WriteRspErrorToWriter(wsfsprotocol.RspError{Desc: desc}, s.writer)
	s.writeDone(err)
}

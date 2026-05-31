package session

import (
	"bytes"
	"errors"
	"wsfs-core/internal/share/wsfsprotocol"

	"github.com/rs/zerolog/log"
)

const (
	maxWriteStreamOpenPayload int = maxFrameSize - 14 // header(2) + FD(4) + Offset(8)
	maxWriteStreamDataPayload int = maxFrameSize - 3  // header(2) + IsEnd(1)
)

type WriteStream struct {
	session      *Session
	clientMark   uint8
	writeErrCode uint8
	writeErrDesc string
	closed       bool
}

func (s *Session) OpenWriteStream(fd uint32, offset uint64, first []byte) (*WriteStream, error) {
	clientMark := s.newClientMark()
	stream := &WriteStream{
		session:    s,
		clientMark: clientMark,
	}

	firstChunk := first
	rest := []byte(nil)
	if len(first) > maxWriteStreamOpenPayload {
		firstChunk = first[:maxWriteStreamOpenPayload]
		rest = first[maxWriteStreamOpenPayload:]
	}

	if !s.beginRequest(clientMark, wsfsprotocol.CmdWriteStreamOpen) {
		s.marks[clientMark].Unlock()
		return nil, errWriteStreamIO
	}
	err := wsfsprotocol.WriteCmdWriteStreamOpenStructToWriter(wsfsprotocol.CmdWriteStreamOpenStruct{
		FD:     fd,
		Offset: offset,
		Data:   firstChunk,
	}, s.writer)
	s.writeDone(err)
	if err != nil {
		s.marks[clientMark].Unlock()
		return nil, errWriteStreamIO
	}

	if len(rest) > 0 {
		if err := stream.Write(rest); err != nil {
			stream.closed = true
			s.marks[clientMark].Unlock()
			return nil, err
		}
	}

	return stream, nil
}

func (ws *WriteStream) Write(data []byte) error {
	if ws.closed {
		return errWriteStreamClosed
	}

	for len(data) > 0 {
		chunk := data
		if len(chunk) > maxWriteStreamDataPayload {
			chunk = data[:maxWriteStreamDataPayload]
		}

		if !ws.session.beginRequest(ws.clientMark, wsfsprotocol.CmdWriteStreamData) {
			return errWriteStreamIO
		}
		err := wsfsprotocol.WriteCmdWriteStreamDataStructToWriter(wsfsprotocol.CmdWriteStreamDataStruct{
			Data:  chunk,
			IsEnd: 0,
		}, ws.session.writer)
		ws.session.writeDone(err)
		if err != nil {
			return errWriteStreamIO
		}

		data = data[len(chunk):]
	}

	return nil
}

func (ws *WriteStream) WriteError() (uint8, string) {
	return ws.writeErrCode, ws.writeErrDesc
}

func (ws *WriteStream) Close(last []byte) (written uint64, code uint8, desc string) {
	if ws.closed {
		return 0, wsfsprotocol.ErrorInvail, "write stream already closed"
	}

	if len(last) > maxWriteStreamDataPayload {
		if err := ws.Write(last[:len(last)-maxWriteStreamDataPayload]); err != nil {
			ws.closed = true
			ws.session.marks[ws.clientMark].Unlock()
			return 0, wsfsprotocol.ErrorUnknown, err.Error()
		}
		last = last[len(last)-maxWriteStreamDataPayload:]
	}
	ws.closed = true

	if !ws.session.beginRequest(ws.clientMark, wsfsprotocol.CmdWriteStreamData) {
		ws.session.marks[ws.clientMark].Unlock()
		return 0, wsfsprotocol.ErrorUnknown, "session error mode"
	}
	err := wsfsprotocol.WriteCmdWriteStreamDataStructToWriter(wsfsprotocol.CmdWriteStreamDataStruct{
		Data:  last,
		IsEnd: 1,
	}, ws.session.writer)
	ws.session.writeDone(err)
	if err != nil {
		ws.session.marks[ws.clientMark].Unlock()
		return 0, wsfsprotocol.ErrorUnknown, err.Error()
	}

	defer ws.session.marks[ws.clientMark].Unlock()

	for {
		rsp := <-ws.session.responses[ws.clientMark]
		code = rsp.Bytes[1]
		payload := rsp.Bytes[2:rsp.Writted()]

		switch code {
		case wsfsprotocol.ErrorOK:
			var closeRsp wsfsprotocol.RspWriteStreamClose
			err = wsfsprotocol.ReadRspWriteStreamCloseFromReader(&closeRsp, bytes.NewReader(payload))
			bufPool.Put(rsp)
			if err != nil {
				log.Error().Err(err).Msg("Failed to decode WriteStream close response")
				return 0, wsfsprotocol.ErrorUnknown, "bad close response"
			}
			return closeRsp.Written, code, ""
		case wsfsprotocol.ErrorUnknown:
			desc = readRspErrorDesc(payload)
			bufPool.Put(rsp)
			return 0, code, desc
		default:
			errDesc := readRspErrorDesc(payload)
			if code == wsfsprotocol.ErrorIO && errDesc == "Session error mode" {
				bufPool.Put(rsp)
				return 0, wsfsprotocol.ErrorUnknown, errDesc
			}
			if ws.writeErrCode == 0 {
				ws.writeErrCode = code
				ws.writeErrDesc = errDesc
			}
			bufPool.Put(rsp)
		}
	}
}

var (
	errWriteStreamIO     = errors.New("write stream I/O error")
	errWriteStreamClosed = errors.New("write stream closed")
)

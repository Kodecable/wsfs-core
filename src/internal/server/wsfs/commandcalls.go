// Code generated by "genServerCommandCalls.py". DO NOT EDIT.
package wsfs

import (
	"encoding/binary"
	"errors"
	"io"
	"wsfs-core/internal/share/wsfsprotocol"
	"wsfs-core/internal/util"
)

func (s *session) doCommandCall(clientMark, cmd uint8, r io.Reader, writeCh chan<- *util.Buffer) (err error) {
	switch cmd {
case wsfsprotocol.CmdOpen:
    var v0 string
    err = util.CopyStrFromReader(r, &v0)
    if err != nil {
        goto BadCmdFormat
    }
    var v1 uint32
    err = binary.Read(r, binary.LittleEndian, &v1)
    if err != nil {
        goto BadCmdFormat
    }
    var v2 uint32
    err = binary.Read(r, binary.LittleEndian, &v2)
    if err != nil {
        goto BadCmdFormat
    }
    s.cmdOpen(clientMark, writeCh, v0, v1, v2)
case wsfsprotocol.CmdClose:
    var v0 uint32
    err = binary.Read(r, binary.LittleEndian, &v0)
    if err != nil {
        goto BadCmdFormat
    }
    s.cmdClose(clientMark, writeCh, v0)
case wsfsprotocol.CmdRead:
    var v0 uint32
    err = binary.Read(r, binary.LittleEndian, &v0)
    if err != nil {
        goto BadCmdFormat
    }
    var v1 uint64
    err = binary.Read(r, binary.LittleEndian, &v1)
    if err != nil {
        goto BadCmdFormat
    }
    s.cmdRead(clientMark, writeCh, v0, v1)
case wsfsprotocol.CmdReadDir:
    var v0 string
    err = util.CopyStrFromReader(r, &v0)
    if err != nil {
        goto BadCmdFormat
    }
    s.cmdReadDir(clientMark, writeCh, v0)
case wsfsprotocol.CmdReadLink:
    var v0 string
    err = util.CopyStrFromReader(r, &v0)
    if err != nil {
        goto BadCmdFormat
    }
    s.cmdReadLink(clientMark, writeCh, v0)
case wsfsprotocol.CmdWrite:
    var v0 uint32
    err = binary.Read(r, binary.LittleEndian, &v0)
    if err != nil {
        goto BadCmdFormat
    }
    var v1 = bufPool.Get().(*util.Buffer)
    _, err = io.Copy(v1, r)
    if err != nil {
        goto BadCmdFormat
    }
    s.cmdWrite(clientMark, writeCh, v0, v1)
case wsfsprotocol.CmdSeek:
    var v0 uint32
    err = binary.Read(r, binary.LittleEndian, &v0)
    if err != nil {
        goto BadCmdFormat
    }
    var v1 uint8
    err = binary.Read(r, binary.LittleEndian, &v1)
    if err != nil {
        goto BadCmdFormat
    }
    var v2 int64
    err = binary.Read(r, binary.LittleEndian, &v2)
    if err != nil {
        goto BadCmdFormat
    }
    s.cmdSeek(clientMark, writeCh, v0, v1, v2)
case wsfsprotocol.CmdAllocate:
    var v0 uint32
    err = binary.Read(r, binary.LittleEndian, &v0)
    if err != nil {
        goto BadCmdFormat
    }
    var v1 uint32
    err = binary.Read(r, binary.LittleEndian, &v1)
    if err != nil {
        goto BadCmdFormat
    }
    var v2 uint64
    err = binary.Read(r, binary.LittleEndian, &v2)
    if err != nil {
        goto BadCmdFormat
    }
    var v3 uint64
    err = binary.Read(r, binary.LittleEndian, &v3)
    if err != nil {
        goto BadCmdFormat
    }
    s.cmdAllocate(clientMark, writeCh, v0, v1, v2, v3)
case wsfsprotocol.CmdGetAttr:
    var v0 string
    err = util.CopyStrFromReader(r, &v0)
    if err != nil {
        goto BadCmdFormat
    }
    s.cmdGetAttr(clientMark, writeCh, v0)
case wsfsprotocol.CmdSetAttr:
    var v0 string
    err = util.CopyStrFromReader(r, &v0)
    if err != nil {
        goto BadCmdFormat
    }
    var v1 uint8
    err = binary.Read(r, binary.LittleEndian, &v1)
    if err != nil {
        goto BadCmdFormat
    }
    var v2 uint64
    err = binary.Read(r, binary.LittleEndian, &v2)
    if err != nil {
        goto BadCmdFormat
    }
    var v3 int64
    err = binary.Read(r, binary.LittleEndian, &v3)
    if err != nil {
        goto BadCmdFormat
    }
    var v4 uint32
    err = binary.Read(r, binary.LittleEndian, &v4)
    if err != nil {
        goto BadCmdFormat
    }
    var v5 uint8
    err = binary.Read(r, binary.LittleEndian, &v5)
    if err != nil {
        goto BadCmdFormat
    }
    s.cmdSetAttr(clientMark, writeCh, v0, v1, v2, v3, v4, v5)
case wsfsprotocol.CmdSync:
    var v0 uint32
    err = binary.Read(r, binary.LittleEndian, &v0)
    if err != nil {
        goto BadCmdFormat
    }
    s.cmdSync(clientMark, writeCh, v0)
case wsfsprotocol.CmdMkdir:
    var v0 string
    err = util.CopyStrFromReader(r, &v0)
    if err != nil {
        goto BadCmdFormat
    }
    var v1 uint32
    err = binary.Read(r, binary.LittleEndian, &v1)
    if err != nil {
        goto BadCmdFormat
    }
    s.cmdMkdir(clientMark, writeCh, v0, v1)
case wsfsprotocol.CmdSymLink:
    var v0 string
    err = util.CopyStrFromReader(r, &v0)
    if err != nil {
        goto BadCmdFormat
    }
    var v1 string
    err = util.CopyStrFromReader(r, &v1)
    if err != nil {
        goto BadCmdFormat
    }
    s.cmdSymLink(clientMark, writeCh, v0, v1)
case wsfsprotocol.CmdRemove:
    var v0 string
    err = util.CopyStrFromReader(r, &v0)
    if err != nil {
        goto BadCmdFormat
    }
    s.cmdRemove(clientMark, writeCh, v0)
case wsfsprotocol.CmdRmDir:
    var v0 string
    err = util.CopyStrFromReader(r, &v0)
    if err != nil {
        goto BadCmdFormat
    }
    s.cmdRmDir(clientMark, writeCh, v0)
case wsfsprotocol.CmdFsStat:
    var v0 string
    err = util.CopyStrFromReader(r, &v0)
    if err != nil {
        goto BadCmdFormat
    }
    s.cmdFsStat(clientMark, writeCh, v0)
case wsfsprotocol.CmdReadAt:
    var v0 uint32
    err = binary.Read(r, binary.LittleEndian, &v0)
    if err != nil {
        goto BadCmdFormat
    }
    var v1 uint64
    err = binary.Read(r, binary.LittleEndian, &v1)
    if err != nil {
        goto BadCmdFormat
    }
    var v2 uint64
    err = binary.Read(r, binary.LittleEndian, &v2)
    if err != nil {
        goto BadCmdFormat
    }
    s.cmdReadAt(clientMark, writeCh, v0, v1, v2)
case wsfsprotocol.CmdWriteAt:
    var v0 uint32
    err = binary.Read(r, binary.LittleEndian, &v0)
    if err != nil {
        goto BadCmdFormat
    }
    var v1 uint64
    err = binary.Read(r, binary.LittleEndian, &v1)
    if err != nil {
        goto BadCmdFormat
    }
    var v2 = bufPool.Get().(*util.Buffer)
    _, err = io.Copy(v2, r)
    if err != nil {
        goto BadCmdFormat
    }
    s.cmdWriteAt(clientMark, writeCh, v0, v1, v2)
case wsfsprotocol.CmdCopyFileRange:
    var v0 uint32
    err = binary.Read(r, binary.LittleEndian, &v0)
    if err != nil {
        goto BadCmdFormat
    }
    var v1 uint32
    err = binary.Read(r, binary.LittleEndian, &v1)
    if err != nil {
        goto BadCmdFormat
    }
    var v2 uint64
    err = binary.Read(r, binary.LittleEndian, &v2)
    if err != nil {
        goto BadCmdFormat
    }
    var v3 uint64
    err = binary.Read(r, binary.LittleEndian, &v3)
    if err != nil {
        goto BadCmdFormat
    }
    var v4 uint64
    err = binary.Read(r, binary.LittleEndian, &v4)
    if err != nil {
        goto BadCmdFormat
    }
    s.cmdCopyFileRange(clientMark, writeCh, v0, v1, v2, v3, v4)
case wsfsprotocol.CmdRename:
    var v0 string
    err = util.CopyStrFromReader(r, &v0)
    if err != nil {
        goto BadCmdFormat
    }
    var v1 string
    err = util.CopyStrFromReader(r, &v1)
    if err != nil {
        goto BadCmdFormat
    }
    var v2 uint32
    err = binary.Read(r, binary.LittleEndian, &v2)
    if err != nil {
        goto BadCmdFormat
    }
    s.cmdRename(clientMark, writeCh, v0, v1, v2)
case wsfsprotocol.CmdSetAttrByFD:
    var v0 uint32
    err = binary.Read(r, binary.LittleEndian, &v0)
    if err != nil {
        goto BadCmdFormat
    }
    var v1 uint8
    err = binary.Read(r, binary.LittleEndian, &v1)
    if err != nil {
        goto BadCmdFormat
    }
    var v2 uint64
    err = binary.Read(r, binary.LittleEndian, &v2)
    if err != nil {
        goto BadCmdFormat
    }
    var v3 int64
    err = binary.Read(r, binary.LittleEndian, &v3)
    if err != nil {
        goto BadCmdFormat
    }
    var v4 uint32
    err = binary.Read(r, binary.LittleEndian, &v4)
    if err != nil {
        goto BadCmdFormat
    }
    var v5 uint8
    err = binary.Read(r, binary.LittleEndian, &v5)
    if err != nil {
        goto BadCmdFormat
    }
    s.cmdSetAttrByFD(clientMark, writeCh, v0, v1, v2, v3, v4, v5)
case wsfsprotocol.CmdTreeDir:
    var v0 string
    err = util.CopyStrFromReader(r, &v0)
    if err != nil {
        goto BadCmdFormat
    }
    var v1 uint8
    err = binary.Read(r, binary.LittleEndian, &v1)
    if err != nil {
        goto BadCmdFormat
    }
    var v2 string
    err = util.CopyStrFromReader(r, &v2)
    if err != nil {
        goto BadCmdFormat
    }
    s.cmdTreeDir(clientMark, writeCh, v0, v1, v2)
	default:
		err = errors.New("unknwon command")
		writeCh <- msg(clientMark, wsfsprotocol.ErrorInvail, "Unknwon command")
		s.wg.Done()
	}
	return
BadCmdFormat:
	writeCh <- msg(clientMark, wsfsprotocol.ErrorInvail, "Bad command format")
	s.wg.Done()
	return
}

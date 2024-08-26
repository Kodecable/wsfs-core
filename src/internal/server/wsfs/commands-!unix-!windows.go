//go:build !unix && !windows

package wsfs

import (
	"wsfs-core/internal/share/wsfsprotocol"
	"wsfs-core/internal/util"
)

func (s *session) cmdFsStat(clientMark uint8, writeCh chan<- *util.Buffer, _ string) {
	defer s.wg.Done()

	//fake data
	writeCh <- msg(clientMark, wsfsprotocol.ErrorOK,
		uint64(10995116277760),
		uint64(5497558138880),
		uint64(5497558138880),
	)
}

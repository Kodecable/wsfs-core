//go:build linux

package wsfsunixconv

import (
	"wsfs-core/internal/share/wsfsprotocol"

	"golang.org/x/sys/unix"
)

var RenameFlagToUnix = map[uint32]uint32{
	wsfsprotocol.RENAME_NOREPLACE: unix.RENAME_NOREPLACE,
	wsfsprotocol.RENAME_EXCHANGE:  unix.RENAME_EXCHANGE,
	wsfsprotocol.RENAME_WHITEOUT:  unix.RENAME_WHITEOUT,
}

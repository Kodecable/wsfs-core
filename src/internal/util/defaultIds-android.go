//go:build android

package util

import (
	"os/user"
	"strconv"
)

const uidBitSize = 32

func GetDefaultFsIds() (FsIds, error) {
	currentuser, err := user.Current()
	if err != nil {
		return FsIds{}, err
	}
	uid, err := strconv.ParseUint(currentuser.Uid, 10, uidBitSize)
	if err != nil {
		return FsIds{}, err
	}
	gid, err := strconv.ParseUint(currentuser.Gid, 10, uidBitSize)
	if err != nil {
		return FsIds{}, err
	}

	return FsIds{
		Uid:      uint32(uid),
		Gid:      uint32(gid),
		OtherUid: uint32(9999),
		OtherGid: uint32(9999),
	}, nil
}

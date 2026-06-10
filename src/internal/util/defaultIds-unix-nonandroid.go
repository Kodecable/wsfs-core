//go:build unix && !android

package util

import (
	"os/user"
	"strconv"
)

const uidBitSize = 32
const nobodyId = "nobody"

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

	nobody, err := user.Lookup(nobodyId)
	if err != nil {
		return FsIds{}, err
	}
	nuid, err := strconv.ParseUint(nobody.Uid, 10, uidBitSize)
	if err != nil {
		return FsIds{}, err
	}
	ngid, err := strconv.ParseUint(nobody.Gid, 10, uidBitSize)
	if err != nil {
		return FsIds{}, err
	}

	return FsIds{
		Uid:      uint32(uid),
		Gid:      uint32(gid),
		OtherUid: uint32(nuid),
		OtherGid: uint32(ngid),
	}, nil
}

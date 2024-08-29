//go:build android

package util

import (
	"os/user"
	"strconv"
)

const uidBitSize = 32

func GetDefaultIds() (IdInfo, error) {
	currentuser, err := user.Current()
	if err != nil {
		return IdInfo{}, err
	}
	uid, err := strconv.ParseUint(currentuser.Uid, 10, uidBitSize)
	if err != nil {
		return IdInfo{}, err
	}
	gid, err := strconv.ParseUint(currentuser.Gid, 10, uidBitSize)
	if err != nil {
		return IdInfo{}, err
	}

	return IdInfo{CurrentUser: uint32(uid),
		UserGroup:   uint32(gid),
		NobodyUser:  uint32(9999),
		NobodyGroup: uint32(9999)}, nil
}

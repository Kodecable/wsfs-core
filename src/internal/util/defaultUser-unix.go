//go:build unix

package util

import (
	"os/user"
	"strconv"
)

const uidBitSize = 32
const nobodyId = "nobody"

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

	nobody, err := user.Lookup(nobodyId)
	if err != nil {
		return IdInfo{}, err
	}
	nuid, err := strconv.ParseUint(nobody.Uid, 10, uidBitSize)
	if err != nil {
		return IdInfo{}, err
	}
	ngid, err := strconv.ParseUint(nobody.Uid, 10, uidBitSize)
	if err != nil {
		return IdInfo{}, err
	}

	return IdInfo{CurrentUser: uint32(uid),
		UserGroup:   uint32(gid),
		NobodyUser:  uint32(nuid),
		NobodyGroup: uint32(ngid)}, nil
}

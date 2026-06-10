package util

import "fmt"

type FsIds struct {
	Uid      uint32
	Gid      uint32
	OtherUid uint32
	OtherGid uint32
}

type OptionalFsIds struct {
	Uid      *uint32
	Gid      *uint32
	OtherUid *uint32
	OtherGid *uint32
}

func (ids OptionalFsIds) Resolve() (resolved FsIds, err error) {
	if ids.Uid == nil || ids.Gid == nil ||
		ids.OtherUid == nil || ids.OtherGid == nil {
		resolved, err = GetDefaultFsIds()
		if err != nil {
			return resolved, fmt.Errorf("unable to determine default filesystem ids: %w", err)
		}
	}

	if ids.Uid != nil {
		resolved.Uid = *ids.Uid
	}
	if ids.Gid != nil {
		resolved.Gid = *ids.Gid
	}
	if ids.OtherUid != nil {
		resolved.OtherUid = *ids.OtherUid
	}
	if ids.OtherGid != nil {
		resolved.OtherGid = *ids.OtherGid
	}

	return resolved, nil
}

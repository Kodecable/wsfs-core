package wsfsprotocol

//go:generate go run genStructHelper.go 0 "!amd64" "struct.go" "struct.gen.go"
//go:generate go run genStructHelper.go 1 "amd64" "struct.go" "struct_amd64.gen.go"

type FileInfo struct {
	Size  uint64
	MTime int64
	Mode  uint32
	Owner uint8
}

type Dirent struct {
	Name  string
	Size  uint64
	MTime int64
	Mode  uint32
	Owner uint8
}

type CmdOpenStruct struct {
	Path  string
	OFlag uint32
	FMode uint32
}

type CmdCloseStruct struct {
	FD uint32
}

type CmdReadStruct struct {
	FD   uint32
	Size uint64
}

type CmdReadDirStruct struct {
	Path string
}

type CmdReadLinkStruct struct {
	Path string
}

type CmdWriteStruct struct {
	FD   uint32
	Data []byte
}

type CmdSeekStruct struct {
	FD     uint32
	Flag   uint32
	Offset int64
}

type CmdAllocateStruct struct {
	FD     uint32
	Flag   uint32
	Offset uint64
	Size   uint64
}

type CmdGetAttrStruct struct {
	Path string
}

type CmdSetAttrStruct struct {
	Path string
	Flag uint8
	FI   FileInfo
}

type CmdSyncStruct struct {
	FD uint32
}

type CmdMkdirStruct struct {
	Path string
	Mode uint32
}

type CmdSymLinkStruct struct {
	TargetPath string
	FilePath   string
}

type CmdRemoveStruct struct {
	Path string
}

type CmdRmDirStruct struct {
	Path string
}

type CmdFsStatStruct struct {
	Path string
}

type CmdReadAtStruct struct {
	FD     uint32
	Offset uint64
	Size   uint64
}

type CmdWriteAtStruct struct {
	FD     uint32
	Offset uint64
	Data   []byte
}

type CmdRenameStruct struct {
	OldPath string
	NewPath string
	Flag    uint32
}

type CmdCopyFileRangeStruct struct {
	SrcFD     uint32
	DstFD     uint32
	SrcOffset uint64
	DstOffset uint64
	Size      uint64
}

type CmdSetAttrByFDStruct struct {
	FD   uint32
	Flag uint8
	FI   FileInfo
}

type CmdReadDirPlusStruct struct {
	Path string
}

type CmdWriteStreamOpenStruct struct {
	FD     uint32
	Offset uint64
	Data   []byte
}

type CmdWriteStreamDataStruct struct {
	IsEnd uint8
	Data  []byte
}

type RspError struct {
	Desc string
}

type RspOpen struct {
	FD uint32
}

type RspClose struct{}

type RspRead struct {
	Data []byte
}

type RspReadDir struct {
	Data []byte
}

type RspReadLink struct {
	TargetPath string
}

type RspWrite struct {
	Written uint64
}

type RspSeek struct {
	Offset uint64
}

type RspAllocate struct{}

type RspGetAttr struct {
	FI FileInfo
}

type RspSetAttr struct{}

type RspSync struct{}

type RspMkdir struct{}

type RspSymLink struct{}

type RspRemove struct{}

type RspRmDir struct{}

type RspFsStat struct {
	Total     uint64
	Free      uint64
	Available uint64
}

type RspReadAt struct {
	Data []byte
}

type RspWriteAt struct {
	Written uint64
}

type RspCopyFileRange struct {
	Copied uint64
}

type RspRename struct{}

type RspSetAttrByFD struct{}


type RspWriteStreamClose struct {
	Written uint64
}

package wsfsprotocol

// There are consts defined by spec

const (
	MaxMsgSize        int = 8192
	MaxCommandLength  int = MaxMsgSize
	MaxResponseLength int = MaxMsgSize
	WSSubprotocol         = "WSFS/draft.6"
)

const MaxErrorDescLength int = MaxResponseLength - 4 // header(2) + string length prefix(2)

const (
	CmdOpen            uint8 = 1
	CmdClose           uint8 = 2
	CmdRead            uint8 = 3
	CmdReadDir         uint8 = 4
	CmdReadLink        uint8 = 5
	CmdWrite           uint8 = 6
	CmdSeek            uint8 = 7
	CmdAllocate        uint8 = 8
	CmdGetAttr         uint8 = 9
	CmdSetAttr         uint8 = 10
	CmdSync            uint8 = 11
	CmdMkdir           uint8 = 12
	CmdSymLink         uint8 = 13
	CmdRemove          uint8 = 14
	CmdRmDir           uint8 = 15
	CmdFsStat          uint8 = 16
	CmdReadAt          uint8 = 17
	CmdWriteAt         uint8 = 18
	CmdCopyFileRange   uint8 = 19
	CmdRename          uint8 = 20
	CmdSetAttrByFD     uint8 = 21
	CmdReadDirPlus     uint8 = 22
	CmdWriteStreamOpen uint8 = 23
	CmdWriteStreamData uint8 = 24
	CmdCloneFileRange  uint8 = 25
	CmdGetFileLock     uint8 = 26
	CmdSetFileLock     uint8 = 27
	CmdSetFileLockWait uint8 = 28
)

const (
	ErrorOK                 uint8 = 0
	ErrorPartialResponse    uint8 = 1
	ErrorUnknown            uint8 = 2
	ErrorBusy               uint8 = 3
	ErrorExists             uint8 = 4
	ErrorNotExists          uint8 = 5
	ErrorLoop               uint8 = 6
	ErrorNoSpace            uint8 = 7
	ErrorNotEmpty           uint8 = 8
	ErrorInvalid            uint8 = 9
	ErrorInvalidFD          uint8 = 10
	ErrorType               uint8 = 11
	ErrorIO                 uint8 = 12
	ErrorNotSupport         uint8 = 13
	ErrorAccessRestricted   uint8 = 14
	ErrorTooLong            uint8 = 15
	ErrorStateBlocked       uint8 = 16
	ErrorSpecialFileBlocked uint8 = 17
	ErrorCrossDevice        uint8 = 18
)

const (
	O_RDONLY  uint32 = 0x0
	O_WRONLY  uint32 = 0x1
	O_RDWR    uint32 = 0x2
	O_ACCMODE uint32 = 0x3

	O_TRUNC     uint32 = 0x200
	O_EXCL      uint32 = 0x80
	O_CREAT     uint32 = 0x40
	O_DIRECTORY uint32 = 0x10000
	O_APPEND    uint32 = 0x400
	O_SYNC      uint32 = 0x101000
	O_DSYNC     uint32 = 0x1000
	O_NOFOLLOW  uint32 = 0x20000
	O_NOATIME   uint32 = 0x40000
	O_DIRECT    uint32 = 0x4000
)

var OpenFlags = []uint32{
	O_RDONLY,
	O_WRONLY,
	O_RDWR,
	O_TRUNC,
	O_EXCL,
	O_CREAT,
	O_DIRECTORY,
	O_APPEND,
	O_SYNC,
	O_DSYNC,
	O_NOFOLLOW,
	O_NOATIME,
	O_DIRECT,
}

const (
	FALLOC_FL_FALLOCATE      uint32 = 0x00
	FALLOC_FL_KEEP_SIZE      uint32 = 0x01
	FALLOC_FL_PUNCH_HOLE     uint32 = 0x02
	FALLOC_FL_COLLAPSE_RANGE uint32 = 0x08
	FALLOC_FL_ZERO_RANGE     uint32 = 0x10
	FALLOC_FL_INSERT_RANGE   uint32 = 0x20
	FALLOC_FL_UNSHARE_RANGE  uint32 = 0x40
)

const (
	WHENCE_SET  uint8 = 0
	WHENCE_CUR  uint8 = 1
	WHENCE_END  uint8 = 2
	WHENCE_DATA uint8 = 3
	WHENCE_HOLE uint8 = 4
)

const (
	RENAME_NOREPLACE uint32 = 0x1
	RENAME_EXCHANGE  uint32 = 0x2
	RENAME_WHITEOUT  uint32 = 0x4
)

var RenameFlags = []uint32{
	RENAME_NOREPLACE,
	RENAME_EXCHANGE,
	RENAME_WHITEOUT,
}

const (
	OWNER_NN uint8 = 0
	OWNER_UN uint8 = 1
	OWNER_NG uint8 = 2
	OWNER_UG uint8 = 3
)

const (
	SETATTR_SIZE  uint8 = 0b0001
	SETATTR_MTIME uint8 = 0b0010
	SETATTR_MODE  uint8 = 0b0100
	SETATTR_OWNER uint8 = 0b1000
)

const (
	READDIRPLUS_INDICATOR_CONTINUE      uint8 = 1
	READDIRPLUS_INDICATOR_PREFETCH      uint8 = 2
	READDIRPLUS_INDICATOR_PREFETCH_SKIP uint8 = 3
)

const (
	FILELOCK_UNLOCK    uint8 = 0
	FILELOCK_READLOCK  uint8 = 1
	FILELOCK_WRITELOCK uint8 = 2
)

const (
	// MaxCopyFileRangeChunk is the maximum number of bytes a single
	// CmdCopyFileRange request may ask the server to copy. Larger copies
	// must be split into multiple requests by the client. This bounds the
	// per-call syscall work (DoS protection) and keeps the length within
	// int32 on 32-bit platforms so it can be passed to copy_file_range
	// without truncation.
	MaxCopyFileRangeChunk uint64 = 32 << 20 // 32 MiB
)

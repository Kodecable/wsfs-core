package wsfsprotocol

// There are consts defined by spec

const (
	MaxCommandLength  int = 8192
	MaxResponseLength int = MaxCommandLength
)

const (
	CmdOpen          uint8 = 1
	CmdClose         uint8 = 2
	CmdRead          uint8 = 3
	CmdReadDir       uint8 = 4
	CmdReadLink      uint8 = 5
	CmdWrite         uint8 = 6
	CmdSeek          uint8 = 7
	CmdAllocate      uint8 = 8
	CmdGetAttr       uint8 = 9
	CmdSetAttr       uint8 = 10
	CmdSync          uint8 = 11
	CmdMkdir         uint8 = 12
	CmdSymLink       uint8 = 13
	CmdRemove        uint8 = 14
	CmdRmDir         uint8 = 15
	CmdFsStat        uint8 = 16
	CmdReadAt        uint8 = 17
	CmdWriteAt       uint8 = 18
	CmdCopyFileRange uint8 = 19
	CmdRename        uint8 = 20
	CmdSetAttrByFD   uint8 = 21
	CmdTreeDir       uint8 = 22
)

const (
	ErrorOK              uint8 = 0
	ErrorPartialResponse uint8 = 1
	ErrorUnknown         uint8 = 2
	ErrorBusy            uint8 = 3
	ErrorExists          uint8 = 4
	ErrorNotExists       uint8 = 5
	ErrorLoop            uint8 = 6
	ErrorNoSpace         uint8 = 7
	ErrorNotEmpty        uint8 = 8
	ErrorInvail          uint8 = 9
	ErrorInvailFD        uint8 = 10
	ErrorType            uint8 = 11
	ErrorIO              uint8 = 12
	ErrorNotSupport      uint8 = 13
	ErrorAccess          uint8 = 14
	ErrorTooLoong        uint8 = 15
)

const (
	O_RDONLY    uint32 = 0x0
	O_WRONLY    uint32 = 0x1
	O_RDWR      uint32 = 0x2
	O_TRUNC     uint32 = 0x200
	O_EXCL      uint32 = 0x80
	O_CREAT     uint32 = 0x40
	O_DIRECTORY uint32 = 0x10000
	O_APPEND    uint32 = 0x400
)

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
	SEEK_SET  uint8 = 0
	SEEK_CUR  uint8 = 1
	SEEK_END  uint8 = 2
	SEEK_DATA uint8 = 3
	SEEK_HOLE uint8 = 4
)

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
	TREEDIR_STATUS_OK                uint8 = 0
	TREEDIR_STATUS_OK_WITH_DATA      uint8 = 1
	TREEDIR_STATUS_ENTER_DIR         uint8 = 2
	TREEDIR_STATUS_END_DIR           uint8 = 3
	TREEDIR_STATUS_END_DIR_WITH_FAIL uint8 = 4
)

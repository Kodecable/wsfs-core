package config

type TLS struct {
	Enable   bool
	CertFile string
	KeyFile  string
}

type Webui struct {
	Enable          bool
	ShowDirSize     bool
	CustomResources string
}

type Webdav struct {
	Enable                 bool
	AllowPropfindInfDepth  bool
	EnableContentTypeProbe bool
	Webui                  Webui
}

type WSFS struct {
	Enable   bool
	Uid      int64
	Gid      int64
	OtherUid int64
	OtherGid int64
}

type Listener struct {
	Address string
	Network string
	TLS     TLS
}

type Server struct {
	filePath  string // internal, path to this config file
	Listener  Listener
	Webdav    Webdav
	WSFS      WSFS
	Storages  []Storage
	Anonymous AnonymousUser
	Users     []User
}

type User struct {
	Name       string
	SecretHash string
	ReadOnly   bool
	Storage    string
}

type AnonymousUser struct {
	Enable   bool
	ReadOnly bool
	Storage  string
}

type Storage struct {
	Id       string
	Path     string
	ReadOnly bool
}

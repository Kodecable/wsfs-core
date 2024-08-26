package config

type TLS struct {
	Enable   bool
	CertFile string
	KeyFile  string
}

type Webui struct {
	Enable bool
	//CustomResourcesPath string
	//CustomTemplatesPath string
	ShowDirSize bool
}

type Webdav struct {
	Enable                 bool
	AllowPropfindInfDepth  bool
	EnableContentTypeProbe bool
	Webui                  Webui
}

type Listener struct {
	Address string
	Network string
}

type Server struct {
	filePath  string // internal, path to this config file
	Listener  Listener
	TLS       TLS
	Webdav    Webdav
	Anonymous AnonymousUser
	Users     []User
	Storages  []Storage
	Uid       int64
	Gid       int64
	OtherUid  int64
	OtherGid  int64
}

type User struct {
	Name       string
	SecretHash string
	Storage    string
}

type AnonymousUser struct {
	Enable  bool
	Storage string
}

type Storage struct {
	Id   string
	Path string
}

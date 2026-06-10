package config

var Default = Server{
	filePath: "",
	Listener: Listener{
		Address: ":20001",
		Network: "tcp",
		TLS: TLS{
			Enable:   false,
			CertFile: "/srv/ssl/cert",
			KeyFile:  "/srv/ssl/key",
		},
	},
	Webdav: Webdav{
		Enable:                 true,
		EnableContentTypeProbe: false,
		AllowPropfindInfDepth:  false,
		Webui: Webui{
			Enable:      true,
			ShowDirSize: false,
		},
	},
	WSFS: WSFS{
		Enable: true,
	},
	Anonymous: AnonymousUser{
		Enable:   false,
		ReadOnly: true,
	},
	Users:    []User{},
	Storages: []Storage{},
}

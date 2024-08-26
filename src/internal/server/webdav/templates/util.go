package templates

import (
	"mime"
	"path/filepath"
)

func fileDisplayName(name string) string {
	//if name == "" || name[0] != '/' {
	//	name = "/" + name
	//}
	//name = path.Clean(name)

	if name == "/" {
		// Hide the real name of a possibly prefixed root directory.
		return ""
	}
	return name
}

func fileContentType(name string) string {
	ctype := mime.TypeByExtension(filepath.Ext(name))
	if ctype != "" {
		return ctype
	}
	return "application/octet-stream"
}

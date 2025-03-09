package templates

import (
	"mime"
	"path/filepath"
)

func fileDisplayName(name string) string {
	return filepath.Base(name)
}

func fileContentType(name string) string {
	ctype := mime.TypeByExtension(filepath.Ext(name))
	if ctype != "" {
		return ctype
	}
	return "application/octet-stream"
}

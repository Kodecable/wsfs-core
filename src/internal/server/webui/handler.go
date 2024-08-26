package webui

import (
	"embed"
	"net/http"
	"os"
	"strings"
	"wsfs-core/internal/server/config"
	"wsfs-core/internal/server/storage"
	"wsfs-core/internal/server/webui/templates"
)

type Handler struct {
	Enable              bool
	customResourcesPath string
	cacheId             string
	showDirSize         bool
}

//go:embed resources/*
var builtinResources embed.FS

func NewHandler(c *config.Webui, cacheId string) (h Handler, err error) {
	h.Enable = c.Enable
	if !h.Enable {
		return
	}
	//h.customResourcesPath = c.CustomResourcesPath
	h.cacheId = cacheId
	h.showDirSize = c.ShowDirSize

	return
}

func (w *Handler) ServeList(rsp http.ResponseWriter, req *http.Request, st *storage.Storage) {
	if !w.Enable {
		rsp.WriteHeader(http.StatusNotImplemented)
		return
	}

	rsp.Header().Set("Content-Type", "text/html")
	rsp.Header().Set("Accept-Ranges", "none")

	arg, err := list(req.URL.Path, st)
	if err != nil {
		if os.IsNotExist(err) {
			w.ServeError(rsp, req, http.StatusNotFound, "Not found")
		} else if os.IsPermission(err) {
			w.ServeError(rsp, req, http.StatusForbidden, "Access denied")
		} else {
			w.ServeError(rsp, req, http.StatusInternalServerError, "System busy")
		}
		return
	}

	rsp.WriteHeader(http.StatusOK)
	templates.WriteList(rsp, w.cacheId, arg.Paths, arg.Files, w.showDirSize)
}

func (w *Handler) ServeAssets(rsp http.ResponseWriter, req *http.Request) {
	if !w.Enable {
		rsp.WriteHeader(http.StatusNotImplemented)
		return
	}

	if _, err := os.Stat(w.customResourcesPath + req.URL.Path); err == nil {
		http.ServeFile(rsp, req, w.customResourcesPath+req.URL.Path)
	} else {
		if strings.HasPrefix(req.URL.Path, "/js/") ||
			strings.HasPrefix(req.URL.Path, "/css/") ||
			strings.HasPrefix(req.URL.Path, "/img/") {
			http.ServeFileFS(rsp, req, builtinResources, "resources"+req.URL.Path)
		} else {
			w.ServeError(rsp, req, http.StatusNotFound, "Not found")
		}
	}
}

func (w *Handler) ServeError(rsp http.ResponseWriter, _ *http.Request, status int, msg string) {
	rsp.WriteHeader(status)
	templates.WriteError(rsp, w.cacheId, status, msg)
}

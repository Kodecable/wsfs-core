package webui

import (
	"embed"
	"errors"
	"net/http"
	"os"
	"path"
	"strings"
	"wsfs-core/internal/server/config"
	internalerror "wsfs-core/internal/server/internalError"
	"wsfs-core/internal/server/storage"
	"wsfs-core/internal/server/webui/templates"
)

//go:embed resources/*
var builtinResources embed.FS

type Handler struct {
	customResources string
	cacheId         string
	showDirSize     bool
	customCSS       bool
	customJS        bool
}

func NewHandler(c config.Webui, cacheId string) (h *Handler, err error) {
	h = &Handler{
		cacheId:         cacheId,
		showDirSize:     c.ShowDirSize,
		customResources: c.CustomResources,
	}

	if h.customResources != "" {
		if _, err := os.Stat(path.Join(h.customResources, "custom.css")); err == nil {
			h.customCSS = true
		}
		if _, err := os.Stat(path.Join(h.customResources, "custom.js")); err == nil {
			h.customJS = true
		}
	}

	return
}

func (w *Handler) ServeList(rsp http.ResponseWriter, req *http.Request, user *storage.User) {
	rsp.Header().Set("Content-Type", "text/html")
	rsp.Header().Set("Accept-Ranges", "none")

	arg, err := list(req.URL.Path, user.Storage)
	if err != nil {
		if os.IsNotExist(err) {
			err = internalerror.ErrInternalNotFound
		} else if os.IsPermission(err) {
			err = internalerror.ErrInternalForbidden
		} else {
			err = internalerror.Warp(err)
		}
		w.ServeError(rsp, req, err)
		return
	}

	rsp.WriteHeader(http.StatusOK)
	templates.WriteList(rsp, w.cacheId, arg.Paths, arg.Files, w.showDirSize, w.customCSS, w.customJS, user.ReadOnly)
}

func (w *Handler) ServeAssets(rsp http.ResponseWriter, req *http.Request) {
	if strings.HasPrefix(req.URL.Path, "/js/") ||
		strings.HasPrefix(req.URL.Path, "/css/") ||
		strings.HasPrefix(req.URL.Path, "/img/") {
		http.ServeFileFS(rsp, req, builtinResources, "resources"+req.URL.Path)
		return
	}

	if strings.HasPrefix(req.URL.Path, "/custom/") {
		http.ServeFile(rsp, req, path.Join(w.customResources, req.URL.Path[7:]))
		return
	}

	w.ServeError(rsp, req, internalerror.ErrInternalNotFound)
}

func (w *Handler) ServeErrorPage(rsp http.ResponseWriter, _ *http.Request, status int, msg string) {
	rsp.WriteHeader(status)
	templates.WriteError(rsp, w.cacheId, status, msg, w.customCSS, w.customJS)
}

func (w *Handler) ServeError(rsp http.ResponseWriter, req *http.Request, err error) {
	if errors.Is(err, internalerror.ErrInternalNotFound) {
		w.ServeErrorPage(rsp, req, http.StatusNotFound, "Not found")
	} else if errors.Is(err, internalerror.ErrInternalForbidden) {
		w.ServeErrorPage(rsp, req, http.StatusForbidden, "Forbidden")
	} else {
		w.ServeErrorPage(rsp, req, http.StatusInternalServerError, err.Error())
	}
}

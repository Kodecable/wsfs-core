package webdav

import (
	"errors"
	"io"
	"net/http"
	"os"
	"wsfs-core/internal/server/config"
	internalerror "wsfs-core/internal/server/internalError"
	"wsfs-core/internal/server/storage"
	"wsfs-core/internal/server/webdav/templates"

	"github.com/rs/zerolog/log"
)

var (
	errNoDestinationHeader      = errors.New("webdav: no destination header")
	errDestinationHostDifferent = errors.New("webdav: destination host different")
	errDestinationEqualsSource  = errors.New("webdav: destination equals source")
	errInvalidDepth             = errors.New("webdav: invalid depth")
	errInvalidDestination       = errors.New("webdav: invalid destination")
	errRecursionTooDeep         = errors.New("webdav: recursion too deep")
)

const (
	recursionMax = 256

	preconditionPropfindFiniteDepth = "propfind-finite-depth"
)

type Handler struct {
	enableWebui            bool
	allowPropfindInfDepth  bool
	enableContentTypeProbe bool
	errorHandler           internalerror.ErrorHandler
}

func NewHandler(c config.Webdav, errorHandler internalerror.ErrorHandler) (h *Handler, err error) {
	h = &Handler{
		enableWebui:            c.Webui.Enable,
		allowPropfindInfDepth:  c.AllowPropfindInfDepth,
		enableContentTypeProbe: c.EnableContentTypeProbe,
		errorHandler:           errorHandler,
	}
	return
}

func (h *Handler) ServeHTTP(rsp http.ResponseWriter, req *http.Request, user *storage.User) {
	status := http.StatusNotImplemented
	var err error

	if user.ReadOnly {
		switch req.Method {
		case "OPTIONS":
			status, err = h.handleOptions(rsp, req, user)
		case "GET", "HEAD":
			status, err = h.handleGetHead(rsp, req, user.Storage)
		case "PROPFIND":
			status, err = h.handlePropfind(rsp, req, user.Storage)
		case "PROPPATCH", "COPY", "MOVE", "MKCOL", "PATCH", "PUT", "DELETE":
			status = http.StatusForbidden
		}
	} else {
		switch req.Method {
		case "OPTIONS":
			status, err = h.handleOptions(rsp, req, user)
		case "GET", "HEAD":
			status, err = h.handleGetHead(rsp, req, user.Storage)
		case "DELETE":
			status, err = h.handleDelete(rsp, req, user.Storage)
		case "PUT":
			status, err = h.handlePut(rsp, req, user.Storage)
		case "PATCH":
			status, err = h.handlePatch(rsp, req, user.Storage)
		case "MKCOL":
			status, err = h.handleMkcol(rsp, req, user.Storage)
		case "COPY", "MOVE":
			status, err = h.handleCopyMove(rsp, req, user.Storage)
		case "PROPFIND":
			status, err = h.handlePropfind(rsp, req, user.Storage)
		case "PROPPATCH":
			status, err = h.handlePropfind(rsp, req, user.Storage)
		}
	}

	if err != nil {
		log.Warn().Err(err).Msg("webdav error")
	}

	if status != 0 {
		rsp.WriteHeader(status)
		//if status != http.StatusNoContent {
		//	w.Write([]byte(StatusText(status)))
		//}
	}
}

func (h *Handler) handleOptions(rsp http.ResponseWriter, req *http.Request, user *storage.User) (status int, err error) {
	st := user.Storage
	path := st.Path + req.URL.Path

	var allow string
	if fi, err := os.Stat(path); err == nil {
		if fi.IsDir() {
			allow = "OPTIONS, PROPFIND"
			if !user.ReadOnly {
				allow += ", DELETE, PROPPATCH, COPY, MOVE"
			}
			if h.enableWebui {
				allow += ", GET"
			}
		} else {
			allow = "OPTIONS, GET, HEAD, PROPFIND"
			if !user.ReadOnly {
				allow += ", DELETE, PROPPATCH, COPY, MOVE, PUT, PATCH"
			}
		}
	} else if os.IsNotExist(err) {
		allow = "OPTIONS"
		if !user.ReadOnly {
			allow += ", PUT, MKCOL"
		}
	} else if os.IsPermission(err) {
		return http.StatusForbidden, nil
	} else {
		return http.StatusInternalServerError, err
	}

	rsp.Header().Set("Allow", allow)
	// http://www.webdav.org/specs/rfc4918.html#dav.compliance.classes
	// https://sabre.io/dav/http-patch/
	rsp.Header().Set("DAV", "1, sabredav-partialupdate")
	// http://msdn.microsoft.com/en-au/library/cc250217.aspx
	rsp.Header().Set("MS-Author-Via", "DAV")
	return http.StatusOK, nil
}

func (h *Handler) handleGetHead(rsp http.ResponseWriter, req *http.Request, st *storage.Storage) (status int, err error) {
	path := st.Path + req.URL.Path

	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		// for show error through webui
		if os.IsNotExist(err) {
			h.errorHandler.ServeError(rsp, req, internalerror.ErrInternalNotFound)
		} else if os.IsPermission(err) {
			h.errorHandler.ServeError(rsp, req, internalerror.ErrInternalForbidden)
		} else {
			h.errorHandler.ServeError(rsp, req, internalerror.Warp(err))
		}
		return 0, nil
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if fi.IsDir() {
		return http.StatusMethodNotAllowed, nil
	}

	if !h.enableContentTypeProbe {
		rsp.Header().Set("Content-Type", "application/octet-stream")
	}
	// else: Let http.ServeContent probe the Content-Type header

	// ServeContent will deal HEAD normatively
	http.ServeContent(rsp, req, path, fi.ModTime(), f)
	return 0, nil
}

func (h *Handler) handleDelete(_ http.ResponseWriter, req *http.Request, st *storage.Storage) (status int, err error) {
	path := st.Path + req.URL.Path

	// TODO: return MultiStatus where appropriate.

	// "godoc os RemoveAll" says that "If the path does not exist, RemoveAll
	// returns nil (no error)." WebDAV semantics are that it should return a
	// "404 Not Found". We therefore have to Stat before we RemoveAll.
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return http.StatusNotFound, nil
		} else if os.IsPermission(err) {
			return http.StatusForbidden, nil
		}
		return http.StatusInternalServerError, err
	}
	if err := os.RemoveAll(path); err != nil {
		if os.IsPermission(err) {
			return http.StatusForbidden, nil
		}
		return http.StatusInternalServerError, err
	}
	return http.StatusNoContent, nil
}

func (h *Handler) handlePut(_ http.ResponseWriter, req *http.Request, st *storage.Storage) (status int, err error) {
	path := st.Path + req.URL.Path

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		if os.IsPermission(err) {
			return http.StatusForbidden, nil
		} else {
			// Section 9.7.2 does not define the behavior of 'PUT'ing an
			// existing directory. In most operating systems the 'OpenFile'
			// will fail and reach here
			return http.StatusMethodNotAllowed, nil
		}
	}
	_, copyErr := io.Copy(f, req.Body)

	closeErr := f.Close()
	if copyErr != nil {
		return http.StatusInternalServerError, copyErr
	}
	if closeErr != nil {
		return http.StatusInternalServerError, closeErr
	}

	return http.StatusCreated, nil
}

// follow https://sabre.io/dav/http-patch/
func (h *Handler) handlePatch(_ http.ResponseWriter, req *http.Request, st *storage.Storage) (status int, err error) {
	path := st.Path + req.URL.Path

	if req.Header.Get("Content-Type") != "application/x-sabredav-partialupdate" {
		return http.StatusUnsupportedMediaType, nil
	}

	start, end, err := parseRange(req.Header.Get("X-Update-Range"))
	if err != nil {
		return http.StatusRequestedRangeNotSatisfiable, nil
	}

	f, err := os.OpenFile(path, os.O_WRONLY, 0666)
	if err != nil {
		// Note: sabre/dav doesn't require return what in this case
		if os.IsPermission(err) {
			return http.StatusForbidden, nil
		} else {
			return http.StatusMethodNotAllowed, nil
		}
	}

	if end < 0 {
		f.Seek(0, io.SeekEnd)
	} else if start < 0 {
		f.Seek(start, io.SeekEnd)
	} else {
		f.Seek(start, io.SeekStart)
	}

	_, copyErr := io.Copy(f, req.Body)

	closeErr := f.Close()
	if copyErr != nil {
		return http.StatusInternalServerError, copyErr
	}
	if closeErr != nil {
		return http.StatusInternalServerError, closeErr
	}

	return http.StatusOK, nil
}

func (h *Handler) handleMkcol(_ http.ResponseWriter, req *http.Request, st *storage.Storage) (status int, err error) {
	path := st.Path + req.URL.Path

	if req.ContentLength > 0 {
		return http.StatusUnsupportedMediaType, nil
	}
	if err := os.Mkdir(path, 0777); err != nil {
		if os.IsPermission(err) {
			return http.StatusForbidden, nil
		} else if os.IsNotExist(err) {
			return http.StatusConflict, err
		}
		return http.StatusMethodNotAllowed, err
	}
	return http.StatusCreated, nil
}

func (h *Handler) handleCopyMove(_ http.ResponseWriter, req *http.Request, st *storage.Storage) (status int, err error) {
	var src, dst string
	src, dst, err = getPathAndDest(st, req)

	switch err {
	case nil:
		// pass
	case errNoDestinationHeader, errInvalidDestination, errDestinationEqualsSource:
		return http.StatusBadRequest, nil
	case errDestinationHostDifferent:
		// Copy to another host? It may be abused. We currently do not support this.
		return http.StatusBadGateway, nil
	default:
		return http.StatusInternalServerError, err
	}

	if dst == src {
		// Section 9.8.5 says that "403 (Forbidden) - The operation is forbidden. A
		// special case for COPY could be that the source and destination resources
		// are the same resource."
		return http.StatusForbidden, nil
	}

	if req.Method == "COPY" {
		// Section 9.8.3 says that "The COPY method on a collection without a Depth
		// header must act as if a Depth header with value "infinity" was included".
		depth := infiniteDepth
		if hdr := req.Header.Get("Depth"); hdr != "" {
			depth = parseDepth(hdr)
			if depth != 0 && depth != infiniteDepth {
				// Section 9.8.3 says that "A client may submit a Depth header on a
				// COPY on a collection with a value of "0" or "infinity"."
				return http.StatusBadRequest, errInvalidDepth
			}
		}
		return copyFiles(src, dst, req.Header.Get("Overwrite") != "F", depth, 0)
	}

	// Section 9.9.2 says that "The MOVE method on a collection must act as if
	// a "Depth: infinity" header was used on it. A client must not submit a
	// Depth header on a MOVE on a collection with any value but "infinity"."
	if hdr := req.Header.Get("Depth"); hdr != "" {
		if parseDepth(hdr) != infiniteDepth {
			return http.StatusBadRequest, errInvalidDepth
		}
	}
	return moveFiles(src, dst, req.Header.Get("Overwrite") == "T")
}

func (h *Handler) handlePropfind(rsp http.ResponseWriter, req *http.Request, st *storage.Storage) (status int, err error) {
	path := st.Path + req.URL.Path

	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return http.StatusNotFound, err
		}
		return http.StatusMethodNotAllowed, err
	}
	depth := infiniteDepth
	if hdr := req.Header.Get("Depth"); hdr != "" {
		depth = parseDepth(hdr)
		if depth == invalidDepth {
			return http.StatusBadRequest, errInvalidDepth
		}
	}
	if depth == infiniteDepth && !h.allowPropfindInfDepth {
		preconditionErrorReponse(rsp, preconditionPropfindFiniteDepth, req.URL.Path, http.StatusForbidden)
		return
	}

	rsp.WriteHeader(http.StatusMultiStatus)
	templates.WritePropfindBegin(rsp)
	walkErr := walkFS(depth, st.Path, req.URL.Path, fi, func(reqPath string, info os.FileInfo, err error) error {
		//log.Debug().Str("obj", reqPath).Msg("walk fn")
		if err != nil {
			if os.IsNotExist(err) {
				templates.WritePropfindItemBadResponse(rsp, reqPath, "HTTP/1.1 404 Not Found")
			} else if os.IsPermission(err) {
				templates.WritePropfindItemBadResponse(rsp, reqPath, "HTTP/1.1 403 Forbidden")
			} else {
				templates.WritePropfindItemBadResponse(rsp, reqPath, "HTTP/1.1 500 Internal Server Error")
				log.Warn().Err(err).Str("Path", reqPath).Msg("Walk error")
			}
			return nil
		}
		templates.WritePropfindItemOKResponse(rsp, reqPath, info, h.enableContentTypeProbe)
		return nil
	})
	templates.WritePropfindEnd(rsp)

	if walkErr != nil {
		return http.StatusInternalServerError, walkErr
	}
	return 0, nil
}

func preconditionErrorReponse(rsp http.ResponseWriter, precondition, href string, code int) {
	rsp.Header().Set("Content-Type", "application/xml; charset=\"utf-8\"")
	rsp.WriteHeader(code)
	rsp.Write([]byte(preconditionErrorBody(precondition, href)))
}

func preconditionErrorBody(precondition, href string) string {
	return `<?xml version="1.0" encoding="utf-8" ?>
<D:error xmlns:D="DAV:">
	<D:` + precondition + `><D:href>` + href + `</D:href></D:` + precondition + `>
</D:error>
`
}

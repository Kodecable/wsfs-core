package webdav

import (
	"net/http"
	"net/url"
	"wsfs-core/internal/server/storage"
	"wsfs-core/internal/util"
)

func getPathAndDest(st *storage.Storage, req *http.Request) (path string, dest string, err error) {
	path = st.Path + req.URL.Path

	desthdr := req.Header.Get("Destination")
	if desthdr == "" {
		err = errNoDestinationHeader
		return
	}

	var desturl *url.URL
	desturl, err = url.Parse(desthdr)
	if err != nil {
		return
	}

	if desturl.Host != "" && req.Host != "" && desturl.Host != req.Host {
		err = errDestinationHostDifferent
		return
	}

	if !util.IsUrlValid(desturl.Path) {
		err = errInvalidDestination
		return
	}

	dest = st.Path + desturl.Path
	return
}

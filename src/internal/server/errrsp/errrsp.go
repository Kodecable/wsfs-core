package errrsp

import (
	"fmt"
	"net/http"
	"wsfs-core/buildinfo"
	"wsfs-core/internal/util"
)

func InternalServerError(fun util.ErrorPageFunc, rsp http.ResponseWriter, req *http.Request, err any) {
	if buildinfo.IsDebug() {
		fun(rsp, req, http.StatusInternalServerError, fmt.Sprint(err))
	} else {
		fun(rsp, req, http.StatusInternalServerError, "System busy")
	}
}

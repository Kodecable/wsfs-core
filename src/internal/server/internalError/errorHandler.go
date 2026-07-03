package internalerror

import (
	"errors"
	"net/http"
)

// There are fake errors for ErrorHandler to identify specific situation
var (
	ErrInternalNotFound  = errors.New("wsfs-core.internalError: not found")
	ErrInternalForbidden = errors.New("wsfs-core.internalError: forbidden")
)

type ErrorHandler interface {
	ServeError(http.ResponseWriter, *http.Request, error)
	ServeErrorMessage(http.ResponseWriter, *http.Request, int, string)
	ServeErrorPage(http.ResponseWriter, *http.Request, int, string)
}

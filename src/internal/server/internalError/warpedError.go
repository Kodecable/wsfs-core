package internalerror

import (
	"fmt"
	"wsfs-core/version"
)

// WarpedError be used in some http response.
// It will hide specific error msg in release mode to client for security.
type WarpedError struct {
	origin error
}

func (w *WarpedError) Error() string {
	if version.IsDebug() {
		return w.origin.Error()
	} else {
		return "System Busy"
	}
}

func (w *WarpedError) Unwarp() error {
	return w.origin
}

func Warp(obj any) error {
	if err, ok := obj.(error); ok {
		return &WarpedError{err}
	} else {
		return &WarpedError{fmt.Errorf("wsfs-core.interlnalError: Unknown error: %v", obj)}
	}
}

package internalerror

import (
	"fmt"
	"wsfs-core/version"
)

// WrappedError be used in some http response.
// It will hide specific error msg in release mode to client for security.
type WrappedError struct {
	origin error
}

func (w *WrappedError) Error() string {
	if version.IsDebug() {
		return w.origin.Error()
	} else {
		return "System Busy"
	}
}

func (w *WrappedError) Unwrap() error {
	return w.origin
}

func Wrap(obj any) error {
	if err, ok := obj.(error); ok {
		return &WrappedError{err}
	} else {
		return &WrappedError{fmt.Errorf("wsfs-core.internalError: Unknown error: %v", obj)}
	}
}

//go:build !unix

package util

func SetupSignalHandlers(h SignalHandlers) {
	// do nothing
}

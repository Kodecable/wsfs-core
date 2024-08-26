//go:build !unix

package util

func SetupSignalHandler(sighupHandler func(), sigintHandler func(), sigtermHandler func()) {
	// do nothing
}

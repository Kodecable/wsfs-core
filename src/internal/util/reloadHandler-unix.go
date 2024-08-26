//go:build unix

package util

import (
	"os"
	"os/signal"
	"syscall"
)

func SetupSignalHandler(sighupHandler func(), sigintHandler func(), sigtermHandler func()) {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGHUP)
	go func() {
		for {
			switch s := <-sigc; s {
			case syscall.SIGHUP:
				go sighupHandler()
			case syscall.SIGINT:
				go sigintHandler()
			case syscall.SIGTERM:
				go sigtermHandler()
			}
		}
	}()
}

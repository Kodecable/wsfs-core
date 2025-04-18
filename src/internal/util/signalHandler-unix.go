//go:build unix

package util

import (
	"os"
	"os/signal"
	"syscall"
)

func SetupSignalHandlers(h SignalHandlers) {
	sigch := make(chan os.Signal, 3)
	sigs := []os.Signal{}
	if h.Sighup != nil {
		sigs = append(sigs, syscall.SIGHUP)
	}
	if h.Sigint != nil {
		sigs = append(sigs, syscall.SIGINT)
	}
	if h.Sigterm != nil {
		sigs = append(sigs, syscall.SIGTERM)
	}
	signal.Notify(sigch, sigs...)

	go func() {
		for {
			switch s := <-sigch; s {
			case syscall.SIGHUP:
				go tryCall(h.Sighup, h.OnHandlerPanic)
			case syscall.SIGINT:
				go tryCall(h.Sighup, h.OnHandlerPanic)
			case syscall.SIGTERM:
				go tryCall(h.Sigterm, h.OnHandlerPanic)
			}
		}
	}()
}

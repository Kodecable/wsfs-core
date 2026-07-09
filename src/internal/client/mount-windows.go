//go:build windows

package client

import (
	"os"
	"os/signal"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
	"wsfs-core/internal/client/session"
	"wsfs-core/internal/client/windows"
	"wsfs-core/version"

	"github.com/rs/zerolog/log"
	"github.com/winfsp/cgofuse/fuse"
)

func fuseMount(mountpoint string, session *session.Session, opt MountOption) error {
	destroyed := make(chan struct{})
	shutdownRequested := make(chan struct{})
	var destroyOnce sync.Once
	var shutdownOnce sync.Once
	var exitCode atomic.Int32

	fs := windows.NewFS(session, mountpoint, opt.EntryTimeout, func() {
		destroyOnce.Do(func() {
			close(destroyed)
		})
	})
	host := fuse.NewFileSystemHost(fs)

	// winfsp-fuse opts
	// https://github.com/winfsp/winfsp/blob/2bf9a6c16e3bba46be6ec4ade6d7c70a262d27da/src/dll/fuse/fuse.c#L628
	opts := []string{"-o", "uid=-1,gid=-1", "-o", "dothidden"}

	if version.IsDebug() {
		opts = append(opts, "-d")
	}

	if opt.MasqueradeAsNtfs {
		opts = append(opts, "-o")
		opts = append(opts, "ExactFileSystemName=NTFS")
	} else {
		opts = append(opts, "-o")
		opts = append(opts, "FileSystemName=WSFS")
	}

	opts = append(opts, "-o")
	opts = append(opts, "volname="+opt.VolumeLabel)

	// we only leave 10s for fuse cache, other by us.
	opts = append(opts, "-o")
	opts = append(opts, "FileInfoTimeout="+strconv.FormatInt(min(int64(opt.AttrTimeout/time.Second), 10), 10))
	opts = append(opts, "-o")
	opts = append(opts, "DirInfoTimeout="+strconv.FormatInt(min(int64(opt.EntryTimeout/time.Second), 10), 10))

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt)
	defer signal.Stop(sigc)

	go func() {
		<-sigc
		shutdownOnce.Do(func() {
			close(shutdownRequested)
		})
		host.Unmount()
	}()

	go func() {
		err := session.Wait()
		if err != nil {
			log.Error().Err(err).Msg("Session exit with error")
			exitCode.Store(1)
		}
		host.Unmount()
	}()

	host.SetCapCaseInsensitive(false)
	host.SetCapDeleteAccess(true)
	host.SetCapOpenTrunc(true)
	host.SetCapReaddirPlus(true)
	if !host.Mount(mountpoint, opts) {
		destroyOnce.Do(func() {
			close(destroyed)
		})
	}
	<-destroyed

	select {
	case <-shutdownRequested:
		if err := session.Close(); err != nil {
			return err
		}
	default:
	}
	os.Exit(int(exitCode.Load()))
	return nil
}

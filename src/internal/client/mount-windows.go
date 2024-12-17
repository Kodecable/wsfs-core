//go:build windows

package client

import (
	"os"
	"strconv"
	"wsfs-core/buildinfo"
	"wsfs-core/internal/client/session"
	"wsfs-core/internal/client/windows"

	"github.com/rs/zerolog/log"
	"github.com/winfsp/cgofuse/fuse"
)

func fuseMount(mountpoint string, session *session.Session, opt MountOption) error {
	exitCode := 0

	fs := windows.NewFS(session, mountpoint, func() { os.Exit(exitCode) })
	host := fuse.NewFileSystemHost(fs)

	// winfsp-fuse opts
	// https://github.com/winfsp/winfsp/blob/2bf9a6c16e3bba46be6ec4ade6d7c70a262d27da/src/dll/fuse/fuse.c#L628
	opts := []string{"-o", "uid=-1,gid=-1", "-o", "dothidden"}

	if buildinfo.IsDebug() {
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

	opts = append(opts, "-o")
	opts = append(opts, "FileInfoTimeout="+strconv.FormatInt(opt.AttrTimeout.Milliseconds(), 10))
	opts = append(opts, "-o")
	opts = append(opts, "DirInfoTimeout="+strconv.FormatInt(opt.AttrTimeout.Milliseconds(), 10))

	go func() {
		err := session.Wait()
		if err != nil {
			log.Error().Err(err).Msg("Session exit with error")
			exitCode = 1
		}
		host.Unmount()
	}()

	host.Mount(mountpoint, opts)
	return nil
}

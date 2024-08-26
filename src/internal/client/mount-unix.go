//go:build unix

package client

import (
	"io"
	golanglog "log"
	"os"
	"os/signal"
	"syscall"
	"wsfs-core/buildinfo"
	"wsfs-core/internal/client/session"
	"wsfs-core/internal/client/unix"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/rs/zerolog/log"
)

func fuseMount(mountpoint string, session *session.Session, opt MountOption) error {
	fsroot := unix.NewRoot(session, unix.Suser_t{
		Uid: opt.Uid, Gid: opt.Gid, NobodyUid: opt.NobodyUid, NobodyGid: opt.NobodyGid})
	root := fsroot.NewNode()

	opts := &fs.Options{
		AttrTimeout:     &opt.AttrTimeout,
		EntryTimeout:    &opt.EntryTimeout,
		NullPermissions: true, // Leave file permissions on "000" files as-is
		MountOptions: fuse.MountOptions{
			AllowOther:        false,
			Debug:             buildinfo.IsDebug(),
			DirectMount:       !opt.UseFusemount,
			DirectMountStrict: !opt.UseFusemount,
			FsName:            opt.FuseFsName, // First column in "df -T": original dir
			Name:              "wsfs",         // Second column in "df -T" will be shown as "fuse." + Name
		},
	}
	if opt.EnableFuseLog {
		opts.Logger = golanglog.New(os.Stderr, "", 0)
	} else {
		opts.Logger = nil
		golanglog.SetFlags(0)
		golanglog.SetOutput(io.Discard)
	}

	server, err := fs.Mount(mountpoint, root, opts)
	if err != nil {
		return err
	}
	log.Warn().Str("Mountpoint", mountpoint).Msg("Mounted")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		server.Unmount()
	}()

	server.Wait()
	return nil
}

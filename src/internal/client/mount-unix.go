//go:build unix

package client

import (
	"io"
	golanglog "log"
	"os"
	"os/signal"
	"syscall"
	"wsfs-core/internal/client/session"
	"wsfs-core/internal/client/unix"
	"wsfs-core/version"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/rs/zerolog/log"
)

func fuseMount(mountpoint string, session *session.Session, opt MountOption) error {
	fsroot := unix.NewFS(session,
		opt.FsIds,
		mountpoint,
		opt.AttrTimeout,
		opt.NegativeTimeout,
		opt.FlockMode)
	root := fsroot.NewNode()

	opts := &fs.Options{
		AttrTimeout:     &opt.AttrTimeout,
		EntryTimeout:    &opt.EntryTimeout,
		NegativeTimeout: &opt.NegativeTimeout,
		NullPermissions: true, // Leave file permissions on "000" files as-is
		MountOptions: fuse.MountOptions{
			AllowOther:        false,
			Debug:             version.IsDebug(),
			DirectMount:       !opt.UseFusemount,
			DirectMountStrict: !opt.UseFusemount,
			EnableLocks:       true,
			FsName:            opt.FuseFsName, // First column in "df -T"
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
	if opt.ReportFuseConnection && opt.UseFusemount {
		connectionID, err := fuseConnectionID(mountpoint)
		if err != nil {
			log.Warn().Err(err).Str("Mountpoint", mountpoint).Msg("Unable to determine FUSE connection Id")
		} else {
			if connectionID != nil {
				log.Info().Str("Mountpoint", mountpoint).Uint64("Id", *connectionID).Msg("FUSE connection Id obtained")
			} else {
				// slient fail for platfrom not support
			}
		}
	}
	log.Warn().Str("Mountpoint", mountpoint).Msg("Mounted")
	return fuseMountWait(server, session)
}

func fuseMountWait(server *fuse.Server, session *session.Session) error {
	sigc := make(chan os.Signal, 1)
	shutdownRequested := make(chan struct{})
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		<-sigc
		close(shutdownRequested)
		server.Unmount()
	}()

	go func() {
		err := session.Wait()
		if err != nil {
			log.Error().Err(err).Msg("Session exit with error")
		}
		server.Unmount()
	}()

	server.Wait()

	select {
	case <-shutdownRequested:
		return session.Close()
	default:
	}
	return nil
}

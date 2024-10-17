//go:build windows || unix

package mount

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
	"wsfs-core/buildinfo"
	"wsfs-core/internal/client"
	"wsfs-core/internal/util"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/thediveo/enumflag"
)

var (
	volumeLabel       string
	masqueradeAsNtfs  bool
	fuseStructTimeout int16
	directMount       bool
	noLogTime         bool
	uid               int64
	gid               int64
	nobodyUid         int64
	nobodyGid         int64
	logLevel          zerolog.Level = zerolog.InfoLevel
)

var MountCmd = &cobra.Command{
	Use:   "mount ENDPOINT MOUNTPOINT",
	Short: "Mount a Websocket Filesystem",
	Args:  cobra.ExactArgs(2),
	Run: func(_ *cobra.Command, args []string) {
		setUids()
		util.SetupZerolog(noLogTime, logLevel)

		urlArg := args[0]
		if ok, _ := regexp.MatchString(`.*:?\/\/`, urlArg); !ok {
			urlArg = "//" + urlArg
		}

		inputedEndpoint, err := url.Parse(urlArg)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Parse endpoint url failed")
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		endpoint := url.URL{
			Host:        inputedEndpoint.Host,
			Path:        inputedEndpoint.Path,
			RawPath:     inputedEndpoint.RawPath,
			RawQuery:    inputedEndpoint.RawQuery,
			Fragment:    inputedEndpoint.RawFragment,
			RawFragment: inputedEndpoint.RawFragment,
		}
		passwd, _ := inputedEndpoint.User.Password()
		switch strings.ToLower(inputedEndpoint.Scheme) {
		case "wsfs", "http", "":
			endpoint.Scheme = "ws"
		case "wsfss", "https":
			endpoint.Scheme = "wss"
		case "ws", "wss", "unix":
			endpoint.Scheme = inputedEndpoint.Scheme
		default:
			fmt.Fprintln(os.Stderr, "Bad endpoint url: unknown scheme")
			fmt.Fprintln(os.Stderr, "Want: 'wsfs' or 'wsfss', have: "+endpoint.Scheme)
		}

		opts := client.MountOption{
			AttrTimeout:  time.Duration(fuseStructTimeout) * time.Second,
			EntryTimeout: time.Duration(fuseStructTimeout) * time.Second,
			//EnoentTimeout:    time.Duration(fuseStructTimeout) * time.Second,
			UseFusemount:     !directMount,
			VolumeLabel:      volumeLabel,
			MasqueradeAsNtfs: masqueradeAsNtfs,
			EnableFuseLog:    false,
			FuseFsName:       inputedEndpoint.Host,
			Uid:              uint32(uid),
			Gid:              uint32(gid),
			NobodyUid:        uint32(nobodyUid),
			NobodyGid:        uint32(nobodyGid),
		}
		if logLevel == zerolog.TraceLevel {
			opts.EnableFuseLog = true
		}

		err = client.Mount(args[1], endpoint.String(), inputedEndpoint.User.Username(), passwd, opts)

		if err != nil {
			os.Exit(2)
		}
	},
}

func setUids() {
	if uid >= 0 && gid >= 0 &&
		nobodyUid >= 0 && nobodyGid >= 0 {
		return
	}

	defaultIds, err := util.GetDefaultIds()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Unable to determine default (nobody) u/gids")
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if uid < 0 {
		uid = int64(defaultIds.CurrentUser)
	}
	if gid < 0 {
		gid = int64(defaultIds.UserGroup)
	}
	if nobodyUid < 0 {
		nobodyUid = int64(defaultIds.NobodyUser)
	}
	if nobodyGid < 0 {
		nobodyGid = int64(defaultIds.NobodyGroup)
	}
}

func init() {
	if buildinfo.IsDebug() {
		logLevel = zerolog.DebugLevel
	}
	MountCmd.Flags().Int64VarP(&uid, "uid", "", -1, "Uid in filesystem, use process uid when negtive (Unix only)")
	MountCmd.Flags().Int64VarP(&gid, "gid", "", -1, "Gid in filesystem, use process gid when negtive (Unix only)")
	MountCmd.Flags().Int64VarP(&nobodyUid, "nobody-uid", "", -1, "Nobody uid in filesystem, lookup nobody uid when negtive (Unix only)")
	MountCmd.Flags().Int64VarP(&nobodyGid, "nobody-gid", "", -1, "Nobody gid in filesystem, lookup nobody gid when negtive (Unix only)")
	MountCmd.Flags().StringVarP(&volumeLabel, "volume-label", "", "WSFS Storage", "Volume label (Windows only)")
	MountCmd.Flags().BoolVarP(&directMount, "direct-mount", "", false, "Use mount syscall instead fusemount, root needed (Unix only)")
	MountCmd.Flags().Int16VarP(&fuseStructTimeout, "fuse-struct-timeout", "", 1, "Fuse struct cache timeout in seconds, improves performance and inconsistency (Unix only)")
	MountCmd.Flags().BoolVarP(&masqueradeAsNtfs, "masquerade-as-ntfs", "", false, "Allow Windows to run executable as administrator (Windows only)")
	MountCmd.Flags().BoolVarP(&noLogTime, "no-log-time", "", false, "Use log format without time")
	MountCmd.Flags().VarP(
		enumflag.New(&logLevel, "LEVEL", util.ZerologLevelIds, enumflag.EnumCaseInsensitive),
		"level", "l",
		"Sets logging level; can be 'trace', 'debug', 'info', 'warning', 'error', 'fatal', 'panic'")
}

//go:build windows || unix

package mount

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
	"wsfs-core/internal/client"
	"wsfs-core/internal/util"
	"wsfs-core/version"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/thediveo/enumflag"
)

var (
	volumeLabel      string
	masqueradeAsNtfs bool
	structTimeout    int16
	directMount      bool
	noLogTime        bool
	uid              int64
	gid              int64
	nobodyUid        int64
	nobodyGid        int64
	logLevel         zerolog.Level = zerolog.InfoLevel
)

var MountCmd = &cobra.Command{
	Use:   "mount EndPoint MountPoint",
	Short: "Mount a Websocket Filesystem",
	Example: `  wsfs mount wsfs://localhost:20001/?wsfs /path/to/mountpoint
  wsfs mount wsfss+unix://hostname.sent.to.server/path/to/socket.sock/./?wsfs /path/to/mountpoint
  wsfs mount windows.mountponint.should.be?wsfs "P:"`,
	Args: cobra.ExactArgs(2),
	Run: func(c *cobra.Command, args []string) {
		setUids(c)
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
		case "unix":
			endpoint.Scheme = "unix+wsfs"
		case "ws", "wss", "unix+wsfs", "unix+wsfss":
			endpoint.Scheme = inputedEndpoint.Scheme
		default:
			fmt.Fprintln(os.Stderr, "Bad endpoint url: unknown scheme")
			fmt.Fprintln(os.Stderr, "Want: 'wsfs' or 'wsfss', have: "+endpoint.Scheme)
		}

		opts := client.MountOption{
			AttrTimeout:      time.Duration(structTimeout) * time.Second,
			EntryTimeout:     time.Duration(structTimeout) * time.Second,
			NegativeTimeout:  time.Duration(structTimeout) * time.Second,
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

func setUids(c *cobra.Command) {
	if (!c.Flags().Changed("uid")) ||
		(!c.Flags().Changed("git")) ||
		(!c.Flags().Changed("nobody-uid")) ||
		(!c.Flags().Changed("nobody-gid")) {
		defaultIds, err := util.GetDefaultIds()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Unable to determine default (nobody) u/gids")
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		if !c.Flags().Changed("uid") {
			uid = int64(defaultIds.CurrentUser)
		}
		if !c.Flags().Changed("gid") {
			gid = int64(defaultIds.UserGroup)
		}
		if !c.Flags().Changed("nobody-uid") {
			nobodyUid = int64(defaultIds.NobodyUser)
		}
		if !c.Flags().Changed("nobody-gid") {
			nobodyGid = int64(defaultIds.NobodyGroup)
		}
	}
}

func init() {
	if version.IsDebug() {
		logLevel = zerolog.DebugLevel // default debug level in debug mode
	}

	MountCmd.Flags().Int64VarP(&uid, "uid", "", 0, "Uid in filesystem (Unix only)")
	MountCmd.Flags().Int64VarP(&gid, "gid", "", 0, "Gid in filesystem (Unix only)")
	MountCmd.Flags().Int64VarP(&nobodyUid, "nobody-uid", "", 0, "Nobody uid in filesystem (Unix only)")
	MountCmd.Flags().Int64VarP(&nobodyGid, "nobody-gid", "", 0, "Nobody gid in filesystem (Unix only)")
	MountCmd.Flags().StringVarP(&volumeLabel, "volume-label", "", "WSFS Storage", "Volume label (Windows only)")
	MountCmd.Flags().BoolVarP(&directMount, "direct-mount", "", false, "Use mount syscall instead fusemount, root needed (Unix only)")
	MountCmd.Flags().Int16VarP(&structTimeout, "struct-timeout", "", 180, "Fuse struct cache timeout in seconds, improves performance and inconsistency")
	MountCmd.Flags().BoolVarP(&masqueradeAsNtfs, "masquerade-as-ntfs", "", false, "Allow Windows to run executable as administrator (Windows only)")
	MountCmd.Flags().BoolVarP(&noLogTime, "no-log-time", "", false, "Use log format without time")
	MountCmd.Flags().VarP(
		enumflag.New(&logLevel, "LEVEL", util.ZerologLevelIds, enumflag.EnumCaseInsensitive),
		"level", "l",
		"Sets logging level; can be 'trace', 'debug', 'info', 'warning', 'error', 'fatal', 'panic'")
}

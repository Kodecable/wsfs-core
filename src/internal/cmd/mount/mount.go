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
	cmdpassword "wsfs-core/internal/cmd/password"
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
	pingInterval     int16
	directMount      bool
	noLogTime        bool
	noLogColor       bool
	uid              int64
	gid              int64
	otherUid         int64
	otherGid         int64
	logLevel         zerolog.Level = zerolog.InfoLevel
	certHash         string
	passwordSource   string
)

var MountCmd = &cobra.Command{
	Use:   "mount EndPoint MountPoint",
	Short: "Mount a Websocket Filesystem",
	Example: `  wsfs mount wsfs://username:password@localhost:20001/ /path/to/mountpoint
  wsfs mount wsfss+unix://host.name/path/to/socket.sock/./server/path/?wsfs /path/to/mountpoint
  wsfs mount windows.mountpoint.like "P:"`,
	Args: cobra.ExactArgs(2),
	Run: func(c *cobra.Command, args []string) {
		if pingInterval != 0 && pingInterval < 10 {
			fmt.Fprintln(os.Stderr, "Bad ping interval: must be 0 or at least 10 seconds")
			os.Exit(2)
		}

		fsIds, err := resolveFsIds(c)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		util.SetupZerolog(noLogTime, noLogColor, logLevel)

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
		username := inputedEndpoint.User.Username()
		passwd, hasURLPassword := inputedEndpoint.User.Password()
		passwd, err = cmdpassword.Resolve(passwd, hasURLPassword, username != "", passwordSource, c.Flags().Changed("password"))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if username != "" && passwd == "" {
			fmt.Fprintln(os.Stderr, "Warning: password is empty")
		}
		switch strings.ToLower(inputedEndpoint.Scheme) {
		case "wsfs", "http", "":
			endpoint.Scheme = "ws"
		case "wsfss", "https":
			endpoint.Scheme = "wss"
		case "unix":
			endpoint.Scheme = "wsfs+unix"
		case "ws", "wss", "wsfs+unix", "wsfss+unix":
			endpoint.Scheme = inputedEndpoint.Scheme
		default:
			fmt.Fprintln(os.Stderr, "Bad endpoint url: unknown scheme")
			fmt.Fprintln(os.Stderr, "Want: 'wsfs' or 'wsfss', have: "+inputedEndpoint.Scheme)
			os.Exit(1)
		}

		opts := client.MountOption{
			AttrTimeout:      time.Duration(structTimeout) * time.Second,
			EntryTimeout:     time.Duration(structTimeout) * time.Second,
			NegativeTimeout:  time.Duration(structTimeout) * time.Second,
			PingInterval:     time.Duration(pingInterval) * time.Second,
			UseFusemount:     !directMount,
			VolumeLabel:      volumeLabel,
			MasqueradeAsNtfs: masqueradeAsNtfs,
			EnableFuseLog:    false,
			FuseFsName:       inputedEndpoint.Host,
			FsIds:            fsIds,
		}
		if logLevel == zerolog.TraceLevel {
			opts.EnableFuseLog = true
		}

		err = client.Mount(args[1], endpoint.String(), certHash, username, passwd, opts)

		if err != nil {
			os.Exit(2)
		}
	},
}

func resolveFsIds(c *cobra.Command) (util.FsIds, error) {
	ids := util.OptionalFsIds{}
	if c.Flags().Changed("uid") {
		value := uint32(uid)
		ids.Uid = &value
	}
	if c.Flags().Changed("gid") {
		value := uint32(gid)
		ids.Gid = &value
	}
	if c.Flags().Changed("other-uid") {
		value := uint32(otherUid)
		ids.OtherUid = &value
	}
	if c.Flags().Changed("other-gid") {
		value := uint32(otherGid)
		ids.OtherGid = &value
	}

	return ids.Resolve()
}

func init() {
	if version.IsDebug() {
		logLevel = zerolog.DebugLevel // default debug level in debug mode
	}

	MountCmd.Flags().Int64VarP(&uid, "uid", "", 0, "Uid in filesystem (Unix only)")
	MountCmd.Flags().Int64VarP(&gid, "gid", "", 0, "Gid in filesystem (Unix only)")
	MountCmd.Flags().Int64VarP(&otherUid, "other-uid", "", 0, "Other uid in filesystem (Unix only)")
	MountCmd.Flags().Int64VarP(&otherGid, "other-gid", "", 0, "Other gid in filesystem (Unix only)")
	MountCmd.Flags().StringVarP(&volumeLabel, "volume-label", "", "WSFS Storage", "Volume label (Windows only)")
	MountCmd.Flags().BoolVarP(&directMount, "direct-mount", "", false, "Use mount syscall instead fusemount, root needed (Unix only)")
	MountCmd.Flags().Int16VarP(&structTimeout, "struct-timeout", "", 60, "Fuse struct cache timeout in seconds, improves performance and inconsistency")
	MountCmd.Flags().Int16VarP(&pingInterval, "ping-interval", "", 60, "WebSocket ping interval in seconds; 0 disables client keepalive")
	MountCmd.Flags().BoolVarP(&masqueradeAsNtfs, "masquerade-as-ntfs", "", false, "Allow Windows to run executable as administrator (Windows only)")
	MountCmd.Flags().BoolVarP(&noLogTime, "no-log-time", "", false, "Use log format without time")
	MountCmd.Flags().BoolVarP(&noLogColor, "no-log-color", "", false, "Disable colors in log output")
	MountCmd.Flags().VarP(
		enumflag.New(&logLevel, "LEVEL", util.ZerologLevelIds, enumflag.EnumCaseInsensitive),
		"level", "l",
		"Sets logging level; can be 'trace', 'debug', 'info', 'warning', 'error', 'fatal', 'panic'")
	MountCmd.Flags().StringVarP(&certHash, "cert-hash", "", "", "Only verify TLS server cert hash; copy the hash from the connection log")
	MountCmd.Flags().StringVarP(&passwordSource, "password", "", "", cmdpassword.FlagUsage)
}

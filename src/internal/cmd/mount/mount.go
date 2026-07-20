//go:build windows || unix

package mount

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
	"wsfs-core/internal/client"
	clientSession "wsfs-core/internal/client/session"
	cmdexit "wsfs-core/internal/cmd/exit"
	cmdflags "wsfs-core/internal/cmd/flags"
	cmdpassword "wsfs-core/internal/cmd/password"
	"wsfs-core/internal/util"
	"wsfs-core/version"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/thediveo/enumflag"
)

var (
	volumeLabel        string
	masqueradeAsNtfs   bool
	structTimeout      int16
	pingInterval       int16
	directMount        bool
	noLogTime          bool
	noLogColor         bool
	jsonLog            bool
	uid                uint32
	gid                uint32
	otherUid           uint32
	otherGid           uint32
	logLevel           zerolog.Level = zerolog.InfoLevel
	certHash           string
	passwordSource     string
	flockMode          clientSession.FlockMode
	xattrPrefixes      []string
	disableXAttrAppend bool
)

var MountCmd = &cobra.Command{
	Use:   "mount EndPoint MountPoint",
	Short: "Mount a Websocket Filesystem",
	Example: `  wsfs mount wsfs://username:password@localhost:20001/ /path/to/mountpoint
  wsfs mount wsfss+unix://host.name/path/to/socket.sock/./server/path/?wsfs /path/to/mountpoint
  wsfs mount windows.mountpoint.like "P:"`,
	Args: cobra.ExactArgs(2),
	RunE: func(c *cobra.Command, args []string) error {
		if pingInterval != 0 && pingInterval < 10 {
			return cmdexit.New(2, errors.New("bad ping interval: must be 0 or at least 10 seconds"))
		}

		fsIds, err := resolveFsIds(c)
		if err != nil {
			return cmdexit.New(1, err)
		}
		util.SetupZerolog(noLogTime, noLogColor, jsonLog, logLevel)

		urlArg := args[0]
		if ok, _ := regexp.MatchString(`.*:?\/\/`, urlArg); !ok {
			urlArg = "//" + urlArg
		}

		inputedEndpoint, err := url.Parse(urlArg)
		if err != nil {
			return cmdexit.New(1, fmt.Errorf("parse endpoint URL failed: %w", err))
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
			return cmdexit.New(1, err)
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
			return cmdexit.New(1, fmt.Errorf("bad endpoint URL: unknown scheme; want 'wsfs' or 'wsfss', have: %s", inputedEndpoint.Scheme))
		}

		opts := client.MountOption{
			AttrTimeout:        time.Duration(structTimeout) * time.Second,
			EntryTimeout:       time.Duration(structTimeout) * time.Second,
			NegativeTimeout:    time.Duration(structTimeout) * time.Second,
			PingInterval:       time.Duration(pingInterval) * time.Second,
			UseFusemount:       !directMount,
			VolumeLabel:        volumeLabel,
			MasqueradeAsNtfs:   masqueradeAsNtfs,
			EnableFuseLog:      false,
			FuseFsName:         inputedEndpoint.Host,
			FsIds:              fsIds,
			FlockMode:          flockMode,
			AllowedXAttrPrefix: xattrPrefixes,
			DisableXAttrAppend: disableXAttrAppend,
		}
		if logLevel == zerolog.TraceLevel {
			opts.EnableFuseLog = true
		}

		err = client.Mount(args[1], endpoint.String(), certHash, username, passwd, opts)

		if err != nil {
			return cmdexit.New(2, fmt.Errorf("mount failed: %w", err))
		}
		return nil
	},
}

func resolveFsIds(c *cobra.Command) (util.FsIds, error) {
	ids := util.OptionalFsIds{}
	if c.Flags().Changed("uid") {
		ids.Uid = &uid
	}
	if c.Flags().Changed("gid") {
		ids.Gid = &gid
	}
	if c.Flags().Changed("other-uid") {
		ids.OtherUid = &otherUid
	}
	if c.Flags().Changed("other-gid") {
		ids.OtherGid = &otherGid
	}

	return ids.Resolve()
}

func init() {
	if version.IsDebug() {
		logLevel = zerolog.DebugLevel // default debug level in debug mode
	}

	cmdflags.AddFsIDFlags(MountCmd.Flags(), &uid, &gid, &otherUid, &otherGid)
	MountCmd.Flags().StringVar(&volumeLabel, "volume-label", "WSFS Storage", "Volume label (Windows only)")
	MountCmd.Flags().BoolVar(&directMount, "direct-mount", false, "Use mount syscall instead fusemount, root needed (Unix only)")
	MountCmd.Flags().Int16Var(&structTimeout, "struct-timeout", 60, "Fuse struct cache timeout in seconds, improves performance and inconsistency")
	MountCmd.Flags().Int16Var(&pingInterval, "ping-interval", 60, "WebSocket ping interval in seconds; 0 disables client keepalive")
	MountCmd.Flags().BoolVar(&masqueradeAsNtfs, "masquerade-as-ntfs", false, "Allow Windows to run executable as administrator (Windows only)")
	cmdflags.AddLoggingFlags(MountCmd.Flags(), &logLevel, &noLogTime, &noLogColor, &jsonLog)
	MountCmd.Flags().StringVar(&certHash, "cert-hash", "", "Only verify TLS server cert hash; copy the hash from the connection log")
	cmdflags.AddPasswordFlag(MountCmd.Flags(), &passwordSource)
	MountCmd.Flags().StringArrayVar(&xattrPrefixes, "xattr-prefix", nil, "Allow xattr names with this prefix; may be repeated")
	MountCmd.Flags().BoolVar(&disableXAttrAppend, "disable-xattr-append", false, "Return ERANGE instead of splitting oversized xattr writes")
	MountCmd.Flags().VarP(
		enumflag.New(&flockMode, "MODE", map[clientSession.FlockMode][]string{
			clientSession.FlockModeOFD:         {"ofd"},
			clientSession.FlockModeUnsupported: {"unsupported"},
			clientSession.FlockModeNoop:        {"noop"},
		}, enumflag.EnumCaseInsensitive),
		"flock", "",
		"BSD flock handling on Unix: 'ofd' maps to whole-file OFD locks, 'unsupported' returns ENOTSUP, 'noop' succeeds without locking")
}

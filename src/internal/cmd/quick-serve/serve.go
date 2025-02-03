package quickserve

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"wsfs-core/internal/server"
	"wsfs-core/internal/server/config"
	"wsfs-core/internal/util"
	"wsfs-core/version"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/thediveo/enumflag"
	"golang.org/x/crypto/bcrypt"
)

var cacheIdRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*")

var (
	uid      uint32
	gid      uint32
	otherUid uint32
	otherGid uint32
	storage  string
	logLevel zerolog.Level = zerolog.InfoLevel
)

func exitWithError(code int, msg string, err error) {
	fmt.Fprintln(os.Stderr, msg)
	fmt.Fprintln(os.Stderr, err)
	os.Exit(code)
}

const storageId = "main"

func configStorage(serverConfig *config.Server, c *cobra.Command) {
	if !c.Flags().Changed("storage") {
		fmt.Fprintln(os.Stdout, "Warning: use working directory as storage")
		workingDir, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Unable to determine working directory")
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		storage = workingDir
	}

	serverConfig.Storages = append(serverConfig.Storages, config.Storage{Id: storageId, Path: storage})
	serverConfig.Anonymous.Storage = storageId
}

func configIDs(serverConfig *config.Server, c *cobra.Command) {
	if (!c.Flags().Changed("uid")) ||
		(!c.Flags().Changed("gid")) ||
		(!c.Flags().Changed("other-uid")) ||
		(!c.Flags().Changed("other-gid")) {
		defaultIds, err := util.GetDefaultIds()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Unable to determine default (nobody) u/gids")
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		if !c.Flags().Changed("uid") {
			uid = defaultIds.CurrentUser
		}
		if !c.Flags().Changed("gid") {
			gid = defaultIds.UserGroup
		}
		if !c.Flags().Changed("other-uid") {
			otherUid = defaultIds.NobodyUser
		}
		if !c.Flags().Changed("other-gid") {
			otherGid = defaultIds.NobodyGroup
		}
	}

	serverConfig.Uid = int64(uid)
	serverConfig.Gid = int64(gid)
	serverConfig.OtherUid = int64(otherUid)
	serverConfig.Gid = int64(otherGid)
}

func parseArg(serverConfig *config.Server, args string) {
	arg := strings.TrimSpace(args)

	if _, err := strconv.ParseUint(arg, 10, 16); err == nil {
		serverConfig.Listener.Address = ":" + arg
	} else {
		if ok, _ := regexp.MatchString(`.*:?\/\/`, arg); !ok {
			arg = "//" + arg
		}

		parsedUrl, err := url.Parse(arg)
		if err != nil {
			exitWithError(2, "Parse url failed", err)
		}

		switch strings.ToLower(parsedUrl.Scheme) {
		case "http", "wsfs", "tcp", "":
			serverConfig.Listener.Network = "tcp"
		case "unix":
			serverConfig.Listener.Network = "unix"
		default:
			fmt.Fprintln(os.Stderr, "Unsupported listen network: '"+parsedUrl.Scheme+"'")
			os.Exit(2)
		}

		if username := parsedUrl.User.Username(); username != "" {
			var password string
			var hash []byte
			if password, _ = parsedUrl.User.Password(); password == "" {
				password = util.RandomString(10, cacheIdRunes)
				fmt.Fprintln(os.Stdout, "Password for user '"+username+"' is '"+password+"'")
			}
			if hash, err = bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost); err != nil {
				exitWithError(2, "Uable to generate password hash", err)
			}
			serverConfig.Users = append(serverConfig.Users, config.User{Name: username, SecretHash: string(hash), Storage: storageId})
		}

		if parsedUrl.Scheme == "unix" {
			serverConfig.Listener.Address = parsedUrl.Path
		} else {
			hostname := parsedUrl.Hostname()
			if strings.Contains(hostname, ":") {
				// it's an ipv6 address
				hostname = "[" + hostname + "]"
			}
			if port := parsedUrl.Port(); port != "" {
				hostname += ":" + port
			} else {
				hostname += ":20001"
			}
			serverConfig.Listener.Address = hostname
		}
	}
}

var QuickServeCmd = &cobra.Command{
	Use:   "quick-serve [address]",
	Short: "Serve a Websocket Filesystem in just one command",
	Example: `  wsfs quick-serve 20001
  wsfs quick-serve username@:20001
  wsfs quick-serve username:password@:20001
  wsfs quick-serve http://username:password@[fe80::12:34]:20001
  wsfs qucik-serve unix://username:password@/run/unix.sock`,
	Args: cobra.MaximumNArgs(1),
	Run: func(c *cobra.Command, args []string) {
		util.SetupZerolog(false, logLevel)

		serverConfig := config.Default

		configStorage(&serverConfig, c)
		configIDs(&serverConfig, c)

		if len(args) != 0 {
			parseArg(&serverConfig, args[0])
		}

		if len(serverConfig.Users) == 0 {
			fmt.Fprintln(os.Stdout, "Warning: anonymous mode")
			serverConfig.Anonymous.Enable = true
		}

		server, err := server.NewServer(&serverConfig)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Init server failed")
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}

		err = server.Run(serverConfig.Listener, serverConfig.TLS)

		if err != nil && err != http.ErrServerClosed {
			fmt.Fprintln(os.Stderr, "Server stoped for error")
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	},
}

func init() {
	if version.IsDebug() {
		logLevel = zerolog.DebugLevel // default debug level in debug mode
	}

	QuickServeCmd.Flags().VarP(
		enumflag.New(&logLevel, "LEVEL", util.ZerologLevelIds, enumflag.EnumCaseInsensitive),
		"level", "l",
		"Sets logging level; can be 'trace', 'debug', 'info', 'warning', 'error', 'fatal', 'panic'")
	QuickServeCmd.Flags().Uint32VarP(&uid, "uid", "", 0, "Uid in filesystem")
	QuickServeCmd.Flags().Uint32VarP(&gid, "gid", "", 0, "Gid in filesystem")
	QuickServeCmd.Flags().Uint32VarP(&otherUid, "other-uid", "", 0, "Other uid in filesystem")
	QuickServeCmd.Flags().Uint32VarP(&otherGid, "other-gid", "", 0, "Other gid in filesystem")
	QuickServeCmd.Flags().StringVarP(&storage, "storage", "s", "", "Storage path")
}

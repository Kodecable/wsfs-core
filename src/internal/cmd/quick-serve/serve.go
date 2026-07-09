package quickserve

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	cmdpassword "wsfs-core/internal/cmd/password"
	"wsfs-core/internal/server"
	serverConfig "wsfs-core/internal/server/config"
	"wsfs-core/internal/util"
	"wsfs-core/version"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/thediveo/enumflag"
	"golang.org/x/crypto/bcrypt"
)

var randomPasswordRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*")

var (
	uid                       uint32
	gid                       uint32
	otherUid                  uint32
	otherGid                  uint32
	storage                   string
	noLogTime                 bool
	noLogColor                bool
	insecureSessionIdMathRand bool
	passwordSource            string
	logLevel                  zerolog.Level = zerolog.InfoLevel
)

func exitWithError(code int, msg string, err error) {
	fmt.Fprintln(os.Stderr, msg)
	fmt.Fprintln(os.Stderr, err)
	os.Exit(code)
}

const storageId = "main"

func configStorage(config *serverConfig.Server, c *cobra.Command) {
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

	config.Storages = append(config.Storages, serverConfig.Storage{Id: storageId, Path: storage})
	config.Anonymous.Storage = storageId
}

func configIDs(config *serverConfig.Server, c *cobra.Command) {
	config.FsIds = util.OptionalFsIds{}
	if c.Flags().Changed("uid") {
		config.FsIds.Uid = &uid
	}
	if c.Flags().Changed("gid") {
		config.FsIds.Gid = &gid
	}
	if c.Flags().Changed("other-uid") {
		config.FsIds.OtherUid = &otherUid
	}
	if c.Flags().Changed("other-gid") {
		config.FsIds.OtherGid = &otherGid
	}
}

func parseArg(config *serverConfig.Server, c *cobra.Command, args string) {
	arg := strings.TrimSpace(args)

	if _, err := strconv.ParseUint(arg, 10, 16); err == nil {
		if c.Flags().Changed("password") {
			exitWithError(2, "Resolve password failed", cmdpassword.ErrMissingUsername)
		}
		config.Listener.Address = ":" + arg
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
			config.Listener.Network = "tcp"
		case "unix":
			config.Listener.Network = "unix"
		default:
			fmt.Fprintln(os.Stderr, "Unsupported listen network: '"+parsedUrl.Scheme+"'")
			os.Exit(2)
		}

		username := parsedUrl.User.Username()
		if username == "" && c.Flags().Changed("password") {
			exitWithError(2, "Resolve password failed", cmdpassword.ErrMissingUsername)
		}
		if username != "" {
			password, hasURLPassword := parsedUrl.User.Password()
			var hash []byte
			password, err = cmdpassword.Resolve(password, hasURLPassword, true, passwordSource, c.Flags().Changed("password"))
			if err != nil {
				exitWithError(2, "Resolve password failed", err)
			}
			if password == "" {
				fmt.Fprintln(os.Stderr, "Warning: password is empty; generating a random password")
				password = util.RandomString(10, randomPasswordRunes)
				fmt.Fprintln(os.Stdout, "Password for user '"+username+"' is '"+password+"'")
			}
			if hash, err = bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost); err != nil {
				exitWithError(2, "Unable to generate password hash", err)
			}
			config.Users = append(config.Users, serverConfig.User{Name: username, SecretHash: string(hash), Storage: storageId})
		}

		if parsedUrl.Scheme == "unix" {
			config.Listener.Address = parsedUrl.Path
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
			config.Listener.Address = hostname
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
  wsfs quick-serve unix://username:password@/run/unix.sock`,
	Args: cobra.MaximumNArgs(1),
	Run: func(c *cobra.Command, args []string) {
		util.SetupZerolog(noLogTime, noLogColor, logLevel)

		config := serverConfig.Default

		configStorage(&config, c)
		configIDs(&config, c)
		config.WSFS.InsecureSessionIdMathRand = insecureSessionIdMathRand

		if len(args) != 0 {
			parseArg(&config, c, args[0])
		} else if c.Flags().Changed("password") {
			exitWithError(2, "Resolve password failed", cmdpassword.ErrMissingUsername)
		}

		if len(config.Users) == 0 {
			fmt.Fprintln(os.Stdout, "Warning: anonymous mode")
			config.Anonymous.Enable = true
			config.Anonymous.ReadOnly = false
		}

		hub, err := server.NewHub()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Init server failed")
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}

		err = hub.Run(config)

		if err != nil && err != http.ErrServerClosed {
			fmt.Fprintln(os.Stderr, "Server stopped for error")
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
	QuickServeCmd.Flags().BoolVarP(&noLogTime, "no-log-time", "", false, "Use log format without time")
	QuickServeCmd.Flags().BoolVarP(&noLogColor, "no-log-color", "", false, "Disable colors in log output")
	QuickServeCmd.Flags().BoolVarP(&insecureSessionIdMathRand, "insecure-session-id-math-rand", "", false, "Use math/rand for WSFS session resume IDs instead of crypto/rand; insecure and easier to predict")
	QuickServeCmd.Flags().StringVarP(&passwordSource, "password", "", "", cmdpassword.FlagUsage)
}

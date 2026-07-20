package quickserve

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	cmdexit "wsfs-core/internal/cmd/exit"
	cmdflags "wsfs-core/internal/cmd/flags"
	cmdpassword "wsfs-core/internal/cmd/password"
	"wsfs-core/internal/server"
	serverConfig "wsfs-core/internal/server/config"
	"wsfs-core/internal/util"
	"wsfs-core/version"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
)

var randomPasswordRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*")

var (
	uid            uint32
	gid            uint32
	otherUid       uint32
	otherGid       uint32
	storage        string
	noLogTime      bool
	noLogColor     bool
	jsonLog        bool
	passwordSource string
	logLevel       zerolog.Level = zerolog.InfoLevel
)

const storageId = "main"

func configStorage(config *serverConfig.Server, c *cobra.Command) error {
	if !c.Flags().Changed("storage") {
		fmt.Fprintln(os.Stdout, "Warning: use working directory as storage")
		workingDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("unable to determine working directory: %w", err)
		}
		storage = workingDir
	}

	config.Storages = append(config.Storages, serverConfig.Storage{Id: storageId, Path: storage})
	config.Anonymous.Storage = storageId
	return nil
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

func parseArg(config *serverConfig.Server, c *cobra.Command, args string) error {
	arg := strings.TrimSpace(args)

	if _, err := strconv.ParseUint(arg, 10, 16); err == nil {
		if c.Flags().Changed("password") {
			return fmt.Errorf("resolve password failed: %w", cmdpassword.ErrMissingUsername)
		}
		config.Listener.Address = ":" + arg
	} else {
		if ok, _ := regexp.MatchString(`.*:?\/\/`, arg); !ok {
			arg = "//" + arg
		}

		parsedUrl, err := url.Parse(arg)
		if err != nil {
			return fmt.Errorf("parse URL failed: %w", err)
		}

		switch strings.ToLower(parsedUrl.Scheme) {
		case "http", "wsfs", "tcp", "":
			config.Listener.Network = "tcp"
		case "unix":
			config.Listener.Network = "unix"
		default:
			return fmt.Errorf("unsupported listen network: %q", parsedUrl.Scheme)
		}

		username := parsedUrl.User.Username()
		if username == "" && c.Flags().Changed("password") {
			return fmt.Errorf("resolve password failed: %w", cmdpassword.ErrMissingUsername)
		}
		if username != "" {
			password, hasURLPassword := parsedUrl.User.Password()
			var hash []byte
			password, err = cmdpassword.Resolve(password, hasURLPassword, true, passwordSource, c.Flags().Changed("password"))
			if err != nil {
				return fmt.Errorf("resolve password failed: %w", err)
			}
			if password == "" {
				fmt.Fprintln(os.Stderr, "Warning: password is empty; generating a random password")
				password = util.RandomString(10, randomPasswordRunes)
				fmt.Fprintln(os.Stdout, "Password for user '"+username+"' is '"+password+"'")
			}
			if hash, err = bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost); err != nil {
				return fmt.Errorf("unable to generate password hash: %w", err)
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
	return nil
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
	RunE: func(c *cobra.Command, args []string) error {
		util.SetupZerolog(noLogTime, noLogColor, jsonLog, logLevel)

		config := serverConfig.Default

		if err := configStorage(&config, c); err != nil {
			return cmdexit.New(1, err)
		}
		configIDs(&config, c)

		if len(args) != 0 {
			if err := parseArg(&config, c, args[0]); err != nil {
				return cmdexit.New(2, err)
			}
		} else if c.Flags().Changed("password") {
			return cmdexit.New(2, fmt.Errorf("resolve password failed: %w", cmdpassword.ErrMissingUsername))
		}

		if len(config.Users) == 0 {
			fmt.Fprintln(os.Stdout, "Warning: anonymous mode")
			config.Anonymous.Enable = true
			config.Anonymous.ReadOnly = false
		}

		hub, err := server.NewHub()
		if err != nil {
			return cmdexit.New(2, fmt.Errorf("init server failed: %w", err))
		}

		util.SetupSignalHandlers(util.SignalHandlers{
			Sighup:  hub.IssueShutdown,
			Sigint:  hub.IssueShutdown,
			Sigterm: hub.IssueShutdown,
			OnHandlerPanic: func(obj any) {
				log.Error().Any("Error", obj).Msg("Panic during signal handling")
			},
		})

		err = hub.Run(config)

		if err != nil && err != http.ErrServerClosed {
			return cmdexit.New(1, fmt.Errorf("server stopped for error: %w", err))
		}
		return nil
	},
}

func init() {
	if version.IsDebug() {
		logLevel = zerolog.DebugLevel // default debug level in debug mode
	}

	cmdflags.AddLoggingFlags(QuickServeCmd.Flags(), &logLevel, &noLogTime, &noLogColor, &jsonLog)
	cmdflags.AddFsIDFlags(QuickServeCmd.Flags(), &uid, &gid, &otherUid, &otherGid)
	QuickServeCmd.Flags().StringVarP(&storage, "storage", "s", "", "Storage path")
	cmdflags.AddPasswordFlag(QuickServeCmd.Flags(), &passwordSource)
}

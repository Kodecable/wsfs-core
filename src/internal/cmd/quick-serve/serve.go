package quickserve

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"wsfs-core/buildinfo"
	"wsfs-core/internal/server"
	"wsfs-core/internal/server/config"
	"wsfs-core/internal/util"

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

var QuickServeCmd = &cobra.Command{
	Use:   "quick-serve",
	Short: "Serve a Websocket Filesystem in just one command",
	Args:  cobra.MaximumNArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		util.SetupZerolog(false, logLevel)

		serverConfig := config.Default
		const storageId = "main"
		serverConfig.Storages = append(serverConfig.Storages, config.Storage{Id: storageId, Path: storage})
		serverConfig.Anonymous.Storage = storageId
		serverConfig.Uid = int64(uid)
		serverConfig.Gid = int64(gid)
		serverConfig.OtherUid = int64(otherUid)
		serverConfig.Gid = int64(otherGid)

		if len(args) != 0 {
			arg := strings.TrimSpace(args[0])

			if _, err := strconv.ParseUint(arg, 10, 16); err == nil {
				serverConfig.Listener.Address = ":" + arg
			} else {
				if ok, _ := regexp.MatchString(`.*:?\/\/`, arg); !ok {
					arg = "//" + arg
				}

				parsedUrl, err := url.Parse(arg)
				if err != nil {
					exitWithError(2, "Parse arg failed", err)
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
	if buildinfo.IsDebug() {
		logLevel = zerolog.DebugLevel
	}

	defaultIds, err := util.GetDefaultIds()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Unable to determine default (nobody) u/gids")
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	storage, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Unable to determine working directory")
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	QuickServeCmd.Flags().VarP(
		enumflag.New(&logLevel, "LEVEL", util.ZerologLevelIds, enumflag.EnumCaseInsensitive),
		"level", "l",
		"Sets logging level; can be 'trace', 'debug', 'info', 'warning', 'error', 'fatal', 'panic'")
	QuickServeCmd.Flags().Uint32VarP(&uid, "uid", "", defaultIds.CurrentUser, "Uid in filesystem")
	QuickServeCmd.Flags().Uint32VarP(&gid, "gid", "", defaultIds.UserGroup, "Gid in filesystem")
	QuickServeCmd.Flags().Uint32VarP(&otherUid, "other-uid", "", defaultIds.NobodyUser, "Nobody uid in filesystem")
	QuickServeCmd.Flags().Uint32VarP(&otherGid, "other-gid", "", defaultIds.NobodyGroup, "Nobody gid in filesystem")
	QuickServeCmd.Flags().StringVarP(&storage, "storage", "s", storage, "Nobody gid in filesystem")
}

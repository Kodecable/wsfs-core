package serve

import (
	"fmt"
	"net/http"
	"os"
	"wsfs-core/buildinfo"
	"wsfs-core/internal/server"
	serverConfig "wsfs-core/internal/server/config"
	"wsfs-core/internal/util"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/thediveo/enumflag"
)

var (
	configPath string
	noLogTime  bool
	logLevel   zerolog.Level = zerolog.InfoLevel
)

var ServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start a Websocket Filesystem server",
	Args:  cobra.NoArgs,
	Run: func(_ *cobra.Command, _ []string) {
		util.SetupZerolog(noLogTime, logLevel)

		config := findAndDecodeConfig()

		server, err := server.NewServer(config)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Init server failed")
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}

		util.SetupSignalHandler(func() {
			newConfig := serverConfig.Default
			err := serverConfig.ReDecode(&newConfig, config)
			if err != nil {
				log.Err(err).Msg("Reload canceled")
				return
			}

			newServer, err := server.Reload(&newConfig)
			if err != nil {
				log.Err(err).Msg("Reload canceled")
				return
			}

			server = newServer
		}, func() {}, func() {}) // TODO: gracefully shutdown

		err = server.Run(config.Listener, config.TLS)

		if err != nil && err != http.ErrServerClosed {
			fmt.Fprintln(os.Stderr, "Server stoped for error")
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	},
}

func init() {
	if buildinfo.IsDebug() {
		logLevel = zerolog.DebugLevel // default debug level in debug mode
	}

	ServeCmd.Flags().StringVarP(&configPath, "config", "c", iternalDefaultConfigPath, "Path to config file")
	ServeCmd.Flags().BoolVarP(&noLogTime, "no-log-time", "", false, "Use log format without time")
	ServeCmd.Flags().VarP(
		enumflag.New(&logLevel, "LEVEL", util.ZerologLevelIds, enumflag.EnumCaseInsensitive),
		"level", "l",
		"Sets logging level; can be 'trace', 'debug', 'info', 'warning', 'error', 'fatal', 'panic'")
}

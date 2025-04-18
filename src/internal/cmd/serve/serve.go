package serve

import (
	"fmt"
	"net/http"
	"os"
	"wsfs-core/internal/server"
	serverConfig "wsfs-core/internal/server/config"
	"wsfs-core/internal/util"
	"wsfs-core/version"

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

		hub, err := server.NewHub()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Init server failed")
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}

		hub.GetConfig = func() (serverConfig.Server, error) {
			return serverConfig.ReDecode(&config)
		}

		util.SetupSignalHandlers(util.SignalHandlers{
			Sighup:  hub.IssueReload,
			Sigint:  hub.IssueShutdown,
			Sigterm: hub.IssueShutdown,
			OnHandlerPanic: func(obj any) {
				log.Error().Any("Error", obj).Msg("Panic during signal handling")
			},
		})

		err = hub.Run(config)

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

	ServeCmd.Flags().StringVarP(&configPath, "config", "c", iternalDefaultConfigPath, "Path to config file")
	ServeCmd.Flags().BoolVarP(&noLogTime, "no-log-time", "", false, "Use log format without time")
	ServeCmd.Flags().VarP(
		enumflag.New(&logLevel, "LEVEL", util.ZerologLevelIds, enumflag.EnumCaseInsensitive),
		"level", "l",
		"Sets logging level; can be 'trace', 'debug', 'info', 'warning', 'error', 'fatal', 'panic'")
}

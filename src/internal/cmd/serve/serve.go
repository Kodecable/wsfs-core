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
	configPath                string
	noLogTime                 bool
	noLogColor                bool
	insecureSessionIdMathRand bool
	logLevel                  zerolog.Level = zerolog.InfoLevel
)

var ServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start a Websocket Filesystem server",
	Args:  cobra.NoArgs,
	Run: func(c *cobra.Command, _ []string) {
		util.SetupZerolog(noLogTime, noLogColor, false, logLevel)

		config := findAndDecodeConfig()
		if c.Flags().Changed("insecure-session-id-math-rand") {
			config.WSFS.InsecureSessionIdMathRand = insecureSessionIdMathRand
		}

		hub, err := server.NewHub()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Init server failed")
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}

		hub.GetConfig = func() (serverConfig.Server, error) {
			newConfig, err := serverConfig.ReDecode(&config)
			if err != nil {
				return newConfig, err
			}
			if c.Flags().Changed("insecure-session-id-math-rand") {
				newConfig.WSFS.InsecureSessionIdMathRand = insecureSessionIdMathRand
			}
			return newConfig, nil
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

	ServeCmd.Flags().StringVarP(&configPath, "config", "c", internalDefaultConfigPath, "Path to config file")
	ServeCmd.Flags().BoolVarP(&noLogTime, "no-log-time", "", false, "Use log format without time")
	ServeCmd.Flags().BoolVarP(&noLogColor, "no-log-color", "", false, "Disable colors in log output")
	ServeCmd.Flags().BoolVarP(&insecureSessionIdMathRand, "insecure-session-id-math-rand", "", false, "Use math/rand for WSFS session resume IDs instead of crypto/rand; insecure and easier to predict")
	ServeCmd.Flags().VarP(
		enumflag.New(&logLevel, "LEVEL", util.ZerologLevelIds, enumflag.EnumCaseInsensitive),
		"level", "l",
		"Sets logging level; can be 'trace', 'debug', 'info', 'warning', 'error', 'fatal', 'panic'")
}

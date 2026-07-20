package serve

import (
	"fmt"
	"net/http"
	cmdexit "wsfs-core/internal/cmd/exit"
	cmdflags "wsfs-core/internal/cmd/flags"
	"wsfs-core/internal/server"
	serverConfig "wsfs-core/internal/server/config"
	"wsfs-core/internal/util"
	"wsfs-core/version"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	configPath                string
	noLogTime                 bool
	noLogColor                bool
	jsonLog                   bool
	insecureSessionIdMathRand bool
	logLevel                  zerolog.Level = zerolog.InfoLevel
)

var ServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start a Websocket Filesystem server",
	Args:  cobra.NoArgs,
	RunE: func(c *cobra.Command, _ []string) error {
		util.SetupZerolog(noLogTime, noLogColor, jsonLog, logLevel)

		config, err := findAndDecodeConfig(configPath)
		if err != nil {
			return cmdexit.New(2, err)
		}
		if c.Flags().Changed("insecure-session-id-math-rand") {
			config.WSFS.InsecureSessionIdMathRand = insecureSessionIdMathRand
		}

		hub, err := server.NewHub()
		if err != nil {
			return cmdexit.New(2, fmt.Errorf("init server failed: %w", err))
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
			return cmdexit.New(1, fmt.Errorf("server stopped for error: %w", err))
		}
		return nil
	},
}

func init() {
	if version.IsDebug() {
		logLevel = zerolog.DebugLevel // default debug level in debug mode
	}

	ServeCmd.Flags().StringVarP(&configPath, "config", "c", internalDefaultConfigPath, "Path to config file")
	cmdflags.AddLoggingFlags(ServeCmd.Flags(), &logLevel, &noLogTime, &noLogColor, &jsonLog)
	ServeCmd.Flags().BoolVar(&insecureSessionIdMathRand, "insecure-session-id-math-rand", false, "Use math/rand for WSFS session resume IDs instead of crypto/rand; insecure and easier to predict")
}

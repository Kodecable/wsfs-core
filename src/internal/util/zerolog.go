package util

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var ZerologLevelIds = map[zerolog.Level][]string{
	zerolog.TraceLevel: {"trace"},
	zerolog.DebugLevel: {"debug"},
	zerolog.InfoLevel:  {"info"},
	zerolog.WarnLevel:  {"warning", "warn"},
	zerolog.ErrorLevel: {"error"},
	zerolog.FatalLevel: {"fatal"},
	zerolog.PanicLevel: {"panic"},
}

func SetupZerolog(noLogTime bool, noColor bool, level zerolog.Level) {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.ErrorFieldName = "Error"
	if noLogTime {
		log.Logger = log.Output(zerolog.ConsoleWriter{
			Out:             os.Stdout,
			NoColor:         noColor,
			FormatTimestamp: func(a interface{}) string { return "" },
		})
	} else {
		log.Logger = log.Output(zerolog.ConsoleWriter{
			Out:        os.Stdout,
			NoColor:    noColor,
			TimeFormat: time.RFC3339,
		})
	}
	zerolog.SetGlobalLevel(level)
}

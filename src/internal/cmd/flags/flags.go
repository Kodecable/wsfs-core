package cmdflags

import (
	"wsfs-core/internal/cmd/password"
	"wsfs-core/internal/util"

	"github.com/rs/zerolog"
	"github.com/spf13/pflag"
	"github.com/thediveo/enumflag"
)

func AddLoggingFlags(flags *pflag.FlagSet, level *zerolog.Level, noLogTime, noLogColor, jsonLog *bool) {
	flags.BoolVar(noLogTime, "no-log-time", false, "Use log format without time")
	flags.BoolVar(noLogColor, "no-log-color", false, "Disable colors in log output")
	flags.BoolVar(jsonLog, "json-log", false, "Write logs as newline-delimited JSON")
	flags.VarP(
		enumflag.New(level, "LEVEL", util.ZerologLevelIds, enumflag.EnumCaseInsensitive),
		"level", "l",
		"Sets logging level; can be 'trace', 'debug', 'info', 'warning', 'error', 'fatal', 'panic'")
}

func AddPasswordFlag(flags *pflag.FlagSet, passwordSource *string) {
	flags.StringVar(passwordSource, "password", "", password.FlagUsage)
}

func AddFsIDFlags(flags *pflag.FlagSet, uid, gid, otherUID, otherGID *uint32) {
	flags.Uint32Var(uid, "uid", 0, "Uid in filesystem (Unix only)")
	flags.Uint32Var(gid, "gid", 0, "Gid in filesystem (Unix only)")
	flags.Uint32Var(otherUID, "other-uid", 0, "Other uid in filesystem (Unix only)")
	flags.Uint32Var(otherGID, "other-gid", 0, "Other gid in filesystem (Unix only)")
}

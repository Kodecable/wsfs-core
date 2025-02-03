package version

import "strings"

func IsDebug() bool {
	return strings.ToLower(Mode) == "debug"
}

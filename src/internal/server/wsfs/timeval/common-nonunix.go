//go:build !unix

package timeval

import (
	"os"
	"time"

	"wsfs-core/internal/share/wsfsprotocol"
)

func SetPathMTime(path string, ts wsfsprotocol.Timespec) error {
	t := time.Unix(ts.Seconds, ts.Nanoseconds)
	return os.Chtimes(path, t, t)
}

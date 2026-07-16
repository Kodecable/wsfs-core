//go:build linux

package client

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func fuseConnectionID(mountpoint string) (*uint64, error) {
	absMountpoint, err := filepath.Abs(mountpoint)
	if err != nil {
		return nil, fmt.Errorf("resolve mountpoint: %w", err)
	}

	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return nil, fmt.Errorf("read mountinfo: %w", err)
	}

	id, err := fuseConnectionIDFromMountInfo(string(data), filepath.Clean(absMountpoint))
	return &id, err
}

func fuseConnectionIDFromMountInfo(mountInfo, mountpoint string) (uint64, error) {
	for line := range strings.SplitSeq(mountInfo, "\n") {
		parts := strings.SplitN(line, " - ", 2)
		if len(parts) != 2 {
			continue
		}

		left := strings.Fields(parts[0])
		right := strings.Fields(parts[1])
		if len(left) < 5 || len(right) < 1 || !isFuseFilesystem(right[0]) {
			continue
		}

		target, err := unescapeMountInfoPath(left[4])
		if err != nil || target != mountpoint {
			continue
		}

		device := strings.SplitN(left[2], ":", 2)
		if len(device) != 2 || device[0] != "0" {
			continue
		}
		connectionID, err := strconv.ParseUint(device[1], 10, 32)
		if err != nil || connectionID == 0 {
			continue
		}
		return connectionID, nil
	}

	return 0, fmt.Errorf("FUSE mountpoint %q not found in mountinfo", mountpoint)
}

func isFuseFilesystem(filesystemType string) bool {
	return filesystemType == "fuse" || filesystemType == "fuseblk" || strings.HasPrefix(filesystemType, "fuse.")
}

func unescapeMountInfoPath(value string) (string, error) {
	var out strings.Builder
	out.Grow(len(value))
	for i := 0; i < len(value); i++ {
		if value[i] != '\\' {
			out.WriteByte(value[i])
			continue
		}
		if i+3 >= len(value) {
			return "", fmt.Errorf("invalid mountinfo escape %q", value)
		}
		escaped, err := strconv.ParseUint(value[i+1:i+4], 8, 8)
		if err != nil {
			return "", fmt.Errorf("invalid mountinfo escape %q: %w", value[i:i+4], err)
		}
		out.WriteByte(byte(escaped))
		i += 3
	}
	return out.String(), nil
}

//go:build windows

package util

import "strings"

var (
	specialDosDeviceNames = []string{
		"AUX",
		"CON",
		"CONIN$",
		"CONOUT$",
		"CLOCK$",
		"CONFIG$",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "COM²", "COM³", "COM¹",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9", "LPT²", "LPT³", "LPT¹",
		"NUL",
		"PRN",
	}
)

func isUrlValid_os(v string) bool {
	if len(v) >= 2 && v[1] == '/' {
		// UNC path or Device path or Verbatim path
		return false
	}
	if strings.Contains(v, ":") {
		// NTFS Alternate Data Streams
		return false
	}

	parts := strings.Split(v, "/")
	for _, part := range parts {
		baseName := part
		if idx := strings.IndexByte(baseName, '.'); idx != -1 {
			baseName = baseName[:idx]
		}
		baseName = strings.TrimSpace(baseName)

		for _, nm := range specialDosDeviceNames {
			if strings.EqualFold(baseName, nm) {
				return false
			}
		}
	}
	return true
}

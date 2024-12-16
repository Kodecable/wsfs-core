package util

import "strings"

func IsUrlValid(v string) bool {
	if len(v) == 0 ||
		v[0] != '/' ||
		strings.Contains(v, "/../") ||
		strings.HasSuffix(v, "/..") {
		return false
	}
	return true
}

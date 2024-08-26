package util

import "strings"

func IsUrlValid(v string) bool {
	if v[0] != '/' {
		return false
	}
	if strings.Contains(v, "/../") {
		return false
	}
	if strings.HasSuffix(v, "/..") {
		return false
	}
	return true
}

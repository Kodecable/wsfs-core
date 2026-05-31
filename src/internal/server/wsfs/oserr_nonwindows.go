//go:build !windows

package wsfs

func osErrCode_osOverride(error) (uint8, bool) {
	return 0, false
}

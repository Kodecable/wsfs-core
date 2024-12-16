package webdav

import (
	"errors"
	"regexp"
	"strconv"
)

var rangeExp = regexp.MustCompile(`bytes=(-?\d+)(-\d+)?`)

// Parse X-Update-Range header.
//
// RETURN:
//  1. if err is not nil: start and end are invaild
//  2. if end < 0: append
//
// REFERS:
//   - https://sabre.io/dav/http-patch/
func parseRange(s string) (start, end int64, err error) {
	if s == "append" {
		return 0, -1, nil
	}

	rr := rangeExp.FindAllStringSubmatch(s, -1)
	if rr == nil || len(rr) != 1 {
		return 0, 0, errors.New("bad range format")
	}

	if rr[0][2] != "" {
		start, _ = strconv.ParseInt(rr[0][1], 10, 64)
		end, _ = strconv.ParseInt(rr[0][2][1:], 10, 64)
		if start > end {
			return 0, 0, errors.New("range invaild")
		}
		if start < 0 {
			return 0, 0, errors.New("range invaild")
		}
	} else {
		start, _ = strconv.ParseInt(rr[0][1], 10, 64)
	}

	return
}

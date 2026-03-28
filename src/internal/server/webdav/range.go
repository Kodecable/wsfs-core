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
		var err1, err2 error
		start, err1 = strconv.ParseInt(rr[0][1], 10, 64)
		end, err2 = strconv.ParseInt(rr[0][2][1:], 10, 64)
		if err1 != nil || err2 != nil || start > end || start < 0 {
			return 0, 0, errors.New("range invaild")
		}
	} else {
		start, _ = strconv.ParseInt(rr[0][1], 10, 64)
	}

	return
}

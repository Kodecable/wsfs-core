package util

import "net/http"

type ErrorPageFunc func(http.ResponseWriter, *http.Request, int, string)

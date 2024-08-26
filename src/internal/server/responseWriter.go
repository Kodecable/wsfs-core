package server

import (
	"bufio"
	"net"
	"net/http"
)

const statusUnwrited = -1

type responseWriter struct {
	http.ResponseWriter
	status int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{w, statusUnwrited}
}

func (rsp *responseWriter) WriteHeader(status int) {
	rsp.status = status
	rsp.ResponseWriter.WriteHeader(status)
}

func (rsp *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	//h, ok := rsp.ResponseWriter.(http.Hijacker)
	//if !ok {
	//	return nil, nil, errors.New("hijack not supported")
	//}
	return rsp.ResponseWriter.(http.Hijacker).Hijack()
}

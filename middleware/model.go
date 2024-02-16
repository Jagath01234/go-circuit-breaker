package middleware

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

var endpointMap sync.Map

type ReqInfo struct {
	uRL            string
	tail           [3]bool
	isBlocked      bool
	blockTimeLevel int
	blockType      int
	lastPing       time.Time
}

type customResponseWriter struct {
	http.ResponseWriter
	status int
}

func (crw *customResponseWriter) WriteHeader(status int) {
	crw.status = status
	crw.ResponseWriter.WriteHeader(status)
}

func (crw *customResponseWriter) Write(b []byte) (int, error) {
	return crw.ResponseWriter.Write(b)
}

func (crw *customResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := crw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

func (crw *customResponseWriter) Flush() {
	flusher, ok := crw.ResponseWriter.(http.Flusher)
	if ok {
		flusher.Flush()
	}
}

package middleware

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

/*
init to

give the consecutive error number
give the success status types
give the retry time series.
give is it for endpoint wise or total endpoints
*/

type blockLevel int

const (
	BlockLevelEndpoint blockLevel = 1
	BlockLevelRouter   blockLevel = 2
)

type httpStatus int

const (
	StatusSwitchingProtocols httpStatus = 101 // RFC 9110, 15.2.2
	StatusContinue           httpStatus = 100 // RFC 9110, 15.2.1
	StatusProcessing         httpStatus = 102 // RFC 2518, 10.1
	StatusEarlyHints         httpStatus = 103 // RFC 8297

	StatusOK                   httpStatus = 200 // RFC 9110, 15.3.1
	StatusCreated              httpStatus = 201 // RFC 9110, 15.3.2
	StatusAccepted             httpStatus = 202 // RFC 9110, 15.3.3
	StatusNonAuthoritativeInfo httpStatus = 203 // RFC 9110, 15.3.4
	StatusNoContent            httpStatus = 204 // RFC 9110, 15.3.5
	StatusResetContent         httpStatus = 205 // RFC 9110, 15.3.6
	StatusPartialContent       httpStatus = 206 // RFC 9110, 15.3.7
	StatusMultiStatus          httpStatus = 207 // RFC 4918, 11.1
	StatusAlreadyReported      httpStatus = 208 // RFC 5842, 7.1
	StatusIMUsed               httpStatus = 226 // RFC 3229, 10.4.1

	StatusMultipleChoices   httpStatus = 300 // RFC 9110, 15.4.1
	StatusMovedPermanently  httpStatus = 301 // RFC 9110, 15.4.2
	StatusFound             httpStatus = 302 // RFC 9110, 15.4.3
	StatusSeeOther          httpStatus = 303 // RFC 9110, 15.4.4
	StatusNotModified       httpStatus = 304 // RFC 9110, 15.4.5
	StatusUseProxy          httpStatus = 305 // RFC 9110, 15.4.6
	_                       httpStatus = 306 // RFC 9110, 15.4.7 (Unused)
	StatusTemporaryRedirect httpStatus = 307 // RFC 9110, 15.4.8
	StatusPermanentRedirect httpStatus = 308 // RFC 9110, 15.4.9

	StatusBadRequest                             httpStatus = 400 // RFC 9110, 15.5.1
	StatusUnauthorized                           httpStatus = 401 // RFC 9110, 15.5.2
	StatusPaymentRequired                        httpStatus = 402 // RFC 9110, 15.5.3
	StatusForbidden                              httpStatus = 403 // RFC 9110, 15.5.4
	StatusNotFound                               httpStatus = 404 // RFC 9110, 15.5.5
	StatusMethodNotAllowed                       httpStatus = 405 // RFC 9110, 15.5.6
	StatusNotAcceptable                          httpStatus = 406 // RFC 9110, 15.5.7
	StatusProxyAuthRequired                      httpStatus = 407 // RFC 9110, 15.5.8
	StatusRequestTimeout                         httpStatus = 408 // RFC 9110, 15.5.9
	StatusConflict                               httpStatus = 409 // RFC 9110, 15.5.10
	StatusGone                                   httpStatus = 410 // RFC 9110, 15.5.11
	StatusLengthRequired                         httpStatus = 411 // RFC 9110, 15.5.12
	StatusPreconditionFailed                     httpStatus = 412 // RFC 9110, 15.5.13
	StatusRequestEntityTooLarge                  httpStatus = 413 // RFC 9110, 15.5.14
	StatusRequestURITooLong                      httpStatus = 414 // RFC 9110, 15.5.15
	StatusUnsupportedMediaType                   httpStatus = 415 // RFC 9110, 15.5.16
	StatusRequestedRangeNotSatisfiablehttpStatus            = 416 // RFC 9110, 15.5.17
	StatusExpectationFailed                      httpStatus = 417 // RFC 9110, 15.5.18
	StatusTeapot                                 httpStatus = 418 // RFC 9110, 15.5.19 (Unused)
	StatusMisdirectedRequest                     httpStatus = 421 // RFC 9110, 15.5.20
	StatusUnprocessableEntity                    httpStatus = 422 // RFC 9110, 15.5.21
	StatusLocked                                 httpStatus = 423 // RFC 4918, 11.3
	StatusFailedDependency                       httpStatus = 424 // RFC 4918, 11.4
	StatusTooEarly                               httpStatus = 425 // RFC 8470, 5.2.
	StatusUpgradeRequired                        httpStatus = 426 // RFC 9110, 15.5.22
	StatusPreconditionRequired                   httpStatus = 428 // RFC 6585, 3
	StatusTooManyRequests                        httpStatus = 429 // RFC 6585, 4
	StatusRequestHeaderFieldsTooLarge            httpStatus = 431 // RFC 6585, 5
	StatusUnavailableForLegalReasons             httpStatus = 451 // RFC 7725, 3

	StatusInternalServerError           httpStatus = 500 // RFC 9110, 15.6.1
	StatusNotImplemented                httpStatus = 501 // RFC 9110, 15.6.2
	StatusBadGateway                    httpStatus = 502 // RFC 9110, 15.6.3
	StatusServiceUnavailable            httpStatus = 503 // RFC 9110, 15.6.4
	StatusGatewayTimeout                httpStatus = 504 // RFC 9110, 15.6.5
	StatusHTTPVersionNotSupported       httpStatus = 505 // RFC 9110, 15.6.6
	StatusVariantAlsoNegotiates         httpStatus = 506 // RFC 2295, 8.1
	StatusInsufficientStorage           httpStatus = 507 // RFC 4918, 11.5
	StatusLoopDetected                  httpStatus = 508 // RFC 5842, 7.2
	StatusNotExtended                   httpStatus = 510 // RFC 2774, 7
	StatusNetworkAuthenticationRequired httpStatus = 511 // RFC 6585, 6

)

type breakerConfig struct {
	NumAllowedErrorResp       int //number of consequent error responses allowed before blocking the requests
	SuccessStatus             []httpStatus
	RetryDurationAfterBlocked []time.Duration
	BlockLevel                blockLevel
}

var breakerConf breakerConfig
var endpointMap sync.Map
var tailArr []bool
var RouterReqStatObj RouterReqStat

type RouterReqStat struct {
	tail           []bool
	isBlocked      bool
	blockTimeLevel int
	lastPing       time.Time
}

type ReqInfo struct {
	uRL            string
	tail           []bool
	isBlocked      bool
	blockTimeLevel int
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

package middleware

import (
	"log"
	"net/http"
	"time"
)

func InitBreakerConfig(numAllowedErrorResp int, successStatus []httpStatus, retryDurationAfterBlocked []time.Duration, blockLevel blockLevel) *breakerConfig {
	if numAllowedErrorResp <= 0 {
		numAllowedErrorResp = 3
	}
	tailArr = make([]bool, breakerConf.NumAllowedErrorResp)
	for i := range tailArr {
		tailArr[i] = true
	}
	if successStatus == nil || len(successStatus) == 0 {
		var successStatus []httpStatus
		successStatus[0] = StatusOK
	}
	if retryDurationAfterBlocked == nil || len(retryDurationAfterBlocked) == 0 {
		var allowMillsAfterWhenBlocked []time.Duration
		allowMillsAfterWhenBlocked[0] = time.Second
	}
	if blockLevel != BlockLevelEndpoint && blockLevel != BlockLevelRouter {
		blockLevel = BlockLevelEndpoint
	}
	return &breakerConfig{
		NumAllowedErrorResp:       numAllowedErrorResp,
		SuccessStatus:             successStatus,
		RetryDurationAfterBlocked: retryDurationAfterBlocked,
		BlockLevel:                blockLevel,
	}
}

func CircuitBreaker(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Request URL: %s", r.URL)
		if breakerConf.BlockLevel == BlockLevelEndpoint {
			prevCalls, ok := endpointMap.Load(r.URL.String())

			crw := &customResponseWriter{ResponseWriter: w}
			if !ok {
				newCallInfo := ReqInfo{
					uRL:            r.URL.String(),
					tail:           tailArr,
					isBlocked:      false,
					blockTimeLevel: 0,
					lastPing:       time.Now(),
				}

				endpointMap.Store(r.URL.String(), newCallInfo)
				next.ServeHTTP(crw, r)
			} else {
				info := prevCalls.(ReqInfo)
				if info.isBlocked {
					if time.Since(info.lastPing) > 3*time.Second {
						info.lastPing = time.Now()
						endpointMap.Store(r.URL.String(), info)
					} else {
						http.Error(w, "Forbidden", http.StatusForbidden)
						return
					}
				} else {
					info.lastPing = time.Now()
					endpointMap.Store(r.URL.String(), info)
				}
			}
			next.ServeHTTP(crw, r)

			log.Printf("Response status: %d", crw.status)
			prevCalls, ok = endpointMap.Load(r.URL.String())
			info := prevCalls.(ReqInfo)
			lastIndex := len(info.tail) - 1
			if breakerConf.containsState(httpStatus(crw.status)) {
				for i := 0; i <= lastIndex; i++ {
					info.tail[i] = true
				}
			} else {
				j := 0
				for i := 0; i <= lastIndex; i++ {
					if i < lastIndex {
						info.tail[i] = info.tail[i+1]
					} else {
						info.tail[i] = false
					}
					if !info.tail[i] {
						j++
					}
				}
				if j > lastIndex {
					info.isBlocked = true
				}
			}
			endpointMap.Store(r.URL.String(), info)
		} else {
			crw := &customResponseWriter{ResponseWriter: w}
			if RouterReqStatObj.lastPing.IsZero() {
				RouterReqStatObj = RouterReqStat{
					tail:           []bool{},
					isBlocked:      false,
					blockTimeLevel: 0,
					lastPing:       time.Now(),
				}
				next.ServeHTTP(crw, r)
			} else {

				if RouterReqStatObj.isBlocked {
					if time.Since(RouterReqStatObj.lastPing) > 3*time.Second {
						RouterReqStatObj.lastPing = time.Now()
					} else {
						http.Error(w, "Forbidden", http.StatusForbidden)
						return
					}
				} else {
					RouterReqStatObj.lastPing = time.Now()
				}
			}
			next.ServeHTTP(crw, r)

			log.Printf("Response status: %d", crw.status)

			lastIndex := len(RouterReqStatObj.tail) - 1
			if breakerConf.containsState(httpStatus(crw.status)) {
				for i := 0; i <= lastIndex; i++ {
					RouterReqStatObj.tail[i] = true
				}
			} else {
				j := 0
				for i := 0; i <= lastIndex; i++ {
					if i < lastIndex {
						RouterReqStatObj.tail[i] = RouterReqStatObj.tail[i+1]
					} else {
						RouterReqStatObj.tail[i] = false
					}
					if !RouterReqStatObj.tail[i] {
						j++
					}
				}
				if j > lastIndex {
					RouterReqStatObj.isBlocked = true
				}
			}
		}
	})
}

func (s breakerConfig) containsState(status httpStatus) bool {
	for _, code := range s.SuccessStatus {
		if code == status {
			return true
		}
	}
	return false
}

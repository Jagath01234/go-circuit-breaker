package middleware

import (
	"log"
	"net/http"
	"time"
)

func CircuitBreaker(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Request URL: %s", r.URL)
		prevCalls, ok := endpointMap.Load(r.URL.String())
		crw := &customResponseWriter{ResponseWriter: w}
		if !ok {
			newCallInfo := ReqInfo{
				uRL:            r.URL.String(),
				tail:           [3]bool{},
				isBlocked:      false,
				blockTimeLevel: 0,
				blockType:      1,
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

		// Intercept the response here
		log.Printf("Response status: %d", crw.status)
		prevCalls, ok = endpointMap.Load(r.URL.String())
		info := prevCalls.(ReqInfo)
		if crw.status == http.StatusOK || crw.status == http.StatusCreated {
			lastIndex := len(info.tail) - 1
			for i := 0; i <= lastIndex; i++ {
				info.tail[i] = true
				endpointMap.Store(r.URL.String(), info)
			}
		} else {
			{
				lastIndex := len(info.tail) - 1
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
				if j == lastIndex {
					info.isBlocked = true
					endpointMap.Store(r.URL.String(), info)
				}
			}
		}
	})
}

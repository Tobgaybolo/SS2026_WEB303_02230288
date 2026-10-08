package main

import (
	"log"
	"net/http"
	"time"
)

// statusRecorder remembers the status code so the logger can print it.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// logging prints one line per request, including how long it took. The
// durations make "fast failure" visible when the circuit breaker is open.
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s -> %d (%s)", r.Method, r.URL.Path, rec.status,
			time.Since(start).Round(time.Millisecond))
	})
}

// recoverPanics turns a panic in a handler into a 500 instead of a crashed process.
func recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				log.Printf("panic: %v", p)
				writeError(w, http.StatusInternalServerError, "Internal", "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
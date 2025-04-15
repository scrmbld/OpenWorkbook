package logging

import (
	"log"
	"net/http"
)

// capture the status code from and http.ResponseWriter
// https://gist.github.com/Boerworz/b683e46ae0761056a636
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func NewLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	// default to OK because if WriteHeader isn't called, the writer defaults to OK
	return &loggingResponseWriter{w, http.StatusOK}
}

func (lrw *loggingResponseWriter) WriteHeader(statusCode int) {
	lrw.statusCode = statusCode
	lrw.ResponseWriter.WriteHeader(statusCode)
}

func NewLoggingMiddleware(next http.Handler, logger *log.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// logger.Printf("<-- %s %s", r.Method, r.URL.Path)
		lrw := NewLoggingResponseWriter(w)
		next.ServeHTTP(lrw, r)
		logger.Printf("%d %s %s", lrw.statusCode, r.Method, r.URL.Path)
	})
}

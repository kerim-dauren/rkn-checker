package rest

import (
	"log/slog"
	"net/http"
	"time"
)

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapper := &responseWriter{ResponseWriter: w, statusCode: 200}
		next.ServeHTTP(wrapper, r)

		duration := time.Since(start)

		if wrapper.statusCode >= 400 {
			slog.Warn("HTTP request failed",
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapper.statusCode,
				"duration", duration.String(),
				"user_agent", r.UserAgent(),
				"remote_ip", getRemoteIP(r))
		} else {
			slog.Info("HTTP request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapper.statusCode,
				"duration", duration.String(),
				"user_agent", r.UserAgent(),
				"remote_ip", getRemoteIP(r))
		}
	})
}

func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("HTTP request panicked",
					"method", r.Method,
					"path", r.URL.Path,
					"panic", err)

				WriteErrorResponse(w, http.StatusInternalServerError, "Internal server error")
			}
		}()

		next.ServeHTTP(w, r)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func getRemoteIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		return forwarded
	}

	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	return r.RemoteAddr
}

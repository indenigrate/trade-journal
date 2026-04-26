package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type contextKey string

const TraceIDKey contextKey = "traceId"
const UserIDKey contextKey = "userId"

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.statusCode = 200
		rw.written = true
	}
	return rw.ResponseWriter.Write(b)
}

// Flush implements http.Flusher for SSE support.
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Logger creates zerolog middleware that injects traceId and logs every request.
func Logger(logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			traceID := uuid.New().String()
			ctx := context.WithValue(r.Context(), TraceIDKey, traceID)
			rw := &responseWriter{ResponseWriter: w, statusCode: 200}
			start := time.Now()

			next.ServeHTTP(rw, r.WithContext(ctx))

			userID := ""
			if uid, ok := ctx.Value(UserIDKey).(string); ok {
				userID = uid
			}

			logger.Info().
				Str("traceId", traceID).
				Str("userId", userID).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("latency", int(time.Since(start).Milliseconds())).
				Int("statusCode", rw.statusCode).
				Msg("")
		})
	}
}

// TraceIDFromContext retrieves the trace ID from context.
func TraceIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(TraceIDKey).(string); ok {
		return v
	}
	return ""
}

// UserIDFromContext retrieves the user ID from context.
func UserIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(UserIDKey).(string); ok {
		return v
	}
	return ""
}

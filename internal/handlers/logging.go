package handlers

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const maxBodyLog = 64 << 10 // 64 KiB

var redacted = map[string]struct{}{
	"Authorization": {},
	"Cookie":        {},
	"Set-Cookie":    {},
	"X-Api-Key":     {},
	"X-Auth-Token":  {},
	"X-Csrf-Token":  {},
}

type loggingRW struct {
	http.ResponseWriter
	status int
	bytes  int
	wrote  bool
}

func (w *loggingRW) WriteHeader(code int) {
	if !w.wrote {
		w.status = code
		w.wrote = true
	}
	w.ResponseWriter.WriteHeader(code)
}
func (w *loggingRW) Write(b []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

func sanitizeHeaders(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k, vals := range h {
		ck := http.CanonicalHeaderKey(k)
		if _, ok := redacted[ck]; ok {
			out[ck] = "***REDACTED***"
			continue
		}
		out[ck] = strings.Join(vals, ", ")
	}
	return out
}

func isTextLike(ct string) bool {
	ct = strings.ToLower(ct)
	return strings.Contains(ct, "json") ||
		strings.Contains(ct, "xml") ||
		strings.Contains(ct, "text") ||
		strings.Contains(ct, "form-urlencoded")
}

// LoggingMiddleware logs method, path, headers, (truncated) body, status, size, duration.
func LoggingMiddleware(log *logrus.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Read and restore request body
			var body []byte
			if r.Body != nil {
				body, _ = io.ReadAll(io.LimitReader(r.Body, maxBodyLog+1))
				_ = r.Body.Close()
				r.Body = io.NopCloser(bytes.NewReader(body)) // restore for handler
			}

			lrw := &loggingRW{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(lrw, r)

			duration := time.Since(start)

			fields := logrus.Fields{
				"component":      "http",
				"method":         r.Method,
				"path":           r.URL.Path,
				"query":          r.URL.RawQuery,
				"remote_addr":    r.RemoteAddr,
				"status_code":    lrw.status,
				"response_bytes": lrw.bytes,
				"duration_ms":    duration.Milliseconds(),
				"duration_ns":    duration.Nanoseconds(),
				"user_agent":     r.Header.Get("User-Agent"),
			}

			// Add request ID if available
			if requestID := r.Header.Get("X-Request-ID"); requestID != "" {
				fields["request_id"] = requestID
			}

			// Add user info if available from context
			// Note: Disabled to avoid import cycles. In production, you'd want to
			// extract user info from the context here for better traceability.

			// Only log headers in debug mode to avoid noise
			if log.Level >= logrus.DebugLevel {
				fields["request_headers"] = sanitizeHeaders(r.Header)
			}

			// Log request body for POST/PUT/PATCH requests
			if len(body) > 0 && (r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH") {
				if isTextLike(r.Header.Get("Content-Type")) {
					// truncate if over limit
					if len(body) > maxBodyLog {
						fields["body_truncated"] = true
						body = body[:maxBodyLog]
					}
					fields["request_body"] = string(body)
				} else {
					fields["request_body"] = "[binary or non-text body omitted]"
				}
			}

			// HTMX flag is often useful
			if r.Header.Get("HX-Request") == "true" {
				fields["htmx_request"] = true
			}

			// Choose log level based on status code
			var logLevel logrus.Level
			var message string

			switch {
			case lrw.status >= 500:
				logLevel = logrus.ErrorLevel
				message = "HTTP request completed with server error"
			case lrw.status >= 400:
				logLevel = logrus.WarnLevel
				message = "HTTP request completed with client error"
			case lrw.status >= 300:
				logLevel = logrus.InfoLevel
				message = "HTTP request completed with redirect"
			default:
				logLevel = logrus.InfoLevel
				message = "HTTP request completed successfully"
			}

			// Log slow requests as warnings
			if duration > 1*time.Second {
				logLevel = logrus.WarnLevel
				message = "Slow HTTP request completed"
				fields["slow_request"] = true
			}

			log.WithFields(fields).Log(logLevel, message)
		})
	}
}

// Helper function to get user from request context
func getUserFromRequest(r *http.Request) interface{} {
	// This is a simplified version - you'd need to import your middleware package
	// and use the proper context key. For now, we'll return nil to avoid import cycles.
	// In a real implementation, you'd do something like:
	// return middleware.GetUserFromContext(r.Context())
	return nil
}

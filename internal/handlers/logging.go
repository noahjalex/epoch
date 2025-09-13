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

			fields := logrus.Fields{
				"method":      r.Method,
				"path":        r.URL.Path,
				"query":       r.URL.RawQuery,
				"remote":      r.RemoteAddr,
				"status":      lrw.status,
				"bytes":       lrw.bytes,
				"duration_ms": time.Since(start).Milliseconds(),
				"headers":     sanitizeHeaders(r.Header),
			}
			if len(body) > 0 {
				if isTextLike(r.Header.Get("Content-Type")) {
					// truncate if over limit
					if len(body) > maxBodyLog {
						fields["body_truncated"] = true
						body = body[:maxBodyLog]
					}
					fields["body"] = string(body)
				} else {
					fields["body"] = "[binary or non-text body omitted]"
				}
			}
			// HTMX flag is often useful
			if r.Header.Get("HX-Request") == "true" {
				fields["htmx"] = true
			}
			log.WithFields(fields).Info("http request")
		})
	}
}

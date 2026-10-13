package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
)

// WithETag returns NotModified if the result still matches.
// Permission checks are applied, since the handler always runs.
func WithETag(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}

		bw := &bufferedWriter{ResponseWriter: w}
		next.ServeHTTP(bw, r)

		status := bw.status
		if status == 0 {
			status = http.StatusOK
		}

		if status == http.StatusOK {
			etag := bodyETag(bw.body.Bytes())

			h := w.Header()
			h.Set("ETag", etag)
			h.Set("Cache-Control", "private, no-cache")
			h.Add("Vary", "Authorization")

			if matchesETag(r.Header.Values("If-None-Match"), etag) {
				h.Del("Content-Type")
				h.Del("Content-Length")
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}

		w.WriteHeader(status)
		_, err := w.Write(bw.body.Bytes())
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to write response", slog.Any("error", err))
		}
	}
}

// bufferedWriter holds back status and body so the ETag can be computed before
// anything is sent.
type bufferedWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (w *bufferedWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *bufferedWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(b)
}

func bodyETag(body []byte) string {
	sum := sha256.Sum256(body)
	return `"` + hex.EncodeToString(sum[:16]) + `"`
}

// matchesETag compares weakly: a proxy that compresses the response might
// turn the ETag into a weak one (W/"...")
func matchesETag(ifNoneMatch []string, etag string) bool {
	for _, value := range ifNoneMatch {
		for candidate := range strings.SplitSeq(value, ",") {
			candidate = strings.TrimSpace(candidate)
			if candidate == "*" {
				return true
			}
			if strings.TrimPrefix(candidate, "W/") == etag {
				return true
			}
		}
	}
	return false
}

package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"git.dittmar.dev/robin/dttmr-api/internal/api/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func jsonHandler(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func serve(h http.HandlerFunc, method string, ifNoneMatch ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/", nil)
	for _, v := range ifNoneMatch {
		req.Header.Add("If-None-Match", v)
	}
	rr := httptest.NewRecorder()

	h(rr, req)

	return rr
}

func TestWithETag_FirstRequest(t *testing.T) {
	rr := serve(middleware.WithETag(jsonHandler(http.StatusOK, `[{"id":"1"}]`)), http.MethodGet)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, `[{"id":"1"}]`, rr.Body.String())
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	assert.Regexp(t, `^"[0-9a-f]{32}"$`, rr.Header().Get("ETag"))
	assert.Equal(t, "private, no-cache", rr.Header().Get("Cache-Control"))
	assert.Equal(t, "Authorization", rr.Header().Get("Vary"))
}

func TestWithETag_ETagDependsOnBodyOnly(t *testing.T) {
	first := serve(middleware.WithETag(jsonHandler(http.StatusOK, `[{"id":"1"}]`)), http.MethodGet)
	same := serve(middleware.WithETag(jsonHandler(http.StatusOK, `[{"id":"1"}]`)), http.MethodGet)
	other := serve(middleware.WithETag(jsonHandler(http.StatusOK, `[{"id":"2"}]`)), http.MethodGet)

	assert.Equal(t, first.Header().Get("ETag"), same.Header().Get("ETag"))
	assert.NotEqual(t, first.Header().Get("ETag"), other.Header().Get("ETag"))
}

func TestWithETag_IfNoneMatch(t *testing.T) {
	h := middleware.WithETag(jsonHandler(http.StatusOK, `[{"id":"1"}]`))
	etag := serve(h, http.MethodGet).Header().Get("ETag")
	require.NotEmpty(t, etag)

	tests := []struct {
		name        string
		ifNoneMatch []string
		want        int
	}{
		{name: "exact", ifNoneMatch: []string{etag}, want: http.StatusNotModified},
		{name: "weak", ifNoneMatch: []string{"W/" + etag}, want: http.StatusNotModified},
		{name: "wildcard", ifNoneMatch: []string{"*"}, want: http.StatusNotModified},
		{name: "in list", ifNoneMatch: []string{`"other", ` + etag}, want: http.StatusNotModified},
		{name: "in list without spaces", ifNoneMatch: []string{`"other",` + etag}, want: http.StatusNotModified},
		{name: "in second header line", ifNoneMatch: []string{`"other"`, etag}, want: http.StatusNotModified},
		{name: "surrounding whitespace", ifNoneMatch: []string{"  " + etag + " "}, want: http.StatusNotModified},
		{name: "absent", ifNoneMatch: nil, want: http.StatusOK},
		{name: "empty", ifNoneMatch: []string{""}, want: http.StatusOK},
		{name: "stale", ifNoneMatch: []string{`"00000000000000000000000000000000"`}, want: http.StatusOK},
		{name: "unquoted", ifNoneMatch: []string{etag[1 : len(etag)-1]}, want: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := serve(h, http.MethodGet, tt.ifNoneMatch...)

			assert.Equal(t, tt.want, rr.Code)
			assert.Equal(t, etag, rr.Header().Get("ETag"))
			if tt.want == http.StatusNotModified {
				assert.Empty(t, rr.Body.String())
			} else {
				assert.Equal(t, `[{"id":"1"}]`, rr.Body.String())
			}
		})
	}
}

func TestWithETag_NotModifiedHeaders(t *testing.T) {
	rr := serve(middleware.WithETag(jsonHandler(http.StatusOK, `[]`)), http.MethodGet, "*")

	require.Equal(t, http.StatusNotModified, rr.Code)
	assert.Empty(t, rr.Header().Get("Content-Type"))
	assert.Equal(t, "private, no-cache", rr.Header().Get("Cache-Control"))
	assert.Equal(t, "Authorization", rr.Header().Get("Vary"))
}

func TestWithETag_NonOKPassesThrough(t *testing.T) {
	statuses := []int{
		http.StatusCreated,
		http.StatusNoContent,
		http.StatusBadRequest,
		http.StatusForbidden,
		http.StatusInternalServerError,
	}

	for _, status := range statuses {
		t.Run(http.StatusText(status), func(t *testing.T) {
			rr := serve(middleware.WithETag(jsonHandler(status, `{"error":"x"}`)), http.MethodGet, "*")

			assert.Equal(t, status, rr.Code)
			assert.Equal(t, `{"error":"x"}`, rr.Body.String())
			assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
			assert.Empty(t, rr.Header().Get("ETag"))
			assert.Empty(t, rr.Header().Get("Cache-Control"))
			assert.Empty(t, rr.Header().Get("Vary"))
		})
	}
}

func TestWithETag_Methods(t *testing.T) {
	tests := []struct {
		method   string
		wantETag bool
	}{
		{method: http.MethodGet, wantETag: true},
		{method: http.MethodHead, wantETag: true},
		{method: http.MethodPost, wantETag: false},
		{method: http.MethodPut, wantETag: false},
		{method: http.MethodDelete, wantETag: false},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			rr := serve(middleware.WithETag(jsonHandler(http.StatusOK, `{}`)), tt.method, "*")

			if tt.wantETag {
				assert.Equal(t, http.StatusNotModified, rr.Code)
				assert.NotEmpty(t, rr.Header().Get("ETag"))
			} else {
				assert.Equal(t, http.StatusOK, rr.Code)
				assert.Equal(t, `{}`, rr.Body.String())
				assert.Empty(t, rr.Header().Get("ETag"))
			}
		})
	}
}

func TestWithETag_ImplicitStatus(t *testing.T) {
	h := middleware.WithETag(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	})

	rr := serve(h, http.MethodGet)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, `{}`, rr.Body.String())
	assert.NotEmpty(t, rr.Header().Get("ETag"))
}

func TestWithETag_NothingWritten(t *testing.T) {
	h := middleware.WithETag(func(w http.ResponseWriter, r *http.Request) {})

	rr := serve(h, http.MethodGet)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Empty(t, rr.Body.String())
	assert.NotEmpty(t, rr.Header().Get("ETag"))
}

func TestWithETag_FirstStatusWins(t *testing.T) {
	h := middleware.WithETag(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.WriteHeader(http.StatusOK)
	})

	rr := serve(h, http.MethodGet)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Empty(t, rr.Header().Get("ETag"))
}

func TestWithETag_BodyWrittenInParts(t *testing.T) {
	whole := serve(middleware.WithETag(jsonHandler(http.StatusOK, `[{"id":"1"}]`)), http.MethodGet)
	parts := serve(middleware.WithETag(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id"`))
		_, _ = w.Write([]byte(`:"1"}]`))
	}), http.MethodGet)

	assert.Equal(t, `[{"id":"1"}]`, parts.Body.String())
	assert.Equal(t, whole.Header().Get("ETag"), parts.Header().Get("ETag"))
}

func TestWithETag_KeepsExistingVary(t *testing.T) {
	h := middleware.WithETag(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Vary", "Accept-Encoding")
		_, _ = w.Write([]byte(`{}`))
	})

	rr := serve(h, http.MethodGet)

	assert.Equal(t, []string{"Accept-Encoding", "Authorization"}, rr.Header().Values("Vary"))
}

func TestWithETag_HandlerRunsOnce(t *testing.T) {
	calls := 0
	h := middleware.WithETag(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{}`))
	})

	serve(h, http.MethodGet, "*")

	assert.Equal(t, 1, calls)
}

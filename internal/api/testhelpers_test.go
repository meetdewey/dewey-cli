package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeAPI captures requests and dispatches to a router map keyed by
// "METHOD path". Path matching is exact unless the registered key ends in "*".
type fakeAPI struct {
	t        *testing.T
	server   *httptest.Server
	handlers map[string]http.HandlerFunc
	mu       chan struct{}
	requests []recordedRequest
}

type recordedRequest struct {
	Method      string
	Path        string
	RawQuery    string
	Body        []byte
	ContentType string
	Headers     http.Header
}

func newFakeAPI(t *testing.T) *fakeAPI {
	t.Helper()
	f := &fakeAPI{
		t:        t,
		handlers: make(map[string]http.HandlerFunc),
		mu:       make(chan struct{}, 1),
	}
	f.server = httptest.NewServer(http.HandlerFunc(f.dispatch))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeAPI) Handle(methodAndPath string, h http.HandlerFunc) {
	f.handlers[methodAndPath] = h
}

func (f *fakeAPI) HandleJSON(methodAndPath string, status int, payload any) {
	f.Handle(methodAndPath, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if payload != nil {
			_ = json.NewEncoder(w).Encode(payload)
		}
	})
}

func (f *fakeAPI) Client() *Client {
	return New("test-key", f.server.URL, "test-agent")
}

func (f *fakeAPI) Requests() []recordedRequest {
	f.mu <- struct{}{}
	defer func() { <-f.mu }()
	out := make([]recordedRequest, len(f.requests))
	copy(out, f.requests)
	return out
}

// LastRequest returns the most recent recorded request, failing the test if
// none were recorded.
func (f *fakeAPI) LastRequest() recordedRequest {
	reqs := f.Requests()
	if len(reqs) == 0 {
		f.t.Fatalf("no requests recorded")
	}
	return reqs[len(reqs)-1]
}

func (f *fakeAPI) dispatch(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	rec := recordedRequest{
		Method:      r.Method,
		Path:        r.URL.Path,
		RawQuery:    r.URL.RawQuery,
		Body:        body,
		ContentType: r.Header.Get("Content-Type"),
		Headers:     r.Header.Clone(),
	}

	f.mu <- struct{}{}
	f.requests = append(f.requests, rec)
	<-f.mu

	// Auth via either Bearer header or ?key=<token> query param (used by the
	// document-events SSE stream because EventSource can't set headers).
	hasBearer := r.Header.Get("Authorization") == "Bearer test-key"
	hasKeyParam := r.URL.Query().Get("key") == "test-key"
	if !hasBearer && !hasKeyParam {
		http.Error(w, "missing/invalid bearer", http.StatusUnauthorized)
		return
	}

	// Replace the body with a fresh reader so handlers can re-read it.
	r.Body = io.NopCloser(strings.NewReader(string(body)))

	key := r.Method + " " + r.URL.Path
	if h, ok := f.handlers[key]; ok {
		h(w, r)
		return
	}
	// Wildcard handler: matches "METHOD prefix*"
	for k, h := range f.handlers {
		if !strings.HasSuffix(k, "*") {
			continue
		}
		prefix := strings.TrimSuffix(k, "*")
		if r.Method+" "+r.URL.Path != "" && strings.HasPrefix(r.Method+" "+r.URL.Path, prefix) {
			h(w, r)
			return
		}
	}

	http.Error(w, "no handler for "+key, http.StatusNotFound)
}

// jsonBody decodes the body of the last request as JSON.
func jsonBody(t *testing.T, rec recordedRequest, dst any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body, dst); err != nil {
		t.Fatalf("decode body: %v\nbody: %s", err, string(rec.Body))
	}
}

func mustEqual[T comparable](t *testing.T, got, want T, label string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", label, got, want)
	}
}

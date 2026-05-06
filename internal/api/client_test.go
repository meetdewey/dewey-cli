package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestClient_New_TrimsTrailingSlashes(t *testing.T) {
	c := New("k", "https://example.com/v1//", "ua")
	if c.BaseURL != "https://example.com/v1" {
		t.Errorf("base url not trimmed: %q", c.BaseURL)
	}
}

func TestClient_New_DefaultBaseURL(t *testing.T) {
	c := New("k", "", "ua")
	if c.BaseURL != DefaultBaseURL {
		t.Errorf("default base url not used: %q", c.BaseURL)
	}
}

func TestError_Error_FormatsCodeAndStatus(t *testing.T) {
	e := &Error{Status: 429, Message: "too many", Code: "RATE_LIMITED"}
	got := e.Error()
	if !strings.Contains(got, "429") || !strings.Contains(got, "RATE_LIMITED") || !strings.Contains(got, "too many") {
		t.Errorf("unexpected formatting: %q", got)
	}
	noCode := &Error{Status: 500, Message: "boom"}
	if !strings.Contains(noCode.Error(), "500") || !strings.Contains(noCode.Error(), "boom") {
		t.Errorf("no-code formatting: %q", noCode.Error())
	}
}

func TestClient_Do_AttachesAuthAndUserAgent(t *testing.T) {
	api := newFakeAPI(t)
	api.HandleJSON("GET /collections", 200, []Collection{})
	c := api.Client()
	if _, err := c.ListCollections(context.Background()); err != nil {
		t.Fatal(err)
	}
	rec := api.LastRequest()
	mustEqual(t, rec.Headers.Get("Authorization"), "Bearer test-key", "auth header")
	mustEqual(t, rec.Headers.Get("User-Agent"), "test-agent", "user-agent")
}

func TestClient_Do_ParsesAPIError(t *testing.T) {
	api := newFakeAPI(t)
	api.Handle("GET /collections", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(429)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"message": "rate limited",
			"code":    "RATE_LIMITED",
		})
	})

	_, err := api.Client().ListCollections(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("not an *api.Error: %v", err)
	}
	mustEqual(t, apiErr.Status, 429, "status")
	mustEqual(t, apiErr.Message, "rate limited", "message")
	mustEqual(t, apiErr.Code, "RATE_LIMITED", "code")
}

func TestClient_Do_FallsBackToErrorField(t *testing.T) {
	api := newFakeAPI(t)
	api.Handle("GET /collections", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	})
	_, err := api.Client().ListCollections(context.Background())
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Message != "not found" {
		t.Fatalf("expected 'not found' message, got %v", err)
	}
}

func TestClient_Do_NonJSONErrorFallsBackToBody(t *testing.T) {
	api := newFakeAPI(t)
	api.Handle("GET /collections", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(503)
		_, _ = w.Write([]byte("upstream is dead"))
	})
	_, err := api.Client().ListCollections(context.Background())
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("not an *api.Error: %v", err)
	}
	if apiErr.Status != 503 || apiErr.Message != "upstream is dead" {
		t.Errorf("unexpected: status=%d message=%q", apiErr.Status, apiErr.Message)
	}
}

func TestClient_Do_NoContent(t *testing.T) {
	api := newFakeAPI(t)
	api.Handle("DELETE /collections/abc", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	if err := api.Client().DeleteCollection(context.Background(), "abc"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestClient_Do_TextContentType(t *testing.T) {
	api := newFakeAPI(t)
	api.Handle("GET /documents/d1/markdown", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		_, _ = w.Write([]byte("# hello"))
	})
	md, err := api.Client().GetDocumentMarkdown(context.Background(), "d1")
	if err != nil {
		t.Fatal(err)
	}
	if md != "# hello" {
		t.Errorf("got %q", md)
	}
}

func TestClient_Ping_Success(t *testing.T) {
	api := newFakeAPI(t)
	api.HandleJSON("GET /collections", 200, []Collection{
		{ID: "1", Name: "a"},
		{ID: "2", Name: "b"},
	})
	res, err := api.Client().Ping(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != 200 || res.Collections != 2 {
		t.Errorf("unexpected ping: %+v", res)
	}
}

func TestClient_Ping_Error(t *testing.T) {
	api := newFakeAPI(t)
	api.Handle("GET /collections", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"message":"unauthorized"}`))
	})
	res, err := api.Client().Ping(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if res == nil || res.Status != 401 {
		t.Errorf("expected status 401, got %+v", res)
	}
}

package cmd

import (
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lambdabaa/dewey/apps/cli/internal/api"
)

func TestUpload_Multipart_SmallFile(t *testing.T) {
	h := newHarness(t)
	h.HandleJSON("GET /collections", 200, []api.Collection{{ID: "c1", Name: "papers"}})
	h.Handle("POST /collections/c1/documents", func(w http.ResponseWriter, r *http.Request) {
		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
			t.Errorf("expected multipart, got %s", mediaType)
		}
		mr := multipart.NewReader(r.Body, params["boundary"])
		var filename string
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			if p.FormName() == "file" {
				filename = p.FileName()
				_, _ = io.Copy(io.Discard, p)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.Document{
			ID: "d1", Filename: filename, Status: "uploading",
		})
	})

	tmp := t.TempDir()
	path := filepath.Join(tmp, "small.txt")
	if err := os.WriteFile(path, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	if h.run("upload", path, "-c", "papers") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	if !strings.Contains(h.stderr.String(), "uploaded") {
		t.Errorf("expected status line: %q", h.stderr.String())
	}
	// Last collection should be remembered.
	if h.state.LastCollection != "c1" {
		t.Errorf("last_collection not set: %q", h.state.LastCollection)
	}
}

func TestUpload_GlobExpansion(t *testing.T) {
	h := newHarness(t)
	h.HandleJSON("GET /collections", 200, []api.Collection{{ID: "c1", Name: "papers"}})

	var calls int32
	h.Handle("POST /collections/c1/documents", func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.Document{ID: "d", Status: "uploading"})
	})

	tmp := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		_ = os.WriteFile(filepath.Join(tmp, name), []byte("x"), 0o644)
	}
	pattern := filepath.Join(tmp, "*.txt")
	if h.run("upload", pattern, "-c", "papers") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	if atomic.LoadInt32(&calls) != 3 {
		t.Errorf("expected 3 upload calls, got %d", calls)
	}
}

func TestUpload_NoMatchesIsError(t *testing.T) {
	h := newHarness(t)
	h.HandleJSON("GET /collections", 200, []api.Collection{{ID: "c1", Name: "papers"}})
	tmp := t.TempDir()
	pattern := filepath.Join(tmp, "*.never")
	if h.run("upload", pattern, "-c", "papers") == ExitOK {
		t.Errorf("expected non-zero exit for no matches")
	}
}

func TestUpload_PresignThenConfirm(t *testing.T) {
	// Stand up a fake S3 server.
	var s3Hits int32
	s3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("S3 expected PUT got %s", r.Method)
		}
		atomic.AddInt32(&s3Hits, 1)
		w.WriteHeader(200)
	}))
	t.Cleanup(s3.Close)

	h := newHarness(t)
	h.HandleJSON("GET /collections", 200, []api.Collection{{ID: "c1", Name: "papers"}})
	h.Handle("POST /collections/c1/documents/upload-url", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		uploadURL := s3.URL + "/upload"
		_ = json.NewEncoder(w).Encode(map[string]any{
			"documentId": "d-presign",
			"uploadUrl":  uploadURL,
		})
	})
	h.Handle("POST /collections/c1/documents/d-presign/confirm", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.Document{ID: "d-presign", Status: "processing"})
	})

	tmp := t.TempDir()
	path := filepath.Join(tmp, "big.bin")
	data := make([]byte, 5*1024*1024)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if h.run("upload", path, "-c", "papers") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	if atomic.LoadInt32(&s3Hits) != 1 {
		t.Errorf("expected one S3 PUT, got %d", s3Hits)
	}
}

func TestUpload_TagsAndMetadataPassedThrough(t *testing.T) {
	h := newHarness(t)
	h.HandleJSON("GET /collections", 200, []api.Collection{{ID: "c1", Name: "papers"}})

	var receivedTags, receivedMeta string
	h.Handle("POST /collections/c1/documents", func(w http.ResponseWriter, r *http.Request) {
		_, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		mr := multipart.NewReader(r.Body, params["boundary"])
		for {
			p, err := mr.NextPart()
			if err != nil {
				break
			}
			data, _ := io.ReadAll(p)
			switch p.FormName() {
			case "tags":
				receivedTags = string(data)
			case "metadata":
				receivedMeta = string(data)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.Document{ID: "d1", Status: "uploading"})
	})

	tmp := t.TempDir()
	path := filepath.Join(tmp, "x.txt")
	_ = os.WriteFile(path, []byte("hi"), 0o644)

	exit := h.run(
		"upload", path, "-c", "papers",
		"--tag", "research",
		"--tag", "archive",
		"--metadata", "author=ari",
		"--metadata", `year=2026`,
	)
	if exit != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	if !strings.Contains(receivedTags, "research") || !strings.Contains(receivedTags, "archive") {
		t.Errorf("tags not forwarded: %q", receivedTags)
	}
	if !strings.Contains(receivedMeta, `"author":"ari"`) {
		t.Errorf("metadata not forwarded: %q", receivedMeta)
	}
	// year=2026 should parse as JSON number.
	if !strings.Contains(receivedMeta, `"year":2026`) {
		t.Errorf("numeric metadata not parsed: %q", receivedMeta)
	}
}

func TestParseKVPairs_RejectsInvalid(t *testing.T) {
	_, err := parseKVPairs([]string{"oops"})
	if err == nil {
		t.Error("expected error for bad pair")
	}
}

func TestParseKVPairs_ParsesJSONValue(t *testing.T) {
	out, err := parseKVPairs([]string{`tags=["a","b"]`, `count=3`, `name=ari`})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out["tags"].([]any); !ok {
		t.Errorf("tags not array: %T", out["tags"])
	}
	if out["count"] != float64(3) {
		t.Errorf("count = %v", out["count"])
	}
	if out["name"] != "ari" {
		t.Errorf("name = %v", out["name"])
	}
}

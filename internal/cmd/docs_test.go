package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/lambdabaa/dewey/apps/cli/internal/api"
)

func TestDocs_List(t *testing.T) {
	h := newHarness(t)
	h.HandleJSON("GET /collections", 200, []api.Collection{{ID: "c1", Name: "papers"}})
	h.HandleJSON("GET /collections/c1/documents", 200, api.DocumentList{
		Documents: []api.Document{{ID: "d1", Filename: "a.pdf", Status: "ready"}},
		Total:     1,
	})
	if h.run("docs", "list", "papers") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	if !strings.Contains(h.stdout.String(), "a.pdf") {
		t.Errorf("missing filename: %q", h.stdout.String())
	}
}

func TestDocs_Get(t *testing.T) {
	h := newHarness(t)
	size := int64(1024)
	h.HandleJSON("GET /documents/d1", 200, api.Document{
		ID: "d1", Filename: "a.pdf", Status: "ready", FileSizeBytes: &size,
	})
	if h.run("docs", "get", "d1") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	if !strings.Contains(h.stdout.String(), "Filename: a.pdf") {
		t.Errorf("got: %q", h.stdout.String())
	}
	if !strings.Contains(h.stdout.String(), "Size: 1.0 KB") {
		t.Errorf("expected human-formatted size: %q", h.stdout.String())
	}
}

func TestDocs_Markdown(t *testing.T) {
	h := newHarness(t)
	h.Handle("GET /documents/d1/markdown", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		_, _ = w.Write([]byte("# Title\n\nbody"))
	})
	if h.run("docs", "markdown", "d1") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	if !strings.Contains(h.stdout.String(), "# Title") {
		t.Errorf("expected markdown on stdout: %q", h.stdout.String())
	}
}

func TestDocs_Markdown_JSON(t *testing.T) {
	h := newHarness(t)
	h.Handle("GET /documents/d1/markdown", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		_, _ = w.Write([]byte("# Title"))
	})
	if h.run("--json", "docs", "markdown", "d1") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	var out map[string]string
	if err := json.Unmarshal(h.stdout.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, h.stdout.String())
	}
	if out["documentId"] != "d1" || !strings.Contains(out["markdown"], "# Title") {
		t.Errorf("unexpected: %+v", out)
	}
}

func TestDocs_Sections(t *testing.T) {
	h := newHarness(t)
	h.HandleJSON("GET /documents/d1/sections", 200, []api.Section{
		{ID: "s1", Title: "Intro", Level: 1, Position: 0, ChunkCount: 3},
	})
	if h.run("docs", "sections", "d1") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	if !strings.Contains(h.stdout.String(), "Intro") {
		t.Errorf("missing title: %q", h.stdout.String())
	}
}

func TestDocs_Chunks(t *testing.T) {
	h := newHarness(t)
	h.HandleJSON("GET /sections/s1/chunks", 200, []api.Chunk{
		{ID: "ch1", Position: 0, TokenCount: 42, Content: "hello"},
	})
	if h.run("docs", "chunks", "s1") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	if !strings.Contains(h.stdout.String(), "hello") {
		t.Errorf("missing content: %q", h.stdout.String())
	}
}

func TestDocs_Delete(t *testing.T) {
	h := newHarness(t)
	h.Handle("DELETE /documents/d1", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	if h.run("docs", "delete", "d1") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	if !strings.Contains(h.stderr.String(), "Deleted") {
		t.Errorf("missing confirmation: %q", h.stderr.String())
	}
}

func TestDocs_Wait(t *testing.T) {
	h := newHarness(t)
	h.HandleJSON("GET /documents/d1/wait", 200, api.Document{ID: "d1", Status: "ready"})
	if h.run("docs", "wait", "d1") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
}

package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/lambdabaa/dewey/apps/cli/internal/api"
)

func TestCollections_List_Human(t *testing.T) {
	h := newHarness(t)
	h.HandleJSON("GET /collections", 200, []api.Collection{
		{ID: "c1", Name: "papers", Visibility: "private", CreatedAt: "2026-01-01"},
	})

	exit := h.run("collections", "list")
	if exit != ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, h.stderr.String())
	}
	if !strings.Contains(h.stdout.String(), "papers") {
		t.Errorf("expected name on stdout: %q", h.stdout.String())
	}
	if !strings.Contains(h.stdout.String(), "ID") {
		t.Errorf("expected header on stdout: %q", h.stdout.String())
	}
}

func TestCollections_List_JSON(t *testing.T) {
	h := newHarness(t)
	h.HandleJSON("GET /collections", 200, []api.Collection{{ID: "c1", Name: "papers"}})

	exit := h.run("--json", "collections", "list")
	if exit != ExitOK {
		t.Fatalf("exit=%d", exit)
	}
	var out []map[string]any
	if err := json.Unmarshal(h.stdout.Bytes(), &out); err != nil {
		t.Fatalf("stdout was not valid JSON: %v\n%s", err, h.stdout.String())
	}
	if len(out) != 1 || out[0]["id"] != "c1" {
		t.Errorf("unexpected JSON shape: %v", out)
	}
}

func TestCollections_List_EmptyShowsHint(t *testing.T) {
	h := newHarness(t)
	h.HandleJSON("GET /collections", 200, []api.Collection{})
	if h.run("collections", "list") != ExitOK {
		t.Fatal("expected success")
	}
	if !strings.Contains(h.stderr.String(), "Create one with") {
		t.Errorf("expected hint on stderr: %q", h.stderr.String())
	}
	if h.stdout.String() != "" {
		t.Errorf("stdout should be empty, got: %q", h.stdout.String())
	}
}

func TestCollections_Get_ByName(t *testing.T) {
	h := newHarness(t)
	// resolveCollection finds the collection in the list by name and returns
	// that record directly — so the list response must carry full fields.
	h.HandleJSON("GET /collections", 200, []api.Collection{
		{ID: "c1", Name: "papers", EmbeddingModel: "text-embedding-3-small", ChunkSize: 512, ChunkOverlap: 64},
	})

	if h.run("collections", "get", "papers") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	if !strings.Contains(h.stdout.String(), "Embedding model: text-embedding-3-small") {
		t.Errorf("expected key/value: %q", h.stdout.String())
	}
}

func TestCollections_Create_PersistsLastCollection(t *testing.T) {
	h := newHarness(t)
	h.Handle("POST /collections", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.Collection{ID: "col_new", Name: "papers"})
	})

	if h.run("collections", "create", "papers") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	if h.state.LastCollection != "col_new" {
		t.Errorf("expected last_collection=col_new, got %q", h.state.LastCollection)
	}
	if !strings.Contains(h.stderr.String(), "Created") {
		t.Errorf("expected status on stderr: %q", h.stderr.String())
	}
}

func TestCollections_Update_OnlySendsChangedFields(t *testing.T) {
	h := newHarness(t)
	h.HandleJSON("GET /collections", 200, []api.Collection{{ID: "c1", Name: "papers"}})
	h.Handle("PATCH /collections/c1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.Collection{ID: "c1", Name: "renamed"})
	})

	if h.run("collections", "update", "papers", "--name", "renamed") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	body := string(h.lastRequest().Body)
	if !strings.Contains(body, `"name":"renamed"`) {
		t.Errorf("body missing name: %s", body)
	}
	if strings.Contains(body, "visibility") {
		t.Errorf("visibility unexpectedly sent: %s", body)
	}
}

func TestCollections_Delete(t *testing.T) {
	h := newHarness(t)
	h.HandleJSON("GET /collections", 200, []api.Collection{{ID: "c1", Name: "papers"}})
	h.Handle("DELETE /collections/c1", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	if h.run("collections", "delete", "papers") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	if !strings.Contains(h.stderr.String(), "Deleted") {
		t.Errorf("expected confirmation: %q", h.stderr.String())
	}
}

func TestCollections_Stats(t *testing.T) {
	h := newHarness(t)
	h.HandleJSON("GET /collections", 200, []api.Collection{{ID: "c1", Name: "papers"}})
	h.HandleJSON("GET /collections/c1/stats", 200, api.CollectionStats{
		DocCount: 42, TotalChunks: 1000, TotalSections: 200, TotalClaimsCount: 50,
	})
	if h.run("collections", "stats", "papers") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	for _, want := range []string{"Documents: 42", "Chunks: 1000", "Sections: 200", "Claims: 50"} {
		if !strings.Contains(h.stdout.String(), want) {
			t.Errorf("missing %q in stdout: %q", want, h.stdout.String())
		}
	}
}

func TestCollections_Get_404_ReturnsInputError(t *testing.T) {
	h := newHarness(t)
	h.Handle("GET /collections/missing", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	})
	exit := h.run("collections", "get", "col_missing")
	if exit != ExitInputError {
		t.Errorf("expected exit=%d, got %d", ExitInputError, exit)
	}
}

func TestRoot_MissingAPIKey_ReturnsAuthExitCode(t *testing.T) {
	h := newHarness(t)
	t.Setenv("DEWEY_API_KEY", "")
	exit := h.run("collections", "list")
	if exit != ExitNoPermission {
		t.Errorf("expected %d, got %d (stderr=%q)", ExitNoPermission, exit, h.stderr.String())
	}
}

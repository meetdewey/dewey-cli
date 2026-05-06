package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/lambdabaa/dewey/apps/cli/internal/api"
)

func TestQuery_Human(t *testing.T) {
	h := newHarness(t)
	h.HandleJSON("GET /collections", 200, []api.Collection{{ID: "c1", Name: "papers"}})
	h.Handle("POST /collections/c1/query", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		mustEqualS(t, body["q"].(string), "what is X?", "q")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]api.RetrievalResult{
			{
				Score: 0.85,
				Chunk: struct {
					ID         string `json:"id"`
					Content    string `json:"content"`
					Position   int    `json:"position"`
					TokenCount int    `json:"tokenCount"`
				}{Content: "the answer is …"},
				Document: struct {
					ID       string `json:"id"`
					Filename string `json:"filename"`
				}{Filename: "paper.pdf"},
			},
		})
	})

	if h.run("query", "papers", "what is X?", "--top-k", "5") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	if !strings.Contains(h.stdout.String(), "paper.pdf") {
		t.Errorf("expected filename: %q", h.stdout.String())
	}
}

func TestQuery_NoResultsHint(t *testing.T) {
	h := newHarness(t)
	h.HandleJSON("GET /collections", 200, []api.Collection{{ID: "c1", Name: "papers"}})
	h.HandleJSON("POST /collections/c1/query", 200, []api.RetrievalResult{})
	if h.run("query", "papers", "needle") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	if !strings.Contains(h.stderr.String(), "No results") {
		t.Errorf("expected 'No results' on stderr: %q", h.stderr.String())
	}
}

func TestQuery_UsesLastCollection(t *testing.T) {
	const uuid = "11111111-1111-1111-1111-111111111111"
	h := newHarness(t)
	h.HandleJSON("GET /collections/"+uuid, 200, api.Collection{ID: uuid, Name: "papers"})
	h.HandleJSON("POST /collections/"+uuid+"/query", 200, []api.RetrievalResult{})
	h.HandleJSON("POST /collections", 200, api.Collection{ID: uuid, Name: "papers"})

	// Populate state via `collections create`.
	if h.run("collections", "create", "papers", "--project-id", "proj_test") != ExitOK {
		t.Fatal(h.stderr.String())
	}
	if h.state.LastCollection != uuid {
		t.Fatalf("setup: expected LastCollection=%s, got %q", uuid, h.state.LastCollection)
	}

	h.stdout.Reset()
	h.stderr.Reset()
	if h.run("query", "needle") != ExitOK {
		t.Fatalf("query without -c failed: %s", h.stderr.String())
	}
}

func TestScan_Human(t *testing.T) {
	h := newHarness(t)
	h.HandleJSON("GET /collections", 200, []api.Collection{{ID: "c1", Name: "papers"}})
	h.HandleJSON("POST /collections/c1/sections/scan", 200, api.SectionScanResponse{
		Results: []api.SectionScanResult{},
	})
	if h.run("scan", "papers", "needle") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	if !strings.Contains(h.stderr.String(), "No matching sections") {
		t.Errorf("expected empty message: %q", h.stderr.String())
	}
}

func TestResearch_NoStream_Sync(t *testing.T) {
	h := newHarness(t)
	h.HandleJSON("GET /collections", 200, []api.Collection{{ID: "c1", Name: "papers"}})
	h.Handle("POST /collections/c1/research/sync", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		mustEqualS(t, body["q"].(string), "explain", "q")
		mustEqualS(t, body["depth"].(string), "deep", "depth")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.ResearchResult{
			Answer:    "the answer is 42.",
			SessionID: "sess1",
			Sources:   []api.ResearchSource{{Filename: "paper.pdf", SectionTitle: "Conclusion"}},
		})
	})

	exit := h.run("research", "papers", "explain", "--depth", "deep", "--no-stream")
	if exit != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	if !strings.Contains(h.stdout.String(), "the answer is 42") {
		t.Errorf("expected answer on stdout: %q", h.stdout.String())
	}
	if !strings.Contains(h.stdout.String(), "paper.pdf § Conclusion") {
		t.Errorf("expected citation: %q", h.stdout.String())
	}
}

func TestResearch_Streaming(t *testing.T) {
	h := newHarness(t)
	h.HandleJSON("GET /collections", 200, []api.Collection{{ID: "c1", Name: "papers"}})
	h.Handle("POST /collections/c1/research", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fl, _ := w.(http.Flusher)
		writeFrame := func(s string) {
			_, _ = w.Write([]byte("data: " + s + "\n\n"))
			if fl != nil {
				fl.Flush()
			}
		}
		writeFrame(`{"type":"chunk","content":"hello "}`)
		writeFrame(`{"type":"chunk","content":"world"}`)
		writeFrame(`{"type":"done","sources":[{"filename":"a.pdf","sectionTitle":"Intro"}]}`)
	})

	exit := h.run("research", "papers", "explain", "--stream")
	if exit != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	out := h.stdout.String()
	if !strings.Contains(out, "hello world") {
		t.Errorf("expected streamed chunks concatenated: %q", out)
	}
	if !strings.Contains(out, "a.pdf § Intro") {
		t.Errorf("expected citation: %q", out)
	}
}

func TestResearch_StreamingJSON_NDJSON(t *testing.T) {
	h := newHarness(t)
	h.HandleJSON("GET /collections", 200, []api.Collection{{ID: "c1", Name: "papers"}})
	h.Handle("POST /collections/c1/research", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fl, _ := w.(http.Flusher)
		write := func(s string) {
			_, _ = w.Write([]byte("data: " + s + "\n\n"))
			if fl != nil {
				fl.Flush()
			}
		}
		write(`{"type":"chunk","content":"hi"}`)
		write(`{"type":"done","sources":[]}`)
	})

	if h.run("--json", "research", "papers", "explain", "--stream") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	lines := strings.Split(strings.TrimRight(h.stdout.String(), "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected NDJSON, got: %q", h.stdout.String())
	}
	for _, line := range lines {
		var v map[string]any
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			t.Errorf("non-JSON NDJSON line: %q", line)
		}
	}
}

// Local mustEqual that doesn't conflict with the api package's helper.
func mustEqualS(t *testing.T, got, want, label string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %q, want %q", label, got, want)
	}
}

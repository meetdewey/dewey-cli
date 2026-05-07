package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/lambdabaa/dewey/apps/cli/internal/api"
)

func TestAgents_InvokeSync_NonTTY(t *testing.T) {
	h := newHarness(t)
	h.Handle("POST /orgs/org-1/projects/proj-2/agents/qa-test/invoke/sync", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		mustEqualS(t, body["query"], "why?", "query")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.AgentInvokeResult{
			RunID:    "run-1",
			Response: "Polio cases dropped.",
			Status:   "succeeded",
			Sources: []api.AgentSource{
				{Filename: "paper.pdf", SectionTitle: "Methods"},
			},
		})
	})

	// TTY=false in the harness, so streaming is off by default — sync path runs.
	if h.run("agents", "invoke", "qa-test", "why?", "--org", "org-1", "--project", "proj-2") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	out := h.stdout.String()
	if !strings.Contains(out, "Polio cases dropped.") {
		t.Errorf("missing response: %q", out)
	}
	if !strings.Contains(out, "paper.pdf") {
		t.Errorf("missing source: %q", out)
	}
}

func TestAgents_InvokeStream(t *testing.T) {
	h := newHarness(t)
	h.Handle("POST /orgs/org-1/projects/proj-2/agents/qa-test/invoke", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		flusher, _ := w.(http.Flusher)
		writeEv := func(ev any) {
			b, _ := json.Marshal(ev)
			fmt.Fprintf(w, "data: %s\n\n", string(b))
			if flusher != nil {
				flusher.Flush()
			}
		}
		writeEv(map[string]any{"type": "run_started", "runId": "run-1"})
		writeEv(map[string]any{"type": "chunk", "content": "Polio cases "})
		writeEv(map[string]any{"type": "chunk", "content": "dropped."})
		writeEv(map[string]any{
			"type":     "done",
			"status":   "succeeded",
			"response": "Polio cases dropped.",
			"sources": []map[string]any{
				{"filename": "paper.pdf", "sectionTitle": "Methods"},
			},
		})
	})

	if h.run("agents", "invoke", "qa-test", "why?",
		"--org", "org-1", "--project", "proj-2", "--stream") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	out := h.stdout.String()
	if !strings.Contains(out, "Polio cases dropped.") {
		t.Errorf("expected streamed text, got: %q", out)
	}
	if !strings.Contains(out, "paper.pdf") {
		t.Errorf("expected source line, got: %q", out)
	}
}

func TestAgents_Invoke_RequiresOrg(t *testing.T) {
	h := newHarness(t)
	if h.run("agents", "invoke", "qa-test", "why?", "--project", "proj-2") == ExitOK {
		t.Fatalf("expected failure when org missing")
	}
	if !strings.Contains(h.stderr.String(), "org ID required") {
		t.Errorf("missing org-required hint: %q", h.stderr.String())
	}
}

func TestAgents_Invoke_RequiresProject(t *testing.T) {
	h := newHarness(t)
	if h.run("agents", "invoke", "qa-test", "why?", "--org", "org-1") == ExitOK {
		t.Fatalf("expected failure when project missing")
	}
	if !strings.Contains(h.stderr.String(), "project ID required") {
		t.Errorf("missing project-required hint: %q", h.stderr.String())
	}
}

func TestAgents_Invoke_UsesProjectFlagP(t *testing.T) {
	h := newHarness(t)
	h.HandleJSON("POST /orgs/org-1/projects/proj-2/agents/qa-test/invoke/sync", 200, api.AgentInvokeResult{
		RunID: "run-1", Response: "ok", Status: "succeeded",
	})
	if h.run("agents", "invoke", "qa-test", "why?", "--org", "org-1", "-p", "proj-2") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
}

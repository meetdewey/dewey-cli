package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestInvokeAgentSync(t *testing.T) {
	api := newFakeAPI(t)
	api.Handle("POST /orgs/org-1/projects/proj-2/agents/qa-test/invoke/sync", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		mustEqual(t, body["query"], "why?", "query")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(AgentInvokeResult{
			RunID:    "run-1",
			Response: "Polio cases dropped [1].",
			Status:   "succeeded",
			Sources: []AgentSource{
				{ChunkID: "c1", SectionTitle: "Methods", Filename: "paper.pdf", Score: 0.83},
			},
		})
	})

	res, err := api.Client().InvokeAgentSync(context.Background(), "org-1", "proj-2", "qa-test", "why?")
	if err != nil {
		t.Fatal(err)
	}
	if res.RunID != "run-1" || res.Status != "succeeded" {
		t.Errorf("unexpected: %+v", res)
	}
	if len(res.Sources) != 1 || res.Sources[0].Filename != "paper.pdf" {
		t.Errorf("sources: %+v", res.Sources)
	}
}

func TestStreamAgent(t *testing.T) {
	api := newFakeAPI(t)
	api.Handle("POST /orgs/org-1/projects/proj-2/agents/qa-test/invoke", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		mustEqual(t, body["query"], "why?", "query")
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
		writeEv(map[string]any{"type": "tool_call", "tool": "search_collection", "args": map[string]any{"query": "polio"}, "stepIndex": 0})
		writeEv(map[string]any{"type": "chunk", "content": "Polio "})
		writeEv(map[string]any{"type": "chunk", "content": "cases dropped."})
		writeEv(map[string]any{
			"type":     "done",
			"runId":    "run-1",
			"status":   "succeeded",
			"response": "Polio cases dropped.",
			"sources": []map[string]any{
				{"chunkId": "c1", "filename": "paper.pdf", "sectionTitle": "Methods"},
			},
		})
	})

	var events []AgentRunEvent
	err := api.Client().StreamAgent(context.Background(), "org-1", "proj-2", "qa-test", "why?", func(ev AgentRunEvent) error {
		events = append(events, ev)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 5 {
		t.Fatalf("expected 5 events, got %d: %+v", len(events), events)
	}
	if events[0].Type != "run_started" || events[0].RunID != "run-1" {
		t.Errorf("first event: %+v", events[0])
	}
	if events[4].Type != "done" || events[4].Response != "Polio cases dropped." {
		t.Errorf("done event: %+v", events[4])
	}
	if len(events[4].Sources) != 1 || events[4].Sources[0].Filename != "paper.pdf" {
		t.Errorf("done sources: %+v", events[4].Sources)
	}
}

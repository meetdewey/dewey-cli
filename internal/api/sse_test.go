package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// sseFlush writes an SSE data frame and flushes the response.
func sseFlush(w http.ResponseWriter, payload string) {
	fmt.Fprintf(w, "data: %s\n\n", payload)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func TestStreamResearch_DeliversFramesInOrder(t *testing.T) {
	api := newFakeAPI(t)
	api.Handle("POST /collections/c1/research", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		sseFlush(w, `: ping comment to ignore`)
		sseFlush(w, `{"type":"tool_call","query":"what is X?","tool":"search"}`)
		sseFlush(w, `{"type":"chunk","content":"hello "}`)
		sseFlush(w, `{"type":"chunk","content":"world"}`)
		sseFlush(w, `{"type":"done","sessionId":"s1","sources":[{"chunkId":"ch1","filename":"a.pdf"}]}`)
	})

	var events []ResearchEvent
	err := api.Client().StreamResearch(context.Background(), "c1", "what is X?", ResearchOptions{}, func(ev ResearchEvent) error {
		events = append(events, ev)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 4 {
		t.Fatalf("got %d events: %+v", len(events), events)
	}
	mustEqual(t, events[0].Type, "tool_call", "type[0]")
	mustEqual(t, events[1].Content, "hello ", "chunk[0]")
	mustEqual(t, events[2].Content, "world", "chunk[1]")
	mustEqual(t, events[3].Type, "done", "type[3]")
	if len(events[3].Sources) != 1 || events[3].Sources[0].Filename != "a.pdf" {
		t.Errorf("done sources wrong: %+v", events[3].Sources)
	}
}

func TestStreamResearch_HandlerEOFStopsEarly(t *testing.T) {
	api := newFakeAPI(t)
	api.Handle("POST /collections/c1/research", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		for i := 0; i < 5; i++ {
			sseFlush(w, fmt.Sprintf(`{"type":"chunk","content":"%d"}`, i))
		}
	})

	var n int
	err := api.Client().StreamResearch(context.Background(), "c1", "q", ResearchOptions{}, func(ResearchEvent) error {
		n++
		if n == 2 {
			return io.EOF
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("expected to stop after 2, got %d", n)
	}
}

func TestStreamResearch_MalformedFramesSkipped(t *testing.T) {
	api := newFakeAPI(t)
	api.Handle("POST /collections/c1/research", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		sseFlush(w, "not valid json")
		sseFlush(w, `[DONE]`)
		sseFlush(w, `{"type":"chunk","content":"ok"}`)
	})

	var seen []string
	err := api.Client().StreamResearch(context.Background(), "c1", "q", ResearchOptions{}, func(ev ResearchEvent) error {
		seen = append(seen, ev.Type)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Malformed JSON frames are dropped before reaching the handler; [DONE] is
	// dropped at the SSE-parser level. Only the well-formed chunk arrives.
	if len(seen) != 1 || seen[0] != "chunk" {
		t.Errorf("unexpected events: %+v", seen)
	}
}

func TestStreamResearch_ServerError(t *testing.T) {
	api := newFakeAPI(t)
	api.Handle("POST /collections/c1/research", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"message":"too many"}`))
	})

	err := api.Client().StreamResearch(context.Background(), "c1", "q", ResearchOptions{}, func(ResearchEvent) error {
		return nil
	})
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Status != 429 {
		t.Errorf("expected 429 api.Error, got %v", err)
	}
}

func TestStreamDocumentEvents_AttachesAPIKeyAsQueryParam(t *testing.T) {
	api := newFakeAPI(t)
	var captured atomic.Value
	api.Handle("GET /collections/c1/documents/events", func(w http.ResponseWriter, r *http.Request) {
		captured.Store(r.URL.RawQuery)
		w.Header().Set("Content-Type", "text/event-stream")
		sseFlush(w, `{"documentId":"d1","status":"ready"}`)
		// Server closes the connection.
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := api.Client().StreamDocumentEvents(ctx, "c1", func(ev DocumentEvent) error {
		if ev.DocumentID == "d1" && ev.Status == "ready" {
			return io.EOF
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := captured.Load().(string)
	if !strings.Contains(got, "key=test-key") {
		t.Errorf("api key not on query string: %q", got)
	}
}

func TestStreamDocumentEvents_ReconnectsAfterTransientError(t *testing.T) {
	var hits int32
	api := newFakeAPI(t)
	api.Handle("GET /collections/c1/documents/events", func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if n == 1 {
			// First connection: server is unhappy.
			w.WriteHeader(503)
			_, _ = w.Write([]byte(`{"message":"upstream"}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		sseFlush(w, `{"documentId":"d1","status":"ready"}`)
	})

	// Backoff starts at 1s. Give the test plenty of headroom.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := api.Client().StreamDocumentEvents(ctx, "c1", func(ev DocumentEvent) error {
		if ev.Status == "ready" {
			return io.EOF
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&hits) < 2 {
		t.Errorf("expected at least one reconnect, got %d hits", hits)
	}
}

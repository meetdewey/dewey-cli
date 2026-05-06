package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestQuery_PostsBody(t *testing.T) {
	api := newFakeAPI(t)
	api.Handle("POST /collections/c1/query", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		mustEqual(t, body["q"].(string), "what is X?", "q")
		if body["limit"].(float64) != 5 {
			t.Errorf("limit = %v", body["limit"])
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]RetrievalResult{{Score: 0.9}})
	})

	res, err := api.Client().Query(context.Background(), "c1", "what is X?", QueryOptions{Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Score != 0.9 {
		t.Errorf("unexpected: %+v", res)
	}
}

func TestQuery_OmitsEmptyOptions(t *testing.T) {
	api := newFakeAPI(t)
	api.HandleJSON("POST /collections/c1/query", 200, []RetrievalResult{})
	if _, err := api.Client().Query(context.Background(), "c1", "q", QueryOptions{}); err != nil {
		t.Fatal(err)
	}
	body := string(api.LastRequest().Body)
	for _, k := range []string{"limit", "tags", "anyTags", "metadata"} {
		if strings.Contains(body, `"`+k+`"`) {
			t.Errorf("expected %s to be omitted: %s", k, body)
		}
	}
}

func TestScanSections_PostsBody(t *testing.T) {
	api := newFakeAPI(t)
	api.Handle("POST /collections/c1/sections/scan", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		mustEqual(t, body["query"].(string), "what is X?", "query")
		if body["top_k"].(float64) != 7 {
			t.Errorf("top_k = %v", body["top_k"])
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(SectionScanResponse{Results: []SectionScanResult{{Score: 0.5}}})
	})

	res, err := api.Client().ScanSections(context.Background(), "c1", "what is X?", ScanOptions{TopK: 7})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Results) != 1 || res.Results[0].Score != 0.5 {
		t.Errorf("unexpected: %+v", res)
	}
}

func TestResearchSync(t *testing.T) {
	api := newFakeAPI(t)
	api.Handle("POST /collections/c1/research/sync", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		mustEqual(t, body["q"].(string), "explain", "q")
		mustEqual(t, body["depth"].(string), "deep", "depth")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ResearchResult{
			Answer:    "the answer",
			SessionID: "sess1",
			Sources:   []ResearchSource{{ChunkID: "ch1"}},
		})
	})

	res, err := api.Client().ResearchSync(context.Background(), "c1", "explain", ResearchOptions{Depth: "deep"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Answer != "the answer" || res.SessionID != "sess1" || len(res.Sources) != 1 {
		t.Errorf("unexpected: %+v", res)
	}
}

func TestResearchSync_OmitsEmptyOptions(t *testing.T) {
	api := newFakeAPI(t)
	api.HandleJSON("POST /collections/c1/research/sync", 200, ResearchResult{})
	if _, err := api.Client().ResearchSync(context.Background(), "c1", "q", ResearchOptions{}); err != nil {
		t.Fatal(err)
	}
	body := string(api.LastRequest().Body)
	for _, k := range []string{"depth", "model", "tags", "anyTags", "metadata"} {
		if strings.Contains(body, `"`+k+`"`) {
			t.Errorf("expected %s to be omitted: %s", k, body)
		}
	}
}

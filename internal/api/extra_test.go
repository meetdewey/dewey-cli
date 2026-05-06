package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// ── Provider keys ──────────────────────────────────────────────────────────

func TestListProviderKeys(t *testing.T) {
	api := newFakeAPI(t)
	api.HandleJSON("GET /provider-keys", 200, []ProviderKey{{ID: "pk1", Provider: "openai"}})
	keys, err := api.Client().ListProviderKeys(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0].Provider != "openai" {
		t.Errorf("unexpected: %+v", keys)
	}
}

func TestCreateProviderKey(t *testing.T) {
	api := newFakeAPI(t)
	api.Handle("POST /provider-keys", func(w http.ResponseWriter, r *http.Request) {
		var body CreateProviderKeyInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		mustEqual(t, body.Provider, "openai", "provider")
		mustEqual(t, body.Key, "sk-secret", "key")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ProviderKey{ID: "pk1", Provider: body.Provider, Name: body.Name})
	})

	pk, err := api.Client().CreateProviderKey(context.Background(), CreateProviderKeyInput{
		Provider: "openai", Key: "sk-secret", Name: "default",
	})
	if err != nil {
		t.Fatal(err)
	}
	if pk.ID != "pk1" {
		t.Errorf("got id %q", pk.ID)
	}
}

func TestDeleteProviderKey(t *testing.T) {
	api := newFakeAPI(t)
	api.Handle("DELETE /provider-keys/pk1", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	if err := api.Client().DeleteProviderKey(context.Background(), "pk1"); err != nil {
		t.Fatal(err)
	}
}

// ── Duplicates ─────────────────────────────────────────────────────────────

func TestDetectDuplicates(t *testing.T) {
	api := newFakeAPI(t)
	api.HandleJSON("POST /collections/c1/duplicates/detect", 202, DuplicateDetectResult{
		RunID: "run1", Status: "queued", JobsEnqueued: 3,
	})
	res, err := api.Client().DetectDuplicates(context.Background(), "c1")
	if err != nil {
		t.Fatal(err)
	}
	if res.RunID != "run1" || res.JobsEnqueued != 3 {
		t.Errorf("unexpected: %+v", res)
	}
}

func TestListDuplicateGroups_PagingQueryParams(t *testing.T) {
	api := newFakeAPI(t)
	api.HandleJSON("GET /collections/c1/duplicates", 200, DuplicateGroupList{
		Total: 0, Items: []DuplicateGroup{},
	})
	if _, err := api.Client().ListDuplicateGroups(context.Background(), "c1", 25, 50); err != nil {
		t.Fatal(err)
	}
	rec := api.LastRequest()
	if !strings.Contains(rec.RawQuery, "limit=25") || !strings.Contains(rec.RawQuery, "offset=50") {
		t.Errorf("paging not on query string: %q", rec.RawQuery)
	}
}

func TestPromoteDuplicateCanonical(t *testing.T) {
	api := newFakeAPI(t)
	api.Handle("PATCH /collections/c1/duplicates/g1", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		mustEqual(t, body["canonicalDocumentId"], "d2", "canonical")
		w.WriteHeader(http.StatusNoContent)
	})
	if err := api.Client().PromoteDuplicateCanonical(context.Background(), "c1", "g1", "d2"); err != nil {
		t.Fatal(err)
	}
}

func TestDisbandDuplicateGroup(t *testing.T) {
	api := newFakeAPI(t)
	api.Handle("DELETE /collections/c1/duplicates/g1", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	if err := api.Client().DisbandDuplicateGroup(context.Background(), "c1", "g1"); err != nil {
		t.Fatal(err)
	}
}

func TestDuplicatesLatestRun(t *testing.T) {
	api := newFakeAPI(t)
	api.HandleJSON("GET /collections/c1/duplicates/runs/latest", 200, DuplicateRun{ID: "r1", Status: "completed"})
	run, err := api.Client().DuplicatesLatestRun(context.Background(), "c1")
	if err != nil {
		t.Fatal(err)
	}
	if run.ID != "r1" || run.Status != "completed" {
		t.Errorf("unexpected: %+v", run)
	}
}

// ── Contradictions ─────────────────────────────────────────────────────────

func TestDetectContradictions(t *testing.T) {
	api := newFakeAPI(t)
	api.HandleJSON("POST /collections/c1/contradictions/detect", 202, ContradictionDetectResult{RunID: "run1"})
	res, err := api.Client().DetectContradictions(context.Background(), "c1")
	if err != nil {
		t.Fatal(err)
	}
	if res.RunID != "run1" {
		t.Errorf("unexpected: %+v", res)
	}
}

func TestListContradictions_QueryParams(t *testing.T) {
	api := newFakeAPI(t)
	api.HandleJSON("GET /collections/c1/contradictions", 200, ContradictionList{})
	if _, err := api.Client().ListContradictions(context.Background(), "c1", "high", "active", 10); err != nil {
		t.Fatal(err)
	}
	q := api.LastRequest().RawQuery
	for _, want := range []string{"severity=high", "status=active", "limit=10"} {
		if !strings.Contains(q, want) {
			t.Errorf("query missing %s: %q", want, q)
		}
	}
}

func TestDismissContradiction(t *testing.T) {
	api := newFakeAPI(t)
	api.Handle("PATCH /collections/c1/contradictions/x1", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		mustEqual(t, body["status"], "dismissed", "status")
		w.WriteHeader(http.StatusNoContent)
	})
	if err := api.Client().DismissContradiction(context.Background(), "c1", "x1"); err != nil {
		t.Fatal(err)
	}
}

func TestApplyContradictionInstruction_OmittedWhenEmpty(t *testing.T) {
	api := newFakeAPI(t)
	api.Handle("POST /collections/c1/contradictions/x1/apply-instruction", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if _, ok := body["instruction"]; ok {
			t.Errorf("instruction unexpectedly sent: %v", body)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	if err := api.Client().ApplyContradictionInstruction(context.Background(), "c1", "x1", ""); err != nil {
		t.Fatal(err)
	}
}

func TestApplyContradictionInstruction_PassedThrough(t *testing.T) {
	api := newFakeAPI(t)
	api.Handle("POST /collections/c1/contradictions/x1/apply-instruction", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		mustEqual(t, body["instruction"], "prefer 2024 numbers", "instruction")
		w.WriteHeader(http.StatusNoContent)
	})
	if err := api.Client().ApplyContradictionInstruction(context.Background(), "c1", "x1", "prefer 2024 numbers"); err != nil {
		t.Fatal(err)
	}
}

// ── Claims ─────────────────────────────────────────────────────────────────

func TestDocumentClaims(t *testing.T) {
	api := newFakeAPI(t)
	api.HandleJSON("GET /documents/d1/claims", 200, DocumentClaims{
		DocumentID: "d1",
		Claims:     []Claim{{ID: "c1", Text: "the sky is blue", Importance: 4}},
	})
	res, err := api.Client().DocumentClaims(context.Background(), "d1", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Claims) != 1 || res.Claims[0].Importance != 4 {
		t.Errorf("unexpected: %+v", res)
	}
	if !strings.Contains(api.LastRequest().RawQuery, "minImportance=3") {
		t.Errorf("minImportance not on query: %q", api.LastRequest().RawQuery)
	}
}

func TestDocumentClaims_NoMinImportance(t *testing.T) {
	api := newFakeAPI(t)
	api.HandleJSON("GET /documents/d1/claims", 200, DocumentClaims{DocumentID: "d1"})
	if _, err := api.Client().DocumentClaims(context.Background(), "d1", 0); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(api.LastRequest().RawQuery, "minImportance") {
		t.Errorf("minImportance should be absent: %q", api.LastRequest().RawQuery)
	}
}

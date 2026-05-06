package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/lambdabaa/dewey/apps/cli/internal/api"
)

// ── provider-keys ─────────────────────────────────────────────────────────

func TestProviderKeys_List(t *testing.T) {
	h := newHarness(t)
	h.HandleJSON("GET /provider-keys", 200, []api.ProviderKey{
		{ID: "pk1", Provider: "openai", Name: "default", KeyPreview: "sk-…abcd"},
	})
	if h.run("provider-keys", "list") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	if !strings.Contains(h.stdout.String(), "openai") {
		t.Errorf("missing provider: %q", h.stdout.String())
	}
}

func TestProviderKeys_Set(t *testing.T) {
	h := newHarness(t)
	h.Handle("POST /provider-keys", func(w http.ResponseWriter, r *http.Request) {
		var body api.CreateProviderKeyInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		mustEqualS(t, body.Provider, "openai", "provider")
		mustEqualS(t, body.Key, "sk-secret", "key")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.ProviderKey{ID: "pk1", Provider: body.Provider, Name: body.Name})
	})
	if h.run("provider-keys", "set", "openai", "sk-secret", "--name", "default") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
}

func TestProviderKeys_Delete(t *testing.T) {
	h := newHarness(t)
	h.Handle("DELETE /provider-keys/pk1", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	if h.run("provider-keys", "delete", "pk1") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
}

// ── duplicates ─────────────────────────────────────────────────────────────

func TestDuplicates_Detect(t *testing.T) {
	h := newHarness(t)
	h.HandleJSON("GET /collections", 200, []api.Collection{{ID: "c1", Name: "papers"}})
	h.HandleJSON("POST /collections/c1/duplicates/detect", 202, api.DuplicateDetectResult{
		RunID: "run1", Status: "queued", JobsEnqueued: 4,
	})
	if h.run("duplicates", "detect", "papers") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	if !strings.Contains(h.stderr.String(), "run1") {
		t.Errorf("missing run id: %q", h.stderr.String())
	}
}

func TestDuplicates_List(t *testing.T) {
	h := newHarness(t)
	h.HandleJSON("GET /collections", 200, []api.Collection{{ID: "c1", Name: "papers"}})
	h.HandleJSON("GET /collections/c1/duplicates", 200, api.DuplicateGroupList{
		Total: 0, Items: []api.DuplicateGroup{},
	})
	if h.run("duplicates", "list", "papers") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	if !strings.Contains(h.stderr.String(), "No duplicate groups") {
		t.Errorf("expected empty msg: %q", h.stderr.String())
	}
}

// ── contradictions ─────────────────────────────────────────────────────────

func TestContradictions_List(t *testing.T) {
	h := newHarness(t)
	h.HandleJSON("GET /collections", 200, []api.Collection{{ID: "c1", Name: "papers"}})
	h.HandleJSON("GET /collections/c1/contradictions", 200, api.ContradictionList{
		Total: 0, Items: []api.Contradiction{},
	})
	if h.run("contradictions", "list", "papers") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	if !strings.Contains(h.stderr.String(), "No contradictions") {
		t.Errorf("expected empty msg: %q", h.stderr.String())
	}
}

func TestContradictions_Apply(t *testing.T) {
	const uuid = "22222222-2222-2222-2222-222222222222"
	h := newHarness(t)
	h.HandleJSON("GET /collections/"+uuid, 200, api.Collection{ID: uuid, Name: "papers"})
	h.Handle("POST /collections/"+uuid+"/contradictions/x1/apply-instruction", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		mustEqualS(t, body["instruction"], "prefer 2024", "instruction")
		w.WriteHeader(http.StatusNoContent)
	})
	// Populate last_collection so apply can use resolveCollection("").
	h.HandleJSON("POST /collections", 200, api.Collection{ID: uuid, Name: "papers"})
	if h.run("collections", "create", "papers", "--project-id", "proj_test") != ExitOK {
		t.Fatal("setup")
	}
	h.stdout.Reset()
	h.stderr.Reset()
	if h.run("contradictions", "apply", "x1", "--instruction", "prefer 2024") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
}

// ── claims ─────────────────────────────────────────────────────────────────

func TestClaims_List(t *testing.T) {
	h := newHarness(t)
	h.HandleJSON("GET /documents/d1/claims", 200, api.DocumentClaims{
		DocumentID: "d1",
		Claims: []api.Claim{
			{ID: "c1", Text: "the sky is blue", Importance: 4, SectionTitle: "Intro"},
		},
	})
	if h.run("claims", "list", "d1") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	if !strings.Contains(h.stdout.String(), "the sky is blue") {
		t.Errorf("missing claim: %q", h.stdout.String())
	}
}

// ── config ─────────────────────────────────────────────────────────────────

func TestConfig_Set_Get_Roundtrip(t *testing.T) {
	h := newHarness(t)
	if h.run("config", "set", "default_collection", "papers") != ExitOK {
		t.Fatalf("set failed: %s", h.stderr.String())
	}
	h.stdout.Reset()
	if h.run("config", "get", "default_collection") != ExitOK {
		t.Fatalf("get failed: %s", h.stderr.String())
	}
	if strings.TrimSpace(h.stdout.String()) != "papers" {
		t.Errorf("got %q", h.stdout.String())
	}
}

func TestConfig_Set_RejectsInvalidOutput(t *testing.T) {
	h := newHarness(t)
	if h.run("config", "set", "output", "yaml") == ExitOK {
		t.Errorf("expected error for invalid output value")
	}
}

func TestConfig_Set_RejectsUnknownKey(t *testing.T) {
	h := newHarness(t)
	if h.run("config", "set", "fake_key", "value") == ExitOK {
		t.Errorf("expected error for unknown key")
	}
}

func TestConfig_Reset_ClearsState(t *testing.T) {
	const uuid = "33333333-3333-3333-3333-333333333333"
	h := newHarness(t)
	h.HandleJSON("POST /collections", 200, api.Collection{ID: uuid, Name: "papers"})
	if h.run("collections", "create", "papers", "--project-id", "proj_test") != ExitOK {
		t.Fatal(h.stderr.String())
	}
	if h.state.LastCollection != uuid {
		t.Fatal("setup: expected last_collection set")
	}
	if h.run("config", "reset") != ExitOK {
		t.Fatalf("reset failed: %s", h.stderr.String())
	}
	if h.state.LastCollection != "" {
		t.Errorf("expected last_collection cleared, got %q", h.state.LastCollection)
	}
}

func TestConfig_Path(t *testing.T) {
	h := newHarness(t)
	if h.run("config", "path") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	if !strings.Contains(h.stdout.String(), ".dewey/config.toml") {
		t.Errorf("path missing: %q", h.stdout.String())
	}
}

// ── version ────────────────────────────────────────────────────────────────

func TestVersion(t *testing.T) {
	h := newHarness(t)
	if h.run("version") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	for _, want := range []string{"dewey", "api", "commit"} {
		if !strings.Contains(h.stdout.String(), want) {
			t.Errorf("version output missing %q: %s", want, h.stdout.String())
		}
	}
}

// ── doctor ─────────────────────────────────────────────────────────────────

func TestDoctor_AllChecksPass(t *testing.T) {
	h := newHarness(t)
	h.HandleJSON("GET /collections", 200, []api.Collection{{ID: "c1"}})
	if h.run("doctor") != ExitOK {
		t.Fatalf("stderr=%s", h.stderr.String())
	}
	out := h.stdout.String() + h.stderr.String()
	for _, want := range []string{"DEWEY_API_KEY is set", "GET /collections", "All systems go"} {
		if !strings.Contains(out, want) {
			t.Errorf("doctor missing %q in output: %s", want, out)
		}
	}
}

func TestDoctor_FailsWhenAPIRejects(t *testing.T) {
	h := newHarness(t)
	h.Handle("GET /collections", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"message":"nope"}`))
	})
	exit := h.run("doctor")
	if exit == ExitOK {
		t.Errorf("expected non-zero exit when /collections fails")
	}
}

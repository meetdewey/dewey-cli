package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestListCollections(t *testing.T) {
	api := newFakeAPI(t)
	api.HandleJSON("GET /collections", 200, []Collection{
		{ID: "c1", Name: "papers"},
	})
	cols, err := api.Client().ListCollections(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 1 || cols[0].ID != "c1" {
		t.Errorf("unexpected: %+v", cols)
	}
}

func TestGetCollection(t *testing.T) {
	api := newFakeAPI(t)
	api.HandleJSON("GET /collections/c1", 200, Collection{ID: "c1", Name: "papers"})
	col, err := api.Client().GetCollection(context.Background(), "c1")
	if err != nil {
		t.Fatal(err)
	}
	if col.Name != "papers" {
		t.Errorf("got name %q", col.Name)
	}
}

func TestCreateCollection_PostsBody(t *testing.T) {
	api := newFakeAPI(t)
	api.Handle("POST /collections", func(w http.ResponseWriter, r *http.Request) {
		var body CreateCollectionInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		mustEqual(t, body.Name, "papers", "create name")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Collection{ID: "c1", Name: body.Name})
	})
	col, err := api.Client().CreateCollection(context.Background(), CreateCollectionInput{Name: "papers"})
	if err != nil {
		t.Fatal(err)
	}
	if col.ID != "c1" {
		t.Errorf("got id %q", col.ID)
	}
}

func TestUpdateCollection_OmitsUnsetFields(t *testing.T) {
	api := newFakeAPI(t)
	api.HandleJSON("PATCH /collections/c1", 200, Collection{ID: "c1", Name: "renamed"})

	name := "renamed"
	_, err := api.Client().UpdateCollection(context.Background(), "c1", UpdateCollectionInput{Name: &name})
	if err != nil {
		t.Fatal(err)
	}
	rec := api.LastRequest()
	body := string(rec.Body)
	if !strings.Contains(body, `"name":"renamed"`) {
		t.Errorf("name field missing: %s", body)
	}
	// Other fields should be omitted because of `omitempty`.
	for _, k := range []string{"visibility", "chunkSize", "description"} {
		if strings.Contains(body, `"`+k+`"`) {
			t.Errorf("expected %s to be omitted: %s", k, body)
		}
	}
}

func TestDeleteCollection(t *testing.T) {
	api := newFakeAPI(t)
	api.Handle("DELETE /collections/c1", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	if err := api.Client().DeleteCollection(context.Background(), "c1"); err != nil {
		t.Fatal(err)
	}
}

func TestCollectionStats(t *testing.T) {
	api := newFakeAPI(t)
	api.HandleJSON("GET /collections/c1/stats", 200, CollectionStats{DocCount: 5, TotalChunks: 100})
	stats, err := api.Client().CollectionStats(context.Background(), "c1")
	if err != nil {
		t.Fatal(err)
	}
	if stats.DocCount != 5 || stats.TotalChunks != 100 {
		t.Errorf("unexpected: %+v", stats)
	}
}

func TestResolveCollection_ByUUID(t *testing.T) {
	api := newFakeAPI(t)
	const uuid = "bfd84c1e-6f32-4dae-b323-157687e75cfe"
	api.HandleJSON("GET /collections/"+uuid, 200, Collection{ID: uuid, Name: "x"})
	col, err := api.Client().ResolveCollection(context.Background(), uuid)
	if err != nil {
		t.Fatal(err)
	}
	if col.ID != uuid {
		t.Errorf("got id %q", col.ID)
	}
}

func TestResolveCollection_ByColPrefix(t *testing.T) {
	api := newFakeAPI(t)
	api.HandleJSON("GET /collections/col_abc", 200, Collection{ID: "col_abc", Name: "x"})
	col, err := api.Client().ResolveCollection(context.Background(), "col_abc")
	if err != nil {
		t.Fatal(err)
	}
	if col.ID != "col_abc" {
		t.Fatal("not resolved")
	}
}

func TestResolveCollection_ByName(t *testing.T) {
	api := newFakeAPI(t)
	api.HandleJSON("GET /collections", 200, []Collection{
		{ID: "1", Name: "Papers"},
		{ID: "2", Name: "Notes"},
	})
	col, err := api.Client().ResolveCollection(context.Background(), "papers")
	if err != nil {
		t.Fatal(err)
	}
	if col.ID != "1" {
		t.Errorf("expected case-insensitive match, got %s", col.ID)
	}
}

func TestResolveCollection_NoMatch(t *testing.T) {
	api := newFakeAPI(t)
	api.HandleJSON("GET /collections", 200, []Collection{{ID: "1", Name: "Papers"}})
	_, err := api.Client().ResolveCollection(context.Background(), "missing")
	if err == nil || !strings.Contains(err.Error(), "no collection matched") {
		t.Errorf("expected no-match error, got %v", err)
	}
}

func TestResolveCollection_AmbiguousName(t *testing.T) {
	api := newFakeAPI(t)
	api.HandleJSON("GET /collections", 200, []Collection{
		{ID: "1", Name: "Papers"},
		{ID: "2", Name: "papers"},
	})
	_, err := api.Client().ResolveCollection(context.Background(), "papers")
	if err == nil || !strings.Contains(err.Error(), "multiple collections") {
		t.Errorf("expected ambiguous error, got %v", err)
	}
}

func TestLooksLikeID(t *testing.T) {
	cases := map[string]bool{
		"col_abc":                              true,
		"bfd84c1e-6f32-4dae-b323-157687e75cfe": true,
		"papers":                               false,
		"":                                     false,
		"col":                                  false,
		"too-short":                            false,
	}
	for s, want := range cases {
		if got := looksLikeID(s); got != want {
			t.Errorf("looksLikeID(%q) = %v, want %v", s, got, want)
		}
	}
}

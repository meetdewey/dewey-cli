package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestListDocuments_UnwrapsPagedResponse(t *testing.T) {
	api := newFakeAPI(t)
	api.HandleJSON("GET /collections/c1/documents", 200, DocumentList{
		Documents: []Document{{ID: "d1", Filename: "a.pdf"}},
		Total:     1,
	})
	docs, err := api.Client().ListDocuments(context.Background(), "c1")
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 || docs[0].ID != "d1" {
		t.Errorf("unexpected: %+v", docs)
	}
}

func TestListDocumentsPaged(t *testing.T) {
	api := newFakeAPI(t)
	api.HandleJSON("GET /collections/c1/documents", 200, DocumentList{
		Documents: []Document{{ID: "d1"}},
		Total:     17,
	})
	page, err := api.Client().ListDocumentsPaged(context.Background(), "c1")
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 17 || len(page.Documents) != 1 {
		t.Errorf("unexpected: %+v", page)
	}
}

func TestGetDocument(t *testing.T) {
	api := newFakeAPI(t)
	api.HandleJSON("GET /documents/d1", 200, Document{ID: "d1", Filename: "x.pdf"})
	doc, err := api.Client().GetDocument(context.Background(), "d1")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Filename != "x.pdf" {
		t.Errorf("got %q", doc.Filename)
	}
}

func TestDeleteDocument(t *testing.T) {
	api := newFakeAPI(t)
	api.Handle("DELETE /documents/d1", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	if err := api.Client().DeleteDocument(context.Background(), "d1"); err != nil {
		t.Fatal(err)
	}
}

func TestListSections(t *testing.T) {
	api := newFakeAPI(t)
	api.HandleJSON("GET /documents/d1/sections", 200, []Section{
		{ID: "s1", Title: "Intro", Level: 1},
	})
	sections, err := api.Client().ListSections(context.Background(), "d1")
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 1 || sections[0].Title != "Intro" {
		t.Errorf("unexpected: %+v", sections)
	}
}

func TestListChunks(t *testing.T) {
	api := newFakeAPI(t)
	api.HandleJSON("GET /sections/s1/chunks", 200, []Chunk{{ID: "ch1", TokenCount: 42}})
	chunks, err := api.Client().ListChunks(context.Background(), "s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 || chunks[0].TokenCount != 42 {
		t.Errorf("unexpected: %+v", chunks)
	}
}

func TestListImages(t *testing.T) {
	api := newFakeAPI(t)
	api.HandleJSON("GET /documents/d1/images", 200, []map[string]interface{}{
		{"id": "img1", "caption": "fig"},
	})
	imgs, err := api.Client().ListImages(context.Background(), "d1")
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != 1 || imgs[0]["id"] != "img1" {
		t.Errorf("unexpected: %+v", imgs)
	}
}

func TestWaitForDocument(t *testing.T) {
	api := newFakeAPI(t)
	api.HandleJSON("GET /documents/d1/wait", 200, Document{ID: "d1", Status: "ready"})
	doc, err := api.Client().WaitForDocument(context.Background(), "d1")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Status != "ready" {
		t.Errorf("got status %q", doc.Status)
	}
}

func TestUploadFile_Multipart_SmallFile(t *testing.T) {
	api := newFakeAPI(t)

	var receivedFilename, receivedTagsField, receivedMetaField string
	var receivedBytes []byte

	api.Handle("POST /collections/c1/documents", func(w http.ResponseWriter, r *http.Request) {
		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
			http.Error(w, "expected multipart", 400)
			return
		}
		mr := multipart.NewReader(r.Body, params["boundary"])
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			data, _ := io.ReadAll(p)
			switch p.FormName() {
			case "file":
				receivedFilename = p.FileName()
				receivedBytes = data
			case "tags":
				receivedTagsField = string(data)
			case "metadata":
				receivedMetaField = string(data)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Document{ID: "d1", Filename: receivedFilename, Status: "uploading"})
	})

	tmp := t.TempDir()
	path := filepath.Join(tmp, "small.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	doc, err := api.Client().UploadFile(context.Background(), "c1", path, UploadOptions{
		Tags:     []string{"a", "b"},
		Metadata: map[string]any{"author": "ari"},
	})
	if err != nil {
		t.Fatal(err)
	}
	mustEqual(t, doc.ID, "d1", "doc id")
	mustEqual(t, receivedFilename, "small.txt", "filename")
	mustEqual(t, string(receivedBytes), "hello world", "body")
	if !strings.Contains(receivedTagsField, `"a"`) || !strings.Contains(receivedTagsField, `"b"`) {
		t.Errorf("tags field missing entries: %q", receivedTagsField)
	}
	if !strings.Contains(receivedMetaField, `"author":"ari"`) {
		t.Errorf("metadata field missing: %q", receivedMetaField)
	}
}

func TestUploadFile_Presigned_LargeFile(t *testing.T) {
	// Stand up a separate server to act as S3.
	var s3Hits int32
	s3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		// Drain body so the client finishes writing and progressReader.Read is called.
		_, _ = io.Copy(io.Discard, r.Body)
		atomic.AddInt32(&s3Hits, 1)
		w.WriteHeader(200)
	}))
	t.Cleanup(s3.Close)

	api := newFakeAPI(t)

	api.Handle("POST /collections/c1/documents/upload-url", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["filename"] != "big.bin" {
			t.Errorf("wrong filename: %v", body["filename"])
		}
		// Sanity-check hash matches what we pass.
		if _, ok := body["contentHash"].(string); !ok {
			t.Errorf("missing contentHash")
		}
		uploadURL := s3.URL + "/upload"
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"documentId": "d-presign",
			"uploadUrl":  uploadURL,
		})
	})

	api.Handle("POST /collections/c1/documents/d-presign/confirm", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Document{ID: "d-presign", Filename: "big.bin", Status: "processing"})
	})

	tmp := t.TempDir()
	path := filepath.Join(tmp, "big.bin")
	// Need >= 4 MiB to hit the presigned path.
	data := make([]byte, 5*1024*1024)
	for i := range data {
		data[i] = byte(i % 251)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	wantHash := sha256.Sum256(data)
	wantHex := hex.EncodeToString(wantHash[:])

	var progressTransferred atomic.Int64
	doc, err := api.Client().UploadFile(context.Background(), "c1", path, UploadOptions{
		OnProgress: func(transferred, total int64) {
			progressTransferred.Store(transferred)
			_ = total
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if doc.ID != "d-presign" {
		t.Errorf("got %s", doc.ID)
	}
	if atomic.LoadInt32(&s3Hits) != 1 {
		t.Errorf("expected one S3 PUT, got %d", s3Hits)
	}
	if progressTransferred.Load() == 0 {
		t.Errorf("expected progress callbacks")
	}

	// Verify the presign request carried the correct hash.
	for _, rec := range api.Requests() {
		if rec.Path == "/collections/c1/documents/upload-url" {
			var body map[string]any
			if err := json.Unmarshal(rec.Body, &body); err != nil {
				t.Fatal(err)
			}
			if body["contentHash"] != wantHex {
				t.Errorf("wrong hash: got %v want %s", body["contentHash"], wantHex)
			}
		}
	}
}

func TestUploadFile_Presigned_DedupHit(t *testing.T) {
	api := newFakeAPI(t)
	api.Handle("POST /collections/c1/documents/upload-url", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// uploadUrl == nil signals dedup hit; the existing doc is returned.
		_ = json.NewEncoder(w).Encode(map[string]any{
			"documentId": "d-existing",
			"uploadUrl":  nil,
			"document":   Document{ID: "d-existing", Filename: "big.bin", Status: "ready"},
		})
	})

	tmp := t.TempDir()
	path := filepath.Join(tmp, "big.bin")
	data := make([]byte, 5*1024*1024)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := api.Client().UploadFile(context.Background(), "c1", path, UploadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if doc.ID != "d-existing" {
		t.Errorf("dedup hit doc not returned: %+v", doc)
	}
	// Verify no /confirm call happened.
	for _, rec := range api.Requests() {
		if strings.Contains(rec.Path, "/confirm") {
			t.Errorf("unexpected confirm call: %+v", rec)
		}
	}
}

func TestUploadFile_RejectsDirectory(t *testing.T) {
	api := newFakeAPI(t)
	tmp := t.TempDir()
	_, err := api.Client().UploadFile(context.Background(), "c1", tmp, UploadOptions{})
	if err == nil || !strings.Contains(err.Error(), "directory") {
		t.Errorf("expected directory error, got %v", err)
	}
}

func TestGuessContentType(t *testing.T) {
	cases := map[string]string{
		"foo.pdf":     "application/pdf",
		"foo.PDF":     "application/pdf",
		"foo.docx":    "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"foo.html":    "text/html",
		"foo.md":      "text/markdown",
		"foo.txt":     "text/plain",
		"foo.json":    "application/json",
		"foo.unknown": "application/octet-stream",
	}
	for fn, want := range cases {
		if got := guessContentType(fn); got != want {
			t.Errorf("guessContentType(%q) = %q, want %q", fn, got, want)
		}
	}
}

func TestSha256File(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "x")
	if err := os.WriteFile(path, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := sha256File(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got != want {
		t.Errorf("got %s want %s", got, want)
	}
}

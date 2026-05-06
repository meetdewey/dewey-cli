package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// UploadOptions controls how a file is uploaded.
type UploadOptions struct {
	Tags     []string
	Metadata map[string]interface{}
	// OnProgress is called as the upload makes progress, with the number of
	// bytes transferred so far and the total size in bytes. Optional.
	OnProgress func(transferred, total int64)
}

// UploadFile picks the best upload strategy. For files >= 5 MiB we use the
// presigned-URL flow (request URL → PUT to S3 → confirm). For smaller files we
// use multipart/form-data which is simpler and suffices.
func (c *Client) UploadFile(ctx context.Context, collectionID, path string, opts UploadOptions) (*Document, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		return nil, fmt.Errorf("%s is a directory", path)
	}

	if st.Size() >= 4*1024*1024 {
		return c.uploadViaPresign(ctx, collectionID, path, st.Size(), opts)
	}
	return c.uploadMultipart(ctx, collectionID, path, opts)
}

func (c *Client) uploadMultipart(ctx context.Context, collectionID, path string, opts UploadOptions) (*Document, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)

	filename := filepath.Base(path)
	contentType := guessContentType(filename)

	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, filename))
	hdr.Set("Content-Type", contentType)
	part, err := mw.CreatePart(hdr)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, f); err != nil {
		return nil, err
	}

	if len(opts.Tags) > 0 {
		raw, _ := json.Marshal(opts.Tags)
		_ = mw.WriteField("tags", string(raw))
	}
	if len(opts.Metadata) > 0 {
		raw, _ := json.Marshal(opts.Metadata)
		_ = mw.WriteField("metadata", string(raw))
	}

	if err := mw.Close(); err != nil {
		return nil, err
	}

	var doc Document
	err = c.do(ctx, "POST", "/collections/"+collectionID+"/documents",
		&requestOptions{rawBody: body, contentType: mw.FormDataContentType()}, &doc)
	if err != nil {
		return nil, err
	}
	if opts.OnProgress != nil {
		size, _ := fileSize(path)
		opts.OnProgress(size, size)
	}
	return &doc, nil
}

func (c *Client) uploadViaPresign(ctx context.Context, collectionID, path string, size int64, opts UploadOptions) (*Document, error) {
	hash, err := sha256File(path)
	if err != nil {
		return nil, err
	}
	filename := filepath.Base(path)
	contentType := guessContentType(filename)

	type uploadURLReq struct {
		Filename      string                 `json:"filename"`
		ContentType   string                 `json:"contentType"`
		FileSizeBytes int64                  `json:"fileSizeBytes"`
		ContentHash   string                 `json:"contentHash"`
		Tags          []string               `json:"tags,omitempty"`
		Metadata      map[string]interface{} `json:"metadata,omitempty"`
	}
	type uploadURLRes struct {
		DocumentID string    `json:"documentId"`
		UploadURL  *string   `json:"uploadUrl"`
		Document   *Document `json:"document,omitempty"`
	}

	reqBody := uploadURLReq{
		Filename:      filename,
		ContentType:   contentType,
		FileSizeBytes: size,
		ContentHash:   hash,
		Tags:          opts.Tags,
		Metadata:      opts.Metadata,
	}

	var urlRes uploadURLRes
	if err := c.do(ctx, "POST", "/collections/"+collectionID+"/documents/upload-url",
		&requestOptions{body: reqBody}, &urlRes); err != nil {
		return nil, err
	}

	if urlRes.UploadURL == nil {
		// Dedup hit — file already exists.
		if urlRes.Document != nil {
			if opts.OnProgress != nil {
				opts.OnProgress(size, size)
			}
			return urlRes.Document, nil
		}
		return nil, fmt.Errorf("api returned no upload URL and no document")
	}

	if err := putFileToURL(ctx, *urlRes.UploadURL, path, contentType, size, opts.OnProgress); err != nil {
		return nil, err
	}

	confirmBody := map[string]interface{}{}
	if len(opts.Tags) > 0 {
		confirmBody["tags"] = opts.Tags
	}
	if len(opts.Metadata) > 0 {
		confirmBody["metadata"] = opts.Metadata
	}

	var doc Document
	confirmReq := &requestOptions{}
	if len(confirmBody) > 0 {
		confirmReq.body = confirmBody
	}
	if err := c.do(ctx, "POST",
		fmt.Sprintf("/collections/%s/documents/%s/confirm", collectionID, urlRes.DocumentID),
		confirmReq, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

func putFileToURL(ctx context.Context, url, path, contentType string, size int64, onProgress func(int64, int64)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var body io.Reader = f
	if onProgress != nil {
		body = &progressReader{r: f, total: size, onProgress: onProgress}
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	req.ContentLength = size

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		raw, _ := io.ReadAll(res.Body)
		return fmt.Errorf("S3 PUT failed %d: %s", res.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}

type progressReader struct {
	r          io.Reader
	total      int64
	read       int64
	onProgress func(int64, int64)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	if n > 0 {
		p.read += int64(n)
		if p.onProgress != nil {
			p.onProgress(p.read, p.total)
		}
	}
	return n, err
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func fileSize(path string) (int64, error) {
	st, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return st.Size(), nil
}

func guessContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".pdf":
		return "application/pdf"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".html", ".htm":
		return "text/html"
	case ".md", ".markdown":
		return "text/markdown"
	case ".txt":
		return "text/plain"
	case ".json":
		return "application/json"
	}
	return "application/octet-stream"
}

// DocumentList is the paginated response returned by GET /collections/:id/documents.
type DocumentList struct {
	Documents []Document `json:"documents"`
	Total     int        `json:"total"`
}

func (c *Client) ListDocuments(ctx context.Context, collectionID string) ([]Document, error) {
	var page DocumentList
	if err := c.do(ctx, "GET", "/collections/"+collectionID+"/documents", nil, &page); err != nil {
		return nil, err
	}
	return page.Documents, nil
}

// ListDocumentsPaged returns the full paginated payload (with the total count).
func (c *Client) ListDocumentsPaged(ctx context.Context, collectionID string) (*DocumentList, error) {
	var page DocumentList
	if err := c.do(ctx, "GET", "/collections/"+collectionID+"/documents", nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

func (c *Client) GetDocument(ctx context.Context, documentID string) (*Document, error) {
	var out Document
	if err := c.do(ctx, "GET", "/documents/"+documentID, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetDocumentMarkdown(ctx context.Context, documentID string) (string, error) {
	var out string
	if err := c.do(ctx, "GET", "/documents/"+documentID+"/markdown", nil, &out); err != nil {
		return "", err
	}
	return out, nil
}

func (c *Client) ListSections(ctx context.Context, documentID string) ([]Section, error) {
	var out []Section
	if err := c.do(ctx, "GET", "/documents/"+documentID+"/sections", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) ListChunks(ctx context.Context, sectionID string) ([]Chunk, error) {
	var out []Chunk
	if err := c.do(ctx, "GET", "/sections/"+sectionID+"/chunks", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) ListImages(ctx context.Context, documentID string) ([]map[string]interface{}, error) {
	var out []map[string]interface{}
	if err := c.do(ctx, "GET", "/documents/"+documentID+"/images", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) DeleteDocument(ctx context.Context, documentID string) error {
	return c.do(ctx, "DELETE", "/documents/"+documentID, nil, nil)
}

// WaitForDocument long-polls until the document reaches a terminal state. The
// API endpoint times out after ~5.5 minutes.
func (c *Client) WaitForDocument(ctx context.Context, documentID string) (*Document, error) {
	var out Document
	if err := c.do(ctx, "GET", "/documents/"+documentID+"/wait", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SizeAsInt64String formats a size in bytes as the human-readable representation.
// Used by callers and exposed here for convenience.
func SizeAsInt64String(n int64) string {
	return strconv.FormatInt(n, 10)
}

package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// streamSSE opens an SSE stream and sends each `data:` payload (parsed as JSON)
// to the provided handler. The handler may return io.EOF to stop the stream
// cleanly. The function blocks until the server closes the connection, the
// context is cancelled, or the handler returns an error.
func (c *Client) streamSSE(ctx context.Context, method, path string, body any, handler func(json.RawMessage) error) error {
	url := c.BaseURL + path

	var reqBody io.Reader
	contentType := ""
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = strings.NewReader(string(buf))
		contentType = "application/json"
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "text/event-stream")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		return parseError(res)
	}

	r := bufio.NewReader(res.Body)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		if err := handler(json.RawMessage(data)); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

// ResearchEvent represents one SSE frame emitted by /research.
type ResearchEvent struct {
	Type     string           `json:"type"`
	Query    string           `json:"query,omitempty"`
	Tool     string           `json:"tool,omitempty"`
	Content  string           `json:"content,omitempty"`
	SessionID string          `json:"sessionId,omitempty"`
	Sources  []ResearchSource `json:"sources,omitempty"`
	Message  string           `json:"message,omitempty"`
}

func (c *Client) StreamResearch(ctx context.Context, collectionID, q string, opts ResearchOptions, handler func(ResearchEvent) error) error {
	body := map[string]interface{}{"q": q}
	if opts.Depth != "" {
		body["depth"] = opts.Depth
	}
	if opts.Model != "" {
		body["model"] = opts.Model
	}
	if len(opts.Tags) > 0 {
		body["tags"] = opts.Tags
	}
	if len(opts.AnyTags) > 0 {
		body["anyTags"] = opts.AnyTags
	}
	if len(opts.Metadata) > 0 {
		body["metadata"] = opts.Metadata
	}
	return c.streamSSE(ctx, "POST", "/collections/"+collectionID+"/research", body, func(raw json.RawMessage) error {
		var ev ResearchEvent
		if err := json.Unmarshal(raw, &ev); err != nil {
			return nil // skip malformed frames
		}
		return handler(ev)
	})
}

// StreamDocumentEvents subscribes to the per-collection document status SSE
// stream. Auth is via ?key=<api-key> because EventSource implementations can't
// set headers. Reconnects with exponential backoff until ctx is cancelled.
func (c *Client) StreamDocumentEvents(ctx context.Context, collectionID string, handler func(DocumentEvent) error) error {
	path := fmt.Sprintf("/collections/%s/documents/events?key=%s", collectionID, c.APIKey)
	url := c.BaseURL + path

	backoff := time.Second
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := func() error {
			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				return err
			}
			req.Header.Set("Accept", "text/event-stream")
			if c.UserAgent != "" {
				req.Header.Set("User-Agent", c.UserAgent)
			}
			res, err := c.HTTPClient.Do(req)
			if err != nil {
				return err
			}
			defer res.Body.Close()

			if res.StatusCode >= 400 {
				return parseError(res)
			}

			backoff = time.Second // reset on successful connect
			r := bufio.NewReader(res.Body)
			for {
				line, err := r.ReadString('\n')
				if err != nil {
					return err
				}
				line = strings.TrimRight(line, "\r\n")
				if line == "" || strings.HasPrefix(line, ":") {
					continue
				}
				if !strings.HasPrefix(line, "data:") {
					continue
				}
				data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				if data == "" {
					continue
				}
				var ev DocumentEvent
				if jsonErr := json.Unmarshal([]byte(data), &ev); jsonErr != nil {
					continue
				}
				if cbErr := handler(ev); cbErr != nil {
					if errors.Is(cbErr, io.EOF) {
						return io.EOF
					}
					return cbErr
				}
			}
		}()

		if errors.Is(err, io.EOF) {
			return nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		// transient — backoff and retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

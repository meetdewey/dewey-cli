// Package api is a hand-written HTTP client for the Dewey data plane.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.meetdewey.com/v1"

type Error struct {
	Status  int
	Message string
	Code    string
}

func (e *Error) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("dewey api %d (%s): %s", e.Status, e.Code, e.Message)
	}
	return fmt.Sprintf("dewey api %d: %s", e.Status, e.Message)
}

type Client struct {
	APIKey     string
	BaseURL    string
	UserAgent  string
	HTTPClient *http.Client
}

func New(apiKey, baseURL, userAgent string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &Client{
		APIKey:    apiKey,
		BaseURL:   baseURL,
		UserAgent: userAgent,
		HTTPClient: &http.Client{
			Timeout: 0,
		},
	}
}

type requestOptions struct {
	body        any
	rawBody     io.Reader
	contentType string
	query       url.Values
	headers     map[string]string
	accept      string
}

func (c *Client) do(ctx context.Context, method, path string, opts *requestOptions, out any) error {
	if opts == nil {
		opts = &requestOptions{}
	}

	u := c.BaseURL + path
	if opts.query != nil && len(opts.query) > 0 {
		u += "?" + opts.query.Encode()
	}

	var body io.Reader
	if opts.rawBody != nil {
		body = opts.rawBody
	} else if opts.body != nil {
		buf, err := json.Marshal(opts.body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		body = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	if opts.accept != "" {
		req.Header.Set("Accept", opts.accept)
	}
	if opts.contentType != "" {
		req.Header.Set("Content-Type", opts.contentType)
	} else if opts.body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range opts.headers {
		req.Header.Set(k, v)
	}

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		return parseError(res)
	}

	if res.StatusCode == http.StatusNoContent || out == nil {
		_, _ = io.Copy(io.Discard, res.Body)
		return nil
	}

	contentType := res.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "text/") {
		raw, err := io.ReadAll(res.Body)
		if err != nil {
			return err
		}
		switch dst := out.(type) {
		case *string:
			*dst = string(raw)
			return nil
		case *[]byte:
			*dst = raw
			return nil
		}
	}

	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func parseError(res *http.Response) error {
	raw, _ := io.ReadAll(res.Body)
	var payload struct {
		Message string `json:"message"`
		Error   string `json:"error"`
		Code    string `json:"code"`
	}
	_ = json.Unmarshal(raw, &payload)

	msg := payload.Message
	if msg == "" {
		msg = payload.Error
	}
	if msg == "" {
		msg = strings.TrimSpace(string(raw))
		if msg == "" {
			msg = res.Status
		}
	}
	return &Error{Status: res.StatusCode, Message: msg, Code: payload.Code}
}

// Ping issues a GET /collections request to validate connectivity + auth.
// Returns the response in raw form along with HTTP status, latency and number
// of collections returned (or -1 if the body could not be parsed).
type PingResult struct {
	Status      int
	Latency     time.Duration
	Collections int
}

func (c *Client) Ping(ctx context.Context) (*PingResult, error) {
	start := time.Now()
	u := c.BaseURL + "/collections"
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	latency := time.Since(start)

	if res.StatusCode >= 400 {
		return &PingResult{Status: res.StatusCode, Latency: latency}, parseError(res)
	}

	var cols []json.RawMessage
	if err := json.NewDecoder(res.Body).Decode(&cols); err != nil {
		return &PingResult{Status: res.StatusCode, Latency: latency, Collections: -1}, nil
	}
	return &PingResult{Status: res.StatusCode, Latency: latency, Collections: len(cols)}, nil
}

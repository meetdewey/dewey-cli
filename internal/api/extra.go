package api

import (
	"context"
	"net/url"
	"strconv"
)

// ── Provider keys ──────────────────────────────────────────────────────────

func (c *Client) ListProviderKeys(ctx context.Context) ([]ProviderKey, error) {
	var out []ProviderKey
	if err := c.do(ctx, "GET", "/provider-keys", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) CreateProviderKey(ctx context.Context, in CreateProviderKeyInput) (*ProviderKey, error) {
	var out ProviderKey
	if err := c.do(ctx, "POST", "/provider-keys", &requestOptions{body: in}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteProviderKey(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/provider-keys/"+id, nil, nil)
}

// ── Duplicates ─────────────────────────────────────────────────────────────

func (c *Client) DetectDuplicates(ctx context.Context, collectionID string) (*DuplicateDetectResult, error) {
	var out DuplicateDetectResult
	if err := c.do(ctx, "POST", "/collections/"+collectionID+"/duplicates/detect", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ListDuplicateGroups(ctx context.Context, collectionID string, limit, offset int) (*DuplicateGroupList, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	var out DuplicateGroupList
	if err := c.do(ctx, "GET", "/collections/"+collectionID+"/duplicates",
		&requestOptions{query: q}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) PromoteDuplicateCanonical(ctx context.Context, collectionID, groupID, canonicalDocumentID string) error {
	return c.do(ctx, "PATCH", "/collections/"+collectionID+"/duplicates/"+groupID,
		&requestOptions{body: map[string]string{"canonicalDocumentId": canonicalDocumentID}}, nil)
}

func (c *Client) DisbandDuplicateGroup(ctx context.Context, collectionID, groupID string) error {
	return c.do(ctx, "DELETE", "/collections/"+collectionID+"/duplicates/"+groupID, nil, nil)
}

func (c *Client) DuplicatesLatestRun(ctx context.Context, collectionID string) (*DuplicateRun, error) {
	var out DuplicateRun
	if err := c.do(ctx, "GET", "/collections/"+collectionID+"/duplicates/runs/latest", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ── Contradictions ─────────────────────────────────────────────────────────

func (c *Client) DetectContradictions(ctx context.Context, collectionID string) (*ContradictionDetectResult, error) {
	var out ContradictionDetectResult
	if err := c.do(ctx, "POST", "/collections/"+collectionID+"/contradictions/detect", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ListContradictions(ctx context.Context, collectionID, severity, status string, limit int) (*ContradictionList, error) {
	q := url.Values{}
	if severity != "" {
		q.Set("severity", severity)
	}
	if status != "" {
		q.Set("status", status)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var out ContradictionList
	if err := c.do(ctx, "GET", "/collections/"+collectionID+"/contradictions",
		&requestOptions{query: q}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DismissContradiction(ctx context.Context, collectionID, contradictionID string) error {
	return c.do(ctx, "PATCH", "/collections/"+collectionID+"/contradictions/"+contradictionID,
		&requestOptions{body: map[string]string{"status": "dismissed"}}, nil)
}

func (c *Client) ApplyContradictionInstruction(ctx context.Context, collectionID, contradictionID, instruction string) error {
	body := map[string]string{}
	if instruction != "" {
		body["instruction"] = instruction
	}
	return c.do(ctx, "POST",
		"/collections/"+collectionID+"/contradictions/"+contradictionID+"/apply-instruction",
		&requestOptions{body: body}, nil)
}

func (c *Client) ContradictionsLatestRun(ctx context.Context, collectionID string) (*ContradictionRun, error) {
	var out ContradictionRun
	if err := c.do(ctx, "GET", "/collections/"+collectionID+"/contradictions/runs/latest", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ── Claims ─────────────────────────────────────────────────────────────────

func (c *Client) DocumentClaims(ctx context.Context, documentID string, minImportance int) (*DocumentClaims, error) {
	q := url.Values{}
	if minImportance > 0 {
		q.Set("minImportance", strconv.Itoa(minImportance))
	}
	var out DocumentClaims
	if err := c.do(ctx, "GET", "/documents/"+documentID+"/claims",
		&requestOptions{query: q}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

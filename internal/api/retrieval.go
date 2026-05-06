package api

import "context"

type QueryOptions struct {
	Limit    int                    `json:"limit,omitempty"`
	Tags     []string               `json:"tags,omitempty"`
	AnyTags  []string               `json:"anyTags,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type queryRequest struct {
	Q string `json:"q"`
	QueryOptions
}

func (c *Client) Query(ctx context.Context, collectionID, q string, opts QueryOptions) ([]RetrievalResult, error) {
	body := queryRequest{Q: q, QueryOptions: opts}
	var out []RetrievalResult
	if err := c.do(ctx, "POST", "/collections/"+collectionID+"/query",
		&requestOptions{body: body}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

type ScanOptions struct {
	TopK     int                    `json:"top_k,omitempty"`
	Tags     []string               `json:"tags,omitempty"`
	AnyTags  []string               `json:"anyTags,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type scanRequest struct {
	Query string `json:"query"`
	ScanOptions
}

func (c *Client) ScanSections(ctx context.Context, collectionID, q string, opts ScanOptions) (*SectionScanResponse, error) {
	body := scanRequest{Query: q, ScanOptions: opts}
	var out SectionScanResponse
	if err := c.do(ctx, "POST", "/collections/"+collectionID+"/sections/scan",
		&requestOptions{body: body}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ResearchSync(ctx context.Context, collectionID, q string, opts ResearchOptions) (*ResearchResult, error) {
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
	var out ResearchResult
	if err := c.do(ctx, "POST", "/collections/"+collectionID+"/research/sync",
		&requestOptions{body: body}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

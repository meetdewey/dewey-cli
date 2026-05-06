package api

import (
	"context"
	"fmt"
)

func (c *Client) ListCollections(ctx context.Context) ([]Collection, error) {
	var out []Collection
	if err := c.do(ctx, "GET", "/collections", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetCollection(ctx context.Context, id string) (*Collection, error) {
	var out Collection
	if err := c.do(ctx, "GET", "/collections/"+id, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CreateCollection(ctx context.Context, in CreateCollectionInput) (*Collection, error) {
	var out Collection
	if err := c.do(ctx, "POST", "/collections", &requestOptions{body: in}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateCollection(ctx context.Context, id string, in UpdateCollectionInput) (*Collection, error) {
	var out Collection
	if err := c.do(ctx, "PATCH", "/collections/"+id, &requestOptions{body: in}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteCollection(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/collections/"+id, nil, nil)
}

func (c *Client) CollectionStats(ctx context.Context, id string) (*CollectionStats, error) {
	var out CollectionStats
	if err := c.do(ctx, "GET", "/collections/"+id+"/stats", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ResolveCollection accepts either a collection ID (UUID or col_* prefix) or
// a name and returns the matching collection. Names match case-insensitively;
// if multiple names match, returns an error listing the candidates.
func (c *Client) ResolveCollection(ctx context.Context, idOrName string) (*Collection, error) {
	if looksLikeID(idOrName) {
		return c.GetCollection(ctx, idOrName)
	}
	cols, err := c.ListCollections(ctx)
	if err != nil {
		return nil, err
	}
	var matches []Collection
	for _, col := range cols {
		if equalFold(col.Name, idOrName) {
			matches = append(matches, col)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no collection matched %q (try `dewey collections list`)", idOrName)
	}
	if len(matches) > 1 {
		var names []string
		for _, m := range matches {
			names = append(names, m.ID)
		}
		return nil, fmt.Errorf("multiple collections matched %q: %v — pass the collection ID instead", idOrName, names)
	}
	return &matches[0], nil
}

// looksLikeID reports whether s appears to be a collection identifier (either
// a UUID or a col_* prefixed string). A loose check is fine — ResolveCollection
// falls back to a name lookup on a 404.
func looksLikeID(s string) bool {
	if len(s) > 4 && s[:4] == "col_" {
		return true
	}
	// UUID v4-ish: 8-4-4-4-12 hex with hyphens.
	if len(s) == 36 && s[8] == '-' && s[13] == '-' && s[18] == '-' && s[23] == '-' {
		return true
	}
	return false
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca := a[i]
		cb := b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

package api

import (
	"context"
	"encoding/json"
)

// InvokeAgentSync calls POST /orgs/:orgId/projects/:projectId/agents/:slug/invoke/sync.
func (c *Client) InvokeAgentSync(ctx context.Context, orgID, projectID, agentSlug, query string) (*AgentInvokeResult, error) {
	path := "/orgs/" + orgID + "/projects/" + projectID + "/agents/" + agentSlug + "/invoke/sync"
	var out AgentInvokeResult
	if err := c.do(ctx, "POST", path, &requestOptions{
		body: map[string]string{"query": query},
	}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// StreamAgent streams an agent run via POST /orgs/:orgId/projects/:projectId/agents/:slug/invoke.
// Returning io.EOF from the handler terminates the stream cleanly.
func (c *Client) StreamAgent(ctx context.Context, orgID, projectID, agentSlug, query string, handler func(AgentRunEvent) error) error {
	path := "/orgs/" + orgID + "/projects/" + projectID + "/agents/" + agentSlug + "/invoke"
	return c.streamSSE(ctx, "POST", path, map[string]string{"query": query}, func(raw json.RawMessage) error {
		var ev AgentRunEvent
		if err := json.Unmarshal(raw, &ev); err != nil {
			return nil // skip malformed frames
		}
		return handler(ev)
	})
}

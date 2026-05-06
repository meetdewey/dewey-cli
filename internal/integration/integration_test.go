// Package integration runs the CLI's API client against a real Dewey backend.
//
// These tests are skipped unless CLI_INTEGRATION=1. They expect:
//
//   - DEWEY_API_KEY to be set to a valid project key
//   - DEWEY_BASE_URL (default: http://localhost:3000/v1) to point at a running server
//
// In CI we run them against `pnpm dev` started in a sibling job; locally you
// can run `pnpm dev` and invoke `CLI_INTEGRATION=1 go test ./internal/integration/...`.
package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/lambdabaa/dewey/apps/cli/internal/api"
)

func skipUnlessEnabled(t *testing.T) {
	t.Helper()
	if os.Getenv("CLI_INTEGRATION") != "1" {
		t.Skip("CLI_INTEGRATION not set")
	}
	if os.Getenv("DEWEY_API_KEY") == "" {
		t.Skip("DEWEY_API_KEY not set")
	}
}

func newClient(t *testing.T) *api.Client {
	t.Helper()
	baseURL := os.Getenv("DEWEY_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:3000/v1"
	}
	return api.New(os.Getenv("DEWEY_API_KEY"), baseURL, "dewey-cli-integration")
}

func TestIntegration_Ping(t *testing.T) {
	skipUnlessEnabled(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res, err := newClient(t).Ping(ctx)
	if err != nil {
		t.Fatalf("ping: %v", err)
	}
	if res.Status != 200 {
		t.Errorf("expected 200, got %d", res.Status)
	}
	if res.Latency <= 0 {
		t.Errorf("expected non-zero latency")
	}
}

func TestIntegration_ListCollections(t *testing.T) {
	skipUnlessEnabled(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cols, err := newClient(t).ListCollections(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	t.Logf("found %d collections", len(cols))
}

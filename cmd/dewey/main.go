// Command dewey is the Dewey CLI entrypoint.
//
// Build with:
//
//	go build -ldflags="-X github.com/lambdabaa/dewey/apps/cli/internal/version.Version=$VERSION \
//	                   -X github.com/lambdabaa/dewey/apps/cli/internal/version.Commit=$SHA \
//	                   -s -w" -trimpath ./cmd/dewey
package main

import (
	"os"

	"github.com/lambdabaa/dewey/apps/cli/internal/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}

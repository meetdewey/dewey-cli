// Package version exposes the build-time CLI version metadata.
package version

// Set via -ldflags="-X github.com/lambdabaa/dewey/apps/cli/internal/version.Version=..."
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// APIVersion is the API surface this CLI was built against.
const APIVersion = "v1"

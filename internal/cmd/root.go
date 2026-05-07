// Package cmd assembles the dewey command tree using Cobra.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/lambdabaa/dewey/apps/cli/internal/api"
	"github.com/lambdabaa/dewey/apps/cli/internal/config"
	"github.com/lambdabaa/dewey/apps/cli/internal/output"
	"github.com/lambdabaa/dewey/apps/cli/internal/version"
	"github.com/spf13/cobra"
)

// Exit codes follow sysexits.h conventions.
const (
	ExitOK             = 0
	ExitGenericError   = 1
	ExitUsageError     = 2
	ExitInputError     = 64
	ExitUnavailable    = 69
	ExitTempFail       = 75
	ExitNoPermission   = 77
)

// AppContext holds shared dependencies threaded into every command.
type AppContext struct {
	Client     *api.Client
	Config     *config.Config
	State      *config.State
	Renderer   *output.Renderer
	Collection string // resolved collection (id) when -c is given/cached
	ProjectID  string // resolved from --project-id flag, config, or env
}

type rootFlags struct {
	apiKey       string
	baseURL      string
	jsonMode     bool
	noColor      bool
	color        string
	collection   string
	projectID    string
	versionShort bool
}

// Execute runs the root command and returns the exit code.
func Execute() int {
	flags := &rootFlags{}

	root := &cobra.Command{
		Use:   "dewey",
		Short: "Real-time document backend for AI applications",
		Long: `dewey — terminal-native access to the Dewey API.

Authenticate with DEWEY_API_KEY. See ` + "`dewey doctor`" + ` to validate your setup.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       fmt.Sprintf("%s (api %s, commit %s)", version.Version, version.APIVersion, version.Commit),
	}

	root.PersistentFlags().StringVar(&flags.apiKey, "api-key", "", "Dewey API key (prefer "+config.EnvAPIKey+" env var)")
	root.PersistentFlags().StringVar(&flags.baseURL, "base-url", "", "Override the API base URL")
	root.PersistentFlags().BoolVar(&flags.jsonMode, "json", false, "Output machine-readable JSON")
	root.PersistentFlags().BoolVar(&flags.noColor, "no-color", false, "Disable ANSI color")
	root.PersistentFlags().StringVar(&flags.color, "color", "", "Color mode: auto|always|never")
	root.PersistentFlags().StringVarP(&flags.collection, "collection", "c", "", "Collection ID or name (defaults to last-used)")
	root.PersistentFlags().StringVarP(&flags.projectID, "project-id", "p", "", "Project ID (saved to config after first use)")

	// Build a function that lazily constructs the AppContext for each subcommand.
	makeAppCtx := func(requireAuth bool) (*AppContext, error) {
		cfg, err := config.Load()
		if err != nil {
			return nil, err
		}
		state, err := config.LoadState()
		if err != nil {
			return nil, err
		}
		colorMode := flags.color
		if flags.noColor {
			colorMode = "never"
		}
		if cfg.Color != "" && colorMode == "" {
			colorMode = cfg.Color
		}
		jsonMode := flags.jsonMode
		if !jsonMode && cfg.Output == "json" {
			jsonMode = true
		}
		r := output.New(jsonMode, true, colorMode)

		apiKey := config.ResolveAPIKey(flags.apiKey)
		if requireAuth && apiKey == "" {
			return nil, errMissingAPIKey
		}

		baseURL := config.ResolveBaseURL(flags.baseURL, cfg)
		client := api.New(apiKey, baseURL, "dewey-cli/"+version.Version)

		// One-time telemetry notice on first run.
		if !state.TelemetryNoticeShown && config.TelemetryEnabled() && r.TTY {
			fmt.Fprintln(r.Err, r.Dim("Anonymous usage telemetry is on. Disable with DEWEY_TELEMETRY=0."))
			state.TelemetryNoticeShown = true
			_ = state.Save()
		}

		ctx := &AppContext{
			Client:   client,
			Config:   cfg,
			State:    state,
			Renderer: r,
		}

		// Resolve --project-id: flag > config.
		if flags.projectID != "" {
			ctx.ProjectID = flags.projectID
		} else if cfg.ProjectID != "" {
			ctx.ProjectID = cfg.ProjectID
		}

		// Resolve --collection: either explicit, last-used, or empty.
		if flags.collection != "" {
			ctx.Collection = flags.collection
		} else if state.LastCollection != "" {
			ctx.Collection = state.LastCollection
		}

		return ctx, nil
	}

	// Wire up all subcommands.
	root.AddCommand(newUploadCmd(makeAppCtx))
	root.AddCommand(newQueryCmd(makeAppCtx))
	root.AddCommand(newScanCmd(makeAppCtx))
	root.AddCommand(newResearchCmd(makeAppCtx))
	root.AddCommand(newWatchCmd(makeAppCtx))
	root.AddCommand(newDoctorCmd(makeAppCtx))
	root.AddCommand(newVersionCmd())
	root.AddCommand(newCollectionsCmd(makeAppCtx))
	root.AddCommand(newDocsCmd(makeAppCtx))
	root.AddCommand(newConfigCmd(makeAppCtx))
	root.AddCommand(newProviderKeysCmd(makeAppCtx))
	root.AddCommand(newDuplicatesCmd(makeAppCtx))
	root.AddCommand(newContradictionsCmd(makeAppCtx))
	root.AddCommand(newClaimsCmd(makeAppCtx))
	root.AddCommand(newAgentsCmd(makeAppCtx))

	// Cancel context on Ctrl-C.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	root.SetContext(ctx)

	if err := root.ExecuteContext(ctx); err != nil {
		return handleError(err)
	}
	return ExitOK
}

var errMissingAPIKey = errors.New("missing API key")

func handleError(err error) int {
	// Determine destination + exit code.
	if errors.Is(err, errMissingAPIKey) {
		fmt.Fprintln(os.Stderr, "error: missing API key — set DEWEY_API_KEY or pass --api-key")
		return ExitNoPermission
	}

	var apiErr *api.Error
	if errors.As(err, &apiErr) {
		fmt.Fprintf(os.Stderr, "error: %s\n", apiErr.Message)
		switch apiErr.Status {
		case 401, 403:
			fmt.Fprintln(os.Stderr, "  hint: check that DEWEY_API_KEY is set to a valid project key")
			return ExitNoPermission
		case 404:
			fmt.Fprintln(os.Stderr, "  hint: list available collections with `dewey collections list`")
			return ExitInputError
		case 429:
			fmt.Fprintln(os.Stderr, "  hint: see https://meetdewey.com/pricing to upgrade")
			return ExitTempFail
		case 503, 502, 504:
			return ExitUnavailable
		case 408:
			return ExitTempFail
		}
		return ExitGenericError
	}

	fmt.Fprintf(os.Stderr, "error: %s\n", err.Error())
	return ExitGenericError
}

// rememberCollection persists `colID` as the last-used collection.
func (a *AppContext) rememberCollection(colID string) {
	if a.State == nil {
		return
	}
	if a.State.LastCollection == colID {
		return
	}
	a.State.LastCollection = colID
	_ = a.State.Save()
}

// resolveCollection turns a user-supplied id-or-name into a Collection.
// If `slug` is empty, falls back to AppContext.Collection (which already
// reflects --collection or last-used). Returns an error suitable for surfacing.
func (a *AppContext) resolveCollection(ctx context.Context, slug string) (*api.Collection, error) {
	if slug == "" {
		slug = a.Collection
	}
	if slug == "" {
		return nil, errors.New("no collection specified — pass -c <name|id>")
	}
	col, err := a.Client.ResolveCollection(ctx, slug)
	if err != nil {
		return nil, err
	}
	a.rememberCollection(col.ID)
	return col, nil
}

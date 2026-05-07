package cmd

import (
	"fmt"
	"strings"

	"github.com/lambdabaa/dewey/apps/cli/internal/api"
	"github.com/spf13/cobra"
)

func newAgentsCmd(makeAppCtx func(bool) (*AppContext, error)) *cobra.Command {
	c := &cobra.Command{
		Use:   "agents",
		Short: "Invoke saved agents",
	}

	c.AddCommand(newAgentsInvokeCmd(makeAppCtx))
	return c
}

func newAgentsInvokeCmd(makeAppCtx func(bool) (*AppContext, error)) *cobra.Command {
	c := &cobra.Command{
		Use:   "invoke <agent-slug> <query>",
		Short: "Invoke a saved agent (streaming)",
		Long: `Run a saved agent against a query. The model issues tool calls to search and
read sections, then produces a cited markdown answer.

Org and project IDs are UUIDs (visible in the dashboard URL or 'dewey config get').
Pass --org / --project, or set them once with:

  dewey config set org_id <id>
  dewey config set project_id <id>

By default, output is streamed when stdout is a TTY and returned as a single
JSON envelope when piped. Override with --stream / --no-stream.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := makeAppCtx(true)
			if err != nil {
				return err
			}
			slug, query := args[0], args[1]

			orgID, _ := cmd.Flags().GetString("org")
			if orgID == "" {
				orgID = app.Config.OrgID
			}
			if orgID == "" {
				return fmt.Errorf(
					"org ID required — pass --org <id> or run:\n" +
						"  dewey config set org_id <id>\n" +
						"Your org ID is visible in the dashboard URL.")
			}

			projectID := app.ProjectID
			if inline, _ := cmd.Flags().GetString("project"); inline != "" {
				projectID = inline
			}
			if projectID == "" {
				return fmt.Errorf(
					"project ID required — pass --project <id>, -p <id>, or run:\n" +
						"  dewey config set project_id <id>")
			}

			r := app.Renderer
			noStream, _ := cmd.Flags().GetBool("no-stream")
			stream, _ := cmd.Flags().GetBool("stream")
			useStream := r.TTY
			if cmd.Flags().Changed("stream") {
				useStream = stream
			}
			if cmd.Flags().Changed("no-stream") && noStream {
				useStream = false
			}

			if !useStream {
				res, err := app.Client.InvokeAgentSync(cmd.Context(), orgID, projectID, slug, query)
				if err != nil {
					return err
				}
				if r.IsJSON() {
					return r.JSON(res)
				}
				r.Println(res.Response)
				printAgentSources(app, res.Sources)
				for _, w := range res.Warnings {
					r.Statusln(r.Yellow("warning:"), w)
				}
				return nil
			}

			// Streaming path.
			var sources []api.AgentSource
			err = app.Client.StreamAgent(cmd.Context(), orgID, projectID, slug, query, func(ev api.AgentRunEvent) error {
				if r.IsJSON() {
					_ = r.JSONLine(ev)
				}
				switch ev.Type {
				case "tool_call":
					if !r.IsJSON() {
						label := ev.Tool
						if label == "" {
							label = "tool"
						}
						q := summarizeToolArgs(ev.Args)
						r.Statusln(r.Dim(fmt.Sprintf("→ %s%s", label, q)))
					}
				case "chunk":
					if !r.IsJSON() {
						r.Print(ev.Content)
					}
				case "done":
					sources = ev.Sources
				case "warning":
					if !r.IsJSON() && ev.Message != "" {
						r.Statusln(r.Yellow("warning:"), ev.Message)
					}
				case "error":
					if ev.Message != "" {
						return fmt.Errorf("agent stream error: %s", ev.Message)
					}
				}
				return nil
			})
			if err != nil {
				return err
			}
			if !r.IsJSON() {
				r.Println()
				printAgentSources(app, sources)
			}
			return nil
		},
	}
	c.Flags().String("org", "", "Org ID (UUID; defaults to org_id from config)")
	c.Flags().String("project", "", "Project ID (UUID; defaults to --project-id / config)")
	c.Flags().Bool("stream", true, "Stream the answer token-by-token")
	c.Flags().Bool("no-stream", false, "Disable streaming; return a single response")
	return c
}

// summarizeToolArgs renders a compact ": <key=value>" suffix for tool_call
// status lines. Returns an empty string when nothing useful is present.
func summarizeToolArgs(args map[string]interface{}) string {
	if len(args) == 0 {
		return ""
	}
	// Prefer the most user-meaningful field — the search query — when present.
	for _, k := range []string{"query", "q", "filename"} {
		if v, ok := args[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return ": " + s
			}
		}
	}
	return ""
}

func printAgentSources(app *AppContext, sources []api.AgentSource) {
	r := app.Renderer
	if len(sources) == 0 {
		return
	}
	r.Println()
	r.Section("— Sources ———————————————————————————————————————")
	dedup := make(map[string]int)
	for _, s := range sources {
		key := s.Filename + " § " + s.SectionTitle
		if _, seen := dedup[key]; seen {
			continue
		}
		dedup[key] = len(dedup) + 1
		r.Printf("[%d] %s\n", dedup[key], strings.TrimSpace(key))
	}
}

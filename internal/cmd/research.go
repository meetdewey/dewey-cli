package cmd

import (
	"fmt"
	"strings"

	"github.com/lambdabaa/dewey/apps/cli/internal/api"
	"github.com/spf13/cobra"
)

func newResearchCmd(makeAppCtx func(bool) (*AppContext, error)) *cobra.Command {
	c := &cobra.Command{
		Use:   "research [collection] <question>",
		Short: "Run an agentic research query (streaming)",
		Long: `Run a research query against a collection. The model issues tool calls to
search and read sections, then produces a cited markdown answer.

By default, output is streamed when stdout is a TTY and returned as a single
JSON object when piped. Override with --stream / --no-stream.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := makeAppCtx(true)
			if err != nil {
				return err
			}
			var slug, q string
			if len(args) == 2 {
				slug, q = args[0], args[1]
			} else {
				q = args[0]
			}
			col, err := app.resolveCollection(cmd.Context(), slug)
			if err != nil {
				return err
			}

			depth, _ := cmd.Flags().GetString("depth")
			model, _ := cmd.Flags().GetString("model")
			tags, _ := cmd.Flags().GetStringSlice("tag")
			anyTags, _ := cmd.Flags().GetStringSlice("any-tag")
			metaRaw, _ := cmd.Flags().GetStringArray("metadata")
			metadata, err := parseKVPairs(metaRaw)
			if err != nil {
				return err
			}

			r := app.Renderer
			streamFlag := cmd.Flags().Lookup("stream")
			noStream, _ := cmd.Flags().GetBool("no-stream")
			stream, _ := cmd.Flags().GetBool("stream")
			useStream := r.TTY
			if streamFlag != nil && cmd.Flags().Changed("stream") {
				useStream = stream
			}
			if cmd.Flags().Changed("no-stream") && noStream {
				useStream = false
			}

			opts := api.ResearchOptions{
				Depth: depth, Model: model, Tags: tags, AnyTags: anyTags, Metadata: metadata,
			}

			if !useStream {
				res, err := app.Client.ResearchSync(cmd.Context(), col.ID, q, opts)
				if err != nil {
					return err
				}
				if r.IsJSON() {
					return r.JSON(res)
				}
				r.Println(res.Answer)
				printCitations(app, res.Sources)
				return nil
			}

			// Streaming path.
			var sources []api.ResearchSource
			err = app.Client.StreamResearch(cmd.Context(), col.ID, q, opts, func(ev api.ResearchEvent) error {
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
						r.Statusln(r.Dim(fmt.Sprintf("→ %s: %s", label, ev.Query)))
					}
				case "chunk":
					if !r.IsJSON() {
						r.Print(ev.Content)
					}
				case "done":
					sources = ev.Sources
				case "error":
					if ev.Message != "" {
						return fmt.Errorf("research stream error: %s", ev.Message)
					}
				}
				return nil
			})
			if err != nil {
				return err
			}
			if !r.IsJSON() {
				r.Println()
				printCitations(app, sources)
			}
			return nil
		},
	}
	c.Flags().String("depth", "", "quick | balanced | deep | exhaustive")
	c.Flags().String("model", "", "Model override (e.g. gpt-4o-mini)")
	c.Flags().StringSlice("tag", nil, "Require ALL of these tags (repeatable)")
	c.Flags().StringSlice("any-tag", nil, "Match ANY of these tags (repeatable)")
	c.Flags().StringArray("metadata", nil, "Metadata key=value filter (repeatable)")
	c.Flags().Bool("stream", true, "Stream the answer token-by-token")
	c.Flags().Bool("no-stream", false, "Disable streaming; return a single response")
	return c
}

func printCitations(app *AppContext, sources []api.ResearchSource) {
	r := app.Renderer
	if len(sources) == 0 {
		return
	}
	r.Println()
	r.Section("— Citations —————————————————————————————————————")
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

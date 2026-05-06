package cmd

import (
	"fmt"

	"github.com/lambdabaa/dewey/apps/cli/internal/api"
	"github.com/spf13/cobra"
)

func newQueryCmd(makeAppCtx func(bool) (*AppContext, error)) *cobra.Command {
	c := &cobra.Command{
		Use:   "query [collection] <text>",
		Short: "Hybrid retrieval over a collection",
		Long: `Run a hybrid (BM25 + vector) retrieval query against a collection.

If a collection is not given as the first positional argument, --collection / -c
or the last-used collection is used.`,
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

			limit, _ := cmd.Flags().GetInt("top-k")
			tags, _ := cmd.Flags().GetStringSlice("tag")
			anyTags, _ := cmd.Flags().GetStringSlice("any-tag")
			metaRaw, _ := cmd.Flags().GetStringArray("metadata")
			metadata, err := parseKVPairs(metaRaw)
			if err != nil {
				return err
			}

			results, err := app.Client.Query(cmd.Context(), col.ID, q, api.QueryOptions{
				Limit:    limit,
				Tags:     tags,
				AnyTags:  anyTags,
				Metadata: metadata,
			})
			if err != nil {
				return err
			}

			r := app.Renderer
			if r.IsJSON() {
				return r.JSON(results)
			}
			if len(results) == 0 {
				r.Statusln(r.Dim("No results."))
				return nil
			}
			t := r.Table("SCORE", "DOC", "SECTION", "PREVIEW")
			for _, res := range results {
				t.Row(
					fmt.Sprintf("%.3f", res.Score),
					res.Document.Filename,
					truncate(res.Section.Title, 40),
					truncate(res.Chunk.Content, 80),
				)
			}
			t.Render()
			return nil
		},
	}
	c.Flags().IntP("top-k", "k", 10, "Maximum number of results")
	c.Flags().StringSlice("tag", nil, "Require ALL of these tags (repeatable)")
	c.Flags().StringSlice("any-tag", nil, "Match ANY of these tags (repeatable)")
	c.Flags().StringArray("metadata", nil, "Metadata key=value filter (repeatable)")
	return c
}

package cmd

import (
	"fmt"

	"github.com/lambdabaa/dewey/apps/cli/internal/api"
	"github.com/spf13/cobra"
)

func newScanCmd(makeAppCtx func(bool) (*AppContext, error)) *cobra.Command {
	c := &cobra.Command{
		Use:   "scan [collection] <text>",
		Short: "Lightweight section-summary scan",
		Args:  cobra.RangeArgs(1, 2),
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
			topK, _ := cmd.Flags().GetInt("top-k")
			tags, _ := cmd.Flags().GetStringSlice("tag")
			anyTags, _ := cmd.Flags().GetStringSlice("any-tag")
			res, err := app.Client.ScanSections(cmd.Context(), col.ID, q, api.ScanOptions{
				TopK: topK, Tags: tags, AnyTags: anyTags,
			})
			if err != nil {
				return err
			}
			r := app.Renderer
			if r.IsJSON() {
				return r.JSON(res)
			}
			if len(res.Results) == 0 {
				r.Statusln(r.Dim("No matching sections."))
				return nil
			}
			t := r.Table("SCORE", "DOC", "SECTION", "SUMMARY")
			for _, item := range res.Results {
				summary := ""
				if item.Section.Summary != nil {
					summary = *item.Section.Summary
				}
				t.Row(
					fmt.Sprintf("%.3f", item.Score),
					item.Document.Filename,
					truncate(item.Section.Title, 40),
					truncate(summary, 80),
				)
			}
			t.Render()
			return nil
		},
	}
	c.Flags().IntP("top-k", "k", 10, "Maximum number of results")
	c.Flags().StringSlice("tag", nil, "Require ALL of these tags (repeatable)")
	c.Flags().StringSlice("any-tag", nil, "Match ANY of these tags (repeatable)")
	return c
}

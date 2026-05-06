package cmd

import (
	"fmt"

	"github.com/lambdabaa/dewey/apps/cli/internal/api"
	"github.com/spf13/cobra"
)

func newWatchCmd(makeAppCtx func(bool) (*AppContext, error)) *cobra.Command {
	c := &cobra.Command{
		Use:   "watch [collection]",
		Short: "Tail the document-status SSE stream for a collection",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := makeAppCtx(true)
			if err != nil {
				return err
			}
			var slug string
			if len(args) == 1 {
				slug = args[0]
			}
			col, err := app.resolveCollection(cmd.Context(), slug)
			if err != nil {
				return err
			}
			docFilter, _ := cmd.Flags().GetString("doc")

			r := app.Renderer
			r.Statusln(r.Dim(fmt.Sprintf("Watching %s. Ctrl-C to exit.", col.Name)))

			return app.Client.StreamDocumentEvents(cmd.Context(), col.ID, func(ev api.DocumentEvent) error {
				if docFilter != "" && ev.DocumentID != docFilter {
					return nil
				}
				if r.IsJSON() {
					return r.JSONLine(ev)
				}
				r.Println(formatDocEvent(r, ev, ev.Filename))
				return nil
			})
		},
	}
	c.Flags().String("doc", "", "Filter to a single document ID")
	return c
}

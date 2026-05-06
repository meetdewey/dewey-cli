package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newDuplicatesCmd(makeAppCtx func(bool) (*AppContext, error)) *cobra.Command {
	c := &cobra.Command{
		Use:   "duplicates",
		Short: "Manage near-duplicate detection",
	}

	c.AddCommand(&cobra.Command{
		Use:   "detect [collection]",
		Short: "Trigger an asynchronous deduplication run",
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
			res, err := app.Client.DetectDuplicates(cmd.Context(), col.ID)
			if err != nil {
				return err
			}
			r := app.Renderer
			if r.IsJSON() {
				return r.JSON(res)
			}
			r.Statusln(r.Tick(), "enqueued run", res.RunID, fmt.Sprintf("(%d jobs)", res.JobsEnqueued))
			return nil
		},
	})

	listCmd := &cobra.Command{
		Use:   "list [collection]",
		Short: "List duplicate groups",
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
			limit, _ := cmd.Flags().GetInt("limit")
			offset, _ := cmd.Flags().GetInt("offset")
			groups, err := app.Client.ListDuplicateGroups(cmd.Context(), col.ID, limit, offset)
			if err != nil {
				return err
			}
			r := app.Renderer
			if r.IsJSON() {
				return r.JSON(groups)
			}
			if len(groups.Items) == 0 {
				r.Statusln(r.Dim("No duplicate groups."))
				return nil
			}
			for _, g := range groups.Items {
				r.Section(fmt.Sprintf("%s  (%d members; canonical %s)", g.ID, len(g.Members), g.CanonicalDocumentID))
				for _, m := range g.Members {
					rel := "—"
					if m.Relationship != nil {
						rel = *m.Relationship
					}
					r.Println("  ", m.ID, m.Filename, r.Dim(rel))
				}
			}
			return nil
		},
	}
	listCmd.Flags().Int("limit", 50, "Maximum groups to return")
	listCmd.Flags().Int("offset", 0, "Pagination offset")
	c.AddCommand(listCmd)

	c.AddCommand(&cobra.Command{
		Use:   "resolve <group-id> <canonical-document-id>",
		Short: "Promote a member to canonical",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := makeAppCtx(true)
			if err != nil {
				return err
			}
			col, err := app.resolveCollection(cmd.Context(), "")
			if err != nil {
				return err
			}
			if err := app.Client.PromoteDuplicateCanonical(cmd.Context(), col.ID, args[0], args[1]); err != nil {
				return err
			}
			app.Renderer.Statusln(app.Renderer.Tick(), "promoted", args[1])
			return nil
		},
	})

	c.AddCommand(&cobra.Command{
		Use:   "dismiss <group-id>",
		Short: "Disband a duplicate group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := makeAppCtx(true)
			if err != nil {
				return err
			}
			col, err := app.resolveCollection(cmd.Context(), "")
			if err != nil {
				return err
			}
			if err := app.Client.DisbandDuplicateGroup(cmd.Context(), col.ID, args[0]); err != nil {
				return err
			}
			app.Renderer.Statusln(app.Renderer.Tick(), "disbanded", args[0])
			return nil
		},
	})

	return c
}

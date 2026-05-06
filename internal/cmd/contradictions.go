package cmd

import (
	"github.com/spf13/cobra"
)

func newContradictionsCmd(makeAppCtx func(bool) (*AppContext, error)) *cobra.Command {
	c := &cobra.Command{
		Use:   "contradictions",
		Short: "Manage contradiction detection",
	}

	c.AddCommand(&cobra.Command{
		Use:   "detect [collection]",
		Short: "Trigger contradiction detection",
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
			res, err := app.Client.DetectContradictions(cmd.Context(), col.ID)
			if err != nil {
				return err
			}
			r := app.Renderer
			if r.IsJSON() {
				return r.JSON(res)
			}
			r.Statusln(r.Tick(), "enqueued run", res.RunID)
			return nil
		},
	})

	listCmd := &cobra.Command{
		Use:   "list [collection]",
		Short: "List detected contradictions",
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
			severity, _ := cmd.Flags().GetString("severity")
			status, _ := cmd.Flags().GetString("status")
			limit, _ := cmd.Flags().GetInt("limit")

			out, err := app.Client.ListContradictions(cmd.Context(), col.ID, severity, status, limit)
			if err != nil {
				return err
			}
			r := app.Renderer
			if r.IsJSON() {
				return r.JSON(out)
			}
			if len(out.Items) == 0 {
				r.Statusln(r.Dim("No contradictions."))
				return nil
			}
			for _, item := range out.Items {
				r.Section(item.ID + "  (" + item.Severity + " · " + item.Status + ")")
				r.Println(item.Explanation)
				if item.SuggestedInstruction != nil && *item.SuggestedInstruction != "" {
					r.Println(r.Dim("  → " + *item.SuggestedInstruction))
				}
			}
			return nil
		},
	}
	listCmd.Flags().String("severity", "", "low | medium | high")
	listCmd.Flags().String("status", "active", "active | dismissed | applied")
	listCmd.Flags().Int("limit", 0, "Maximum results")
	c.AddCommand(listCmd)

	applyCmd := &cobra.Command{
		Use:   "apply <id>",
		Short: "Apply the suggested resolution instruction (or a custom one)",
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
			instruction, _ := cmd.Flags().GetString("instruction")
			if err := app.Client.ApplyContradictionInstruction(cmd.Context(), col.ID, args[0], instruction); err != nil {
				return err
			}
			app.Renderer.Statusln(app.Renderer.Tick(), "applied", args[0])
			return nil
		},
	}
	applyCmd.Flags().String("instruction", "", "Override the suggested instruction")
	c.AddCommand(applyCmd)

	c.AddCommand(&cobra.Command{
		Use:   "dismiss <id>",
		Short: "Dismiss a contradiction (mark as ignored)",
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
			if err := app.Client.DismissContradiction(cmd.Context(), col.ID, args[0]); err != nil {
				return err
			}
			app.Renderer.Statusln(app.Renderer.Tick(), "dismissed", args[0])
			return nil
		},
	})

	return c
}

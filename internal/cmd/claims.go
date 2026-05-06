package cmd

import (
	"github.com/spf13/cobra"
)

func newClaimsCmd(makeAppCtx func(bool) (*AppContext, error)) *cobra.Command {
	c := &cobra.Command{
		Use:   "claims",
		Short: "Read extracted factual claims",
	}

	listCmd := &cobra.Command{
		Use:   "list <document-id>",
		Short: "List claims extracted from a document",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := makeAppCtx(true)
			if err != nil {
				return err
			}
			minImportance, _ := cmd.Flags().GetInt("min-importance")
			out, err := app.Client.DocumentClaims(cmd.Context(), args[0], minImportance)
			if err != nil {
				return err
			}
			r := app.Renderer
			if r.IsJSON() {
				return r.JSON(out)
			}
			if len(out.Claims) == 0 {
				r.Statusln(r.Dim("No claims."))
				return nil
			}
			t := r.Table("ID", "IMP", "SECTION", "TEXT")
			for _, c := range out.Claims {
				t.Row(c.ID, intToString(c.Importance), truncate(c.SectionTitle, 40), truncate(c.Text, 80))
			}
			t.Render()
			return nil
		},
	}
	listCmd.Flags().Int("min-importance", 0, "Minimum importance score (1–5)")
	c.AddCommand(listCmd)

	c.AddCommand(&cobra.Command{
		Use:   "get <document-id>",
		Short: "Same as `list`; alias for symmetry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := makeAppCtx(true)
			if err != nil {
				return err
			}
			out, err := app.Client.DocumentClaims(cmd.Context(), args[0], 0)
			if err != nil {
				return err
			}
			r := app.Renderer
			if r.IsJSON() {
				return r.JSON(out)
			}
			for _, c := range out.Claims {
				r.Println(r.Bold(c.SectionTitle), r.Dim("(imp "+intToString(c.Importance)+")"))
				r.Println("  " + c.Text)
			}
			return nil
		},
	})

	return c
}

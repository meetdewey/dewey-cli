package cmd

import (
	"github.com/lambdabaa/dewey/apps/cli/internal/api"
	"github.com/spf13/cobra"
)

func newProviderKeysCmd(makeAppCtx func(bool) (*AppContext, error)) *cobra.Command {
	c := &cobra.Command{
		Use:     "provider-keys",
		Aliases: []string{"provider-key"},
		Short:   "Manage BYOK provider API keys",
	}

	c.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List provider keys",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := makeAppCtx(true)
			if err != nil {
				return err
			}
			keys, err := app.Client.ListProviderKeys(cmd.Context())
			if err != nil {
				return err
			}
			r := app.Renderer
			if r.IsJSON() {
				return r.JSON(keys)
			}
			if len(keys) == 0 {
				r.Statusln(r.Dim("No provider keys."))
				return nil
			}
			t := r.Table("ID", "PROVIDER", "NAME", "PREVIEW", "CREATED")
			for _, k := range keys {
				t.Row(k.ID, k.Provider, k.Name, k.KeyPreview, k.CreatedAt)
			}
			t.Render()
			return nil
		},
	})

	setCmd := &cobra.Command{
		Use:   "set <provider> <key>",
		Short: "Add a provider API key",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := makeAppCtx(true)
			if err != nil {
				return err
			}
			name, _ := cmd.Flags().GetString("name")
			if name == "" {
				name = args[0] + " key"
			}
			pk, err := app.Client.CreateProviderKey(cmd.Context(), api.CreateProviderKeyInput{
				Provider: args[0],
				Key:      args[1],
				Name:     name,
			})
			if err != nil {
				return err
			}
			r := app.Renderer
			if r.IsJSON() {
				return r.JSON(pk)
			}
			r.Statusln(r.Tick(), "Saved", pk.Provider, "key", pk.ID)
			return nil
		},
	}
	setCmd.Flags().String("name", "", "Friendly name (defaults to '<provider> key')")
	c.AddCommand(setCmd)

	c.AddCommand(&cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a provider key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := makeAppCtx(true)
			if err != nil {
				return err
			}
			if err := app.Client.DeleteProviderKey(cmd.Context(), args[0]); err != nil {
				return err
			}
			app.Renderer.Statusln(app.Renderer.Tick(), "Deleted", args[0])
			return nil
		},
	})

	return c
}

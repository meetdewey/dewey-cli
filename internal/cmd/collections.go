package cmd

import (
	"github.com/lambdabaa/dewey/apps/cli/internal/api"
	"github.com/spf13/cobra"
)

func newCollectionsCmd(makeAppCtx func(bool) (*AppContext, error)) *cobra.Command {
	c := &cobra.Command{
		Use:     "collections",
		Aliases: []string{"cols"},
		Short:   "Manage collections",
	}

	c.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List collections in the project",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := makeAppCtx(true)
			if err != nil {
				return err
			}
			cols, err := app.Client.ListCollections(cmd.Context())
			if err != nil {
				return err
			}
			r := app.Renderer
			if r.IsJSON() {
				return r.JSON(cols)
			}
			if len(cols) == 0 {
				r.Status("No collections yet. Create one with `dewey collections create <name>`.\n")
				return nil
			}
			t := r.Table("ID", "NAME", "DOCS", "VISIBILITY", "CREATED")
			for _, col := range cols {
				docs := "—"
				if col.DescriptionDocCount != nil {
					docs = intToString(*col.DescriptionDocCount)
				}
				t.Row(col.ID, col.Name, docs, col.Visibility, col.CreatedAt)
			}
			t.Render()
			return nil
		},
	})

	c.AddCommand(&cobra.Command{
		Use:   "get <name|id>",
		Short: "Get a single collection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := makeAppCtx(true)
			if err != nil {
				return err
			}
			col, err := app.resolveCollection(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			r := app.Renderer
			if r.IsJSON() {
				return r.JSON(col)
			}
			r.KeyValue("ID", col.ID)
			r.KeyValue("Name", col.Name)
			r.KeyValue("Visibility", col.Visibility)
			r.KeyValue("Embedding model", col.EmbeddingModel)
			r.KeyValue("Chunk size", intToString(col.ChunkSize))
			r.KeyValue("Chunk overlap", intToString(col.ChunkOverlap))
			if col.LLMModel != nil {
				r.KeyValue("LLM model", *col.LLMModel)
			}
			r.KeyValue("Created", col.CreatedAt)
			return nil
		},
	})

	createCmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new collection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := makeAppCtx(true)
			if err != nil {
				return err
			}
			visibility, _ := cmd.Flags().GetString("visibility")
			embedModel, _ := cmd.Flags().GetString("embedding-model")
			chunkSize, _ := cmd.Flags().GetInt("chunk-size")
			chunkOverlap, _ := cmd.Flags().GetInt("chunk-overlap")
			in := api.CreateCollectionInput{
				Name:           args[0],
				Visibility:     visibility,
				EmbeddingModel: embedModel,
				ChunkSize:      chunkSize,
				ChunkOverlap:   chunkOverlap,
			}
			col, err := app.Client.CreateCollection(cmd.Context(), in)
			if err != nil {
				return err
			}
			app.rememberCollection(col.ID)
			r := app.Renderer
			if r.IsJSON() {
				return r.JSON(col)
			}
			r.Statusln(r.Tick(), "Created", col.Name, "("+col.ID+")")
			return nil
		},
	}
	createCmd.Flags().String("visibility", "", "private | public")
	createCmd.Flags().String("embedding-model", "", "Embedding model override")
	createCmd.Flags().Int("chunk-size", 0, "Chunk size in tokens")
	createCmd.Flags().Int("chunk-overlap", 0, "Chunk overlap in tokens")
	c.AddCommand(createCmd)

	updateCmd := &cobra.Command{
		Use:   "update <name|id>",
		Short: "Update a collection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := makeAppCtx(true)
			if err != nil {
				return err
			}
			col, err := app.resolveCollection(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			var input api.UpdateCollectionInput
			if name, _ := cmd.Flags().GetString("name"); cmd.Flags().Changed("name") {
				input.Name = &name
			}
			if vis, _ := cmd.Flags().GetString("visibility"); cmd.Flags().Changed("visibility") {
				input.Visibility = &vis
			}
			if v, _ := cmd.Flags().GetString("instructions"); cmd.Flags().Changed("instructions") {
				input.Instructions = &v
			}
			if v, _ := cmd.Flags().GetString("description"); cmd.Flags().Changed("description") {
				input.Description = &v
			}
			if v, _ := cmd.Flags().GetBool("enable-summarization"); cmd.Flags().Changed("enable-summarization") {
				input.EnableSummarization = &v
			}
			if v, _ := cmd.Flags().GetBool("enable-captioning"); cmd.Flags().Changed("enable-captioning") {
				input.EnableCaptioning = &v
			}
			if v, _ := cmd.Flags().GetBool("enable-deduplication"); cmd.Flags().Changed("enable-deduplication") {
				input.EnableDeduplication = &v
			}
			if v, _ := cmd.Flags().GetBool("enable-reranking"); cmd.Flags().Changed("enable-reranking") {
				input.EnableReranking = &v
			}
			if v, _ := cmd.Flags().GetString("llm-model"); cmd.Flags().Changed("llm-model") {
				input.LLMModel = &v
			}

			updated, err := app.Client.UpdateCollection(cmd.Context(), col.ID, input)
			if err != nil {
				return err
			}
			r := app.Renderer
			if r.IsJSON() {
				return r.JSON(updated)
			}
			r.Statusln(r.Tick(), "Updated", updated.Name)
			return nil
		},
	}
	updateCmd.Flags().String("name", "", "New name")
	updateCmd.Flags().String("visibility", "", "private | public")
	updateCmd.Flags().String("description", "", "Description text")
	updateCmd.Flags().String("instructions", "", "Research instructions")
	updateCmd.Flags().String("llm-model", "", "Model used for summarization/captioning")
	updateCmd.Flags().Bool("enable-summarization", false, "Toggle section summarization")
	updateCmd.Flags().Bool("enable-captioning", false, "Toggle image/table captioning")
	updateCmd.Flags().Bool("enable-deduplication", false, "Toggle deduplication")
	updateCmd.Flags().Bool("enable-reranking", false, "Toggle reranking")
	c.AddCommand(updateCmd)

	c.AddCommand(&cobra.Command{
		Use:   "delete <name|id>",
		Short: "Delete a collection (soft delete)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := makeAppCtx(true)
			if err != nil {
				return err
			}
			col, err := app.resolveCollection(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if err := app.Client.DeleteCollection(cmd.Context(), col.ID); err != nil {
				return err
			}
			r := app.Renderer
			if r.IsJSON() {
				return r.JSON(map[string]string{"id": col.ID, "status": "deleted"})
			}
			r.Statusln(r.Tick(), "Deleted", col.Name)
			return nil
		},
	})

	c.AddCommand(&cobra.Command{
		Use:   "stats <name|id>",
		Short: "Show collection statistics",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := makeAppCtx(true)
			if err != nil {
				return err
			}
			col, err := app.resolveCollection(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			stats, err := app.Client.CollectionStats(cmd.Context(), col.ID)
			if err != nil {
				return err
			}
			r := app.Renderer
			if r.IsJSON() {
				return r.JSON(stats)
			}
			r.KeyValue("Documents", intToString(stats.DocCount))
			r.KeyValue("Total bytes", intToString(int(stats.TotalFileSizeBytes)))
			r.KeyValue("Sections", intToString(stats.TotalSections))
			r.KeyValue("Chunks", intToString(stats.TotalChunks))
			r.KeyValue("Claims", intToString(stats.TotalClaimsCount))
			return nil
		},
	})

	return c
}

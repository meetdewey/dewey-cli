package cmd

import (
	"fmt"
	"strings"

	"github.com/lambdabaa/dewey/apps/cli/internal/output"
	"github.com/spf13/cobra"
)

func newDocsCmd(makeAppCtx func(bool) (*AppContext, error)) *cobra.Command {
	c := &cobra.Command{
		Use:     "docs",
		Aliases: []string{"documents"},
		Short:   "Manage documents",
	}

	c.AddCommand(&cobra.Command{
		Use:   "list [collection]",
		Short: "List documents in a collection",
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
			docs, err := app.Client.ListDocuments(cmd.Context(), col.ID)
			if err != nil {
				return err
			}
			r := app.Renderer
			if r.IsJSON() {
				return r.JSON(docs)
			}
			if len(docs) == 0 {
				r.Statusln(r.Dim("No documents."))
				return nil
			}
			t := r.Table("ID", "FILENAME", "STATUS", "SIZE", "SECTIONS", "CHUNKS")
			for _, d := range docs {
				size := "—"
				if d.FileSizeBytes != nil {
					size = output.HumanBytes(*d.FileSizeBytes)
				}
				sections := "—"
				if d.SectionCount != nil {
					sections = intToString(*d.SectionCount)
				}
				chunks := "—"
				if d.ChunkCount != nil {
					chunks = intToString(*d.ChunkCount)
				}
				t.Row(d.ID, truncate(d.Filename, 40), d.Status, size, sections, chunks)
			}
			t.Render()
			return nil
		},
	})

	c.AddCommand(&cobra.Command{
		Use:   "get <document-id>",
		Short: "Get a single document",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := makeAppCtx(true)
			if err != nil {
				return err
			}
			d, err := app.Client.GetDocument(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			r := app.Renderer
			if r.IsJSON() {
				return r.JSON(d)
			}
			r.KeyValue("ID", d.ID)
			r.KeyValue("Filename", d.Filename)
			r.KeyValue("Status", d.Status)
			if d.FileSizeBytes != nil {
				r.KeyValue("Size", output.HumanBytes(*d.FileSizeBytes))
			}
			if d.SectionCount != nil {
				r.KeyValue("Sections", intToString(*d.SectionCount))
			}
			if d.ChunkCount != nil {
				r.KeyValue("Chunks", intToString(*d.ChunkCount))
			}
			r.KeyValue("Created", d.CreatedAt)
			if d.ErrorMessage != nil && *d.ErrorMessage != "" {
				r.KeyValue("Error", *d.ErrorMessage)
			}
			return nil
		},
	})

	c.AddCommand(&cobra.Command{
		Use:   "markdown <document-id>",
		Short: "Print the rendered Markdown for a document",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := makeAppCtx(true)
			if err != nil {
				return err
			}
			md, err := app.Client.GetDocumentMarkdown(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			r := app.Renderer
			if r.IsJSON() {
				return r.JSON(map[string]string{"documentId": args[0], "markdown": md})
			}
			r.Print(md)
			if !strings.HasSuffix(md, "\n") {
				r.Println()
			}
			return nil
		},
	})

	c.AddCommand(&cobra.Command{
		Use:   "sections <document-id>",
		Short: "List sections for a document",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := makeAppCtx(true)
			if err != nil {
				return err
			}
			sections, err := app.Client.ListSections(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			r := app.Renderer
			if r.IsJSON() {
				return r.JSON(sections)
			}
			t := r.Table("ID", "LVL", "POS", "TITLE", "CHUNKS")
			for _, s := range sections {
				t.Row(s.ID, intToString(s.Level), intToString(s.Position), truncate(s.Title, 60), intToString(s.ChunkCount))
			}
			t.Render()
			return nil
		},
	})

	c.AddCommand(&cobra.Command{
		Use:   "chunks <section-id>",
		Short: "List chunks for a section",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := makeAppCtx(true)
			if err != nil {
				return err
			}
			chunks, err := app.Client.ListChunks(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			r := app.Renderer
			if r.IsJSON() {
				return r.JSON(chunks)
			}
			t := r.Table("ID", "POS", "TOKENS", "PREVIEW")
			for _, ch := range chunks {
				t.Row(ch.ID, intToString(ch.Position), intToString(ch.TokenCount), truncate(ch.Content, 80))
			}
			t.Render()
			return nil
		},
	})

	c.AddCommand(&cobra.Command{
		Use:   "images <document-id>",
		Short: "List images extracted from a document",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := makeAppCtx(true)
			if err != nil {
				return err
			}
			imgs, err := app.Client.ListImages(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			r := app.Renderer
			if r.IsJSON() {
				return r.JSON(imgs)
			}
			if len(imgs) == 0 {
				r.Statusln(r.Dim("No images."))
				return nil
			}
			for _, img := range imgs {
				r.Println(fmt.Sprintf("%v", img))
			}
			return nil
		},
	})

	c.AddCommand(&cobra.Command{
		Use:   "delete <document-id>",
		Short: "Delete a document",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := makeAppCtx(true)
			if err != nil {
				return err
			}
			if err := app.Client.DeleteDocument(cmd.Context(), args[0]); err != nil {
				return err
			}
			r := app.Renderer
			if r.IsJSON() {
				return r.JSON(map[string]string{"id": args[0], "status": "deleted"})
			}
			r.Statusln(r.Tick(), "Deleted", args[0])
			return nil
		},
	})

	c.AddCommand(&cobra.Command{
		Use:   "wait <document-id>",
		Short: "Block until a document reaches a terminal state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := makeAppCtx(true)
			if err != nil {
				return err
			}
			d, err := app.Client.WaitForDocument(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			r := app.Renderer
			if r.IsJSON() {
				return r.JSON(d)
			}
			r.Statusln(r.Tick(), d.ID, "→", d.Status)
			return nil
		},
	})

	return c
}

package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/lambdabaa/dewey/apps/cli/internal/api"
	"github.com/lambdabaa/dewey/apps/cli/internal/output"
	"github.com/spf13/cobra"
)

func newUploadCmd(makeAppCtx func(bool) (*AppContext, error)) *cobra.Command {
	c := &cobra.Command{
		Use:   "upload <files...>",
		Short: "Upload one or more files to a collection",
		Long: `Upload one or more files to a collection.

Files larger than 4 MiB are uploaded directly to S3 via a presigned URL; smaller
files use a multipart/form-data POST. Pass "-" as a path to read from stdin.

Use --watch to stream live status events for the uploaded documents until they
reach a terminal state. Use --wait to block silently until all docs are ready.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := makeAppCtx(true)
			if err != nil {
				return err
			}
			watch, _ := cmd.Flags().GetBool("watch")
			wait, _ := cmd.Flags().GetBool("wait")
			concurrency, _ := cmd.Flags().GetInt("concurrency")
			if concurrency < 1 {
				concurrency = 4
			}
			tagsRaw, _ := cmd.Flags().GetStringSlice("tag")
			metaRaw, _ := cmd.Flags().GetStringArray("metadata")
			metadata, err := parseKVPairs(metaRaw)
			if err != nil {
				return err
			}

			col, err := app.resolveCollection(cmd.Context(), "")
			if err != nil {
				return err
			}

			paths, err := expandPaths(args)
			if err != nil {
				return err
			}
			if len(paths) == 0 {
				return fmt.Errorf("no files matched")
			}

			r := app.Renderer
			r.Statusln(r.Bold(fmt.Sprintf("Uploading %d files to %s (%s)", len(paths), col.Name, col.ID)))

			docs, err := uploadFiles(cmd.Context(), app, col.ID, paths, concurrency, tagsRaw, metadata)
			if err != nil {
				return err
			}

			if r.IsJSON() && !watch && !wait {
				for _, d := range docs {
					if err := r.JSONLine(d); err != nil {
						return err
					}
				}
				return nil
			}

			if watch {
				return watchDocsUntilTerminal(cmd.Context(), app, col.ID, docs)
			}
			if wait {
				return waitDocsTerminal(cmd.Context(), app, docs)
			}
			r.Statusln(r.Dim(fmt.Sprintf("%d uploaded; processing continues in the background. Re-run with --watch to follow.", len(docs))))
			return nil
		},
	}
	c.Flags().Bool("watch", false, "Stream live status events until all docs reach a terminal state")
	c.Flags().Bool("wait", false, "Block silently until all docs reach a terminal state")
	c.Flags().Int("concurrency", 4, "Parallel uploads")
	c.Flags().StringSlice("tag", nil, "Tags (repeatable)")
	c.Flags().StringArray("metadata", nil, "Metadata key=value pair (repeatable)")
	return c
}

func expandPaths(args []string) ([]string, error) {
	var out []string
	seen := make(map[string]struct{})
	for _, a := range args {
		if a == "-" {
			out = append(out, "-")
			continue
		}
		// Try literal path first.
		if _, err := os.Stat(a); err == nil {
			abs, _ := filepath.Abs(a)
			if _, dup := seen[abs]; !dup {
				seen[abs] = struct{}{}
				out = append(out, a)
			}
			continue
		}
		// Otherwise treat as glob.
		matches, err := filepath.Glob(a)
		if err != nil {
			return nil, fmt.Errorf("glob %s: %w", a, err)
		}
		for _, m := range matches {
			abs, _ := filepath.Abs(m)
			if _, dup := seen[abs]; !dup {
				seen[abs] = struct{}{}
				out = append(out, m)
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

func uploadFiles(ctx context.Context, app *AppContext, collectionID string, paths []string, concurrency int, tags []string, metadata map[string]any) ([]*api.Document, error) {
	r := app.Renderer
	results := make([]*api.Document, len(paths))
	errs := make([]error, len(paths))

	type job struct {
		index int
		path  string
	}
	jobs := make(chan job)
	var wg sync.WaitGroup

	// Used to print progress lines from workers without interleaving badly.
	var mu sync.Mutex

	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				if err := ctx.Err(); err != nil {
					errs[j.index] = err
					continue
				}
				doc, err := uploadOne(ctx, app, collectionID, j.path, tags, metadata, &mu)
				if err != nil {
					errs[j.index] = err
					mu.Lock()
					r.Statusln(r.Cross(), filepath.Base(j.path), "—", err.Error())
					mu.Unlock()
					continue
				}
				results[j.index] = doc
				mu.Lock()
				r.Statusln(r.Tick(), filepath.Base(j.path), "uploaded", r.Dim(doc.ID))
				mu.Unlock()
			}
		}()
	}

	for i, p := range paths {
		jobs <- job{index: i, path: p}
	}
	close(jobs)
	wg.Wait()

	var failures []error
	var docs []*api.Document
	for i, d := range results {
		if errs[i] != nil {
			failures = append(failures, errs[i])
			continue
		}
		if d != nil {
			docs = append(docs, d)
		}
	}
	if len(failures) > 0 && len(docs) == 0 {
		return nil, failures[0]
	}
	return docs, nil
}

func uploadOne(ctx context.Context, app *AppContext, collectionID, path string, tags []string, metadata map[string]any, mu *sync.Mutex) (*api.Document, error) {
	if path == "-" {
		// Read stdin into a temp file then upload.
		tmp, err := os.CreateTemp("", "dewey-stdin-*")
		if err != nil {
			return nil, err
		}
		defer os.Remove(tmp.Name())
		if _, err := io.Copy(tmp, os.Stdin); err != nil {
			tmp.Close()
			return nil, err
		}
		tmp.Close()
		path = tmp.Name()
	}

	opts := api.UploadOptions{Tags: tags, Metadata: metadata}
	return app.Client.UploadFile(ctx, collectionID, path, opts)
}

// watchDocsUntilTerminal opens an SSE stream and prints status transitions for
// the supplied docs, exiting once each one is in a terminal state.
func watchDocsUntilTerminal(ctx context.Context, app *AppContext, collectionID string, docs []*api.Document) error {
	r := app.Renderer
	r.Statusln(r.Dim("Watching processing events (Ctrl-C to detach):"))
	pending := make(map[string]string, len(docs))
	for _, d := range docs {
		pending[d.ID] = d.Filename
	}

	start := time.Now()
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	emitJSON := r.IsJSON()

	err := app.Client.StreamDocumentEvents(streamCtx, collectionID, func(ev api.DocumentEvent) error {
		filename, tracked := pending[ev.DocumentID]
		if !tracked {
			return nil
		}
		if filename == "" && ev.Filename != "" {
			filename = ev.Filename
			pending[ev.DocumentID] = filename
		}
		if emitJSON {
			if isTerminalStatus(ev.Status) {
				_ = r.JSONLine(ev)
			}
		} else {
			r.Statusln(formatDocEvent(r, ev, filename))
		}
		if isTerminalStatus(ev.Status) {
			delete(pending, ev.DocumentID)
		}
		if len(pending) == 0 {
			return io.EOF
		}
		return nil
	})

	// EOF is success.
	if err != nil && err != io.EOF && err != context.Canceled {
		return err
	}
	if !emitJSON {
		r.Statusln(r.Tick(), fmt.Sprintf("%d/%d ready in %s.", len(docs)-len(pending), len(docs), time.Since(start).Round(time.Second)))
	}
	return nil
}

func waitDocsTerminal(ctx context.Context, app *AppContext, docs []*api.Document) error {
	r := app.Renderer
	for _, d := range docs {
		final, err := app.Client.WaitForDocument(ctx, d.ID)
		if err != nil {
			return err
		}
		if r.IsJSON() {
			_ = r.JSONLine(final)
		}
	}
	return nil
}

func isTerminalStatus(s string) bool {
	return s == "ready" || s == "error"
}

func formatDocEvent(r *output.Renderer, ev api.DocumentEvent, filename string) string {
	icon := "  "
	switch ev.Status {
	case "ready":
		icon = r.Tick()
	case "error":
		icon = r.Cross()
	default:
		icon = r.Dim("…")
	}
	if filename == "" {
		filename = ev.DocumentID
	}
	return fmt.Sprintf("%s %s  %s  %s", icon, ev.DocumentID, filename, ev.Status)
}

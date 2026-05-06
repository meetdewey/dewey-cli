package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/lambdabaa/dewey/apps/cli/internal/api"
	"github.com/lambdabaa/dewey/apps/cli/internal/config"
	"github.com/lambdabaa/dewey/apps/cli/internal/output"
	"github.com/spf13/cobra"
)

// harness rebuilds the dewey command tree with stdout/stderr captured to
// buffers and the API client pointed at an httptest server. It mirrors the
// wiring in Execute() but never calls os.Exit and never touches os.Stdout.
type harness struct {
	t        *testing.T
	server   *httptest.Server
	handlers map[string]http.HandlerFunc
	mu       sync.Mutex
	requests []recordedReq
	stdout   *bytes.Buffer
	stderr   *bytes.Buffer
	state    *config.State
	cfg      *config.Config
}

type recordedReq struct {
	Method   string
	Path     string
	RawQuery string
	Body     []byte
	Headers  http.Header
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	t.Setenv("DEWEY_API_KEY", "test-key")
	t.Setenv("DEWEY_TELEMETRY", "0")
	// Isolate the home directory so config/state don't leak between tests.
	t.Setenv("HOME", t.TempDir())

	h := &harness{
		t:        t,
		handlers: make(map[string]http.HandlerFunc),
		stdout:   &bytes.Buffer{},
		stderr:   &bytes.Buffer{},
	}
	h.server = httptest.NewServer(http.HandlerFunc(h.dispatch))
	t.Cleanup(h.server.Close)
	return h
}

func (h *harness) Handle(methodAndPath string, fn http.HandlerFunc) {
	h.handlers[methodAndPath] = fn
}

func (h *harness) HandleJSON(methodAndPath string, status int, payload any) {
	h.Handle(methodAndPath, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if payload != nil {
			_ = json.NewEncoder(w).Encode(payload)
		}
	})
}

func (h *harness) dispatch(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	h.mu.Lock()
	h.requests = append(h.requests, recordedReq{
		Method:   r.Method,
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
		Body:     body,
		Headers:  r.Header.Clone(),
	})
	h.mu.Unlock()
	r.Body = io.NopCloser(strings.NewReader(string(body)))

	hasBearer := r.Header.Get("Authorization") == "Bearer test-key"
	hasKey := r.URL.Query().Get("key") == "test-key"
	if !hasBearer && !hasKey {
		http.Error(w, "missing/invalid bearer", http.StatusUnauthorized)
		return
	}

	key := r.Method + " " + r.URL.Path
	if fn, ok := h.handlers[key]; ok {
		fn(w, r)
		return
	}
	http.Error(w, "no handler for "+key, http.StatusNotFound)
}

func (h *harness) lastRequest() recordedReq {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.requests) == 0 {
		h.t.Fatal("no requests recorded")
	}
	return h.requests[len(h.requests)-1]
}

// run wires up the full command tree with stdout/stderr captured to harness
// buffers and the API client pointed at the httptest server. Returns the exit
// code that Execute would have produced.
func (h *harness) run(args ...string) int {
	rf := &rootFlags{}

	root := &cobra.Command{
		Use:           "dewey",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&rf.apiKey, "api-key", "", "")
	root.PersistentFlags().StringVar(&rf.baseURL, "base-url", "", "")
	root.PersistentFlags().BoolVar(&rf.jsonMode, "json", false, "")
	root.PersistentFlags().BoolVar(&rf.noColor, "no-color", false, "")
	root.PersistentFlags().StringVar(&rf.color, "color", "", "")
	root.PersistentFlags().StringVarP(&rf.collection, "collection", "c", "", "")

	makeAppCtx := func(requireAuth bool) (*AppContext, error) {
		cfg, err := config.Load()
		if err != nil {
			return nil, err
		}
		state, err := config.LoadState()
		if err != nil {
			return nil, err
		}
		h.cfg = cfg
		h.state = state

		jsonMode := rf.jsonMode
		if !jsonMode && cfg.Output == "json" {
			jsonMode = true
		}
		mode := output.ModeHuman
		if jsonMode {
			mode = output.ModeJSON
		}
		// Force color off so assertions are simpler.
		r := &output.Renderer{
			Mode:  mode,
			Out:   h.stdout,
			Err:   h.stderr,
			Color: false,
			TTY:   false,
		}

		apiKey := config.ResolveAPIKey(rf.apiKey)
		if requireAuth && apiKey == "" {
			return nil, errMissingAPIKey
		}

		baseURL := rf.baseURL
		if baseURL == "" {
			baseURL = h.server.URL
		}
		client := api.New(apiKey, baseURL, "dewey-test")

		ctx := &AppContext{Client: client, Config: cfg, State: state, Renderer: r}
		if rf.collection != "" {
			ctx.Collection = rf.collection
		} else if state.LastCollection != "" {
			ctx.Collection = state.LastCollection
		}
		return ctx, nil
	}

	root.AddCommand(newUploadCmd(makeAppCtx))
	root.AddCommand(newQueryCmd(makeAppCtx))
	root.AddCommand(newScanCmd(makeAppCtx))
	root.AddCommand(newResearchCmd(makeAppCtx))
	root.AddCommand(newWatchCmd(makeAppCtx))
	root.AddCommand(newDoctorCmd(makeAppCtx))
	root.AddCommand(newVersionCmd())
	root.AddCommand(newCollectionsCmd(makeAppCtx))
	root.AddCommand(newDocsCmd(makeAppCtx))
	root.AddCommand(newConfigCmd(makeAppCtx))
	root.AddCommand(newProviderKeysCmd(makeAppCtx))
	root.AddCommand(newDuplicatesCmd(makeAppCtx))
	root.AddCommand(newContradictionsCmd(makeAppCtx))
	root.AddCommand(newClaimsCmd(makeAppCtx))

	root.SetArgs(args)
	root.SetOut(h.stdout)
	root.SetErr(h.stderr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := root.ExecuteContext(ctx); err != nil {
		// Mimic handleError but write to harness buffers.
		switch {
		case strings.Contains(err.Error(), "missing API key"):
			h.stderr.WriteString("error: missing API key\n")
			return ExitNoPermission
		}
		var apiErr *api.Error
		if asAPIError(err, &apiErr) {
			h.stderr.WriteString("error: " + apiErr.Message + "\n")
			switch apiErr.Status {
			case 401, 403:
				return ExitNoPermission
			case 404:
				return ExitInputError
			case 429:
				return ExitTempFail
			case 503, 502, 504:
				return ExitUnavailable
			}
			return ExitGenericError
		}
		h.stderr.WriteString("error: " + err.Error() + "\n")
		return ExitGenericError
	}
	return ExitOK
}

func asAPIError(err error, dst **api.Error) bool {
	for err != nil {
		if e, ok := err.(*api.Error); ok {
			*dst = e
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

package output

import (
	"bytes"
	"strings"
	"testing"
)

// newTestRenderer constructs a Renderer with explicit buffers — so tests can
// assert on stdout vs stderr separation. Color is forced on/off as needed.
func newTestRenderer(jsonMode, color, tty bool) (*Renderer, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	mode := ModeHuman
	if jsonMode {
		mode = ModeJSON
	}
	r := &Renderer{Mode: mode, Out: out, Err: errBuf, Color: color, TTY: tty}
	return r, out, errBuf
}

func TestRenderer_JSON_PrettyPrints(t *testing.T) {
	r, out, _ := newTestRenderer(true, false, false)
	if err := r.JSON(map[string]string{"a": "1", "b": "2"}); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "\"a\": \"1\"") || !strings.Contains(got, "\"b\": \"2\"") {
		t.Errorf("expected pretty JSON, got: %s", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Error("expected trailing newline")
	}
}

func TestRenderer_JSONLine_NDJSON(t *testing.T) {
	r, out, _ := newTestRenderer(true, false, false)
	for _, v := range []map[string]int{{"n": 1}, {"n": 2}} {
		if err := r.JSONLine(v); err != nil {
			t.Fatal(err)
		}
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 NDJSON lines, got %d: %q", len(lines), out.String())
	}
	if !strings.HasPrefix(lines[0], `{"n":1}`) || !strings.HasPrefix(lines[1], `{"n":2}`) {
		t.Errorf("unexpected NDJSON: %v", lines)
	}
}

func TestRenderer_StdoutVsStderrSeparation(t *testing.T) {
	r, out, errBuf := newTestRenderer(false, false, false)
	r.Println("data")
	r.Statusln("status")
	if !strings.Contains(out.String(), "data") {
		t.Error("data should be on stdout")
	}
	if strings.Contains(out.String(), "status") {
		t.Error("status leaked to stdout")
	}
	if !strings.Contains(errBuf.String(), "status") {
		t.Error("status should be on stderr")
	}
}

func TestRenderer_Table_AlignsColumns(t *testing.T) {
	r, out, _ := newTestRenderer(false, false, false)
	t1 := r.Table("ID", "NAME")
	t1.Row("1", "alice")
	t1.Row("22", "bob")
	t1.Render()

	got := out.String()
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), got)
	}
	// Columns should be padded so subsequent lines line up.
	if !strings.HasPrefix(lines[0], "ID  NAME") {
		t.Errorf("header not padded: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "1   alice") {
		t.Errorf("row 1 not padded: %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "22  bob") {
		t.Errorf("row 2 not padded: %q", lines[2])
	}
}

func TestRenderer_Table_HandlesShortRows(t *testing.T) {
	r, out, _ := newTestRenderer(false, false, false)
	t1 := r.Table("A", "B", "C")
	t1.Row("1") // missing B and C
	t1.Render()
	got := out.String()
	if !strings.Contains(got, "A  B  C") {
		t.Errorf("header missing: %q", got)
	}
	// Should not panic — short row pads to column count.
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d: %q", len(lines), got)
	}
}

func TestRenderer_Color_DisabledByDefault(t *testing.T) {
	r, out, _ := newTestRenderer(false, false, false)
	r.Print(r.Bold("hi"))
	if strings.Contains(out.String(), "\x1b[") {
		t.Errorf("color leaked when Color=false: %q", out.String())
	}
}

func TestRenderer_Color_EmitsANSIWhenEnabled(t *testing.T) {
	r, out, _ := newTestRenderer(false, true, true)
	r.Print(r.Bold("hi"))
	if !strings.Contains(out.String(), "\x1b[1m") {
		t.Errorf("expected ANSI bold: %q", out.String())
	}
}

func TestRenderer_TickFallsBackToASCII(t *testing.T) {
	r, _, _ := newTestRenderer(false, false, false /* not TTY */)
	if r.Tick() != "[ok]" {
		t.Errorf("non-TTY Tick = %q", r.Tick())
	}
	if r.Cross() != "[fail]" {
		t.Errorf("non-TTY Cross = %q", r.Cross())
	}
	if r.Warn() != "[warn]" {
		t.Errorf("non-TTY Warn = %q", r.Warn())
	}
}

func TestRenderer_TickUsesUnicodeOnTTY(t *testing.T) {
	r, _, _ := newTestRenderer(false, true, true)
	if !strings.Contains(r.Tick(), "✓") {
		t.Errorf("TTY Tick should be unicode: %q", r.Tick())
	}
}

func TestRenderer_KeyValue_NoColor(t *testing.T) {
	r, out, _ := newTestRenderer(false, false, false)
	r.KeyValue("ID", "abc")
	if !strings.Contains(out.String(), "ID: abc") {
		t.Errorf("got: %q", out.String())
	}
}

func TestRenderer_KeyValue_WithColor(t *testing.T) {
	r, out, _ := newTestRenderer(false, true, true)
	r.KeyValue("ID", "abc")
	if !strings.Contains(out.String(), "\x1b[1mID:\x1b[0m abc") {
		t.Errorf("color not applied: %q", out.String())
	}
}

func TestNew_RespectsNoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	r := New(false, true, "auto")
	if r.Color {
		t.Error("expected color disabled when NO_COLOR is set")
	}
}

func TestNew_ColorAlways_OverridesTTY(t *testing.T) {
	r := New(false, true, "always")
	if !r.Color {
		t.Error("expected color enabled with --color always")
	}
}

func TestNew_ColorNever_DisablesEvenIfTTY(t *testing.T) {
	r := New(false, true, "never")
	if r.Color {
		t.Error("expected color disabled with --color never")
	}
}

func TestNew_JSONForcesNoColor(t *testing.T) {
	r := New(true, true, "always")
	if r.Color {
		t.Error("JSON mode should disable color")
	}
	if !r.IsJSON() {
		t.Error("IsJSON should be true")
	}
}

func TestStripANSI(t *testing.T) {
	got := stripANSI("\x1b[1mhi\x1b[0m there")
	if got != "hi there" {
		t.Errorf("got %q", got)
	}
}

func TestDisplayWidth_IgnoresANSI(t *testing.T) {
	// "hi" with bold styling should render as width 2, same as plain.
	if displayWidth("\x1b[1mhi\x1b[0m") != 2 {
		t.Errorf("expected width 2")
	}
	if displayWidth("hello") != 5 {
		t.Errorf("expected width 5")
	}
}

func TestPadRight(t *testing.T) {
	cases := map[string]struct {
		s    string
		n    int
		want string
	}{
		"shorter": {"hi", 5, "hi   "},
		"exact":   {"hello", 5, "hello"},
		"longer":  {"hello", 3, "hello"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := padRight(tc.s, tc.n); got != tc.want {
				t.Errorf("padRight(%q, %d) = %q, want %q", tc.s, tc.n, got, tc.want)
			}
		})
	}
}

func TestHumanBytes(t *testing.T) {
	cases := map[int64]string{
		0:                   "0 B",
		512:                 "512 B",
		1024:                "1.0 KB",
		1536:                "1.5 KB",
		2 * 1024 * 1024:     "2.0 MB",
		3 * 1024 * 1024 * 1024: "3.0 GB",
	}
	for n, want := range cases {
		if got := HumanBytes(n); got != want {
			t.Errorf("HumanBytes(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestSection_NoColor(t *testing.T) {
	r, out, _ := newTestRenderer(false, false, false)
	r.Section("Hello")
	if out.String() != "Hello\n" {
		t.Errorf("got %q", out.String())
	}
}

func TestSection_WithColor(t *testing.T) {
	r, out, _ := newTestRenderer(false, true, true)
	r.Section("Hello")
	if !strings.Contains(out.String(), "\x1b[1mHello\x1b[0m") {
		t.Errorf("expected bold: %q", out.String())
	}
}

// Package output provides human + JSON renderers for CLI commands. Diagnostic
// noise (status, progress) goes to stderr; data goes to stdout.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// Mode is the output format selected by the user.
type Mode int

const (
	ModeHuman Mode = iota
	ModeJSON
)

// Renderer holds the streams + formatting choices used by every command.
type Renderer struct {
	Mode  Mode
	Out   io.Writer
	Err   io.Writer
	Color bool
	TTY   bool
}

// New constructs a renderer using stdout/stderr and detecting TTY.
func New(jsonMode, color bool, colorMode string) *Renderer {
	out := os.Stdout
	errW := os.Stderr
	tty := term.IsTerminal(int(out.Fd()))

	useColor := color
	switch colorMode {
	case "always":
		useColor = true
	case "never":
		useColor = false
	case "auto", "":
		useColor = tty && os.Getenv("NO_COLOR") == "" && color
	}
	if jsonMode {
		useColor = false
	}

	mode := ModeHuman
	if jsonMode {
		mode = ModeJSON
	}
	return &Renderer{Mode: mode, Out: out, Err: errW, Color: useColor, TTY: tty}
}

// IsJSON reports whether the renderer is in JSON mode.
func (r *Renderer) IsJSON() bool { return r.Mode == ModeJSON }

// JSON marshals v as JSON to stdout with a trailing newline.
func (r *Renderer) JSON(v any) error {
	enc := json.NewEncoder(r.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// JSONLine marshals v as a single line of NDJSON to stdout.
func (r *Renderer) JSONLine(v any) error {
	enc := json.NewEncoder(r.Out)
	return enc.Encode(v)
}

// Print writes to stdout. Does not append a newline.
func (r *Renderer) Print(s string) {
	fmt.Fprint(r.Out, s)
}

// Println writes a line to stdout.
func (r *Renderer) Println(args ...any) {
	fmt.Fprintln(r.Out, args...)
}

// Printf writes formatted output to stdout.
func (r *Renderer) Printf(format string, args ...any) {
	fmt.Fprintf(r.Out, format, args...)
}

// Status writes diagnostic noise to stderr.
func (r *Renderer) Status(format string, args ...any) {
	fmt.Fprintf(r.Err, format, args...)
}

// Statusln writes a status line to stderr.
func (r *Renderer) Statusln(args ...any) {
	fmt.Fprintln(r.Err, args...)
}

// Section prints a heading to stdout.
func (r *Renderer) Section(title string) {
	if r.Color {
		fmt.Fprintf(r.Out, "\x1b[1m%s\x1b[0m\n", title)
		return
	}
	fmt.Fprintln(r.Out, title)
}

// Bold returns a styled string when color is enabled.
func (r *Renderer) Bold(s string) string {
	if r.Color {
		return "\x1b[1m" + s + "\x1b[0m"
	}
	return s
}

// Dim returns a dimmed string when color is enabled.
func (r *Renderer) Dim(s string) string {
	if r.Color {
		return "\x1b[2m" + s + "\x1b[0m"
	}
	return s
}

// Green returns a green string when color is enabled.
func (r *Renderer) Green(s string) string {
	if r.Color {
		return "\x1b[32m" + s + "\x1b[0m"
	}
	return s
}

// Red returns a red string when color is enabled.
func (r *Renderer) Red(s string) string {
	if r.Color {
		return "\x1b[31m" + s + "\x1b[0m"
	}
	return s
}

// Yellow returns a yellow string when color is enabled.
func (r *Renderer) Yellow(s string) string {
	if r.Color {
		return "\x1b[33m" + s + "\x1b[0m"
	}
	return s
}

// Tick returns ✓ if TTY else ASCII v.
func (r *Renderer) Tick() string {
	if r.TTY {
		return r.Green("✓")
	}
	return "[ok]"
}

// Cross returns ✗ if TTY else ASCII x.
func (r *Renderer) Cross() string {
	if r.TTY {
		return r.Red("✗")
	}
	return "[fail]"
}

// Warn returns ⚠ if TTY else ASCII !.
func (r *Renderer) Warn() string {
	if r.TTY {
		return r.Yellow("⚠")
	}
	return "[warn]"
}

// Table renders a simple aligned table to stdout.
type Table struct {
	r       *Renderer
	headers []string
	rows    [][]string
}

func (r *Renderer) Table(headers ...string) *Table {
	return &Table{r: r, headers: headers}
}

func (t *Table) Row(cells ...string) {
	t.rows = append(t.rows, cells)
}

func (t *Table) Render() {
	cols := len(t.headers)
	if cols == 0 {
		return
	}
	widths := make([]int, cols)
	for i, h := range t.headers {
		widths[i] = displayWidth(h)
	}
	for _, row := range t.rows {
		for i := 0; i < cols && i < len(row); i++ {
			w := displayWidth(row[i])
			if w > widths[i] {
				widths[i] = w
			}
		}
	}

	// header
	parts := make([]string, cols)
	for i, h := range t.headers {
		parts[i] = padRight(h, widths[i])
	}
	t.r.Out.Write([]byte(t.r.Bold(strings.Join(parts, "  ")) + "\n"))

	for _, row := range t.rows {
		parts := make([]string, cols)
		for i := 0; i < cols; i++ {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			parts[i] = padRight(cell, widths[i])
		}
		t.r.Out.Write([]byte(strings.Join(parts, "  ") + "\n"))
	}
}

func displayWidth(s string) int {
	// Strip ANSI escape sequences for width calculation. Naive but adequate.
	stripped := stripANSI(s)
	width := 0
	for _, r := range stripped {
		if r < 0x20 {
			continue
		}
		width++
	}
	return width
}

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == 0x1b {
			inEsc = true
			continue
		}
		if inEsc {
			if c == 'm' || c == 'K' {
				inEsc = false
			}
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

func padRight(s string, n int) string {
	w := displayWidth(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}

// KeyValue writes a labeled `key: value` line to stdout.
func (r *Renderer) KeyValue(key, value string) {
	if r.Color {
		fmt.Fprintf(r.Out, "%s %s\n", r.Bold(key+":"), value)
		return
	}
	fmt.Fprintf(r.Out, "%s: %s\n", key, value)
}

// HumanBytes formats a byte count as a human-readable size.
func HumanBytes(n int64) string {
	const k = 1024
	if n < k {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	v := float64(n) / k
	i := 0
	for v >= k && i < len(units)-1 {
		v /= k
		i++
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}

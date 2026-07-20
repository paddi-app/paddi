package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/mattn/go-runewidth"

	"github.com/paddi-app/paddi/internal/api"
)

// colPad is the number of spaces separating adjacent columns.
const colPad = 2

// Table renders rows as a column-aligned table with a header row. Column
// widths are measured by terminal display width so that double-width runes
// (CJK, etc.) stay aligned.
func Table(w io.Writer, headers []string, rows [][]string) error {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = runewidth.StringWidth(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) {
				if cw := runewidth.StringWidth(cell); cw > widths[i] {
					widths[i] = cw
				}
			}
		}
	}

	var b strings.Builder
	writeRow(&b, headers, widths)
	for _, row := range rows {
		writeRow(&b, row, widths)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// writeRow writes one tab-free row, padding every cell but the last to its
// column's display width.
func writeRow(b *strings.Builder, cells []string, widths []int) {
	for i, width := range widths {
		var cell string
		if i < len(cells) {
			cell = cells[i]
		}
		b.WriteString(cell)
		if i < len(widths)-1 {
			b.WriteString(strings.Repeat(" ", width-runewidth.StringWidth(cell)+colPad))
		}
	}
	b.WriteByte('\n')
}

// JSON writes raw API JSON followed by a newline.
func JSON(w io.Writer, raw json.RawMessage) error {
	_, err := w.Write(append(bytes.TrimRight(raw, "\n"), '\n'))
	return err
}

// OrNone returns s, or a placeholder when it's empty.
func OrNone(s string) string {
	if s == "" {
		return "(not set)"
	}
	return s
}

// Truncate collapses whitespace in s and shortens it to n runes, appending an
// ellipsis when truncated.
func Truncate(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// SpecFilename derives a safe local filename from a spec's title.
func SpecFilename(s *api.Spec) string {
	title := strings.TrimSpace(s.Title)
	if title == "" {
		return s.ID + ".md"
	}
	repl := strings.NewReplacer(
		"/", "-", "\\", "-", ":", "-", "*", "-", "?", "-",
		"\"", "-", "<", "-", ">", "-", "|", "-",
	)
	return repl.Replace(title) + ".md"
}

// Request renders a request's analysis, score, and solution paths.
func Request(w io.Writer, r *api.Request) {
	fmt.Fprintf(w, "%s (%s)\n", r.Name, r.ID)
	fmt.Fprintf(w, "Type: %s  Status: %s  Score: %.1f\n", r.Type, r.Status, r.Score)
	fmt.Fprintf(w, "RIGE: reach %.2g x impact %.2g x goal %.2g / effort %.2g\n", r.Reach, r.Impact, r.GoalAlignment, r.Effort)
	if r.Description != "" {
		fmt.Fprintf(w, "\nDescription:\n%s\n", r.Description)
	}
	if r.Analysis != "" {
		fmt.Fprintf(w, "\nAnalysis:\n%s\n", r.Analysis)
	}
	if len(r.Captures) > 0 {
		fmt.Fprintf(w, "\nCaptures: %d\n", len(r.Captures))
	}
	if len(r.SolutionPaths) > 0 {
		fmt.Fprintln(w, "\nSolution paths:")
		for i, p := range r.SolutionPaths {
			multiple := ""
			if p.Multiple {
				multiple = " [multiple]"
			}
			fmt.Fprintf(w, "%d. %s%s (%s)\n", i+1, p.Question, multiple, p.ID)
			if p.Context != "" {
				fmt.Fprintf(w, "   Context: %s\n", p.Context)
			}
			if p.Impact != "" {
				fmt.Fprintf(w, "   Impact: %s\n", p.Impact)
			}
			for _, o := range p.Options {
				fmt.Fprintf(w, "   - %s — %s\n", o.Label, o.Impact)
			}
			for _, sel := range p.Selections {
				custom := ""
				if sel.Custom {
					custom = " (custom)"
				}
				fmt.Fprintf(w, "   > selected: %s%s\n", sel.Label, custom)
			}
		}
	}
	if r.Spec != nil {
		locked := ""
		if r.Spec.Locked {
			locked = ", locked"
		}
		fmt.Fprintf(w, "\nSpec: %s (%s%s)\n", r.Spec.Title, r.Spec.ID, locked)
	}
}

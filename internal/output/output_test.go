package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/paddi-app/paddi/internal/api"
)

func TestTable(t *testing.T) {
	cases := []struct {
		name    string
		headers []string
		rows    [][]string
		want    string
	}{
		{
			name:    "pads to widest cell",
			headers: []string{"ID", "NAME"},
			rows:    [][]string{{"1", "short"}, {"22", "a longer name"}},
			want: "ID  NAME\n" +
				"1   short\n" +
				"22  a longer name\n",
		},
		{
			name:    "aligns double-width runes by display width",
			headers: []string{"NAME", "X"},
			rows:    [][]string{{"你好", "a"}},
			want: "NAME  X\n" +
				"你好  a\n",
		},
		{
			name:    "no rows still prints header",
			headers: []string{"ID"},
			rows:    nil,
			want:    "ID\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			if err := Table(&b, tc.headers, tc.rows); err != nil {
				t.Fatalf("Table() error = %v", err)
			}
			if got := b.String(); got != tc.want {
				t.Errorf("Table() =\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		name string
		s    string
		n    int
		want string
	}{
		{"under limit unchanged", "short", 10, "short"},
		{"collapses internal whitespace", "a  b\nc", 10, "a b c"},
		{"exact limit unchanged", "abcde", 5, "abcde"},
		{"truncates with ellipsis", "abcdef", 5, "abcd…"},
		{"counts runes not bytes", "日本語のテスト", 3, "日本…"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Truncate(tc.s, tc.n); got != tc.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tc.s, tc.n, got, tc.want)
			}
		})
	}
}

func TestSpecFilename(t *testing.T) {
	cases := []struct {
		name string
		spec *api.Spec
		want string
	}{
		{"falls back to id when title empty", &api.Spec{ID: "abc123", Title: "  "}, "abc123.md"},
		{"replaces path-unsafe characters", &api.Spec{ID: "x", Title: "a/b\\c:d*e?f\"g<h>i|j"}, "a-b-c-d-e-f-g-h-i-j.md"},
		{"leaves safe title untouched", &api.Spec{ID: "x", Title: "My Spec"}, "My Spec.md"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SpecFilename(tc.spec); got != tc.want {
				t.Errorf("SpecFilename() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestOrNone(t *testing.T) {
	if got := OrNone(""); got != "(not set)" {
		t.Errorf("OrNone(\"\") = %q, want %q", got, "(not set)")
	}
	if got := OrNone("x"); got != "x" {
		t.Errorf("OrNone(%q) = %q, want %q", "x", got, "x")
	}
}

func TestJSON(t *testing.T) {
	cases := []struct {
		name string
		raw  json.RawMessage
		want string
	}{
		{"adds trailing newline", json.RawMessage(`{"a":1}`), "{\"a\":1}\n"},
		{"does not double an existing trailing newline", json.RawMessage("{\"a\":1}\n"), "{\"a\":1}\n"},
		{"empty input yields just a newline", json.RawMessage(``), "\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b bytes.Buffer
			if err := JSON(&b, tc.raw); err != nil {
				t.Fatalf("JSON() error = %v", err)
			}
			if got := b.String(); got != tc.want {
				t.Errorf("JSON() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRequest(t *testing.T) {
	cases := []struct {
		name string
		req  *api.Request
		want string
	}{
		{
			name: "minimal request omits every optional section",
			req: &api.Request{
				ID: "r1", Name: "Fix bug", Type: "bug", Status: "open",
				Reach: 0.5, Impact: 0.8, GoalAlignment: 0.3, Effort: 0.2, Score: 4.2,
			},
			want: "Fix bug (r1)\n" +
				"Type: bug  Status: open  Score: 4.2\n" +
				"RIGE: reach 0.5 x impact 0.8 x goal 0.3 / effort 0.2\n",
		},
		{
			name: "full request renders every section, multiple flag, and custom selection",
			req: &api.Request{
				ID: "r1", Name: "Fix bug", Type: "bug", Status: "open",
				Reach: 0.5, Impact: 0.8, GoalAlignment: 0.3, Effort: 0.2, Score: 4.2,
				Description: "desc text",
				Analysis:    "analysis text",
				Captures:    []api.Capture{{ID: "c1"}, {ID: "c2"}},
				SolutionPaths: []api.SolutionPath{
					{
						ID: "sp1", Question: "Which approach?", Context: "ctx", Impact: "high", Multiple: true,
						Options:    []api.SolutionOption{{Label: "A", Impact: "low"}},
						Selections: []api.SolutionSelection{{Label: "A"}, {Label: "B", Custom: true}},
					},
				},
				Spec: &api.Spec{ID: "s1", Title: "My Spec", Locked: true},
			},
			want: "Fix bug (r1)\n" +
				"Type: bug  Status: open  Score: 4.2\n" +
				"RIGE: reach 0.5 x impact 0.8 x goal 0.3 / effort 0.2\n" +
				"\nDescription:\ndesc text\n" +
				"\nAnalysis:\nanalysis text\n" +
				"\nCaptures: 2\n" +
				"\nSolution paths:\n" +
				"1. Which approach? [multiple] (sp1)\n" +
				"   Context: ctx\n" +
				"   Impact: high\n" +
				"   - A — low\n" +
				"   > selected: A\n" +
				"   > selected: B (custom)\n" +
				"\nSpec: My Spec (s1, locked)\n",
		},
		{
			name: "single-select solution path without context/impact/options, unlocked spec",
			req: &api.Request{
				ID: "r1", Name: "Fix bug", Type: "bug", Status: "open",
				SolutionPaths: []api.SolutionPath{
					{ID: "sp1", Question: "Which approach?"},
				},
				Spec: &api.Spec{ID: "s1", Title: "My Spec"},
			},
			want: "Fix bug (r1)\n" +
				"Type: bug  Status: open  Score: 0.0\n" +
				"RIGE: reach 0 x impact 0 x goal 0 / effort 0\n" +
				"\nSolution paths:\n" +
				"1. Which approach? (sp1)\n" +
				"\nSpec: My Spec (s1)\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			Request(&b, tc.req)
			if got := b.String(); got != tc.want {
				t.Errorf("Request() =\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
}

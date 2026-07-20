package prompt

import (
	"os"
	"strings"
	"testing"

	"golang.org/x/term"
)

func TestSelect_NoItems(t *testing.T) {
	if _, err := Select("pick one", nil); err == nil {
		t.Error("Select() error = nil, want error for empty items")
	}
}

func TestSelect_RequiresTerminal(t *testing.T) {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		t.Skip("stdin is a terminal in this environment; the non-terminal path isn't reachable")
	}
	_, err := Select("pick one", []Item{{ID: "1", Label: "one"}})
	if err == nil {
		t.Error("Select() error = nil, want an error when stdin isn't a terminal")
	}
}

func TestRender(t *testing.T) {
	var b strings.Builder
	items := []Item{{ID: "1", Label: "First"}, {ID: "2", Label: "Second"}}
	render(&b, "Pick one:", items, 1)
	want := "Pick one:\r\n" +
		"  First\r\n" +
		"\x1b[36m> Second\x1b[0m\r\n"
	if got := b.String(); got != want {
		t.Errorf("render() =\n%q\nwant\n%q", got, want)
	}
}

func TestErase(t *testing.T) {
	var b strings.Builder
	erase(&b, 3)
	want := "\x1b[3A\x1b[J"
	if got := b.String(); got != want {
		t.Errorf("erase() = %q, want %q", got, want)
	}
}

func TestIsUpIsDown(t *testing.T) {
	cases := []struct {
		name     string
		key      []byte
		wantUp   bool
		wantDown bool
	}{
		{"up arrow", []byte{27, '[', 'A'}, true, false},
		{"down arrow", []byte{27, '[', 'B'}, false, true},
		{"k selects up (vim binding)", []byte{'k'}, true, false},
		{"j selects down (vim binding)", []byte{'j'}, false, true},
		{"enter is neither", []byte{'\r'}, false, false},
		{"unrelated escape sequence", []byte{27, '[', 'C'}, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isUp(tc.key); got != tc.wantUp {
				t.Errorf("isUp(%v) = %v, want %v", tc.key, got, tc.wantUp)
			}
			if got := isDown(tc.key); got != tc.wantDown {
				t.Errorf("isDown(%v) = %v, want %v", tc.key, got, tc.wantDown)
			}
		})
	}
}

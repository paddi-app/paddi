// Package prompt provides small interactive terminal components.
package prompt

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mattn/go-runewidth"
	"golang.org/x/term"
)

// ErrCancelled is returned by Select when the user aborts the selection
// (q, Esc, or Ctrl-C).
var ErrCancelled = errors.New("selection cancelled")

// Item is a single selectable entry. ID is returned to the caller; Label is
// shown to the user.
type Item struct {
	ID    string
	Label string
}

// Select renders an interactive list and returns the chosen item.
//
// Navigate with Up/Down or k/j, choose with Enter, cancel with q/Esc/Ctrl-C.
// It requires stdin to be a terminal and draws to stderr so stdout stays clean
// for piping. The drawn UI is erased before returning.
func Select(label string, items []Item) (Item, error) {
	if len(items) == 0 {
		return Item{}, errors.New("nothing to select")
	}
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return Item{}, errors.New("interactive selection requires a terminal")
	}

	state, err := term.MakeRaw(fd)
	if err != nil {
		return Item{}, err
	}
	defer func() { _ = term.Restore(fd, state) }()

	out := os.Stderr
	width, _, err := term.GetSize(fd)
	if err != nil || width <= 0 {
		width = 80 // ponytail: fallback when size is unavailable
	}
	lines := renderedLines(width, label, items) // physical rows, accounting for wrap

	fmt.Fprint(out, "\x1b[?25l")       // hide cursor
	defer fmt.Fprint(out, "\x1b[?25h") // show cursor

	cursor := 0
	render(out, label, items, cursor)

	buf := make([]byte, 3)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			erase(out, lines)
			return Item{}, err
		}
		key := buf[:n]
		switch {
		case n == 1 && (key[0] == 3 || key[0] == 27 || key[0] == 'q'): // Ctrl-C, Esc, q
			erase(out, lines)
			return Item{}, ErrCancelled
		case key[0] == '\r' || key[0] == '\n': // Enter
			erase(out, lines)
			return items[cursor], nil
		case isUp(key) && cursor > 0:
			cursor--
		case isDown(key) && cursor < len(items)-1:
			cursor++
		default:
			continue
		}
		fmt.Fprintf(out, "\x1b[%dA\x1b[J", lines) // move to top of list, clear below
		render(out, label, items, cursor)
	}
}

func isUp(key []byte) bool {
	return (len(key) == 3 && key[0] == 27 && key[1] == '[' && key[2] == 'A') ||
		(len(key) == 1 && key[0] == 'k')
}

func isDown(key []byte) bool {
	return (len(key) == 3 && key[0] == 27 && key[1] == '[' && key[2] == 'B') ||
		(len(key) == 1 && key[0] == 'j')
}

func render(out io.Writer, label string, items []Item, cursor int) {
	var b strings.Builder
	b.WriteString(label + "\r\n")
	for i, it := range items {
		if i == cursor {
			b.WriteString("\x1b[36m> " + it.Label + "\x1b[0m\r\n") // cyan
		} else {
			b.WriteString("  " + it.Label + "\r\n")
		}
	}
	fmt.Fprint(out, b.String())
}

// renderedLines counts the physical terminal rows render() produces, treating a
// line wider than the terminal as wrapping onto extra rows. The color escapes
// and "> "/"  " prefix occupy no or 2 columns respectively.
func renderedLines(width int, label string, items []Item) int {
	rows := rowsFor(runewidth.StringWidth(label), width)
	for _, it := range items {
		rows += rowsFor(2+runewidth.StringWidth(it.Label), width)
	}
	return rows
}

func rowsFor(cells, width int) int {
	if cells <= 0 {
		return 1
	}
	return (cells + width - 1) / width // ceil(cells/width)
}

func erase(out io.Writer, lines int) {
	fmt.Fprintf(out, "\x1b[%dA\x1b[J", lines)
}

package app

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// Wrapping is on from startup: a line longer than the text width
// renders across rows at the current text width — panel minus gutter
// minus the reserved indicator width, zero in wrap mode — and
// continuation rows carry a blank gutter aligned with the first row's
// text.
func TestWrapOnByDefaultBlankContinuationGutter(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "a.txt", strings.Repeat("x", 100)+"\nshort\n")
	m, cmd := browseModel(t, dir, 80, 24,
		`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"short\n"},"line_number":2,"submatches":[{"match":{"text":"short"},"start":0,"end":5}]}}`,
		`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}`,
		`{"type":"summary","data":{}}`,
	)
	m, _ = update(t, m, cmd())

	// listW = 7 (a.txt + padding), gutterW = 3, reserved 0 → textW 70.
	// The single file occupies only list row 0, so content rows carry
	// seven blank list cells before the gutter.
	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	row0 := strings.TrimRight(lines[1], " ")
	row1 := strings.TrimRight(lines[2], " ")
	row2 := strings.TrimRight(lines[3], " ")
	if want := "       1  " + strings.Repeat("x", 70); row0 != want {
		t.Fatalf("first content row = %q, want %q", row0, want)
	}
	// Continuation row: blank list column, blank gutter, the next 30
	// cells aligned where the first row's text began.
	if want := "          " + strings.Repeat("x", 30); row1 != want {
		t.Fatalf("continuation row = %q, want %q", row1, want)
	}
	if want := "       2  short"; row2 != want {
		t.Fatalf("next line row = %q, want %q", row2, want)
	}
}

// w toggles to run-off-edge mode — one clipped row per line with the
// rightmost column reserved for the indicator — and back to wrap.
func TestWTogglesRunOffEdge(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "a.txt", strings.Repeat("x", 100)+"\nshort\n")
	m, cmd := browseModel(t, dir, 80, 24,
		`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"short\n"},"line_number":2,"submatches":[{"match":{"text":"short"},"start":0,"end":5}]}}`,
		`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}`,
		`{"type":"summary","data":{}}`,
	)
	m, _ = update(t, m, cmd())

	m, c := update(t, m, keyPress("w"))
	if c != nil {
		t.Fatalf("w produced a command %T, want none", c)
	}
	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	// Run-off-edge reserves one indicator column: textW = 69.
	if want := "       1  " + strings.Repeat("x", 69); strings.TrimRight(lines[1], " ") != want {
		t.Fatalf("run-off-edge row = %q, want %q", strings.TrimRight(lines[1], " "), want)
	}
	if strings.Contains(lines[2], "x") {
		t.Fatalf("run-off-edge still wraps: %q", lines[2])
	}

	m, _ = update(t, m, keyPress("w"))
	lines = strings.Split(ansi.Strip(m.View().Content), "\n")
	if want := "          " + strings.Repeat("x", 30); strings.TrimRight(lines[2], " ") != want {
		t.Fatalf("after w again continuation row = %q, want %q", strings.TrimRight(lines[2], " "), want)
	}
}

// A match far down a wrapped source line is revealed on its own
// rendered row: the destination line's wrapped rows place the match
// start's row at floor(content height/3). The long line is line 1 —
// with 60 lines the gutter takes two digits, so textW is 69 — so a
// same-file n away and p back reveal it again.
func TestNRevealInsideWrappedLine(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("x", 1900) + "hit" + strings.Repeat("x", 97) // 2000 cells
	var sb strings.Builder
	sb.WriteString(long + "\n")
	for i := 2; i <= 60; i++ {
		if i == 40 {
			sb.WriteString("hit040\n")
		} else {
			fmt.Fprintf(&sb, "x%05d\n", i)
		}
	}
	writeWorkFile(t, dir, "a.txt", sb.String())
	m, cmd := browseModel(t, dir, 80, 24,
		`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
		fmt.Sprintf(`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":%s},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":1900,"end":1903}]}}`,
			strconv.Quote(long+"\n")),
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit040\n"},"line_number":40,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}`,
		`{"type":"summary","data":{}}`,
	)
	m, _ = update(t, m, cmd())

	// Startup reveal: the 2000-cell line wraps into rows 0..28; cell
	// 1900 sits in row 27 and floor(23/3) = 7, so the top lands at 20.
	if m.vp.Top() != 20 {
		t.Fatalf("top after startup reveal = %d, want 20 — row 27 at row 7", m.vp.Top())
	}

	// n to the line-40 match: source line 39 is rendered row 67, so
	// the top moves to 60.
	m, _ = update(t, m, keyPress("n"))
	if m.vp.Top() != 60 {
		t.Fatalf("top after n = %d, want 60 — row 67 at row 7", m.vp.Top())
	}

	// p back to the wrapped-line match: hidden above the window, the
	// reveal lands row 27 at row 7 again.
	m, _ = update(t, m, keyPress("p"))
	if m.vp.Top() != 20 {
		t.Fatalf("top after p = %d, want 20", m.vp.Top())
	}
	if v := ansi.Strip(m.View().Content); !strings.Contains(v, "hit") {
		t.Fatalf("revealed view lacks the match: %q", v)
	}
}

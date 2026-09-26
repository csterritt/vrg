package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// panRecords is a one-stop record set for a file whose first line is
// the match.
func panRecords(path, line1 string) []string {
	return []string{
		`{"type":"begin","data":{"path":{"text":"` + path + `"}}}`,
		`{"type":"match","data":{"path":{"text":"` + path + `"},"lines":{"text":"` + line1 + `\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"end","data":{"path":{"text":"` + path + `"},"binary_offset":null}}`,
	}
}

// , . < > [ ] drive the horizontal offset in run-off-edge mode: one
// column, ten columns, and max(1, floor(text width / 2)) — all clamped
// to the visible-lines extent.
func TestPanKeysMoveOffset(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "a.txt", "hit\n"+strings.Repeat("x", 300)+"\n")
	recs := append(panRecords("a.txt", "hit"), `{"type":"summary","data":{}}`)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)
	m = pump(t, m, keyPress("w"))

	half := max(1, m.wantTextW()/2)
	for _, s := range []struct {
		key  string
		want int
	}{
		{".", 1},
		{">", 11},
		{"]", 11 + half},
		{",", 10 + half},
		{"[", 10},
	} {
		m, _ = update(t, m, keyPress(s.key))
		if m.vp.Offset() != s.want {
			t.Fatalf("offset after %q = %d, want %d", s.key, m.vp.Offset(), s.want)
		}
	}
}

// In wrap mode the pan keys are inert: the offset stays at zero and
// the wrapped rendering is untouched.
func TestPanKeysNoOpInWrapMode(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "a.txt", "hit\n"+strings.Repeat("x", 300)+"\n")
	recs := append(panRecords("a.txt", "hit"), `{"type":"summary","data":{}}`)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)

	before := m.View().Content
	for _, k := range []string{",", ".", "<", ">", "[", "]"} {
		m, _ = update(t, m, keyPress(k))
	}
	if m.vp.Offset() != 0 {
		t.Fatalf("offset after wrap-mode pans = %d, want 0", m.vp.Offset())
	}
	if m.View().Content != before {
		t.Fatal("wrap-mode pan keys changed the view")
	}
}

// w w keeps the offset through the asynchronous layout swap: re-entry
// into run-off-edge mode with an unchanged visible set leaves the
// stored offset inside the recomputed maximum.
func TestPanOffsetSurvivesWrapToggle(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "a.txt", "hit\n"+strings.Repeat("x", 300)+"\n")
	recs := append(panRecords("a.txt", "hit"), `{"type":"summary","data":{}}`)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)
	m = pump(t, m, keyPress("w"))
	m, _ = update(t, m, keyPress(">"))
	if m.vp.Offset() != 10 {
		t.Fatalf("offset = %d, want 10", m.vp.Offset())
	}
	m = pump(t, m, keyPress("w"))
	m = pump(t, m, keyPress("w"))
	if m.vp.Offset() != 10 {
		t.Fatalf("offset after w w = %d, want the retained 10", m.vp.Offset())
	}
}

// n into another file resets the horizontal offset to zero before any
// reveal — and p back does not restore the old value.
func TestPanOffsetResetsOnFileChange(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "a.txt", "hit\n"+strings.Repeat("x", 300)+"\n")
	writeWorkFile(t, dir, "b.txt", "hit\n"+strings.Repeat("y", 300)+"\n")
	recs := append(panRecords("a.txt", "hit"), panRecords("b.txt", "hit")...)
	recs = append(recs, `{"type":"summary","data":{}}`)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)
	m = pump(t, m, keyPress("w"))
	m, _ = update(t, m, keyPress(">"))
	if m.vp.Offset() != 10 {
		t.Fatalf("offset = %d, want 10", m.vp.Offset())
	}

	m = pump(t, m, keyPress("n"))
	if string(m.currentPath()) != "b.txt" || m.vp.Offset() != 0 {
		t.Fatalf("after n: path=%s offset=%d, want b.txt at 0", m.currentPath(), m.vp.Offset())
	}
	m = pump(t, m, keyPress("p"))
	if string(m.currentPath()) != "a.txt" || m.vp.Offset() != 0 {
		t.Fatalf("after p: path=%s offset=%d, want a.txt at 0 — reset, not restored", m.currentPath(), m.vp.Offset())
	}
}

// A wide glyph half-clipped at the left edge paints blank cells rather
// than a broken glyph: at offset 3 the window opens on 世's trailing
// cell, so the text area shows a blank then the c's.
func TestHalfClippedGlyphPaintsBlank(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "a.txt", "hit\nab世"+strings.Repeat("c", 300)+"\n")
	recs := append(panRecords("a.txt", "hit"), `{"type":"summary","data":{}}`)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)
	m = pump(t, m, keyPress("w"))
	for i := 0; i < 3; i++ {
		m, _ = update(t, m, keyPress("."))
	}
	if m.vp.Offset() != 3 {
		t.Fatalf("offset = %d, want 3", m.vp.Offset())
	}

	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	// The text area begins after the file list and the gutter; the
	// first text cell of the second content row is the blank standing
	// in for 世's clipped half.
	col := m.listW + m.gutterDigits() + 2
	row := lines[2]
	if len(row) < col+2 || row[col] != ' ' || row[col+1] != 'c' {
		t.Fatalf("row after half-clip = %q, want a blank then c's at column %d", row, col)
	}
}

// Panning to the maximum stops at the widest visible line's final
// cluster's start so the cluster paints whole — and further pans do
// nothing.
func TestPanToMaximumPaintsFinalCluster(t *testing.T) {
	dir := t.TempDir()
	// Line 2 is 80 x's plus 世: content extent 82, final cluster start
	// 80 — the maximum offset.
	writeWorkFile(t, dir, "a.txt", "hit\n"+strings.Repeat("x", 80)+"世\n")
	recs := append(panRecords("a.txt", "hit"), `{"type":"summary","data":{}}`)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)
	m = pump(t, m, keyPress("w"))

	for i := 0; i < 100; i++ {
		m, _ = update(t, m, keyPress("."))
	}
	if m.vp.Offset() != 80 {
		t.Fatalf("offset at maximum = %d, want 80 — 世's start, not 81", m.vp.Offset())
	}
	m, _ = update(t, m, keyPress("]"))
	if m.vp.Offset() != 80 {
		t.Fatalf("offset past the maximum = %d, want 80", m.vp.Offset())
	}

	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	col := m.listW + m.gutterDigits() + 2
	if row := lines[2]; !strings.HasPrefix(row[col:], "世") {
		t.Fatalf("row at maximum = %q, want 世 fully painted at column %d", row, col)
	}
}

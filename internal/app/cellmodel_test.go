package app

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"vrg/internal/theme"
)

// b64sub renders a submatch member in the {"bytes": base64} form so a
// recorded match can cover bytes that are not valid UTF-8 on their
// own — a partial-grapheme match.
func b64sub(b []byte, start, end int) string {
	return fmt.Sprintf(`{"match":{"bytes":%q},"start":%d,"end":%d}`,
		base64.StdEncoding.EncodeToString(b), start, end)
}

// textCol is the frame column where a content row's first text cell
// lands: the list's cells, the digit gutter, and the two trailing
// spaces.
func textCol(m Model) int { return m.listW + m.gutterDigits() + 2 }

// A match overlapping a two-cell CJK character highlights exactly that
// cluster's cells: the recorded bytes sit inside the cluster, the
// whole glyph paints inverse, and the following character is never
// swallowed.
func TestComposedViewWideClusterMatchCoversExactlyItsCells(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "a.txt", "x世界y\n")
	// The recorded match covers one interior byte of 世 (bytes 1–3);
	// validation compares raw bytes, so the {"bytes":...} form carries
	// the non-UTF-8 fragment.
	m, cmd := browseModel(t, dir, 80, 24,
		`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"x世界y\n"},"line_number":1,"submatches":[`+
			b64sub([]byte{0xb8}, 2, 3)+`]}}`,
		`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}`,
		`{"type":"summary","data":{}}`,
	)
	m = settle(t, m, cmd)

	v := m.View().Content
	// The current matched line paints the whole cluster in inverse +
	// underline; 界 and the trailing y stay in the base colours.
	if !strings.Contains(v, "x\x1b[30;47;4m世\x1b[24;37;40m界y") {
		t.Fatalf("wide-cluster match not painted as exactly the cluster's cells: %q", v)
	}
	if strings.Contains(v, "世界\x1b[24;37;40m") {
		t.Fatalf("the highlight swallowed the following character: %q", v)
	}

	// Cell layout: the cluster owns cells 1–2 of the text area and the
	// following character stands on its own lead cells.
	row := frameLines(m)[1]
	col := textCol(m)
	for i, want := range []string{"x", "世", "世", "界", "界", "y"} {
		if got := cellAt(row, col+i); got != want {
			t.Fatalf("text cell %d = %q, want %q", i, got, want)
		}
	}
}

// A base-plus-combining sequence is styled and clipped only as one
// cluster: a combining-only match paints the whole é cell, and a clip
// edge splitting a wide combining cluster blanks its in-window cells
// rather than showing half the glyph.
func TestComposedViewCombiningClusterStaysWhole(t *testing.T) {
	dir := t.TempDir()
	l2 := strings.Repeat("q", 5) + "世́" + strings.Repeat("r", 80)
	writeWorkFile(t, dir, "a.txt", "abécd\n"+l2+"\n")
	// The line-1 match covers the combining mark's bytes alone (é's
	// bytes are e + U+0301 at 2–4); it highlights the whole é cell.
	m, cmd := browseModel(t, dir, 80, 24,
		`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"abécd\n"},"line_number":1,"submatches":[`+
			subJSON("́", 3, 5)+`]}}`,
		`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}`,
		`{"type":"summary","data":{}}`,
	)
	m = settle(t, m, cmd)

	v := m.View().Content
	if !strings.Contains(v, "b\x1b[30;47;4mé\x1b[24;37;40mc") {
		t.Fatalf("combining-only match did not style the whole cluster cell: %q", v)
	}

	// Run-off-edge mode: panning into line 2's 世́ cluster (cells 5–6)
	// splits it — the in-window cell paints blank, never half a glyph.
	m = pump(t, m, keyPress("w"))
	m.vp.SetOffset(6)
	row := frameLines(m)[2]
	col := textCol(m)
	if got := cellAt(row, col); got != " " {
		t.Fatalf("split combining cluster's in-window cell = %q, want a blank", got)
	}
	if strings.Contains(row, "世") {
		t.Fatalf("the clip edge let part of the split cluster paint: %q", row)
	}
}

// An emoji ZWJ sequence occupies its measured two cells as one
// cluster: an interior-bytes match covers the whole sequence and a
// clip boundary never splits it.
func TestComposedViewZWJClusterMeasuredAndUnsplit(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "a.txt", "a👨‍👩‍👧c\n")
	// The recorded match covers 👩's bytes inside the ZWJ sequence —
	// 👨 (4), ZWJ (3), 👩 (4) at offsets 8–11; the whole two-cell
	// cluster highlights.
	m, cmd := browseModel(t, dir, 80, 24,
		`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"a👨‍👩‍👧c\n"},"line_number":1,"submatches":[`+
			subJSON("👩", 8, 12)+`]}}`,
		`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}`,
		`{"type":"summary","data":{}}`,
	)
	m = settle(t, m, cmd)

	v := m.View().Content
	if !strings.Contains(v, "a\x1b[30;47;4m👨‍👩‍👧\x1b[24;37;40mc") {
		t.Fatalf("interior match did not paint the whole ZWJ cluster: %q", v)
	}
	row := frameLines(m)[1]
	col := textCol(m)
	for _, c := range []int{col + 1, col + 2} {
		if got := cellAt(row, c); got != "👨‍👩‍👧" {
			t.Fatalf("text cell %d = %q, want the whole two-cell cluster", c-col, got)
		}
	}

	// A clip edge inside the cluster blanks the in-window cell.
	m = pump(t, m, keyPress("w"))
	m.vp.SetOffset(2)
	row = frameLines(m)[1]
	col = textCol(m)
	if got := cellAt(row, col); got != " " {
		t.Fatalf("split ZWJ cluster's in-window cell = %q, want a blank", got)
	}
	if strings.Contains(row, "👨") {
		t.Fatalf("the clip edge split the ZWJ sequence: %q", row)
	}
}

// File-list entry padding and filename-row fitting measure cells, so a
// wide or combining path occupies exactly its measured width, never
// splits mid-cluster, and leaves the row exactly the frame's width.
func TestComposedViewWidePathListAndRuleGeometry(t *testing.T) {
	dir := t.TempDir()
	// Byte order puts "é.txt" (0x65…) before "世界.txt" (0xe4…), so the
	// first stop's file is the combining path; the wide path follows.
	recs := append(fileWithStops(t, dir, "世界.txt", 3, 1),
		fileWithStops(t, dir, "é.txt", 3, 1)...)
	recs = append(recs, `{"type":"summary","data":{}}`)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)

	if m.listW != 10 {
		t.Fatalf("listW = %d, want 10 — the longest entry's 8 cells plus two", m.listW)
	}
	lines := frameLines(m)
	// Row 0's list entry is the current file "é.txt" (5 cells, underlined
	// in the styled view) padded to the list's ten cells; the filename
	// rule starts on the next column.
	if got := cellAt(lines[0], 0); got != "é" {
		t.Fatalf("list entry cell 0 = %q, want the é cluster", got)
	}
	for c := 5; c < 10; c++ {
		if got := cellAt(lines[0], c); got != " " {
			t.Fatalf("list padding cell %d = %q, want a blank", c, got)
		}
	}
	if got := cellAt(lines[0], 10); got != "─" {
		t.Fatalf("cell 10 = %q, want the filename rule's lead dash", got)
	}
	if !strings.Contains(lines[0], "── é.txt ") {
		t.Fatalf("filename rule does not embed the combining path: %q", lines[0])
	}
	// Row 1's entry is "世界.txt" (8 cells) padded by two.
	for i, want := range []string{"世", "世", "界", "界", ".", "t", "x", "t", " ", " "} {
		if got := cellAt(lines[1], i); got != want {
			t.Fatalf("list entry cell %d = %q, want %q", i, got, want)
		}
	}
	for i, row := range lines {
		if w := ansi.StringWidth(row); w != 80 {
			t.Fatalf("frame row %d is %d cells, want 80: %q", i, w, row)
		}
	}

	// After the crossing, the wide path embeds in the rule whole.
	m, _ = update(t, m, keyPress("n"))
	m = settle(t, m, nil)
	m, _ = update(t, m, escPress()) // dismiss the file-change pop-up
	lines = frameLines(m)
	if !strings.Contains(lines[0], "── 世界.txt ") {
		t.Fatalf("filename rule does not embed the wide path: %q", lines[0])
	}
}

// In run-off-edge mode the indicator column still measures cells: a
// match hidden entirely left upgrades the gutter "_" to "*", a match
// partially inside the window draws no star, and the reserved column's
// "*" sits on the frame's last cell — all with two-cell text.
func TestComposedViewIndicatorColumnWideText(t *testing.T) {
	dir := t.TempDir()
	l1 := strings.Repeat("世", 40) // 80 cells
	writeWorkFile(t, dir, "a.txt", l1+"\n")
	recs := append(fileRecs("a.txt",
		// Match 1 at cells 0–1; match 2 on the last two cells 78–79
		// (世 is 3 bytes, so byte offsets 117–119).
		matchRec("a.txt", l1, 1, subJSON("世", 0, 3), subJSON("世", 117, 120)),
	), `{"type":"summary","data":{}}`)
	m := indModel(t, dir, recs...)
	if m.textW != 69 {
		t.Fatalf("textW = %d, want 69 for this fixture", m.textW)
	}
	ind, right := indCol(m), m.width-1

	// Offset 0: match 2 is entirely hidden right on the current line →
	// the reserved column's "*"; nothing is hidden left.
	lines := frameLines(m)
	if got := cellAt(lines[1], right); got != "*" {
		t.Fatalf("reserved column = %q, want \"*\" for the hidden-right match", got)
	}
	if got := cellAt(lines[1], ind); got != " " {
		t.Fatalf("gutter indicator = %q, want blank", got)
	}

	// Offset 11: match 1 is entirely hidden left → the gutter "*";
	// match 2's whole cluster paints inside the window → partially
	// visible, no right star. (Offset 10 would split the cluster's
	// lead cell — blanked cells are not visible, so the star would
	// legitimately stay.)
	m.vp.SetOffset(11)
	lines = frameLines(m)
	if got := cellAt(lines[1], ind); got != "*" {
		t.Fatalf("gutter indicator at offset 11 = %q, want \"*\"", got)
	}
	if got := cellAt(lines[1], right); got != " " {
		t.Fatalf("reserved column at offset 11 = %q, want blank — the match is partially visible", got)
	}
}

// The file-change pop-up sizes, truncates, and centres by measured
// cells: a path with a combining mark counts once per painted cell and
// an over-wide path left-truncates at a cluster boundary — never
// assuming the escaped path is free of combining marks.
func TestPopupWideAndCombiningPathGeometry(t *testing.T) {
	dir := t.TempDir()
	wide := strings.Repeat("世界", 20) + ".txt" // 84 cells — over the 78-cell budget
	recs := append(fileWithStops(t, dir, "a.txt", 4, 1, 3),
		fileWithStops(t, dir, "café.txt", 2, 1)...)
	recs = append(recs, fileWithStops(t, dir, wide, 2, 1)...)
	recs = append(recs, `{"type":"summary","data":{}}`)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)
	m.theme = theme.Plain()
	instantPopupTimer(&m)

	// n lands on a.txt:3 (same file — no pop-up); the next n crosses
	// into café.txt — é is e + U+0301, one cell of the 8-cell path.
	m, _ = update(t, m, keyPress("n"))
	if m.popupID != 0 {
		t.Fatal("a same-file step opened a pop-up")
	}
	m, _ = update(t, m, keyPress("n"))
	if m.popupID == 0 {
		t.Fatal("the crossing did not open a pop-up")
	}
	x, _, w, inner := popupBox(t, m.View().Content)
	if inner != "café.txt" || w != 10 {
		t.Fatalf("pop-up interior = %q width %d, want \"café.txt\" in a 10-cell box", inner, w)
	}
	if x != (80-w)/2 {
		t.Fatalf("pop-up x = %d, want centred at %d for a %d-cell box", x, (80-w)/2, w)
	}

	// The next crossing shows the over-wide path truncated at a
	// cluster boundary inside the frame.
	m, _ = update(t, m, keyPress("n"))
	if m.popupID == 0 {
		t.Fatal("the second crossing did not open a fresh pop-up")
	}
	x, _, w, inner = popupBox(t, m.View().Content)
	if !strings.HasPrefix(inner, "…") || !strings.HasSuffix(inner, ".txt") {
		t.Fatalf("truncated pop-up interior = %q, want a leading … and the .txt tail", inner)
	}
	if iw := ansi.StringWidth(inner); iw > 78 {
		t.Fatalf("pop-up interior is %d cells, over the 78-cell budget", iw)
	}
	if w > 80 {
		t.Fatalf("pop-up box is %d cells — overflows the frame", w)
	}
	if x != max(0, (80-w)/2) {
		t.Fatalf("pop-up x = %d, want centred at %d", x, max(0, (80-w)/2))
	}
	// The cut kept the path's rightmost whole clusters: the kept
	// suffix starts on a grapheme boundary of the escaped path.
	suffix := inner[len("…"):]
	var boundaries []int
	for i := 0; i < len(wide); {
		boundaries = append(boundaries, i)
		cl, _ := ansi.FirstGraphemeCluster(wide[i:], ansi.GraphemeWidth)
		i += len(cl)
	}
	cut := len(wide) - len(suffix)
	ok := false
	for _, b := range boundaries {
		if b == cut {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("the cut landed inside a cluster: suffix %q is not a cluster-boundary tail of %q",
			suffix, wide)
	}
}

package app

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// staleNote is the filename-row status text a stale buffer carries.
const staleNote = "file changed since search"

// writeStaleFixture writes a lines-line file: line i reads "xNNNNNN"
// unless special overrides it. It is fileWithStops's sibling for
// content that no longer matches the recorded search data.
func writeStaleFixture(t *testing.T, dir, name string, lines int, special map[int]string) {
	t.Helper()
	var b strings.Builder
	for i := 1; i <= lines; i++ {
		if s, ok := special[i]; ok {
			b.WriteString(s)
		} else {
			fmt.Fprintf(&b, "x%06d\n", i)
		}
	}
	writeWorkFile(t, dir, name, b.String())
}

// frameRow returns the composed frame's row containing needle, or ""
// when no row carries it.
func frameRow(v, needle string) string {
	for _, row := range strings.Split(v, "\n") {
		if strings.Contains(row, needle) {
			return row
		}
	}
	return ""
}

// A buffer whose recorded submatches fail validation is stale: the
// filename row carries "file changed since search" in the Issue #24
// status slot on every render — after scrolling, after navigation,
// after a resize — with no timer behind it, and the dropped matches
// never paint a highlight.
func TestStaleBufferShowsFileChangedNote(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "a.txt", "zzz\nzzz\nzzz\n")
	m, cmd := browseModel(t, dir, 80, 24,
		`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit\n"},"line_number":2,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}`,
		recSummary,
	)
	m = settle(t, m, cmd)

	assertNote := func(m Model, when string) {
		t.Helper()
		if v := ansi.Strip(m.View().Content); !strings.Contains(v, "a.txt "+staleNote) {
			t.Fatalf("view %s lacks the stale note: %q", when, v)
		}
	}
	assertNote(m, "on load")

	// Every display carries the note: scrolling and n/p leave it, and
	// so does a resize — nothing times it out.
	m, _ = update(t, m, codePress(tea.KeyDown))
	assertNote(m, "after scrolling")
	m, _ = update(t, m, keyPress("n"))
	assertNote(m, "after n")
	var lay tea.Cmd
	m, lay = update(t, m, tea.WindowSizeMsg{Width: 60, Height: 20})
	m = settle(t, m, lay)
	assertNote(m, "after resize")

	// The dropped submatches paint nothing: no inverse video anywhere
	// in the frame.
	if v := m.View().Content; strings.Contains(v, "\x1b[30;47") {
		t.Fatalf("a stale match still renders inverse: %q", v)
	}
}

// r recomputes the note: while the reread runs the verdict is not yet
// in, so the note stays; when the reverted bytes validate cleanly the
// note clears and the highlights return.
func TestStaleNoteClearsOnlyOnCleanReload(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "a.txt", "zzz\n")
	m, cmd := browseModel(t, dir, 80, 24,
		`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}`,
		recSummary,
	)
	m = settle(t, m, cmd)
	if v := ansi.Strip(m.View().Content); !strings.Contains(v, staleNote) {
		t.Fatalf("stale view lacks the note: %q", v)
	}

	// The file reverts under the held load; the note must survive the
	// whole reread and clear only on the validating completion.
	writeWorkFile(t, dir, "a.txt", "hit\n")
	gate := make(chan struct{})
	m.loadGate = gate
	m, reload := update(t, m, keyPress("r"))
	done := make(chan tea.Msg, 1)
	go func() { done <- reload() }()
	select {
	case <-done:
		t.Fatal("the reload completed while its gate was held")
	case <-time.After(50 * time.Millisecond):
	}
	if v := ansi.Strip(m.View().Content); !strings.Contains(v, staleNote) {
		t.Fatalf("the note vanished before the reload's verdict: %q", v)
	}

	close(gate)
	m = pump(t, m, <-done)
	v := ansi.Strip(m.View().Content)
	if strings.Contains(v, staleNote) {
		t.Fatalf("the note survived a clean reload: %q", v)
	}
	if got := m.View().Content; !strings.Contains(got, "\x1b[30;47;4mhit\x1b[24;37;40m") {
		t.Fatalf("the validating match did not return: %q", got)
	}
}

// The stale note composes in the filename-row status slot at every
// size: where it fits it paints whole — the long escaped path
// truncating to make room — and where it cannot fit even with an empty
// path it drops rather than clipping mid-text. No frame row overflows
// the terminal and no layout dimension goes negative.
func TestStaleNoteComposedAtAllWidths(t *testing.T) {
	dir := t.TempDir()
	// A path needing escapes and wide enough to force truncation: its
	// safe form carries \t and \n literally and far exceeds the slot.
	name := "a-very-long-filename-with\ttabs\nand-newlines.txt"
	writeWorkFile(t, dir, name, "zzz\n")
	q := strconv.Quote(name)
	recs := []string{
		fmt.Sprintf(`{"type":"begin","data":{"path":{"text":%s}}}`, q),
		fmt.Sprintf(`{"type":"match","data":{"path":{"text":%s},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`, q),
		fmt.Sprintf(`{"type":"end","data":{"path":{"text":%s},"binary_offset":null}}`, q),
		recSummary,
	}
	for _, sz := range []struct {
		w, h    int
		noteFit bool
	}{
		{80, 24, true}, {60, 15, true}, {40, 10, false}, {26, 6, false}, {20, 3, false},
	} {
		t.Run(fmt.Sprintf("%dx%d", sz.w, sz.h), func(t *testing.T) {
			m, cmd := browseModel(t, dir, sz.w, sz.h, recs...)
			m = settle(t, m, cmd)

			v := ansi.Strip(m.View().Content)
			rows := strings.Split(v, "\n")
			if len(rows) != sz.h {
				t.Fatalf("frame has %d rows, want %d — the escaped path must stay one line",
					len(rows), sz.h)
			}
			for i, row := range rows {
				if w := ansi.StringWidth(row); w > sz.w {
					t.Fatalf("row %d is %d cells wide, want ≤%d: %q", i, w, sz.w, row)
				}
			}
			if m.listW < 0 || m.textW < 0 {
				t.Fatalf("negative layout: listW=%d textW=%d", m.listW, m.textW)
			}
			if got := strings.Contains(rows[0], staleNote); got != sz.noteFit {
				t.Fatalf("note present = %v, want %v: %q", got, sz.noteFit, rows[0])
			}
			if !strings.Contains(rows[0], "…") {
				t.Fatalf("filename row lost the truncated path's marker: %q", rows[0])
			}
		})
	}
}

// The stale fallback reveal rides Issue #28's two-stage commit: while
// the current file's reread is gate-held, n selects the stop whose
// first recorded submatch the new bytes drop, and when the reload's
// matching prepared layout installs the commit reveals the first
// surviving submatch — never the dropped recorded span.
func TestGatedReloadCommitRevealsSurvivingSubmatch(t *testing.T) {
	dir := t.TempDir()
	writeStaleFixture(t, dir, "a.txt", 300, map[int]string{
		5:   "hit00005\n",
		200: "hit mid end\n",
	})
	recs := []string{
		`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit00005\n"},"line_number":5,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit mid end\n"},"line_number":200,"submatches":[{"match":{"text":"hit"},"start":0,"end":3},{"match":{"text":"end"},"start":8,"end":11}]}}`,
		`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}`,
		recSummary,
	}
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd) // a.txt v1 loads; cursor on line 5, top 0

	// v2 keeps the second recorded submatch's bytes in place while the
	// first's move on — a partial survival.
	writeStaleFixture(t, dir, "a.txt", 300, map[int]string{
		5:   "hit00005\n",
		200: "xxx mid end\n",
	})

	gate := make(chan struct{})
	m.loadGate = gate
	m, reload := update(t, m, keyPress("r"))
	done := make(chan tea.Msg, 1)
	go func() { done <- reload() }()
	select {
	case <-done:
		t.Fatal("the reload completed while its gate was held")
	case <-time.After(50 * time.Millisecond):
	}

	// The selection moves mid-load: the newest stop's reveal — with
	// the new content's stale verdict — is the intent the install
	// commits.
	m, _ = update(t, m, keyPress("n"))
	if s, _ := m.currentStop(); s.Line != 200 {
		t.Fatalf("n during the reload selected %+v, want a.txt:200", s)
	}

	close(gate)
	m = pump(t, m, <-done)
	if m.vp.Top() != 192 {
		t.Fatalf("top after the commit = %d, want 192 — the survivor's row a third down",
			m.vp.Top())
	}
	v := m.View().Content
	if !strings.Contains(v, "\x1b[30;47;4mend\x1b[24;37;40m") {
		t.Fatalf("the surviving submatch is not the revealed highlight: %q", v)
	}
	// The dropped submatch's bytes paint plain beside the survivor's
	// inverse — the ANSI boundary sits between "mid " and "end".
	if !strings.Contains(v, "xxx mid \x1b[30;47;4mend") {
		t.Fatalf("the dropped submatch did not paint plain beside the survivor: %q", v)
	}
	if !strings.Contains(ansi.Strip(v), "a.txt "+staleNote) {
		t.Fatalf("filename row lacks the stale note: %q", ansi.Strip(v))
	}
}

// The same gated route landing on the clamped fallback: the
// destination's only recorded submatch dropped, its line still
// present, so the commit reveals the row containing the recorded
// start's clamped cell — deep into a line that wraps — and paints no
// highlight there.
func TestGatedReloadCommitRevealsClampedFallback(t *testing.T) {
	dir := t.TempDir()
	writeStaleFixture(t, dir, "a.txt", 300, map[int]string{
		5:   "hit00005\n",
		200: strings.Repeat("a", 150) + "hit" + strings.Repeat("b", 50) + "\n",
	})
	recs := []string{
		`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit00005\n"},"line_number":5,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		fmt.Sprintf(`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"%s"},"line_number":200,"submatches":[{"match":{"text":"hit"},"start":150,"end":153}]}}`,
			strings.Repeat("a", 150)+"hit"+strings.Repeat("b", 50)+`\n`),
		`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}`,
		recSummary,
	}
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)

	// v2 moves the recorded bytes on: the only submatch drops, but
	// line 200 still exists — 203 display cells wrapping to three
	// rows at the 70-cell text width.
	writeStaleFixture(t, dir, "a.txt", 300, map[int]string{
		5:   "hit00005\n",
		200: strings.Repeat("a", 150) + "zzz" + strings.Repeat("b", 50) + "\n",
	})

	m, reload := update(t, m, keyPress("r"))
	m, _ = update(t, m, keyPress("n")) // a.txt:200 while the load is in flight
	m = settle(t, m, reload)

	// The clamped recorded start is cell 150 — the line's third
	// wrapped row (cells 140-202, rendered row 201) — so the reveal
	// lands it a third down, not the line's first row.
	if m.vp.Top() != 194 {
		t.Fatalf("top after the commit = %d, want 194 — the clamped cell's row", m.vp.Top())
	}
	v := m.View().Content
	row := frameRow(v, strings.Repeat("b", 50))
	if row == "" {
		t.Fatalf("the destination line's tail row is not visible: %q", v)
	}
	if strings.Contains(row, "30;47") {
		t.Fatalf("the fallback invented a highlight: %q", row)
	}
	if !strings.Contains(ansi.Strip(v), "a.txt "+staleNote) {
		t.Fatalf("filename row lacks the stale note: %q", ansi.Strip(v))
	}
}

// A stale stop whose submatches all dropped while its line still
// exists reveals the row at the clamped recorded start — the fallback
// paints no highlight and no marker.
func TestStaleAllDroppedRevealsClampedStart(t *testing.T) {
	dir := t.TempDir()
	writeStaleFixture(t, dir, "a.txt", 300, map[int]string{
		5:   "hit00005\n",
		200: "zzz ccc ddd\n",
	})
	m, cmd := browseModel(t, dir, 80, 24,
		`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit00005\n"},"line_number":5,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"aaa hit mid\n"},"line_number":200,"submatches":[{"match":{"text":"hit"},"start":4,"end":7}]}}`,
		`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}`,
		recSummary,
	)
	m = settle(t, m, cmd)

	m, _ = update(t, m, keyPress("n"))
	if s, _ := m.currentStop(); s.Line != 200 {
		t.Fatalf("n selected %+v, want a.txt:200", s)
	}
	if m.vp.Top() != 192 {
		t.Fatalf("top after n = %d, want 192 — the clamped-start row a third down", m.vp.Top())
	}
	v := m.View().Content
	row := frameRow(v, "zzz ccc ddd")
	if row == "" {
		t.Fatalf("the destination row is not visible: %q", v)
	}
	if strings.Contains(row, "30;47") {
		t.Fatalf("the fallback invented a highlight or marker: %q", row)
	}
}

// A stale stop whose line is gone lands at the last source line's
// start — no highlight, no invented marker, the entry stays a
// navigation stop.
func TestStaleMissingLineLandsOnLastSourceLine(t *testing.T) {
	dir := t.TempDir()
	writeStaleFixture(t, dir, "a.txt", 150, map[int]string{5: "hit00005\n"})
	m, cmd := browseModel(t, dir, 80, 24,
		`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit00005\n"},"line_number":5,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit00200\n"},"line_number":200,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}`,
		recSummary,
	)
	m = settle(t, m, cmd)

	m, _ = update(t, m, keyPress("n"))
	if s, _ := m.currentStop(); s.Line != 200 {
		t.Fatalf("n selected %+v, want the retained a.txt:200 stop", s)
	}
	// Line 200 does not exist in the 150-line file: the landing is the
	// last source line's start — rendered row 149, pinned to the
	// frame's bottom at the clamped top 127.
	if m.vp.Top() != 127 {
		t.Fatalf("top after n = %d, want 127 — the last source line at the frame's bottom",
			m.vp.Top())
	}
	v := m.View().Content
	row := frameRow(v, "x000150")
	if row == "" {
		t.Fatalf("the last source line is not visible: %q", v)
	}
	if strings.Contains(row, "30;47") {
		t.Fatalf("the missing-line fallback invented a highlight: %q", row)
	}
	if !strings.Contains(ansi.Strip(v), "a.txt "+staleNote) {
		t.Fatalf("filename row lacks the stale note: %q", ansi.Strip(v))
	}
}

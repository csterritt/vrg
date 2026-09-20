package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clipperhouse/displaywidth"

	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

// staleBuffer builds a prepared source whose recorded submatches all
// fail validation — the file's bytes differ from every recorded match —
// for delivery through an injected loadResult.
func staleBuffer(t *testing.T, stops []searchindex.Stop) *filebuffer.Buffer {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stale.txt")
	if err := os.WriteFile(path, []byte("zzz\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf, err := filebuffer.Load([]byte(path), stops)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !buf.Stale() {
		t.Fatal("the stale fixture did not come out stale")
	}
	return buf
}

// A buffer whose recorded submatches no longer all validate shows the
// buffer-status note "file changed since search" in the filename row's
// Issue 24 status slot — on every display, with no timer. A reload
// recomputes it: the note stands while the new content still fails
// validation and disappears only once it validates fully.
func TestStaleNoteInFilenameRow(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", "zzz one\nzzz two\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit one\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("a.txt", "hit two\n", 2, 0, 3, "hit"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m.theme = theme.Plain()

	if row := viewRow(t, m, 0); !strings.Contains(row, "file changed since search") {
		t.Fatalf("stale filename row lacks the note: %q", row)
	}
	// The note is not a transient: moving the cursor repaints the row
	// with it still in place.
	m, _ = update(t, m, keyMsg("n"))
	if row := viewRow(t, m, 0); !strings.Contains(row, "file changed since search") {
		t.Fatalf("the note did not persist across the display: %q", row)
	}

	// A reload whose content still fails validation keeps the note.
	writeMatchFile(t, dir, "a.txt", "hat one\nhat two\n")
	m, cmd = update(t, m, keyMsg("r"))
	m = applyLoad(t, m, cmd())
	if row := viewRow(t, m, 0); !strings.Contains(row, "file changed since search") {
		t.Fatalf("a still-stale reload cleared the note: %q", row)
	}

	// A reload whose content validates fully clears it.
	writeMatchFile(t, dir, "a.txt", "hit one\nhit two\n")
	m, cmd = update(t, m, keyMsg("r"))
	m = applyLoad(t, m, cmd())
	row := viewRow(t, m, 0)
	if strings.Contains(row, "file changed since search") {
		t.Fatalf("a validating reload kept the note: %q", row)
	}
	if !strings.Contains(row, "── a.txt ") {
		t.Fatalf("the filename row lost the path: %q", row)
	}
}

// The note is composed into the filename row at ordinary and
// constrained widths: the path truncates to make room where possible,
// nothing overflows, and the dimensions stay nonnegative.
func TestStaleNoteComposedAtConstrainedWidths(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("n", 60) + ".txt"
	writeMatchFile(t, dir, long, "zzz\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec(long, "hit\n", 1, 0, 3, "hit"))
	idx.Finish()

	for _, w := range []int{80, 60, 30, 20} {
		m, cmd := startBrowse(t, dir, idx, w, 10)
		m = applyLoad(t, m, cmd())
		m.theme = theme.Plain()

		row := viewRow(t, m, 0)
		if got := displaywidth.String(row); got != w {
			t.Fatalf("width %d: filename row is %d cells, want %d — nothing overflows",
				w, got, w)
		}
		if w >= 60 {
			// There is room to truncate the path for the whole note.
			if !strings.Contains(row, "file changed since search") {
				t.Fatalf("width %d: filename row lacks the note: %q", w, row)
			}
			if !strings.Contains(row, "…") {
				t.Fatalf("width %d: the path did not truncate for the note: %q", w, row)
			}
		}
	}
}

// The reload's two-stage completion carries the stale buffer: the
// latest selected stop's first recorded submatch dropped by the new
// content, and the matching prepared-layout install commits the reveal
// on the first surviving submatch — never the dropped start.
func TestStaleGatedReloadRevealsSurvivor(t *testing.T) {
	dir := t.TempDir()
	wide := func(mid string) string {
		return strings.Repeat("x", 100) + mid + strings.Repeat("x", 100) + "zzzz"
	}
	var b strings.Builder
	for i := 1; i <= 100; i++ {
		if i == 95 {
			b.WriteString(wide("aaaa") + "\n")
		} else {
			fmt.Fprintf(&b, "line-%02d\n", i)
		}
	}
	writeMatchFile(t, dir, "a.txt", b.String())
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-05\n", 5, 0, 4, "line"))
	addRec(t, idx, matchRec("a.txt", wide("aaaa")+"\n", 95, 100, 104, "aaaa"))
	addRec(t, idx, matchRec("a.txt", wide("aaaa")+"\n", 95, 204, 208, "zzzz"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m, cmd = update(t, m, keyMsg("w")) // run-off-edge, so the offset is observable
	m = deliverCmd(t, m, cmd)
	m, _ = update(t, m, keyMsg("n")) // the line-95 stop, the reload's subject

	// The file changes under the selection — the first recorded
	// submatch's bytes no longer hold — and r starts a gated reload.
	writeMatchFile(t, dir, "a.txt", strings.Replace(b.String(), wide("aaaa"), wide("bbbb"), 1))
	loadGate := make(chan struct{})
	m.loadGate = loadGate
	m, cmd = update(t, m, keyMsg("r"))
	jobR := runWorker(t, cmd)
	assertSilent(t, jobR, "a.txt's reload")

	// While the load is held, n wraps and p returns: the newest
	// selection stays the line-95 stop and the reveal stays pending.
	m, c := update(t, m, keyMsg("n"))
	if c != nil {
		t.Fatalf("n during the held reload returned a command: %v", c)
	}
	m, c = update(t, m, keyMsg("p"))
	if c != nil {
		t.Fatalf("p during the held reload returned a command: %v", c)
	}
	if stop, _ := m.index.Current(); stop.Line != 95 {
		t.Fatalf("the cursor = line %d, want the newest selection 95", stop.Line)
	}
	if m.pendingIntent != intentReveal {
		t.Fatal("the held reload lost the reveal intent")
	}

	// Stage one installs the recomputed buffer — stale — and requests
	// the prepared layout, itself held.
	layoutGate := make(chan struct{})
	m.layoutGate = layoutGate
	close(loadGate)
	m, cmd = update(t, m, collectMsg(t, jobR, "a.txt's reload"))
	jobL := runLayoutJob(t, cmd)
	assertHeld(t, jobL, "the reload's layout")

	// The matching install commits the reveal on the first survivor:
	// its cell 204 is a width-1 cluster, so the run-off-edge offset is
	// 204 + 1 − 67 = 138 (panel 73 − gutter 5 − reserved 1).
	close(layoutGate)
	m, _ = update(t, m, collectMsg(t, jobL, "the reload's layout"))
	if got := m.vps["a.txt"].Offset(); got != 138 {
		t.Fatalf("committed offset = %d, want 138 — the surviving submatch's cell", got)
	}
	if got := m.vps["a.txt"].Top(); got != 77 {
		t.Fatalf("committed top = %d, want 77", got)
	}
	if got := m.sources["a.txt"].Highlights(94); len(got) != 1 ||
		got[0] != (filebuffer.Span{Start: 204, End: 208}) {
		t.Fatalf("Highlights(94) = %+v, want [{204 208}] — the survivor alone", got)
	}
	m.theme = theme.Plain()
	if row := viewRow(t, m, 0); !strings.Contains(row, "file changed since search") {
		t.Fatalf("the stale reload's filename row lacks the note: %q", row)
	}
}

// With every recorded submatch dropped but the line still present, the
// committed reveal lands on the clamped recorded start — and the stale
// line invents no highlight: no inverse cell paints.
func TestStaleAllDroppedRevealsClampedStart(t *testing.T) {
	dir := t.TempDir()
	wide := func(mid string) string {
		return strings.Repeat("x", 100) + mid + strings.Repeat("x", 100) + "zzzz"
	}
	var b strings.Builder
	for i := 1; i <= 100; i++ {
		if i == 95 {
			b.WriteString(wide("aaaa") + "\n")
		} else {
			fmt.Fprintf(&b, "line-%02d\n", i)
		}
	}
	writeMatchFile(t, dir, "a.txt", b.String())
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-05\n", 5, 0, 4, "line"))
	addRec(t, idx, matchRec("a.txt", wide("aaaa")+"\n", 95, 100, 104, "aaaa"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m, cmd = update(t, m, keyMsg("w"))
	m = deliverCmd(t, m, cmd)
	m, _ = update(t, m, keyMsg("n"))

	writeMatchFile(t, dir, "a.txt", strings.Replace(b.String(), wide("aaaa"), wide("bbbb"), 1))
	loadGate := make(chan struct{})
	m.loadGate = loadGate
	m, cmd = update(t, m, keyMsg("r"))
	jobR := runWorker(t, cmd)
	assertSilent(t, jobR, "a.txt's reload")

	layoutGate := make(chan struct{})
	m.layoutGate = layoutGate
	close(loadGate)
	m, cmd = update(t, m, collectMsg(t, jobR, "a.txt's reload"))
	jobL := runLayoutJob(t, cmd)
	assertHeld(t, jobL, "the reload's layout")
	close(layoutGate)
	m, _ = update(t, m, collectMsg(t, jobL, "the reload's layout"))

	// The clamped recorded start is cell 100: offset 100 + 1 − 67 = 34.
	// The commit is proved by the consumed intent and the installed
	// revision-2 layout — the offset lands on the same cell the
	// pre-reload reveal used.
	if m.pendingIntent != intentNone {
		t.Fatal("the matching layout left the reveal intent pending")
	}
	if got := m.buffers["a.txt"].Key().Revision; got != 2 {
		t.Fatalf("installed revision = %d, want 2 — the reload's content", got)
	}
	if got := m.vps["a.txt"].Offset(); got != 34 {
		t.Fatalf("committed offset = %d, want 34 — the clamped recorded start", got)
	}
	if got := m.vps["a.txt"].Top(); got != 77 {
		t.Fatalf("committed top = %d, want 77", got)
	}
	if got := m.sources["a.txt"].Highlights(94); len(got) != 0 {
		t.Fatalf("Highlights(94) = %+v, want none — the fallback invents no highlight", got)
	}
	if got := m.sources["a.txt"].(interface{ Markers(int) []int }).Markers(94); len(got) != 0 {
		t.Fatalf("Markers(94) = %v, want none — the fallback invents no marker", got)
	}
	// The revealed stale row carries no inverse match paint — the
	// hidden-left gutter indicator legitimately paints inverse "_",
	// so the check is for inverse-painted text cells on the row
	// showing the stale line's window (cells 34–100: x's then 'b').
	if row := rowWith(t, m.View().Content, strings.Repeat("x", 40)+"b"); strings.Contains(row, "\x1b[30;47mx") || strings.Contains(row, "\x1b[30;47mb") {
		t.Fatalf("the stale line painted a match cell:\n%q", row)
	}
}

// A stale stop whose recorded line is gone lands at the last source
// line's start, inventing no highlight on it.
func TestStaleMissingLineLandsAtLastLine(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(30))
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-02\n", 2, 0, 4, "line"))
	addRec(t, idx, matchRec("a.txt", "line-50\n", 50, 0, 4, "line"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m.theme = theme.Plain()

	// The buffer is already stale on the first load — the recorded
	// line 50 does not exist.
	if row := viewRow(t, m, 0); !strings.Contains(row, "file changed since search") {
		t.Fatalf("the missing-line buffer's row lacks the note: %q", row)
	}

	// n selects the missing-line stop: the reveal lands on the last
	// source line's start — row 29, top clamped to the 30-line extent.
	m, _ = update(t, m, keyMsg("n"))
	if stop, _ := m.index.Current(); stop.Line != 50 {
		t.Fatalf("n selected line %d, want the missing-line stop 50", stop.Line)
	}
	if got := m.vps["a.txt"].Top(); got != 7 {
		t.Fatalf("top = %d, want 7 — the last line at the bottom", got)
	}
	if row := viewRow(t, m, 23); !strings.Contains(row, "30  line-30") {
		t.Fatalf("bottom content row = %q, want the last source line", row)
	}
	if got := m.sources["a.txt"].Highlights(29); len(got) != 0 {
		t.Fatalf("Highlights(29) = %+v, want none — no invented highlight", got)
	}
}

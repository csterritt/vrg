package app

import (
	"errors"
	"strings"
	"testing"

	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

// r while the current file's startup load is in flight is dropped
// whole — no command, no request bookkeeping change, no reload mark,
// and revision, pending reveal, and the rendered panel stay exactly as
// they were. The in-flight load then completes under its original
// classification: exactly one revision and the first-match reveal of
// story 50 — the top lands on the revealed target window — never the
// anchor-preserving install a reload would commit (Issue 42).
func TestDroppedReloadDuringStartupLoadKeepsReveal(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(100))
	writeMatchFile(t, dir, "b.txt", "hit b\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-95\n", 95, 0, 4, "line"))
	addRec(t, idx, matchRec("b.txt", "hit b\n", 1, 0, 3, "hit"))
	idx.Finish()

	m, gate, jobA := gatedBrowse(t, dir, idx, 80, 24)
	m.theme = theme.Plain()
	reqA := m.loading["a.txt"]
	before := m.View().Content

	m, c := update(t, m, keyMsg("r"))
	if c != nil {
		t.Fatalf("r during the startup load returned a command: %v", c)
	}
	if got := m.loading["a.txt"]; got != reqA || m.loadSeq != 1 {
		t.Fatalf("dropped r changed the load bookkeeping: req %d seq %d, want %d and 1",
			got, m.loadSeq, reqA)
	}
	if len(m.reloading) != 0 {
		t.Fatalf("dropped r marked the in-flight load a reload: %v", m.reloading)
	}
	if got := m.revs["a.txt"]; got != 0 {
		t.Fatalf("dropped r bumped the revision to %d, want 0", got)
	}
	if m.pendingIntent != intentReveal {
		t.Fatalf("dropped r disturbed the pending intent: %v, want the startup reveal", m.pendingIntent)
	}
	if got := m.View().Content; got != before {
		t.Fatalf("dropped r changed the presentation:\nbefore:\n%s\nafter:\n%s", before, got)
	}
	assertSilent(t, jobA, "a.txt's startup load")

	// The in-flight load completes as the startup load it is: one
	// revision, and the reveal commits — target row 94 lands EOF-clamped
	// at top 77 rather than holding the file's top 0.
	close(gate)
	m = applyLoad(t, m, collectMsg(t, jobA, "a.txt's startup load"))
	if got := m.revs["a.txt"]; got != 1 {
		t.Fatalf("startup-load revision = %d, want exactly 1", got)
	}
	if m.pendingIntent != intentNone {
		t.Fatalf("the committed reveal left intent %v pending", m.pendingIntent)
	}
	if got := m.vps["a.txt"].Top(); got != 77 {
		t.Fatalf("top after the startup completion = %d, want the reveal's 77, not the preserved 0", got)
	}
	if row := contentRow(t, m); !strings.Contains(row, "line-78") {
		t.Fatalf("panel top row = %q, want line-78", row)
	}
}

// r while a navigation load is in flight for the destination is
// likewise dropped whole: the pending destination reveal survives per
// the latest-target rules, and the load's completion commits it — the
// cursor's target row is revealed — rather than an anchor-preserving
// install holding the destination's saved top (Issue 42).
func TestDroppedReloadDuringNavigationLoadKeepsReveal(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(100))
	writeMatchFile(t, dir, "b.txt", numberedContent(100))
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-05\n", 5, 0, 4, "line"))
	addRec(t, idx, matchRec("b.txt", "line-95\n", 95, 0, 4, "line"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m.theme = theme.Plain()

	// n to b.txt: its load is minted and held behind the gate, and the
	// destination reveal is the pending intent.
	loadGate := make(chan struct{})
	m.loadGate = loadGate
	m, nav := update(t, m, keyMsg("n"))
	jobsB := navJobs(t, nav)
	reqB := m.loading["b.txt"]
	if reqB == 0 {
		t.Fatal("n onto b.txt started no load")
	}
	if m.pendingIntent != intentReveal {
		t.Fatalf("n left intent %v, want the pending destination reveal", m.pendingIntent)
	}
	// A no-op key dismisses the file-change pop-up first so the panel
	// comparison isolates the dropped r.
	m, _ = update(t, m, keyMsg("x"))
	before := m.View().Content

	m, c := update(t, m, keyMsg("r"))
	if c != nil {
		t.Fatalf("r during the navigation load returned a command: %v", c)
	}
	if got := m.loading["b.txt"]; got != reqB || m.loadSeq != 2 {
		t.Fatalf("dropped r changed the load bookkeeping: req %d seq %d, want %d and 2",
			got, m.loadSeq, reqB)
	}
	if len(m.reloading) != 0 {
		t.Fatalf("dropped r marked the in-flight load a reload: %v", m.reloading)
	}
	if m.pendingIntent != intentReveal {
		t.Fatalf("dropped r disturbed the pending intent: %v, want the destination reveal", m.pendingIntent)
	}
	if got := m.View().Content; got != before {
		t.Fatalf("dropped r changed the presentation:\nbefore:\n%s\nafter:\n%s", before, got)
	}

	// The held load completes under its original classification:
	// exactly one revision, and the destination reveal commits — target
	// row 94 lands EOF-clamped at top 77, never the saved top 0.
	close(loadGate)
	for _, j := range jobsB {
		msg := collectMsg(t, j, "b.txt navigation job")
		if _, isLoad := msg.(loadResult); isLoad {
			m = applyLoad(t, m, msg)
		}
	}
	if got := m.revs["b.txt"]; got != 1 {
		t.Fatalf("navigation-load revision = %d, want exactly 1", got)
	}
	if m.pendingIntent != intentNone {
		t.Fatalf("the committed reveal left intent %v pending", m.pendingIntent)
	}
	if got := m.vps["b.txt"].Top(); got != 77 {
		t.Fatalf("top after the navigation completion = %d, want the reveal's 77, not the preserved 0", got)
	}
	if row := contentRow(t, m); !strings.Contains(row, "line-78") {
		t.Fatalf("panel top row = %q, want line-78", row)
	}
}

// A dropped r commits no reload state at all — including the
// prior-failure presentation: on a failed file whose retry is already
// in flight, r neither re-shows the failure overlay nor disturbs the
// retry's bookkeeping (Issue 42).
func TestDroppedReloadDuringRetryKeepsOverlayClosed(t *testing.T) {
	dir := t.TempDir()
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("b.txt", "hit b\n", 1, 0, 3, "hit"))
	idx.Finish()

	loader := &stubLoader{
		fail: map[string]bool{"a.txt": true},
		err:  errors.New("denied"),
		src:  &stubSource{gutter: 3, widths: []int{8}},
	}
	m, gate, jobA := gatedLoaderBrowse(t, dir, idx, 80, 24, loader)
	close(gate)
	m, _ = update(t, m, collectMsg(t, jobA, "a.txt's startup load"))
	if m.overlay == nil {
		t.Fatal("a.txt's startup failure opened no overlay")
	}
	m, _ = update(t, m, keyPress("esc"))

	// r is the retry route: it re-shows the prior failure while the
	// retry runs behind the load gate.
	loadGate := make(chan struct{})
	m.loadGate = loadGate
	m, c := update(t, m, keyMsg("r"))
	if c == nil {
		t.Fatal("r on the failed file issued no retry")
	}
	jobR := runWorker(t, c)
	assertSilent(t, jobR, "a.txt's retry")
	if m.overlay == nil {
		t.Fatal("the accepted retry did not re-show the prior failure")
	}
	// The reader dismisses the overlay while the retry is still held;
	// the in-flight request is untouched.
	m, _ = update(t, m, keyPress("esc"))
	if m.overlay != nil {
		t.Fatal("esc did not dismiss the prior-failure overlay")
	}
	reqR := m.loading["a.txt"]
	seq := m.loadSeq
	before := m.View().Content

	// A second r while the retry is in flight is dropped whole — it
	// must not re-show the prior failure it never got to consult.
	m, c = update(t, m, keyMsg("r"))
	if c != nil {
		t.Fatalf("r during the retry returned a command: %v", c)
	}
	if m.overlay != nil {
		t.Fatalf("dropped r re-showed the prior failure: %v", m.overlay.lines)
	}
	if got := m.loading["a.txt"]; got != reqR || m.loadSeq != seq {
		t.Fatalf("dropped r changed the retry bookkeeping: req %d seq %d, want %d and %d",
			got, m.loadSeq, reqR, seq)
	}
	if !m.reloading["a.txt"] {
		t.Fatal("dropped r cleared the retry's reload mark")
	}
	if got := m.View().Content; got != before {
		t.Fatalf("dropped r changed the presentation:\nbefore:\n%s\nafter:\n%s", before, got)
	}

	// The held retry completes under its own classification — a second
	// failure lands as the current-file overlay.
	close(loadGate)
	m, _ = update(t, m, collectMsg(t, jobR, "a.txt's retry"))
	if m.overlay == nil || !strings.Contains(m.overlay.lines[len(m.overlay.lines)-1], "cannot read a.txt") {
		t.Fatalf("the settled retry did not report its failure: %v", m.overlay)
	}
}

// An r accepted after the previous load finished applies the reload
// contract at the one admission point — the reload mark, the dropped
// display's "Loading…" placeholder under the filename row — and the
// completion records the anchor-preserving intent with exactly one
// revision increment (Issue 42).
func TestAcceptedReloadMutatesOnce(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(100))
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-05\n", 5, 0, 4, "line"))
	addRec(t, idx, matchRec("a.txt", "line-95\n", 95, 0, 4, "line"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m.theme = theme.Plain()
	for i := 0; i < 3; i++ {
		m, _ = update(t, m, keyMsg("d")) // top 33 — the line-5 target is hidden above
	}
	rev := m.revs["a.txt"]
	seq := m.loadSeq

	loadGate := make(chan struct{})
	m.loadGate = loadGate
	layoutGate := make(chan struct{})
	m.layoutGate = layoutGate

	m, c := update(t, m, keyMsg("r"))
	if c == nil {
		t.Fatal("r on a loaded file issued no reload")
	}
	jobR := runWorker(t, c)
	assertSilent(t, jobR, "a.txt's reload")
	if !m.reloading["a.txt"] {
		t.Fatal("the accepted r did not mark the in-flight load a reload")
	}
	if m.buffers["a.txt"] != nil || m.sources["a.txt"] != nil {
		t.Fatal("the accepted r left the old display installed")
	}
	if got := m.loadSeq; got != seq+1 {
		t.Fatalf("the accepted r minted seq %d, want exactly %d", got, seq+1)
	}
	if v := m.View().Content; !strings.Contains(v, "── a.txt ") || !strings.Contains(v, "Loading…") {
		t.Fatalf("the accepted r lacks the filename row or placeholder:\n%s", v)
	}

	// The completion applies exactly one revision increment and records
	// the anchor-preserving intent.
	close(loadGate)
	m, layout := update(t, m, collectMsg(t, jobR, "a.txt's reload"))
	if got := m.revs["a.txt"]; got != rev+1 {
		t.Fatalf("reload revision = %d, want exactly %d", got, rev+1)
	}
	if m.pendingIntent != intentReloadAnchor {
		t.Fatalf("reload completion recorded intent %v, want the reload-anchor intent", m.pendingIntent)
	}
	jobL := runLayoutJob(t, layout)
	assertHeld(t, jobL, "the reloaded layout")

	// The install commits the anchor — the pre-reload top survives —
	// and the revision does not move again.
	close(layoutGate)
	m, _ = update(t, m, collectMsg(t, jobL, "the reloaded layout"))
	if got := m.revs["a.txt"]; got != rev+1 {
		t.Fatalf("the install changed the revision to %d, want %d", got, rev+1)
	}
	if m.pendingIntent != intentNone {
		t.Fatalf("the anchor commit left intent %v pending", m.pendingIntent)
	}
	if got := m.vps["a.txt"].Top(); got != 33 {
		t.Fatalf("top after the accepted reload = %d, want the preserved 33", got)
	}
}

// Rapid repeated r presses admit at most one reload per path: the first
// accepted request runs alone — each duplicate returns no command and
// mints nothing — and the placeholder's change to content, or to
// "(unreadable)" on failure, remains the completion signal (Issue 42).
func TestRepeatedReloadKeepsOneLoadInFlight(t *testing.T) {
	dir := t.TempDir()
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("b.txt", "hit b\n", 1, 0, 3, "hit"))
	idx.Finish()

	loader := &stubLoader{
		fail: map[string]bool{},
		err:  errors.New("denied"),
		src:  &stubSource{gutter: 3, widths: []int{8}},
	}
	m, gate, jobA := gatedLoaderBrowse(t, dir, idx, 80, 24, loader)
	close(gate)
	m = applyLoad(t, m, collectMsg(t, jobA, "a.txt's startup load"))

	// The first r is admitted; the next presses while it is held are
	// dropped — no command, no fresh request, one entry in flight.
	loadGate := make(chan struct{})
	m.loadGate = loadGate
	m, c := update(t, m, keyMsg("r"))
	if c == nil {
		t.Fatal("r on the loaded file issued no reload")
	}
	jobR := runWorker(t, c)
	reqR := m.loading["a.txt"]
	seq := m.loadSeq
	for i := 0; i < 3; i++ {
		m, c = update(t, m, keyMsg("r"))
		if c != nil {
			t.Fatalf("duplicate r %d returned a command: %v", i+1, c)
		}
	}
	if got := m.loading["a.txt"]; got != reqR || m.loadSeq != seq || len(m.loading) != 1 {
		t.Fatalf("duplicate r presses changed the in-flight request: req %d seq %d loads %v",
			got, m.loadSeq, m.loading)
	}
	if !m.reloading["a.txt"] {
		t.Fatal("duplicate r presses disturbed the reload mark")
	}
	assertSilent(t, jobR, "a.txt's reload")

	// The placeholder→content completion signal is preserved.
	close(loadGate)
	m = applyLoad(t, m, collectMsg(t, jobR, "a.txt's reload"))
	if got := loader.callsFor("a.txt"); got != 2 {
		t.Fatalf("a.txt loader calls = %d, want exactly 2", got)
	}
	if got := m.View().Content; !strings.Contains(got, "xxxxxxxx") || strings.Contains(got, "Loading…") {
		t.Fatalf("the settled reload did not repaint the content:\n%s", got)
	}

	// The placeholder→"(unreadable)" signal is likewise preserved.
	loader.setFail("a.txt", true)
	m, c = update(t, m, keyMsg("r"))
	if c == nil {
		t.Fatal("r after settlement issued no reload")
	}
	m, _ = update(t, m, c())
	if got := loader.callsFor("a.txt"); got != 3 {
		t.Fatalf("a.txt loader calls = %d, want 3", got)
	}
	m, _ = update(t, m, keyPress("esc"))
	if got := m.View().Content; !strings.Contains(got, "(unreadable)") {
		t.Fatalf("the failed reload lacks the unreadable placeholder:\n%s", got)
	}
}

// Navigation re-entry is not gated by load admission: onto a path whose
// load is already in flight the duplicate request is dropped, but the
// re-entry's own effects — the new selection, the placeholder
// presentation, and the pending destination reveal — still apply, and
// the held load's completion commits that reveal (Issue 42).
func TestReentryDuringInFlightLoadStillCommitsReveal(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(100))
	writeMatchFile(t, dir, "b.txt", "hit b\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-95\n", 95, 0, 4, "line"))
	addRec(t, idx, matchRec("b.txt", "hit b\n", 1, 0, 3, "hit"))
	idx.Finish()

	m, gate, jobA := gatedBrowse(t, dir, idx, 80, 24)
	reqA := m.loading["a.txt"]
	m.theme = theme.Plain()

	// n to b.txt — its load is held behind the same gate — then p
	// re-enters a.txt: the duplicate load is dropped while the
	// re-entry's selection, placeholder, and reveal intent stand.
	m, nav := update(t, m, keyMsg("n"))
	jobsB := navJobs(t, nav)
	m, nav = update(t, m, keyMsg("p"))
	for _, msg := range navMsgs(t, nav) {
		if _, isLoad := msg.(loadResult); isLoad {
			t.Fatal("re-entry onto the loading path started a second load")
		}
	}
	if got := m.loading["a.txt"]; got != reqA || m.loadSeq != 2 {
		t.Fatalf("re-entry changed the in-flight request: req %d seq %d, want %d and 2",
			got, m.loadSeq, reqA)
	}
	if stop, _ := m.index.Current(); string(stop.Path) != "a.txt" || stop.Line != 95 {
		t.Fatalf("re-entry selected %+v, want a.txt's line-95 stop", stop)
	}
	if m.pendingIntent != intentReveal {
		t.Fatalf("re-entry left intent %v, want the pending destination reveal", m.pendingIntent)
	}
	if got := m.View().Content; !strings.Contains(got, "── a.txt ") || !strings.Contains(got, "Loading…") {
		t.Fatalf("re-entry did not present a.txt's placeholder:\n%s", got)
	}

	// The held load completes under the re-entry's classification: the
	// destination reveal commits — top 77 — never a preserved top 0.
	close(gate)
	m = applyLoad(t, m, collectMsg(t, jobA, "a.txt's load"))
	for _, j := range jobsB {
		msg := collectMsg(t, j, "b.txt navigation job")
		if _, isLoad := msg.(loadResult); isLoad {
			m, _ = update(t, m, msg)
		}
	}
	if got := m.vps["a.txt"].Top(); got != 77 {
		t.Fatalf("top after the re-entry completion = %d, want the reveal's 77", got)
	}
	if row := contentRow(t, m); !strings.Contains(row, "line-78") {
		t.Fatalf("panel top row = %q, want line-78", row)
	}
}

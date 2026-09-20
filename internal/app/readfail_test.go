package app

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/clipperhouse/displaywidth"

	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
	"vrg/internal/theme"
	"vrg/internal/viewport"
)

// stubLoader is the injected file loader: it records each call's raw
// path and answers from the test's script — err for paths marked
// failing, src otherwise — so read failures come from injection rather
// than filesystem permissions. The mutex makes it safe on a load
// worker's goroutine.
type stubLoader struct {
	mu    sync.Mutex
	calls []string
	fail  map[string]bool
	err   error
	src   viewport.Source
}

func (l *stubLoader) load(resolved []byte, stops []searchindex.Stop) (viewport.Source, error) {
	path := string(resolved)
	if len(stops) > 0 {
		path = string(stops[0].Path)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls = append(l.calls, path)
	if l.fail[path] {
		return nil, l.err
	}
	return l.src, nil
}

// callsFor reports how many loads the model has started for path — the
// observable proof behind "exactly one retry".
func (l *stubLoader) callsFor(path string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, c := range l.calls {
		if c == path {
			n++
		}
	}
	return n
}

// setFail flips path's scripted outcome between phases — a retry that
// succeeds after earlier failures.
func (l *stubLoader) setFail(path string, on bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fail[path] = on
}

// browseWithLoader enters the browse state over idx at w×h with the
// loader seam driven by l and the instant pop-up timer installed,
// returning the model and the startup file's load command.
func browseWithLoader(t *testing.T, dir string, idx *searchindex.Index, w, h int, l *stubLoader) (Model, tea.Cmd) {
	t.Helper()
	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, dir)
	m.popupTimer = instantPopupTimer
	m.loader = l.load
	m.theme = theme.Plain()
	m, _ = update(t, m, tea.WindowSizeMsg{Width: w, Height: h})
	return update(t, m, searchResult{index: idx, integrity: completeStream})
}

// countDiag reports how many occurrences of line the session
// diagnostic collection — the replay's source — holds.
func countDiag(m Model, line string) int {
	n := 0
	for _, got := range m.diags.snapshot() {
		if got == line {
			n++
		}
	}
	return n
}

// The current file's read failure interrupts with the error overlay
// while the panel shows "(unreadable)": the file's cursor stops stay
// in the navigation index and the filename row still names the path.
func TestCurrentFileReadFailure(t *testing.T) {
	dir := t.TempDir()
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit one\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("a.txt", "hit two\n", 3, 0, 3, "hit"))
	idx.Finish()

	loader := &stubLoader{
		fail: map[string]bool{"a.txt": true},
		err:  errors.New("permission denied"),
		src:  &stubSource{gutter: 3, widths: []int{8}},
	}
	m, cmd := browseWithLoader(t, dir, idx, 80, 24, loader)
	m = applyLoad(t, m, cmd()) // the startup file's injected failure

	if m.overlay == nil {
		t.Fatal("the current file's read failure opened no error overlay")
	}
	if v := m.View().Content; !strings.Contains(v, "cannot read a.txt: permission denied") {
		t.Fatalf("overlay lacks the failure diagnostic:\n%s", v)
	}
	if _, bad := m.failed["a.txt"]; !bad {
		t.Fatal("a.txt's failure was not recorded")
	}

	// Dismissal leaves the "(unreadable)" placeholder under a filename
	// row still naming the path — and the retained stops navigable.
	m, _ = update(t, m, keyPress("esc"))
	v := m.View().Content
	for _, want := range []string{"── a.txt ", "(unreadable)"} {
		if !strings.Contains(v, want) {
			t.Fatalf("failed-file panel lacks %q:\n%s", want, v)
		}
	}
	m, c := update(t, m, keyMsg("n"))
	if c != nil {
		t.Fatalf("a same-file step on a failed file returned a command: %v", c)
	}
	stop, _ := m.index.Current()
	if stop.Line != 3 {
		t.Fatalf("n on a failed file selected %+v, want its second stop at line 3", stop)
	}
	if got := loader.callsFor("a.txt"); got != 1 {
		t.Fatalf("a same-file step retried the failed load: %d calls, want 1", got)
	}
	if got := m.View().Content; !strings.Contains(got, "(unreadable)") {
		t.Fatalf("the failed file lost its placeholder:\n%s", got)
	}
}

// n and p steps between stops of the same failed file never retry its
// load — including across the circular wrap in both directions, which a
// single-file index exercises without leaving the file. Entry from a
// different file requests exactly one retry; TestCrossFileEntryRetriesOnce
// covers that half.
func TestSameFileStepNeverRetries(t *testing.T) {
	dir := t.TempDir()
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit one\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("a.txt", "hit two\n", 3, 0, 3, "hit"))
	addRec(t, idx, matchRec("a.txt", "hit three\n", 5, 0, 3, "hit"))
	idx.Finish()

	loader := &stubLoader{
		fail: map[string]bool{"a.txt": true},
		err:  errors.New("denied"),
		src:  &stubSource{gutter: 3, widths: []int{8}},
	}
	m, cmd := browseWithLoader(t, dir, idx, 80, 24, loader)
	m = applyLoad(t, m, cmd()) // a.txt fails while current
	m, _ = update(t, m, keyPress("esc"))

	// Steps among a.txt's three stops — a.txt:5 wraps forward to
	// a.txt:1 and a.txt:1 wraps back to a.txt:5 — stay inside the
	// failed file and request nothing.
	for _, key := range []string{"n", "n", "n", "p", "p", "p", "p"} {
		var c tea.Cmd
		m, c = update(t, m, keyMsg(key))
		if c != nil {
			t.Fatalf("same-file step %q returned a command: %v", key, c)
		}
		if m.overlay != nil {
			t.Fatalf("same-file step %q reopened the overlay", key)
		}
	}
	if got := loader.callsFor("a.txt"); got != 1 {
		t.Fatalf("same-file steps retried the load: %d calls, want 1", got)
	}
	if got := m.View().Content; !strings.Contains(got, "(unreadable)") {
		t.Fatalf("the failed file lost its placeholder:\n%s", got)
	}
}

// A non-current file's read failure is collected as a diagnostic only —
// no overlay, no in-UI indicator, an untouched view — and is discovered
// later by visiting the file, which then shows the prior-failure
// overlay and the retry's "Loading…" placeholder.
func TestNonCurrentReadFailureDiagnosticOnly(t *testing.T) {
	dir := t.TempDir()
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("b.txt", "hit b\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("c.txt", "hit c\n", 1, 0, 3, "hit"))
	idx.Finish()

	loader := &stubLoader{
		fail: map[string]bool{"b.txt": true},
		err:  errors.New("denied"),
		src:  &stubSource{gutter: 3, widths: []int{8}},
	}
	m, cmd := browseWithLoader(t, dir, idx, 80, 24, loader)
	m = applyLoad(t, m, cmd()) // a.txt loads

	// b.txt's load is requested on entry; hold its result undelivered —
	// indistinguishable to the model from a worker still running — and
	// move on to c.txt so the failure lands while b.txt is not current.
	m, cmd = update(t, m, keyMsg("n"))
	msgB := navLoadMsg(t, cmd)
	m, cmd = update(t, m, keyMsg("n"))
	m = applyLoad(t, m, deliverNavLoad(t, cmd)) // c.txt loads
	before := m.View().Content

	m, _ = update(t, m, msgB)
	if m.overlay != nil {
		t.Fatal("a non-current failure opened the overlay")
	}
	if got := m.View().Content; got != before {
		t.Fatalf("a non-current failure changed the view:\nbefore:\n%s\nafter:\n%s", before, got)
	}
	want := "cannot read b.txt: denied"
	if got := countDiag(m, want); got != 1 {
		t.Fatalf("collected %d occurrences of %q, want 1", got, want)
	}

	// Discovery on visit: p onto b.txt shows the prior failure's
	// overlay immediately — before the retry it mints settles.
	m, cmd = update(t, m, keyMsg("p"))
	if m.overlay == nil {
		t.Fatal("visiting the failed file opened no overlay")
	}
	if len(m.overlay.lines) != 1 || m.overlay.lines[0] != want {
		t.Fatalf("visit overlay lines = %q, want the prior failure %q", m.overlay.lines, want)
	}
	if got := loader.callsFor("b.txt"); got != 1 {
		t.Fatalf("b.txt loader calls before the retry runs = %d, want 1", got)
	}
	loads := 0
	var retry tea.Msg
	for _, msg := range navMsgs(t, cmd) {
		if _, isLoad := msg.(loadResult); isLoad {
			loads++
			retry = msg
		}
	}
	if loads != 1 {
		t.Fatalf("the visit issued %d loads, want exactly 1", loads)
	}
	if got := loader.callsFor("b.txt"); got != 2 {
		t.Fatalf("b.txt loader calls = %d, want the one retry", got)
	}

	// The second failure appends exactly one occurrence to the open
	// overlay and collects exactly one more for replay; the settled
	// placeholder repaints under the still-open overlay.
	m, _ = update(t, m, retry)
	if len(m.overlay.lines) != 2 || m.overlay.lines[1] != want {
		t.Fatalf("second failure left overlay lines %q, want one appended occurrence", m.overlay.lines)
	}
	if got := countDiag(m, want); got != 2 {
		t.Fatalf("collected %d occurrences of %q, want 2", got, want)
	}
	if v := m.View().Content; !strings.Contains(v, "(unreadable)") {
		t.Fatalf("settled retry left no placeholder under the open overlay:\n%s", v)
	}
	m, _ = update(t, m, keyPress("esc"))
	if v := m.View().Content; !strings.Contains(v, "(unreadable)") {
		t.Fatalf("failed panel lacks the placeholder:\n%s", v)
	}
}

// Entry into a failed file from a different file requests exactly one
// retry load and presents the prior failure immediately: the overlay
// opens while the panel reads "Loading…" — not "(unreadable)" — for
// the retry's duration.
func TestCrossFileEntryRetriesOnce(t *testing.T) {
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
	m, cmd := browseWithLoader(t, dir, idx, 80, 24, loader)
	m = applyLoad(t, m, cmd()) // a.txt fails while current
	m, _ = update(t, m, keyPress("esc"))

	// Step onto b.txt, which loads, then p re-enters failed a.txt.
	m, cmd = update(t, m, keyMsg("n"))
	m = applyLoad(t, m, deliverNavLoad(t, cmd))
	m, cmd = update(t, m, keyMsg("p"))

	if m.overlay == nil {
		t.Fatal("re-entry into the failed file opened no prior-failure overlay")
	}
	if want := "cannot read a.txt: denied"; len(m.overlay.lines) != 1 || m.overlay.lines[0] != want {
		t.Fatalf("re-entry overlay lines = %q, want just %q", m.overlay.lines, want)
	}
	if _, ok := m.loading["a.txt"]; !ok {
		t.Fatal("re-entry minted no retry request")
	}
	v := m.View().Content
	if !strings.Contains(v, "Loading…") || strings.Contains(v, "(unreadable)") {
		t.Fatalf("re-entry placeholder is not %q:\n%s", "Loading…", v)
	}
	loads := 0
	for _, msg := range navMsgs(t, cmd) {
		if lr, isLoad := msg.(loadResult); isLoad {
			loads++
			if string(lr.path) != "a.txt" {
				t.Fatalf("the retry loaded %q, want a.txt", lr.path)
			}
		}
	}
	if loads != 1 {
		t.Fatalf("re-entry issued %d loads, want exactly 1", loads)
	}
	if got := loader.callsFor("a.txt"); got != 2 {
		t.Fatalf("a.txt loader calls = %d, want the original load plus one retry", got)
	}
}

// The unreadable file's composed frame stays well-formed at ordinary
// and constrained widths: the filename row keeps identifying the
// truncated safe path through Issue 24's slot rules, the panel shows
// the placeholder, no row overflows the terminal, and every layout
// dimension stays nonnegative.
func TestUnreadableComposedView(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("deep/", 10) + "end.txt" // 55 cells

	for _, size := range []struct{ w, h int }{{80, 24}, {30, 8}, {20, 3}} {
		idx := searchindex.New(dir)
		addRec(t, idx, matchRec(long, "hit\n", 1, 0, 3, "hit"))
		addRec(t, idx, matchRec("z.txt", "hit\n", 1, 0, 3, "hit"))
		idx.Finish()

		loader := &stubLoader{
			fail: map[string]bool{long: true},
			err:  errors.New("denied"),
			src:  &stubSource{gutter: 3, widths: []int{8}},
		}
		m, cmd := browseWithLoader(t, dir, idx, size.w, size.h, loader)
		m = applyLoad(t, m, cmd()) // the long-named file fails while current
		m, _ = update(t, m, keyPress("esc"))

		v := m.View().Content
		for i, r := range strings.Split(v, "\n") {
			if got := displaywidth.String(r); got > size.w {
				t.Fatalf("row %d at %dx%d is %d cells wide, overflowing:\n%q",
					i, size.w, size.h, got, r)
			}
		}
		if !strings.Contains(v, "(unreadable)") {
			t.Fatalf("%dx%d panel lacks the placeholder:\n%s", size.w, size.h, v)
		}
		if !strings.Contains(v, "…") || !strings.Contains(v, "end.txt") {
			t.Fatalf("%dx%d filename row lost the truncated safe path:\n%s", size.w, size.h, v)
		}
		if m.listWidth(size.w) < 0 || m.panelWidth() < 0 || m.contentHeight() < 0 {
			t.Fatalf("%dx%d produced a negative layout dimension", size.w, size.h)
		}
	}
}

// A non-current load failure's diagnostic reaches the session
// collection the replay writer emits: it lands on stderr through the
// real replay path even though no overlay ever showed it.
func TestNonCurrentFailureCollectedForReplay(t *testing.T) {
	dir := t.TempDir()
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("b.txt", "hit b\n", 1, 0, 3, "hit"))
	idx.Finish()

	loader := &stubLoader{
		fail: map[string]bool{"b.txt": true},
		err:  errors.New("denied"),
		src:  &stubSource{gutter: 3, widths: []int{8}},
	}
	m, cmd := browseWithLoader(t, dir, idx, 80, 24, loader)
	m = applyLoad(t, m, cmd()) // a.txt loads

	// b.txt's load is minted on entry; its result is held undelivered
	// while the cursor returns to a.txt, so the failure lands
	// non-current.
	m, cmd = update(t, m, keyMsg("n"))
	msgB := navLoadMsg(t, cmd)
	m, _ = update(t, m, keyMsg("p"))
	m, _ = update(t, m, msgB)
	if m.overlay != nil {
		t.Fatal("a non-current failure opened the overlay")
	}

	var replay strings.Builder
	m.diags.replay(&replay)
	if got := replay.String(); got != "cannot read b.txt: denied\n" {
		t.Fatalf("replay = %q, want the non-current failure diagnostic", got)
	}
}

// gatedLoaderBrowse is gatedBrowse with the loader seam driven by l and
// the no-style theme: every load the model starts blocks inside its
// worker until the gate closes or the model cancels.
func gatedLoaderBrowse(t *testing.T, dir string, idx *searchindex.Index, w, h int, l *stubLoader) (Model, chan struct{}, <-chan tea.Msg) {
	t.Helper()
	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, dir)
	m.popupTimer = instantPopupTimer
	m.theme = theme.Plain()
	m.loader = l.load
	gate := make(chan struct{})
	m.loadGate = gate
	m, _ = update(t, m, tea.WindowSizeMsg{Width: w, Height: h})
	m, cmd := update(t, m, searchResult{index: idx, integrity: completeStream})
	if cmd == nil {
		t.Fatal("browse entry started no file load")
	}
	return m, gate, runWorker(t, cmd)
}

// reentrySetup drives the shared prefix of the gated re-entry tests:
// a.txt's held startup load fails while current, its overlay is
// dismissed, b.txt is visited and loads ungated, and a fresh gate is
// installed so the p re-entry's retry stays held. It returns the model
// ready for the re-entry key press and that fresh gate.
func reentrySetup(t *testing.T, w, h int, l *stubLoader) (Model, chan struct{}) {
	t.Helper()
	dir := t.TempDir()
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("b.txt", "hit b\n", 1, 0, 3, "hit"))
	idx.Finish()

	m, gate, jobA := gatedLoaderBrowse(t, dir, idx, w, h, l)
	assertSilent(t, jobA, "a.txt's startup load")
	close(gate)
	m, _ = update(t, m, collectMsg(t, jobA, "a.txt's startup load"))
	if m.overlay == nil {
		t.Fatal("a.txt's startup failure opened no overlay")
	}
	m, _ = update(t, m, keyPress("esc"))

	m.loadGate = nil // b.txt's load runs to completion synchronously
	m, cmd := update(t, m, keyMsg("n"))
	m = applyLoad(t, m, deliverNavLoad(t, cmd))
	if m.buffers["b.txt"] == nil {
		t.Fatal("b.txt did not load")
	}

	gate = make(chan struct{})
	m.loadGate = gate
	return m, gate
}

// The re-entry sequence is deterministic: p onto the failed file shows
// the prior-failure overlay immediately while the panel switches from
// "(unreadable)" to "Loading…", exactly one retry starts behind the
// gate, Esc dismisses the overlay without disturbing the in-flight
// load, and the load's settlement — not the dismissal — drives the
// placeholder. A successful retry collects nothing new and fills the
// panel with the file's content.
func TestReentryRetrySequenceGated(t *testing.T) {
	loader := &stubLoader{
		fail: map[string]bool{"a.txt": true},
		err:  errors.New("denied"),
		src:  &stubSource{gutter: 3, widths: []int{8}},
	}
	m, gate := reentrySetup(t, 80, 24, loader)
	want := "cannot read a.txt: denied"

	m, cmd := update(t, m, keyMsg("p"))
	if m.overlay == nil {
		t.Fatal("re-entry opened no prior-failure overlay")
	}
	if len(m.overlay.lines) != 1 || m.overlay.lines[0] != want {
		t.Fatalf("re-entry overlay lines = %q, want just the prior failure %q", m.overlay.lines, want)
	}
	if _, ok := m.loading["a.txt"]; !ok {
		t.Fatal("re-entry minted no retry request")
	}
	if v := m.View().Content; !strings.Contains(v, "Loading…") || strings.Contains(v, "(unreadable)") {
		t.Fatalf("re-entry placeholder is not %q:\n%s", "Loading…", v)
	}

	// Exactly one retry — the returned command is the load itself —
	// held behind the gate while the overlay stays open.
	retry := runWorker(t, cmd)
	assertSilent(t, retry, "a.txt's re-entry retry")
	if m.overlay == nil {
		t.Fatal("starting the retry closed the overlay")
	}

	// Esc dismisses the overlay; the in-flight load is undisturbed.
	m, c := update(t, m, keyPress("esc"))
	if c != nil {
		t.Fatalf("Esc during a retry returned a command: %v", c)
	}
	if m.overlay != nil {
		t.Fatal("Esc did not dismiss the prior-failure overlay")
	}
	assertSilent(t, retry, "a.txt's retry after Esc")

	// Settlement drives the placeholder: the retry's success installs
	// the content without waiting on any dismissal, collecting nothing.
	loader.setFail("a.txt", false)
	close(gate)
	m = applyLoad(t, m, collectMsg(t, retry, "a.txt's retry"))
	if got := countDiag(m, want); got != 1 {
		t.Fatalf("a successful retry collected diagnostics: %d occurrences of %q, want 1", got, want)
	}
	if m.buffers["a.txt"] == nil {
		t.Fatal("the settled retry installed no content")
	}
	if v := m.View().Content; !strings.Contains(v, "xxxxxxxx") ||
		strings.Contains(v, "(unreadable)") || strings.Contains(v, "Loading…") {
		t.Fatalf("a settled successful retry left a placeholder:\n%s", v)
	}
	if got := loader.callsFor("a.txt"); got != 2 {
		t.Fatalf("a.txt loader calls = %d, want the original load plus one retry", got)
	}
}

// A retry that settles while the prior-failure overlay is still open
// updates the panel without waiting for dismissal: content replaces
// "Loading…" under the open overlay, nothing new is collected, and the
// overlay stays up until the reader dismisses it.
func TestReentryRetrySuccessKeepsOverlay(t *testing.T) {
	loader := &stubLoader{
		fail: map[string]bool{"a.txt": true},
		err:  errors.New("denied"),
		src:  &stubSource{gutter: 3, widths: []int{8}},
	}
	m, gate := reentrySetup(t, 80, 24, loader)
	want := "cannot read a.txt: denied"

	m, cmd := update(t, m, keyMsg("p"))
	if m.overlay == nil {
		t.Fatal("re-entry opened no prior-failure overlay")
	}
	retry := runWorker(t, cmd)
	assertSilent(t, retry, "a.txt's re-entry retry")

	loader.setFail("a.txt", false)
	close(gate)
	m = applyLoad(t, m, collectMsg(t, retry, "a.txt's retry"))

	if m.overlay == nil {
		t.Fatal("the retry's settlement dismissed the prior-failure overlay")
	}
	if len(m.overlay.lines) != 1 || m.overlay.lines[0] != want {
		t.Fatalf("a successful retry touched the overlay lines: %q", m.overlay.lines)
	}
	if got := countDiag(m, want); got != 1 {
		t.Fatalf("a successful retry collected diagnostics: %d occurrences, want 1", got)
	}
	// The panel repaints under the open overlay: the box covers only
	// the centre rows, so the content row shows through.
	if v := m.View().Content; !strings.Contains(v, "xxxxxxxx") || strings.Contains(v, "(unreadable)") {
		t.Fatalf("the settled panel lacks the content under the open overlay:\n%s", v)
	}
	m, _ = update(t, m, keyPress("esc"))
	if m.overlay != nil {
		t.Fatal("the prior-failure overlay stayed after Esc")
	}
}

// A second failure while the re-entry overlay is open appends exactly
// one new diagnostic occurrence and leaves the reader's scroll position
// alone: scroll names the first shown wrapped row, and the append
// extends only the tail.
func TestReentrySecondFailureAppendsPreservingScroll(t *testing.T) {
	longErr := errors.New("denied: " + strings.Repeat("x", 200))
	loader := &stubLoader{
		fail: map[string]bool{"a.txt": true},
		err:  longErr,
		src:  &stubSource{gutter: 3, widths: []int{8}},
	}
	m, gate := reentrySetup(t, 40, 6, loader)
	want := "cannot read a.txt: " + longErr.Error()

	m, cmd := update(t, m, keyMsg("p"))
	if m.overlay == nil {
		t.Fatal("re-entry opened no prior-failure overlay")
	}
	if max := m.overlay.maxScroll(m.width, m.height); max == 0 {
		t.Fatalf("the prior-failure overlay is not scrollable at 40x6 (maxScroll %d)", max)
	}
	retry := runWorker(t, cmd)
	assertSilent(t, retry, "a.txt's re-entry retry")

	// The reader scrolls down inside the open overlay while the retry
	// is still in flight.
	m, _ = update(t, m, keyPress("down"))
	if m.overlay.scroll != 1 {
		t.Fatalf("overlay scroll = %d, want 1", m.overlay.scroll)
	}

	// The second failure appends exactly one occurrence — collected
	// once more for replay — and the reader's position holds.
	close(gate)
	m, _ = update(t, m, collectMsg(t, retry, "a.txt's retry"))
	if len(m.overlay.lines) != 2 || m.overlay.lines[1] != want {
		t.Fatalf("second failure left overlay lines %q, want one appended occurrence", m.overlay.lines)
	}
	if m.overlay.scroll != 1 {
		t.Fatalf("the appended occurrence moved the reader: scroll = %d, want 1", m.overlay.scroll)
	}
	if got := countDiag(m, want); got != 2 {
		t.Fatalf("collected %d occurrences of the failure, want 2", got)
	}
}

// Navigating away while a retry is in flight leaves the completion to
// update only that path's cache and status: a retry failure landing
// non-current is a diagnostic only, and a later re-entry follows the
// same sequence against the new prior state. Re-entry while the retry
// is still in flight reuses it per the one-load-per-path rule.
func TestReentryAwayDuringRetry(t *testing.T) {
	loader := &stubLoader{
		fail: map[string]bool{"a.txt": true},
		err:  errors.New("denied"),
		src:  &stubSource{gutter: 3, widths: []int{8}},
	}
	m, gate := reentrySetup(t, 80, 24, loader)
	want := "cannot read a.txt: denied"

	m, cmd := update(t, m, keyMsg("p"))
	if m.overlay == nil {
		t.Fatal("re-entry opened no prior-failure overlay")
	}
	retry := runWorker(t, cmd)
	assertSilent(t, retry, "a.txt's re-entry retry")

	// Dismiss, leave for b.txt, then come back while the retry is
	// still held: the prior-failure overlay shows again and the
	// in-flight load is reused — no second request.
	m, _ = update(t, m, keyPress("esc"))
	m, _ = update(t, m, keyMsg("n"))
	m, c := update(t, m, keyMsg("p"))
	if m.overlay == nil {
		t.Fatal("re-entry during the in-flight retry opened no overlay")
	}
	if c != nil {
		t.Fatalf("re-entry during an in-flight retry returned a command: %v", c)
	}
	// The gate holds the in-flight retry ahead of the loader, so the
	// one recorded call is still the startup load; a second request
	// would mint a second worker and a second call on release.
	if got := loader.callsFor("a.txt"); got != 1 {
		t.Fatalf("re-entry during an in-flight retry reached the loader: %d calls", got)
	}
	assertSilent(t, retry, "a.txt's still-in-flight retry")

	// Leave again and let the retry settle while a.txt is non-current:
	// the failure updates only the path's cache and diagnostics.
	m, _ = update(t, m, keyPress("esc"))
	m, _ = update(t, m, keyMsg("n"))
	before := m.View().Content
	close(gate)
	m, _ = update(t, m, collectMsg(t, retry, "a.txt's retry"))
	if m.overlay != nil {
		t.Fatal("a retry failure settling non-current opened the overlay")
	}
	if got := m.View().Content; got != before {
		t.Fatalf("a non-current retry settlement changed the view:\nbefore:\n%s\nafter:\n%s", before, got)
	}
	if got := countDiag(m, want); got != 2 {
		t.Fatalf("collected %d occurrences of %q, want 2", got, want)
	}
	if _, bad := m.failed["a.txt"]; !bad || m.sources["a.txt"] != nil {
		t.Fatal("the retry settlement did not update a.txt's status")
	}

	// A later re-entry follows the same sequence against the new prior
	// state: the overlay shows the latest failure and exactly one new
	// retry starts.
	m.loadGate = nil
	m, cmd = update(t, m, keyMsg("p"))
	if m.overlay == nil || len(m.overlay.lines) != 1 || m.overlay.lines[0] != want {
		t.Fatalf("later re-entry overlay = %v, want the new prior failure", m.overlay)
	}
	loads := 0
	var result tea.Msg
	for _, msg := range navMsgs(t, cmd) {
		if _, isLoad := msg.(loadResult); isLoad {
			loads++
			result = msg
		}
	}
	if loads != 1 {
		t.Fatalf("later re-entry issued %d loads, want exactly 1", loads)
	}
	m, _ = update(t, m, result)
	if len(m.overlay.lines) != 2 {
		t.Fatalf("the new failure did not append to the re-entry overlay: %q", m.overlay.lines)
	}
	if got := countDiag(m, want); got != 3 {
		t.Fatalf("collected %d occurrences of %q, want 3", got, want)
	}
	m, _ = update(t, m, keyPress("esc"))
	if v := m.View().Content; !strings.Contains(v, "(unreadable)") {
		t.Fatalf("the twice-failed file lacks the placeholder:\n%s", v)
	}
}

// hostileNames are the Issue 47 read-failure adversaries: an embedded
// newline would split an unsanitized diagnostic into several lines,
// while tab, invalid UTF-8, and ESC bytes must each reach the
// diagnostic only in EscapePath form. Every name sorts before the
// companion file "zzz.txt", so the hostile file is the startup
// selection of each fixture.
var hostileNames = [][]byte{
	[]byte("two\nlines.txt"),
	[]byte("tab\tname.txt"),
	{'b', 'a', 'd', 0xff, '.', 't', 'x', 't'},
	[]byte("esc\x1bname.txt"),
}

// gatedHostileFixture indexes a real file under a hostile name inside a
// disposable directory — plus a plain companion file so cross-file
// navigation exists — and enters browse with the production loader
// held at the load gate. It returns the model, the fixture directory,
// the gate holding the hostile file's startup load, and that load's
// worker channel.
func gatedHostileFixture(t *testing.T, name []byte) (Model, string, chan struct{}, <-chan tea.Msg) {
	t.Helper()
	dir := t.TempDir()
	writeMatchFile(t, dir, string(name), "hit\n")
	writeMatchFile(t, dir, "zzz.txt", "hit z\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRecBytes(name, []byte("hit\n"), 1, 0, 3, []byte("hit")))
	addRec(t, idx, matchRec("zzz.txt", "hit z\n", 1, 0, 3, "hit"))
	idx.Finish()
	m, gate, job := gatedBrowse(t, dir, idx, 80, 24)
	if stop, _ := m.index.Current(); string(stop.Path) != string(name) {
		t.Fatalf("the startup selection is %q, want the hostile file %q", stop.Path, name)
	}
	return m, dir, gate, job
}

// failHeldLoad removes the fixture while its read is held at the gate,
// then releases the real os.ReadFile attempt: the settled result is a
// genuine not-exist *fs.PathError — a deterministic real failure, not
// a permission denial that elevated privileges or unusual ACLs could
// bypass.
func failHeldLoad(t *testing.T, dir string, name []byte, gate chan struct{}, job <-chan tea.Msg) loadResult {
	t.Helper()
	assertSilent(t, job, "the gated read of "+safepresentation.EscapePath(name))
	if err := os.Remove(filepath.Join(dir, string(name))); err != nil {
		t.Fatalf("removing the fixture: %v", err)
	}
	close(gate)
	return collectReadFailure(t, job, name, "the released read")
}

// collectReadFailure reads a settled load result and asserts it is the
// expected genuine failure: a not-exist *fs.PathError answering for
// the hostile path.
func collectReadFailure(t *testing.T, job <-chan tea.Msg, name []byte, what string) loadResult {
	t.Helper()
	msg := collectMsg(t, job, what)
	lr, ok := msg.(loadResult)
	if !ok {
		t.Fatalf("%s produced %T, want loadResult", what, msg)
	}
	if string(lr.path) != string(name) {
		t.Fatalf("%s answered for %q, want %q", what, lr.path, name)
	}
	var pe *fs.PathError
	if !errors.As(lr.err, &pe) || !errors.Is(lr.err, fs.ErrNotExist) {
		t.Fatalf("%s read error = %v (%T), want a not-exist *fs.PathError", what, lr.err, lr.err)
	}
	return lr
}

// wantFailureLine is the single-line diagnostic the Issue 47 contract
// expects for a genuine read failure: "cannot read " plus the raw
// path's EscapePath form plus the path error's own reason — the Err
// the *fs.PathError wraps — never the wrapper's message, which would
// embed the resolved path a second time.
func wantFailureLine(t *testing.T, lr loadResult) string {
	t.Helper()
	var pe *fs.PathError
	if !errors.As(lr.err, &pe) {
		t.Fatalf("load error = %v (%T), want *fs.PathError", lr.err, lr.err)
	}
	line := "cannot read " + safepresentation.EscapePath(lr.path) +
		": " + safepresentation.EscapePath([]byte(pe.Err.Error()))
	if strings.Contains(line, "\n") {
		t.Fatalf("the expected diagnostic is not a single line: %q", line)
	}
	if strings.Count(line, safepresentation.EscapePath(lr.path)) != 1 {
		t.Fatalf("the escaped path repeats inside %q", line)
	}
	if strings.Contains(line, pe.Path) {
		t.Fatalf("the raw resolved path %q leaks into %q", pe.Path, line)
	}
	return line
}

// assertReadFailureLine asserts the settled failure produced the
// single-line diagnostic in every sink: the recorded prior failure and
// every overlay row are exactly want, the session collection holds it
// occurrences times in total, and the stderr replay writes those lines
// verbatim — one line per failure, with no re-splitting.
func assertReadFailureLine(t *testing.T, m Model, key, want string, occurrences int) {
	t.Helper()
	if got := m.failed[key]; got != want {
		t.Fatalf("recorded failure = %q, want %q", got, want)
	}
	if m.overlay == nil || len(m.overlay.lines) != occurrences {
		t.Fatalf("overlay lines = %q, want %d occurrences of %q", m.overlay.lines, occurrences, want)
	}
	for i, line := range m.overlay.lines {
		if line != want {
			t.Fatalf("overlay line %d = %q, want %q", i, line, want)
		}
	}
	if got := countDiag(m, want); got != occurrences {
		t.Fatalf("collected %d occurrences of %q, want %d", got, want, occurrences)
	}
	var replay strings.Builder
	m.diags.replay(&replay)
	if got, all := replay.String(), strings.Repeat(want+"\n", occurrences); got != all {
		t.Fatalf("stderr replay = %q, want %q", got, all)
	}
}

// A genuine read failure under a hostile filename — newline, tab,
// invalid UTF-8, or ESC — produces exactly one diagnostic line: the
// EscapePath-escaped raw path plus a sanitized reason that does not
// repeat the raw path the way PathError.Error() does. The same line
// is the overlay's row set and the collection's stderr replay, and the
// removed fixture leaves nothing behind outside the temporary
// directory.
func TestHostileNameReadFailureSingleLine(t *testing.T) {
	for _, name := range hostileNames {
		t.Run(safepresentation.EscapePath(name), func(t *testing.T) {
			m, dir, gate, job := gatedHostileFixture(t, name)
			lr := failHeldLoad(t, dir, name, gate, job)
			want := wantFailureLine(t, lr)
			m = applyLoad(t, m, lr)
			assertReadFailureLine(t, m, string(name), want, 1)
		})
	}
}

// The r retry of a failed hostile-named file uses the same single-line
// construction: r re-shows the prior failure while the gated retry
// runs, and the second genuine failure appends exactly one occurrence
// — two in total through the overlay and the replay.
func TestHostileNameReloadRetrySingleLine(t *testing.T) {
	for _, name := range hostileNames {
		t.Run(safepresentation.EscapePath(name), func(t *testing.T) {
			m, dir, gate, job := gatedHostileFixture(t, name)
			lr := failHeldLoad(t, dir, name, gate, job)
			want := wantFailureLine(t, lr)
			m = applyLoad(t, m, lr)
			m, _ = update(t, m, keyPress("esc"))

			gateR := make(chan struct{})
			m.loadGate = gateR
			m, c := update(t, m, keyMsg("r"))
			if c == nil {
				t.Fatal("r on the failed file issued no retry")
			}
			if m.overlay == nil || len(m.overlay.lines) != 1 || m.overlay.lines[0] != want {
				t.Fatalf("r did not re-show the prior failure: %v", m.overlay)
			}
			retry := runWorker(t, c)
			assertSilent(t, retry, "the gated reload retry")

			close(gateR)
			lr = collectReadFailure(t, retry, name, "the reload retry")
			if got := wantFailureLine(t, lr); got != want {
				t.Fatalf("the retry's diagnostic = %q, want %q", got, want)
			}
			m = applyLoad(t, m, lr)
			assertReadFailureLine(t, m, string(name), want, 2)
		})
	}
}

// Re-entering the failed hostile-named file from another file uses the
// same single-line construction: the prior failure shows immediately
// while the gated retry runs, and the retry's genuine failure appends
// exactly one occurrence.
func TestHostileNameReentryRetrySingleLine(t *testing.T) {
	for _, name := range hostileNames {
		t.Run(safepresentation.EscapePath(name), func(t *testing.T) {
			m, dir, gate, job := gatedHostileFixture(t, name)
			lr := failHeldLoad(t, dir, name, gate, job)
			want := wantFailureLine(t, lr)
			m = applyLoad(t, m, lr)
			m, _ = update(t, m, keyPress("esc"))

			// Visit the companion file — a real load, ungated — then
			// re-enter the failed file behind a fresh gate.
			m.loadGate = nil
			m, cmd := update(t, m, keyMsg("n"))
			m = applyLoad(t, m, deliverNavLoad(t, cmd))
			if m.buffers["zzz.txt"] == nil {
				t.Fatal("the companion file did not load")
			}

			gateE := make(chan struct{})
			m.loadGate = gateE
			m, cmd = update(t, m, keyMsg("p"))
			if m.overlay == nil || len(m.overlay.lines) != 1 || m.overlay.lines[0] != want {
				t.Fatalf("re-entry did not show the prior failure: %v", m.overlay)
			}
			if _, ok := m.loading[string(name)]; !ok {
				t.Fatal("re-entry minted no retry")
			}
			retry := runWorker(t, cmd)
			assertSilent(t, retry, "the re-entry retry")

			close(gateE)
			lr = collectReadFailure(t, retry, name, "the re-entry retry")
			if got := wantFailureLine(t, lr); got != want {
				t.Fatalf("the re-entry retry's diagnostic = %q, want %q", got, want)
			}
			m = applyLoad(t, m, lr)
			assertReadFailureLine(t, m, string(name), want, 2)
		})
	}
}

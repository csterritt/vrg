package app

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

// errUnreadable is the injected loader's deterministic read failure —
// the model-test substitute for chmod 000, which a filesystem-level
// test could not express portably (and which does not deny root).
var errUnreadable = errors.New("permission denied")

// failAllLoader is the injected loader failing every read — the seam
// the Issue #26 tests use instead of filesystem permissions.
func failAllLoader([]byte) ([]byte, error) { return nil, errUnreadable }

// failPathsLoader returns an injected loader failing exactly the
// listed raw paths; every other path reads through to the real
// filebuffer.Read.
func failPathsLoader(fail ...string) func([]byte) ([]byte, error) {
	bad := map[string]bool{}
	for _, p := range fail {
		bad[p] = true
	}
	return func(p []byte) ([]byte, error) {
		if bad[string(p)] {
			return nil, errUnreadable
		}
		return filebuffer.Read(p)
	}
}

// gatedFailLoader is the injected loader for the re-entry tests: the
// fail set is mutex-guarded so the test can flip a path's policy while
// a gated worker waits and have that worker's read observe the armed
// outcome — success or another failure.
type gatedFailLoader struct {
	mu   sync.Mutex
	fail map[string]error // non-nil entry: reads fail with this error
}

func newGatedFailLoader() *gatedFailLoader {
	return &gatedFailLoader{fail: map[string]error{}}
}

func (l *gatedFailLoader) read(p []byte) ([]byte, error) {
	l.mu.Lock()
	err := l.fail[string(p)]
	l.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return filebuffer.Read(p)
}

// set arms path's read outcome: a non-nil err fails reads, nil reads
// through to the real file.
func (l *gatedFailLoader) set(path string, err error) {
	l.mu.Lock()
	l.fail[path] = err
	l.mu.Unlock()
}

// heldNthLoad holds only the nth load worker to reach the gate —
// earlier loads run straight through. The re-entry fixture's load
// order is deterministic, so the retry is a known ordinal.
type heldNthLoad struct {
	calls   atomic.Int32
	n       int32
	entered chan struct{}
	release chan struct{}
}

func newHeldNthLoad(n int32) *heldNthLoad {
	return &heldNthLoad{n: n, entered: make(chan struct{}), release: make(chan struct{})}
}

func (h *heldNthLoad) fn() func() {
	return func() {
		if h.calls.Add(1) == h.n {
			close(h.entered)
			<-h.release
		}
	}
}

// A read failure for the current file shows the error overlay carrying
// the load diagnostic and the "(unreadable)" placeholder; the filename
// row still names the path and the file's cursor stops stay navigable.
func TestCurrentFileFailureShowsOverlay(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{loader: failAllLoader})
	idx := navIndex(t, navFiles)
	keyA := string(idx.Files[0].Path)

	msg, ok := fileLoadOf(startBrowse(t, m, idx))
	if !ok {
		t.Fatal("the startup load produced no completion")
	}
	m.Update(msg)

	if !m.failed[keyA] {
		t.Fatal("the failed load left no failure record")
	}
	if !m.overlayOpen {
		t.Fatal("a current-file read failure did not open the error overlay")
	}
	want := loadDiag(idx.Files[0].Path, errUnreadable)
	if !strings.Contains(m.overlay.text, want) {
		t.Fatalf("overlay = %q, want the load diagnostic %q", m.overlay.text, want)
	}
	v := viewText(m)
	if !strings.Contains(v, "(unreadable)") {
		t.Fatalf("view = %q, want the (unreadable) placeholder", v)
	}
	if !strings.Contains(v, "─ "+escapedPath(idx.Files[0])+" ") {
		t.Fatalf("view = %q, want the filename row still naming the path", v)
	}
	assertReplayLines(t, m.diags, []string{want})

	// The failed file's stops remain navigable: n moves the cursor to
	// a.txt's second stop without minting a load or a new overlay.
	m.Update(keyEsc)
	if _, cmd := m.Update(keyN); cmd != nil {
		t.Fatal("a same-file n on the failed file returned a command")
	}
	if cur, _ := m.idx.Cursor(); cur.File != 0 || cur.Stop != 1 {
		t.Fatalf("n on the failed file selected %+v, want its retained stop 1", cur)
	}
	if _, ok := m.loading[keyA]; ok {
		t.Fatal("a same-file step minted a reload request")
	}
	if m.overlayOpen {
		t.Fatal("a same-file step on the failed file opened a new overlay")
	}
}

// A read failure for a non-current file is collected as a diagnostic
// only: no overlay, no in-UI indicator — the frame is unchanged. The
// failure surfaces when the user visits that file or in the replay.
func TestNonCurrentFailureIsDiagnosticOnly(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, isoFiles)
	finishLoad(t, m, startBrowse(t, m, idx))
	before := viewText(m)

	m.Update(fileLoadedMsg{
		path: idx.Files[1].Path,
		req:  mintRequest(m, idx.Files[1].Path),
		err:  errUnreadable,
	})
	if !m.failed[string(idx.Files[1].Path)] {
		t.Fatal("the non-current failure left no failure record for a later visit")
	}
	if m.overlayOpen {
		t.Fatal("a non-current read failure opened the overlay")
	}
	if got := viewText(m); got != before {
		t.Fatalf("a non-current failure changed the frame:\nbefore %q\nafter  %q", before, got)
	}
	want := loadDiag(idx.Files[1].Path, errUnreadable)
	assertReplayLines(t, m.diags, []string{want})
}

// The non-current failure's diagnostic reaches the Issue #11 replay
// collection even though nothing in the UI ever displayed it.
func TestNonCurrentFailureAppearsInReplay(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, isoFiles)
	finishLoad(t, m, startBrowse(t, m, idx))

	m.Update(fileLoadedMsg{
		path: idx.Files[1].Path,
		req:  mintRequest(m, idx.Files[1].Path),
		err:  errUnreadable,
	})
	want := loadDiag(idx.Files[1].Path, errUnreadable)
	if out := replayOutput(t, m); !strings.Contains(out, want) {
		t.Fatalf("replay = %q, want the non-current failure %q", out, want)
	}
}

// Entering a previously failed file from a different file shows the
// prior failure immediately — the overlay opens with its diagnostic
// while the panel switches to "Loading…" — and mints exactly one
// retry load while the overlay is up. A same-file step never retries.
func TestCrossFileEntryRetriesFailedFile(t *testing.T) {
	idx := navIndex(t, isoFiles)
	keyB := string(idx.Files[1].Path)
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{
		loader:     failPathsLoader(keyB),
		popupTimer: popupStubTicks.popupTimer,
	})
	finishLoad(t, m, startBrowse(t, m, idx))

	// b fails on first entry under the current-file contract.
	msg, ok := fileLoadOf(navCmd(t, m, keyN))
	if !ok {
		t.Fatal("crossing into b produced no load completion")
	}
	m.Update(msg)
	m.Update(keyEsc)
	finishLoad(t, m, navCmd(t, m, keyN)) // c

	// p re-enters b from a different file: the prior failure's overlay
	// is up, the panel reads "Loading…", and exactly one retry is
	// minted — the navigation batch's load leaf.
	_, nav := m.Update(keyP)
	if _, ok := m.loading[keyB]; !ok {
		t.Fatal("re-entering a failed file minted no retry load")
	}
	if !m.overlayOpen {
		t.Fatal("re-entering a failed file did not show the prior failure")
	}
	want := loadDiag(idx.Files[1].Path, errUnreadable)
	if n := strings.Count(m.overlay.text, "cannot read"); n != 1 || !strings.Contains(m.overlay.text, want) {
		t.Fatalf("overlay = %q, want exactly the prior failure %q", m.overlay.text, want)
	}
	if v := viewText(m); !strings.Contains(v, "Loading…") {
		t.Fatalf("view = %q, want Loading… for the minted retry", v)
	}

	// The minted retry is the batch's load leaf; its second failure
	// lands as one more collected occurrence, appended to the still
	// open overlay.
	msg, ok = fileLoadOf(nav)
	if !ok {
		t.Fatal("the re-entry batch carried no retry leaf")
	}
	m.Update(msg)
	assertReplayLines(t, m.diags, []string{want, want})
	if n := strings.Count(m.overlay.text, "cannot read"); n != 2 {
		t.Fatalf("overlay = %q, want the second failure appended once", m.overlay.text)
	}
}

// The unreadable state composes safely at ordinary and constrained
// widths: the filename row keeps the left-truncated safe path in its
// Issue #24 slot, the panel shows the placeholder, no row overflows
// the frame, and every layout dimension stays nonnegative.
func TestUnreadableComposedViewAtConstrainedWidths(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{loader: failAllLoader})
	m.theme = theme.Plain()
	idx := navIndex(t, []navFile{{
		name:    "deep-" + strings.Repeat("n", 90) + "-leaf.txt",
		content: "hit\n",
		stops:   []navStop{{line: 1, start: 0, end: 3}},
	}})
	msg, ok := fileLoadOf(startBrowse(t, m, idx))
	if !ok {
		t.Fatal("the startup load produced no completion")
	}
	m.Update(msg)
	m.Update(keyEsc) // dismiss the failure overlay to see the frame

	for _, w := range []int{80, 30, 20} {
		m.Update(tea.WindowSizeMsg{Width: w, Height: 24})
		v := viewText(m)
		if got := strings.Count(v, "\n") + 1; got != 24 {
			t.Fatalf("width %d: view rows = %d, want 24", w, got)
		}
		if !strings.Contains(v, "(unreadable)") {
			t.Fatalf("width %d: view = %q, want the (unreadable) placeholder", w, v)
		}
		rule := frameRow(t, m, 0)
		if !strings.Contains(rule, "…") || !strings.Contains(rule, "leaf.txt") {
			t.Fatalf("width %d: filename row = %q, want the …-truncated path ending at the basename", w, rule)
		}
		for r := 0; r < 24; r++ {
			if cw := safepresentation.CellWidth(frameRow(t, m, r)); cw > w {
				t.Fatalf("width %d: row %d is %d cells — overflow: %q", w, r, cw, frameRow(t, m, r))
			}
		}
		if m.listWidth() < 0 || w-m.listWidth()-1 < 0 {
			t.Fatalf("width %d: negative layout dimension: list %d", w, m.listWidth())
		}
	}
}

// reentryRig bundles the gated re-entry fixture: the model positioned
// on c.txt after a's load, b's first failure (armed with firstErr),
// its dismissal, and c's load — ready for the p re-entry. The gate
// holds exactly load number four — b's re-entry retry is the fourth
// minted load (a=1, b=2, c=3, retry=4) in this deterministic order.
type reentryRig struct {
	m      *model
	idx    *searchindex.Index
	loader *gatedFailLoader
	gate   *heldNthLoad
	keyB   string
}

func newReentryRig(t *testing.T, firstErr error) *reentryRig {
	t.Helper()
	idx := navIndex(t, isoFiles)
	loader := newGatedFailLoader()
	keyB := string(idx.Files[1].Path)
	loader.set(keyB, firstErr)
	gate := newHeldNthLoad(4)
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{
		loadGate:   gate.fn(),
		loader:     loader.read,
		popupTimer: popupStubTicks.popupTimer,
	})
	finishLoad(t, m, startBrowse(t, m, idx)) // call 1: a
	msg, ok := fileLoadOf(navCmd(t, m, keyN))
	if !ok {
		t.Fatal("crossing into b produced no load completion")
	}
	m.Update(msg) // call 2: b fails → overlay + (unreadable)
	m.Update(keyEsc)
	finishLoad(t, m, navCmd(t, m, keyN)) // call 3: c — cursor on c
	return &reentryRig{m: m, idx: idx, loader: loader, gate: gate, keyB: keyB}
}

// reenter drives the p re-entry into the failed b.txt and launches the
// minted retry worker — the crossing batch's first leaf — returning
// its message channel once the worker is in flight and held. The
// prior-failure overlay and "Loading…" are already up.
func (r *reentryRig) reenter(t *testing.T) <-chan tea.Msg {
	t.Helper()
	_, nav := r.m.Update(keyP)
	if _, ok := r.m.loading[r.keyB]; !ok {
		t.Fatal("re-entering the failed file minted no retry load")
	}
	batch, ok := nav().(tea.BatchMsg)
	if !ok || len(batch) < 1 {
		t.Fatalf("re-entry batch = %#v, want the retry leaf first", nav())
	}
	worker := runCmd(batch[0])
	<-r.gate.entered // the retry is in flight and held
	return worker
}

// The re-entry sequence opens the prior-failure overlay and switches
// the panel to "Loading…" immediately — before the retry settles —
// and mints exactly one retry while the overlay is up: re-entering
// again during the in-flight retry drops the duplicate per Issue #25's
// one-load-per-path rule.
func TestReentryShowsPriorFailureAndStartsOneRetry(t *testing.T) {
	rig := newReentryRig(t, errUnreadable)
	worker := rig.reenter(t)

	req := rig.m.loading[rig.keyB]
	if !rig.m.overlayOpen {
		t.Fatal("re-entry did not show the prior-failure overlay")
	}
	want := loadDiag(rig.idx.Files[1].Path, errUnreadable)
	if !strings.Contains(rig.m.overlay.text, want) {
		t.Fatalf("overlay = %q, want the prior failure %q", rig.m.overlay.text, want)
	}
	if v := viewText(rig.m); !strings.Contains(v, "Loading…") {
		t.Fatalf("view = %q, want Loading… for the in-flight retry", v)
	}

	// Re-entry while the retry is in flight drops the duplicate: the
	// request identity and the call count are untouched.
	rig.m.Update(keyN) // to c — cached
	rig.m.Update(keyP) // back to b — the retry still held
	if got := rig.m.loading[rig.keyB]; got != req {
		t.Fatal("re-entry during the in-flight retry re-minted the request")
	}
	if n := rig.gate.calls.Load(); n != 4 {
		t.Fatalf("load calls = %d, want 4 — the duplicate retry must be dropped", n)
	}
	select {
	case msg := <-worker:
		t.Fatalf("the held retry completed early: %#v", msg)
	default:
	}

	// Settle success so the worker cannot outlive the test.
	rig.loader.set(rig.keyB, nil)
	close(rig.gate.release)
	_, lc := rig.m.Update(<-worker)
	deliverLayout(t, rig.m, lc)
}

// Esc dismisses the re-entry overlay without disturbing the in-flight
// retry: the minted request stays live, the worker stays held, and the
// settlement lands on its own — a second failure restores
// "(unreadable)" and re-opens the overlay with the new occurrence.
func TestReentryRetryEscLeavesLoadUndisturbed(t *testing.T) {
	rig := newReentryRig(t, errUnreadable)
	worker := rig.reenter(t)
	req := rig.m.loading[rig.keyB]

	rig.m.Update(keyEsc)
	if rig.m.overlayOpen {
		t.Fatal("Esc did not dismiss the prior-failure overlay")
	}
	if got := rig.m.loading[rig.keyB]; got != req {
		t.Fatal("dismissing the overlay disturbed the in-flight retry")
	}
	select {
	case msg := <-worker:
		t.Fatalf("the retry completed while its gate was held: %#v", msg)
	default:
	}

	close(rig.gate.release)
	rig.m.Update(<-worker)
	if v := viewText(rig.m); !strings.Contains(v, "(unreadable)") {
		t.Fatalf("view = %q, want (unreadable) after the second failure", v)
	}
	if !rig.m.overlayOpen || !strings.Contains(rig.m.overlay.text, "cannot read") {
		t.Fatalf("overlay open=%v text=%q, want the second failure shown",
			rig.m.overlayOpen, rig.m.overlay.text)
	}
	want := loadDiag(rig.idx.Files[1].Path, errUnreadable)
	assertReplayLines(t, rig.m.diags, []string{want, want})
}

// A successful retry collects nothing new, replaces "Loading…" with
// content, and leaves the prior-failure overlay displayed until the
// user dismisses it.
func TestReentryRetrySuccessKeepsPriorOverlay(t *testing.T) {
	rig := newReentryRig(t, errUnreadable)
	worker := rig.reenter(t)

	rig.loader.set(rig.keyB, nil)
	close(rig.gate.release)
	_, lc := rig.m.Update(<-worker)
	deliverLayout(t, rig.m, lc)

	if v := viewText(rig.m); strings.Contains(v, "Loading…") || !strings.Contains(v, "b line") {
		t.Fatalf("view = %q, want b's content after the successful retry", v)
	}
	if !rig.m.overlayOpen {
		t.Fatal("the prior-failure overlay closed on the successful retry")
	}
	want := loadDiag(rig.idx.Files[1].Path, errUnreadable)
	assertReplayLines(t, rig.m.diags, []string{want})
	rig.m.Update(keyEsc)
	if rig.m.overlayOpen {
		t.Fatal("Esc did not dismiss the prior-failure overlay")
	}
}

// A second failure appends exactly one new occurrence to the open
// overlay without moving the reader's scroll position — the
// append-preserving-scroll primitive this issue owns — and collects
// exactly one new occurrence for the replay.
func TestReentryRetrySecondFailureAppends(t *testing.T) {
	// A long failure reason wraps the overlay past one screen so the
	// reader can scroll before the append lands.
	longErr := errors.New("denied: " + strings.Repeat("again ", 120))
	rig := newReentryRig(t, longErr)
	rig.m.Update(tea.WindowSizeMsg{Width: 30, Height: 10})
	worker := rig.reenter(t)

	rig.m.scrollOverlay(2)
	if rig.m.overlay.scroll != 2 {
		t.Fatalf("overlay scroll = %d, want 2 — the prior failure must be scrollable", rig.m.overlay.scroll)
	}

	close(rig.gate.release)
	rig.m.Update(<-worker)

	// The settlement landed without waiting on dismissal: the failure
	// record marks the placeholder "(unreadable)" — visible once the
	// frame-filling overlay is dismissed below.
	if !rig.m.failed[rig.keyB] {
		t.Fatal("the second failure left no failure record")
	}
	if n := strings.Count(rig.m.overlay.text, "cannot read"); n != 2 {
		t.Fatalf("overlay occurrences = %d, want the second failure appended exactly once: %q",
			n, rig.m.overlay.text)
	}
	if rig.m.overlay.scroll != 2 {
		t.Fatalf("overlay scroll = %d after the append, want the preserved 2", rig.m.overlay.scroll)
	}
	want := loadDiag(rig.idx.Files[1].Path, longErr)
	assertReplayLines(t, rig.m.diags, []string{want, want})

	rig.m.Update(keyEsc)
	if v := viewText(rig.m); !strings.Contains(v, "(unreadable)") {
		t.Fatalf("view = %q after dismissal, want (unreadable)", v)
	}
}

// Navigating away while the retry is in flight lets it settle per
// Issue #25 — updating only that path's cache and failure state — and
// a later re-entry runs the same sequence again against the new prior
// state: prior-failure overlay, "Loading…", one minted retry.
func TestReentryRetryAwayAndBack(t *testing.T) {
	rig := newReentryRig(t, errUnreadable)
	worker := rig.reenter(t)
	rig.m.Update(keyEsc)
	rig.m.Update(keyN) // away to c — the retry still held

	// The retry settles while b is non-current: the second failure is
	// diagnostic-only — no overlay, no panel change — and b records
	// the new prior failure a later visit will show.
	close(rig.gate.release)
	rig.m.Update(<-worker)
	if rig.m.overlayOpen {
		t.Fatal("a non-current retry settlement opened the overlay")
	}
	if !rig.m.failed[rig.keyB] {
		t.Fatal("the away settlement left no new prior failure")
	}
	if v := viewText(rig.m); !strings.Contains(v, "c line") {
		t.Fatalf("view = %q, want c's content — settlement must not touch the panel", v)
	}
	want := loadDiag(rig.idx.Files[1].Path, errUnreadable)
	assertReplayLines(t, rig.m.diags, []string{want, want})

	// The later re-entry follows the same sequence against the new
	// prior state: overlay up, "Loading…", one retry — whose third
	// failure appends to the open overlay and the collection.
	_, nav := rig.m.Update(keyP)
	if _, ok := rig.m.loading[rig.keyB]; !ok {
		t.Fatal("the later re-entry minted no retry")
	}
	if !rig.m.overlayOpen || !strings.Contains(rig.m.overlay.text, want) {
		t.Fatalf("overlay open=%v text=%q, want the new prior failure shown",
			rig.m.overlayOpen, rig.m.overlay.text)
	}
	if v := viewText(rig.m); !strings.Contains(v, "Loading…") {
		t.Fatalf("view = %q, want Loading… for the new retry", v)
	}
	msg, ok := fileLoadOf(nav)
	if !ok {
		t.Fatal("the later re-entry batch carried no retry leaf")
	}
	rig.m.Update(msg)
	if n := strings.Count(rig.m.overlay.text, "cannot read"); n != 2 {
		t.Fatalf("overlay occurrences = %d, want the new failure appended once", n)
	}
	assertReplayLines(t, rig.m.diags, []string{want, want, want})
}

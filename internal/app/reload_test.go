package app

import (
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/filebuffer"
	"vrg/internal/viewport"
)

// keyR is the explicit-reload key: r rereads the current file.
var keyR = tea.KeyPressMsg{Text: "r", Code: 'r'}

// layoutHold is a layout-preparation hold armed mid-test: while armed,
// every arriving worker signals entered then blocks on release, so the
// initial install runs through and only workers minted after arming
// are held.
type layoutHold struct {
	armed   atomic.Bool
	entered chan struct{}
	release chan struct{}
}

func newLayoutHold() *layoutHold {
	return &layoutHold{entered: make(chan struct{}, 16), release: make(chan struct{})}
}

func (h *layoutHold) fn() func() {
	return func() {
		if h.armed.Load() {
			h.entered <- struct{}{}
			<-h.release
		}
	}
}

// reloadCmd presses r and returns the reload command it issued.
func reloadCmd(t *testing.T, m *model) tea.Cmd {
	t.Helper()
	_, cmd := m.Update(keyR)
	if cmd == nil {
		t.Fatal("r issued no reload command")
	}
	return cmd
}

// rewriteFile replaces a fixture file's bytes on disk — the simulated
// disk edit the cached display intentionally ignores until r.
func rewriteFile(t *testing.T, path []byte, content string) {
	t.Helper()
	if err := os.WriteFile(string(path), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// reloadFiles is the reload fixture: a.txt deep enough to scroll and
// b.txt for the crossing legs.
var reloadFiles = []navFile{
	{name: "a.txt", content: numberedContent("a", 60), stops: []navStop{{line: 10, start: 0, end: 1}}},
	{name: "b.txt", content: numberedContent("b", 40), stops: []navStop{{line: 20, start: 0, end: 1}}},
}

// r rereads the current file without rerunning rg: exactly one new
// load is minted under the path's identity, "Loading…" is shown while
// it is in flight, the filename row keeps naming the path, and neither
// the index nor the cursor stops change.
func TestRShowsLoadingAndRereadsOnce(t *testing.T) {
	var reads atomic.Int32
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{
		loader: func(p []byte) ([]byte, error) {
			reads.Add(1)
			return filebuffer.Read(p)
		},
	})
	idx := navIndex(t, reloadFiles)
	keyA := string(idx.Files[0].Path)
	finishLoad(t, m, startBrowse(t, m, idx))
	idx0 := m.idx
	cur0, _ := m.idx.Cursor()
	rev0 := m.revs[keyA]

	_, cmd := m.Update(keyR)
	if cmd == nil {
		t.Fatal("r issued no reload command")
	}
	if req := m.loading[keyA]; req == 0 {
		t.Fatal("r minted no load request for the current file")
	}
	v := viewText(m)
	if !strings.Contains(v, "Loading…") {
		t.Fatalf("view = %q, want Loading… during the reload", v)
	}
	if !strings.Contains(v, "─ "+escapedPath(idx.Files[0])+" ") {
		t.Fatalf("view = %q, want the filename row still naming the path", v)
	}
	if m.idx != idx0 {
		t.Fatal("r rebuilt the search index — a reload never reruns rg")
	}
	if cur, _ := m.idx.Cursor(); cur != cur0 {
		t.Fatalf("r moved the cursor to %v, want %v — stops are unchanged", cur, cur0)
	}
	if got := m.revs[keyA]; got != rev0 {
		t.Fatalf("the request alone bumped the revision to %d, want %d", got, rev0)
	}

	// The command performs exactly one reread of the file.
	msg, ok := fileLoadOf(cmd)
	if !ok {
		t.Fatal("the reload command produced no fileLoadedMsg")
	}
	if got := reads.Load(); got != 2 {
		t.Fatalf("loader calls = %d, want 2 — exactly one reread after the initial load", got)
	}
	_, lc := m.Update(msg)
	deliverLayout(t, m, lc)
	if v := viewText(m); strings.Contains(v, "Loading…") || !strings.Contains(v, "a line") {
		t.Fatalf("view = %q after the reload settled, want content", v)
	}
}

// A second r while the reload is in flight is dropped, not queued: no
// new request is minted and no work is queued for settlement. Once the
// placeholder settles, r starts a new load.
func TestDuplicateRDroppedNotQueued(t *testing.T) {
	gate := newHeldNthLoad(2) // the reload is the second minted load
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{loadGate: gate.fn()})
	idx := navIndex(t, reloadFiles)
	keyA := string(idx.Files[0].Path)
	finishLoad(t, m, startBrowse(t, m, idx)) // load 1: the initial read

	_, cmd := m.Update(keyR)
	if cmd == nil {
		t.Fatal("r issued no reload command")
	}
	worker := runCmd(cmd)
	<-gate.entered // the reload is in flight and held
	req := m.loading[keyA]

	// r again while in flight: dropped, not queued — no load leaf,
	// no re-minted request, no extra worker.
	_, dup := m.Update(keyR)
	for _, msg := range leafMsgs(dup) {
		if _, ok := msg.(fileLoadedMsg); ok {
			t.Fatalf("duplicate r issued a load: %#v", msg)
		}
	}
	if got := m.loading[keyA]; got != req {
		t.Fatal("duplicate r re-minted the in-flight request")
	}
	if n := gate.calls.Load(); n != 2 {
		t.Fatalf("load calls = %d, want 2 — the duplicate must be dropped, not queued", n)
	}

	// The placeholder's change is the completion signal: after the
	// load settles, r starts a new load.
	close(gate.release)
	_, lc := m.Update(<-worker)
	deliverLayout(t, m, lc)
	if _, ok := m.loading[keyA]; ok {
		t.Fatal("the settled load still shows in flight")
	}
	_, cmd2 := m.Update(keyR)
	if cmd2 == nil {
		t.Fatal("r after settlement issued no new load")
	}
	if got := m.loading[keyA]; got == 0 || got == req {
		t.Fatalf("no fresh request minted after settlement: %d", got)
	}
	finishLoad(t, m, cmd2) // settle so no worker outlives the test
}

// A re-entry to a path whose reload is in flight is likewise dropped:
// the crossing mints no second load for it.
func TestReentryDuringReloadDropped(t *testing.T) {
	gate := newHeldNthLoad(2)
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{
		loadGate:   gate.fn(),
		popupTimer: popupStubTicks.popupTimer,
	})
	idx := navIndex(t, reloadFiles)
	keyA := string(idx.Files[0].Path)
	finishLoad(t, m, startBrowse(t, m, idx))

	_, cmd := m.Update(keyR)
	if cmd == nil {
		t.Fatal("r issued no reload command")
	}
	worker := runCmd(cmd)
	<-gate.entered // A's reload is in flight and held

	// n to B — its load is call 3 and runs through the gate, which
	// holds only call 2 — then p back to A: the re-entry is dropped.
	finishLoad(t, m, navCmd(t, m, keyN))
	req := m.loading[keyA]
	_, nav := m.Update(keyP)
	for _, msg := range leafMsgs(nav) {
		if _, ok := msg.(fileLoadedMsg); ok {
			t.Fatalf("re-entry during A's reload issued a load: %#v", msg)
		}
	}
	if got := m.loading[keyA]; got != req {
		t.Fatal("re-entry re-minted the in-flight reload request")
	}
	if n := gate.calls.Load(); n != 3 {
		t.Fatalf("load calls = %d, want 3 — no second load for A", n)
	}
	if v := viewText(m); !strings.Contains(v, "Loading…") {
		t.Fatalf("view = %q, want Loading… — the in-flight reload's placeholder", v)
	}

	close(gate.release)
	_, lc := m.Update(<-worker)
	deliverLayout(t, m, lc)
}

// The reload preserves the cursor and the logical viewport anchor —
// committed only when the new revision's matching prepared layout
// installs, never against the old revision's. While the replacement
// layout is held, the placeholder stays up and the saved viewport is
// untouched; on install the anchor lands on its text location in the
// new rows — no reveal runs.
func TestReloadPreservesAnchor(t *testing.T) {
	hold := newLayoutHold()
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{layoutGate: hold.fn()})
	idx := navIndex(t, reloadFiles)
	keyA := string(idx.Files[0].Path)
	finishLoad(t, m, startBrowse(t, m, idx)) // the unarmed install

	for i := 0; i < 20; i++ {
		m.Update(keyDown)
	}
	anchor0 := m.vps[keyA].Anchor()
	top0 := m.vps[keyA].Top()
	cur0, _ := m.idx.Cursor()
	if anchor0.Line == 1 {
		t.Fatal("scrolling did not move the anchor off line 1")
	}
	hold.armed.Store(true)

	// The file grows on disk — the anchor's line still exists and new
	// content follows it.
	rewriteFile(t, idx.Files[0].Path, numberedContent("a", 90))
	msg, ok := fileLoadOf(reloadCmd(t, m))
	if !ok {
		t.Fatal("the reload produced no completion")
	}
	_, lcmd := m.Update(msg)
	if got := m.revs[keyA]; got != 2 {
		t.Fatalf("content revision = %d after the reload, want 2", got)
	}
	if lcmd == nil {
		t.Fatal("the reload completion issued no layout request")
	}
	worker := runCmd(lcmd)
	<-hold.entered // the new revision's layout is held

	// Nothing commits against the old revision's layout: the
	// placeholder stays up and the saved viewport is untouched.
	if v := viewText(m); !strings.Contains(v, "Loading…") {
		t.Fatalf("view = %q, want Loading… until the new revision's layout installs", v)
	}
	if got := m.vps[keyA].Top(); got != top0 {
		t.Fatalf("top moved to %d before the layout installed, want %d", got, top0)
	}
	if got := m.vps[keyA].Anchor(); got != anchor0 {
		t.Fatalf("anchor = %+v before install, want %+v", got, anchor0)
	}

	close(hold.release)
	m.Update(<-worker)
	rows, ok := m.rows[keyA].(*viewport.Rows)
	if !ok {
		t.Fatal("the new revision's layout did not install")
	}
	if got, want := m.vps[keyA].Top(), rows.RowOf(anchor0); got != want {
		t.Fatalf("top after install = %d, want the anchor's row %d — reload preserves position, it does not reveal", got, want)
	}
	if got := m.vps[keyA].Anchor(); got != anchor0 {
		t.Fatalf("anchor = %+v after the reload, want %+v", got, anchor0)
	}
	if cur, _ := m.idx.Cursor(); cur != cur0 {
		t.Fatalf("the reload moved the cursor to %v, want %v", cur, cur0)
	}
	if got := rows.Len(); got != 90 {
		t.Fatalf("installed rows = %d, want 90 — the reloaded content", got)
	}
}

// The preserved anchor is clamped to the new content: when the reload
// finds a shorter file, the top clamps upward and the anchor updates
// to the resulting top — the documented lossy clamp, applied to the
// new revision's rows.
func TestReloadAnchorClampedOnShrink(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, reloadFiles)
	keyA := string(idx.Files[0].Path)
	finishLoad(t, m, startBrowse(t, m, idx))

	for i := 0; i < 35; i++ {
		m.Update(keyDown)
	}
	anchor0 := m.vps[keyA].Anchor()
	if anchor0.Line < 36 {
		t.Fatalf("anchor = %+v, want it deep in the file", anchor0)
	}

	rewriteFile(t, idx.Files[0].Path, numberedContent("a", 40))
	finishLoad(t, m, reloadCmd(t, m))

	rows, ok := m.rows[keyA].(*viewport.Rows)
	if !ok {
		t.Fatal("no layout installed after the reload")
	}
	want := viewport.MaxTop(rows.Len(), contentRows24)
	if got := m.vps[keyA].Top(); got != want {
		t.Fatalf("top after the shrink reload = %d, want the clamped %d", got, want)
	}
	if got := m.vps[keyA].Anchor(); got != rows.AnchorAt(want) {
		t.Fatalf("anchor = %+v after the clamp, want the clamped top's %+v", got, rows.AnchorAt(want))
	}
	if got := m.vps[keyA].Anchor(); got == anchor0 {
		t.Fatal("the lossy clamp left the out-of-range anchor untouched")
	}
}

// A failed reload replaces the old display with "(unreadable)": the
// stale buffer and layout are dropped, the error overlay shows the
// failure, and the filename row keeps naming the path — old content is
// never presented as refreshed.
func TestFailedReloadReplacesContent(t *testing.T) {
	loader := newGatedFailLoader()
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{loader: loader.read})
	idx := navIndex(t, reloadFiles)
	keyA := string(idx.Files[0].Path)
	finishLoad(t, m, startBrowse(t, m, idx))
	if v := viewText(m); !strings.Contains(v, "a line") {
		t.Fatalf("view = %q, want the loaded content before the reload", v)
	}

	loader.set(keyA, errUnreadable)
	msg, ok := fileLoadOf(reloadCmd(t, m))
	if !ok {
		t.Fatal("the reload produced no completion")
	}
	m.Update(msg)

	if !m.failed[keyA] {
		t.Fatal("the failed reload left no failure record")
	}
	if m.bufs[keyA] != nil {
		t.Fatal("the failed reload kept the stale buffer — old content must never pass as refreshed")
	}
	if m.rows[keyA] != nil {
		t.Fatal("the failed reload kept the stale layout")
	}
	if !m.overlayOpen || !strings.Contains(m.overlayText, "cannot read") {
		t.Fatalf("overlay open=%v text=%q, want the current-file failure shown",
			m.overlayOpen, m.overlayText)
	}
	v := viewText(m)
	if !strings.Contains(v, "(unreadable)") {
		t.Fatalf("view = %q, want (unreadable) after the failed reload", v)
	}
	if strings.Contains(v, "a line") {
		t.Fatalf("view = %q — stale content is still displayed", v)
	}
	if !strings.Contains(v, "─ "+escapedPath(idx.Files[0])+" ") {
		t.Fatalf("view = %q, want the filename row still naming the path", v)
	}
	assertReplayLines(t, m.diags, []string{loadDiag(idx.Files[0].Path, errUnreadable)})
}

// A second consecutive r failure appends exactly one new occurrence to
// the still-open overlay — the reader's position preserved — and
// collects exactly one new occurrence for the replay. r works while
// the failure overlay is open: it is the retry route and never
// dismisses the overlay.
func TestSecondConsecutiveReloadFailureAppends(t *testing.T) {
	longErr := errors.New("denied: " + strings.Repeat("again ", 120))
	loader := newGatedFailLoader()
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{loader: loader.read})
	idx := navIndex(t, reloadFiles)
	keyA := string(idx.Files[0].Path)
	finishLoad(t, m, startBrowse(t, m, idx))
	_, rc := m.Update(tea.WindowSizeMsg{Width: 30, Height: 10})
	deliverLayout(t, m, rc)
	loader.set(keyA, longErr)

	// The first r failure opens the overlay with the diagnostic; the
	// reader scrolls into it.
	msg, ok := fileLoadOf(reloadCmd(t, m))
	if !ok {
		t.Fatal("the first reload produced no completion")
	}
	m.Update(msg)
	if !m.overlayOpen {
		t.Fatal("the first r failure did not open the overlay")
	}
	m.scrollOverlay(2)
	if m.overlayScroll != 2 {
		t.Fatalf("overlay scroll = %d, want 2 — the diagnostic must be scrollable", m.overlayScroll)
	}

	// The second r is pressed while the overlay is still open — the
	// overlay does not block the retry — then fails again.
	_, cmd := m.Update(keyR)
	if cmd == nil {
		t.Fatal("r under the open overlay minted no retry load")
	}
	if !m.overlayOpen {
		t.Fatal("r dismissed the failure overlay")
	}
	if _, ok := m.loading[keyA]; !ok {
		t.Fatal("the second r minted no in-flight request")
	}
	msg, ok = fileLoadOf(cmd)
	if !ok {
		t.Fatal("the second reload produced no completion")
	}
	m.Update(msg)

	if n := strings.Count(m.overlayText, "cannot read"); n != 2 {
		t.Fatalf("overlay occurrences = %d, want the second failure appended exactly once: %q",
			n, m.overlayText)
	}
	if m.overlayScroll != 2 {
		t.Fatalf("overlay scroll = %d after the append, want the preserved 2", m.overlayScroll)
	}
	want := loadDiag(idx.Files[0].Path, longErr)
	assertReplayLines(t, m.diags, []string{want, want})

	m.Update(keyEsc)
	if v := viewText(m); !strings.Contains(v, "(unreadable)") {
		t.Fatalf("view = %q after dismissal, want (unreadable)", v)
	}
}

// A successful reload behind the open failure overlay leaves it
// displayed until the user dismisses it — the re-entry sequence's
// settlement rule, shared by r.
func TestReloadSuccessLeavesOverlayOpen(t *testing.T) {
	loader := newGatedFailLoader()
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{loader: loader.read})
	idx := navIndex(t, reloadFiles)
	keyA := string(idx.Files[0].Path)
	finishLoad(t, m, startBrowse(t, m, idx))
	loader.set(keyA, errUnreadable)

	msg, ok := fileLoadOf(reloadCmd(t, m))
	if !ok {
		t.Fatal("the first reload produced no completion")
	}
	m.Update(msg)
	if !m.overlayOpen {
		t.Fatal("the r failure did not open the overlay")
	}

	loader.set(keyA, nil) // the file reads again
	finishLoad(t, m, reloadCmd(t, m))
	if v := viewText(m); strings.Contains(v, "Loading…") || !strings.Contains(v, "a line") {
		t.Fatalf("view = %q, want the reloaded content behind the overlay", v)
	}
	if !m.overlayOpen {
		t.Fatal("the successful reload closed the prior-failure overlay")
	}
	m.Update(keyEsc)
	if m.overlayOpen {
		t.Fatal("Esc did not dismiss the prior-failure overlay")
	}
}

// r is the one-stop index's retry route: with a single matched line
// there is no n/p escape, yet r still rereads the file.
func TestRWorksWithOneStopIndex(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, []navFile{
		{name: "solo.txt", content: "one\ntwo hit\nthree\n", stops: []navStop{{line: 2, start: 4, end: 7}}},
	})
	finishLoad(t, m, startBrowse(t, m, idx))

	finishLoad(t, m, reloadCmd(t, m))
	if v := viewText(m); strings.Contains(v, "Loading…") || !strings.Contains(v, "three") {
		t.Fatalf("view = %q, want the reloaded content", v)
	}
	if got := m.revs[string(idx.Files[0].Path)]; got != 2 {
		t.Fatalf("revision = %d after the one-stop reload, want 2", got)
	}
}

// Cached content is intentionally stable until r: a simulated disk
// edit triggers no load — resize, scroll, and a there-and-back
// navigation leave the cached buffer, revision, and frame on the old
// content.
func TestDiskChangeWithoutRIsStable(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{popupTimer: popupStubTicks.popupTimer})
	idx := navIndex(t, reloadFiles)
	keyA := string(idx.Files[0].Path)
	finishLoad(t, m, startBrowse(t, m, idx))
	bufA, revA := m.bufs[keyA], m.revs[keyA]

	rewriteFile(t, idx.Files[0].Path, numberedContent("a", 90))

	// Nothing short of r rereads the file: a resize reprepares the
	// cached buffer's layout, scrolling moves within it, and a
	// there-and-back navigation mints no load for it.
	_, rc := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	deliverLayout(t, m, rc)
	for _, k := range []tea.KeyPressMsg{keyDown, keyPgDn, keyUp} {
		if _, cmd := m.Update(k); cmd != nil {
			for _, msg := range leafMsgs(cmd) {
				if _, ok := msg.(fileLoadedMsg); ok {
					t.Fatalf("a disk change triggered a load: %#v", msg)
				}
			}
		}
	}
	finishLoad(t, m, navCmd(t, m, keyN)) // to b.txt
	for _, msg := range navLeafMsgs(t, m, keyP) {
		if _, ok := msg.(fileLoadedMsg); ok {
			t.Fatalf("re-entry after a disk change issued a load: %#v", msg)
		}
	}
	if len(m.loading) != 0 {
		t.Fatalf("loads in flight after the disk change: %v", m.loading)
	}
	if m.bufs[keyA] != bufA || m.revs[keyA] != revA {
		t.Fatal("the disk edit changed the cached buffer or revision without r")
	}
	if got := m.rows[keyA].Len(); got != 60 {
		t.Fatalf("rows = %d, want 60 — the cached content, not the disk edit's 90", got)
	}
	if v := viewText(m); !strings.Contains(v, "a line") {
		t.Fatalf("view = %q, want the cached content still shown", v)
	}
}

// A reload produces a new content revision: a layout prepared for the
// pre-reload revision and released only after the reload completes is
// discarded — it never replaces the reloaded content, the installed
// layout, or the anchor (the Issue #17 revision-superseded contract).
func TestPreReloadLayoutSupersededDiscarded(t *testing.T) {
	hold := newLayoutHold()
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{layoutGate: hold.fn()})
	idx := navIndex(t, reloadFiles)
	keyA := string(idx.Files[0].Path)
	finishLoad(t, m, startBrowse(t, m, idx))
	installed := m.rows[keyA]

	for i := 0; i < 10; i++ {
		m.Update(keyDown)
	}
	anchor0 := m.vps[keyA].Anchor()
	vp0 := m.vps[keyA]
	hold.armed.Store(true)

	// Mint a pre-reload layout request for the current parameters,
	// then reload: its completion bumps the revision and requests the
	// new revision's layout — both workers held.
	_, rc := m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	if rc == nil {
		t.Fatal("the resize minted no layout request")
	}
	workerOld := runCmd(rc)
	<-hold.entered

	rewriteFile(t, idx.Files[0].Path, numberedContent("a", 90))
	msg, ok := fileLoadOf(reloadCmd(t, m))
	if !ok {
		t.Fatal("the reload produced no completion")
	}
	_, lc := m.Update(msg)
	if got := m.revs[keyA]; got != 2 {
		t.Fatalf("revision = %d after the reload, want 2", got)
	}
	if lc == nil {
		t.Fatal("the reload completion minted no layout request")
	}
	workerNew := runCmd(lc)
	<-hold.entered

	// Release both; the pre-reload layout arrives first — obsolete
	// against revision 2 — and is discarded without touching the
	// installed layout, the saved viewport, or the panel.
	close(hold.release)
	before := viewText(m)
	m.Update(<-workerOld)
	if m.rows[keyA] != installed {
		t.Fatal("a superseded-revision layout replaced the installed model")
	}
	if got := m.vps[keyA]; got != vp0 {
		t.Fatalf("a superseded layout moved the saved viewport to %+v", got)
	}
	if got := m.vps[keyA].Anchor(); got != anchor0 {
		t.Fatalf("a superseded layout changed the anchor to %+v", got)
	}
	if got := viewText(m); got != before {
		t.Fatal("a superseded layout changed the panel")
	}

	// The new revision's matching layout installs and the anchor
	// commits against it.
	m.Update(<-workerNew)
	rows, ok := m.rows[keyA].(*viewport.Rows)
	if !ok || rows.Key().Revision != 2 {
		t.Fatalf("installed layout = %#v, want revision 2", m.rows[keyA])
	}
	if got, want := m.vps[keyA].Top(), rows.RowOf(anchor0); got != want {
		t.Fatalf("top after install = %d, want the anchor's row %d", got, want)
	}
	if got := rows.Len(); got != 90 {
		t.Fatalf("installed rows = %d, want 90 — the reloaded content", got)
	}
}

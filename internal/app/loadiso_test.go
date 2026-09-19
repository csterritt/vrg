package app

import (
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/filebuffer"
)

// heldLoads is the load gate holding every file-load worker until
// release closes; entered is buffered so the test counts how many
// loads were started — each entered send is one minted request.
type heldLoads struct {
	entered chan struct{}
	release chan struct{}
}

func newHeldLoads() *heldLoads {
	return &heldLoads{entered: make(chan struct{}, 16), release: make(chan struct{})}
}

func (h *heldLoads) fn() func() {
	return func() {
		h.entered <- struct{}{}
		<-h.release
	}
}

// heldFirstLoad holds only the first load worker to arrive — the slow
// file's load — while every later load runs straight through: the
// A→B→C fixture needs exactly one slow load.
type heldFirstLoad struct {
	calls   atomic.Int32
	entered chan struct{}
	release chan struct{}
}

func newHeldFirstLoad() *heldFirstLoad {
	return &heldFirstLoad{entered: make(chan struct{}), release: make(chan struct{})}
}

func (h *heldFirstLoad) fn() func() {
	return func() {
		if h.calls.Add(1) == 1 {
			close(h.entered)
			<-h.release
		}
	}
}

// reqOf reports the in-flight load request identity recorded for path
// — 0 when no request is live.
func reqOf(m *model, path []byte) int { return m.loading[string(path)] }

// mintRequest records a fresh in-flight load request for path — the
// worker-side state a real request would set, including the reload
// requests Issue #27 will mint — and returns the request identity its
// completion must carry.
func mintRequest(m *model, path []byte) int {
	m.loadSeq++
	m.loading[string(path)] = m.loadSeq
	return m.loadSeq
}

// isoFiles is the three-file A→B→C fixture: one stop per file so every
// n/p step crosses a file boundary.
var isoFiles = []navFile{
	{name: "a.txt", content: numberedContent("a", 60), stops: []navStop{{line: 30, start: 0, end: 1}}},
	{name: "b.txt", content: numberedContent("b", 40), stops: []navStop{{line: 20, start: 0, end: 1}}},
	{name: "c.txt", content: numberedContent("c", 40), stops: []navStop{{line: 10, start: 0, end: 1}}},
}

// Navigation remains fully active while a file's load is held: n moves
// past the slow file at once — cursor, filename rule, pop-up, and the
// destination's own load request all land — scrolling the placeholder
// is a no-op, and w, c, and resize keep their normal meanings.
func TestNavigationRemainsActiveWhileLoadHeld(t *testing.T) {
	gate := newHeldLoads()
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{
		loadGate:   gate.fn(),
		popupTimer: popupStubTicks.popupTimer,
	})
	idx := navIndex(t, isoFiles)
	cmdA := startBrowse(t, m, idx)
	keyB := string(idx.Files[1].Path)

	workerA := runCmd(cmdA)
	<-gate.entered // A's load is in flight and held

	// n moves past the slow file at once: the cursor is on B's stop,
	// the panel names b.txt over a Loading… placeholder, the crossing
	// opens its pop-up, and B's own load is requested.
	_, nav := m.Update(keyN)
	cur, _ := m.idx.Cursor()
	if cur.File != 1 {
		t.Fatalf("cursor file = %d while A's load is held, want 1 (moved past it)", cur.File)
	}
	if _, ok := m.loading[keyB]; !ok {
		t.Fatal("n into B minted no load request")
	}
	v := viewText(m)
	if !strings.Contains(v, "Loading…") {
		t.Fatalf("view = %q, want Loading… for the uncached destination", v)
	}
	if !strings.Contains(v, "─ "+escapedPath(idx.Files[1])+" ") {
		t.Fatalf("view = %q, want the filename rule naming b.txt", v)
	}
	if m.popupID == 0 {
		t.Fatal("the file-change pop-up did not start at selection")
	}
	// The navigation batches the destination's load leaf first.
	batch, ok := nav().(tea.BatchMsg)
	if !ok || len(batch) < 1 {
		t.Fatalf("crossing batch = %#v, want the load leaf first", nav())
	}
	workerB := runCmd(batch[0])
	<-gate.entered // B's load is in flight and held too

	// Scrolling the placeholder is a strict no-op — the crossing's
	// horizontal reset left a zero viewport the scroll keys must not
	// move; w, c, and a resize keep their normal meanings — none wait
	// on the held workers.
	for _, k := range []tea.KeyPressMsg{keyDown, keyUp, keyD, keyPgDn} {
		m.Update(k)
	}
	if got := m.vps[keyB].Top(); got != 0 {
		t.Fatalf("scrolling Loading… moved the viewport to %d, want 0", got)
	}
	m.Update(keyW)
	if m.wrap {
		t.Fatal("w did not toggle wrap while a load was held")
	}
	m.Update(keyW) // restore wrap for the content assertions below
	m.Update(keyC)
	if v := viewText(m); !strings.HasPrefix(v, "\x1b[30;47m") {
		t.Fatalf("view after c = %q, want the light scheme applied", v)
	}
	m.Update(keyC) // restore dark
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	if m.width != 100 {
		t.Fatalf("resize not applied while loads held: width = %d", m.width)
	}
	select {
	case msg := <-workerA:
		t.Fatalf("A's completion arrived while its load was held: %#v", msg)
	default:
	}

	// Release: B's completion lands and only B settles; A's late
	// completion then caches A without disturbing the panel.
	close(gate.release)
	_, lc := m.Update(<-workerB)
	deliverLayout(t, m, lc)
	if v := viewText(m); strings.Contains(v, "Loading…") || !strings.Contains(v, "b line") {
		t.Fatalf("view = %q after B's completion, want B's content", v)
	}
	before := viewText(m)
	m.Update(<-workerA)
	if got := viewText(m); got != before {
		t.Fatal("A's late completion changed B's panel")
	}
}

// A→B→C while the first file's load is held: A's completion arriving
// while C is current updates only A's cache — C's panel, the cursor,
// and C's viewport are untouched.
func TestLateCompletionCachesOnlyItsOwnPath(t *testing.T) {
	gate := newHeldFirstLoad()
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{
		loadGate:   gate.fn(),
		popupTimer: popupStubTicks.popupTimer,
	})
	idx := navIndex(t, isoFiles)
	keyA := string(idx.Files[0].Path)
	cmdA := startBrowse(t, m, idx)
	workerA := runCmd(cmdA)
	<-gate.entered // A's load is held; later loads run straight through

	// A→B→C: each destination loads at once and becomes current.
	finishLoad(t, m, navCmd(t, m, keyN)) // B current
	finishLoad(t, m, navCmd(t, m, keyN)) // C current
	before := viewText(m)
	if !strings.Contains(before, "c line") {
		t.Fatalf("C's panel = %q, want c.txt content", before)
	}

	close(gate.release)
	_, ac := m.Update(<-workerA)
	deliverLayout(t, m, ac) // even A's layout install stays invisible
	if m.bufs[keyA] == nil {
		t.Fatal("A's late completion was not cached")
	}
	if _, ok := m.loading[keyA]; ok {
		t.Fatal("A's settled load still shows in flight")
	}
	if got := viewText(m); got != before {
		t.Fatalf("A's late completion changed C's panel:\nbefore %q\nafter  %q", before, got)
	}
	if cur, _ := m.idx.Cursor(); cur.File != 2 {
		t.Fatalf("cursor moved to file %d on a late completion, want 2", cur.File)
	}
}

// At most one load is ever in flight per raw path: re-entering a path
// whose load is pending starts nothing and queues nothing — the
// crossing's batch carries only the pop-up leaf, the live request is
// untouched, and settlement leaves no queued work behind.
func TestReentryDuringLoadStartsNothing(t *testing.T) {
	gate := newHeldLoads()
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{
		loadGate:   gate.fn(),
		popupTimer: popupStubTicks.popupTimer,
	})
	idx := navIndex(t, isoFiles)
	keyA := string(idx.Files[0].Path)
	cmdA := startBrowse(t, m, idx)
	workerA := runCmd(cmdA)
	<-gate.entered // load #1: A

	_, nav := m.Update(keyN) // cross to B: load #2
	batch, ok := nav().(tea.BatchMsg)
	if !ok || len(batch) < 1 {
		t.Fatalf("crossing batch = %#v, want the load leaf first", nav())
	}
	workerB := runCmd(batch[0])
	<-gate.entered // load #2: B

	// p re-enters A while its load is still held. The request is
	// dropped, not queued: the batch is only the pop-up leaf — a
	// second load leaf would appear here as a BatchMsg — and A's
	// recorded request is still the live one.
	reqA := m.loading[keyA]
	_, navA := m.Update(keyP)
	if navA != nil {
		if _, ok := navA().(popupExpireMsg); !ok {
			t.Fatalf("re-entry batch carried a load leaf: %#v", navA())
		}
	}
	if got := m.loading[keyA]; got != reqA {
		t.Fatal("re-entry re-minted A's request — the duplicate must be dropped")
	}
	if len(gate.entered) != 0 {
		t.Fatal("re-entering a loading path started a second load")
	}

	// Settlement leaves nothing queued: both completions land, no
	// request remains in flight, and A's single completion caches it.
	close(gate.release)
	m.Update(<-workerA)
	m.Update(<-workerB)
	if len(m.loading) != 0 {
		t.Fatalf("queued load work survived settlement: %v", m.loading)
	}
	if m.bufs[keyA] == nil {
		t.Fatal("A's single completion did not cache it")
	}
}

// A completed buffer is retained for the session — no eviction:
// revisiting the file shows the cached content and issues no new load.
func TestCachedRevisitIssuesNoLoad(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, popupStubTicks)
	idx := navIndex(t, isoFiles)
	finishLoad(t, m, startBrowse(t, m, idx))
	keyA := string(idx.Files[0].Path)
	bufA := m.bufs[keyA]
	if bufA == nil {
		t.Fatal("A's completion did not cache a buffer")
	}

	finishLoad(t, m, navCmd(t, m, keyN)) // visit B

	// p back to A: the cached buffer serves — the crossing runs only
	// the pop-up leaf, never a load.
	for _, msg := range navLeafMsgs(t, m, keyP) {
		if _, ok := msg.(fileLoadedMsg); ok {
			t.Fatalf("revisiting a cached file issued a load: %#v", msg)
		}
	}
	if m.bufs[keyA] != bufA {
		t.Fatal("the cached buffer was replaced or evicted on revisit")
	}
	if v := viewText(m); !strings.Contains(v, "a line") {
		t.Fatalf("view = %q, want A's cached content on the revisit", v)
	}
}

// A completion naming a path with no request in flight cannot be that
// path's answer: it is discarded without touching the cache, the
// failure record, the diagnostics, or the visible panel.
func TestUnrequestedCompletionDropped(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, isoFiles)
	startBrowse(t, m, idx) // only A's request is in flight
	keyB, keyC := string(idx.Files[1].Path), string(idx.Files[2].Path)

	buf, err := filebuffer.Load(idx.Files[1].Path, idx.Files[1].Stops)
	if err != nil {
		t.Fatal(err)
	}
	before := viewText(m)
	diags := len(m.diags)
	if _, cmd := m.Update(fileLoadedMsg{path: idx.Files[1].Path, buf: buf}); cmd != nil {
		t.Fatal("an unrequested completion issued work")
	}
	if m.bufs[keyB] != nil {
		t.Fatal("an unrequested completion cached a buffer")
	}
	if m.failed[keyB] {
		t.Fatal("an unrequested completion set failure state")
	}
	if got := viewText(m); got != before {
		t.Fatal("an unrequested completion changed the panel")
	}

	// A fabricated failure is likewise dropped: no failure state and
	// no collected diagnostic.
	m.Update(fileLoadedMsg{path: idx.Files[2].Path, err: errors.New("nope")})
	if m.failed[keyC] {
		t.Fatal("an unrequested failure marked the path failed")
	}
	if len(m.diags) != diags {
		t.Fatal("an unrequested failure collected a diagnostic")
	}
}

// A completion that does not carry the live request's identity is
// stale or forged: it is discarded — the in-flight request stays
// pending and the placeholder stays up for the real answer.
func TestStaleCompletionDroppedWhileLoadInFlight(t *testing.T) {
	gate := newHeldLoads()
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{loadGate: gate.fn()})
	idx := navIndex(t, isoFiles)
	keyA := string(idx.Files[0].Path)
	cmdA := startBrowse(t, m, idx)
	workerA := runCmd(cmdA)
	<-gate.entered // A's real request is in flight

	buf, err := filebuffer.Load(idx.Files[0].Path, idx.Files[0].Stops)
	if err != nil {
		t.Fatal(err)
	}
	// A completion not carrying the live request identity: the
	// in-flight request must survive it untouched.
	m.Update(fileLoadedMsg{path: idx.Files[0].Path, buf: buf})
	if _, ok := m.loading[keyA]; !ok {
		t.Fatal("a stale completion settled the live request")
	}
	if m.bufs[keyA] != nil {
		t.Fatal("a stale completion cached its buffer")
	}
	if v := viewText(m); !strings.Contains(v, "Loading…") {
		t.Fatalf("view = %q, want Loading… still up for the pending load", v)
	}

	// The real answer still lands when the gate releases.
	close(gate.release)
	_, lc := m.Update(<-workerA)
	deliverLayout(t, m, lc)
	if v := viewText(m); !strings.Contains(v, "a line") {
		t.Fatalf("view = %q, want A's content from the real completion", v)
	}
}

// A second completion for an already settled load is discarded: the
// cached buffer, the content revision, the installed layout, and the
// panel all stay put.
func TestSettledCompletionDropped(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, isoFiles)
	finishLoad(t, m, startBrowse(t, m, idx))
	keyA := string(idx.Files[0].Path)
	bufA, revA, rowsA := m.bufs[keyA], m.revs[keyA], m.rows[keyA]
	before := viewText(m)

	other, err := filebuffer.Load(idx.Files[1].Path, idx.Files[1].Stops)
	if err != nil {
		t.Fatal(err)
	}
	if _, cmd := m.Update(fileLoadedMsg{path: idx.Files[0].Path, buf: other}); cmd != nil {
		t.Fatal("a duplicate completion issued work")
	}
	if m.bufs[keyA] != bufA {
		t.Fatal("a duplicate completion replaced the cached buffer")
	}
	if m.revs[keyA] != revA {
		t.Fatalf("a duplicate completion bumped the revision to %d", m.revs[keyA])
	}
	if m.rows[keyA] != rowsA {
		t.Fatal("a duplicate completion disturbed the installed layout")
	}
	if got := viewText(m); got != before {
		t.Fatal("a duplicate completion changed the panel")
	}
}

// Completions — success or failure — arriving after cancellation are
// discarded: nothing is cached, marked, or collected once the
// controlled exit begins.
func TestPostCancellationCompletionDiscarded(t *testing.T) {
	gate := newHeldLoads()
	m := newTestModel(newKillChild(Result{Code: -1, Err: errors.New("signal: killed")}),
		options{loadGate: gate.fn()})
	idx := navIndex(t, isoFiles)
	cmdA := startBrowse(t, m, idx)
	workerA := runCmd(cmdA)
	<-gate.entered

	_, q := m.Update(keyCtrlC)
	runQuittingCmd(t, q)
	close(gate.release)
	m.Update(<-workerA)
	m.Update(fileLoadedMsg{path: idx.Files[1].Path, err: errors.New("late")})
	if len(m.bufs) != 0 || len(m.failed) != 0 || len(m.diags) != 0 {
		t.Fatalf("post-cancellation completions mutated state: bufs=%v failed=%v diags=%v",
			m.bufs, m.failed, m.diags)
	}
}

// The decode/map phase of the current file's load is separately
// gatable: with the read done and decode/map held, ctrl+c exits 130,
// n/p move the cursor, w toggles wrap, c toggles the scheme, and a
// resize applies — none wait on the worker. The released completion
// then lands on a cancelled UI and is discarded.
func TestGatedDecodeMapKeepsInputsResponsive(t *testing.T) {
	gate := newHeldLoads()
	m := newTestModel(newKillChild(Result{Code: -1, Err: errors.New("signal: killed")}), options{
		decodeGate: gate.fn(),
		popupTimer: popupStubTicks.popupTimer,
	})
	idx := navIndex(t, isoFiles)
	keyA, keyB := string(idx.Files[0].Path), string(idx.Files[1].Path)
	cmdA := startBrowse(t, m, idx)
	workerA := runCmd(cmdA)
	<-gate.entered // the read is done; decode/map is held

	// n advances to B at once — its own request minted — and p comes
	// back; neither waits on the held phase.
	_, navB := m.Update(keyN)
	cur, _ := m.idx.Cursor()
	if cur.File != 1 {
		t.Fatalf("cursor file = %d while decode/map held, want 1", cur.File)
	}
	if _, ok := m.loading[keyB]; !ok {
		t.Fatal("n minted no load request while decode/map held")
	}
	// The crossing's load leaf joins the gate on its own goroutine.
	batch, ok := navB().(tea.BatchMsg)
	if !ok || len(batch) < 1 {
		t.Fatalf("crossing batch = %#v, want the load leaf first", navB())
	}
	workerB := runCmd(batch[0])
	<-gate.entered
	m.Update(keyP) // back to A: dropped re-entry, still one request
	if _, ok := m.loading[keyA]; !ok {
		t.Fatal("A's live request disappeared under n/p while held")
	}

	// w toggles wrap and c the scheme while the worker stays held.
	m.Update(keyW)
	if m.wrap {
		t.Fatal("w did not toggle wrap while decode/map was held")
	}
	m.Update(keyW)
	m.Update(keyC)
	if v := viewText(m); !strings.HasPrefix(v, "\x1b[30;47m") {
		t.Fatalf("view after c = %q, want the light scheme applied", v)
	}
	m.Update(keyC)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	if m.width != 100 {
		t.Fatalf("resize not applied while decode/map held: width = %d", m.width)
	}
	select {
	case msg := <-workerA:
		t.Fatalf("completion arrived while decode/map was held: %#v", msg)
	default:
	}

	// ctrl+c cancels without waiting on the held phase; the released
	// completions land on a cancelled UI and are discarded.
	_, q := m.Update(keyCtrlC)
	runQuittingCmd(t, q)
	if m.status != 130 {
		t.Fatalf("status = %d, want ctrl+c's 130", m.status)
	}
	close(gate.release)
	m.Update(<-workerA)
	m.Update(<-workerB)
	if len(m.bufs) != 0 {
		t.Fatalf("post-cancellation completions cached buffers: %v", m.bufs)
	}
}

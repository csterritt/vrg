package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/viewport"
)

// admitFiles is the load-admission fixture: each file's single stop
// sits at line 50 — deep enough that the destination reveal must move
// the viewport, so a misapplied anchor preservation (top 0 on a first
// visit) is visibly different from the reveal's placement.
var admitFiles = []navFile{
	{name: "a.txt", content: numberedContent("a", 60), stops: []navStop{{line: 50, start: 0, end: 6}}},
	{name: "b.txt", content: numberedContent("b", 60), stops: []navStop{{line: 50, start: 0, end: 6}}},
}

// revealTop is the viewport top the destination reveal leaves for the
// cursor's current stop once path's layout is installed: the target's
// rendered row placed at floor(content height / 3), clamped to valid
// tops (Issue #14's rule, Issue #28's commit).
func revealTop(t *testing.T, m *model, path string) int {
	t.Helper()
	rows, ok := m.rows[path].(*viewport.Rows)
	if !ok {
		t.Fatalf("no installed row model for %s", path)
	}
	cur, ok := m.idx.Cursor()
	if !ok {
		t.Fatal("no cursor")
	}
	st := m.idx.Files[cur.File].Stops[cur.Stop]
	want := rows.TargetRow(st) - m.contentRows()/3
	if max := viewport.MaxTop(rows.Len(), m.contentRows()); want > max {
		want = max
	}
	return want
}

// wantNoLoadLeaf runs cmd — nil or not — and fails if any leaf
// produced a fileLoadedMsg: a dropped request issues no load.
func wantNoLoadLeaf(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	for _, msg := range leafMsgs(cmd) {
		if _, ok := msg.(fileLoadedMsg); ok {
			t.Fatalf("a dropped request issued a load: %#v", msg)
		}
	}
}

// r pressed while the startup load is in flight is dropped whole: no
// new request is minted, revision, reveal intent, and the visible
// presentation are exactly as they were, and the in-flight load's
// completion still performs the required first-match reveal — it is
// never misclassified as a reload owed anchor preservation (Issue #42).
func TestDroppedRDuringStartupLoadPreservesIntent(t *testing.T) {
	gate := newHeldLoads()
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{loadGate: gate.fn()})
	idx := navIndex(t, admitFiles)
	keyA := string(idx.Files[0].Path)
	cmdA := startBrowse(t, m, idx)
	workerA := runCmd(cmdA)
	<-gate.entered // the startup load is in flight and held

	reqA, seq0 := m.loading[keyA], m.loadSeq
	before := viewText(m)
	if !strings.Contains(before, "Loading…") {
		t.Fatalf("view = %q, want Loading… during the startup load", before)
	}

	// The dropped r mints nothing and mutates nothing: same live
	// request, same revision, same intents, same frame.
	_, dup := m.Update(keyR)
	wantNoLoadLeaf(t, dup)
	if got := m.loading[keyA]; got != reqA {
		t.Fatalf("dropped r re-minted the in-flight request %d as %d", reqA, got)
	}
	if m.loadSeq != seq0 {
		t.Fatalf("dropped r minted request identity %d, want the sequence still at %d", m.loadSeq, seq0)
	}
	if got := m.revs[keyA]; got != 0 {
		t.Fatalf("dropped r bumped the content revision to %d, want 0", got)
	}
	if m.pendingAnchor[keyA] {
		t.Fatal("dropped r recorded a reload-anchor intent")
	}
	if m.pendingReveals[keyA] {
		t.Fatal("dropped r recorded a reveal intent out of band")
	}
	if got := viewText(m); got != before {
		t.Fatalf("dropped r changed the presentation:\nbefore %q\nafter  %q", before, got)
	}

	// The in-flight load completes under its original classification:
	// not a reload, exactly one revision bump, the reveal intent
	// recorded, and the install commits the first-match reveal.
	close(gate.release)
	msg := <-workerA
	lm, ok := msg.(fileLoadedMsg)
	if !ok {
		t.Fatalf("startup completion = %#v, want fileLoadedMsg", msg)
	}
	if lm.reload {
		t.Fatal("the startup load's completion was misclassified as a reload")
	}
	_, lc := m.Update(msg)
	if got := m.revs[keyA]; got != 1 {
		t.Fatalf("revision = %d after the startup load, want 1 — a dropped r adds none", got)
	}
	if !m.pendingReveals[keyA] {
		t.Fatal("the startup completion recorded no reveal intent")
	}
	if m.pendingAnchor[keyA] {
		t.Fatal("the startup completion recorded a reload-anchor intent")
	}
	deliverLayout(t, m, lc)

	want := revealTop(t, m, keyA)
	if want == 0 {
		t.Fatal("the fixture's reveal leaves the top at 0 — it cannot distinguish reveal from anchor preservation")
	}
	if got := m.vps[keyA].Top(); got != want {
		t.Fatalf("top = %d after the startup load, want the first-match reveal's %d — anchor preservation would leave 0", got, want)
	}
	wantCurrentMatch(t, viewText(m), "a line")
}

// r pressed while a navigation load is in flight is dropped the same
// way: the destination's pending reveal survives untouched per the
// latest-target rules and commits when the load completes — never
// replaced by reload-anchor behaviour (Issue #42).
func TestDroppedRDuringNavigationLoadPreservesIntent(t *testing.T) {
	loads := newHeldNthLoad(2) // the navigation load is the second minted load
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{
		loadGate:   loads.fn(),
		popupTimer: popupStubTicks.popupTimer,
	})
	idx := navIndex(t, admitFiles)
	keyB := string(idx.Files[1].Path)
	finishLoad(t, m, startBrowse(t, m, idx)) // load 1: the startup read

	_, nav := m.Update(keyN) // cross to B: its load is minted and held
	batch, ok := nav().(tea.BatchMsg)
	if !ok || len(batch) < 1 {
		t.Fatalf("crossing batch = %#v, want the load leaf first", nav())
	}
	workerB := runCmd(batch[0])
	<-loads.entered // B's load is in flight and held

	// Expire the crossing's pop-up first — any key press dismisses it,
	// so the frame comparison below must not measure that dismissal.
	m.Update(popupExpireMsg{id: m.popupID})
	reqB, seq0 := m.loading[keyB], m.loadSeq
	if !m.pendingReveals[keyB] {
		t.Fatal("navigation to the uncached file recorded no reveal intent")
	}
	before := viewText(m)
	if !strings.Contains(before, "Loading…") {
		t.Fatalf("view = %q, want Loading… during the navigation load", before)
	}

	_, dup := m.Update(keyR)
	wantNoLoadLeaf(t, dup)
	if got := m.loading[keyB]; got != reqB {
		t.Fatalf("dropped r re-minted the in-flight request %d as %d", reqB, got)
	}
	if m.loadSeq != seq0 {
		t.Fatalf("dropped r minted request identity %d, want the sequence still at %d", m.loadSeq, seq0)
	}
	if got := m.revs[keyB]; got != 0 {
		t.Fatalf("dropped r bumped the content revision to %d, want 0", got)
	}
	if !m.pendingReveals[keyB] {
		t.Fatal("dropped r dropped the destination's pending reveal")
	}
	if m.pendingAnchor[keyB] {
		t.Fatal("dropped r recorded a reload-anchor intent")
	}
	if got := viewText(m); got != before {
		t.Fatalf("dropped r changed the presentation:\nbefore %q\nafter  %q", before, got)
	}
	if got := loads.calls.Load(); got != 2 {
		t.Fatalf("load calls = %d, want 2 — the dropped r started no worker", got)
	}

	// B's in-flight load completes under its original classification
	// and the pending destination reveal commits on install.
	close(loads.release)
	msg := <-workerB
	lm, ok := msg.(fileLoadedMsg)
	if !ok {
		t.Fatalf("navigation completion = %#v, want fileLoadedMsg", msg)
	}
	if lm.reload {
		t.Fatal("the navigation load's completion was misclassified as a reload")
	}
	_, lc := m.Update(msg)
	if got := m.revs[keyB]; got != 1 {
		t.Fatalf("revision = %d after the navigation load, want 1 — a dropped r adds none", got)
	}
	if !m.pendingReveals[keyB] {
		t.Fatal("the navigation completion dropped the pending reveal")
	}
	if m.pendingAnchor[keyB] {
		t.Fatal("the navigation completion recorded a reload-anchor intent")
	}
	deliverLayout(t, m, lc)

	want := revealTop(t, m, keyB)
	if got := m.vps[keyB].Top(); got != want {
		t.Fatalf("top = %d after the navigation load, want the destination reveal's %d", got, want)
	}
	wantCurrentMatch(t, viewText(m), "b line")
}

// An r accepted once the previous load settled applies the reload
// contract at once — the request is minted, the panel drops to
// "Loading…" — and at completion exactly one revision increment plus
// the anchor-preservation intent, never a reveal (Issue #42).
func TestAcceptedReloadAppliesFlagsIntentAndOneRevision(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, admitFiles)
	keyA := string(idx.Files[0].Path)
	finishLoad(t, m, startBrowse(t, m, idx))
	if got := m.revs[keyA]; got != 1 {
		t.Fatalf("revision = %d after the startup load, want 1", got)
	}
	seq0 := m.loadSeq

	_, cmd := m.Update(keyR)
	if cmd == nil {
		t.Fatal("accepted r issued no reload command")
	}
	if got := m.loading[keyA]; got == 0 || got <= seq0 {
		t.Fatalf("accepted r minted request %d, want a fresh identity above %d", got, seq0)
	}
	if m.loadSeq != seq0+1 {
		t.Fatalf("accepted r minted more than one request: seq = %d, want %d", m.loadSeq, seq0+1)
	}
	if v := viewText(m); !strings.Contains(v, "Loading…") {
		t.Fatalf("view = %q, want Loading… — the accepted reload's placeholder", v)
	}
	if got := m.revs[keyA]; got != 1 {
		t.Fatalf("the request alone bumped the revision to %d, want 1 — the bump belongs to completion", got)
	}

	msg, ok := fileLoadOf(cmd)
	if !ok {
		t.Fatal("the reload produced no completion")
	}
	if !msg.reload {
		t.Fatal("the accepted r's completion is not marked a reload")
	}
	_, lc := m.Update(msg)
	if got := m.revs[keyA]; got != 2 {
		t.Fatalf("revision = %d after the reload, want 2 — exactly one increment", got)
	}
	if !m.pendingAnchor[keyA] {
		t.Fatal("the undisturbed reload recorded no anchor intent")
	}
	if m.pendingReveals[keyA] {
		t.Fatal("the undisturbed reload recorded a reveal intent")
	}
	deliverLayout(t, m, lc)
	if v := viewText(m); strings.Contains(v, "Loading…") || !strings.Contains(v, "a line") {
		t.Fatalf("view = %q after the reload settled, want content", v)
	}
}

// Rapid repeated r presses while a reload is in flight keep at most
// one load in flight per path: every extra r is dropped without
// touching the live request, and the placeholder→content change stays
// the completion signal — only a settled load lets the next r mint
// (Issue #42).
func TestRapidRPressesKeepOneLoadInFlight(t *testing.T) {
	loads := newHeldNthLoad(2) // the reload is the second minted load
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{loadGate: loads.fn()})
	idx := navIndex(t, admitFiles)
	keyA := string(idx.Files[0].Path)
	finishLoad(t, m, startBrowse(t, m, idx))

	_, cmd := m.Update(keyR)
	if cmd == nil {
		t.Fatal("r issued no reload command")
	}
	worker := runCmd(cmd)
	<-loads.entered // the reload is in flight and held
	req := m.loading[keyA]

	for i := 0; i < 3; i++ {
		_, dup := m.Update(keyR)
		wantNoLoadLeaf(t, dup)
		if got := m.loading[keyA]; got != req {
			t.Fatalf("r press %d re-minted the in-flight request %d as %d", i, req, got)
		}
	}
	if got := loads.calls.Load(); got != 2 {
		t.Fatalf("load calls = %d, want 2 — repeated r presses started extra workers", got)
	}
	if v := viewText(m); !strings.Contains(v, "Loading…") {
		t.Fatalf("view = %q, want Loading… — the in-flight reload's placeholder", v)
	}

	// Settlement is the completion signal: the placeholder becomes
	// content and only then does r mint a new load.
	close(loads.release)
	_, lc := m.Update(<-worker)
	deliverLayout(t, m, lc)
	if _, ok := m.loading[keyA]; ok {
		t.Fatal("the settled reload still shows in flight")
	}
	if v := viewText(m); strings.Contains(v, "Loading…") || !strings.Contains(v, "a line") {
		t.Fatalf("view = %q after settlement, want content — the completion signal", v)
	}
	_, cmd2 := m.Update(keyR)
	if cmd2 == nil {
		t.Fatal("r after settlement issued no new load")
	}
	finishLoad(t, m, cmd2)
	if got := loads.calls.Load(); got != 3 {
		t.Fatalf("load calls = %d, want 3 — one reload per settled path", got)
	}
}

// Navigation re-entry is deliberately ungated: returning to a path
// whose load is still in flight drops only the duplicate load — the
// selection, the "Loading…" placeholder, and the reveal intent all
// update, and the in-flight load's completion commits the re-entry's
// reveal (Issue #42).
func TestReentryDuringInFlightLoadUpdatesSelectionAndIntent(t *testing.T) {
	gate := newHeldLoads()
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{
		loadGate:   gate.fn(),
		popupTimer: popupStubTicks.popupTimer,
	})
	idx := navIndex(t, admitFiles)
	keyA := string(idx.Files[0].Path)
	cmdA := startBrowse(t, m, idx)
	workerA := runCmd(cmdA)
	<-gate.entered // A's load is in flight and held

	_, nav := m.Update(keyN) // cross to B — its load is minted and held too
	batch, ok := nav().(tea.BatchMsg)
	if !ok || len(batch) < 1 {
		t.Fatalf("crossing batch = %#v, want the load leaf first", nav())
	}
	workerB := runCmd(batch[0])
	<-gate.entered

	// p re-enters A while its load is still in flight: the duplicate
	// load is dropped — no fileLoadedMsg leaf — but the cursor, the
	// placeholder, and the reveal intent all move to A.
	reqA := m.loading[keyA]
	_, navA := m.Update(keyP)
	wantNoLoadLeaf(t, navA)
	if cur, _ := m.idx.Cursor(); cur.File != 0 {
		t.Fatalf("re-entry left the cursor on file %d, want 0 — selection still updates", cur.File)
	}
	if got := m.loading[keyA]; got != reqA {
		t.Fatal("re-entry re-minted A's live request")
	}
	if v := viewText(m); !strings.Contains(v, "Loading…") {
		t.Fatalf("view = %q, want Loading… — A's load is still in flight", v)
	}
	if !m.pendingReveals[keyA] {
		t.Fatal("re-entry recorded no reveal intent for A")
	}
	if m.pendingAnchor[keyA] {
		t.Fatal("re-entry recorded a reload-anchor intent")
	}

	// A's original load completes under its own classification — not
	// a reload — and the re-entry's reveal commits against the install.
	close(gate.release)
	msg := <-workerA
	lm, ok := msg.(fileLoadedMsg)
	if !ok {
		t.Fatalf("A's completion = %#v, want fileLoadedMsg", msg)
	}
	if lm.reload {
		t.Fatal("A's in-flight load was misclassified as a reload")
	}
	_, lc := m.Update(msg)
	deliverLayout(t, m, lc)
	m.Update(<-workerB) // B's late completion caches it only

	want := revealTop(t, m, keyA)
	if got := m.vps[keyA].Top(); got != want {
		t.Fatalf("top = %d after re-entry, want the reveal's %d", got, want)
	}
	wantCurrentMatch(t, viewText(m), "a line")
}

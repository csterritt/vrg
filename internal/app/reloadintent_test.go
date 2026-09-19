package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/viewport"
)

// intentFiles is the reload-intent fixture: a.txt's two stops give
// same-file away-and-back room deep enough to scroll the match off
// screen; b.txt provides the cross-file leg.
var intentFiles = []navFile{
	{name: "a.txt", content: numberedContent("a", 60), stops: []navStop{
		{line: 10, start: 0, end: 1},
		{line: 50, start: 0, end: 1},
	}},
	{name: "b.txt", content: numberedContent("b", 40), stops: []navStop{
		{line: 20, start: 0, end: 1},
	}},
}

// An undisturbed reload records the anchor-preservation intent at
// completion and commits it against the new revision's matching
// layout — never a reveal: the held layout keeps the placeholder up
// and the saved viewport untouched until the install.
func TestReloadWithoutNavigationRecordsAnchorIntent(t *testing.T) {
	hold := newLayoutHold()
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{layoutGate: hold.fn()})
	idx := navIndex(t, intentFiles)
	keyA := string(idx.Files[0].Path)
	finishLoad(t, m, startBrowse(t, m, idx))

	for i := 0; i < 20; i++ {
		m.Update(keyDown)
	}
	anchor0 := m.vps[keyA].Anchor()
	top0 := m.vps[keyA].Top()
	hold.armed.Store(true)

	msg, ok := fileLoadOf(reloadCmd(t, m))
	if !ok {
		t.Fatal("the reload produced no completion")
	}
	_, lc := m.Update(msg)
	if !m.pendingAnchor[keyA] {
		t.Fatal("an undisturbed reload recorded no anchor intent")
	}
	if m.pendingReveals[keyA] {
		t.Fatal("an undisturbed reload recorded a reveal intent")
	}
	worker := runCmd(lc)
	<-hold.entered
	if got := m.vps[keyA].Top(); got != top0 {
		t.Fatalf("top moved to %d before the layout installed, want %d", got, top0)
	}
	close(hold.release)
	m.Update(<-worker)

	if m.pendingAnchor[keyA] {
		t.Fatal("the anchor intent survived its own commit")
	}
	rows := m.rows[keyA].(*viewport.Rows)
	if got, want := m.vps[keyA].Top(), rows.RowOf(anchor0); got != want {
		t.Fatalf("top after install = %d, want the anchor's row %d — no reveal runs", got, want)
	}
}

// Navigation during the load replaces the reload's anchor intent with
// the latest selection's reveal intent: the completion records no
// anchor intent at all, and the commit reveals the newest stop's
// target — navigation intent, not cursor equality, decides.
func TestReloadNavigationDuringLoadReplacesIntent(t *testing.T) {
	loads := newHeldNthLoad(2) // the reload is the second minted load
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{loadGate: loads.fn()})
	idx := navIndex(t, intentFiles)
	keyA := string(idx.Files[0].Path)
	finishLoad(t, m, startBrowse(t, m, idx)) // load 1 runs through

	_, cmd := m.Update(keyR)
	worker := runCmd(cmd)
	<-loads.entered // the reload is in flight and held

	m.Update(keyN) // navigation during the load: the reveal intent pends
	if !m.pendingReveals[keyA] {
		t.Fatal("navigation during the load recorded no reveal intent")
	}
	close(loads.release)
	_, lc := m.Update(<-worker)

	if m.pendingAnchor[keyA] {
		t.Fatal("the reload recorded the anchor intent despite navigation during the load")
	}
	if !m.pendingReveals[keyA] {
		t.Fatal("the reload completion dropped the reveal intent")
	}
	deliverLayout(t, m, lc)

	// The commit reveals the newest selection: line 50's row 49 lands
	// at 49 − 7 = 42 — never the pre-navigation position.
	rows := m.rows[keyA].(*viewport.Rows)
	want := rows.TargetRow(idx.Files[0].Stops[1]) - contentRows24/3
	if max := viewport.MaxTop(rows.Len(), contentRows24); want > max {
		want = max
	}
	if got := m.vps[keyA].Top(); got != want {
		t.Fatalf("commit top = %d, want the latest selection's %d", got, want)
	}
}

// A same-file away-and-back during the reload — n to the second stop
// then p home — leaves the cursor on its initial stop yet still
// counts as navigation: the entry reveal, not anchor preservation,
// commits. The match scrolled off-screen before r proves it: the
// committed viewport moves to the target's one-third placement.
func TestReloadAwayBackSameFileEntryReveal(t *testing.T) {
	loads := newHeldNthLoad(2)
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{loadGate: loads.fn()})
	idx := navIndex(t, intentFiles)
	keyA := string(idx.Files[0].Path)
	finishLoad(t, m, startBrowse(t, m, idx))
	cur0, _ := m.idx.Cursor()

	for i := 0; i < 30; i++ {
		m.Update(keyDown) // the match at row 9 scrolls off screen
	}

	_, cmd := m.Update(keyR)
	worker := runCmd(cmd)
	<-loads.entered // the reload is in flight and held

	m.Update(keyN) // away to stop 1 (line 50)
	m.Update(keyP) // and back — the cursor equals the initial stop
	if cur, _ := m.idx.Cursor(); cur != cur0 {
		t.Fatalf("cursor = %v, want the initial %v — away-and-back ends where it began", cur, cur0)
	}
	close(loads.release)
	_, lc := m.Update(<-worker)

	if m.pendingAnchor[keyA] {
		t.Fatal("the reload recorded the anchor intent despite away-and-back navigation")
	}
	if !m.pendingReveals[keyA] {
		t.Fatal("the away-and-back navigation left no reveal intent")
	}
	deliverLayout(t, m, lc)

	// The committed reveal moves the viewport to the target — row 9
	// hidden from the scrolled top lands at 9 − 7 = 2; anchor
	// preservation would have left the scrolled position in place.
	if got := m.vps[keyA].Top(); got != 2 {
		t.Fatalf("commit top = %d, want the entry reveal's 2 — the cursor-equality trap did not decide", got)
	}
}

// A cross-file away-and-back during the reload — A→B→A — shows
// "Loading…" on the return and applies the entry reveal on commit:
// the navigation during the load replaced the reload's anchor intent.
// The one-stop reloadFiles fixture crosses to b.txt on the first n.
func TestReloadAwayBackCrossFileEntryReveal(t *testing.T) {
	loads := newHeldNthLoad(2)
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{
		loadGate:   loads.fn(),
		popupTimer: popupStubTicks.popupTimer,
	})
	idx := navIndex(t, reloadFiles)
	keyA := string(idx.Files[0].Path)
	finishLoad(t, m, startBrowse(t, m, idx))

	for i := 0; i < 30; i++ {
		m.Update(keyDown) // the match at row 9 scrolls off screen
	}

	_, cmd := m.Update(keyR)
	worker := runCmd(cmd)
	<-loads.entered // A's reload is in flight and held

	finishLoad(t, m, navCmd(t, m, keyN)) // away to b.txt — load 3 runs free
	m.Update(keyP)                       // and back: re-entry during the reload is dropped
	if v := viewText(m); !strings.Contains(v, "Loading…") {
		t.Fatalf("view = %q on return to A, want Loading… — the reload is still in flight", v)
	}
	close(loads.release)
	_, lc := m.Update(<-worker)

	if m.pendingAnchor[keyA] {
		t.Fatal("the reload recorded the anchor intent despite the cross-file navigation")
	}
	if !m.pendingReveals[keyA] {
		t.Fatal("the return to A recorded no entry-reveal intent")
	}
	deliverLayout(t, m, lc)

	// A's stop at line 10 — row 9 — lands at 9 − 7 = 2.
	if got := m.vps[keyA].Top(); got != 2 {
		t.Fatalf("commit top = %d, want the entry reveal's 2", got)
	}
}

// Old- and new-revision layouts completing out of order commit the
// intent only against the new revision: the late old-revision layout
// is discarded without consuming the reveal intent navigation
// recorded during the load.
func TestReloadOutOfOrderRevisionsCommitReveal(t *testing.T) {
	loads := newHeldNthLoad(2)
	hold := newLayoutHold()
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{
		loadGate:   loads.fn(),
		layoutGate: hold.fn(),
	})
	idx := navIndex(t, intentFiles)
	keyA := string(idx.Files[0].Path)
	finishLoad(t, m, startBrowse(t, m, idx))
	installed := m.rows[keyA]
	hold.armed.Store(true)

	// Mint a revision-1 layout request — held.
	tw0 := m.rows[keyA].(*viewport.Rows).Key().TextWidth
	_, rc := m.Update(tea.WindowSizeMsg{Width: frameWidthFor(m, keyA, tw0+20), Height: 24})
	workerOld := runCmd(rc)
	<-hold.entered

	// The reload (held load), navigation during it, then the
	// completion minting the revision-2 request — also held.
	_, cmd := m.Update(keyR)
	loadWorker := runCmd(cmd)
	<-loads.entered
	m.Update(keyN) // navigation during the load: the reveal intent pends
	close(loads.release)
	_, lc := m.Update(<-loadWorker)
	workerNew := runCmd(lc)
	<-hold.entered

	// The old revision's layout arrives first — obsolete against
	// revision 2: discarded without consuming the pending intent.
	close(hold.release)
	m.Update(<-workerOld)
	if m.rows[keyA] != installed {
		t.Fatal("a stale-revision layout replaced the installed model")
	}
	if !m.pendingReveals[keyA] {
		t.Fatal("a late old-revision layout consumed the reveal intent")
	}

	// The new revision's matching layout installs and commits the
	// reveal — the only layout the intent may land on.
	m.Update(<-workerNew)
	if m.pendingReveals[keyA] {
		t.Fatal("the new revision's install did not commit the reveal intent")
	}
	rows := m.rows[keyA].(*viewport.Rows)
	if rows.Key().Revision != 2 {
		t.Fatalf("installed revision = %d, want 2", rows.Key().Revision)
	}
	want := rows.TargetRow(idx.Files[0].Stops[1]) - contentRows24/3
	if max := viewport.MaxTop(rows.Len(), contentRows24); want > max {
		want = max
	}
	if got := m.vps[keyA].Top(); got != want {
		t.Fatalf("commit top = %d, want the reveal's %d against revision 2", got, want)
	}
}

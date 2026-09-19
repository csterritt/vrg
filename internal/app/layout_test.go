package app

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
	"vrg/internal/theme"
	"vrg/internal/viewport"
)

// heldLayouts is the layout-preparation gate: every entering worker
// signals entered, then blocks until release closes. Buffering entered
// lets the test count in-flight workers without a dedicated reader.
type heldLayouts struct {
	entered chan struct{}
	release chan struct{}
}

func newHeldLayouts() *heldLayouts {
	return &heldLayouts{entered: make(chan struct{}, 16), release: make(chan struct{})}
}

func (h *heldLayouts) fn() func() {
	return func() {
		h.entered <- struct{}{}
		<-h.release
	}
}

// runCmd starts a gated command on a goroutine; its message arrives on
// the returned channel once the gate releases the worker. The model is
// never touched off the test goroutine — messages go through Update.
func runCmd(cmd tea.Cmd) <-chan tea.Msg {
	ch := make(chan tea.Msg, 1)
	go func() { ch <- cmd() }()
	return ch
}

// gateOpts couples the layout gate with the instant pop-up timer so
// navigation batches are safe to drive leaf by leaf.
func gateOpts(h *heldLayouts) options {
	return options{
		layoutGate: h.fn(),
		popupTimer: func(id int) tea.Cmd {
			return func() tea.Msg { return popupExpireMsg{id: id} }
		},
	}
}

// injectLoad feeds a fileLoadedMsg through Update and delivers the
// layout completion its preparation request produced. The fabricated
// completion stands in for the live request's answer, so it carries
// that request's identity — a message for a path with no request in
// flight is discarded exactly like a stale completion (Issue #25).
func injectLoad(t *testing.T, m *model, msg fileLoadedMsg) {
	t.Helper()
	msg.req = reqOf(m, msg.path)
	_, cmd := m.Update(msg)
	deliverLayout(t, m, cmd)
}

// frameWidthFor returns the terminal width at which path's cached
// buffer is laid out with text width tw under the current wrap mode —
// so a resize can target an exact layout key regardless of how long
// the fixture's temporary path is.
func frameWidthFor(m *model, path string, tw int) int {
	return frameWidthForG(m, m.bufs[path].GutterWidth(), tw)
}

// frameWidthForG is frameWidthFor with the gutter given explicitly —
// for sizing a frame before the file's buffer exists. Text width
// grows monotonically with frame width under the Issue #24 formula,
// so the first hit is the tightest frame reaching tw. The one-cell
// separator between the list and the panel is subtracted too (Issue
// #38): text width is terminal − list − separator − gutter −
// reserved indicator.
func frameWidthForG(m *model, gutter, tw int) int {
	ind := viewport.ReservedIndicator(m.wrap)
	for w := tw + gutter + ind; ; w++ {
		lw := fileListWidth(m.listWBase, w, gutter, ind)
		if !m.listVisible {
			lw = 0
		}
		if w-lw-1-gutter-ind == tw {
			return w
		}
	}
}

// gatedFiles is the responsiveness fixture: a.txt's three stops give
// same-file navigation room and b.txt provides the file crossing.
var gatedFiles = []navFile{
	{name: "a.txt", content: numberedContent("a", 60), stops: []navStop{
		{line: 10, start: 0, end: 1},
		{line: 30, start: 0, end: 1},
		{line: 50, start: 0, end: 1},
	}},
	{name: "b.txt", content: numberedContent("b", 40), stops: []navStop{
		{line: 35, start: 0, end: 1},
	}},
}

// crossFiles is the cached-file fixture: one stop per file so n/p
// cross directly between a.txt and b.txt; both stops sit deep enough
// that their reveals move the viewport.
var crossFiles = []navFile{
	{name: "a.txt", content: numberedContent("a", 60), stops: []navStop{
		{line: 40, start: 0, end: 1},
	}},
	{name: "b.txt", content: numberedContent("b", 40), stops: []navStop{
		{line: 35, start: 0, end: 1},
	}},
}

// While a layout worker is held, every AC6 input stays actionable:
// ctrl+c cancels with 130, q exits with the search-derived status, n/p
// move the cursor and preserve the reveal intent, w flips the wrap mode
// and issues a newly keyed request, and a further resize is accepted —
// none of it waits on the worker.
func TestGatedLayoutKeepsInputsResponsive(t *testing.T) {
	gate := newHeldLayouts()
	m := newTestModel(fakeChild{res: Result{Code: 0}}, gateOpts(gate))
	idx := navIndex(t, gatedFiles)
	keyA := string(idx.Files[0].Path)
	cmd := startBrowse(t, m, idx)

	msg, ok := fileLoadOf(cmd)
	if !ok {
		t.Fatal("load command produced no fileLoadedMsg")
	}
	_, layoutCmd := m.Update(msg)
	if layoutCmd == nil {
		t.Fatal("file load completion issued no layout request")
	}
	worker1 := runCmd(layoutCmd)
	<-gate.entered // the layout worker is now held mid-preparation

	// n and p move the cursor immediately; the reveal cannot run
	// against a missing layout, so the intent pends.
	for _, step := range []struct {
		key  tea.KeyPressMsg
		want int
	}{{keyN, 1}, {keyP, 0}, {keyN, 1}, {keyN, 2}} {
		m.Update(step.key)
		cur, _ := m.idx.Cursor()
		if cur.Stop != step.want {
			t.Fatalf("cursor stop = %d while layout held, want %d", cur.Stop, step.want)
		}
	}
	if !m.pendingReveals[keyA] {
		t.Fatal("reveal intent was not recorded while the layout was pending")
	}

	// w flips the wrap mode at once and issues a request for the new
	// mode's key without releasing the held worker.
	_, wcmd := m.Update(keyW)
	if wcmd == nil {
		t.Fatal("w issued no layout request")
	}
	if m.wrap {
		t.Fatal("w did not toggle wrap mode while a layout was held")
	}
	if got := m.layoutReqs[keyA]; got.Wrap {
		t.Fatalf("pending request key = %+v, want wrap mode off", got)
	}
	worker2 := runCmd(wcmd)
	<-gate.entered
	if len(gate.entered) != 0 {
		t.Fatal("w released the held worker")
	}

	// A second resize is accepted and issues a request under the new
	// parameters, still without releasing the gate.
	w := frameWidthFor(m, keyA, m.layoutReqs[keyA].TextWidth+20)
	_, rcmd := m.Update(tea.WindowSizeMsg{Width: w, Height: 24})
	if m.width != w {
		t.Fatalf("width = %d, want the accepted %d", m.width, w)
	}
	if rcmd == nil {
		t.Fatal("resize issued no layout request")
	}
	worker3 := runCmd(rcmd)
	<-gate.entered
	if len(gate.entered) != 0 {
		t.Fatal("the second resize released the held worker")
	}

	// Release every worker; the stale completions are discarded and
	// only the newest key installs, committing the pending reveal to
	// the newest stop — line 50's row revealed a third of the way down.
	close(gate.release)
	m.Update(<-worker1)
	m.Update(<-worker2)
	m.Update(<-worker3)

	rows, ok := m.rows[keyA].(*viewport.Rows)
	if !ok {
		t.Fatalf("no layout installed for %q", keyA)
	}
	want := rows.TargetRow(idx.Files[0].Stops[2]) - contentRows24/3
	if max := viewport.MaxTop(rows.Len(), contentRows24); want > max {
		want = max
	}
	if got := m.vps[keyA].Top(); got != want {
		t.Fatalf("top after install = %d, want the pending reveal at %d", got, want)
	}
	if m.pendingReveals[keyA] {
		t.Fatal("pending reveal intent survived its own commit")
	}

	// q exits with the fixed search-derived status even though further
	// workers could still be gated.
	wantStatus := m.status
	m.Update(keyQ)
	if !m.quitting || m.status != wantStatus {
		t.Fatalf("q while held: quitting=%v status=%d, want quitting with status %d",
			m.quitting, m.status, wantStatus)
	}
}

// ctrl+c while a layout worker is held cancels with 130 without
// waiting on the worker.
func TestGatedLayoutCtrlCExits130(t *testing.T) {
	gate := newHeldLayouts()
	m := newTestModel(fakeChild{res: Result{Code: 0}}, gateOpts(gate))
	idx := navIndex(t, gatedFiles)
	cmd := startBrowse(t, m, idx)

	msg, ok := fileLoadOf(cmd)
	if !ok {
		t.Fatal("load command produced no fileLoadedMsg")
	}
	_, layoutCmd := m.Update(msg)
	worker := runCmd(layoutCmd)
	<-gate.entered

	m.Update(keyCtrlC)
	if !m.quitting || m.status != 130 {
		t.Fatalf("ctrl+c while held: quitting=%v status=%d, want quitting with 130",
			m.quitting, m.status)
	}
	close(gate.release)
	<-worker
}

// Out-of-order completions: only the request matching the current
// parameters installs. Stale deliveries are discarded without touching
// the installed layout, the saved viewport, or the anchor.
func TestOutOfOrderLayoutCompletions(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, gatedFiles)
	finishLoad(t, m, startBrowse(t, m, idx))
	keyA := string(idx.Files[0].Path)
	installed80 := m.rows[keyA]

	for i := 0; i < 3; i++ {
		m.Update(keyDown)
	}
	anchor0 := m.vps[keyA].Anchor()
	vp0 := m.vps[keyA]

	// Three resizes mint three keyed requests; delivered out of order.
	// The widths are chosen for text widths distinct from each other
	// and from the installed key, whatever the fixture path's length.
	tw0 := installed80.(*viewport.Rows).Key().TextWidth
	_, c60 := m.Update(tea.WindowSizeMsg{Width: frameWidthFor(m, keyA, tw0+30), Height: 24})
	_, c40 := m.Update(tea.WindowSizeMsg{Width: frameWidthFor(m, keyA, tw0+15), Height: 24})
	_, c100 := m.Update(tea.WindowSizeMsg{Width: frameWidthFor(m, keyA, tw0+45), Height: 24})
	msg60, msg40, msg100 := c60(), c40(), c100()

	for _, stale := range []tea.Msg{msg40, msg60} {
		m.Update(stale)
		if m.rows[keyA] != installed80 {
			t.Fatalf("obsolete completion replaced the installed layout: %#v", stale)
		}
		if got := m.vps[keyA]; got != vp0 {
			t.Fatalf("obsolete completion moved the saved viewport to %+v", got)
		}
		if got := m.vps[keyA].Anchor(); got != anchor0 {
			t.Fatalf("obsolete completion changed the anchor to %+v", got)
		}
	}

	m.Update(msg100)
	rows, ok := m.rows[keyA].(*viewport.Rows)
	if !ok {
		t.Fatal("the current-key completion did not install")
	}
	if got, want := m.vps[keyA].Top(), rows.RowOf(anchor0); got != want {
		t.Fatalf("top after install = %d, want the anchor's row %d", got, want)
	}
	if got := m.vps[keyA].Anchor(); got != anchor0 {
		t.Fatalf("anchor after install = %+v, want %+v", got, anchor0)
	}
}

// Rapid wrap toggles: the request for a superseded mode is discarded on
// delivery, and toggling back to the mode still installed is the
// matching fast path — no redundant request is issued.
func TestRapidWrapToggleDiscardsSupersededLayout(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, gatedFiles)
	finishLoad(t, m, startBrowse(t, m, idx))
	keyA := string(idx.Files[0].Path)
	installed := m.rows[keyA]

	for i := 0; i < 3; i++ {
		m.Update(keyDown)
	}
	anchor0 := m.vps[keyA].Anchor()

	_, cOff := m.Update(keyW) // wrap off: new key, request issued
	if cOff == nil {
		t.Fatal("w issued no layout request for the toggled mode")
	}
	_, cOn := m.Update(keyW) // wrap back on: the installed layout matches
	if cOn != nil {
		t.Fatal("toggling back to the installed mode issued a redundant request")
	}

	m.Update(cOff()) // superseded: wrap is on again
	if m.rows[keyA] != installed {
		t.Fatal("a superseded completion replaced the installed layout")
	}
	if got := m.vps[keyA].Anchor(); got != anchor0 {
		t.Fatalf("superseded completion changed the anchor to %+v", got)
	}
	if !m.wrap {
		t.Fatal("wrap mode regressed after the completions")
	}
}

// A completion for a file that is no longer current — minted while it
// was current, superseded by a later parameter change — is discarded
// without touching the installed layout, the saved per-file viewport,
// or the visible panel; and an obsolete completion for the current
// file does not consume its pending reveal intent.
func TestObsoleteLayoutForOtherFileDiscarded(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{popupTimer: popupStubTicks.popupTimer})
	idx := navIndex(t, crossFiles)
	keyA, keyB := string(idx.Files[0].Path), string(idx.Files[1].Path)
	finishLoad(t, m, startBrowse(t, m, idx))

	// Visit B and install its layout at width 80, then mint a w70
	// request for B and leave before it completes.
	m.Update(keyN)
	buf, err := filebuffer.Load(idx.Files[1].Path, idx.Files[1].Stops)
	if err != nil {
		t.Fatal(err)
	}
	injectLoad(t, m, fileLoadedMsg{path: idx.Files[1].Path, buf: buf})
	installedB := m.rows[keyB]
	vpB := m.vps[keyB]

	// Mint a fresh-width request for B, then leave before it
	// completes. Widths are chosen for text widths distinct from each
	// other and from the installed keys, whatever the fixture path's
	// length.
	twB := installedB.(*viewport.Rows).Key().TextWidth
	_, cB70 := m.Update(tea.WindowSizeMsg{Width: frameWidthFor(m, keyB, twB+30), Height: 24})
	msgB70 := cB70()

	// Back to A: its layout is stale at the new width, so the return
	// reveal pends and a fresh request is issued — then a further
	// resize supersedes it.
	_, navCmd := m.Update(keyP)
	if !m.pendingReveals[keyA] {
		t.Fatal("returning to a stale-layout file did not pend the reveal")
	}
	var msgA70 tea.Msg
	for _, lm := range leafMsgs(navCmd) {
		if lr, ok := lm.(layoutReadyMsg); ok && lr.key.Path == keyA {
			msgA70 = lm
		}
	}
	if msgA70 == nil {
		t.Fatal("returning to a stale-layout file issued no layout request")
	}
	_, cA60 := m.Update(tea.WindowSizeMsg{Width: frameWidthFor(m, keyA, twB+15), Height: 24})
	msgA60 := cA60()

	// B's w70 completion is obsolete: B's current key wants w60.
	before := viewText(m)
	m.Update(msgB70)
	if m.rows[keyB] != installedB {
		t.Fatal("an obsolete completion replaced another file's installed layout")
	}
	if got := m.vps[keyB]; got != vpB {
		t.Fatalf("an obsolete completion moved file B's saved viewport to %+v", got)
	}
	if v := viewText(m); v != before {
		t.Fatal("an obsolete completion for a non-current file changed the visible frame")
	}

	// A's own w70 completion is obsolete too — the w60 request
	// superseded it — and the discard must not consume the pending
	// reveal intent.
	m.Update(msgA70)
	if !m.pendingReveals[keyA] {
		t.Fatal("an obsolete completion consumed the pending reveal intent")
	}

	// The w60 completion installs and the pending reveal commits: the
	// cursor's stop — a.txt's last, line 50 — lands a third down.
	m.Update(msgA60)
	if m.pendingReveals[keyA] {
		t.Fatal("the pending reveal intent was never committed")
	}
	rows := m.rows[keyA].(*viewport.Rows)
	want := rows.TargetRow(idx.Files[0].Stops[0]) - contentRows24/3
	if max := viewport.MaxTop(rows.Len(), contentRows24); want > max {
		want = max
	}
	if got := m.vps[keyA].Top(); got != want {
		t.Fatalf("top after install = %d, want the committed reveal at %d", got, want)
	}
}

// The content revision is part of the layout key: a completion minted
// under a superseded revision — the file reloaded while its layout was
// in flight — is obsolete and never installs.
func TestStaleRevisionCompletionDiscarded(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, crossFiles)
	finishLoad(t, m, startBrowse(t, m, idx))
	keyA := string(idx.Files[0].Path)
	installed := m.rows[keyA]

	// A second load of the same path bumps the revision; the request
	// it issues is keyed by revision 2. The request is minted the way
	// Issue #27's reload will mint it.
	buf, err := filebuffer.Load(idx.Files[0].Path, idx.Files[0].Stops)
	if err != nil {
		t.Fatal(err)
	}
	_, lc := m.Update(fileLoadedMsg{path: idx.Files[0].Path, req: mintRequest(m, idx.Files[0].Path), buf: buf})
	if m.revs[keyA] != 2 {
		t.Fatalf("revision = %d after reload, want 2", m.revs[keyA])
	}
	if got := m.layoutReqs[keyA].Revision; got != 2 {
		t.Fatalf("in-flight request revision = %d, want 2", got)
	}

	// A completion carrying the superseded revision is obsolete.
	stale := layoutReadyMsg{
		key:  viewport.Key{Path: keyA, Revision: 1, TextWidth: m.layoutReqs[keyA].TextWidth, Wrap: m.wrap},
		rows: viewport.Prepare(buf, viewport.Key{Path: keyA, Revision: 1}),
	}
	m.Update(stale)
	if m.rows[keyA] != installed {
		t.Fatal("a superseded-revision completion replaced the installed layout")
	}

	deliverLayout(t, m, lc)
	if got := m.rows[keyA].(*viewport.Rows).Key().Revision; got != 2 {
		t.Fatalf("installed revision = %d, want 2", got)
	}
}

// Navigating to a cached file whose installed layout is stale requests
// a fresh layout under the current parameters; the pending reveal
// commits when it installs.
func TestCachedFileStaleLayoutRequestsFresh(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{popupTimer: popupStubTicks.popupTimer})
	idx := navIndex(t, crossFiles)
	keyB := string(idx.Files[1].Path)
	finishLoad(t, m, startBrowse(t, m, idx))

	// Cache B's layout at width 80, then return to A and resize so
	// B's installed layout goes stale.
	m.Update(keyN)
	buf, err := filebuffer.Load(idx.Files[1].Path, idx.Files[1].Stops)
	if err != nil {
		t.Fatal(err)
	}
	injectLoad(t, m, fileLoadedMsg{path: idx.Files[1].Path, buf: buf})
	m.Update(keyP)
	tw := m.rows[keyB].(*viewport.Rows).Key().TextWidth + 30
	_, lc := m.Update(tea.WindowSizeMsg{Width: frameWidthFor(m, keyB, tw), Height: 24})
	deliverLayout(t, m, lc)

	// n crosses to B: no load is needed, but the stale layout pends
	// the reveal and requests a fresh preparation under the new width.
	_, navCmd := m.Update(keyN)
	want := m.layoutKey(keyB, m.bufs[keyB])
	if got := m.layoutReqs[keyB]; got != want {
		t.Fatalf("requested key = %+v, want current %+v", got, want)
	}
	if !m.pendingReveals[keyB] {
		t.Fatal("the destination reveal did not pend against the stale layout")
	}
	if v := viewText(m); !strings.Contains(v, "Loading…") {
		t.Fatal("a stale layout rendered instead of the placeholder")
	}

	// The fresh layout installs and commits the pending reveal.
	for _, lm := range leafMsgs(navCmd) {
		if lr, ok := lm.(layoutReadyMsg); ok && lr.key.Path == keyB {
			m.Update(lr)
		}
	}
	if m.pendingReveals[keyB] {
		t.Fatal("the pending reveal intent was never committed")
	}
	rows := m.rows[keyB]
	wantTop := rows.TargetRow(idx.Files[1].Stops[0]) - contentRows24/3
	if max := viewport.MaxTop(rows.Len(), contentRows24); wantTop > max {
		wantTop = max
	}
	if got := m.vps[keyB].Top(); got != wantTop {
		t.Fatalf("top after install = %d, want the committed reveal at %d", got, wantTop)
	}
}

// Navigating to a cached file whose installed layout already matches
// the current parameters is the immediate fast path: the reveal
// applies at once and no request is issued.
func TestCachedFileFreshLayoutFastPath(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{popupTimer: popupStubTicks.popupTimer})
	idx := navIndex(t, crossFiles)
	keyB := string(idx.Files[1].Path)
	finishLoad(t, m, startBrowse(t, m, idx))

	m.Update(keyN)
	buf, err := filebuffer.Load(idx.Files[1].Path, idx.Files[1].Stops)
	if err != nil {
		t.Fatal(err)
	}
	injectLoad(t, m, fileLoadedMsg{path: idx.Files[1].Path, buf: buf})
	m.Update(keyP)

	// B's layout is still current: the revisit reveals immediately and
	// no preparation request exists for it.
	m.Update(keyN)
	if _, ok := m.layoutReqs[keyB]; ok {
		t.Fatal("the fresh-layout fast path issued a redundant layout request")
	}
	if m.pendingReveals[keyB] {
		t.Fatal("the fresh-layout fast path pended a reveal that could run")
	}
	rows := m.rows[keyB]
	want := rows.TargetRow(idx.Files[1].Stops[0]) - contentRows24/3
	if max := viewport.MaxTop(rows.Len(), contentRows24); want > max {
		want = max
	}
	if got := m.vps[keyB].Top(); got != want {
		t.Fatalf("fast-path reveal top = %d, want %d", got, want)
	}
}

// The frame render materializes no paths at all: every file's display
// metadata — the escaped text, its grapheme-cluster boundaries, and
// its full cell width — is prepared once when the search completes
// (Issue #40), so View queries the escaper for nothing, not even the
// visible entries.
func TestRenderEscapesNoPaths(t *testing.T) {
	var escapes atomic.Int32
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{
		escapePath: func(p []byte) string {
			escapes.Add(1)
			return safepresentation.EscapePath(p)
		},
	})
	files := make([]fixtureFile, 50)
	for i := range files {
		files[i] = numberedFile(fmt.Sprintf("f%02d.txt", i), 5)
	}
	idx := browseIndex(t, files)
	finishLoad(t, m, startBrowse(t, m, idx))

	escapes.Store(0)
	viewText(m)
	if got := escapes.Load(); got != 0 {
		t.Fatalf("one frame escaped %d paths of 50, want 0 — path display metadata is prepared once at search completion",
			got)
	}
}

// A navigation Update plus the frame it produces performs no per-path
// work either: the counter spans both halves of the model transition
// and is never reset between Update and View, so a whole-index scan,
// copy, regroup, or whole-group reallocation fails whether it happens
// in the render or is merely moved into navigation handling.
func TestNavigateAndRenderEscapeNoPaths(t *testing.T) {
	var escapes atomic.Int32
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{
		escapePath: func(p []byte) string {
			escapes.Add(1)
			return safepresentation.EscapePath(p)
		},
		popupTimer: popupStubTicks.popupTimer,
	})
	files := make([]navFile, 50)
	for i := range files {
		files[i] = navFile{
			name:    fmt.Sprintf("f%02d.txt", i),
			content: "hit\n",
			stops:   []navStop{{line: 1, start: 0, end: 3}},
		}
	}
	idx := navIndex(t, files)
	finishLoad(t, m, startBrowse(t, m, idx))

	escapes.Store(0)
	for _, k := range []tea.KeyPressMsg{keyN, keyP, keyN} {
		m.Update(k)
		viewText(m)
	}
	if got := escapes.Load(); got != 0 {
		t.Fatalf("navigation plus render escaped %d paths, want 0 across the combined Update/View path", got)
	}
}

// Navigation moves only the current-file pointer: the per-file
// grouping is prepared once at search completion and shared by every
// step — n/p never regroups or reallocates it (Issue #40).
func TestNavigationKeepsPreparedGroups(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, popupStubTicks)
	idx := navIndex(t, navFiles)
	finishLoad(t, m, startBrowse(t, m, idx))

	base := &m.idx.Files[0]
	paths := &m.displayPaths[0]
	for i := 0; i < 4; i++ {
		m.Update(keyN)
		m.Update(keyP)
	}
	if got := &m.idx.Files[0]; got != base {
		t.Fatal("navigation reallocated the per-file grouping")
	}
	if got := &m.displayPaths[0]; got != paths {
		t.Fatal("navigation reallocated the prepared path metadata")
	}
}

// A resize that narrows the allotted list width re-truncates the
// visible entries against the new width at grapheme boundaries, and a
// gutter growth re-truncates them again — both with zero escaper
// queries across the Update plus the frame, because truncation
// consumes the prepared cluster boundaries (Issue #40).
func TestResizeRetruncatesFromPreparedPaths(t *testing.T) {
	var escapes atomic.Int32
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{
		escapePath: func(p []byte) string {
			escapes.Add(1)
			return safepresentation.EscapePath(p)
		},
	})
	m.theme = theme.Plain()
	idx := navIndex(t, []navFile{
		{name: "文文文文-deep-leaf.txt", content: "hit\n", stops: []navStop{{line: 1, start: 0, end: 3}}},
		{name: "b.txt", content: "hit\n", stops: []navStop{{line: 1, start: 0, end: 3}}},
	})
	finishLoad(t, m, startBrowse(t, m, idx))
	key := string(idx.Files[0].Path)
	wantEntry := func() string {
		got := leftTruncate(escapedPath(idx.Files[0]), m.listWidth())
		if !strings.HasPrefix(got, "…") {
			t.Fatalf("entry %q fits the %d-cell list — the fixture must exercise truncation", got, m.listWidth())
		}
		return got
	}

	// Narrow the frame so the 40% cap shrinks the list; the resulting
	// layout request stays in flight — re-truncation cannot wait on it.
	escapes.Store(0)
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 24})
	if got, want := viewText(m), wantEntry(); !strings.Contains(got, want) {
		t.Fatalf("view = %q, want the current entry re-truncated to %d cells as %q",
			got, m.listWidth(), want)
	}

	// Gutter growth shrinks the third term of the width formula: the
	// same frame re-truncates again, still querying nothing.
	m.rows[key] = &synthRows{countingRows: countingRows{n: 5}, gutter: 20}
	listW := m.listWidth()
	if got, want := viewText(m), wantEntry(); !strings.Contains(got, want) {
		t.Fatalf("view = %q after gutter growth, want the entry re-truncated to %d cells as %q",
			got, listW, want)
	}
	if got := escapes.Load(); got != 0 {
		t.Fatalf("resize and gutter growth plus renders escaped %d paths, want 0", got)
	}
}

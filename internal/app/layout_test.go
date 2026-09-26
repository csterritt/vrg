package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"vrg/internal/present"
	"vrg/internal/viewport"
)

// wantKey is the key a prepared layout must carry to install under the
// model's current parameters — the same formula the install guard
// applies.
func (m Model) wantKey(path string) viewport.Key {
	return viewport.Key{
		Path:  path,
		Rev:   m.revs[path],
		Width: m.wantTextW(),
		Wrap:  m.wrap,
	}
}

// longLineModel returns a settled browse model whose a.txt leads with
// a cells-wide single line followed by short tail lines — wrap width
// moves the rendered rows — with the viewport scrolled downs rendered
// rows into the file.
func longLineModel(t *testing.T, w, h, cells, downs int) Model {
	t.Helper()
	dir := t.TempDir()
	var sb strings.Builder
	sb.WriteString(strings.Repeat("x", cells) + "\n")
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&sb, "t%03d\n", i)
	}
	writeWorkFile(t, dir, "a.txt", sb.String())
	m, cmd := browseModel(t, dir, w, h,
		`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`,
		`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}`,
		`{"type":"summary","data":{}}`,
	)
	m = settle(t, m, cmd)
	for i := 0; i < downs; i++ {
		m, _ = update(t, m, codePress(tea.KeyDown))
	}
	return m
}

// twoFileLayoutModel returns a settled browse model over a.txt (one
// stop on line aStop) and b.txt (one stop on line bStop) — both files
// loaded and laid out at the current parameters — with the cursor back
// on a.txt's stop.
func twoFileLayoutModel(t *testing.T, w, h, aLines, aStop, bLines, bStop int) Model {
	t.Helper()
	dir := t.TempDir()
	var recs []string
	recs = append(recs, fileWithStops(t, dir, "a.txt", aLines, aStop)...)
	recs = append(recs, fileWithStops(t, dir, "b.txt", bLines, bStop)...)
	recs = append(recs, `{"type":"summary","data":{}}`)
	m, cmd := browseModel(t, dir, w, h, recs...)
	m = settle(t, m, cmd) // a.txt loads and installs
	var c tea.Cmd
	m, c = update(t, m, keyPress("n"))
	m = settle(t, m, c) // b.txt loads and installs
	m, c = update(t, m, keyPress("p"))
	m = settle(t, m, c) // back on a.txt
	return m
}

// A resize records the new dimensions and requests a prepared layout
// for the current file keyed by the post-resize parameters; it does
// not rewrap inline. Until the completion installs, the previous
// layout stays on screen; installing it lands the top on the row
// containing the anchor's text location.
func TestResizeRequestsLayoutOffUpdatePath(t *testing.T) {
	m := longLineModel(t, 80, 24, 2000, 15) // anchor (0, 1035), top 15

	var lay tea.Cmd
	m, lay = update(t, m, tea.WindowSizeMsg{Width: 100, Height: 24})
	if m.width != 100 {
		t.Fatalf("resize not recorded: width = %d", m.width)
	}
	if lay == nil {
		t.Fatal("resize issued no layout-preparation command")
	}
	msgs := cmdMsgs(lay)
	if len(msgs) != 1 {
		t.Fatalf("layout command produced %d messages, want 1", len(msgs))
	}
	lm, ok := msgs[0].(layoutDoneMsg)
	if !ok {
		t.Fatalf("layout command message = %T, want layoutDoneMsg", msgs[0])
	}
	if lm.key != m.wantKey("a.txt") {
		t.Fatalf("layout key = %+v, want the current parameters %+v", lm.key, m.wantKey("a.txt"))
	}

	// The pending request left the previous layout on screen.
	if m.vp.Top() != 15 {
		t.Fatalf("top while pending = %d, want the previous layout's 15", m.vp.Top())
	}
	m = pump(t, m, lm)
	// textW is 89 at width 100: cell 1035 sits on rendered row 11.
	if m.vp.Top() != 11 {
		t.Fatalf("top after install = %d, want 11 — the row containing the anchor", m.vp.Top())
	}
}

// With a layout request in flight — a worker the test simply has not
// invoked — every AC6 input stays actionable: a second resize is
// accepted, w flips the wrap mode and issues the keyed request for the
// new mode, and n/p move the cursor immediately with the reveal for
// the newest stop carried as a pending intent that commits when the
// matching layout installs. Superseded completions arriving later are
// discarded without touching the viewport or the anchor.
func TestLayoutWorkerHeldKeepsEveryInputActionable(t *testing.T) {
	dir := t.TempDir()
	recs := fileWithStops(t, dir, "a.txt", 300, 5, 200)
	recs = append(recs, `{"type":"summary","data":{}}`)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd) // a.txt loaded and laid out; stop a.txt:5

	var pending []tea.Cmd
	var c tea.Cmd
	m, c = update(t, m, tea.WindowSizeMsg{Width: 100, Height: 24})
	if c == nil {
		t.Fatal("resize issued no layout request")
	}
	pending = append(pending, c)

	// A second resize is accepted without waiting on the held worker.
	m, c = update(t, m, tea.WindowSizeMsg{Width: 60, Height: 24})
	if m.width != 60 {
		t.Fatalf("second resize not applied: width = %d", m.width)
	}
	pending = append(pending, c)

	// w flips the mode immediately and issues a request keyed for the
	// new mode — a third request while the first two stay held.
	m, c = update(t, m, keyPress("w"))
	if m.wrap {
		t.Fatal("w did not flip the wrap mode immediately")
	}
	if c == nil {
		t.Fatal("w issued no layout request for the new mode")
	}
	msgs := cmdMsgs(c)
	if len(msgs) != 1 {
		t.Fatalf("w's layout command produced %d messages, want 1", len(msgs))
	}
	lm, ok := msgs[0].(layoutDoneMsg)
	if !ok || lm.key != m.wantKey("a.txt") {
		t.Fatalf("w request = %T %+v, want layoutDoneMsg keyed %+v", msgs[0], lm.key, m.wantKey("a.txt"))
	}

	// n and p move the cursor immediately; the reveal cannot run
	// against the stale layout, so it is carried for the newest stop.
	m, _ = update(t, m, keyPress("n"))
	if s, _ := m.currentStop(); s.Line != 200 {
		t.Fatalf("n during pending layout selected %+v, want a.txt:200", s)
	}
	if m.vp.Top() != 0 {
		t.Fatalf("pending reveal moved the stale top to %d", m.vp.Top())
	}
	m, _ = update(t, m, keyPress("p"))
	if s, _ := m.currentStop(); s.Line != 5 {
		t.Fatalf("p during pending layout selected %+v, want a.txt:5", s)
	}

	// The matching completion installs and commits the newest stop's
	// reveal — a.txt:5 is on screen, so the top stays 0.
	m = pump(t, m, lm)
	if m.vp.Top() != 0 {
		t.Fatalf("top after install = %d, want 0 — the newest stop is visible", m.vp.Top())
	}
	// Reveals commit normally again: n lands line 200 a third down.
	m, _ = update(t, m, keyPress("n"))
	if m.vp.Top() != 192 {
		t.Fatalf("top after install + n = %d, want 192", m.vp.Top())
	}

	// The superseded completions are still in flight; both are
	// discarded without touching the installed layout or the anchor.
	top, anchor := m.vp.Top(), m.vp.Anchor()
	for _, c := range pending {
		for _, msg := range cmdMsgs(c) {
			m = pump(t, m, msg)
		}
	}
	if m.vp.Top() != top || m.vp.Anchor() != anchor {
		t.Fatalf("obsolete layouts moved the viewport: top %d→%d anchor %v→%v",
			top, m.vp.Top(), anchor, m.vp.Anchor())
	}
}

// ctrl+c while a layout request is in flight exits 130 without waiting
// on the worker; the held worker's late completion is discarded by the
// quit model.
func TestCtrlCWhileLayoutPendingExits130(t *testing.T) {
	m := longLineModel(t, 80, 24, 2000, 0)
	cancelled := false
	m.cancel = func() { cancelled = true }
	var lay tea.Cmd
	m, lay = update(t, m, tea.WindowSizeMsg{Width: 100, Height: 24})
	if lay == nil {
		t.Fatal("resize issued no layout request to hold")
	}
	m2, qc := update(t, m, ctrlCPress())
	if qc == nil {
		t.Fatal("ctrl+c during pending layout returned no command")
	}
	if _, ok := qc().(tea.QuitMsg); !ok {
		t.Fatalf("ctrl+c during pending layout command = %T, want tea.QuitMsg", qc())
	}
	if m2.ExitCode() != 130 || !cancelled {
		t.Fatalf("ctrl+c during pending layout: code=%d cancelled=%v", m2.ExitCode(), cancelled)
	}
	for _, msg := range cmdMsgs(lay) {
		m2 = pump(t, m2, msg)
	}
	if !m2.quit || m2.ExitCode() != 130 {
		t.Fatalf("late layout revived a cancelled UI: quit=%v code=%d", m2.quit, m2.ExitCode())
	}
}

// q while a layout request is in flight quits with the fixed status
// without waiting on the worker.
func TestQWhileLayoutPendingQuitsFixedStatus(t *testing.T) {
	m := longLineModel(t, 80, 24, 2000, 0)
	var lay tea.Cmd
	m, lay = update(t, m, tea.WindowSizeMsg{Width: 100, Height: 24})
	if lay == nil {
		t.Fatal("resize issued no layout request to hold")
	}
	m2, qc := update(t, m, keyPress("q"))
	if qc == nil {
		t.Fatal("q during pending layout returned no command")
	}
	if _, ok := qc().(tea.QuitMsg); !ok {
		t.Fatalf("q during pending layout command = %T, want tea.QuitMsg", qc())
	}
	if m2.ExitCode() != 0 {
		t.Fatalf("q during pending layout: code=%d, want the fixed 0", m2.ExitCode())
	}
	for _, msg := range cmdMsgs(lay) {
		m2 = pump(t, m2, msg)
	}
	if !m2.quit || m2.ExitCode() != 0 {
		t.Fatalf("late layout changed a quit UI: quit=%v code=%d", m2.quit, m2.ExitCode())
	}
}

// Resizes W1→W2→W3 mint one keyed request each; the workers completing
// out of order — W2, W1, W3 — install only W3's layout. The stale W2
// and W1 completions never become visible and never move the anchor.
func TestOutOfOrderLayoutCompletionsInstallNewestOnly(t *testing.T) {
	m := longLineModel(t, 80, 24, 2000, 15) // anchor (0, 1035), top 15
	anchor := m.vp.Anchor()

	var c1, c2, c3 tea.Cmd
	m, c1 = update(t, m, tea.WindowSizeMsg{Width: 100, Height: 24})
	m, c2 = update(t, m, tea.WindowSizeMsg{Width: 120, Height: 24})
	m, c3 = update(t, m, tea.WindowSizeMsg{Width: 90, Height: 24})

	// W2's completion is stale on arrival — the current parameters
	// are W3's. The previous layout stays installed.
	for _, msg := range cmdMsgs(c2) {
		m = pump(t, m, msg)
	}
	if m.vp.Top() != 15 || m.vp.Anchor() != anchor {
		t.Fatalf("stale W2 layout installed: top=%d anchor=%v", m.vp.Top(), m.vp.Anchor())
	}
	for _, msg := range cmdMsgs(c1) {
		m = pump(t, m, msg)
	}
	if m.vp.Top() != 15 || m.vp.Anchor() != anchor {
		t.Fatalf("stale W1 layout installed: top=%d anchor=%v", m.vp.Top(), m.vp.Anchor())
	}

	// W3 installs: textW is 79 at width 90, so cell 1035 lands on
	// rendered row 13 — the anchor's text location, not the old
	// ordinal and not a stale layout's mapping.
	for _, msg := range cmdMsgs(c3) {
		m = pump(t, m, msg)
	}
	if m.vp.Top() != 13 {
		t.Fatalf("top after W3 install = %d, want 13", m.vp.Top())
	}
	if m.vp.Anchor() != anchor {
		t.Fatalf("install moved the anchor to %v, want %v", m.vp.Anchor(), anchor)
	}
}

// Rapid wrap toggles w, w leave the mode back on; the off-mode
// completion arriving after the mode is restored is stale and never
// replaces the visible wrapped layout or moves the anchor.
func TestRapidWrapToggleDiscardsStaleMode(t *testing.T) {
	m := longLineModel(t, 80, 24, 2000, 15) // anchor (0, 1035), top 15
	before := m.View().Content
	anchor := m.vp.Anchor()

	var cOff tea.Cmd
	m, cOff = update(t, m, keyPress("w"))
	if m.wrap {
		t.Fatal("first w did not flip the wrap mode")
	}
	if cOff == nil {
		t.Fatal("w issued no layout request for run-off-edge mode")
	}
	var cOn tea.Cmd
	m, cOn = update(t, m, keyPress("w"))
	if !m.wrap {
		t.Fatal("second w did not restore wrap mode")
	}

	// The off-mode completion arrives after the mode is back on:
	// discarded — the wrapped frame is untouched.
	for _, msg := range cmdMsgs(cOff) {
		m = pump(t, m, msg)
	}
	if got := m.View().Content; got != before {
		t.Fatalf("stale off-mode layout changed the frame:\n%q", got)
	}
	if m.vp.Anchor() != anchor {
		t.Fatalf("stale layout moved the anchor to %v", m.vp.Anchor())
	}

	// An on-mode completion, if the toggle issued one, carries the
	// parameters the frame already shows and installs harmlessly.
	for _, msg := range cmdMsgs(cOn) {
		m = pump(t, m, msg)
	}
	if got := m.View().Content; got != before {
		t.Fatalf("on-mode layout changed the frame:\n%q", got)
	}
}

// A prepared layout for a file that is no longer current installs into
// the per-file cache only when its key still matches the current
// parameters — and never touches the visible panel, the viewport,
// saved per-file state, or the current file's pending reveal intent.
func TestLayoutForDepartedFileLeavesPanelAndSavedState(t *testing.T) {
	m := twoFileLayoutModel(t, 80, 24, 300, 5, 300, 200) // on a.txt:5
	m, _ = update(t, m, codePress(tea.KeyDown))
	m, _ = update(t, m, codePress(tea.KeyDown)) // a.txt anchor (2, 0)

	var c100, c120, cB tea.Cmd
	m, c100 = update(t, m, tea.WindowSizeMsg{Width: 100, Height: 24})
	m, c120 = update(t, m, tea.WindowSizeMsg{Width: 120, Height: 24})

	// Navigation to b.txt: its @80 layout is stale at width 120, so a
	// fresh request is issued and the entry reveal stays pending.
	m, cB = update(t, m, keyPress("n"))
	if cB == nil {
		t.Fatal("navigation to a stale-layout file issued no request")
	}
	if s, _ := m.currentStop(); string(s.Path) != "b.txt" || s.Line != 200 {
		t.Fatalf("stop after n = %+v, want b.txt:200", s)
	}
	before := m.View().Content
	savedA, savedB := m.saved["a.txt"], m.saved["b.txt"]

	// a.txt's completions arrive while b.txt is current: the
	// superseded @100 layout is discarded outright, the @120 layout
	// still matches the current parameters and caches — and neither
	// touches the panel or saved state.
	for _, msg := range cmdMsgs(c100) {
		m = pump(t, m, msg)
	}
	for _, msg := range cmdMsgs(c120) {
		m = pump(t, m, msg)
	}
	if got := m.View().Content; got != before {
		t.Fatalf("departed file's layouts changed the panel:\n%q", got)
	}
	if m.saved["a.txt"] != savedA || m.saved["b.txt"] != savedB {
		t.Fatalf("departed file's layouts touched saved state: %v", m.saved)
	}

	// b's pending intent survived both arrivals: installing its
	// matching layout commits the reveal — row 199 a third down —
	// over the saved (192, 0) anchor, which already shows it.
	for _, msg := range cmdMsgs(cB) {
		m = pump(t, m, msg)
	}
	if m.vp.Top() != 192 {
		t.Fatalf("top after b's install = %d, want the pending reveal's 192", m.vp.Top())
	}

	// The cached a.txt layout makes the revisit a fast path: no
	// request, the saved anchor restores, the on-screen stop does not
	// scroll.
	m, c := update(t, m, keyPress("p"))
	if c != nil {
		t.Fatalf("revisit to a current-layout file issued a command %T", c)
	}
	if s, _ := m.currentStop(); string(s.Path) != "a.txt" || s.Line != 5 {
		t.Fatalf("stop after p = %+v, want a.txt:5", s)
	}
	if m.vp.Top() != 2 {
		t.Fatalf("revisit top = %d, want the saved anchor's row 2", m.vp.Top())
	}
}

// A pending reveal intent is not consumed by obsolete completions: a
// stale layout arriving between the navigation and the matching
// install leaves the newest stop's reveal to commit against the
// installed rows.
func TestObsoleteLayoutDoesNotConsumePendingReveal(t *testing.T) {
	dir := t.TempDir()
	recs := fileWithStops(t, dir, "a.txt", 300, 5, 200)
	recs = append(recs, `{"type":"summary","data":{}}`)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)

	var c1, c2 tea.Cmd
	m, c1 = update(t, m, tea.WindowSizeMsg{Width: 100, Height: 24})
	m, c2 = update(t, m, tea.WindowSizeMsg{Width: 120, Height: 24})
	m, _ = update(t, m, keyPress("n")) // a.txt:200, reveal pending
	if s, _ := m.currentStop(); s.Line != 200 {
		t.Fatalf("n during pending layout selected %+v, want a.txt:200", s)
	}
	if m.vp.Top() != 0 {
		t.Fatalf("reveal committed against the stale layout: top = %d", m.vp.Top())
	}

	// The stale @100 completion arrives first: discarded, and the
	// intent is not consumed — the matching @120 install still
	// commits the reveal.
	for _, msg := range cmdMsgs(c1) {
		m = pump(t, m, msg)
	}
	for _, msg := range cmdMsgs(c2) {
		m = pump(t, m, msg)
	}
	if m.vp.Top() != 192 {
		t.Fatalf("top after installs = %d, want 192 — the pending reveal committed", m.vp.Top())
	}
}

// Navigation to a cached file whose installed layout is stale requests
// a prepared layout for the current parameters; the carried intents —
// the saved viewport and the entry reveal — commit when it installs.
func TestStaleLayoutNavigationCarriesSavedAndRevealIntent(t *testing.T) {
	m := twoFileLayoutModel(t, 80, 24, 300, 5, 300, 200) // on a.txt:5

	// Re-lay only the current file at width 100; b.txt's layout stays
	// stale at 80.
	var c1 tea.Cmd
	m, c1 = mustLayout(t, m, tea.WindowSizeMsg{Width: 100, Height: 24})
	m = settle(t, m, c1)
	// The saved-viewport intent must survive: a position that hides
	// the target so the Issue #14 reveal has to move it.
	m.saved["b.txt"] = viewport.Target{Line: 150}

	var cB tea.Cmd
	m, cB = update(t, m, keyPress("n"))
	if cB == nil {
		t.Fatal("navigation to a stale-layout file issued no layout request")
	}
	if s, _ := m.currentStop(); string(s.Path) != "b.txt" || s.Line != 200 {
		t.Fatalf("stop after n = %+v, want b.txt:200", s)
	}
	if v := m.View().Content; !strings.Contains(v, "── b.txt ") {
		t.Fatalf("panel did not switch to b.txt while pending: %q", v)
	}

	for _, msg := range cmdMsgs(cB) {
		m = pump(t, m, msg)
	}
	// Anchor 150 restored, then the reveal moved the hidden target to
	// a third down and replaced the saved state.
	if m.vp.Top() != 192 {
		t.Fatalf("top after install = %d, want the revealed 192", m.vp.Top())
	}
	if m.saved["b.txt"] != (viewport.Target{Line: 192}) {
		t.Fatalf("moving reveal left saved = %v, want (192, 0)", m.saved["b.txt"])
	}
}

// Navigation to a cached file whose installed layout already matches
// the current parameters is the fast path: no layout request is
// issued and the intents commit immediately.
func TestMatchingLayoutNavigationCommitsImmediately(t *testing.T) {
	m := twoFileLayoutModel(t, 80, 24, 300, 5, 300, 200) // on a.txt:5
	m, _ = update(t, m, codePress(tea.KeyDown))
	m, _ = update(t, m, codePress(tea.KeyDown)) // a.txt saved anchor (2, 0)

	var c tea.Cmd
	m, c = update(t, m, keyPress("n"))
	m = settle(t, m, c)
	m, c = update(t, m, keyPress("p"))
	// a.txt's installed layout still matches: no request, and the
	// commit already ran — anchor row 2 restored, the stop visible.
	if c != nil {
		t.Fatalf("matching-layout revisit issued a command %T", c)
	}
	if m.vp.Top() != 2 {
		t.Fatalf("revisit top = %d, want the saved anchor's row 2", m.vp.Top())
	}
}

// mustLayout applies one update and returns the command it produced —
// the layout request a stale current file must issue.
func mustLayout(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	m, cmd := update(t, m, msg)
	if cmd == nil {
		t.Fatalf("update %v issued no layout request", msg)
	}
	return m, cmd
}

// A frame render queries the file-list item provider only for the
// visible window — not once per file.
func TestRenderQueriesOnlyVisibleListEntries(t *testing.T) {
	dir := t.TempDir()
	var recs []string
	for i := 0; i < 200; i++ {
		name := fmt.Sprintf("f%03d.txt", i)
		recs = append(recs,
			fmt.Sprintf(`{"type":"begin","data":{"path":{"text":"%s"}}}`, name),
			fmt.Sprintf(`{"type":"match","data":{"path":{"text":"%s"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`, name),
			fmt.Sprintf(`{"type":"end","data":{"path":{"text":"%s"},"binary_offset":null}}`, name),
		)
	}
	recs = append(recs, `{"type":"summary","data":{}}`)
	writeWorkFile(t, dir, "f000.txt", "hit\n")
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)

	calls := 0
	m.listEntry = func(b []byte) string { calls++; return present.Path(b) }
	_ = m.View()
	if calls > m.height {
		t.Fatalf("frame queried the item provider %d times for %d visible rows", calls, m.height)
	}
	if calls == 0 {
		t.Fatal("frame never queried the item provider")
	}
	calls = 0
	_ = m.View()
	if calls > m.height {
		t.Fatalf("second frame queried the item provider %d times", calls)
	}
}

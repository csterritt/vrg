package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"vrg/internal/present"
	"vrg/internal/viewport"
)

// Stage one of a load completion installs the buffer, establishes the
// content revision, computes the final gutter, recomputes the text
// width, and requests the prepared layout — while performing no
// row-based decision: the viewport's top, anchor, and offset are
// untouched and the pending reveal intent is neither consumed nor
// mutated. The reveal commits only when the matching layout installs.
func TestLoadCompletionMakesNoRowDecision(t *testing.T) {
	dir := t.TempDir()
	recs := append(fileWithStops(t, dir, "a.txt", 300, 200), recSummary)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	if m.pending != intentReveal {
		t.Fatalf("pending after the search completion = %d, want the reveal intent", m.pending)
	}
	msgs := cmdMsgs(cmd)
	if len(msgs) != 1 {
		t.Fatalf("search completion produced %d messages, want the first file's load", len(msgs))
	}
	ld, ok := msgs[0].(loadDoneMsg)
	if !ok {
		t.Fatalf("load command message = %T, want loadDoneMsg", msgs[0])
	}

	// Stage 1: the completion stores the buffer, bumps the revision,
	// grows the gutter (300 lines → 3 digits), and recomputes the text
	// width — but the viewport does not move and the intent survives.
	m, lay := update(t, m, ld)
	if m.bufs["a.txt"] == nil {
		t.Fatal("the load completion did not install the buffer")
	}
	if m.revs["a.txt"] != 1 {
		t.Fatalf("content revision = %d, want 1", m.revs["a.txt"])
	}
	if m.textW != 68 {
		t.Fatalf("text width after the 3-digit gutter = %d, want 68", m.textW)
	}
	if m.vp.Top() != 0 || m.vp.Anchor() != (viewport.Target{}) || m.vp.Offset() != 0 {
		t.Fatalf("load completion made a row decision: top=%d anchor=%v offset=%d",
			m.vp.Top(), m.vp.Anchor(), m.vp.Offset())
	}
	if m.pending != intentReveal {
		t.Fatalf("load completion mutated the pending intent to %d", m.pending)
	}

	// The layout request is keyed to the current (path, revision, text
	// width, wrap mode) — the grown gutter's 68, not the placeholder's 70.
	lms := cmdMsgs(lay)
	if len(lms) != 1 {
		t.Fatalf("load completion produced %d follow-ups, want the layout request", len(lms))
	}
	lm, ok := lms[0].(layoutDoneMsg)
	if !ok {
		t.Fatalf("layout command message = %T, want layoutDoneMsg", lms[0])
	}
	if lm.key != m.wantKey("a.txt") {
		t.Fatalf("layout key = %+v, want the current parameters %+v", lm.key, m.wantKey("a.txt"))
	}
	if lm.key.Width != 68 {
		t.Fatalf("layout key width = %d, want the post-gutter 68", lm.key.Width)
	}

	// Stage 2: the matching install commits the reveal — the line-200
	// target row lands a third down at top 192.
	m = pump(t, m, lm)
	if m.vp.Top() != 192 {
		t.Fatalf("top after the matching install = %d, want 192", m.vp.Top())
	}
	if m.pending != intentNone {
		t.Fatalf("pending after the commit = %d, want intentNone", m.pending)
	}
	if v := m.View().Content; !strings.Contains(v, "\x1b[30;47;4mhit\x1b[24;37;40m00200") {
		t.Fatalf("committed view lacks the line-200 match: %q", v)
	}
}

// The reveal intent targets the newest selected stop — never a target
// captured when the load was requested. With the layout held after the
// load completes, n and p move the cursor immediately while the top
// stays put, and the install commits whichever target is newest.
func TestNavigationDuringLayoutGapCommitsNewestTarget(t *testing.T) {
	dir := t.TempDir()
	recs := append(fileWithStops(t, dir, "a.txt", 300, 5, 200), recSummary)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m, lay := update(t, m, cmdMsgs(cmd)[0]) // a.txt loaded; layout held

	// n and p move the cursor while the layout is pending; the top
	// cannot move — no matching rows are installed.
	m, _ = update(t, m, keyPress("n"))
	if s, _ := m.currentStop(); s.Line != 200 {
		t.Fatalf("n during the gap selected %+v, want a.txt:200", s)
	}
	m, _ = update(t, m, keyPress("p"))
	if s, _ := m.currentStop(); s.Line != 5 {
		t.Fatalf("p during the gap selected %+v, want a.txt:5", s)
	}
	m, _ = update(t, m, keyPress("n"))
	if s, _ := m.currentStop(); s.Line != 200 {
		t.Fatalf("second n during the gap selected %+v, want a.txt:200", s)
	}
	if m.vp.Top() != 0 {
		t.Fatalf("navigation during the gap moved the top to %d", m.vp.Top())
	}
	if m.pending != intentReveal {
		t.Fatalf("pending during the gap = %d, want the reveal intent", m.pending)
	}

	// The matching install reveals the newest target — line 200, not
	// the line-5 stop current when the load was requested.
	m = settle(t, m, lay)
	if m.vp.Top() != 192 {
		t.Fatalf("top after the install = %d, want the newest target's 192", m.vp.Top())
	}
	if v := m.View().Content; !strings.Contains(v, "\x1b[30;47;4mhit\x1b[24;37;40m00200") {
		t.Fatalf("committed view lacks the line-200 match: %q", v)
	}
}

// A resize arriving between the load completion and the layout
// install supersedes the request: the old-width layout is discarded
// without consuming the intent, and the reveal commits against the new
// width's row model — line 1's 300 cells wrap to five rows at text
// width 68 but four at 88, so the target's row and top differ.
func TestResizeBetweenStagesCommitsAtNewWidth(t *testing.T) {
	dir := t.TempDir()
	var sb strings.Builder
	sb.WriteString(strings.Repeat("x", 300) + "\n")
	for i := 2; i <= 300; i++ {
		fmt.Fprintf(&sb, "x%06d\n", i)
	}
	writeWorkFile(t, dir, "a.txt", sb.String())
	recs := append(fileRecs("a.txt",
		matchLineRec("a.txt", "x000050", 50, "x", 0, 1)), recSummary)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m, lay := update(t, m, cmdMsgs(cmd)[0]) // a.txt loaded; layout held at width 68

	// The resize lands before the layout does: a new request is minted
	// at width 88 and the held worker's completion is obsolete.
	m, lay2 := update(t, m, tea.WindowSizeMsg{Width: 100, Height: 24})
	if m.textW != 88 {
		t.Fatalf("text width after the resize = %d, want 88", m.textW)
	}
	if lay2 == nil {
		t.Fatal("resize during the gap issued no layout request")
	}
	for _, msg := range cmdMsgs(lay) {
		m = pump(t, m, msg) // the old-width layout: discarded
	}
	if m.pending != intentReveal {
		t.Fatal("an obsolete layout consumed the reveal intent")
	}
	if m.vp.Top() != 0 {
		t.Fatalf("obsolete layout moved the top to %d", m.vp.Top())
	}

	// The new width's install commits the reveal against its row
	// model: line 50 is rendered row 52 at width 88 (four wrap rows
	// for line 1) → top 45. The superseded width-68 layout would have
	// put it at row 53 → top 46.
	for _, msg := range cmdMsgs(lay2) {
		m = pump(t, m, msg)
	}
	if m.vp.Top() != 45 {
		t.Fatalf("top after the new-width install = %d, want 45 (row 52 a third down)", m.vp.Top())
	}
	if v := m.View().Content; !strings.Contains(v, "\x1b[30;47;4mx\x1b[24;37;40m000050") {
		t.Fatalf("committed view lacks the line-50 match: %q", v)
	}
}

// Hiding the file list between the stages is a text-width change: the
// superseded layout is discarded, the intent survives, and the reveal
// commits against the final width.
func TestListToggleBetweenStagesCommitsAtFinalWidth(t *testing.T) {
	dir := t.TempDir()
	var sb strings.Builder
	sb.WriteString(strings.Repeat("x", 300) + "\n")
	for i := 2; i <= 300; i++ {
		fmt.Fprintf(&sb, "x%06d\n", i)
	}
	writeWorkFile(t, dir, "a.txt", sb.String())
	recs := append(fileRecs("a.txt",
		matchLineRec("a.txt", "x000050", 50, "x", 0, 1)), recSummary)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m, lay := update(t, m, cmdMsgs(cmd)[0]) // layout held at width 68

	m, lay2 := update(t, m, codePress(tea.KeyTab))
	if m.listShow {
		t.Fatal("tab during the gap did not hide the file list")
	}
	if m.textW != 75 {
		t.Fatalf("text width after hiding the list = %d, want 75", m.textW)
	}
	if lay2 == nil {
		t.Fatal("the list toggle issued no layout request")
	}
	for _, msg := range cmdMsgs(lay) {
		m = pump(t, m, msg)
	}
	if m.pending != intentReveal || m.vp.Top() != 0 {
		t.Fatalf("obsolete layout touched the gap: pending=%d top=%d", m.pending, m.vp.Top())
	}

	// At width 75 the 300-cell line wraps to four rows: line 50 is row
	// 52 → top 45, not the width-68 layout's row 53 → top 46.
	for _, msg := range cmdMsgs(lay2) {
		m = pump(t, m, msg)
	}
	if m.vp.Top() != 45 {
		t.Fatalf("top after the final-width install = %d, want 45", m.vp.Top())
	}
}

// Obsolete layouts — wrong width, wrap mode, revision, or path — are
// discarded without consuming or mutating the reveal intent; the
// commit still lands when the matching layout installs.
func TestObsoleteLayoutsNeverConsumeTheIntent(t *testing.T) {
	dir := t.TempDir()
	recs := append(fileWithStops(t, dir, "a.txt", 300, 5, 200), recSummary)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m, lay := update(t, m, cmdMsgs(cmd)[0]) // layout held

	stale := []viewport.Key{
		{Path: "a.txt", Rev: 1, Width: m.textW + 5, Wrap: true},
		{Path: "a.txt", Rev: 1, Width: m.textW, Wrap: false},
		{Path: "a.txt", Rev: 9, Width: m.textW, Wrap: true},
		{Path: "gone.txt", Rev: 1, Width: m.textW, Wrap: true},
	}
	frame := m.View().Content
	for _, k := range stale {
		m = pump(t, m, layoutDoneMsg{key: k})
		if m.pending != intentReveal {
			t.Fatalf("obsolete layout %+v consumed the intent: %d", k, m.pending)
		}
	}
	if m.vp.Top() != 0 || m.View().Content != frame {
		t.Fatal("an obsolete layout changed the viewport or frame")
	}
	if _, ok := m.rows["a.txt"]; ok {
		t.Fatal("an obsolete layout installed into the cache")
	}

	m = settle(t, m, lay)
	if m.vp.Top() != 0 {
		t.Fatalf("top after the matching install = %d, want 0 — the visible line-5 target stays", m.vp.Top())
	}
	if m.pending != intentNone {
		t.Fatalf("pending after the commit = %d", m.pending)
	}
}

// A startup target hidden from top 0 lands at floor(content height/3)
// once the matching layout installs — and the first n then advances to
// the second stop.
func TestStartupHiddenTargetCommitsOnInstall(t *testing.T) {
	dir := t.TempDir()
	recs := append(fileWithStops(t, dir, "a.txt", 700, 500, 600), recSummary)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m, lay := update(t, m, cmdMsgs(cmd)[0])
	if m.vp.Top() != 0 {
		t.Fatalf("load completion moved the top to %d before the layout installed", m.vp.Top())
	}
	m = settle(t, m, lay)
	if m.vp.Top() != 492 {
		t.Fatalf("top after startup = %d, want 492 — row 499 at floor(23/3)", m.vp.Top())
	}

	m, _ = update(t, m, keyPress("n"))
	if s, _ := m.currentStop(); s.Line != 600 {
		t.Fatalf("first n selected %+v, want the second stop a.txt:600", s)
	}
	if m.vp.Top() != 592 {
		t.Fatalf("top after the first n = %d, want 592", m.vp.Top())
	}
}

// A startup target already visible from top 0 keeps the viewport at
// the top after both stages; the first n advances to the second stop.
func TestStartupVisibleTargetKeepsTopZero(t *testing.T) {
	dir := t.TempDir()
	recs := append(fileWithStops(t, dir, "a.txt", 300, 3, 8), recSummary)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m, lay := update(t, m, cmdMsgs(cmd)[0])
	if m.vp.Top() != 0 || m.pending != intentReveal {
		t.Fatalf("between the stages: top=%d pending=%d", m.vp.Top(), m.pending)
	}
	m = settle(t, m, lay)
	if m.vp.Top() != 0 {
		t.Fatalf("visible startup target scrolled to %d, want 0", m.vp.Top())
	}
	if _, ok := m.saved["a.txt"]; ok {
		t.Fatalf("no-scroll startup reveal wrote saved state: %v", m.saved)
	}

	m, _ = update(t, m, keyPress("n"))
	if s, _ := m.currentStop(); s.Line != 8 {
		t.Fatalf("first n selected %+v, want a.txt:8", s)
	}
	if m.vp.Top() != 0 {
		t.Fatalf("n to the visible line-8 stop scrolled to %d", m.vp.Top())
	}
}

// A revisit while the current file's load is pending starts from the
// saved per-file viewport: the commit leaves the top alone when the
// target is already inside it and moves it when it is not.
func TestSavedViewportRevisitCommitsOnLoad(t *testing.T) {
	dir := t.TempDir()
	var recs []string
	recs = append(recs, fileWithStops(t, dir, "a.txt", 300, 5, 200)...)
	recs = append(recs, fileWithStops(t, dir, "b.txt", 30, 2)...)
	recs = append(recs, recSummary)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)
	m, _ = update(t, m, keyPress("n")) // a.txt:200 → top 192, saved (192,0)
	if m.vp.Top() != 192 {
		t.Fatalf("setup top = %d, want 192", m.vp.Top())
	}

	// The reload empties the cache without touching the saved state;
	// away and back during the load restores the saved anchor as the
	// starting viewport.
	m, reload := update(t, m, keyPress("r"))
	m, _ = update(t, m, keyPress("n")) // → b.txt:2
	m, _ = update(t, m, keyPress("p")) // → a.txt:200
	if v := m.View().Content; !strings.Contains(v, "Loading…") {
		t.Fatalf("return to a reloading file lacks the placeholder: %q", v)
	}
	m, lay := update(t, m, cmdMsgs(reload)[0])
	if m.pending != intentReveal {
		t.Fatalf("pending after the completion = %d, want the entry reveal", m.pending)
	}
	m = settle(t, m, lay)
	// Row 199 sits inside the saved window 192..214: the reveal does
	// not move the restored top.
	if m.vp.Top() != 192 {
		t.Fatalf("revisit top = %d, want the saved 192 — the target was visible", m.vp.Top())
	}
	if m.saved["a.txt"] != (viewport.Target{Line: 192}) {
		t.Fatalf("no-scroll revisit changed saved to %v", m.saved["a.txt"])
	}
}

// The same revisit with a saved viewport that hides the target applies
// the one-third placement on commit.
func TestSavedViewportRevisitHiddenTargetMoves(t *testing.T) {
	dir := t.TempDir()
	var recs []string
	recs = append(recs, fileWithStops(t, dir, "a.txt", 300, 5, 200)...)
	recs = append(recs, fileWithStops(t, dir, "b.txt", 30, 2)...)
	recs = append(recs, recSummary)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)
	m, _ = update(t, m, keyPress("n")) // a.txt:200, top 192
	m.vp.SetTop(0)                     // the match scrolled off-screen

	m, reload := update(t, m, keyPress("r"))
	m, _ = update(t, m, keyPress("n")) // → b.txt:2, a.txt saved at (0,0)
	m, _ = update(t, m, keyPress("p")) // → a.txt:200 from the top
	m, lay := update(t, m, cmdMsgs(reload)[0])
	m = settle(t, m, lay)
	if m.vp.Top() != 192 {
		t.Fatalf("revisit top = %d, want the revealed 192 — the saved top hid the target", m.vp.Top())
	}
	if m.saved["a.txt"] != (viewport.Target{Line: 192}) {
		t.Fatalf("moving revisit reveal left saved = %v, want (192, 0)", m.saved["a.txt"])
	}
}

// An async terminator-only marker target reveals its row on commit and
// paints the marker cell: the `$` on "…hit\r\n" records bytes 298–300,
// maps to the end-of-line marker at display cell 298, and in
// run-off-edge mode the commit moves the offset so the marker paints
// as the last text cell.
func TestMarkerTargetCommitPaintsMarkerCell(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("x", 295) + "hit"
	var sb strings.Builder
	for i := 1; i <= 100; i++ {
		if i == 60 {
			sb.WriteString(long + "\r\n")
		} else {
			fmt.Fprintf(&sb, "x%06d\n", i)
		}
	}
	writeWorkFile(t, dir, "a.txt", sb.String())
	m := newModel(nil, nil)
	m.popupTimer = func(int) tea.Cmd { return nil }
	m.wrap = false // run-off-edge from startup
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	recs := append(fileRecs("a.txt",
		matchRecJSON("a.txt", long+"\r", 60, subJSON("\r\n", 298, 300))), recSummary)
	m, cmd := update(t, m, searchDoneMsg{index: fixtureIndex(t, dir, recs...)})
	m, lay := update(t, m, cmdMsgs(cmd)[0])
	if m.vp.Top() != 0 || m.vp.Offset() != 0 || m.pending != intentReveal {
		t.Fatalf("between the stages: top=%d offset=%d pending=%d",
			m.vp.Top(), m.vp.Offset(), m.pending)
	}

	m = settle(t, m, lay)
	// Row 59 → top 52; the marker at cell 298 → offset 298 + 1 − 67.
	if m.vp.Top() != 52 {
		t.Fatalf("top after the marker commit = %d, want 52", m.vp.Top())
	}
	want := 299 - m.wantTextW()
	if m.vp.Offset() != want {
		t.Fatalf("offset after the marker commit = %d, want %d = 299 − %d",
			m.vp.Offset(), want, m.wantTextW())
	}
	r, ok := visRow(m, 59)
	if !ok {
		t.Fatal("the marker's row is not visible after the commit")
	}
	if len(r.Spans) != 1 || r.Spans[0] != (present.Span{Start: 66, End: 66}) {
		t.Fatalf("marker row spans = %+v, want the marker at window cell 66", r.Spans)
	}
	if v := m.View().Content; !strings.Contains(v, "hit\x1b[30;47;4m \x1b[24;37;40m") {
		t.Fatalf("committed view lacks the painted marker cell: %q", v)
	}
}

// An async standalone-cluster target commits the Issue #43 fallback
// cell: n during the layout gap selects the stop whose recorded
// submatch is a combining mark with no base, and the install's minimal
// horizontal reveal paints the one-cell fallback flush with the right
// edge.
func TestClusterTargetCommitPaintsFallbackCell(t *testing.T) {
	dir := t.TempDir()
	l2 := strings.Repeat("x", 300) + "\x01" + "́"
	writeWorkFile(t, dir, "a.txt", "hit\n"+l2+"\n")
	m := newModel(nil, nil)
	m.popupTimer = func(int) tea.Cmd { return nil }
	m.wrap = false // run-off-edge from startup
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	recs := append(fileRecs("a.txt",
		matchLineRec("a.txt", "hit", 1, "hit", 0, 3),
		matchRecJSON("a.txt", l2, 2, subJSON("́", 301, 303))), recSummary)
	m, cmd := update(t, m, searchDoneMsg{index: fixtureIndex(t, dir, recs...)})
	m, lay := update(t, m, cmdMsgs(cmd)[0]) // loaded; layout held

	// The selection moves while the layout is pending: the reveal is
	// owed to the newest stop — the standalone mark on line 2.
	m, _ = update(t, m, keyPress("n"))
	if s, _ := m.currentStop(); s.Line != 2 {
		t.Fatalf("n during the gap selected %+v, want a.txt:2", s)
	}
	if m.vp.Offset() != 0 {
		t.Fatalf("offset moved before the install: %d", m.vp.Offset())
	}

	m = settle(t, m, lay)
	// The recorded mark maps to its own fallback cell at cell 302 —
	// after the ^A escape's cells 300–301 — so the expanded start
	// cell is 302 and the offset paints it at the window's last
	// column.
	want := 303 - m.wantTextW()
	if m.vp.Offset() != want {
		t.Fatalf("offset after the cluster commit = %d, want %d = 303 − %d",
			m.vp.Offset(), want, m.wantTextW())
	}
	r, ok := visRow(m, 1)
	if !ok {
		t.Fatal("line 2 has no visible row after the commit")
	}
	if len(r.Cells) != m.wantTextW() || r.Cells[m.wantTextW()-1].Text != "◌́" {
		t.Fatalf("line-2 row's last cells = %+v, want the ◌́ fallback cell at the right edge",
			r.Cells[max(0, len(r.Cells)-3):])
	}
}

// A load completion and a layout completion for a file that is no
// longer current touch only that path's cache: the current file's
// panel is byte-identical and its pending intent is untouched.
func TestNonCurrentCompletionLeavesPanelUntouched(t *testing.T) {
	dir := t.TempDir()
	var recs []string
	recs = append(recs, fileWithStops(t, dir, "a.txt", 40, 1)...)
	recs = append(recs, fileWithStops(t, dir, "b.txt", 60, 30)...)
	recs = append(recs, recSummary)
	m, cmdA := browseModel(t, dir, 80, 24, recs...)

	// Cross to b.txt: its load issues, the reveal intent is owed to it.
	m, cmdB := update(t, m, keyPress("n"))
	if s, _ := m.currentStop(); string(s.Path) != "b.txt" || s.Line != 30 {
		t.Fatalf("stop after n = %+v, want b.txt:30", s)
	}
	frame := m.View().Content
	top, off := m.vp.Top(), m.vp.Offset()

	// a.txt's load completes while b.txt is current — stage 1 updates
	// only a.txt's cache.
	m = pump(t, m, cmdMsgs(cmdA)[0])
	if m.bufs["a.txt"] == nil || m.revs["a.txt"] != 1 {
		t.Fatal("a.txt's late completion did not fill its cache")
	}
	if got := m.View().Content; got != frame {
		t.Fatalf("a.txt's completion changed b.txt's panel:\n%q", got)
	}
	if m.pending != intentReveal || m.vp.Top() != top || m.vp.Offset() != off {
		t.Fatal("a.txt's completion touched the viewport or the intent")
	}

	// A prepared layout for a.txt — keyed to its current revision —
	// caches without touching the panel or the intent either.
	m = pump(t, m, layoutDoneMsg{
		key:  viewport.Key{Path: "a.txt", Rev: 1, Width: m.textW, Wrap: m.wrap},
		rows: viewport.Prepare(m.bufs["a.txt"], m.wantKey("a.txt")),
	})
	if got := m.View().Content; got != frame {
		t.Fatalf("a.txt's layout completion changed b.txt's panel:\n%q", got)
	}
	if m.pending != intentReveal {
		t.Fatal("a.txt's layout completion consumed b.txt's intent")
	}
	if inst, ok := m.rows["a.txt"]; !ok || inst.key.Width != m.wantTextW() {
		t.Fatal("a.txt's layout did not cache for its parameters")
	}

	// b.txt's own stages commit normally.
	m = pump(t, m, cmdMsgs(cmdB)[0])
	if m.vp.Top() != 22 {
		t.Fatalf("top after b.txt's stages = %d, want the revealed 22", m.vp.Top())
	}
}

// The file-change pop-up is independent of both stages: opened at
// selection time, it survives the destination's load completion and
// layout install unchanged.
func TestPopupUnaffectedByCompletionStages(t *testing.T) {
	dir := t.TempDir()
	var recs []string
	recs = append(recs, fileWithStops(t, dir, "a.txt", 40, 1)...)
	recs = append(recs, fileWithStops(t, dir, "b.txt", 60, 30)...)
	recs = append(recs, recSummary)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)

	m, load := update(t, m, keyPress("n")) // → b.txt:30, pop-up opens
	if m.popupID == 0 {
		t.Fatal("crossing to b.txt did not open the pop-up")
	}
	id := m.popupID

	// Neither stage dismisses or restarts it: the load completion …
	m, lay := update(t, m, cmdMsgs(load)[0])
	if m.popupID != id {
		t.Fatalf("the load completion changed the pop-up instance %d → %d", id, m.popupID)
	}
	// … and the layout install that commits the reveal.
	m = settle(t, m, lay)
	if m.popupID != id {
		t.Fatalf("the layout install changed the pop-up instance %d → %d", id, m.popupID)
	}
	if _, _, _, inner := popupBox(t, ansi.Strip(m.View().Content)); !strings.Contains(inner, "b.txt") {
		t.Fatalf("pop-up gone or wrong after both stages: %q", inner)
	}
	if m.vp.Top() != 22 {
		t.Fatalf("the committed reveal under the pop-up: top=%d, want 22", m.vp.Top())
	}
}

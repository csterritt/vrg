package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"vrg/internal/filebuffer"
	"vrg/internal/viewport"
)

// Issue #42: the load-admission boundary is atomic — the
// one-load-per-path check and every reload-state mutation form a
// single decision point inside reload(). An r dropped while the
// current path's load is in flight commits nothing: the in-flight
// request's identity, the content revision, the pending intent, and
// the presentation are exactly what they were, so the startup or
// navigation load completes under its original classification — the
// destination reveal — never misclassified as a reload's anchor
// preservation. The boundary applies only to the explicit reread:
// navigation re-entry legitimately mutates selection, placeholder,
// and intent even when its load request is the one dropped.

// r during the startup load is dropped whole: no command, the
// in-flight request's identity intact, no reread mark, the reveal
// intent and the frame untouched — and the load's own completion then
// runs the first-visit destination reveal rather than anchor
// preservation.
func TestDroppedReloadDuringStartupLoadKeepsRevealIntent(t *testing.T) {
	dir := t.TempDir()
	recs := append(fileWithStops(t, dir, "a.txt", 300, 200), recSummary)
	m, load := browseModel(t, dir, 80, 24, recs...)
	req := m.loading["a.txt"]
	if req == 0 || load == nil {
		t.Fatal("a.txt's startup load was not issued in flight")
	}
	if m.pending != intentReveal {
		t.Fatalf("pending before r = %d, want the first-load reveal", m.pending)
	}
	seq, frame := m.loadSeq, m.View().Content

	m, dropped := update(t, m, keyPress("r"))
	if dropped != nil {
		t.Fatalf("r during the startup load produced a command %T — dropped, not queued", dropped)
	}
	if m.loading["a.txt"] != req {
		t.Fatalf("dropped r replaced the in-flight request %d with %d",
			req, m.loading["a.txt"])
	}
	if m.loadSeq != seq {
		t.Fatalf("dropped r minted request identities: loadSeq %d → %d", seq, m.loadSeq)
	}
	if m.reloading["a.txt"] {
		t.Fatal("dropped r marked a.txt's first load a reread")
	}
	if m.pending != intentReveal {
		t.Fatalf("dropped r changed the pending intent to %d", m.pending)
	}
	if m.revs["a.txt"] != 0 {
		t.Fatalf("dropped r touched a.txt's revision: %d", m.revs["a.txt"])
	}
	if got := m.View().Content; got != frame {
		t.Fatalf("dropped r changed the frame:\n%q", got)
	}

	// The original request's completion installs under its own
	// classification: the line-200 target reveals at one-third
	// placement — top 192 — not anchor-preserved at top 0.
	m = settle(t, m, load)
	if m.revs["a.txt"] != 1 {
		t.Fatalf("revision after the first load = %d, want 1", m.revs["a.txt"])
	}
	if m.vp.Top() != 192 {
		t.Fatalf("top after the startup load = %d, want the revealed 192", m.vp.Top())
	}
	if v := m.View().Content; !strings.Contains(v, "\x1b[30;47;4mhit\x1b[24;37;40m00200") {
		t.Fatalf("completed view lacks the revealed line-200 match: %q", v)
	}
}

// r during a navigation load is dropped the same way: b.txt's
// in-flight request, the zero revision, the pending destination
// reveal, and the panel all survive untouched, and the load's
// completion applies the latest-target reveal — the one-third
// placement — rather than the reload-anchor keep.
func TestDroppedReloadDuringNavigationLoadKeepsRevealIntent(t *testing.T) {
	dir := t.TempDir()
	var recs []string
	recs = append(recs, fileWithStops(t, dir, "a.txt", 300, 5)...)
	recs = append(recs, fileWithStops(t, dir, "b.txt", 300, 200)...)
	recs = append(recs, recSummary)
	m, loadA := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, loadA)

	// n lands on b.txt's hidden target: its load issues in flight
	// with the destination reveal pended behind it.
	m, loadB := update(t, m, keyPress("n"))
	if s, _ := m.currentStop(); string(s.Path) != "b.txt" || s.Line != 200 {
		t.Fatalf("n selected %+v, want b.txt:200", s)
	}
	if m.pending != intentReveal {
		t.Fatalf("pending after n = %d, want the destination reveal", m.pending)
	}
	reqB := m.loading["b.txt"]
	if reqB == 0 || loadB == nil {
		t.Fatal("b.txt's navigation load was not issued in flight")
	}
	// Any key dismisses the file-change pop-up: a placeholder no-op
	// scroll clears it so the dropped-r frame comparison measures the
	// panel alone.
	m, _ = update(t, m, codePress(tea.KeyUp))
	seq, frame := m.loadSeq, m.View().Content

	m, dropped := update(t, m, keyPress("r"))
	if dropped != nil {
		t.Fatalf("r during b.txt's load produced a command %T", dropped)
	}
	if m.loading["b.txt"] != reqB {
		t.Fatalf("dropped r replaced b.txt's in-flight request %d with %d",
			reqB, m.loading["b.txt"])
	}
	if m.loadSeq != seq {
		t.Fatalf("dropped r minted request identities: loadSeq %d → %d", seq, m.loadSeq)
	}
	if m.reloading["b.txt"] {
		t.Fatal("dropped r marked b.txt's navigation load a reread")
	}
	if m.pending != intentReveal {
		t.Fatalf("dropped r replaced the destination reveal with intent %d", m.pending)
	}
	if m.revs["b.txt"] != 0 {
		t.Fatalf("dropped r touched b.txt's revision: %d", m.revs["b.txt"])
	}
	if got := m.View().Content; got != frame {
		t.Fatalf("dropped r changed the frame:\n%q", got)
	}

	// b.txt's completion reveals its target: top 192, not the
	// first-visit anchor's top-of-file.
	m = settle(t, m, loadB)
	if m.revs["b.txt"] != 1 {
		t.Fatalf("b.txt revision after its first load = %d, want 1", m.revs["b.txt"])
	}
	if m.vp.Top() != 192 {
		t.Fatalf("top after b.txt's load = %d, want the revealed 192 — not the anchor-preserved 0", m.vp.Top())
	}
	if v := m.View().Content; !strings.Contains(v, "\x1b[30;47;4mhit\x1b[24;37;40m00200") {
		t.Fatalf("completed view lacks the revealed line-200 match: %q", v)
	}
}

// An r admitted once the previous load settled applies the whole
// reload state at once — the minted in-flight request, the reread
// mark, the dropped buffer's "Loading…" panel — issues exactly one
// worker, and its completion records the anchor intent behind exactly
// one revision increment.
func TestAcceptedReloadAppliesReloadStateOnce(t *testing.T) {
	dir := t.TempDir()
	recs := append(fileWithStops(t, dir, "a.txt", 300, 200), recSummary)
	m, load := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, load) // revision 1, revealed top 192
	m.vp.SetTop(30)        // the match scrolled off-screen; anchor (30,0)

	m, reload := update(t, m, keyPress("r"))
	if m.loading["a.txt"] == 0 {
		t.Fatal("admitted r minted no in-flight request")
	}
	if !m.reloading["a.txt"] {
		t.Fatal("admitted r did not mark the load a reread")
	}
	if m.bufs["a.txt"] != nil {
		t.Fatal("admitted r kept the cached buffer — the panel must read Loading…")
	}
	if v := m.View().Content; !strings.Contains(v, "Loading…") {
		t.Fatalf("admitted r's panel = %q, want Loading…", v)
	}
	msgs := cmdMsgs(reload)
	if len(msgs) != 1 {
		t.Fatalf("admitted r produced %d messages, want the single reread", len(msgs))
	}
	if _, ok := msgs[0].(loadDoneMsg); !ok {
		t.Fatalf("admitted r's command = %T, want loadDoneMsg", msgs[0])
	}

	m, lay := update(t, m, msgs[0])
	if m.revs["a.txt"] != 2 {
		t.Fatalf("revision after the reread = %d, want exactly one increment to 2", m.revs["a.txt"])
	}
	if m.pending != intentAnchor {
		t.Fatalf("pending after the undisturbed reread = %d, want the anchor intent", m.pending)
	}
	m = settle(t, m, lay)
	if m.vp.Top() != 30 || m.vp.Anchor() != (viewport.Target{Line: 30}) {
		t.Fatalf("anchor after install: top=%d anchor=%v, want the preserved 30/(30,0)",
			m.vp.Top(), m.vp.Anchor())
	}
}

// Rapid repeat presses while a reread is in flight keep at most one
// load per path: every duplicate drops — the in-flight identity and
// the reread mark untouched, no queued worker — and the placeholder's
// change to content, or to "(unreadable)" on failure, remains the
// only completion signal.
func TestRepeatedReloadKeepsSingleInFlightLoad(t *testing.T) {
	dir := t.TempDir()
	recs := append(fileWithStops(t, dir, "a.txt", 20, 1), recSummary)
	failing := false
	read := func(p []byte) ([]byte, error) {
		if failing {
			return nil, errRead
		}
		return filebuffer.ReadFile(p)
	}
	m, load := loaderModel(t, dir, 80, 24, read, recs...)
	m = settle(t, m, load)

	m, first := update(t, m, keyPress("r"))
	req := m.loading["a.txt"]
	if req == 0 || first == nil || !m.reloading["a.txt"] {
		t.Fatal("the first r was not admitted as a reread")
	}
	seq := m.loadSeq
	for i := 0; i < 3; i++ {
		var dup tea.Cmd
		m, dup = update(t, m, keyPress("r"))
		if dup != nil {
			t.Fatalf("repeat r %d produced a command %T — nothing queues behind the reread", i+2, dup)
		}
	}
	if m.loading["a.txt"] != req || m.loadSeq != seq || !m.reloading["a.txt"] {
		t.Fatalf("repeat r disturbed the in-flight reread: req %d→%d seq %d→%d mark %v",
			req, m.loading["a.txt"], seq, m.loadSeq, m.reloading["a.txt"])
	}
	if v := m.View().Content; !strings.Contains(v, "Loading…") {
		t.Fatalf("panel during the reread = %q, want Loading…", v)
	}

	// Placeholder → content: the one in-flight reread settles it.
	m = settle(t, m, first)
	if m.revs["a.txt"] != 2 || m.bufs["a.txt"] == nil || m.loading["a.txt"] != 0 {
		t.Fatalf("settled reread: revs=%d cached=%v loading=%d",
			m.revs["a.txt"], m.bufs["a.txt"] != nil, m.loading["a.txt"])
	}
	if v := ansi.Strip(m.View().Content); !strings.Contains(v, "hit00001") {
		t.Fatalf("post-reread view = %q, want the content", v)
	}

	// Placeholder → (unreadable): a failing reread under the same
	// duplicate pressure settles through the failure path.
	failing = true
	m, second := update(t, m, keyPress("r"))
	req = m.loading["a.txt"]
	if req == 0 || second == nil {
		t.Fatal("the second reread was not admitted")
	}
	for i := 0; i < 2; i++ {
		var dup tea.Cmd
		m, dup = update(t, m, keyPress("r"))
		if dup != nil {
			t.Fatalf("repeat r during the failing reread produced %T", dup)
		}
	}
	if m.loading["a.txt"] != req {
		t.Fatal("repeat r replaced the failing reread's request")
	}
	m = settle(t, m, second)
	if !m.failed["a.txt"] || m.loading["a.txt"] != 0 {
		t.Fatalf("failed reread: failed=%v loading=%d", m.failed["a.txt"], m.loading["a.txt"])
	}
	if got := contentRow1(t, m); !strings.Contains(got, "(unreadable)") {
		t.Fatalf("failed-reread panel = %q, want (unreadable)", got)
	}
}

// Navigation re-entry is deliberately ungated: crossing away and back
// while a.txt's load is still in flight mints nothing for the
// duplicate — but the selection returns, the panel is the Loading…
// placeholder, and the destination reveal re-pends, so the original
// request's completion reveals the newest target.
func TestNavigationReentryDuringInFlightLoadIsUngated(t *testing.T) {
	dir := t.TempDir()
	var recs []string
	recs = append(recs, fileWithStops(t, dir, "a.txt", 300, 200)...)
	recs = append(recs, fileWithStops(t, dir, "b.txt", 20, 5)...)
	recs = append(recs, recSummary)
	m, loadA := browseModel(t, dir, 80, 24, recs...)
	reqA := m.loading["a.txt"]
	if reqA == 0 || loadA == nil {
		t.Fatal("a.txt's startup load was not issued in flight")
	}

	m, _ = update(t, m, keyPress("n"))
	m, back := update(t, m, keyPress("p"))
	if back != nil {
		t.Fatalf("re-entry to loading a.txt produced a command %T", back)
	}
	if s, _ := m.currentStop(); string(s.Path) != "a.txt" || s.Line != 200 {
		t.Fatalf("p during a.txt's load selected %+v, want a.txt:200", s)
	}
	if m.loading["a.txt"] != reqA {
		t.Fatalf("re-entry replaced a.txt's in-flight request %d with %d",
			reqA, m.loading["a.txt"])
	}
	if m.pending != intentReveal {
		t.Fatalf("re-entry left pending=%d, want the destination reveal", m.pending)
	}
	if v := m.View().Content; !strings.Contains(v, "Loading…") || !strings.Contains(v, "── a.txt ") {
		t.Fatalf("re-entered panel = %q, want a.txt's Loading… placeholder", v)
	}

	// The original request's completion installs under the reveal the
	// re-entry owed: one-third placement at top 192.
	m = settle(t, m, loadA)
	if m.vp.Top() != 192 {
		t.Fatalf("top after a.txt's load = %d, want the revealed 192", m.vp.Top())
	}
	if v := m.View().Content; !strings.Contains(v, "\x1b[30;47;4mhit\x1b[24;37;40m00200") {
		t.Fatalf("completed view lacks the revealed line-200 match: %q", v)
	}
}

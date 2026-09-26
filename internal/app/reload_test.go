package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"vrg/internal/filebuffer"
	"vrg/internal/viewport"
)

// r rereads the current file once — no rg rerun, no cursor or stop
// change — while the panel drops to "Loading…" behind a filename row
// that still names the path; the completion installs the new bytes.
func TestExplicitReloadRereadsCurrentFile(t *testing.T) {
	dir := t.TempDir()
	recs := append(fileWithStops(t, dir, "a.txt", 40, 1), recSummary)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)

	// New bytes on disk: the reload, not the search, observes them.
	var b strings.Builder
	for i := 1; i <= 40; i++ {
		fmt.Fprintf(&b, "v2-%04d\n", i)
	}
	writeWorkFile(t, dir, "a.txt", b.String())

	stops := len(m.stops)
	m, reload := update(t, m, keyPress("r"))
	if reload == nil {
		t.Fatal("r returned no command — the current file's reread is owed")
	}
	if s, _ := m.currentStop(); string(s.Path) != "a.txt" || s.Line != 1 {
		t.Fatalf("r moved the cursor to %+v, want a.txt:1", s)
	}
	if len(m.stops) != stops {
		t.Fatalf("r changed the stop list %d → %d", stops, len(m.stops))
	}
	v := m.View().Content
	if !strings.Contains(v, "Loading…") {
		t.Fatalf("reloading panel lacks the placeholder: %q", v)
	}
	if !strings.Contains(v, "── a.txt ") {
		t.Fatalf("filename row stopped identifying the path during reload: %q", v)
	}
	if strings.Contains(v, "x000001") {
		t.Fatalf("old content still painted during the reload: %q", v)
	}

	// Exactly one reread of the path — nothing else.
	msgs := cmdMsgs(reload)
	if len(msgs) != 1 {
		t.Fatalf("r produced %d messages, want the single load completion", len(msgs))
	}
	ld, ok := msgs[0].(loadDoneMsg)
	if !ok || string(ld.path) != "a.txt" {
		t.Fatalf("r's command = %T %v, want a.txt's loadDoneMsg", msgs[0], msgs[0])
	}

	m = pump(t, m, ld)
	v = ansi.Strip(m.View().Content)
	if strings.Contains(v, "x000001") || !strings.Contains(v, "v2-0020") {
		t.Fatalf("post-reload view = %q, want the new bytes", v)
	}
}

// A duplicate r — or a re-entry — while the path's load is in flight
// is dropped, not queued: no second worker, no waiting request, the
// in-flight identity unchanged; the placeholder's change is the only
// completion signal, after which r starts a new load.
func TestReloadDuplicateDroppedNotQueued(t *testing.T) {
	dir := t.TempDir()
	var recs []string
	recs = append(recs, fileWithStops(t, dir, "a.txt", 20, 1)...)
	recs = append(recs, fileWithStops(t, dir, "b.txt", 20, 5)...)
	recs = append(recs, recSummary)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)

	gate := make(chan struct{})
	m.loadGate = gate
	m, reload := update(t, m, keyPress("r"))
	req := m.loading["a.txt"]
	if req == 0 || reload == nil {
		t.Fatal("r issued no reload request")
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- reload() }()
	select {
	case <-done:
		t.Fatal("the reload completed while its gate was held")
	case <-time.After(50 * time.Millisecond):
	}

	// The duplicate is dropped: no command, no second identity.
	m, dup := update(t, m, keyPress("r"))
	if dup != nil {
		t.Fatalf("duplicate r produced a command %T — nothing is queued", dup)
	}
	if m.loading["a.txt"] != req {
		t.Fatalf("duplicate r replaced the in-flight request %d with %d",
			req, m.loading["a.txt"])
	}

	// Re-entry is dropped the same way: away to b.txt and back issues
	// b's load on the way out but nothing for a.txt on return.
	m, cmdB := update(t, m, keyPress("n"))
	if cmdB == nil {
		t.Fatal("crossing to b.txt issued no load")
	}
	m, again := update(t, m, keyPress("p"))
	if again != nil {
		t.Fatalf("re-entry to a reloading path produced a command %T", again)
	}
	if m.loading["a.txt"] != req {
		t.Fatalf("re-entry replaced a.txt's in-flight request %d with %d",
			req, m.loading["a.txt"])
	}
	if v := m.View().Content; !strings.Contains(v, "Loading…") {
		t.Fatalf("re-entered reloading panel = %q, want Loading…", v)
	}

	// The placeholder's change is the completion signal: release the
	// gate and the in-flight request installs; only now does r mint a
	// fresh request.
	close(gate)
	m = pump(t, m, <-done)
	m = settle(t, m, cmdB)
	m, second := update(t, m, keyPress("r"))
	if second == nil {
		t.Fatal("r after settlement issued no new load")
	}
	msgs := cmdMsgs(second)
	if len(msgs) != 1 {
		t.Fatalf("second r produced %d messages, want one reread", len(msgs))
	}
	if _, ok := msgs[0].(loadDoneMsg); !ok {
		t.Fatalf("second r's command = %T, want loadDoneMsg", msgs[0])
	}
}

// The reload keeps the logical anchor: the completion carries no
// reveal, the gap paints no stale rows, and once the new revision's
// matching layout installs the effective top lands on the row
// containing the same text location.
func TestReloadPreservesAnchorThroughMatchingLayout(t *testing.T) {
	dir := t.TempDir()
	recs := append(fileWithStops(t, dir, "a.txt", 60, 1), recSummary)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)
	for i := 0; i < 30; i++ {
		m, _ = update(t, m, codePress(tea.KeyDown))
	}
	anchor := m.vp.Anchor()
	if anchor != (viewport.Target{Line: 30}) || m.vp.Top() != 30 {
		t.Fatalf("pre-reload anchor %v top %d, want (30, 0)/30", anchor, m.vp.Top())
	}

	// Same-length rewrite: new bytes, same row geometry.
	var b strings.Builder
	for i := 1; i <= 60; i++ {
		fmt.Fprintf(&b, "v2-%04d\n", i)
	}
	writeWorkFile(t, dir, "a.txt", b.String())

	m, reload := update(t, m, keyPress("r"))
	msgs := cmdMsgs(reload)
	if len(msgs) != 1 {
		t.Fatalf("r produced %d messages, want the single load completion", len(msgs))
	}
	m, lay := update(t, m, msgs[0])
	if m.revs["a.txt"] != 2 {
		t.Fatalf("a.txt revision = %d, want the reload's 2", m.revs["a.txt"])
	}
	// Until the new revision's layout installs the superseded one
	// cannot paint: the panel shows no stale rows.
	if v := ansi.Strip(m.View().Content); strings.Contains(v, "x0000") {
		t.Fatalf("pre-install frame paints stale content: %q", v)
	}
	// The anchor intent commits only when the matching layout
	// installs: the same text location returns as the effective top.
	m = settle(t, m, lay)
	if m.vp.Anchor() != anchor {
		t.Fatalf("anchor after reload = %v, want %v", m.vp.Anchor(), anchor)
	}
	if m.vp.Top() != 30 {
		t.Fatalf("top after reload = %d, want the anchor row 30", m.vp.Top())
	}
	if v := ansi.Strip(m.View().Content); !strings.Contains(v, "v2-0031") {
		t.Fatalf("reloaded view lacks the new bytes at the anchor row: %q", v)
	}
}

// An anchor past the shrunken content clamps to the last valid top —
// the documented lossy clamp — and the anchor rewrites to the clamped
// location.
func TestReloadAnchorClampsToShrunkContent(t *testing.T) {
	dir := t.TempDir()
	recs := append(fileWithStops(t, dir, "a.txt", 100, 1), recSummary)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)
	for i := 0; i < 50; i++ {
		m, _ = update(t, m, codePress(tea.KeyDown))
	}
	if m.vp.Anchor() != (viewport.Target{Line: 50}) {
		t.Fatalf("pre-reload anchor %v, want (50, 0)", m.vp.Anchor())
	}

	writeWorkFile(t, dir, "a.txt", "short\nfile\n")
	m, reload := update(t, m, keyPress("r"))
	m = settle(t, m, reload)

	if m.vp.Top() != 0 || m.vp.Anchor() != (viewport.Target{}) {
		t.Fatalf("shrunk reload: top=%d anchor=%v, want 0/(0,0)",
			m.vp.Top(), m.vp.Anchor())
	}
	if v := ansi.Strip(m.View().Content); !strings.Contains(v, "short") || strings.Contains(v, "x0000") {
		t.Fatalf("shrunk reload view = %q", v)
	}
}

// A failed reload replaces the old display: the panel reads
// "(unreadable)", the current-file failure overlay opens, and none of
// the previous revision's text survives on screen.
func TestFailedReloadReplacesContent(t *testing.T) {
	dir := t.TempDir()
	recs := append(fileWithStops(t, dir, "a.txt", 20, 1), recSummary)
	failing := false
	read := func(p []byte) ([]byte, error) {
		if failing {
			return nil, errRead
		}
		return filebuffer.ReadFile(p)
	}
	m, cmd := loaderModel(t, dir, 80, 24, read, recs...)
	m = settle(t, m, cmd)

	failing = true
	m, reload := update(t, m, keyPress("r"))
	if v := m.View().Content; !strings.Contains(v, "Loading…") {
		t.Fatalf("retry panel = %q, want Loading… during the reread", v)
	}
	m = settle(t, m, reload)

	if got := contentRow1(t, m); !strings.Contains(got, "(unreadable)") {
		t.Fatalf("failed reload panel = %q, want (unreadable)", got)
	}
	if m.overlay == nil {
		t.Fatal("failed reload did not open the failure overlay")
	}
	if v := m.View().Content; !strings.Contains(v, "cannot read a.txt: denied") {
		t.Fatalf("overlay lacks the reload failure: %q", v)
	}
	m, _ = pressKey(t, m, "esc")
	if v := ansi.Strip(m.View().Content); strings.Contains(v, "x0000") || strings.Contains(v, "hit000") {
		t.Fatalf("stale content survived the failed reload: %q", v)
	}
}

// The second consecutive r failure appends exactly one new diagnostic
// occurrence to the open overlay — the reader's scroll position
// preserved — and collects exactly one more for the exit replay.
func TestReloadSecondFailureAppendsPreservingScroll(t *testing.T) {
	dir := t.TempDir()
	recs := append(fileWithStops(t, dir, "a.txt", 10, 1), recSummary)
	// A tall diagnostic keeps the overlay scrollable so the reader can
	// be mid-file when the second occurrence lands.
	var sb strings.Builder
	sb.WriteString("denied")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&sb, "\ncontext %02d", i)
	}
	tall := errors.New(sb.String())
	failing := false
	read := func(p []byte) ([]byte, error) {
		if failing {
			return nil, tall
		}
		return filebuffer.ReadFile(p)
	}
	m, cmd := loaderModel(t, dir, 80, 24, read, recs...)
	m = settle(t, m, cmd)

	failing = true
	m, reload := update(t, m, keyPress("r"))
	m = settle(t, m, reload) // first reload failure → overlay opens
	if got := overlayOccurrences(m, "cannot read a.txt"); got != 1 {
		t.Fatalf("overlay holds %d occurrences after the first failure, want 1", got)
	}
	m, _ = pressKey(t, m, "esc")

	// r re-opens the prior-failure overlay — the re-entry sequence's
	// retry presentation — and issues exactly one new load.
	m, retry := update(t, m, keyPress("r"))
	if m.overlay == nil {
		t.Fatal("r on the failed file did not re-open the prior-failure overlay")
	}
	if got := overlayOccurrences(m, "cannot read a.txt"); got != 1 {
		t.Fatalf("re-opened overlay holds %d occurrences, want the single prior failure", got)
	}
	if m.loading["a.txt"] == 0 {
		t.Fatal("r on the failed file minted no retry request")
	}
	for i := 0; i < 5; i++ {
		m, _ = pressKey(t, m, "down")
	}
	if m.overlay.scroll != 5 {
		t.Fatalf("overlay scroll = %d, want the reader 5 rows down", m.overlay.scroll)
	}
	before := len(m.diags)

	m = settle(t, m, retry)
	if m.overlay == nil {
		t.Fatal("the overlay closed on the second failure")
	}
	if m.overlay.scroll != 5 {
		t.Fatalf("append moved the reader's scroll to %d, want 5", m.overlay.scroll)
	}
	if got := overlayOccurrences(m, "cannot read a.txt"); got != 2 {
		t.Fatalf("overlay holds %d failure occurrences, want 2", got)
	}
	if len(m.diags) != before+1 {
		t.Fatalf("second failure collected %d diagnostics, want one", len(m.diags)-before)
	}
}

// With a one-stop index n and p are strict no-ops and r is the only
// retry route: after the first load fails, r re-reads the file and the
// content installs.
func TestReloadIsTheOneStopRetryRoute(t *testing.T) {
	dir := t.TempDir()
	recs := append(fileWithStops(t, dir, "a.txt", 10, 1), recSummary)
	failing := true
	read := func(p []byte) ([]byte, error) {
		if failing {
			return nil, errRead
		}
		return filebuffer.ReadFile(p)
	}
	m, cmd := loaderModel(t, dir, 80, 24, read, recs...)
	m = settle(t, m, cmd) // first load fails
	if got := contentRow1(t, m); !strings.Contains(got, "(unreadable)") {
		t.Fatalf("failed one-stop panel = %q, want (unreadable)", got)
	}
	m, _ = pressKey(t, m, "esc")

	// The no-op navigation keys offer no retry route.
	for _, k := range []string{"n", "p"} {
		var c tea.Cmd
		m, c = update(t, m, keyPress(k))
		if c != nil {
			t.Fatalf("%s on a one-stop index produced a command %T", k, c)
		}
	}
	if s, _ := m.currentStop(); string(s.Path) != "a.txt" || s.Line != 1 {
		t.Fatal("n/p moved the one-stop cursor")
	}

	// r retries: the prior-failure overlay re-opens while the load
	// runs, the content installs beneath it, and it stays up until
	// dismissed.
	failing = false
	m, retry := update(t, m, keyPress("r"))
	if retry == nil {
		t.Fatal("r on the one-stop index issued no reload")
	}
	if m.overlay == nil {
		t.Fatal("r on the failed file did not re-open the prior-failure overlay")
	}
	m = settle(t, m, retry)
	if m.failed["a.txt"] {
		t.Fatal("successful one-stop reload left the failed state")
	}
	if m.overlay == nil {
		t.Fatal("a successful retry dismissed the prior-failure overlay")
	}
	m, _ = pressKey(t, m, "esc")
	if v := ansi.Strip(m.View().Content); !strings.Contains(v, "hit00001") {
		t.Fatalf("one-stop reload view = %q, want the file's content", v)
	}
}

// Cached content ignores disk edits until r: rewriting the file
// changes nothing — no load is issued, the rendered text is the old
// revision's — and only r observes the new bytes.
func TestDiskChangeWithoutRIsNotObserved(t *testing.T) {
	dir := t.TempDir()
	recs := append(fileWithStops(t, dir, "a.txt", 20, 1), recSummary)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)

	var b strings.Builder
	for i := 1; i <= 20; i++ {
		fmt.Fprintf(&b, "v2-%04d\n", i)
	}
	writeWorkFile(t, dir, "a.txt", b.String())

	// Ordinary inputs answer from the cached revision — none rereads
	// the file.
	m, _ = update(t, m, codePress(tea.KeyDown))
	var lay tea.Cmd
	m, lay = update(t, m, tea.WindowSizeMsg{Width: 100, Height: 24})
	for _, msg := range cmdMsgs(lay) {
		if _, ok := msg.(loadDoneMsg); ok {
			t.Fatal("a disk change triggered a reread without r")
		}
		m = pump(t, m, msg)
	}
	if v := ansi.Strip(m.View().Content); !strings.Contains(v, "x000002") || strings.Contains(v, "v2-0002") {
		t.Fatalf("cached view after the disk edit = %q, want the old bytes", v)
	}
}

// A reload mints a new content revision: a prepared layout keyed to
// the pre-reload revision that arrives after the reload completes is
// discarded — it cannot replace the reloaded content or move the
// anchor (Issue #17's superseded-revision isolation).
func TestPreReloadLayoutDiscardedAfterReload(t *testing.T) {
	dir := t.TempDir()
	recs := append(fileWithStops(t, dir, "a.txt", 60, 1), recSummary)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)
	for i := 0; i < 30; i++ {
		m, _ = update(t, m, codePress(tea.KeyDown))
	}
	anchor := m.vp.Anchor()

	// A layout request keyed to the pre-reload revision, held
	// uninvoked — the gate is simply never releasing it.
	m, held := update(t, m, tea.WindowSizeMsg{Width: 100, Height: 24})
	if held == nil {
		t.Fatal("resize issued no layout request to hold")
	}

	var b strings.Builder
	for i := 1; i <= 60; i++ {
		fmt.Fprintf(&b, "v2-%04d\n", i)
	}
	writeWorkFile(t, dir, "a.txt", b.String())
	m, reload := update(t, m, keyPress("r"))
	msgs := cmdMsgs(reload)
	if len(msgs) != 1 {
		t.Fatalf("r produced %d messages, want the single load completion", len(msgs))
	}
	m, lay := update(t, m, msgs[0])
	if m.revs["a.txt"] != 2 {
		t.Fatalf("a.txt revision = %d, want the reload's 2", m.revs["a.txt"])
	}
	gap := m.View().Content

	// The held pre-reload layout arrives late: keyed to revision 1 it
	// is discarded — the panel, anchor, and top do not move.
	for _, msg := range cmdMsgs(held) {
		m = pump(t, m, msg)
	}
	if m.vp.Anchor() != anchor {
		t.Fatalf("superseded layout moved the anchor to %v, want %v", m.vp.Anchor(), anchor)
	}
	if got := m.View().Content; got != gap {
		t.Fatalf("superseded layout changed the panel:\n%q", got)
	}

	// The new revision's own layout installs: the anchor resolves to
	// its text location in the reloaded content.
	m = settle(t, m, lay)
	if m.vp.Top() != 30 || m.vp.Anchor() != anchor {
		t.Fatalf("post-install top=%d anchor=%v, want 30/%v", m.vp.Top(), m.vp.Anchor(), anchor)
	}
	if v := ansi.Strip(m.View().Content); !strings.Contains(v, "v2-0031") {
		t.Fatalf("installed view lacks the reloaded content: %q", v)
	}
}

// Navigation during the in-flight reload supersedes the anchor intent:
// the latest selected stop's reveal commits when the new revision's
// layout installs.
func TestNavigationDuringReloadOverridesAnchor(t *testing.T) {
	dir := t.TempDir()
	recs := append(fileWithStops(t, dir, "a.txt", 300, 5, 200), recSummary)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd) // top 0, line 5 on screen

	gate := make(chan struct{})
	m.loadGate = gate
	m, reload := update(t, m, keyPress("r"))
	if reload == nil {
		t.Fatal("r issued no reload request")
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- reload() }()
	select {
	case <-done:
		t.Fatal("the reload completed while its gate was held")
	case <-time.After(50 * time.Millisecond):
	}

	// The selection moves while the reread is parked: the newest
	// stop's reveal is the intent the install must commit.
	m, _ = update(t, m, keyPress("n"))
	if s, _ := m.currentStop(); s.Line != 200 {
		t.Fatalf("n during the reload selected %+v, want a.txt:200", s)
	}
	close(gate)
	m = pump(t, m, <-done)
	if m.vp.Top() != 192 {
		t.Fatalf("top after install = %d, want the revealed 192 — the reload anchor must not win",
			m.vp.Top())
	}
	if v := m.View().Content; !strings.Contains(v, "\x1b[30;47;4mhit\x1b[24;37;40m00200") {
		t.Fatalf("reveal after reload lacks the line-200 match: %q", v)
	}
}

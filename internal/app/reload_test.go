package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

// r on a loaded file starts exactly one reread of the current path —
// no rg rerun, no cursor-stop changes — and repaints the panel as
// "Loading…" under the same filename row. A second r while that load
// is in flight is dropped, not queued: no new request, no state
// change; so is a re-entry onto the still-loading path. Only once the
// placeholder has settled does another r start a new load.
func TestReloadLifecycleAndDroppedDuplicates(t *testing.T) {
	dir := t.TempDir()
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("b.txt", "hit b\n", 1, 0, 3, "hit"))
	idx.Finish()

	loader := &stubLoader{
		fail: map[string]bool{},
		src:  &stubSource{gutter: 3, widths: []int{8}},
	}
	m, gate, jobA := gatedLoaderBrowse(t, dir, idx, 80, 24, loader)
	reqA := m.loading["a.txt"]

	// r during the startup load is dropped whole: identical request
	// bookkeeping, no command, untouched intent and presentation.
	m, c := update(t, m, keyMsg("r"))
	if c != nil {
		t.Fatalf("r during an in-flight load returned a command: %v", c)
	}
	if got := m.loading["a.txt"]; got != reqA || m.loadSeq != 1 {
		t.Fatalf("dropped r changed the load bookkeeping: req %d seq %d, want %d and 1",
			got, m.loadSeq, reqA)
	}
	if m.pendingIntent != intentReveal {
		t.Fatalf("dropped r disturbed the pending intent: %v, want the startup reveal", m.pendingIntent)
	}
	if got := m.View().Content; !strings.Contains(got, "Loading…") {
		t.Fatalf("dropped r changed the placeholder:\n%s", got)
	}
	assertSilent(t, jobA, "a.txt's startup load")

	// The placeholder settling is the completion signal.
	close(gate)
	m = applyLoad(t, m, collectMsg(t, jobA, "a.txt's startup load"))
	if got := m.View().Content; !strings.Contains(got, "xxxxxxxx") {
		t.Fatalf("a.txt's content did not render:\n%s", got)
	}
	nStops := len(m.index.Stops())
	cur, _ := m.index.Current()

	// r rereads the current file: one fresh request is minted, the
	// cached display is dropped for "Loading…", and the filename row
	// still identifies the path.
	loadGate := make(chan struct{})
	m.loadGate = loadGate
	m, c = update(t, m, keyMsg("r"))
	if c == nil {
		t.Fatal("r on a loaded file issued no reload")
	}
	jobR := runWorker(t, c)
	assertSilent(t, jobR, "a.txt's reload")
	reqR := m.loading["a.txt"]
	if reqR == reqA {
		t.Fatal("r did not mint a fresh request for the reload")
	}
	if !m.reloading["a.txt"] {
		t.Fatal("the accepted r did not mark the in-flight load a reload")
	}
	if m.buffers["a.txt"] != nil || m.sources["a.txt"] != nil {
		t.Fatal("r left the old display installed instead of the placeholder")
	}
	v := m.View().Content
	if !strings.Contains(v, "── a.txt ") || !strings.Contains(v, "Loading…") {
		t.Fatalf("reload panel lacks the filename row or the placeholder:\n%s", v)
	}
	if strings.Contains(v, "xxxxxxxx") {
		t.Fatalf("reload kept the old display instead of the placeholder:\n%s", v)
	}
	if got := len(m.index.Stops()); got != nStops {
		t.Fatalf("r changed the cursor stops: %d, want %d", got, nStops)
	}
	if now, _ := m.index.Current(); now.Line != cur.Line || string(now.Path) != string(cur.Path) {
		t.Fatalf("r moved the cursor to %+v, want %+v", now, cur)
	}

	// A duplicate r in flight is dropped: nothing minted, nothing runs.
	m, c = update(t, m, keyMsg("r"))
	if c != nil {
		t.Fatalf("a duplicate r returned a command: %v", c)
	}
	if got := m.loading["a.txt"]; got != reqR {
		t.Fatalf("a duplicate r changed the request: req %d, want %d", got, reqR)
	}

	// Re-entry onto the loading path is likewise dropped: away to
	// b.txt (loaded ungated) and back starts no second a.txt load.
	m.loadGate = nil
	m, nav := update(t, m, keyMsg("n"))
	m = applyLoad(t, m, deliverNavLoad(t, nav))
	m, nav = update(t, m, keyMsg("p"))
	for _, msg := range navMsgs(t, nav) {
		if _, isLoad := msg.(loadResult); isLoad {
			t.Fatal("re-entry onto the reloading file started a second load")
		}
	}
	if got := m.loading["a.txt"]; got != reqR {
		t.Fatalf("re-entry changed the in-flight request: req %d, want %d", got, reqR)
	}
	if got := m.View().Content; !strings.Contains(got, "Loading…") {
		t.Fatalf("re-entry lost the reload placeholder:\n%s", got)
	}
	assertSilent(t, jobR, "a.txt's reload")

	// The reload completes: a new content revision feeds the layout
	// keying and the settled panel shows the content again.
	close(loadGate)
	m = applyLoad(t, m, collectMsg(t, jobR, "a.txt's reload"))
	if got := m.revs["a.txt"]; got != 2 {
		t.Fatalf("reload revision = %d, want 2", got)
	}
	if _, ok := m.loading["a.txt"]; ok {
		t.Fatal("the settled reload was not retired")
	}
	if got := m.View().Content; !strings.Contains(got, "xxxxxxxx") || strings.Contains(got, "Loading…") {
		t.Fatalf("settled reload did not repaint the content:\n%s", got)
	}
	if got := loader.callsFor("a.txt"); got != 2 {
		t.Fatalf("a.txt loader calls = %d, want exactly 2", got)
	}

	// After settlement r starts a fresh load again.
	seq := m.loadSeq
	m, c = update(t, m, keyMsg("r"))
	if c == nil {
		t.Fatal("r after the placeholder settled issued no new load")
	}
	if got := m.loading["a.txt"]; got == reqR || m.loadSeq != seq+1 {
		t.Fatalf("r after settlement minted req %d at seq %d, want a fresh request", got, m.loadSeq)
	}
	m = applyLoad(t, m, c())
	if got := loader.callsFor("a.txt"); got != 3 {
		t.Fatalf("a.txt loader calls = %d, want 3", got)
	}
}

// A reload with no intervening navigation preserves the cursor and the
// logical viewport anchor — no reveal: the pre-reload top survives even
// though the cursor's target row is hidden from it. The reload-anchor
// intent is recorded when the reload's load completes and commits only
// when the new revision's matching prepared layout installs.
func TestReloadPreservesAnchorWithoutReveal(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(100))
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-05\n", 5, 0, 4, "line"))
	addRec(t, idx, matchRec("a.txt", "line-95\n", 95, 0, 4, "line"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m.theme = theme.Plain()

	// Scroll so the cursor's target row leaves the window: a reveal on
	// commit would move the top toward it.
	for i := 0; i < 30; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if got := m.vps["a.txt"].Top(); got != 30 {
		t.Fatalf("scrolled to top %d, want 30", got)
	}
	anchor := m.vps["a.txt"].Anchor()

	// r starts the reload; the worker holds behind the load gate.
	loadGate := make(chan struct{})
	m.loadGate = loadGate
	m, c := update(t, m, keyMsg("r"))
	if c == nil {
		t.Fatal("r issued no reload")
	}
	jobR := runWorker(t, c)
	assertSilent(t, jobR, "a.txt's reload")

	// Hold the new revision's layout too, so the recorded intent is
	// observable between the completion and the install.
	layoutGate := make(chan struct{})
	m.layoutGate = layoutGate
	close(loadGate)
	m, layout := update(t, m, collectMsg(t, jobR, "a.txt's reload"))
	if layout == nil {
		t.Fatal("the reload's completion requested no layout")
	}
	jobL := runLayoutJob(t, layout)
	assertHeld(t, jobL, "the reloaded layout")
	if got := m.View().Content; !strings.Contains(got, "Loading…") {
		t.Fatalf("the settled reload lost the placeholder before the layout installed:\n%s", got)
	}

	// The recorded intent is anchor preservation — not reveal — for
	// the revision the reload established.
	if m.pendingIntent != intentReloadAnchor {
		t.Fatalf("reload completion recorded intent %v, want the reload-anchor intent", m.pendingIntent)
	}
	if got := m.revs["a.txt"]; got != 2 {
		t.Fatalf("reload revision = %d, want 2", got)
	}

	close(layoutGate)
	m, _ = update(t, m, collectMsg(t, jobL, "the reloaded layout"))
	if m.pendingIntent != intentNone {
		t.Fatalf("the matching layout left intent %v pending", m.pendingIntent)
	}
	if got := m.vps["a.txt"].Top(); got != 30 {
		t.Fatalf("top after reload = %d, want the preserved anchor's row 30", got)
	}
	if got := m.vps["a.txt"].Anchor(); got != anchor {
		t.Fatalf("anchor after reload = %+v, want %+v", got, anchor)
	}
	if stop, _ := m.index.Current(); stop.Line != 5 {
		t.Fatalf("cursor after reload = line %d, want the unchanged stop 5", stop.Line)
	}
	if row := contentRow(t, m); !strings.Contains(row, "line-31") {
		t.Fatalf("reloaded panel = %q, want line-31 at the preserved top", row)
	}
}

// The preserved anchor is clamped to the new content: when the reload
// finds a shorter file, the effective top moves up and the anchor is
// updated to the resulting top — asserted only after the new
// revision's matching layout installs, not against the old revision's.
func TestReloadAnchorClampsToNewContent(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(100))
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-05\n", 5, 0, 4, "line"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m.theme = theme.Plain()

	// Bottom of the file: extent 100, content height 23 → top 77.
	for i := 0; i < 4; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyPgDown})
	}
	if got := m.vps["a.txt"].Top(); got != 77 {
		t.Fatalf("top at EOF = %d, want 77", got)
	}

	// The file shrinks on disk; r observes it through the placeholder.
	writeMatchFile(t, dir, "a.txt", numberedContent(40))
	m, c := update(t, m, keyMsg("r"))
	if c == nil {
		t.Fatal("r issued no reload")
	}
	if got := m.View().Content; !strings.Contains(got, "Loading…") {
		t.Fatalf("reload lost the placeholder:\n%s", got)
	}
	m = applyLoad(t, m, c())

	// The anchor's line 77 is beyond the new end: the installed layout
	// clamps the top to the last full window, extent 40 − 23 = 17, and
	// the anchor follows to that top.
	if got := m.vps["a.txt"].Top(); got != 17 {
		t.Fatalf("top after shrink = %d, want the EOF clamp 17", got)
	}
	if got := m.vps["a.txt"].Anchor(); got.Line != 17 || got.Col != 0 {
		t.Fatalf("anchor after shrink = %+v, want line 17 col 0", got)
	}
	if row := contentRow(t, m); !strings.Contains(row, "line-18") {
		t.Fatalf("panel top row = %q, want line-18", row)
	}
}

// A failed reload never presents stale content as refreshed: the old
// display is replaced by "(unreadable)", the current-file failure
// overlay opens with the diagnostic, the filename row keeps the path,
// and the cursor stops stay. Retrying a failed file with r re-shows
// the prior failure while the retry runs — the re-entry presentation —
// so a second consecutive failure appends exactly one occurrence to
// the open overlay without moving the reader, and collects one more
// for replay.
func TestReloadFailureReplacesContent(t *testing.T) {
	longErr := errors.New("denied: " + strings.Repeat("x", 200))
	loader := &stubLoader{
		fail: map[string]bool{},
		err:  longErr,
		src:  &stubSource{gutter: 3, widths: []int{8}},
	}
	dir := t.TempDir()
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("a.txt", "hit a2\n", 2, 0, 3, "hit"))
	idx.Finish()

	m, cmd := browseWithLoader(t, dir, idx, 40, 6, loader)
	m = applyLoad(t, m, cmd())
	if got := m.View().Content; !strings.Contains(got, "xxxxxxxx") {
		t.Fatalf("a.txt's content did not render:\n%s", got)
	}
	nStops := len(m.index.Stops())

	// The reload fails: "(unreadable)" replaces the old content and the
	// overlay opens with the diagnostic.
	loader.setFail("a.txt", true)
	m, c := update(t, m, keyMsg("r"))
	if c == nil {
		t.Fatal("r issued no reload")
	}
	m, _ = update(t, m, c())
	want := "cannot read a.txt: " + longErr.Error()
	if m.overlay == nil || len(m.overlay.lines) != 1 || m.overlay.lines[0] != want {
		t.Fatalf("reload failure overlay = %v, want %q", m.overlay, want)
	}
	if got := countDiag(m, want); got != 1 {
		t.Fatalf("collected %d occurrences of %q, want 1", got, want)
	}
	m, _ = update(t, m, keyPress("esc"))
	v := m.View().Content
	if !strings.Contains(v, "(unreadable)") || strings.Contains(v, "xxxxxxxx") {
		t.Fatalf("failed reload left the old display:\n%s", v)
	}
	if !strings.Contains(v, "── a.txt ") {
		t.Fatalf("failed reload lost the filename row:\n%s", v)
	}
	if got := len(m.index.Stops()); got != nStops {
		t.Fatalf("failed reload changed the cursor stops: %d, want %d", got, nStops)
	}

	// A second r re-shows the prior failure while it retries.
	gate := make(chan struct{})
	m.loadGate = gate
	m, c = update(t, m, keyMsg("r"))
	if c == nil {
		t.Fatal("r on a failed file issued no retry")
	}
	if m.overlay == nil || len(m.overlay.lines) != 1 || m.overlay.lines[0] != want {
		t.Fatalf("r on a failed file did not re-show the prior failure: %v", m.overlay)
	}
	retry := runWorker(t, c)
	assertSilent(t, retry, "a.txt's reload retry")

	// The reader scrolls inside the open overlay while the retry runs.
	m, _ = update(t, m, keyPress("down"))
	if m.overlay.scroll != 1 {
		t.Fatalf("overlay scroll = %d, want 1", m.overlay.scroll)
	}

	// The second consecutive failure appends exactly one occurrence —
	// the reader's position holds — and collects one more for replay.
	close(gate)
	m, _ = update(t, m, collectMsg(t, retry, "a.txt's reload retry"))
	if len(m.overlay.lines) != 2 || m.overlay.lines[1] != want {
		t.Fatalf("second reload failure left overlay lines %q, want one appended occurrence", m.overlay.lines)
	}
	if m.overlay.scroll != 1 {
		t.Fatalf("the appended occurrence moved the reader: scroll = %d, want 1", m.overlay.scroll)
	}
	if got := countDiag(m, want); got != 2 {
		t.Fatalf("collected %d occurrences of %q, want 2", got, want)
	}
}

// r is the one-stop index's only retry route: n and p are strict
// no-ops there, so a failed file can be retried only by r — which
// re-shows the prior failure while the retry runs — and a loaded file
// reloads in place.
func TestReloadOneStopIndex(t *testing.T) {
	loader := &stubLoader{
		fail: map[string]bool{"a.txt": true},
		err:  errors.New("denied"),
		src:  &stubSource{gutter: 3, widths: []int{8}},
	}
	dir := t.TempDir()
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	idx.Finish()

	m, cmd := browseWithLoader(t, dir, idx, 80, 24, loader)
	m = applyLoad(t, m, cmd()) // the single stop's load fails
	m, _ = update(t, m, keyPress("esc"))
	if got := m.View().Content; !strings.Contains(got, "(unreadable)") {
		t.Fatalf("the failed single stop lacks the placeholder:\n%s", got)
	}

	// n and p go nowhere and retry nothing.
	for _, key := range []string{"n", "p"} {
		var c tea.Cmd
		m, c = update(t, m, keyMsg(key))
		if c != nil {
			t.Fatalf("%s on a one-stop index returned a command: %v", key, c)
		}
	}
	if got := loader.callsFor("a.txt"); got != 1 {
		t.Fatalf("one-stop navigation retried the load: %d calls, want 1", got)
	}

	// r retries: the prior failure shows while the retry is in flight.
	loader.setFail("a.txt", false)
	m, c := update(t, m, keyMsg("r"))
	if c == nil {
		t.Fatal("r on the failed single stop issued no retry")
	}
	if m.overlay == nil {
		t.Fatal("r on a failed file did not re-show the prior failure")
	}
	m = applyLoad(t, m, c())
	if m.buffers["a.txt"] == nil {
		t.Fatal("the r retry installed no content")
	}
	if v := m.View().Content; !strings.Contains(v, "xxxxxxxx") {
		t.Fatalf("the retried file lacks its content:\n%s", v)
	}
	m, _ = update(t, m, keyPress("esc"))

	// And r on the loaded file reloads it in place.
	m, c = update(t, m, keyMsg("r"))
	if c == nil {
		t.Fatal("r on the loaded single stop issued no reload")
	}
	m = applyLoad(t, m, c())
	if got := loader.callsFor("a.txt"); got != 3 {
		t.Fatalf("a.txt loader calls = %d, want 3", got)
	}
	if v := m.View().Content; !strings.Contains(v, "xxxxxxxx") {
		t.Fatalf("the reloaded single stop lacks its content:\n%s", v)
	}
}

// Cached content ignores disk edits until r: rewriting the file and
// ordinary activity — scrolling, a resize — leave the display on the
// old bytes; only an explicit reload observes the new content.
func TestDiskChangeStableUntilReload(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", "hit a\n"+numberedContent(30))
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m.theme = theme.Plain()

	// The file changes on disk; without r the cached display is stable
	// through scrolling and a resize. The new second line sits inside
	// the preserved window once a reload observes it.
	writeMatchFile(t, dir, "a.txt", "hit a\nNEWLINE-XX\n"+numberedContent(30))
	m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	m, layout := update(t, m, tea.WindowSizeMsg{Width: 100, Height: 24})
	m = deliverCmd(t, m, layout)
	if got := m.View().Content; strings.Contains(got, "NEWLINE-XX") {
		t.Fatalf("a disk change reached the display without r:\n%s", got)
	}

	// r observes the new bytes.
	m, c := update(t, m, keyMsg("r"))
	if c == nil {
		t.Fatal("r issued no reload")
	}
	m = applyLoad(t, m, c())
	if got := m.View().Content; !strings.Contains(got, "NEWLINE-XX") {
		t.Fatalf("the reload did not pick up the new content:\n%s", got)
	}
}

// A reload produces a new content revision, superseding layout
// preparation keyed to the old one: a gated pre-reload layout released
// after the reload completes is discarded — it cannot replace the
// reloaded content, move the anchor, or consume the pending intent —
// while the layout built for the new revision installs and commits the
// preserved anchor.
func TestReloadSupersedesOldRevisionLayout(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(100))
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-05\n", 5, 0, 4, "line"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m.theme = theme.Plain()
	for i := 0; i < 30; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	anchor := m.vps["a.txt"].Anchor()
	oldRev := m.revs["a.txt"]

	// A resize's layout job — keyed to the pre-reload revision — is
	// held behind the layout gate.
	layoutGate := make(chan struct{})
	m.layoutGate = layoutGate
	m, stale := update(t, m, tea.WindowSizeMsg{Width: 100, Height: 24})
	if stale == nil {
		t.Fatal("the resize requested no layout preparation")
	}
	jobOld := runLayoutJob(t, stale)
	assertHeld(t, jobOld, "the pre-reload layout")

	// The reload completes while the old layout is still held: a new
	// revision is established and its own layout is requested.
	m, c := update(t, m, keyMsg("r"))
	if c == nil {
		t.Fatal("r issued no reload")
	}
	m, layout := update(t, m, c())
	if layout == nil {
		t.Fatal("the reload's completion requested no layout")
	}
	jobNew := runLayoutJob(t, layout)
	assertHeld(t, jobNew, "the new revision's layout")
	if got := m.revs["a.txt"]; got != oldRev+1 {
		t.Fatalf("reload revision = %d, want %d", got, oldRev+1)
	}
	if m.pendingIntent != intentReloadAnchor {
		t.Fatalf("reload completion recorded intent %v, want the reload-anchor intent", m.pendingIntent)
	}

	// Released together, the pre-reload layout is discarded: it cannot
	// install over the placeholder, move the anchor, or consume the
	// intent.
	close(layoutGate)
	m, _ = update(t, m, collectMsg(t, jobOld, "the pre-reload layout"))
	if m.buffers["a.txt"] != nil {
		t.Fatal("a superseded layout installed over the reload")
	}
	if got := m.vps["a.txt"].Anchor(); got != anchor {
		t.Fatalf("a superseded layout moved the anchor to %+v, want %+v", got, anchor)
	}
	if m.pendingIntent != intentReloadAnchor {
		t.Fatal("a superseded layout consumed the pending intent")
	}
	if got := m.View().Content; !strings.Contains(got, "Loading…") {
		t.Fatalf("a superseded layout replaced the placeholder:\n%s", got)
	}

	// The new revision's layout installs and commits the preserved
	// anchor: identical content maps the anchor back to its own row.
	m, _ = update(t, m, collectMsg(t, jobNew, "the new revision's layout"))
	if m.pendingIntent != intentNone {
		t.Fatalf("the matching layout left intent %v pending", m.pendingIntent)
	}
	if got := m.vps["a.txt"].Top(); got != 30 {
		t.Fatalf("top after the superseding reload = %d, want 30", got)
	}
	if row := contentRow(t, m); !strings.Contains(row, "line-31") {
		t.Fatalf("reloaded panel = %q, want line-31 at the preserved top", row)
	}
}

// wideReloadIndex writes a.txt's reload-intent fixture — a 100-line
// file whose lines 5, 41, and 95 are 200 cells wide so a run-off-edge
// pan survives at either test scroll position — with matched stops at
// lines 5 and 95, each targeting column 5.
func wideReloadIndex(t *testing.T, dir string) *searchindex.Index {
	t.Helper()
	var b strings.Builder
	for i := 1; i <= 100; i++ {
		switch i {
		case 5:
			b.WriteString("aaaaa" + strings.Repeat("x", 195) + "\n")
		case 41:
			b.WriteString(strings.Repeat("x", 200) + "\n")
		case 95:
			b.WriteString("bbbbb" + strings.Repeat("x", 195) + "\n")
		default:
			fmt.Fprintf(&b, "line-%02d\n", i)
		}
	}
	writeMatchFile(t, dir, "a.txt", b.String())
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "aaaaa"+strings.Repeat("x", 195)+"\n", 5, 5, 7, "xx"))
	addRec(t, idx, matchRec("a.txt", "bbbbb"+strings.Repeat("x", 195)+"\n", 95, 5, 7, "xx"))
	idx.Finish()
	return idx
}

// reloadSetup loads a.txt's wide fixture in run-off-edge mode, then
// scrolls to top 33 — hiding the line-5 target above the window — and
// pans to offset 50: the position and offset a navigation-free reload
// preserves, and a navigation reveal commits away from.
func reloadSetup(t *testing.T, dir string, idx *searchindex.Index) Model {
	t.Helper()
	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m.theme = theme.Plain()
	m, cmd = update(t, m, keyMsg("w"))
	m = deliverCmd(t, m, cmd)
	for i := 0; i < 3; i++ {
		m, _ = update(t, m, keyMsg("d")) // half-page: 11 each → top 33
	}
	for i := 0; i < 5; i++ {
		m, _ = update(t, m, keyMsg(">")) // → offset 50
	}
	if got := m.vps["a.txt"].Top(); got != 33 {
		t.Fatalf("setup top = %d, want 33", got)
	}
	if got := m.vps["a.txt"].Offset(); got != 50 {
		t.Fatalf("setup offset = %d, want 50", got)
	}
	return m
}

// A reload with no intervening navigation commits the
// anchor-preserving install: the saved top and the retained horizontal
// offset both survive, re-clamped against the new revision — the
// commit's horizontal reset applies only to a pending reveal.
func TestReloadAnchorKeepsRetainedOffset(t *testing.T) {
	dir := t.TempDir()
	m := reloadSetup(t, dir, wideReloadIndex(t, dir))

	m, c := update(t, m, keyMsg("r"))
	m = applyLoad(t, m, c())

	if got := m.vps["a.txt"].Top(); got != 33 {
		t.Fatalf("anchor commit top = %d, want the preserved 33", got)
	}
	if got := m.vps["a.txt"].Offset(); got != 50 {
		t.Fatalf("anchor commit offset = %d, want the retained 50", got)
	}
}

// Navigation during the reload replaces the anchor-preserving intent
// with the reveal for the newest selection: the commit lands the new
// target at one-third and resets the retained offset first — the
// navigation intent, not the reload, decides what installs.
func TestReloadNavigationDuringLoadCommitsReveal(t *testing.T) {
	dir := t.TempDir()
	m := reloadSetup(t, dir, wideReloadIndex(t, dir))

	loadGate := make(chan struct{})
	m.loadGate = loadGate
	layoutGate := make(chan struct{})
	m.layoutGate = layoutGate
	m, c := update(t, m, keyMsg("r"))
	jobR := runWorker(t, c)
	assertSilent(t, jobR, "a.txt's reload")

	// n during the held load moves the cursor to the second stop and
	// leaves the pending reveal.
	m, c = update(t, m, keyMsg("n"))
	if c != nil {
		t.Fatalf("n during the held reload returned a command: %v", c)
	}
	if stop, _ := m.index.Current(); stop.Line != 95 {
		t.Fatalf("n during the reload selected line %d, want 95", stop.Line)
	}
	if m.pendingIntent != intentReveal {
		t.Fatalf("n during the reload left intent %v, want the pending reveal", m.pendingIntent)
	}

	// The completion keeps the reveal intent — the reload's anchor
	// intent never displaces a pending reveal — and requests the
	// new-revision layout.
	close(loadGate)
	m, cmd := update(t, m, collectMsg(t, jobR, "a.txt's reload"))
	if m.pendingIntent != intentReveal {
		t.Fatal("the reload completion overrode the navigation reveal")
	}
	jobL := runLayoutJob(t, cmd)
	assertHeld(t, jobL, "the rev-2 layout")

	// The install commits the newest selection's reveal: target row 94
	// lands at 94 − 23/3 = 87, EOF-clamped to 77 — never the
	// pre-reload anchor's 33 — and the offset resets before the
	// column-5 target's minimal reveal leaves it at zero.
	close(layoutGate)
	m, _ = update(t, m, collectMsg(t, jobL, "the rev-2 layout"))
	if got := m.vps["a.txt"].Top(); got != 77 {
		t.Fatalf("committed top = %d, want 77 — the newest target's placement, not the anchor 33", got)
	}
	if got := m.vps["a.txt"].Offset(); got != 0 {
		t.Fatalf("committed offset = %d, want the reset 0", got)
	}
	if row := contentRow(t, m); !strings.Contains(row, "line-78") {
		t.Fatalf("panel top row = %q, want line-78", row)
	}
}

// Same-file navigation away and back during the reload — ending on the
// very stop the reload started from — still commits the reveal: the
// intent records that navigation happened, never the cursor's equality
// with its start.
func TestReloadAwayAndBackStillCommitsReveal(t *testing.T) {
	dir := t.TempDir()
	m := reloadSetup(t, dir, wideReloadIndex(t, dir))

	loadGate := make(chan struct{})
	m.loadGate = loadGate
	layoutGate := make(chan struct{})
	m.layoutGate = layoutGate
	m, c := update(t, m, keyMsg("r"))
	jobR := runWorker(t, c)
	assertSilent(t, jobR, "a.txt's reload")

	// Away to line 95 and back to line 5: the cursor returns to its
	// reload-time stop, but the navigation happened — the reveal
	// intent stands.
	m, _ = update(t, m, keyMsg("n"))
	m, _ = update(t, m, keyMsg("p"))
	if stop, _ := m.index.Current(); stop.Line != 5 {
		t.Fatalf("n p during the reload selected line %d, want 5", stop.Line)
	}
	if m.pendingIntent != intentReveal {
		t.Fatalf("the away-and-back left intent %v, want the pending reveal", m.pendingIntent)
	}

	close(loadGate)
	m, cmd := update(t, m, collectMsg(t, jobR, "a.txt's reload"))
	if m.pendingIntent != intentReveal {
		t.Fatal("the reload completion overrode the navigation reveal")
	}
	jobL := runLayoutJob(t, cmd)
	assertHeld(t, jobL, "the rev-2 layout")

	// The commit reveals the line-5 target from the saved top 33: row
	// 4 is hidden above, so the top moves to 4 − 23/3, clamped at the
	// file top — never the anchor's 33 — and the offset resets.
	close(layoutGate)
	m, _ = update(t, m, collectMsg(t, jobL, "the rev-2 layout"))
	if got := m.vps["a.txt"].Top(); got != 0 {
		t.Fatalf("committed top = %d, want 0 — the reveal, not the anchor 33", got)
	}
	if got := m.vps["a.txt"].Offset(); got != 0 {
		t.Fatalf("committed offset = %d, want the reset 0", got)
	}
	if row := contentRow(t, m); !strings.Contains(row, "line-01") {
		t.Fatalf("panel top row = %q, want line-01", row)
	}
}

// Navigation across files during the reload — away to b.txt and back —
// shows "Loading…" on the return and commits the entry reveal for the
// latest selection, never the pre-reload anchor.
func TestReloadCrossFileAwayAndBackCommitsReveal(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(100))
	writeMatchFile(t, dir, "b.txt", "hit b\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-05\n", 5, 0, 4, "line"))
	addRec(t, idx, matchRec("b.txt", "hit b\n", 1, 0, 3, "hit"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m.theme = theme.Plain()
	for i := 0; i < 3; i++ {
		m, _ = update(t, m, keyMsg("d")) // top 33 — target row 4 hidden above
	}

	loadGate := make(chan struct{})
	m.loadGate = loadGate
	m, c := update(t, m, keyMsg("r"))
	jobR := runWorker(t, c)
	assertSilent(t, jobR, "a.txt's reload")

	// Away to b.txt — its load is held behind the same gate — and back
	// to a.txt, which renders the reload placeholder.
	m, nav := update(t, m, keyMsg("n"))
	jobsB := navJobs(t, nav)
	m, _ = update(t, m, keyMsg("p"))
	if got := m.View().Content; !strings.Contains(got, "Loading…") {
		t.Fatalf("the return to a.txt did not show the reload placeholder:\n%s", got)
	}
	if m.pendingIntent != intentReveal {
		t.Fatal("the return to the reloading file did not leave a pending reveal")
	}

	// a.txt's completion commits the entry reveal for the line-5 stop:
	// the saved anchor's top 33 is only the starting point — the
	// hidden target lands at the file top.
	close(loadGate)
	m = applyLoad(t, m, collectMsg(t, jobR, "a.txt's reload"))
	if got := m.vps["a.txt"].Top(); got != 0 {
		t.Fatalf("committed top = %d, want the entry reveal's 0, not the anchor 33", got)
	}
	if row := contentRow(t, m); !strings.Contains(row, "line-01") {
		t.Fatalf("panel top row = %q, want line-01", row)
	}

	// b.txt's held load completes non-current: cache only — the panel
	// is untouched.
	for _, j := range jobsB {
		if lr, ok := collectMsg(t, j, "b.txt navigation job").(loadResult); ok {
			before := m.View().Content
			m, _ = update(t, m, lr)
			if got := m.View().Content; got != before {
				t.Fatal("b.txt's non-current completion changed the panel")
			}
		}
	}
}

// Old-revision and new-revision layouts complete out of order: the new
// revision installs and commits the anchor-preserving intent, and the
// late old-revision layout is discarded without moving the anchor or
// touching the consumed intent.
func TestReloadOutOfOrderRevisionInstall(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(100))
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-05\n", 5, 0, 4, "line"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m.theme = theme.Plain()
	for i := 0; i < 3; i++ {
		m, _ = update(t, m, keyMsg("d")) // top 33
	}

	// A resize mints an old-revision layout, held behind the gate; r
	// then reloads and its completion requests the new-revision one.
	layoutGate := make(chan struct{})
	m.layoutGate = layoutGate
	m, resize := update(t, m, tea.WindowSizeMsg{Width: 100, Height: 24})
	jobOld := runLayoutJob(t, resize)
	assertHeld(t, jobOld, "the old-revision layout")

	m, c := update(t, m, keyMsg("r"))
	m, cmd = update(t, m, c()) // the ungated reload completes inline
	jobNew := runLayoutJob(t, cmd)
	assertHeld(t, jobNew, "the new-revision layout")

	// The new revision installs first and commits the anchor intent:
	// the saved top is preserved and the intent retires.
	close(layoutGate)
	m, _ = update(t, m, collectMsg(t, jobNew, "the new-revision layout"))
	if got := m.buffers["a.txt"].Key().Revision; got != 2 {
		t.Fatalf("installed revision = %d, want 2", got)
	}
	if got := m.vps["a.txt"].Top(); got != 33 {
		t.Fatalf("the anchor-preserving commit moved the top to %d, want 33", got)
	}
	if m.pendingIntent != intentNone {
		t.Fatal("the anchor commit left a pending intent")
	}

	// The late old-revision layout is discarded: the installed
	// revision, the anchor, and the consumed intent are untouched.
	m, c = update(t, m, collectMsg(t, jobOld, "the old-revision layout"))
	if c != nil {
		t.Fatal("the late old-revision layout issued a command")
	}
	if got := m.buffers["a.txt"].Key().Revision; got != 2 {
		t.Fatalf("the old revision displaced the installed layout: revision %d", got)
	}
	if got := m.vps["a.txt"].Top(); got != 33 {
		t.Fatalf("the old-revision discard moved the anchor to %d, want 33", got)
	}
	if m.pendingIntent != intentNone {
		t.Fatal("the old-revision discard mutated the consumed intent")
	}
}

// With navigation during the reload the new-revision install commits
// the reveal instead; a still-later old-revision layout cannot undo
// either the install or the commit.
func TestReloadRevealCommitThenLateOldLayout(t *testing.T) {
	dir := t.TempDir()
	m := reloadSetup(t, dir, wideReloadIndex(t, dir))

	// An old-revision layout is already in flight from a resize before
	// the reload.
	layoutGate := make(chan struct{})
	m.layoutGate = layoutGate
	m, resize := update(t, m, tea.WindowSizeMsg{Width: 100, Height: 24})
	jobOld := runLayoutJob(t, resize)
	assertHeld(t, jobOld, "the old-revision layout")

	m, c := update(t, m, keyMsg("r"))
	m, _ = update(t, m, keyMsg("n")) // select the line-95 stop mid-reload
	m, cmd := update(t, m, c())      // the ungated reload completes → rev 2
	jobNew := runLayoutJob(t, cmd)
	assertHeld(t, jobNew, "the new-revision layout")

	// The new revision installs and commits the reveal for the newest
	// selection, evaluated at the resized width.
	close(layoutGate)
	m, _ = update(t, m, collectMsg(t, jobNew, "the new-revision layout"))
	if got := m.vps["a.txt"].Top(); got != 77 {
		t.Fatalf("committed top = %d, want 77", got)
	}
	if got := m.vps["a.txt"].Offset(); got != 0 {
		t.Fatalf("committed offset = %d, want the reset 0", got)
	}
	if m.pendingIntent != intentNone {
		t.Fatal("the reveal commit left a pending intent")
	}

	// The late old-revision layout is discarded whole — the installed
	// revision and the committed viewport are untouched.
	m, c = update(t, m, collectMsg(t, jobOld, "the old-revision layout"))
	if c != nil || m.buffers["a.txt"].Key().Revision != 2 {
		t.Fatal("the late old-revision layout was not discarded")
	}
	if got := m.vps["a.txt"].Top(); got != 77 {
		t.Fatalf("the old-revision discard moved the top to %d, want 77", got)
	}
	if got := m.vps["a.txt"].Offset(); got != 0 {
		t.Fatalf("the old-revision discard changed the offset to %d, want 0", got)
	}
}

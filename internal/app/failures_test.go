package app

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"vrg/internal/filebuffer"
)

// errRead is the injected loader's read failure.
var errRead = errors.New("denied")

// failLoader is an injected read-phase loader that fails every path —
// the Issue #26 tests substitute it for filesystem permissions, which
// an unprivileged-user assumption cannot rely on.
func failLoader(err error) func([]byte) ([]byte, error) {
	return func([]byte) ([]byte, error) { return nil, err }
}

// failLoaderFor fails only resolved paths ending in name; anything
// else reads through to disk.
func failLoaderFor(name string, err error) func([]byte) ([]byte, error) {
	return func(p []byte) ([]byte, error) {
		if bytes.HasSuffix(p, []byte(name)) {
			return nil, err
		}
		return filebuffer.ReadFile(p)
	}
}

// loaderModel returns a browse model whose load workers read through
// the injected loader, with the startup file's load command returned
// uninvoked — browseModel plus the read-phase seam.
func loaderModel(t *testing.T, dir string, w, h int, read func([]byte) ([]byte, error), recs ...string) (Model, tea.Cmd) {
	t.Helper()
	m := newModel(nil, nil)
	m.popupTimer = func(int) tea.Cmd { return nil }
	m.readFile = read
	m, _ = update(t, m, tea.WindowSizeMsg{Width: w, Height: h})
	return update(t, m, searchDoneMsg{index: fixtureIndex(t, dir, recs...)})
}

// gatedLoaderModel wires both load seams: the gate holding the worker
// before its read and the injected read-phase loader.
func gatedLoaderModel(t *testing.T, dir string, gate chan struct{}, read func([]byte) ([]byte, error), recs ...string) (Model, tea.Cmd) {
	t.Helper()
	m := newModel(nil, nil)
	m.popupTimer = func(int) tea.Cmd { return nil }
	m.loadGate = gate
	m.readFile = read
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	return update(t, m, searchDoneMsg{index: fixtureIndex(t, dir, recs...)})
}

// contentRow1 returns the first content row of the composed frame —
// the row the Loading…/(unreadable) placeholder occupies — stripped of
// ANSI styling.
func contentRow1(t *testing.T, m Model) string {
	t.Helper()
	rows := strings.Split(m.View().Content, "\n")
	if len(rows) < 2 {
		t.Fatalf("frame has %d rows, want a content row", len(rows))
	}
	return ansi.Strip(rows[1])
}

// overlayOccurrences counts how many times the diagnostic intro text
// appears across the open overlay's lines.
func overlayOccurrences(m Model, intro string) int {
	if m.overlay == nil {
		return 0
	}
	n := 0
	for _, l := range m.overlay.lines {
		if strings.Contains(l, intro) {
			n++
		}
	}
	return n
}

// A read failure on the current file interrupts: the diagnostics
// overlay opens carrying the failure, the panel reads "(unreadable)",
// the filename row still names the file — and the file's stops stay
// navigable: an n step moves the cursor within it without a retry.
func TestCurrentFileReadFailureNotifies(t *testing.T) {
	dir := t.TempDir()
	var recs []string
	recs = append(recs, fileWithStops(t, dir, "a.txt", 10, 1, 5)...)
	recs = append(recs, fileWithStops(t, dir, "b.txt", 10, 3)...)
	recs = append(recs, recSummary)
	m, cmd := loaderModel(t, dir, 80, 24, failLoaderFor("a.txt", errRead), recs...)
	m = settle(t, m, cmd)

	v := m.View().Content
	if m.overlay == nil {
		t.Fatal("current-file read failure did not open the error overlay")
	}
	if !strings.Contains(v, "cannot read a.txt: denied") {
		t.Fatalf("overlay lacks the read-failure diagnostic: %q", v)
	}
	if !strings.Contains(v, "(unreadable)") {
		t.Fatalf("failed panel lacks the unreadable placeholder: %q", v)
	}
	if !strings.Contains(v, "── a.txt ") {
		t.Fatalf("filename row stopped identifying the path: %q", v)
	}

	// The failed file's stops are retained: n still moves the cursor
	// within it once the overlay is dismissed — and requests no reload.
	m, _ = pressKey(t, m, "esc")
	m, c := update(t, m, keyPress("n"))
	if s, _ := m.currentStop(); string(s.Path) != "a.txt" || s.Line != 5 {
		t.Fatalf("n inside the failed file selected %+v, want a.txt:5", s)
	}
	if c != nil {
		t.Fatalf("same-file step on the failed file produced a command %T — no retry is owed", c)
	}
	if m.overlay != nil {
		t.Fatal("a same-file step re-opened the failure overlay")
	}
	if got := contentRow1(t, m); !strings.Contains(got, "(unreadable)") {
		t.Fatalf("same-file step changed the placeholder: %q", got)
	}
}

// A read failure on a non-current file is a diagnostic only: no
// overlay, no indicator — the frame is byte-identical — and exactly
// one occurrence joins the session collection for the exit replay.
// Visiting the file later surfaces its overlay.
func TestNonCurrentReadFailureIsDiagnosticOnly(t *testing.T) {
	dir := t.TempDir()
	var recs []string
	recs = append(recs, fileWithStops(t, dir, "a.txt", 10, 1)...)
	recs = append(recs, fileWithStops(t, dir, "b.txt", 10, 3)...)
	recs = append(recs, recSummary)
	m, cmd := loaderModel(t, dir, 80, 24, failLoaderFor("b.txt", errRead), recs...)
	m = settle(t, m, cmd) // a.txt loads

	// b.txt's load starts on entry, but its completion lands after the
	// cursor has returned to a.txt — a non-current failure.
	m, cmdB := update(t, m, keyPress("n"))
	m, _ = update(t, m, keyPress("p"))
	before := m.View().Content
	diags := len(m.diags)
	m = settle(t, m, cmdB)

	if m.overlay != nil {
		t.Fatal("a non-current read failure opened the overlay")
	}
	if got := m.View().Content; got != before {
		t.Fatalf("a non-current read failure changed the current frame:\n%q", got)
	}
	if len(m.diags) != diags+1 {
		t.Fatalf("non-current failure collected %d diagnostics, want exactly one", len(m.diags)-diags)
	}
	if strings.Contains(before, "cannot read") || strings.Contains(before, "(unreadable)") {
		t.Fatalf("a non-current failure surfaced an in-UI indicator: %q", before)
	}

	// Visiting the failed file surfaces the failure: its overlay opens.
	m, _ = update(t, m, keyPress("n"))
	if m.overlay == nil {
		t.Fatal("visiting the failed file did not open its overlay")
	}
	if v := m.View().Content; !strings.Contains(v, "cannot read b.txt: denied") {
		t.Fatalf("visited failure lacks its diagnostic: %q", v)
	}
}

// Entry into a failed file from a different file runs the re-entry
// sequence: the prior failure's overlay opens immediately, the panel
// switches back to "Loading…", and exactly one retry load is issued —
// one load per path.
func TestCrossFileReEntryRetriesOnce(t *testing.T) {
	dir := t.TempDir()
	var recs []string
	recs = append(recs, fileWithStops(t, dir, "a.txt", 10, 1)...)
	recs = append(recs, fileWithStops(t, dir, "b.txt", 10, 3)...)
	recs = append(recs, recSummary)
	m, cmd := loaderModel(t, dir, 80, 24, failLoaderFor("b.txt", errRead), recs...)
	m = settle(t, m, cmd)

	// b.txt fails while it is the current file.
	m, cmdB := update(t, m, keyPress("n"))
	m = settle(t, m, cmdB)
	if got := contentRow1(t, m); !strings.Contains(got, "(unreadable)") {
		t.Fatalf("failed b.txt panel = %q, want (unreadable)", got)
	}
	m, _ = pressKey(t, m, "esc")
	m, _ = update(t, m, keyPress("p")) // back to a.txt

	// Re-entering b.txt opens the prior-failure overlay at once, puts
	// the panel back on "Loading…", and issues exactly one retry.
	m, c := update(t, m, keyPress("n"))
	if m.overlay == nil {
		t.Fatal("re-entry did not re-open the prior-failure overlay")
	}
	if v := m.View().Content; !strings.Contains(v, "cannot read b.txt: denied") {
		t.Fatalf("re-entry overlay lacks the prior failure: %q", v)
	}
	if got := contentRow1(t, m); !strings.Contains(got, "Loading…") {
		t.Fatalf("re-entry panel = %q, want Loading… for the in-flight retry", got)
	}
	if m.loading["b.txt"] == 0 {
		t.Fatal("re-entry minted no retry request")
	}
	loads := 0
	for _, msg := range cmdMsgs(c) {
		if ld, ok := msg.(loadDoneMsg); ok && string(ld.path) == "b.txt" {
			loads++
		}
	}
	if loads != 1 {
		t.Fatalf("re-entry produced %d b.txt load completions, want exactly one", loads)
	}
}

// The unreadable state composes cleanly at every size: the filename
// row keeps identifying the — truncated, escaped — path, the panel
// shows the placeholder clipped to its slot, no frame row overflows
// the terminal, and the layout dimensions stay nonnegative.
func TestUnreadableComposedViewStaysWellFormed(t *testing.T) {
	dir := t.TempDir()
	// A path needing escapes and wide enough to force truncation: its
	// safe form carries \n and \t literally and far exceeds the slot.
	long := "deep/nested/dir/file\nwith\ttabs-and-a-very-long-tail.txt"
	q := strconv.Quote(long)
	recs := []string{
		fmt.Sprintf(`{"type":"begin","data":{"path":{"text":%s}}}`, q),
		fmt.Sprintf(`{"type":"match","data":{"path":{"text":%s},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`, q),
		fmt.Sprintf(`{"type":"end","data":{"path":{"text":%s},"binary_offset":null}}`, q),
		recSummary,
	}
	for _, sz := range []struct{ w, h int }{{80, 24}, {40, 10}, {26, 6}, {20, 3}} {
		t.Run(fmt.Sprintf("%dx%d", sz.w, sz.h), func(t *testing.T) {
			m, cmd := loaderModel(t, dir, sz.w, sz.h, failLoader(errRead), recs...)
			m = settle(t, m, cmd)
			m, _ = pressKey(t, m, "esc") // drop the overlay to see the composed frame

			v := m.View().Content
			rows := strings.Split(v, "\n")
			if len(rows) != sz.h {
				t.Fatalf("frame has %d rows, want %d — the escaped path must stay one line", len(rows), sz.h)
			}
			for i, row := range rows {
				if w := ansi.StringWidth(row); w > sz.w {
					t.Fatalf("row %d is %d cells wide, want ≤%d: %q", i, w, sz.w, row)
				}
			}
			if m.listW < 0 || m.textW < 0 {
				t.Fatalf("negative layout: listW=%d textW=%d", m.listW, m.textW)
			}
			// The truncated safe path always keeps its "…" marker;
			// where the slot is wide enough the filename row keeps the
			// path's tail legible next to the status note.
			if !strings.Contains(rows[0], "…") {
				t.Fatalf("filename row lost the truncated safe path: %q", rows[0])
			}
			if sz.w == 80 && !strings.Contains(rows[0], "long-tail.txt") {
				t.Fatalf("filename row lost the path's tail at 80 cells: %q", rows[0])
			}
			if sz.w >= 26 && !strings.Contains(rows[1], "(unreadable)") {
				t.Fatalf("panel row lacks the unreadable placeholder: %q", rows[1])
			}
		})
	}
}

// The re-entry sequence, gated: entering a previously failed file from
// a different file opens the prior-failure overlay and returns the
// panel to "Loading…" immediately, and exactly one retry load runs
// while the overlay is up. Esc dismisses the overlay without
// disturbing the in-flight load; settlement updates the panel without
// waiting for any dismissal.
func TestReEntrySequenceGated(t *testing.T) {
	dir := t.TempDir()
	var recs []string
	recs = append(recs, fileWithStops(t, dir, "a.txt", 10, 1)...)
	recs = append(recs, fileWithStops(t, dir, "b.txt", 10, 3)...)
	recs = append(recs, recSummary)
	gate := make(chan struct{})
	m, cmd := gatedLoaderModel(t, dir, nil, failLoaderFor("b.txt", errRead), recs...)
	m = settle(t, m, cmd)

	// First visit: b.txt fails as the current file.
	m, cmdB := update(t, m, keyPress("n"))
	m = settle(t, m, cmdB)
	if m.overlay == nil {
		t.Fatal("the first failure did not open the overlay")
	}
	m, _ = pressKey(t, m, "esc")
	m, _ = update(t, m, keyPress("p")) // back to a.txt

	// Arm the gate and re-enter: the prior-failure overlay and the
	// "Loading…" panel appear at once, with exactly one retry issued.
	m.loadGate = gate
	m, retry := update(t, m, keyPress("n"))
	if m.overlay == nil {
		t.Fatal("re-entry did not open the prior-failure overlay")
	}
	if v := m.View().Content; !strings.Contains(v, "cannot read b.txt: denied") {
		t.Fatalf("re-entry overlay lacks the prior failure: %q", v)
	}
	if got := contentRow1(t, m); !strings.Contains(got, "Loading…") {
		t.Fatalf("re-entry panel = %q, want Loading…", got)
	}
	req := m.loading["b.txt"]
	if req == 0 {
		t.Fatal("re-entry minted no retry request")
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- retry() }()
	select {
	case <-done:
		t.Fatal("the retry completed while its gate was held")
	case <-time.After(50 * time.Millisecond):
	}

	// Esc dismisses the overlay; the in-flight load is undisturbed.
	m, _ = pressKey(t, m, "esc")
	if m.overlay != nil {
		t.Fatal("Esc did not dismiss the overlay")
	}
	select {
	case <-done:
		t.Fatal("dismissing the overlay cancelled the retry")
	case <-time.After(50 * time.Millisecond):
	}
	if got := contentRow1(t, m); !strings.Contains(got, "Loading…") {
		t.Fatalf("panel after dismissal = %q, want Loading… still", got)
	}

	// Settlement with a second failure updates the panel without
	// waiting on any dismissal: "(unreadable)" returns and exactly one
	// new occurrence is collected and shown.
	close(gate)
	diags := len(m.diags)
	m = pump(t, m, <-done)
	if got := contentRow1(t, m); !strings.Contains(got, "(unreadable)") {
		t.Fatalf("panel after the failed retry = %q, want (unreadable)", got)
	}
	if len(m.diags) != diags+1 {
		t.Fatalf("second failure collected %d diagnostics, want one", len(m.diags)-diags)
	}
	if m.overlay == nil {
		t.Fatal("the second failure did not re-notify the current file")
	}
}

// With the re-entry overlay still open, a second failure appends
// exactly one new diagnostic occurrence without moving the reader's
// scroll position — the append-preserving-scroll primitive — and
// collects exactly one new occurrence for the exit replay.
func TestReEntrySecondFailureAppendsPreservingScroll(t *testing.T) {
	dir := t.TempDir()
	var recs []string
	recs = append(recs, fileWithStops(t, dir, "a.txt", 10, 1)...)
	recs = append(recs, fileWithStops(t, dir, "b.txt", 10, 3)...)
	recs = append(recs, recSummary)
	// A tall diagnostic makes the re-entry overlay scrollable so the
	// reader can be mid-file when the second occurrence lands.
	var b strings.Builder
	b.WriteString("denied")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&b, "\ncontext %02d", i)
	}
	tall := errors.New(b.String())
	gate := make(chan struct{})
	m, cmd := gatedLoaderModel(t, dir, nil, failLoaderFor("b.txt", tall), recs...)
	m = settle(t, m, cmd)

	m, cmdB := update(t, m, keyPress("n"))
	m = settle(t, m, cmdB) // first failure
	m, _ = pressKey(t, m, "esc")
	m, _ = update(t, m, keyPress("p"))

	// Re-entry with the retry parked: scroll the open overlay down.
	m.loadGate = gate
	m, retry := update(t, m, keyPress("n"))
	done := make(chan tea.Msg, 1)
	go func() { done <- retry() }()
	for i := 0; i < 5; i++ {
		m, _ = pressKey(t, m, "down")
	}
	if m.overlay == nil || m.overlay.scroll != 5 {
		t.Fatalf("overlay scroll = %v, want the reader 5 rows down", m.overlay)
	}
	before := len(m.diags)
	lines := len(m.overlay.lines)

	// The second failure appends one occurrence; the reader's position
	// does not move.
	close(gate)
	m = pump(t, m, <-done)
	if m.overlay == nil {
		t.Fatal("the overlay closed on the second failure")
	}
	if m.overlay.scroll != 5 {
		t.Fatalf("append moved the reader's scroll to %d, want 5", m.overlay.scroll)
	}
	if got := overlayOccurrences(m, "cannot read b.txt"); got != 2 {
		t.Fatalf("overlay holds %d failure occurrences, want 2", got)
	}
	if len(m.overlay.lines) != 2*lines {
		t.Fatalf("overlay grew to %d lines from %d, want one appended occurrence", len(m.overlay.lines), lines)
	}
	if len(m.diags) != before+1 {
		t.Fatalf("second failure collected %d diagnostics, want one", len(m.diags)-before)
	}
}

// A retry that succeeds collects nothing new: the content replaces
// "Loading…" while the prior-failure overlay stays displayed until the
// user dismisses it.
func TestReEntryRetrySuccessKeepsPriorOverlay(t *testing.T) {
	dir := t.TempDir()
	var recs []string
	recs = append(recs, fileWithStops(t, dir, "a.txt", 10, 1)...)
	recs = append(recs, fileWithStops(t, dir, "b.txt", 10, 3)...)
	recs = append(recs, recSummary)
	failing := true
	read := func(p []byte) ([]byte, error) {
		if failing && bytes.HasSuffix(p, []byte("b.txt")) {
			return nil, errRead
		}
		return filebuffer.ReadFile(p)
	}
	gate := make(chan struct{})
	m, cmd := gatedLoaderModel(t, dir, nil, read, recs...)
	m = settle(t, m, cmd)

	m, cmdB := update(t, m, keyPress("n"))
	m = settle(t, m, cmdB) // first failure
	m, _ = pressKey(t, m, "esc")
	m, _ = update(t, m, keyPress("p"))

	// Re-entry with the retry parked; the loader now answers content.
	m.loadGate = gate
	failing = false
	m, retry := update(t, m, keyPress("n"))
	done := make(chan tea.Msg, 1)
	go func() { done <- retry() }()
	if got := contentRow1(t, m); !strings.Contains(got, "Loading…") {
		t.Fatalf("re-entry panel = %q, want Loading…", got)
	}

	close(gate)
	diags := len(m.diags)
	m = pump(t, m, <-done)
	if m.bufs["b.txt"] == nil || m.failed["b.txt"] {
		t.Fatal("the successful retry did not install b.txt's buffer")
	}
	if len(m.diags) != diags {
		t.Fatalf("a successful retry collected %d diagnostics, want none", len(m.diags)-diags)
	}
	if m.overlay == nil {
		t.Fatal("a successful retry dismissed the prior-failure overlay")
	}
	if v := ansi.Strip(m.View().Content); !strings.Contains(v, "hit00003") {
		t.Fatalf("content did not replace Loading… under the open overlay: %q", v)
	}
	// The overlay remains until the user dismisses it; the content is
	// beneath it.
	m, _ = pressKey(t, m, "esc")
	if v := ansi.Strip(m.View().Content); !strings.Contains(v, "hit00003") || strings.Contains(v, "Loading…") {
		t.Fatalf("post-dismissal view = %q, want b.txt's content", v)
	}
}

// Navigating away while the retry is in flight lets it settle as a
// non-current completion — a second failure there is diagnostic-only —
// and a later re-entry runs the same sequence against the new prior
// state.
func TestReEntryRetrySettlesAfterNavigatingAway(t *testing.T) {
	dir := t.TempDir()
	var recs []string
	recs = append(recs, fileWithStops(t, dir, "a.txt", 10, 1)...)
	recs = append(recs, fileWithStops(t, dir, "b.txt", 10, 3)...)
	recs = append(recs, recSummary)
	gate := make(chan struct{})
	m, cmd := gatedLoaderModel(t, dir, nil, failLoaderFor("b.txt", errRead), recs...)
	m = settle(t, m, cmd)

	m, cmdB := update(t, m, keyPress("n"))
	m = settle(t, m, cmdB) // first failure
	m, _ = pressKey(t, m, "esc")
	m, _ = update(t, m, keyPress("p"))

	// Re-entry with the retry parked; dismiss, then leave for a.txt
	// while the load is still in flight.
	m.loadGate = gate
	m, retry := update(t, m, keyPress("n"))
	done := make(chan tea.Msg, 1)
	go func() { done <- retry() }()
	m, _ = pressKey(t, m, "esc")
	m, _ = update(t, m, keyPress("p"))

	// Settlement while a.txt is current is per Issue #25: the path's
	// status updates and the failure is diagnostic-only.
	close(gate)
	diags := len(m.diags)
	m = pump(t, m, <-done)
	if m.overlay != nil {
		t.Fatal("a retry settling while non-current opened an overlay")
	}
	if len(m.diags) != diags+1 {
		t.Fatalf("the away-settled failure collected %d diagnostics, want one", len(m.diags)-diags)
	}
	if !m.failed["b.txt"] {
		t.Fatal("the away-settled failure did not update b.txt's status")
	}

	// A later re-entry follows the same sequence against the new
	// prior state: overlay immediately, "Loading…", one retry.
	m.loadGate = nil
	m, c := update(t, m, keyPress("n"))
	if m.overlay == nil {
		t.Fatal("the later re-entry did not open the prior-failure overlay")
	}
	if v := m.View().Content; !strings.Contains(v, "cannot read b.txt: denied") {
		t.Fatalf("later re-entry overlay lacks the failure: %q", v)
	}
	if got := contentRow1(t, m); !strings.Contains(got, "Loading…") {
		t.Fatalf("later re-entry panel = %q, want Loading…", got)
	}
	loads := 0
	for _, msg := range cmdMsgs(c) {
		if ld, ok := msg.(loadDoneMsg); ok && string(ld.path) == "b.txt" {
			loads++
		}
	}
	if loads != 1 {
		t.Fatalf("later re-entry produced %d b.txt loads, want exactly one", loads)
	}
}

// Re-entering a failed file whose retry is already in flight issues no
// second load — the request is dropped, not queued — while the
// prior-failure overlay still opens.
func TestReEntryDuringInflightRetryIsDropped(t *testing.T) {
	dir := t.TempDir()
	var recs []string
	recs = append(recs, fileWithStops(t, dir, "a.txt", 10, 1)...)
	recs = append(recs, fileWithStops(t, dir, "b.txt", 10, 3)...)
	recs = append(recs, recSummary)
	gate := make(chan struct{})
	defer close(gate)
	m, cmd := gatedLoaderModel(t, dir, nil, failLoaderFor("b.txt", errRead), recs...)
	m = settle(t, m, cmd)

	m, cmdB := update(t, m, keyPress("n"))
	m = settle(t, m, cmdB) // first failure
	m, _ = pressKey(t, m, "esc")
	m, _ = update(t, m, keyPress("p"))

	m.loadGate = gate
	m, retry := update(t, m, keyPress("n"))
	req := m.loading["b.txt"]
	done := make(chan tea.Msg, 1)
	go func() { done <- retry() }()

	// Away and back while the retry is parked: the overlay re-opens
	// but no second request is minted.
	m, _ = pressKey(t, m, "esc")
	m, _ = update(t, m, keyPress("p"))
	m, again := update(t, m, keyPress("n"))
	if m.overlay == nil {
		t.Fatal("re-entry during the in-flight retry did not re-open the overlay")
	}
	if again != nil {
		t.Fatalf("re-entry during the in-flight retry produced a command %T", again)
	}
	if m.loading["b.txt"] != req {
		t.Fatalf("the in-flight request %d was replaced by %d", req, m.loading["b.txt"])
	}
}

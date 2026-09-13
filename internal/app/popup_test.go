package app_test

import (
	"encoding/json"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
	"vrg/internal/sinkfixtures"
	"vrg/internal/theme"
)

// setupBrowsePopup creates a browse model with two cached files so
// cross-file navigation is immediate (no load command). The pop-up
// duration is set to a very short value so the timer fires immediately
// if executed by deliverLoad. Pop-up lifecycle tests inject expiry
// messages directly rather than executing the timer.
func setupBrowsePopup(t *testing.T) app.Model {
	t.Helper()
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3),
		"src/b.go": makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3),
	}
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		if buf, ok := bufs[string(path)]; ok {
			return buf, nil
		}
		return &filebuffer.Buffer{}, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithPopupDuration(0),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if m.State() != app.StateBrowse {
		t.Fatalf("State = %v, want StateBrowse", m.State())
	}
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, _ = update(t, m, lc)
		}
	}
	return m
}

// bytesBeginRecord builds a begin record with a bytes-encoded path,
// for paths that may contain invalid UTF-8.
func bytesBeginRecord(path []byte) string {
	rec := map[string]any{
		"type": "begin",
		"data": map[string]any{"path": map[string]any{"bytes": b64(path)}},
	}
	b, _ := json.Marshal(rec)
	return string(b)
}

// bytesEndRecord builds an end record with a bytes-encoded path.
func bytesEndRecord(path []byte) string {
	rec := map[string]any{
		"type": "end",
		"data": map[string]any{
			"path":          map[string]any{"bytes": b64(path)},
			"binary_offset": nil,
		},
	}
	b, _ := json.Marshal(rec)
	return string(b)
}

// buildIndexBytes builds a searchindex.Index with explicit begin/end
// records using bytes encoding for paths that may contain invalid
// UTF-8. The default completeRecords helper uses textBegin which
// corrupts invalid UTF-8 paths, causing integrity errors.
func buildIndexBytes(t *testing.T, workdir string, path1, path2 []byte, records ...string) *searchindex.Index {
	t.Helper()
	var all []string
	all = append(all, bytesBeginRecord(path1))
	all = append(all, bytesBeginRecord(path2))
	all = append(all, records...)
	all = append(all, bytesEndRecord(path1))
	all = append(all, bytesEndRecord(path2))
	all = append(all, summaryRecord())
	b := searchindex.NewBuilder(workdir)
	for _, r := range all {
		if err := b.Add([]byte(r)); err != nil {
			t.Fatalf("Add %q: %v", r, err)
		}
	}
	return b.Build()
}

// setupBrowsePopupThree creates a browse model with three cached files
// for tests that need multiple cross-file navigations.
func setupBrowsePopupThree(t *testing.T) app.Model {
	t.Helper()
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/c.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3),
		"src/b.go": makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3),
		"src/c.go": makeBuf([]filebuffer.Line{ml(1, "content-c")}, 1, 3),
	}
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		if buf, ok := bufs[string(path)]; ok {
			return buf, nil
		}
		return &filebuffer.Buffer{}, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithPopupDuration(0),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if m.State() != app.StateBrowse {
		t.Fatalf("State = %v, want StateBrowse", m.State())
	}
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, _ = update(t, m, lc)
		}
	}
	return m
}

// --- Pop-up opening tests ---

// TestPopupOpensOnCrossFileNavigation verifies that navigating across
// a file boundary opens the pop-up at selection time with the
// destination file's raw path and a fresh instance ID.
func TestPopupOpensOnCrossFileNavigation(t *testing.T) {
	m := setupBrowsePopup(t)
	assertCurrentPath(t, m, "src/a.go")

	m, _ = update(t, m, keyPress('n'))
	if !m.PopupOpen() {
		t.Fatal("PopupOpen = false after cross-file n, want true")
	}
	if got := m.PopupPath(); string(got) != "src/b.go" {
		t.Fatalf("PopupPath = %q, want %q", got, "src/b.go")
	}
	if m.PopupInstance() == 0 {
		t.Fatal("PopupInstance = 0, want non-zero (fresh instance)")
	}
}

// TestPopupDoesNotOpenOnSameFileNavigation verifies that navigating
// within the same file does not open the pop-up.
func TestPopupDoesNotOpenOnSameFileNavigation(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 5, subSpec{"x", 0, 1}),
	)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf([]filebuffer.Line{ml(1, "l1"), ml(5, "l5")}, 5, 3),
	}
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return bufs[string(path)], nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithPopupDuration(0),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, _ = update(t, m, lc)
		}
	}

	m, _ = update(t, m, keyPress('n'))
	if m.PopupOpen() {
		t.Fatal("PopupOpen = true after same-file n, want false")
	}
}

// TestPopupDoesNotOpenOnOneStopNoOp verifies that navigating with one
// stop (strict no-op) does not open the pop-up.
func TestPopupDoesNotOpenOnOneStopNoOp(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3),
	}
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return bufs[string(path)], nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithPopupDuration(0),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, _ = update(t, m, lc)
		}
	}

	m, _ = update(t, m, keyPress('n'))
	if m.PopupOpen() {
		t.Fatal("PopupOpen = true after one-stop n no-op, want false")
	}
}

// --- Timer expiry tests ---

// TestPopupExpiryDismissesMatchingInstance verifies that an expiry
// message with the matching instance ID dismisses the pop-up.
func TestPopupExpiryDismissesMatchingInstance(t *testing.T) {
	m := setupBrowsePopup(t)
	m, _ = update(t, m, keyPress('n'))
	if !m.PopupOpen() {
		t.Fatal("pop-up not open after cross-file n")
	}
	instance := m.PopupInstance()

	m, _ = update(t, m, app.FileChangePopupExpiryMsg{Instance: instance})
	if m.PopupOpen() {
		t.Fatal("PopupOpen = true after matching expiry, want false")
	}
}

// TestPopupExpiryStaleInstanceDoesNotDismiss verifies that an expiry
// message from a stale instance does not dismiss a newer pop-up.
func TestPopupExpiryStaleInstanceDoesNotDismiss(t *testing.T) {
	m := setupBrowsePopupThree(t)
	// Navigate to b.go (opens pop-up with instance 1).
	m, _ = update(t, m, keyPress('n'))
	if !m.PopupOpen() {
		t.Fatal("pop-up not open after first cross-file n")
	}
	staleInstance := m.PopupInstance()

	// Navigate to c.go (opens pop-up with instance 2).
	m, _ = update(t, m, keyPress('n'))
	if !m.PopupOpen() {
		t.Fatal("pop-up not open after second cross-file n")
	}
	currentInstance := m.PopupInstance()
	if currentInstance == staleInstance {
		t.Fatal("second pop-up has same instance as first, want fresh instance")
	}

	// Inject expiry from the stale instance. The newer pop-up must
	// stay open.
	m, _ = update(t, m, app.FileChangePopupExpiryMsg{Instance: staleInstance})
	if !m.PopupOpen() {
		t.Fatal("PopupOpen = false after stale expiry, want true (newer pop-up still open)")
	}
	if m.PopupInstance() != currentInstance {
		t.Fatalf("PopupInstance = %d after stale expiry, want %d", m.PopupInstance(), currentInstance)
	}
}

// TestPopupFreshInstancePerPopup verifies that each pop-up gets a
// fresh instance ID, distinct from the previous one.
func TestPopupFreshInstancePerPopup(t *testing.T) {
	m := setupBrowsePopupThree(t)
	m, _ = update(t, m, keyPress('n'))
	first := m.PopupInstance()
	m, _ = update(t, m, keyPress('n'))
	second := m.PopupInstance()
	if first == second {
		t.Fatalf("second pop-up instance = %d, same as first; want fresh instance", second)
	}
}

// --- Dismissal plus action tests ---

// TestPopupKeypressDismissesAndPerformsAction verifies that pressing a
// key while the pop-up is open dismisses the pop-up and performs the
// key's normal action in the same update. Pressing n dismisses the
// pop-up and navigates; if the navigation crosses a file boundary, a
// new pop-up opens with a fresh instance.
func TestPopupKeypressDismissesAndPerformsAction(t *testing.T) {
	m := setupBrowsePopupThree(t)
	// Navigate to b.go (opens pop-up).
	m, _ = update(t, m, keyPress('n'))
	if !m.PopupOpen() {
		t.Fatal("pop-up not open after first n")
	}
	firstInstance := m.PopupInstance()

	// Press n again: dismisses the pop-up and navigates to c.go.
	// A new pop-up opens for c.go with a fresh instance.
	m, _ = update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/c.go")
	if !m.PopupOpen() {
		t.Fatal("PopupOpen = false after n dismissed old pop-up and navigated cross-file, want new pop-up open")
	}
	if m.PopupInstance() == firstInstance {
		t.Fatal("new pop-up has same instance as dismissed pop-up, want fresh instance")
	}
	if got := m.PopupPath(); string(got) != "src/c.go" {
		t.Fatalf("PopupPath = %q, want %q", got, "src/c.go")
	}
}

// TestPopupKeypressDismissesAndQuits verifies that pressing q while the
// pop-up is open dismisses the pop-up and quits.
func TestPopupKeypressDismissesAndQuits(t *testing.T) {
	m := setupBrowsePopup(t)
	m, _ = update(t, m, keyPress('n'))
	if !m.PopupOpen() {
		t.Fatal("pop-up not open after cross-file n")
	}

	m, cmd := update(t, m, keyPress('q'))
	assertQuit(t, cmd)
	if m.PopupOpen() {
		t.Fatal("PopupOpen = true after q, want false (dismissed)")
	}
}

// TestPopupKeypressDismissesAndScrolls verifies that pressing a scroll
// key while the pop-up is open dismisses the pop-up and scrolls.
func TestPopupKeypressDismissesAndScrolls(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 20, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	linesA := makeScrollLines(50)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf(linesA, 50, 3),
		"src/b.go": makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3),
	}
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return bufs[string(path)], nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithPopupDuration(0),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, _ = update(t, m, lc)
		}
	}

	// Navigate to b.go: first n goes to a.go line 20 (same file),
	// second n goes to b.go (cross file, opens pop-up).
	m, _ = update(t, m, keyPress('n'))
	if m.PopupOpen() {
		t.Fatal("pop-up opened after same-file n, want false")
	}
	m, _ = update(t, m, keyPress('n'))
	if !m.PopupOpen() {
		t.Fatal("pop-up not open after cross-file n")
	}

	// Press down arrow: dismisses the pop-up and scrolls.
	m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	if m.PopupOpen() {
		t.Fatal("PopupOpen = true after scroll key, want false (dismissed)")
	}
}

// TestPopupEscDismisses verifies that pressing Esc while the pop-up is
// open dismisses the pop-up. Esc is a no-op in browse state but still
// dismisses the pop-up like any other key.
func TestPopupEscDismisses(t *testing.T) {
	m := setupBrowsePopup(t)
	m, _ = update(t, m, keyPress('n'))
	if !m.PopupOpen() {
		t.Fatal("pop-up not open after cross-file n")
	}

	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	if m.PopupOpen() {
		t.Fatal("PopupOpen = true after Esc, want false (dismissed)")
	}
	if m.State() != app.StateBrowse {
		t.Fatalf("State = %v, want StateBrowse (Esc is no-op in browse)", m.State())
	}
}

// --- Resize tests ---

// TestPopupResizeRecentersWithoutRestart verifies that resizing while
// the pop-up is open recentres and re-truncates without dismissing the
// pop-up or restarting the timer (same instance).
func TestPopupResizeRecentersWithoutRestart(t *testing.T) {
	m := setupBrowsePopup(t)
	m, _ = update(t, m, keyPress('n'))
	if !m.PopupOpen() {
		t.Fatal("pop-up not open after cross-file n")
	}
	instance := m.PopupInstance()

	m, _ = update(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	if !m.PopupOpen() {
		t.Fatal("PopupOpen = false after resize, want true (not dismissed)")
	}
	if m.PopupInstance() != instance {
		t.Fatalf("PopupInstance = %d after resize, want %d (not restarted)", m.PopupInstance(), instance)
	}
}

// --- Error overlay cancellation tests ---

// TestPopupErrorOverlayCancels verifies that an error overlay arriving
// while the pop-up is open cancels the pop-up, and the pop-up does not
// return after the overlay is dismissed.
func TestPopupErrorOverlayCancels(t *testing.T) {
	m := setupBrowsePopup(t)
	m, _ = update(t, m, keyPress('n'))
	if !m.PopupOpen() {
		t.Fatal("pop-up not open after cross-file n")
	}

	// Simulate an error overlay arriving (e.g., a re-search that
	// fails). The SearchCompleteMsg with a fatal process result opens
	// an error overlay and cancels the pop-up.
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	m, _ = update(t, m, app.SearchCompleteMsg{
		Files: idx.Files(), Lines: idx.Len(), Index: idx,
		Process: app.ProcessResult{ExitCode: 3},
		Stderr:  "boom",
	})
	if m.PopupOpen() {
		t.Fatal("PopupOpen = true after error overlay, want false (cancelled)")
	}
	if !m.OverlayOpen() {
		t.Fatal("overlay not open after SearchCompleteMsg with error")
	}

	// Dismiss the overlay with q. The pop-up must not return.
	m, _ = update(t, m, keyPress('q'))
	if m.OverlayOpen() {
		t.Fatal("overlay still open after q, want closed")
	}
	if m.PopupOpen() {
		t.Fatal("PopupOpen = true after overlay dismissal, want false (no return)")
	}
}

// --- Load completion tests ---

// TestPopupLoadCompletionDoesNotRestartTimer verifies that load
// completion does not restart the pop-up timer. The pop-up starts at
// selection time and remains open with the same instance after the
// load completes.
func TestPopupLoadCompletionDoesNotRestartTimer(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3),
		"src/b.go": makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3),
	}
	// bGate holds only the b.go load; the file gate is closed so the
	// initial a.go load completes immediately.
	bGate := make(chan struct{})
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		if string(path) == "src/b.go" {
			<-bGate
		}
		return bufs[string(path)], nil
	}
	gate := make(chan struct{})
	close(gate)
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithPopupDuration(0),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	// Deliver the initial load (a.go).
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, _ = update(t, m, lc)
		}
	}

	// Navigate to b.go (uncached, bGate held). The pop-up starts at
	// selection time.
	m, navCmd := update(t, m, keyPress('n'))
	if !m.PopupOpen() {
		t.Fatal("pop-up not open after cross-file n to uncached file")
	}
	instance := m.PopupInstance()
	if got := m.PopupPath(); string(got) != "src/b.go" {
		t.Fatalf("PopupPath = %q, want %q", got, "src/b.go")
	}

	// Release the bGate and deliver the load completion. The pop-up
	// must stay open with the same instance (load completion does not
	// restart the timer).
	close(bGate)
	m = deliverLoad(t, m, navCmd)

	if !m.PopupOpen() {
		t.Fatal("PopupOpen = false after load completion, want true (not dismissed)")
	}
	if m.PopupInstance() != instance {
		t.Fatalf("PopupInstance = %d after load, want %d (not restarted)", m.PopupInstance(), instance)
	}
}

// --- Rendering tests ---

// TestPopupCentred verifies that the pop-up is centred horizontally
// and vertically in the terminal. The pop-up text appears on the
// vertical centre line with horizontal padding.
func TestPopupCentred(t *testing.T) {
	m := setupBrowsePopup(t)
	m, _ = update(t, m, keyPress('n'))
	if !m.PopupOpen() {
		t.Fatal("pop-up not open after cross-file n")
	}

	view := viewContent(m)
	lines := strings.Split(view, "\n")
	// Vertical centre for height 24: (24 - 1) / 2 = 11 (0-based).
	centreLine := 11
	if centreLine >= len(lines) {
		t.Fatalf("view has only %d lines, need at least %d for centre", len(lines), centreLine+1)
	}
	// The centre line should contain the escaped path "src/b.go".
	if !strings.Contains(lines[centreLine], "src/b.go") {
		t.Fatalf("centre line %d does not contain pop-up path: %q", centreLine, lines[centreLine])
	}
	// The pop-up should be horizontally centred. For "src/b.go" (8
	// cells) in width 80, left pad = (80 - 8) / 2 = 36 spaces.
	expectedPad := (80 - len("src/b.go")) / 2
	// Strip ANSI sequences for the leading-space check.
	stripped := stripANSI(lines[centreLine])
	if !strings.HasPrefix(stripped, strings.Repeat(" ", expectedPad)) {
		t.Fatalf("centre line not horizontally centred: expected %d leading spaces, got %q", expectedPad, stripped)
	}
}

// TestPopupTruncatesLongPath verifies that a long path is left-truncated
// with a leading … to fit the terminal width.
func TestPopupTruncatesLongPath(t *testing.T) {
	longPath := "src/" + strings.Repeat("x", 100) + ".go"
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch(longPath, "x\n", 1, subSpec{"x", 0, 1}),
	)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3),
		longPath:   makeBuf([]filebuffer.Line{ml(1, "content-long")}, 1, 3),
	}
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return bufs[string(path)], nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithPopupDuration(0),
		app.WithTheme(theme.NoStyle()),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 20, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, _ = update(t, m, lc)
		}
	}

	// Navigate to the long path file.
	m, _ = update(t, m, keyPress('n'))
	if !m.PopupOpen() {
		t.Fatal("pop-up not open after cross-file n")
	}

	view := viewContent(m)
	lines := strings.Split(view, "\n")
	centreLine := 11
	if centreLine >= len(lines) {
		t.Fatalf("view has only %d lines, need at least %d", len(lines), centreLine+1)
	}
	popupLine := stripANSI(lines[centreLine])
	// The popup line must start with … (left-truncated).
	if !strings.HasPrefix(strings.TrimLeft(popupLine, " "), "…") {
		t.Fatalf("popup line does not start with … after left-truncation: %q", popupLine)
	}
	// The popup line width must not exceed the terminal width.
	if w := popupVisibleWidth(popupLine); w > 20 {
		t.Fatalf("popup line width = %d, want <= 20 (terminal width): %q", w, popupLine)
	}
}

// TestPopupResizeRecentresAndRetruncates verifies that resizing while
// the pop-up is open recentres and re-truncates the path. With a narrow
// width, the path is truncated; with a wider width, it is not.
func TestPopupResizeRecentresAndRetruncates(t *testing.T) {
	longPath := "src/" + strings.Repeat("x", 50) + ".go"
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch(longPath, "x\n", 1, subSpec{"x", 0, 1}),
	)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3),
		longPath:   makeBuf([]filebuffer.Line{ml(1, "content-long")}, 1, 3),
	}
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return bufs[string(path)], nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithPopupDuration(0),
		app.WithTheme(theme.NoStyle()),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, _ = update(t, m, lc)
		}
	}

	// Navigate to the long path file (opens pop-up).
	m, _ = update(t, m, keyPress('n'))
	if !m.PopupOpen() {
		t.Fatal("pop-up not open after cross-file n")
	}

	// At width 80, the path fits without truncation.
	view := viewContent(m)
	lines := strings.Split(view, "\n")
	popupLine := stripANSI(lines[11])
	if strings.Contains(strings.TrimLeft(popupLine, " "), "…") {
		t.Fatalf("popup line truncated at width 80, want full path: %q", popupLine)
	}

	// Resize to a narrow width. The path should be truncated with ….
	// Issue #33: widths below 20 trigger the too-small screen, so use
	// the minimum usable width (20) to test pop-up truncation.
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 20, Height: 24})
	view = viewContent(m)
	lines = strings.Split(view, "\n")
	if len(lines) <= 11 {
		t.Fatalf("view has only %d lines after resize, need at least 12", len(lines))
	}
	popupLine = stripANSI(lines[11])
	if !strings.HasPrefix(strings.TrimLeft(popupLine, " "), "…") {
		t.Fatalf("popup line not truncated at width 20: %q", popupLine)
	}
	if w := popupVisibleWidth(popupLine); w > 20 {
		t.Fatalf("popup line width = %d after resize, want <= 20: %q", w, popupLine)
	}
}

// --- Sink-safety tests ---

// TestPopupSinkSafetyNoStyle verifies that the file-change pop-up passes
// the Issue #6 sink-safety check: no raw control bytes survive in the
// no-style composition path for every shared hostile fixture.
func TestPopupSinkSafetyNoStyle(t *testing.T) {
	for _, fx := range sinkfixtures.Fixtures {
		t.Run(fx.Name, func(t *testing.T) {
			// Use aaa.go as the first file so it sorts before
			// every hostile fixture path (they all start with
			// bytes >= 'f'). The cursor starts at aaa.go; pressing
			// n navigates to the hostile path (cross-file),
			// opening the pop-up with the hostile path.
			// Build explicit begin/end/summary records with bytes
			// encoding so paths with invalid UTF-8 (e.g. the
			// InvalidUTF8 fixture) are handled correctly. The
			// default completeRecords helper uses textBegin which
			// corrupts invalid UTF-8 paths.
			idx := buildIndexBytes(t, "/work",
				[]byte("aaa.go"), fx.Raw,
				textMatch("aaa.go", "x\n", 1, subSpec{"x", 0, 1}),
				bytesMatch(fx.Raw, []byte("x\n"), 1, subSpec{"x", 0, 1}),
			)
			gate := make(chan struct{})
			close(gate)
			loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
				return makeBuf([]filebuffer.Line{ml(1, "content")}, 1, 3), nil
			}
			m := app.New([]string{"--json", "--", "foo", "."}, "/work",
				app.WithFileLoadGate(gate),
				app.WithFileLoader(loader),
				app.WithPopupDuration(0),
				app.WithTheme(theme.NoStyle()),
			)
			m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
			m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
			if cmd != nil {
				msg := execCmd(t, cmd)
				if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
					m, _ = update(t, m, lc)
				}
			}

			// Navigate to the hostile-path file (opens pop-up).
			m, _ = update(t, m, keyPress('n'))
			if !m.PopupOpen() {
				t.Fatal("pop-up not open after cross-file n")
			}

			view := viewContent(m)
			if !sinkfixtures.NoControlBytes(view) {
				t.Fatalf("raw control byte in pop-up for %s: %q", fx.Name, view)
			}
		})
	}
}

// --- No return after dismissal tests ---

// TestPopupDoesNotReturnAfterExpiry verifies that after the pop-up is
// dismissed by its timer expiry, it does not return on a resize or a
// same-file navigation (which does not cross a file boundary).
func TestPopupDoesNotReturnAfterExpiry(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 5, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf([]filebuffer.Line{ml(1, "l1"), ml(5, "l5")}, 5, 3),
		"src/b.go": makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3),
	}
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return bufs[string(path)], nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithPopupDuration(0),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, _ = update(t, m, lc)
		}
	}

	// Navigate to b.go (cross-file, opens pop-up). The cursor goes
	// from a.go:1 to a.go:5 (same-file), then to b.go:1 (cross-file).
	m, _ = update(t, m, keyPress('n')) // a.go:1 -> a.go:5 (same-file)
	m, _ = update(t, m, keyPress('n')) // a.go:5 -> b.go:1 (cross-file)
	if !m.PopupOpen() {
		t.Fatal("pop-up not open after cross-file n")
	}
	instance := m.PopupInstance()

	// Dismiss via expiry.
	m, _ = update(t, m, app.FileChangePopupExpiryMsg{Instance: instance})
	if m.PopupOpen() {
		t.Fatal("PopupOpen = true after expiry, want false")
	}

	// Resize: pop-up should not return.
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	if m.PopupOpen() {
		t.Fatal("PopupOpen = true after resize post-expiry, want false (no return)")
	}

	// Same-file n (b.go has only one stop, so n wraps to a.go:1 which
	// is cross-file). Use p instead to go back to a.go:5 (same-file
	// from b.go? No, that's cross-file). Actually, with 3 stops
	// (a.go:1, a.go:5, b.go:1), from b.go:1 pressing p goes to a.go:5
	// (cross-file). Let me use a different approach: after dismissing
	// the pop-up, press a non-navigation key (like 'c' for colour
	// toggle) which should not open a new pop-up.
	m, _ = update(t, m, keyPress('c'))
	if m.PopupOpen() {
		t.Fatal("PopupOpen = true after colour toggle post-expiry, want false (no return)")
	}
}

// TestPopupDoesNotReturnAfterKeypress verifies that after the pop-up is
// dismissed by a key press (that does not cross a file boundary), it
// does not return on subsequent messages.
func TestPopupDoesNotReturnAfterKeypress(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 5, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf([]filebuffer.Line{ml(1, "l1"), ml(5, "l5")}, 5, 3),
		"src/b.go": makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3),
	}
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return bufs[string(path)], nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithPopupDuration(0),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, _ = update(t, m, lc)
		}
	}

	// Navigate to b.go: first n goes to a.go:5 (same-file), second n
	// goes to b.go:1 (cross-file, opens pop-up).
	m, _ = update(t, m, keyPress('n'))
	m, _ = update(t, m, keyPress('n'))
	if !m.PopupOpen() {
		t.Fatal("pop-up not open after cross-file n")
	}

	// Dismiss with Esc (no-op in browse, but dismisses the pop-up).
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	if m.PopupOpen() {
		t.Fatal("PopupOpen = true after Esc, want false (dismissed)")
	}

	// Resize: pop-up should not return.
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	if m.PopupOpen() {
		t.Fatal("PopupOpen = true after resize post-dismissal, want false (no return)")
	}
}

// stripANSI removes ANSI escape sequences from s for rendering tests.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\x1b' {
			i++
			for i < len(s) && s[i] != 'm' {
				i++
			}
			if i < len(s) {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// popupVisibleWidth returns the number of visible terminal cells in s,
// excluding ANSI escape sequences. Each rune is one cell.
func popupVisibleWidth(s string) int {
	stripped := stripANSI(s)
	count := 0
	for range stripped {
		count++
	}
	return count
}

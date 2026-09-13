package app_test

import (
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
)

// --- Issue #26 test helpers ---

// failingLoader is a test FileLoader that returns an error for
// registered failing paths and a buffer for others. It counts calls
// per path so tests can verify the retry rules (same-file step → no
// reload, cross-file entry → exactly one reload).
type failingLoader struct {
	mu    sync.Mutex
	bufs  map[string]*filebuffer.Buffer
	fails map[string]string
	calls map[string]int
}

func newFailingLoader() *failingLoader {
	return &failingLoader{
		bufs:  make(map[string]*filebuffer.Buffer),
		fails: make(map[string]string),
		calls: make(map[string]int),
	}
}

func (l *failingLoader) setBuffer(path string, buf *filebuffer.Buffer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.bufs[path] = buf
}

func (l *failingLoader) setFailing(path, errMsg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fails[path] = errMsg
}

func (l *failingLoader) clearFailing(path string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, path)
}

func (l *failingLoader) load(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
	l.mu.Lock()
	l.calls[string(path)]++
	errMsg, fail := l.fails[string(path)]
	buf := l.bufs[string(path)]
	l.mu.Unlock()
	if fail {
		return nil, errors.New(errMsg)
	}
	if buf != nil {
		return buf, nil
	}
	return &filebuffer.Buffer{}, nil
}

func (l *failingLoader) callCount(path string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.calls[path]
}

// gatedFailingLoader is a test FileLoader that combines per-path
// gating (like gatedLoader) with per-path failure injection. Closing
// a path's gate (via release) unblocks that path's load; the load
// then returns the registered error (if any) or the registered
// buffer. This lets tests control both when each file's load
// completes and whether it succeeds or fails, which is essential
// for verifying non-current failures and the re-entry retry
// sequence.
type gatedFailingLoader struct {
	mu        sync.Mutex
	bufs      map[string]*filebuffer.Buffer
	fails     map[string]string
	gates     map[string]chan struct{}
	calls     map[string]int
	started   map[string]chan struct{}
	startOnce map[string]*sync.Once
}

func newGatedFailingLoader() *gatedFailingLoader {
	return &gatedFailingLoader{
		bufs:      make(map[string]*filebuffer.Buffer),
		fails:     make(map[string]string),
		gates:     make(map[string]chan struct{}),
		calls:     make(map[string]int),
		started:   make(map[string]chan struct{}),
		startOnce: make(map[string]*sync.Once),
	}
}

func (l *gatedFailingLoader) set(path string, buf *filebuffer.Buffer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.bufs[path] = buf
	l.gates[path] = make(chan struct{})
	l.started[path] = make(chan struct{})
	l.startOnce[path] = &sync.Once{}
}

func (l *gatedFailingLoader) setFailing(path, errMsg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fails[path] = errMsg
}

func (l *gatedFailingLoader) clearFailing(path string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, path)
}

func (l *gatedFailingLoader) release(path string) {
	l.mu.Lock()
	gate, ok := l.gates[path]
	l.mu.Unlock()
	if ok {
		close(gate)
	}
}

// rearm resets the gate for the given path so the next load blocks
// until release is called. This lets tests control the timing of
// re-entry retries.
func (l *gatedFailingLoader) rearm(path string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.gates[path] = make(chan struct{})
	l.started[path] = make(chan struct{})
	l.startOnce[path] = &sync.Once{}
}

// setBuffer sets a successful buffer for the given path, clearing
// any prior failure injection.
func (l *gatedFailingLoader) setBuffer(path string, buf *filebuffer.Buffer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.bufs[path] = buf
}

func (l *gatedFailingLoader) callCount(path string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.calls[path]
}

func (l *gatedFailingLoader) waitStarted(path string) {
	l.mu.Lock()
	ch := l.started[path]
	l.mu.Unlock()
	if ch != nil {
		<-ch
	}
}

func (l *gatedFailingLoader) load(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
	l.mu.Lock()
	l.calls[string(path)]++
	gate := l.gates[string(path)]
	buf := l.bufs[string(path)]
	errMsg, fail := l.fails[string(path)]
	started := l.started[string(path)]
	once := l.startOnce[string(path)]
	l.mu.Unlock()

	if once != nil {
		once.Do(func() { close(started) })
	}

	if gate != nil {
		<-gate
	}
	if fail {
		return nil, errors.New(errMsg)
	}
	if buf != nil {
		return buf, nil
	}
	return &filebuffer.Buffer{}, nil
}

// setupBrowseFailing creates a browse model with the given failing
// loader and popup duration 0. The file gate is closed immediately so
// loads proceed to the loader. Returns the model and the async load
// channel for the startup file.
func setupBrowseFailing(t *testing.T, idx *searchindex.Index, loader func([]byte, []searchindex.Stop) (*filebuffer.Buffer, error)) (app.Model, <-chan app.FileLoadCompleteMsg) {
	t.Helper()
	gate := make(chan struct{})
	close(gate)
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
	loadCh := startLoadAsync(cmd)
	return m, loadCh
}

// setupBrowseGatedFailing creates a browse model with the given gated
// failing loader and popup duration 0. The file gate is closed
// immediately so loads proceed to the gated loader. All per-path
// gates are held; tests release them individually. Returns the model
// and the async load channel for the startup file.
func setupBrowseGatedFailing(t *testing.T, idx *searchindex.Index, loader *gatedFailingLoader) (app.Model, <-chan app.FileLoadCompleteMsg) {
	t.Helper()
	gate := make(chan struct{})
	close(gate)
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader.load),
		app.WithPopupDuration(0),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if m.State() != app.StateBrowse {
		t.Fatalf("State = %v, want StateBrowse", m.State())
	}
	loadCh := startLoadAsync(cmd)
	return m, loadCh
}

// noLineExceedsWidth checks that no visible line in the view exceeds
// the given terminal width. ANSI escape sequences are stripped before
// measuring.
func noLineExceedsWidth(view string, width int) bool {
	for _, line := range strings.Split(view, "\n") {
		if popupVisibleWidth(line) > width {
			return false
		}
	}
	return true
}

// --- PRD #38: Current-file read failure ---

// TestReadFailureCurrentFileOverlayPlaceholder verifies that a
// current-file read failure shows the Issue #9 error overlay and the
// "(unreadable)" placeholder in the panel while the file's cursor
// stops are retained and the filename row still identifies the path.
func TestReadFailureCurrentFileOverlayPlaceholder(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "y\n", 3, subSpec{"y", 0, 1}),
		textMatch("src/b.go", "z\n", 1, subSpec{"z", 0, 1}),
	)
	loader := newFailingLoader()
	loader.setFailing("src/a.go", "permission denied")
	loader.setBuffer("src/b.go", makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3))
	m, loadCh := setupBrowseFailing(t, idx, loader.load)

	// Startup file A fails to load.
	lc := <-loadCh
	m = deliverCompletion(t, m, lc)

	// Error overlay should be open (Issue #9 component).
	if !m.OverlayOpen() {
		t.Fatalf("overlay should be open after current-file read failure")
	}
	if m.OverlayKind() != app.OverlayError {
		t.Fatalf("OverlayKind = %v, want OverlayError", m.OverlayKind())
	}
	if m.OverlayFatal() {
		t.Fatalf("read-failure overlay should be non-fatal (dismissal returns to browse)")
	}

	// Dismiss the overlay to see the panel content.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	if m.OverlayOpen() {
		t.Fatalf("overlay should be closed after Esc")
	}

	// Panel should show "(unreadable)".
	view := viewContent(m)
	if !strings.Contains(view, "(unreadable)") {
		t.Fatalf("view should contain '(unreadable)':\n%s", view)
	}
	if strings.Contains(view, "Loading") {
		t.Fatalf("view should not contain 'Loading' after failure:\n%s", view)
	}

	// Cursor stops should be retained: n should advance within the
	// same file (A has two stops).
	cursorBefore := m.CursorPosition()
	m, _ = update(t, m, keyPress('n'))
	if m.CursorPosition() == cursorBefore {
		t.Fatalf("cursor did not advance after n (stops not retained)")
	}
	assertCurrentPath(t, m, "src/a.go")

	// Filename row should still identify the path.
	view = viewContent(m)
	if !strings.Contains(view, "a.go") {
		t.Fatalf("view should contain 'a.go' (filename row identifies path):\n%s", view)
	}

	// The error diagnostic should be collected for replay.
	diags := m.Diagnostics()
	if len(diags) == 0 {
		t.Fatalf("no diagnostics collected after current-file read failure")
	}
	found := false
	for _, d := range diags {
		if strings.Contains(d, "permission denied") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("diagnostics %q do not contain 'permission denied'", diags)
	}
}

// TestReadFailureCurrentFileDismissReturnsToBrowse verifies that
// dismissing the read-failure overlay (q or Esc) returns to the browse
// state with the "(unreadable)" placeholder still shown, not exiting.
func TestReadFailureCurrentFileDismissReturnsToBrowse(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "z\n", 1, subSpec{"z", 0, 1}),
	)
	loader := newFailingLoader()
	loader.setFailing("src/a.go", "permission denied")
	loader.setBuffer("src/b.go", makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3))
	m, loadCh := setupBrowseFailing(t, idx, loader.load)

	lc := <-loadCh
	m = deliverCompletion(t, m, lc)
	if !m.OverlayOpen() {
		t.Fatalf("overlay should be open")
	}

	// Esc dismisses the overlay (non-fatal).
	m, cmd := update(t, m, keyPressOrEscape(tea.KeyEscape))
	if m.OverlayOpen() {
		t.Fatalf("overlay should be closed after Esc")
	}
	if cmd != nil {
		msg := execCmd(t, cmd)
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Fatalf("Esc on non-fatal read-failure overlay should not quit")
		}
	}
	if m.State() != app.StateBrowse {
		t.Fatalf("State = %v, want StateBrowse after Esc dismiss", m.State())
	}

	// Panel should still show "(unreadable)".
	view := viewContent(m)
	if !strings.Contains(view, "(unreadable)") {
		t.Fatalf("view should still contain '(unreadable)' after dismiss:\n%s", view)
	}

	// q from browse should quit with the fixed exit status (0 for
	// clean rg 0).
	m, cmd = update(t, m, keyPress('q'))
	assertQuit(t, cmd)
	if m.ExitCode() != 0 {
		t.Fatalf("ExitCode = %d, want 0 (fixed status unchanged)", m.ExitCode())
	}
}

// --- PRD #37: Non-current failure is diagnostic-only ---

// TestReadFailureNonCurrentDiagnosticOnly verifies that a non-current
// file read failure is collected as a diagnostic only — no overlay,
// no in-UI indicator — and is discovered by visiting that file.
func TestReadFailureNonCurrentDiagnosticOnly(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "y\n", 1, subSpec{"y", 0, 1}),
	)
	loader := newGatedFailingLoader()
	loader.set("src/a.go", makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3))
	loader.set("src/b.go", nil)
	loader.setFailing("src/b.go", "permission denied")
	m, loadA := setupBrowseGatedFailing(t, idx, loader)

	// Release A's gate so A loads successfully.
	loader.release("src/a.go")
	lc := <-loadA
	m = deliverCompletion(t, m, lc)

	// Navigate to B (B starts loading, B is current).
	m, cmd := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	loadB := startLoadAsync(cmd)
	loader.waitStarted("src/b.go")

	// Navigate back to A (A is current, B is still loading).
	m, cmd = update(t, m, keyPress('p'))
	assertCurrentPath(t, m, "src/a.go")
	_ = startLoadAsync(cmd)

	// Release B's gate → B fails while A is current (non-current
	// failure).
	loader.release("src/b.go")
	lc = <-loadB
	m, _ = update(t, m, lc)

	// No overlay should be open.
	if m.OverlayOpen() {
		t.Fatalf("overlay should not be open for non-current failure")
	}

	// No in-UI indicator: panel should show A's content, not
	// "(unreadable)" or "Loading".
	view := viewContent(m)
	if strings.Contains(view, "(unreadable)") {
		t.Fatalf("non-current failure should not show '(unreadable)':\n%s", view)
	}
	if strings.Contains(view, "Loading") {
		t.Fatalf("non-current failure should not show 'Loading':\n%s", view)
	}
	if !strings.Contains(view, "content-a") {
		t.Fatalf("panel should show A's content:\n%s", view)
	}

	// The diagnostic should be collected for replay.
	diags := m.Diagnostics()
	found := false
	for _, d := range diags {
		if strings.Contains(d, "permission denied") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("non-current failure diagnostic not collected: %q", diags)
	}

	// Visiting the failed file (B) should show the overlay and
	// "(unreadable)".
	m, cmd = update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	loadB2 := startLoadAsync(cmd)
	lc = <-loadB2
	m = deliverCompletion(t, m, lc)
	if !m.OverlayOpen() {
		t.Fatalf("overlay should be open after visiting failed file")
	}
	// Dismiss the overlay to see the panel content.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	view = viewContent(m)
	if !strings.Contains(view, "(unreadable)") {
		t.Fatalf("view should contain '(unreadable)' after visiting failed file:\n%s", view)
	}
}

// TestReadFailureNonCurrentDiagnosticInReplay verifies that a
// non-current file read failure diagnostic appears in the stderr
// replay at exit (Issue #11).
func TestReadFailureNonCurrentDiagnosticInReplay(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "y\n", 1, subSpec{"y", 0, 1}),
	)
	loader := newGatedFailingLoader()
	loader.set("src/a.go", makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3))
	loader.set("src/b.go", nil)
	loader.setFailing("src/b.go", "permission denied")
	m, loadA := setupBrowseGatedFailing(t, idx, loader)

	// A loads successfully.
	loader.release("src/a.go")
	lc := <-loadA
	m = deliverCompletion(t, m, lc)

	// Navigate to B, then back to A.
	m, cmd := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	loadB := startLoadAsync(cmd)
	loader.waitStarted("src/b.go")
	m, cmd = update(t, m, keyPress('p'))
	assertCurrentPath(t, m, "src/a.go")
	_ = startLoadAsync(cmd)

	// B fails while A is current.
	loader.release("src/b.go")
	lc = <-loadB
	m, _ = update(t, m, lc)

	// Quit from A (current file is fine).
	m, cmd = update(t, m, keyPress('q'))
	assertQuit(t, cmd)

	// The non-current failure diagnostic should be in the replay.
	diags := m.Diagnostics()
	found := false
	for _, d := range diags {
		if strings.Contains(d, "permission denied") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("non-current failure diagnostic not in replay: %q", diags)
	}
}

// --- PRD #39: Same-file step → no reload, cross-file entry → one reload ---

// TestReadFailureSameFileStepNoRetry verifies that navigating between
// stops within the same failed file requests no reload. The loader
// call count should not increase.
func TestReadFailureSameFileStepNoRetry(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "y\n", 3, subSpec{"y", 0, 1}),
		textMatch("src/b.go", "z\n", 1, subSpec{"z", 0, 1}),
	)
	loader := newFailingLoader()
	loader.setFailing("src/a.go", "permission denied")
	loader.setBuffer("src/b.go", makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3))
	m, loadCh := setupBrowseFailing(t, idx, loader.load)

	// A fails to load.
	lc := <-loadCh
	m = deliverCompletion(t, m, lc)
	callsAfterFail := loader.callCount("src/a.go")

	// Dismiss the overlay so n/p are not captured by it.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))

	// n within the same file (A has two stops) → no reload.
	m, _ = update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/a.go")
	if got := loader.callCount("src/a.go"); got != callsAfterFail {
		t.Fatalf("loader called %d times for A after same-file n, want %d (no retry)", got, callsAfterFail)
	}

	// p back to the first stop in A → no reload.
	m, _ = update(t, m, keyPress('p'))
	assertCurrentPath(t, m, "src/a.go")
	if got := loader.callCount("src/a.go"); got != callsAfterFail {
		t.Fatalf("loader called %d times for A after same-file p, want %d (no retry)", got, callsAfterFail)
	}
}

// TestReadFailureCrossFileEntryRetries verifies that entering a failed
// file from a different file requests exactly one reload. The loader
// call count should increase by exactly one.
func TestReadFailureCrossFileEntryRetries(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "y\n", 1, subSpec{"y", 0, 1}),
	)
	loader := newFailingLoader()
	loader.setFailing("src/a.go", "permission denied")
	loader.setFailing("src/b.go", "permission denied")
	m, loadCh := setupBrowseFailing(t, idx, loader.load)

	// A fails to load (startup file).
	lc := <-loadCh
	m = deliverCompletion(t, m, lc)
	callsA := loader.callCount("src/a.go")
	callsB := loader.callCount("src/b.go")

	// Dismiss the overlay.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))

	// Navigate to B (cross-file entry) → exactly one reload for B.
	m, cmd := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	loadB := startLoadAsync(cmd)
	lc = <-loadB // wait for the load to complete
	if got := loader.callCount("src/b.go"); got != callsB+1 {
		t.Fatalf("loader called %d times for B after cross-file entry, want %d (exactly one retry)", got, callsB+1)
	}

	// B fails too. Dismiss the overlay.
	m = deliverCompletion(t, m, lc)
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))

	// Navigate back to A (cross-file entry) → exactly one reload for A.
	m, cmd = update(t, m, keyPress('p'))
	assertCurrentPath(t, m, "src/a.go")
	loadA2 := startLoadAsync(cmd)
	lc = <-loadA2 // wait for the load to complete
	if got := loader.callCount("src/a.go"); got != callsA+1 {
		t.Fatalf("loader called %d times for A after cross-file entry, want %d (exactly one retry)", got, callsA+1)
	}
}

// --- Composed-view robustness (Issue #24 slot rules) ---

// TestReadFailureComposedViewRobustness verifies that the unreadable
// state with a long escaped path at ordinary and constrained widths
// keeps the composed view well-formed: the filename row shows the
// truncated safe path per Issue #24's slot rules, the panel shows
// "(unreadable)", nothing overflows the terminal, and layout
// dimensions stay nonnegative.
func TestReadFailureComposedViewRobustness(t *testing.T) {
	longPath := "src/a" + strings.Repeat("x", 60) + ".go"
	idx := buildIndex(t, "/work",
		textMatch(longPath, "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "y\n", 1, subSpec{"y", 0, 1}),
	)
	loader := newFailingLoader()
	loader.setFailing(longPath, "permission denied")
	loader.setBuffer("src/b.go", makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3))

	for _, width := range []int{80, 40, 20} {
		t.Run("width-"+strconv.Itoa(width), func(t *testing.T) {
			gate := make(chan struct{})
			close(gate)
			m := app.New([]string{"--json", "--", "foo", "."}, "/work",
				app.WithFileLoadGate(gate),
				app.WithFileLoader(loader.load),
				app.WithPopupDuration(0),
			)
			m, _ = update(t, m, tea.WindowSizeMsg{Width: width, Height: 24})
			m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
			loadCh := startLoadAsync(cmd)
			lc := <-loadCh
			m = deliverCompletion(t, m, lc)

			// Overlay should be open (current file failed).
			if !m.OverlayOpen() {
				t.Fatalf("overlay should be open after read failure")
			}

			// Dismiss the overlay to see the composed view.
			m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))

			view := viewContent(m)

			// Panel should show "(unreadable)".
			if !strings.Contains(view, "(unreadable)") {
				t.Fatalf("view should contain '(unreadable)' at width %d:\n%s", width, view)
			}

			// Filename row should still identify the path (at least
			// the ".go" suffix).
			if !strings.Contains(view, ".go") {
				t.Fatalf("view should contain '.go' (filename row identifies path) at width %d:\n%s", width, view)
			}

			// Nothing should overflow the terminal width.
			if !noLineExceedsWidth(view, width) {
				t.Fatalf("view has lines exceeding width %d:\n%s", width, view)
			}

			// Layout dimensions should be nonnegative.
			if m.ListWidth() < 0 {
				t.Fatalf("ListWidth = %d, want nonnegative", m.ListWidth())
			}
			panelWidth := width - m.ListWidth() - 1
			if panelWidth < 0 {
				t.Fatalf("panelWidth = %d, want nonnegative", panelWidth)
			}
		})
	}
}

// --- Outcome-matrix rows: load failures never change the fixed exit status ---

// TestReadFailureOutcomeAllFailFixed0 verifies that every retained file
// failing to load with a fixed status of 0 still exits 0. Load
// failures affect only file presentation and diagnostics, not the
// exit status.
func TestReadFailureOutcomeAllFailFixed0(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "y\n", 1, subSpec{"y", 0, 1}),
	)
	loader := newFailingLoader()
	loader.setFailing("src/a.go", "permission denied")
	loader.setFailing("src/b.go", "permission denied")
	m, loadCh := setupBrowseFailing(t, idx, loader.load)

	// A fails (startup file).
	lc := <-loadCh
	m = deliverCompletion(t, m, lc)

	// Dismiss the overlay, navigate to B (also fails).
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	m, cmd := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	loadB := startLoadAsync(cmd)
	lc = <-loadB
	m = deliverCompletion(t, m, lc)

	// Both files failed. Exit status should still be 0.
	if m.ExitCode() != 0 {
		t.Fatalf("ExitCode = %d, want 0 (all files fail, fixed status 0)", m.ExitCode())
	}

	// Dismiss the overlay and quit.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	m, cmd = update(t, m, keyPress('q'))
	assertQuit(t, cmd)
	if m.ExitCode() != 0 {
		t.Fatalf("ExitCode = %d, want 0 after q (all files fail, fixed status 0)", m.ExitCode())
	}
}

// TestReadFailureOutcomeCurrentFileFailureFixed2 verifies that a
// current-file failure with a fixed status of 2 still exits 2. The
// load failure does not change the already-fixed fatal-search exit
// status.
func TestReadFailureOutcomeCurrentFileFailureFixed2(t *testing.T) {
	idx := buildIndex(t, "/work",
		textBegin("src/a.go"),
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		endRecord("src/a.go", nil),
		summaryRecord(),
	)
	loader := newFailingLoader()
	loader.setFailing("src/a.go", "permission denied")
	gate := make(chan struct{})
	close(gate)
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader.load),
		app.WithPopupDuration(0),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	// Fatal search (exit code 3) with usable results → browse + error
	// overlay, exit 2.
	m, cmd := update(t, m, app.SearchCompleteMsg{
		Files: idx.Files(), Lines: idx.Len(), Index: idx,
		Process: app.ProcessResult{ExitCode: 3},
		Stderr:  "fatal search error",
	})
	if m.ExitCode() != 2 {
		t.Fatalf("ExitCode = %d, want 2 (fixed fatal-search status)", m.ExitCode())
	}

	// The search-complete overlay is open. Deliver the file load
	// (which fails). The file load failure should not change the
	// exit status.
	loadCh := startLoadAsync(cmd)
	lc := <-loadCh
	m = deliverCompletion(t, m, lc)

	if m.ExitCode() != 2 {
		t.Fatalf("ExitCode = %d, want 2 (load failure does not change fixed status)", m.ExitCode())
	}

	// Dismiss the search-complete overlay. The panel should show
	// "(unreadable)" (the file failed to load).
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	view := viewContent(m)
	if !strings.Contains(view, "(unreadable)") {
		t.Fatalf("view should contain '(unreadable)' after dismissing search overlay:\n%s", view)
	}

	// Quit should still exit 2.
	m, cmd = update(t, m, keyPress('q'))
	assertQuit(t, cmd)
	if m.ExitCode() != 2 {
		t.Fatalf("ExitCode = %d, want 2 after q (load failure does not change fixed status)", m.ExitCode())
	}
}

// TestReadFailureOutcomeComposedAllFailFixed2 verifies the composed
// row: usable search results with fixed status 2 where every retained
// file subsequently fails to load. The ordinary status remains 2, the
// load failures affect only file presentation and diagnostics, and
// the already-fixed fatal-search outcome is not recomputed.
func TestReadFailureOutcomeComposedAllFailFixed2(t *testing.T) {
	idx := buildIndex(t, "/work",
		textBegin("src/a.go"),
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		endRecord("src/a.go", nil),
		textBegin("src/b.go"),
		textMatch("src/b.go", "y\n", 1, subSpec{"y", 0, 1}),
		endRecord("src/b.go", nil),
		summaryRecord(),
	)
	loader := newFailingLoader()
	loader.setFailing("src/a.go", "permission denied")
	loader.setFailing("src/b.go", "permission denied")
	gate := make(chan struct{})
	close(gate)
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader.load),
		app.WithPopupDuration(0),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	// Fatal search (exit code 3) with usable results → browse + error
	// overlay, exit 2.
	m, cmd := update(t, m, app.SearchCompleteMsg{
		Files: idx.Files(), Lines: idx.Len(), Index: idx,
		Process: app.ProcessResult{ExitCode: 3},
		Stderr:  "fatal search error",
	})
	if m.ExitCode() != 2 {
		t.Fatalf("ExitCode = %d, want 2 (fixed fatal-search status)", m.ExitCode())
	}
	exitBefore := m.ExitCode()

	// Dismiss the search-complete overlay.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))

	// A fails (startup file).
	loadCh := startLoadAsync(cmd)
	lc := <-loadCh
	m = deliverCompletion(t, m, lc)
	if m.ExitCode() != exitBefore {
		t.Fatalf("ExitCode = %d, want %d (A failure does not change fixed status)", m.ExitCode(), exitBefore)
	}

	// Dismiss the read-failure overlay, navigate to B (also fails).
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	m, cmd = update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	loadB := startLoadAsync(cmd)
	lc = <-loadB
	m = deliverCompletion(t, m, lc)
	if m.ExitCode() != exitBefore {
		t.Fatalf("ExitCode = %d, want %d (B failure does not change fixed status)", m.ExitCode(), exitBefore)
	}

	// Both files failed. The load failures should affect only file
	// presentation and diagnostics, not the exit status. Dismiss the
	// overlay to see the panel content.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	view := viewContent(m)
	if !strings.Contains(view, "(unreadable)") {
		t.Fatalf("view should contain '(unreadable)' (load failure affects presentation):\n%s", view)
	}

	// Diagnostics should include both the search error and the load
	// failures.
	diags := m.Diagnostics()
	if len(diags) < 2 {
		t.Fatalf("expected at least 2 diagnostics (search + load failures), got %d: %q", len(diags), diags)
	}

	// Quit should still exit 2.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	m, cmd = update(t, m, keyPress('q'))
	assertQuit(t, cmd)
	if m.ExitCode() != exitBefore {
		t.Fatalf("ExitCode = %d, want %d after q (all files fail, fixed status 2)", m.ExitCode(), exitBefore)
	}
}

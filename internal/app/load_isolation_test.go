package app_test

import (
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
)

// --- Issue #25 test helpers ---

// gatedLoader is a test FileLoader that blocks each path on its own
// gate channel and counts calls per path. Closing a path's gate
// (via release) unblocks that path's load. This lets tests control
// when each file's load completes independently, which is essential
// for verifying keyed late completions and the one-load-per-path rule.
// The started channel is closed when the loader is first called for a
// path, letting tests synchronize with the async load goroutine without
// sleeps. A sync.Once guards the close so multiple calls to the same
// path (which should not happen under the one-load-per-path rule but
// does under the current implementation) do not panic.
type gatedLoader struct {
	mu        sync.Mutex
	bufs      map[string]*filebuffer.Buffer
	gates     map[string]chan struct{}
	calls     map[string]int
	started   map[string]chan struct{}
	startOnce map[string]*sync.Once
}

func newGatedLoader() *gatedLoader {
	return &gatedLoader{
		bufs:      make(map[string]*filebuffer.Buffer),
		gates:     make(map[string]chan struct{}),
		calls:     make(map[string]int),
		started:   make(map[string]chan struct{}),
		startOnce: make(map[string]*sync.Once),
	}
}

// set registers a buffer and a held gate for a path.
func (l *gatedLoader) set(path string, buf *filebuffer.Buffer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.bufs[path] = buf
	l.gates[path] = make(chan struct{})
	l.started[path] = make(chan struct{})
	l.startOnce[path] = &sync.Once{}
}

// release closes the gate for a path, unblocking that path's load.
func (l *gatedLoader) release(path string) {
	l.mu.Lock()
	gate, ok := l.gates[path]
	l.mu.Unlock()
	if ok {
		close(gate)
	}
}

// load is the FileLoader function. It increments the call count for
// the path, signals the started channel (once), blocks on the path's
// gate (if held), and returns the registered buffer.
func (l *gatedLoader) load(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
	l.mu.Lock()
	l.calls[string(path)]++
	gate := l.gates[string(path)]
	buf := l.bufs[string(path)]
	started := l.started[string(path)]
	once := l.startOnce[string(path)]
	l.mu.Unlock()

	// Signal that the load has started so tests can synchronize
	// without sleeps. The sync.Once guards against double-close
	// when the one-load-per-path rule is not yet enforced.
	if once != nil {
		once.Do(func() { close(started) })
	}

	if gate != nil {
		<-gate
	}
	if buf != nil {
		return buf, nil
	}
	return &filebuffer.Buffer{}, nil
}

// callCount returns the number of times load was called for a path.
func (l *gatedLoader) callCount(path string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.calls[path]
}

// waitStarted blocks until the loader has been called for the given
// path. This synchronizes tests with the async load goroutine.
func (l *gatedLoader) waitStarted(path string) {
	l.mu.Lock()
	ch := l.started[path]
	l.mu.Unlock()
	if ch != nil {
		<-ch
	}
}

// startLoadAsync runs a command (possibly batched with timer commands)
// asynchronously and returns a channel that receives the first
// FileLoadCompleteMsg when the load completes. Requires
// WithPopupDuration(0) so timer commands return immediately. The
// command blocks inside the gated loader until the per-path gate is
// released.
func startLoadAsync(cmd tea.Cmd) <-chan app.FileLoadCompleteMsg {
	ch := make(chan app.FileLoadCompleteMsg, 1)
	if cmd == nil {
		close(ch)
		return ch
	}
	go func() {
		// Call cmd() in a goroutine because it may block (single
		// load command) or return immediately (batch).
		rawCh := make(chan tea.Msg, 1)
		go func() { rawCh <- cmd() }()
		msg := <-rawCh
		if msg == nil {
			close(ch)
			return
		}
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			ch <- lc
			return
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, c := range batch {
				if c == nil {
					continue
				}
				go func(c tea.Cmd) {
					sub := c()
					if sub == nil {
						return
					}
					if lc, ok := sub.(app.FileLoadCompleteMsg); ok {
						select {
						case ch <- lc:
						default:
						}
					}
				}(c)
			}
		}
	}()
	return ch
}

// deliverCompletion delivers a FileLoadCompleteMsg to the model and
// handles any resulting layout preparation. This simulates the async
// load completing and the viewport being built.
func deliverCompletion(t *testing.T, m app.Model, lc app.FileLoadCompleteMsg) app.Model {
	t.Helper()
	m, layoutCmd := update(t, m, lc)
	return deliverLayout(t, m, layoutCmd)
}

// setupBrowseGated creates a browse model with a gated loader and
// popup duration 0. All gates are held; tests release them
// individually. The file gate is closed immediately so loads proceed
// to the gated loader. Returns the model and the async load channel
// for the startup file.
func setupBrowseGated(t *testing.T, idx *searchindex.Index, loader *gatedLoader) (app.Model, <-chan app.FileLoadCompleteMsg) {
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

// --- PRD #35: Navigation during loads ---

// TestLoadIsolationNavigationActiveWhileLoading verifies that n/p
// navigation remains fully active while a file loads. The user can
// move past a slow file without waiting for it. The cursor advances
// and the panel switches to show the destination's loading
// placeholder.
func TestLoadIsolationNavigationActiveWhileLoading(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/c.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	loader := newGatedLoader()
	loader.set("src/a.go", makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3))
	loader.set("src/b.go", makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3))
	loader.set("src/c.go", makeBuf([]filebuffer.Line{ml(1, "content-c")}, 1, 3))
	m, _ := setupBrowseGated(t, idx, loader)

	// All gates held; A is loading. Navigate to B.
	m, cmd := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	_ = startLoadAsync(cmd)
	view := viewContent(m)
	if !strings.Contains(view, "Loading") {
		t.Fatalf("after n to loading B, view should show Loading: %q", view)
	}

	// Navigate to C while B is still loading.
	m, cmd = update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/c.go")
	_ = startLoadAsync(cmd)
	view = viewContent(m)
	if !strings.Contains(view, "Loading") {
		t.Fatalf("after n to loading C, view should show Loading: %q", view)
	}
}

// TestLoadIsolationPlaceholderScrollNoOp verifies that scrolling a
// "Loading…" placeholder is a no-op. The viewport is nil while
// loading, so scroll keys do nothing.
func TestLoadIsolationPlaceholderScrollNoOp(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	loader := newGatedLoader()
	loader.set("src/a.go", makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3))
	m, _ := setupBrowseGated(t, idx, loader)

	// A is loading. Scroll keys should be no-ops (viewport is nil).
	for _, key := range []tea.KeyPressMsg{
		keyPress('d'), keyPress('u'),
		{Code: tea.KeyDown}, {Code: tea.KeyUp},
		{Code: tea.KeyPgDown}, {Code: tea.KeyPgUp},
	} {
		m2, cmd := update(t, m, key)
		if m2.State() != app.StateBrowse {
			t.Fatalf("scroll key while loading changed state to %v", m2.State())
		}
		if cmd != nil {
			msg := execCmd(t, cmd)
			if _, ok := msg.(tea.QuitMsg); ok {
				t.Fatal("scroll key while loading produced a quit command")
			}
		}
	}
}

// TestLoadIsolationKeysNormalWhileLoading verifies that w, c, and
// resize keep their normal meanings while a file loads.
func TestLoadIsolationKeysNormalWhileLoading(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	loader := newGatedLoader()
	loader.set("src/a.go", makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3))
	loader.set("src/b.go", makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3))
	m, _ := setupBrowseGated(t, idx, loader)

	// w toggles wrap mode (should not crash or quit).
	m, cmd := update(t, m, keyPress('w'))
	if m.State() != app.StateBrowse {
		t.Fatalf("after w while loading, State = %v, want StateBrowse", m.State())
	}
	if cmd != nil {
		msg := execCmd(t, cmd)
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Fatal("w while loading produced a quit command")
		}
	}

	// c toggles theme (should not crash or quit).
	m, cmd = update(t, m, keyPress('c'))
	if m.State() != app.StateBrowse {
		t.Fatalf("after c while loading, State = %v, want StateBrowse", m.State())
	}
	if cmd != nil {
		msg := execCmd(t, cmd)
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Fatal("c while loading produced a quit command")
		}
	}

	// resize is handled (should not crash or quit).
	m, cmd = update(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	if m.State() != app.StateBrowse {
		t.Fatalf("after resize while loading, State = %v, want StateBrowse", m.State())
	}
	if cmd != nil {
		msg := execCmd(t, cmd)
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Fatal("resize while loading produced a quit command")
		}
	}
}

// --- PRD #36: Keyed late completions for non-current files ---

// TestLoadIsolationKeyedCompletionNonCurrentPath verifies that a late
// load completion for a non-current file updates only that file's
// cache, not the visible panel. A→B→C navigation with A's completion
// arriving while C is current leaves C's panel unchanged (still
// loading) and A cached.
func TestLoadIsolationKeyedCompletionNonCurrentPath(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/c.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	loader := newGatedLoader()
	loader.set("src/a.go", makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3))
	loader.set("src/b.go", makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3))
	loader.set("src/c.go", makeBuf([]filebuffer.Line{ml(1, "content-c")}, 1, 3))
	m, loadA := setupBrowseGated(t, idx, loader)
	loader.waitStarted("src/a.go")

	// Navigate to B (B starts loading).
	m, cmd := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	loadB := startLoadAsync(cmd)
	loader.waitStarted("src/b.go")

	// Navigate to C (C starts loading).
	m, cmd = update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/c.go")
	loadC := startLoadAsync(cmd)
	loader.waitStarted("src/c.go")

	// Release A's gate. A's completion arrives while C is current.
	loader.release("src/a.go")
	lc := <-loadA
	m = deliverCompletion(t, m, lc)

	// C's panel should be unchanged (still loading).
	assertCurrentPath(t, m, "src/c.go")
	view := viewContent(m)
	if !strings.Contains(view, "Loading") {
		t.Fatalf("after A's late completion, panel should still show Loading for C: %q", view)
	}
	if strings.Contains(view, "content-a") {
		t.Fatalf("A's late completion replaced C's panel: %q", view)
	}

	// Release C's gate. C's completion arrives and updates the panel
	// (C is current).
	loader.release("src/c.go")
	lc = <-loadC
	m = deliverCompletion(t, m, lc)
	view = viewContent(m)
	if !strings.Contains(view, "content-c") {
		t.Fatalf("after C's completion, panel should show content-c: %q", view)
	}

	// A should be cached: navigate back to A (p twice) and verify
	// content shows immediately without a new load.
	m, cmd = update(t, m, keyPress('p'))
	_ = startLoadAsync(cmd) // B may still be loading
	assertCurrentPath(t, m, "src/b.go")
	m, cmd = update(t, m, keyPress('p'))
	assertCurrentPath(t, m, "src/a.go")
	// A's buffer is cached but its layout was never built (the
	// completion arrived while C was current). Deliver the layout
	// preparation command so the viewport is installed.
	m = deliverLayout(t, m, cmd)
	view = viewContent(m)
	if !strings.Contains(view, "content-a") {
		t.Fatalf("cached A should show content-a immediately: %q", view)
	}
	if strings.Contains(view, "Loading") {
		t.Fatalf("cached A should not show Loading: %q", view)
	}
	// Loader should not have been called again for A.
	if loader.callCount("src/a.go") != 1 {
		t.Fatalf("loader called %d times for A, want 1 (cached revisit)", loader.callCount("src/a.go"))
	}

	// Clean up B's load.
	loader.release("src/b.go")
	<-loadB
}

// TestLoadIsolationKeyedCompletionCurrentPath verifies that a load
// completion for the current path updates the panel (shows content).
func TestLoadIsolationKeyedCompletionCurrentPath(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	loader := newGatedLoader()
	loader.set("src/a.go", makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3))
	m, loadA := setupBrowseGated(t, idx, loader)

	// A is current and loading. Release A's gate.
	loader.release("src/a.go")
	lc := <-loadA
	m = deliverCompletion(t, m, lc)

	view := viewContent(m)
	if !strings.Contains(view, "content-a") {
		t.Fatalf("after A's completion, panel should show content-a: %q", view)
	}
	if strings.Contains(view, "Loading") {
		t.Fatalf("after A's completion, panel should not show Loading: %q", view)
	}
}

// --- One-load-per-path rule ---

// TestLoadIsolationOneLoadPerPath verifies that at most one load is in
// flight per raw path. Re-entering a loading path starts nothing and
// queues nothing. The loader is called at most once per path even if
// the user navigates away and back while the load is still in flight.
func TestLoadIsolationOneLoadPerPath(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	loader := newGatedLoader()
	loader.set("src/a.go", makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3))
	loader.set("src/b.go", makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3))
	m, _ := setupBrowseGated(t, idx, loader)

	// A is loading (loader called once for A). Wait for the async
	// load goroutine to start so the call count is accurate.
	loader.waitStarted("src/a.go")
	if got := loader.callCount("src/a.go"); got != 1 {
		t.Fatalf("loader called %d times for A after startup, want 1", got)
	}

	// Navigate to B (B starts loading, loader called once for B).
	m, cmd := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	_ = startLoadAsync(cmd)
	loader.waitStarted("src/b.go")
	if got := loader.callCount("src/b.go"); got != 1 {
		t.Fatalf("loader called %d times for B after navigation, want 1", got)
	}

	// Navigate back to A. A is still loading (one-load-per-path:
	// no new load started, nothing queued).
	m, cmd = update(t, m, keyPress('p'))
	assertCurrentPath(t, m, "src/a.go")
	// The returned command should NOT contain a load command (only
	// the popup timer). Start it async to drain any timer commands.
	_ = startLoadAsync(cmd)

	// Give the async command a moment to execute. If a new load was
	// started, the loader call count for A would increase.
	time.Sleep(50 * time.Millisecond)

	// Verify the loader was NOT called again for A. This is the
	// definitive one-load-per-path check: if a second load was
	// started, the loader would have been called twice.
	if got := loader.callCount("src/a.go"); got != 1 {
		t.Fatalf("loader called %d times for A after re-entry, want 1 (one-load-per-path)", got)
	}

	// The panel should show Loading (A is still loading).
	view := viewContent(m)
	if !strings.Contains(view, "Loading") {
		t.Fatalf("re-entering loading A should show Loading: %q", view)
	}
}

// --- Cached revisits (PRD #46: session-long retention, no eviction) ---

// TestLoadIsolationCachedRevisitNoReload verifies that a successfully
// loaded buffer is retained for the session. Revisiting a cached
// file shows its content immediately without a new load.
func TestLoadIsolationCachedRevisitNoReload(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	loader := newGatedLoader()
	loader.set("src/a.go", makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3))
	loader.set("src/b.go", makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3))
	m, loadA := setupBrowseGated(t, idx, loader)

	// Release A's gate so A loads and is cached.
	loader.release("src/a.go")
	lc := <-loadA
	m = deliverCompletion(t, m, lc)
	if got := loader.callCount("src/a.go"); got != 1 {
		t.Fatalf("loader called %d times for A, want 1", got)
	}

	// Navigate to B (B starts loading).
	m, cmd := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	loadB := startLoadAsync(cmd)

	// Navigate back to A. A is cached, so content should show
	// immediately without a new load.
	m, cmd = update(t, m, keyPress('p'))
	assertCurrentPath(t, m, "src/a.go")
	view := viewContent(m)
	if !strings.Contains(view, "content-a") {
		t.Fatalf("cached revisit to A should show content-a: %q", view)
	}
	if strings.Contains(view, "Loading") {
		t.Fatalf("cached revisit to A should not show Loading: %q", view)
	}
	// Loader should not have been called again for A.
	if got := loader.callCount("src/a.go"); got != 1 {
		t.Fatalf("loader called %d times for A after revisit, want 1 (cached)", got)
	}

	// Clean up B's load.
	loader.release("src/b.go")
	<-loadB
}

// TestLoadIsolationCacheRetainedAcrossMultipleVisits verifies that a
// cached buffer survives multiple visits away and back. No eviction
// occurs during the session.
func TestLoadIsolationCacheRetainedAcrossMultipleVisits(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	loader := newGatedLoader()
	loader.set("src/a.go", makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3))
	loader.set("src/b.go", makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3))
	m, loadA := setupBrowseGated(t, idx, loader)

	// Release A and B gates so both load and are cached.
	loader.release("src/a.go")
	lc := <-loadA
	m = deliverCompletion(t, m, lc)

	m, cmd := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	loadB := startLoadAsync(cmd)
	loader.release("src/b.go")
	lc = <-loadB
	m = deliverCompletion(t, m, lc)

	// Visit A, then B, then A again. Both should be cached.
	for i := 0; i < 3; i++ {
		m, _ = update(t, m, keyPress('p'))
		assertCurrentPath(t, m, "src/a.go")
		view := viewContent(m)
		if !strings.Contains(view, "content-a") {
			t.Fatalf("visit %d: cached A should show content-a: %q", i, view)
		}
		m, _ = update(t, m, keyPress('n'))
		assertCurrentPath(t, m, "src/b.go")
		view = viewContent(m)
		if !strings.Contains(view, "content-b") {
			t.Fatalf("visit %d: cached B should show content-b: %q", i, view)
		}
	}
	// Loader called exactly once per path.
	if got := loader.callCount("src/a.go"); got != 1 {
		t.Fatalf("loader called %d times for A, want 1", got)
	}
	if got := loader.callCount("src/b.go"); got != 1 {
		t.Fatalf("loader called %d times for B, want 1", got)
	}
}

// --- Post-cancellation rejection ---

// TestLoadIsolationPostCancellationRejection verifies that late
// FileLoadCompleteMsg messages arriving after cancellation (ctrl+c)
// are ignored. The model stays cancelled and is not revived.
func TestLoadIsolationPostCancellationRejection(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	loader := newGatedLoader()
	loader.set("src/a.go", makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3))
	loader.set("src/b.go", makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3))
	m, _ := setupBrowseGated(t, idx, loader)

	// Cancel with ctrl+c.
	m, cmd := update(t, m, ctrlC())
	assertQuit(t, cmd)
	if m.ExitCode() != 130 {
		t.Fatalf("ExitCode = %d, want 130", m.ExitCode())
	}

	// Deliver a late FileLoadCompleteMsg for A. The model should
	// not be revived.
	m2, _ := update(t, m, app.FileLoadCompleteMsg{
		Path:   []byte("src/a.go"),
		Buffer: makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3),
	})
	if m2.State() == app.StateBrowse {
		t.Fatal("late FileLoadCompleteMsg revived the cancelled UI")
	}
	if m2.ExitCode() != 130 {
		t.Fatalf("after late completion, ExitCode = %d, want 130", m2.ExitCode())
	}
}

// TestLoadIsolationLateCompletionAfterQuit verifies that a late
// completion for a file that is no longer current does not change
// the exit code or revive a quit-in-progress. This is a variant of
// the post-cancellation test where cancellation happens via q from
// browse. The late completion must not update the panel (the view
// should still show the loading placeholder, not content).
func TestLoadIsolationLateCompletionAfterQuit(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	loader := newGatedLoader()
	loader.set("src/a.go", makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3))
	m, _ := setupBrowseGated(t, idx, loader)

	// Quit with q from browse. The state stays StateBrowse but
	// m.cancelled is set.
	m, cmd := update(t, m, keyPress('q'))
	assertQuit(t, cmd)
	beforeView := viewContent(m)

	// Deliver a late FileLoadCompleteMsg. The model should not
	// be revived: the panel should not change.
	m2, _ := update(t, m, app.FileLoadCompleteMsg{
		Path:   []byte("src/a.go"),
		Buffer: makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3),
	})
	afterView := viewContent(m2)
	if beforeView != afterView {
		t.Fatalf("late FileLoadCompleteMsg changed the view after quit:\nbefore: %q\nafter:  %q", beforeView, afterView)
	}
	if strings.Contains(afterView, "content-a") {
		t.Fatal("late FileLoadCompleteMsg revived the quit UI (panel shows content)")
	}
}

// --- Input responsiveness during decode/map phase ---

// TestLoadIsolationResponsiveNWhileFileGateHeld verifies that n is
// actionable while the current file's load is held at the file gate
// (the decode/map phase gate). The cursor advances and the panel
// switches to the destination.
func TestLoadIsolationResponsiveNWhileFileGateHeld(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	m, _ := setupBrowseLoading(t, idx)

	// A is loading (file gate held). Press n to navigate to B.
	m, cmd := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	if m.State() != app.StateBrowse {
		t.Fatalf("after n while gate held, State = %v, want StateBrowse", m.State())
	}
	if cmd != nil {
		msg := execCmd(t, cmd)
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Fatal("n while gate held produced a quit command")
		}
	}
}

// TestLoadIsolationResponsivePWhileFileGateHeld verifies that p is
// actionable while the current file's load is held at the file gate.
func TestLoadIsolationResponsivePWhileFileGateHeld(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	m, _ := setupBrowseLoading(t, idx)

	// A is loading. Press p to wrap to B (last stop).
	m, cmd := update(t, m, keyPress('p'))
	assertCurrentPath(t, m, "src/b.go")
	if m.State() != app.StateBrowse {
		t.Fatalf("after p while gate held, State = %v, want StateBrowse", m.State())
	}
	if cmd != nil {
		msg := execCmd(t, cmd)
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Fatal("p while gate held produced a quit command")
		}
	}
}

// TestLoadIsolationResponsiveWWhileFileGateHeld verifies that w is
// actionable while the current file's load is held at the file gate.
func TestLoadIsolationResponsiveWWhileFileGateHeld(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	m, _ := setupBrowseLoading(t, idx)

	// A is loading. Press w to toggle wrap mode.
	m, cmd := update(t, m, keyPress('w'))
	if m.State() != app.StateBrowse {
		t.Fatalf("after w while gate held, State = %v, want StateBrowse", m.State())
	}
	if cmd != nil {
		msg := execCmd(t, cmd)
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Fatal("w while gate held produced a quit command")
		}
	}
}

// TestLoadIsolationResponsiveCWhileFileGateHeld verifies that c is
// actionable while the current file's load is held at the file gate.
func TestLoadIsolationResponsiveCWhileFileGateHeld(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	m, _ := setupBrowseLoading(t, idx)

	// A is loading. Press c to toggle theme.
	m, cmd := update(t, m, keyPress('c'))
	if m.State() != app.StateBrowse {
		t.Fatalf("after c while gate held, State = %v, want StateBrowse", m.State())
	}
	if cmd != nil {
		msg := execCmd(t, cmd)
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Fatal("c while gate held produced a quit command")
		}
	}
}

// TestLoadIsolationResponsiveResizeWhileFileGateHeld verifies that
// resize is actionable while the current file's load is held at the
// file gate.
func TestLoadIsolationResponsiveResizeWhileFileGateHeld(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	m, _ := setupBrowseLoading(t, idx)

	// A is loading. Resize the terminal.
	m, cmd := update(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	if m.State() != app.StateBrowse {
		t.Fatalf("after resize while gate held, State = %v, want StateBrowse", m.State())
	}
	if cmd != nil {
		msg := execCmd(t, cmd)
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Fatal("resize while gate held produced a quit command")
		}
	}
}

// TestLoadIsolationResponsiveCtrlCWhileFileGateHeld verifies that
// ctrl+c is actionable while the current file's load is held at the
// file gate, exiting with code 130.
func TestLoadIsolationResponsiveCtrlCWhileFileGateHeld(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	m, _ := setupBrowseLoading(t, idx)

	// A is loading. Press ctrl+c to cancel.
	m, cmd := update(t, m, ctrlC())
	assertQuit(t, cmd)
	if m.ExitCode() != 130 {
		t.Fatalf("ExitCode = %d, want 130", m.ExitCode())
	}
}

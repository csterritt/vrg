package app

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"vrg/internal/filebuffer"
)

// gatedModel returns a browse model wired with the load test seams —
// the pop-up timer suppressed, loadGate holding the worker's start and
// mapGate holding its decode/map phase after the read — with the
// startup file's load command returned uninvoked. Nil gates hold
// nothing.
func gatedModel(t *testing.T, dir string, loadGate, mapGate chan struct{}, recs ...string) (Model, tea.Cmd) {
	t.Helper()
	m := newModel(nil, nil)
	m.popupTimer = func(int) tea.Cmd { return nil }
	m.loadGate = loadGate
	m.mapGate = mapGate
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	return update(t, m, searchDoneMsg{index: fixtureIndex(t, dir, recs...)})
}

// mintLoad registers an in-flight load request for path on the model
// and returns its request identity. Update drops a completion whose
// (path, request) pair is not the in-flight request, so a test that
// injects a completion without running the real worker command mints
// the request first.
func mintLoad(m *Model, path string) int {
	m.loadSeq++
	m.loading[path] = m.loadSeq
	return m.loadSeq
}

// A load in flight does not hold navigation: with a.txt's worker
// genuinely started and parked on the gate, n crosses to b.txt at
// once — the panel switches to its placeholder and b.txt's own load
// issues — and p returns to the still-loading a.txt, whose content
// arrives only when the held worker completes.
func TestNavigationActiveWhileLoadInFlight(t *testing.T) {
	dir := t.TempDir()
	var recs []string
	recs = append(recs, fileWithStops(t, dir, "a.txt", 20, 1)...)
	recs = append(recs, fileWithStops(t, dir, "b.txt", 20, 5)...)
	recs = append(recs, `{"type":"summary","data":{}}`)
	gate := make(chan struct{})
	m, cmdA := gatedModel(t, dir, gate, nil, recs...)
	if cmdA == nil {
		t.Fatal("search completion returned no load command")
	}
	reqA := m.loading["a.txt"]

	// a.txt's worker is in flight, parked on the gate.
	done := make(chan tea.Msg, 1)
	go func() { done <- cmdA() }()
	select {
	case <-done:
		t.Fatal("a.txt's load completed while its gate was held")
	case <-time.After(50 * time.Millisecond):
	}

	// n crosses to b.txt immediately: the panel switches to b.txt's
	// placeholder and its own load issues.
	var cmdB tea.Cmd
	m, cmdB = update(t, m, keyPress("n"))
	if s, _ := m.currentStop(); string(s.Path) != "b.txt" || s.Line != 5 {
		t.Fatalf("n during a.txt's load selected %+v, want b.txt:5", s)
	}
	if cmdB == nil {
		t.Fatal("crossing to b.txt during a.txt's load issued no load command")
	}
	if v := m.View().Content; !strings.Contains(v, "── b.txt ") || !strings.Contains(v, "Loading…") {
		t.Fatalf("b.txt's panel during its load = %q", v)
	}

	// p lands back on a.txt — still loading, still the placeholder.
	m, _ = update(t, m, keyPress("p"))
	if s, _ := m.currentStop(); string(s.Path) != "a.txt" || s.Line != 1 {
		t.Fatalf("p during a.txt's load selected %+v, want a.txt:1", s)
	}
	if m.loading["a.txt"] != reqA {
		t.Fatalf("a.txt's in-flight request changed %d → %d", reqA, m.loading["a.txt"])
	}
	if v := m.View().Content; !strings.Contains(v, "Loading…") {
		t.Fatalf("a.txt view while still loading = %q", v)
	}

	// Releasing the gate lets the original request complete; its
	// content renders for the now-current a.txt.
	close(gate)
	m = pump(t, m, <-done)
	if v := m.View().Content; !strings.Contains(v, "\x1b[30;47;4mhit\x1b[24;37;40m00001") {
		t.Fatalf("a.txt view after its load lacks the match: %q", v)
	}
	if strings.Contains(m.View().Content, "Loading…") {
		t.Fatal("a.txt still shows the placeholder after its load")
	}
}

// Scrolling and panning a "Loading…" placeholder are no-ops — no
// command, a byte-identical frame, and no saved viewport state — while
// keys with ordinary browse meanings keep working during the load.
func TestPlaceholderScrollAndPanAreNoOp(t *testing.T) {
	dir := t.TempDir()
	var recs []string
	recs = append(recs, fileWithStops(t, dir, "a.txt", 20, 1)...)
	recs = append(recs, `{"type":"summary","data":{}}`)
	m, cmd := gatedModel(t, dir, make(chan struct{}), nil, recs...)
	if cmd == nil {
		t.Fatal("search completion returned no load command")
	}
	before := m.View().Content
	for _, k := range []tea.KeyPressMsg{
		codePress(tea.KeyUp), codePress(tea.KeyDown),
		codePress(tea.KeyPgUp), codePress(tea.KeyPgDown),
		keyPress("u"), keyPress("d"),
		keyPress(","), keyPress("."), keyPress("<"), keyPress(">"),
		keyPress("["), keyPress("]"),
	} {
		var c tea.Cmd
		m, c = update(t, m, k)
		if c != nil {
			t.Fatalf("placeholder key %v produced a command %T", k.Code, c)
		}
	}
	if got := m.View().Content; got != before {
		t.Fatalf("scroll/pan changed the placeholder frame: %q", got)
	}
	if _, ok := m.saved["a.txt"]; ok {
		t.Fatal("placeholder scrolling wrote saved viewport state")
	}

	// Ordinary browse keys keep their meanings during the load: w
	// flips the wrap mode (no buffer, so no layout request), c flips
	// the colour scheme, and tab hides the file list.
	m, c := update(t, m, keyPress("w"))
	if m.wrap {
		t.Fatal("w during a load did not flip the wrap mode")
	}
	if c != nil {
		t.Fatalf("w during a load produced a command %T — no buffer, no relayout", c)
	}
	m, _ = update(t, m, keyPress("c"))
	if v := m.View().Content; !strings.HasPrefix(v, "\x1b[30;47m") {
		t.Fatalf("c during a load did not switch to the light scheme: %q", v)
	}
	m, _ = update(t, m, codePress(tea.KeyTab))
	if m.listShow {
		t.Fatal("tab during a load did not hide the file list")
	}
}

// Re-entering a path whose load is in flight is dropped, not queued:
// the crossing back to a.txt returns no command — no second worker
// starts and nothing waits to run — the in-flight request's identity
// is unchanged, and its completion still installs the content.
func TestReEnterLoadingPathIsDroppedNotQueued(t *testing.T) {
	dir := t.TempDir()
	var recs []string
	recs = append(recs, fileWithStops(t, dir, "a.txt", 20, 1)...)
	recs = append(recs, fileWithStops(t, dir, "b.txt", 20, 5)...)
	recs = append(recs, `{"type":"summary","data":{}}`)
	gate := make(chan struct{})
	m, cmdA := gatedModel(t, dir, gate, nil, recs...)
	reqA := m.loading["a.txt"]
	done := make(chan tea.Msg, 1)
	go func() { done <- cmdA() }()
	select {
	case <-done:
		t.Fatal("a.txt's load completed while its gate was held")
	case <-time.After(50 * time.Millisecond):
	}

	// Away and back: the re-entry issues nothing — one load per path,
	// and no queued request waits behind it.
	m, _ = update(t, m, keyPress("n"))
	m, again := update(t, m, keyPress("p"))
	if again != nil {
		t.Fatalf("re-entry to loading a.txt produced a command %T", again)
	}
	if m.loading["a.txt"] != reqA {
		t.Fatalf("re-entry replaced a.txt's in-flight request %d with %d",
			reqA, m.loading["a.txt"])
	}

	// The single in-flight worker is the only completion that ever
	// arrives; nothing queued fires behind it.
	close(gate)
	m = pump(t, m, <-done)
	if m.bufs["a.txt"] == nil {
		t.Fatal("the in-flight request's completion did not install")
	}
	if m.loading["a.txt"] != 0 {
		t.Fatal("a.txt still marked loading after its completion")
	}
	if v := m.View().Content; !strings.Contains(v, "\x1b[30;47;4mhit\x1b[24;37;40m00001") {
		t.Fatalf("a.txt view after its load lacks the match: %q", v)
	}
}

// A→B→C with every worker held uninvoked: a.txt's completion arriving
// while c.txt is current fills only a.txt's cache entry — c.txt's
// panel, viewport, and saved state are byte-identical — and the same
// holds for b.txt's. Revisiting either file shows its cached content
// with no new load: the crossing issues only the layout request, and
// the buffers stay cached for the session.
func TestLateCompletionIsolatedToItsPath(t *testing.T) {
	dir := t.TempDir()
	var recs []string
	recs = append(recs, fileWithStops(t, dir, "a.txt", 20, 1)...)
	recs = append(recs, fileWithStops(t, dir, "b.txt", 20, 5)...)
	recs = append(recs, fileWithStops(t, dir, "c.txt", 20, 9)...)
	recs = append(recs, `{"type":"summary","data":{}}`)
	m, cmdA := browseModel(t, dir, 80, 24, recs...)
	var cmdB, cmdC tea.Cmd
	m, cmdB = update(t, m, keyPress("n"))
	m, cmdC = update(t, m, keyPress("n"))
	if s, _ := m.currentStop(); string(s.Path) != "c.txt" || s.Line != 9 {
		t.Fatalf("stop after n n = %+v, want c.txt:9", s)
	}
	before := m.View().Content
	savedA, topBefore := m.saved["a.txt"], m.vp.Top()

	// a.txt's completion while c.txt is current updates only a.txt.
	m = pump(t, m, cmdA())
	if got := m.View().Content; got != before {
		t.Fatalf("a.txt's late completion changed c.txt's panel:\n%q", got)
	}
	if m.bufs["a.txt"] == nil {
		t.Fatal("a.txt's late completion did not fill its cache")
	}
	if m.loading["a.txt"] != 0 {
		t.Fatal("a.txt still marked loading after its completion")
	}
	if m.bufs["c.txt"] != nil || m.failed["c.txt"] {
		t.Fatal("a.txt's completion touched c.txt's status")
	}
	if m.saved["a.txt"] != savedA || m.vp.Top() != topBefore {
		t.Fatal("a.txt's completion moved the viewport or saved state")
	}

	// b.txt's completion while c.txt is current is isolated the same
	// way.
	m = pump(t, m, cmdB())
	if got := m.View().Content; got != before {
		t.Fatalf("b.txt's late completion changed c.txt's panel:\n%q", got)
	}
	if m.bufs["b.txt"] == nil {
		t.Fatal("b.txt's late completion did not fill its cache")
	}

	// Revisits show cached content and start no new load: a crossing
	// back issues only the destination's layout request.
	var c tea.Cmd
	m, c = update(t, m, keyPress("p")) // c.txt:9 → b.txt:5
	for _, msg := range cmdMsgs(c) {
		if _, ok := msg.(loadDoneMsg); ok {
			t.Fatal("revisit to cached b.txt started a new load")
		}
	}
	m = settle(t, m, c)
	if v := m.View().Content; !strings.Contains(v, "\x1b[30;47;4mhit\x1b[24;37;40m00005") {
		t.Fatalf("cached b.txt did not render its match: %q", v)
	}
	m, c = update(t, m, keyPress("p")) // b.txt:5 → a.txt:1
	for _, msg := range cmdMsgs(c) {
		if _, ok := msg.(loadDoneMsg); ok {
			t.Fatal("revisit to cached a.txt started a new load")
		}
	}
	m = settle(t, m, c)
	if v := m.View().Content; !strings.Contains(v, "\x1b[30;47;4mhit\x1b[24;37;40m00001") {
		t.Fatalf("cached a.txt did not render its match: %q", v)
	}
	// No eviction: every buffer loaded this session stays cached.
	if m.bufs["a.txt"] == nil || m.bufs["b.txt"] == nil {
		t.Fatal("a visited buffer was evicted")
	}
	_ = cmdC // c.txt's worker stays held; nothing about it changes
}

// Load completions are keyed by raw path and request identity: a
// completion whose identity is not the in-flight request's — or whose
// path has no request in flight at all — is dropped without touching
// the cache, the status maps, the diagnostics, or the panel. The
// in-flight request's own completion still installs.
func TestLoadCompletionKeyedByRequestIdentity(t *testing.T) {
	dir := t.TempDir()
	var recs []string
	recs = append(recs, fileWithStops(t, dir, "a.txt", 20, 1)...)
	recs = append(recs, `{"type":"summary","data":{}}`)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	req := m.loading["a.txt"]
	if req == 0 {
		t.Fatal("a.txt's issued load carries no request identity")
	}

	// A completion whose identity is not the in-flight request's is
	// stale: dropped outright, placeholder and in-flight mark intact.
	forged, err := filebuffer.Load([]byte(filepath.Join(dir, "a.txt")), nil)
	if err != nil {
		t.Fatalf("forged buffer load: %v", err)
	}
	m2, c := update(t, m, loadDoneMsg{path: []byte("a.txt"), req: req + 1, buf: forged})
	if c != nil {
		t.Fatalf("stale completion produced a command %T", c)
	}
	if m2.bufs["a.txt"] != nil || m2.loading["a.txt"] != req {
		t.Fatalf("stale completion touched a.txt: cached=%v loading=%d",
			m2.bufs["a.txt"] != nil, m2.loading["a.txt"])
	}
	if v := m2.View().Content; !strings.Contains(v, "Loading…") {
		t.Fatalf("stale completion replaced the placeholder: %q", v)
	}

	// A completion for a path with no request in flight is dropped the
	// same way: no cache, no failure mark, no diagnostic.
	diags := len(m.diags)
	m3, _ := update(t, m, loadDoneMsg{path: []byte("never.txt"), req: req + 7, err: errors.New("boom")})
	if m3.failed["never.txt"] || m3.bufs["never.txt"] != nil {
		t.Fatal("unsolicited completion changed another path's status")
	}
	if len(m3.diags) != diags {
		t.Fatal("unsolicited completion collected a diagnostic")
	}

	// The in-flight request's own completion installs normally.
	m = pump(t, m, cmd())
	if m.bufs["a.txt"] == nil {
		t.Fatal("the in-flight request's completion did not install")
	}
	if v := m.View().Content; !strings.Contains(v, "\x1b[30;47;4mhit\x1b[24;37;40m00001") {
		t.Fatalf("installed view lacks the match: %q", v)
	}
}

// A worker completion arriving after cancellation is discarded: the
// quit model returns no command, stays at exit 130, and fills no
// cache.
func TestLoadCompletionAfterQuitDiscarded(t *testing.T) {
	dir := t.TempDir()
	var recs []string
	recs = append(recs, fileWithStops(t, dir, "a.txt", 20, 1)...)
	recs = append(recs, `{"type":"summary","data":{}}`)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m.cancel = func() {}
	m2, qc := update(t, m, ctrlCPress())
	if _, ok := qc().(tea.QuitMsg); !ok {
		t.Fatalf("ctrl+c command = %T, want tea.QuitMsg", qc())
	}
	if m2.ExitCode() != 130 {
		t.Fatalf("ctrl+c exit = %d, want 130", m2.ExitCode())
	}
	m3, c := update(t, m2, cmd())
	if c != nil {
		t.Fatalf("post-cancellation completion produced a command %T", c)
	}
	if !m3.quit || m3.ExitCode() != 130 {
		t.Fatalf("late load revived a cancelled UI: quit=%v code=%d", m3.quit, m3.ExitCode())
	}
	if m3.bufs["a.txt"] != nil {
		t.Fatal("post-cancellation completion filled the cache")
	}
}

// The decode/map phase is separately gatable: with the read finished
// and the worker parked on the map gate, resize, w, c, n, and p are
// all handled without waiting — and releasing the gate delivers the
// prepared buffer to the then-current file.
func TestDecodeMapGateKeepsInputsActionable(t *testing.T) {
	dir := t.TempDir()
	var recs []string
	recs = append(recs, fileWithStops(t, dir, "a.txt", 20, 1)...)
	recs = append(recs, fileWithStops(t, dir, "b.txt", 20, 5)...)
	recs = append(recs, `{"type":"summary","data":{}}`)
	mapGate := make(chan struct{})
	m, cmdA := gatedModel(t, dir, nil, mapGate, recs...)

	// The worker reads the file, then parks on the decode/map gate.
	done := make(chan tea.Msg, 1)
	go func() { done <- cmdA() }()
	select {
	case <-done:
		t.Fatal("load completed while the decode/map gate was held")
	case <-time.After(50 * time.Millisecond):
	}

	// Every input stays actionable while decode/map is held.
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	if m.width != 100 || m.height != 30 {
		t.Fatalf("resize during held decode/map lost: %dx%d", m.width, m.height)
	}
	var c tea.Cmd
	m, c = update(t, m, keyPress("w"))
	if m.wrap || c != nil {
		t.Fatalf("w during held decode/map: wrap=%v cmd=%T", m.wrap, c)
	}
	m, _ = update(t, m, keyPress("c"))
	if v := m.View().Content; !strings.HasPrefix(v, "\x1b[30;47m") {
		t.Fatalf("c during held decode/map did not flip the scheme: %q", v)
	}
	m, c = update(t, m, keyPress("n"))
	if s, _ := m.currentStop(); string(s.Path) != "b.txt" || s.Line != 5 {
		t.Fatalf("n during held decode/map selected %+v, want b.txt:5", s)
	}
	if c == nil {
		t.Fatal("crossing to b.txt issued no load command")
	}
	m, c = update(t, m, keyPress("p"))
	if s, _ := m.currentStop(); string(s.Path) != "a.txt" || s.Line != 1 {
		t.Fatalf("p during held decode/map selected %+v, want a.txt:1", s)
	}
	if c != nil {
		t.Fatalf("re-entry to loading a.txt produced a command %T", c)
	}
	if v := m.View().Content; !strings.Contains(v, "Loading…") {
		t.Fatalf("a.txt view while decode/map is held = %q", v)
	}

	// Releasing the gate delivers the prepared buffer; a.txt is
	// current again, so its content installs and renders — in the
	// light scheme c switched to above.
	close(mapGate)
	m = pump(t, m, <-done)
	if v := m.View().Content; !strings.Contains(v, "\x1b[37;40;4mhit\x1b[24;30;47m00001") {
		t.Fatalf("a.txt content missing after gate release: %q", v)
	}
}

// The map gate holds the decode/map phase only: a read-phase failure —
// the file is missing — completes promptly even while the gate is
// held, so the two phases are independently observable.
func TestMapGateHoldsDecodeMapNotRead(t *testing.T) {
	dir := t.TempDir()
	_, cmd := gatedModel(t, dir, nil, make(chan struct{}),
		`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`,
		`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}`,
		`{"type":"summary","data":{}}`,
	)
	if cmd == nil {
		t.Fatal("search completion returned no load command")
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case msg := <-done:
		ld, ok := msg.(loadDoneMsg)
		if !ok || ld.err == nil {
			t.Fatalf("missing-file load = %T %v, want loadDoneMsg with an error", msg, msg)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the held decode/map gate blocked a read-phase failure")
	}
}

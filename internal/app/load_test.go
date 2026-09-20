package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

// runWorker executes a command on its own goroutine — a gate-held
// worker blocks until the gate closes or the model cancels — and
// reports the produced message on the returned channel.
func runWorker(t *testing.T, cmd tea.Cmd) <-chan tea.Msg {
	t.Helper()
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	return done
}

// gatedBrowse enters the browse state over idx at w×h with every file
// load held behind the load gate — the returned channel closes to
// release all held workers — and returns the startup file's held
// worker as a message channel.
func gatedBrowse(t *testing.T, dir string, idx *searchindex.Index, w, h int) (Model, chan struct{}, <-chan tea.Msg) {
	t.Helper()
	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, dir)
	m.popupTimer = instantPopupTimer
	gate := make(chan struct{})
	m.loadGate = gate
	m, _ = update(t, m, tea.WindowSizeMsg{Width: w, Height: h})
	m, cmd := update(t, m, searchResult{index: idx, integrity: completeStream})
	if cmd == nil {
		t.Fatal("browse entry started no file load")
	}
	return m, gate, runWorker(t, cmd)
}

// navJobs runs every command a navigation command carries — a load or
// layout request and the pop-up timer — each on its own goroutine,
// returning their message channels in member order. A gate-held
// member's channel stays silent until its gate closes.
func navJobs(t *testing.T, cmd tea.Cmd) []<-chan tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	done := runWorker(t, cmd)
	var msg tea.Msg
	select {
	case msg = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("navigation command did not produce its message")
	}
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		leaf := make(chan tea.Msg, 1)
		leaf <- msg
		return []<-chan tea.Msg{leaf}
	}
	jobs := make([]<-chan tea.Msg, 0, len(batch))
	for _, c := range batch {
		jobs = append(jobs, runWorker(t, c))
	}
	return jobs
}

// collectMsg reads one message from a worker channel, failing when it
// does not arrive.
func collectMsg(t *testing.T, job <-chan tea.Msg, what string) tea.Msg {
	t.Helper()
	select {
	case msg := <-job:
		return msg
	case <-time.After(10 * time.Second):
		t.Fatalf("%s did not complete", what)
		return nil
	}
}

// assertSilent fails when a worker channel has already produced a
// message — used for jobs expected to stay held behind the load gate.
func assertSilent(t *testing.T, job <-chan tea.Msg, what string) {
	t.Helper()
	select {
	case msg := <-job:
		t.Fatalf("%s produced %T while the load gate was held", what, msg)
	default:
	}
}

// beginLoad records a fresh in-flight load request for path — the
// bookkeeping half of a load request without running a worker — and
// returns the request identity a matching completion must carry.
func beginLoad(m Model, path string) (Model, int) {
	m.loadSeq++
	m.loading[path] = m.loadSeq
	return m, m.loadSeq
}

// A slow file never blocks navigation: with a.txt's completed load
// held back undelivered — indistinguishable to the model from a worker
// still running — n moves to b.txt and shows its content, p returns to
// the "Loading…" placeholder without minting a new request, scrolling
// the placeholder is a strict no-op, and the held completion fills in
// a.txt's panel when it finally arrives.
func TestNavigatePastSlowFile(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", "hit a\n")
	writeMatchFile(t, dir, "b.txt", "hit b\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("b.txt", "hit b\n", 1, 0, 3, "hit"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	msgA := cmd() // a.txt's completion, held back: a.txt stays "loading"
	reqA := m.loading["a.txt"]
	// The no-style theme keeps assertions free of the ANSI codes the
	// match highlight inserts inside the fixture text.
	m.theme = theme.Plain()

	// n moves past the slow file immediately and b.txt renders.
	m, cmd = update(t, m, keyMsg("n"))
	m = applyLoad(t, m, deliverNavLoad(t, cmd))
	v := m.View().Content
	if !strings.Contains(v, "── b.txt ") || !strings.Contains(v, "hit b") {
		t.Fatalf("n past a slow file did not show b.txt's content:\n%s", v)
	}
	if got := m.loading["a.txt"]; got != reqA {
		t.Fatalf("a.txt's in-flight request changed to %d, want %d", got, reqA)
	}

	// p re-enters a.txt while it is still loading: the request is
	// dropped — the command carries no load and no new identity is
	// minted — and the panel shows the placeholder again.
	m, cmd = update(t, m, keyMsg("p"))
	for _, msg := range navMsgs(t, cmd) {
		if _, isLoad := msg.(loadResult); isLoad {
			t.Fatal("re-entering a loading file started another load")
		}
	}
	if got := m.loading["a.txt"]; got != reqA || m.loadSeq != 2 {
		t.Fatalf("re-entry re-minted a.txt's request: req %d, seq %d, want %d and 2",
			got, m.loadSeq, reqA)
	}
	v = m.View().Content
	if !strings.Contains(v, "── a.txt ") || !strings.Contains(v, "Loading…") {
		t.Fatalf("re-entry did not restore a.txt's placeholder:\n%s", v)
	}

	// Scrolling the placeholder is a strict no-op — no command, no
	// viewport movement, no view change. The first key also dismisses
	// the file-change pop-up, per Issue 15, so the placeholder is
	// captured after it.
	var c tea.Cmd
	m, c = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	if c != nil {
		t.Fatalf("scrolling the placeholder returned a command: %v", c)
	}
	v = m.View().Content
	if !strings.Contains(v, "Loading…") {
		t.Fatalf("a.txt lost its placeholder after a scroll key:\n%s", v)
	}
	for _, key := range []tea.KeyPressMsg{
		{Code: tea.KeyUp}, {Code: tea.KeyDown}, keyMsg("u"), keyMsg("d"),
		{Code: tea.KeyPgUp}, {Code: tea.KeyPgDown},
	} {
		m, c = update(t, m, key)
		if c != nil {
			t.Fatalf("scrolling the placeholder with %q returned a command: %v", key.String(), c)
		}
	}
	if vp := m.vps["a.txt"]; vp != nil && vp.Top() != 0 {
		t.Fatalf("scrolling the placeholder moved the viewport to row %d", vp.Top())
	}
	if got := m.View().Content; got != v {
		t.Fatalf("scrolling the placeholder changed the view:\nbefore:\n%s\nafter:\n%s", v, got)
	}

	// The held completion lands on its own path: a.txt installs and
	// renders when it finally arrives.
	m = applyLoad(t, m, msgA)
	if got := m.View().Content; !strings.Contains(got, "hit a") {
		t.Fatalf("a.txt's late completion did not render its content:\n%s", got)
	}
	if _, ok := m.loading["a.txt"]; ok {
		t.Fatal("a.txt's completed request was not retired")
	}
}

// A→B→C with each completion held: a.txt's result delivered while
// c.txt is current updates only a.txt's cache — c.txt's panel is
// byte-identical — and a.txt's request retires while b.txt's stays in
// flight. Visiting a.txt after that shows the cached content with no
// new load.
func TestLateCompletionUpdatesOnlyOwnPath(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", "hit a\n")
	writeMatchFile(t, dir, "b.txt", "hit b\n")
	writeMatchFile(t, dir, "c.txt", "hit c\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("b.txt", "hit b\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("c.txt", "hit c\n", 1, 0, 3, "hit"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	msgA := cmd()
	m, cmd = update(t, m, keyMsg("n"))
	msgB := deliverNavLoad(t, cmd)
	m, cmd = update(t, m, keyMsg("n"))
	msgC := deliverNavLoad(t, cmd)
	m.theme = theme.Plain()

	m = applyLoad(t, m, msgC)
	before := m.View().Content
	if !strings.Contains(before, "── c.txt ") || !strings.Contains(before, "hit c") {
		t.Fatalf("c.txt did not render:\n%s", before)
	}

	// a.txt's late completion requests no layout and leaves c.txt's
	// panel untouched; only a.txt's own bookkeeping changes.
	m, c := update(t, m, msgA)
	if c != nil {
		t.Fatalf("a non-current completion returned a command: %v", c)
	}
	if got := m.View().Content; got != before {
		t.Fatalf("a.txt's completion changed c.txt's panel:\nbefore:\n%s\nafter:\n%s", before, got)
	}
	if m.sources["a.txt"] == nil {
		t.Fatal("a.txt's completion did not cache its buffer")
	}
	if _, ok := m.loading["a.txt"]; ok {
		t.Fatal("a.txt's request was not retired by its completion")
	}
	if _, ok := m.loading["b.txt"]; !ok {
		t.Fatal("a.txt's completion retired b.txt's in-flight request")
	}

	// p to b.txt: still in flight, so the placeholder shows and no new
	// load is requested; b.txt's held completion then installs.
	m, cmd = update(t, m, keyMsg("p"))
	for _, msg := range navMsgs(t, cmd) {
		if _, isLoad := msg.(loadResult); isLoad {
			t.Fatal("re-entering a loading file started another load")
		}
	}
	if got := m.View().Content; !strings.Contains(got, "Loading…") {
		t.Fatalf("b.txt did not show its placeholder:\n%s", got)
	}
	m = applyLoad(t, m, msgB)
	if got := m.View().Content; !strings.Contains(got, "hit b") {
		t.Fatalf("b.txt's completion did not render while current:\n%s", got)
	}

	// p to a.txt: the cached buffer needs a prepared layout only — no
	// load — and the panel shows a.txt's content.
	m, cmd = update(t, m, keyMsg("p"))
	sawLayout := false
	for _, msg := range navMsgs(t, cmd) {
		switch msg := msg.(type) {
		case loadResult:
			t.Fatalf("revisiting a cached file started a load for %q", msg.path)
		case layoutResult:
			sawLayout = true
			m, _ = update(t, m, msg)
		}
	}
	if !sawLayout {
		t.Fatal("revisiting a cached file requested no layout")
	}
	if got := m.View().Content; !strings.Contains(got, "hit a") {
		t.Fatalf("a.txt's cached content did not render:\n%s", got)
	}
}

// Successful buffers are retained for the session: once a.txt, b.txt,
// and c.txt have loaded, hopping among them — including across the
// circular wrap — never issues another load request and every panel
// renders from cache.
func TestLoadCacheRetainedForSession(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", "hit a\n")
	writeMatchFile(t, dir, "b.txt", "hit b\n")
	writeMatchFile(t, dir, "c.txt", "hit c\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("b.txt", "hit b\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("c.txt", "hit c\n", 1, 0, 3, "hit"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	for range 2 {
		m, cmd = update(t, m, keyMsg("n"))
		m = applyLoad(t, m, deliverNavLoad(t, cmd))
	}
	m.theme = theme.Plain()
	if len(m.sources) != 3 {
		t.Fatalf("cached %d buffers, want all 3", len(m.sources))
	}

	// Every file now serves from its cache: no load result may appear
	// in any navigation for the rest of the session.
	for _, step := range []struct {
		key  string
		want string
	}{
		{"p", "hit b"},
		{"p", "hit a"},
		{"p", "hit c"}, // p wraps to the last file
		{"n", "hit a"}, // n wraps back to the first
	} {
		var nav tea.Cmd
		m, nav = update(t, m, keyMsg(step.key))
		for _, msg := range navMsgs(t, nav) {
			switch msg := msg.(type) {
			case loadResult:
				t.Fatalf("%s onto a cached file started a load for %q", step.key, msg.path)
			case layoutResult:
				m, _ = update(t, m, msg)
			}
		}
		if got := m.View().Content; !strings.Contains(got, step.want) {
			t.Fatalf("%s did not render %q from cache:\n%s", step.key, step.want, m.View().Content)
		}
	}
}

// With the load gate holding every worker, n onto b.txt starts the one
// permitted b.txt load and p back onto a.txt mints nothing: the
// in-flight request's identity is unchanged, no load rides the
// returned command, and once the gate is released each file completes
// exactly once — a re-entry neither starts nor queues a second load.
func TestReentryWhileLoadingStartsNothing(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", "hit a\n")
	writeMatchFile(t, dir, "b.txt", "hit b\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("b.txt", "hit b\n", 1, 0, 3, "hit"))
	idx.Finish()

	m, gate, jobA := gatedBrowse(t, dir, idx, 80, 24)
	reqA := m.loading["a.txt"]
	m.theme = theme.Plain()

	// n to b.txt starts its one load; both workers stay held.
	m, cmd := update(t, m, keyMsg("n"))
	jobsB := navJobs(t, cmd)
	if _, ok := m.loading["b.txt"]; !ok {
		t.Fatal("n onto b.txt started no load")
	}
	assertSilent(t, jobA, "a.txt's load")

	// p re-enters a.txt mid-load: dropped, not queued — the in-flight
	// request's identity and the request sequence are untouched, and
	// every job the command carries answers instantly (a wrongly
	// started or queued load would be a held worker).
	m, cmd = update(t, m, keyMsg("p"))
	if got := m.loading["a.txt"]; got != reqA {
		t.Fatalf("re-entry changed a.txt's request identity: %d, want %d", got, reqA)
	}
	if m.loadSeq != 2 {
		t.Fatalf("re-entry minted request %d, want the sequence still at 2", m.loadSeq)
	}
	for i, j := range navJobs(t, cmd) {
		select {
		case msg := <-j:
			if _, isLoad := msg.(loadResult); isLoad {
				t.Fatalf("re-entry job %d produced a load result", i)
			}
		default:
			t.Fatalf("re-entry job %d is a held worker — a load was started or queued", i)
		}
	}
	if got := m.View().Content; !strings.Contains(got, "Loading…") {
		t.Fatalf("a.txt lost its placeholder on re-entry:\n%s", got)
	}

	// Releasing the gate completes each started load exactly once.
	close(gate)
	msgA := collectMsg(t, jobA, "a.txt's load")
	lr, ok := msgA.(loadResult)
	if !ok || string(lr.path) != "a.txt" || lr.req != reqA {
		t.Fatalf("a.txt's load produced %#v, want request %d", msgA, reqA)
	}
	loadsB := 0
	for i, j := range jobsB {
		msg := collectMsg(t, j, fmt.Sprintf("b.txt navigation job %d", i))
		if lr, ok := msg.(loadResult); ok {
			loadsB++
			if string(lr.path) != "b.txt" {
				t.Fatalf("navigation loaded %q, want b.txt", lr.path)
			}
		}
	}
	if loadsB != 1 {
		t.Fatalf("b.txt completed %d loads, want exactly 1", loadsB)
	}

	// a.txt is current after the re-entry; delivering its completion
	// installs the content and retires the request.
	m = applyLoad(t, m, msgA)
	if got := m.View().Content; !strings.Contains(got, "hit a") {
		t.Fatalf("a.txt's completion did not render:\n%s", got)
	}
}

// Completions are keyed by raw path and request identity: a result
// whose identity matches no in-flight request — an unknown request on
// a loading path, or any result for a path with no outstanding
// request — is discarded whole: no retired bookkeeping, no cache, no
// diagnostic, no panel change. The real request stays in flight and
// its own completion still lands.
func TestStaleLoadResultDiscarded(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", "hit a\n")
	writeMatchFile(t, dir, "b.txt", "hit b\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("b.txt", "hit b\n", 1, 0, 3, "hit"))
	idx.Finish()

	m, gate, jobA := gatedBrowse(t, dir, idx, 80, 24)
	reqA := m.loading["a.txt"]
	m.theme = theme.Plain()

	// An unknown request identity on a loading path is not a
	// completion: the request stays in flight and nothing is cached.
	m, c := update(t, m, loadResult{
		path: []byte("a.txt"),
		req:  reqA + 1,
		src:  &stubSource{gutter: 9, widths: []int{1}},
	})
	if c != nil {
		t.Fatalf("a stale completion returned a command: %v", c)
	}
	if got := m.loading["a.txt"]; got != reqA {
		t.Fatalf("a stale completion retired a.txt's request: loading = %d, want %d", got, reqA)
	}
	if m.sources["a.txt"] != nil {
		t.Fatal("a stale completion cached a.txt's buffer")
	}
	if got := m.View().Content; !strings.Contains(got, "Loading…") {
		t.Fatalf("a stale completion replaced the placeholder:\n%s", got)
	}

	// A forged failure under an unknown identity records nothing: no
	// failure mark, no diagnostic, no overlay.
	m, c = update(t, m, loadResult{path: []byte("a.txt"), req: reqA + 1, err: errors.New("denied")})
	if c != nil {
		t.Fatalf("a stale failure returned a command: %v", c)
	}
	if m.failed["a.txt"] {
		t.Fatal("a stale failure marked a.txt failed")
	}
	if m.overlay != nil {
		t.Fatal("a stale failure opened the error overlay")
	}

	// Any result for a path with no outstanding request is discarded —
	// even one for a listed file that simply was never asked for.
	m, c = update(t, m, loadResult{
		path: []byte("b.txt"),
		req:  reqA,
		src:  &stubSource{gutter: 9, widths: []int{1}},
	})
	if c != nil {
		t.Fatalf("an unrequested completion returned a command: %v", c)
	}
	if m.sources["b.txt"] != nil {
		t.Fatal("an unrequested completion cached b.txt's buffer")
	}
	if len(m.loading) != 1 {
		t.Fatalf("an unrequested completion disturbed the load bookkeeping: %v", m.loading)
	}

	// The held worker's own completion still matches and lands.
	close(gate)
	msg := collectMsg(t, jobA, "a.txt's gated load")
	lr, ok := msg.(loadResult)
	if !ok || string(lr.path) != "a.txt" || lr.req != reqA {
		t.Fatalf("gated load produced %#v, want a.txt's request %d", msg, reqA)
	}
	m = applyLoad(t, m, msg)
	if got := m.View().Content; !strings.Contains(got, "hit a") {
		t.Fatalf("a.txt's real completion did not render:\n%s", got)
	}

	// A replayed duplicate of the retired request is already stale: it
	// must not rewrite the installed cache.
	m, c = update(t, m, loadResult{
		path: []byte("a.txt"),
		req:  reqA,
		src:  &stubSource{gutter: 9, widths: []int{1}},
	})
	if c != nil {
		t.Fatalf("a retired request's duplicate returned a command: %v", c)
	}
	if got := m.sources["a.txt"].GutterWidth(); got == 9 {
		t.Fatal("a retired request's duplicate overwrote a.txt's buffer")
	}
}

// The decode/map phase is gatable independently of layout: with the
// current file's load held, every input keeps its meaning — n advances
// to b.txt and starts its own held load, p returns to a.txt's
// placeholder without a new request, w flips the wrap mode, c flips
// the scheme, a resize is accepted — and ctrl+c exits 130 through the
// cancellation path, releasing the held workers whose late results
// cannot revive the UI.
func TestGatedDecodeMapKeepsInputResponsive(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", "hit a\n")
	writeMatchFile(t, dir, "b.txt", "hit b\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("b.txt", "hit b\n", 1, 0, 3, "hit"))
	idx.Finish()

	child := &fakeChild{
		stdout:     strings.NewReader(""),
		stderr:     strings.NewReader(""),
		terminated: make(chan struct{}),
	}
	m := New(child, dir)
	m.popupTimer = instantPopupTimer
	m.loadGate = make(chan struct{})
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, searchResult{index: idx, integrity: completeStream})
	jobA := runWorker(t, cmd)
	assertSilent(t, jobA, "a.txt's decode/map phase")

	// n advances to b.txt immediately and starts its own held load.
	m, nav := update(t, m, keyMsg("n"))
	if stop, _ := m.index.Current(); string(stop.Path) != "b.txt" {
		t.Fatalf("n during a held decode selected %q, want b.txt", stop.Path)
	}
	jobsB := navJobs(t, nav)
	if _, ok := m.loading["b.txt"]; !ok {
		t.Fatal("n during a held decode started no load for b.txt")
	}
	v := m.View().Content
	if !strings.Contains(v, "── b.txt ") || !strings.Contains(v, "Loading…") {
		t.Fatalf("b.txt's panel did not appear during a held decode:\n%s", v)
	}

	// p returns to a.txt: the re-entry is dropped, the command is only
	// the pop-up's expiry, and the placeholder shows again.
	m, nav = update(t, m, keyMsg("p"))
	jobsP := navJobs(t, nav)
	if len(jobsP) != 1 {
		t.Fatalf("re-entry carried %d jobs, want just the pop-up timer", len(jobsP))
	}
	msg := collectMsg(t, jobsP[0], "the re-entry's pop-up timer")
	if em, ok := msg.(popupExpiredMsg); !ok || em.id != m.popup.id {
		t.Fatalf("re-entry produced %#v, want the pop-up's expiry", msg)
	}
	if got := m.View().Content; !strings.Contains(got, "Loading…") {
		t.Fatalf("a.txt lost its placeholder during held decodes:\n%s", got)
	}

	// w flips the wrap mode immediately — a.txt has no buffer to lay
	// out yet, so no command — and c flips the scheme.
	m, c := update(t, m, keyMsg("w"))
	if m.wrap {
		t.Fatal("w did not flip the wrap mode while the decode was held")
	}
	if c != nil {
		t.Fatalf("w during a held decode returned a command: %v", c)
	}
	m, c = update(t, m, keyMsg("c"))
	if c != nil {
		t.Fatalf("c during a held decode returned a command: %v", c)
	}
	if got := m.View().Content; !strings.HasPrefix(got, "\x1b[30;47m") {
		t.Fatalf("c did not flip the scheme while the decode was held:\n%q", got)
	}

	// A resize is accepted and applies immediately.
	m, rs := update(t, m, tea.WindowSizeMsg{Width: 100, Height: 40})
	if m.width != 100 || m.height != 40 {
		t.Fatalf("resize during a held decode not accepted: %dx%d", m.width, m.height)
	}
	if rs != nil {
		t.Fatalf("resize during a held decode returned a command: %v", rs)
	}

	// ctrl+c cancels immediately: quit, 130, child terminated.
	m, quit := update(t, m, keyPress("ctrl+c"))
	requireQuit(t, quit, "ctrl+c during a held decode")
	if m.status != 130 {
		t.Fatalf("exit status = %d, want 130", m.status)
	}
	requireClosed(t, child.terminated, "child termination")

	// Both held workers return promptly on cancellation; their late
	// results — and any forged one — cannot revive the cancelled run.
	for i, j := range append([]<-chan tea.Msg{jobA}, jobsB...) {
		msg := collectMsg(t, j, fmt.Sprintf("held job %d", i))
		m, c = update(t, m, msg)
		if c != nil {
			t.Fatalf("a late result after cancellation returned a command: %v", c)
		}
	}
	m, c = update(t, m, loadResult{
		path: []byte("a.txt"),
		req:  1,
		src:  &stubSource{gutter: 3, widths: []int{1}},
	})
	if c != nil {
		t.Fatalf("a forged result after cancellation returned a command: %v", c)
	}
	if m.state != stateCancelled || m.status != 130 {
		t.Fatalf("a late result revived the UI: state %d, status %d", m.state, m.status)
	}
}

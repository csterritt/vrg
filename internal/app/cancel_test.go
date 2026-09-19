package app

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/searchindex"
)

var (
	keyQ     = tea.KeyPressMsg{Text: "q", Code: 'q'}
	keyCtrlC = tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	keyEsc   = tea.KeyPressMsg{Code: tea.KeyEscape}
)

// killChild is a still-running child: Wait blocks until Terminate, so a
// cleanup path that quits without terminating the child hangs instead of
// silently passing.
type killChild struct {
	res  Result
	dead chan struct{}
	once sync.Once
}

func newKillChild(res Result) *killChild {
	return &killChild{res: res, dead: make(chan struct{})}
}

func (k *killChild) Terminate() { k.once.Do(func() { close(k.dead) }) }

func (k *killChild) Wait() Result {
	<-k.dead
	return k.res
}

// runQuittingCmd runs the command a controlled exit returns. It must
// produce tea.QuitMsg only after terminating and reaping the child; the
// bound exists solely to fail a cleanup path that forgets to terminate.
func runQuittingCmd(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("controlled exit returned a nil command, want tea.Quit after cleanup")
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case msg := <-done:
		if _, ok := msg.(tea.QuitMsg); !ok {
			t.Fatalf("cleanup command produced %T, want tea.QuitMsg", msg)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("cleanup command hung; the still-running child was not terminated")
	}
}

// doneMsg builds the search completion message the collection command
// delivers once the stream is collected and the index prepared.
func doneMsg() searchDoneMsg {
	return searchDoneMsg{
		res: Result{Stdout: []byte(happyStream), Code: 0},
		idx: searchindex.Build([]byte(happyStream), "/wd"),
	}
}

// q while searching is cancellation: status 130, the child terminated
// and reaped, then quit — not a browse quit.
func TestQWhileSearchingCancels(t *testing.T) {
	reaped := make(chan Result, 1)
	child := newKillChild(Result{Code: -1, Err: errors.New("signal: killed")})
	m := newTestModel(child, options{reap: func(r Result) { reaped <- r }})

	_, cmd := m.Update(keyQ)
	if !m.quitting {
		t.Fatal("q while searching did not begin a controlled exit")
	}
	runQuittingCmd(t, cmd)
	if m.status != 130 {
		t.Fatalf("status = %d, want 130", m.status)
	}
	select {
	case r := <-reaped:
		if r.Code != -1 {
			t.Fatalf("reaped code = %d, want -1 for the killed child", r.Code)
		}
	default:
		t.Fatal("wait/reap path did not run before quit")
	}
	if v := viewText(m); strings.Contains(v, "matched lines") {
		t.Fatalf("view after cancel = %q, want no summary screen", v)
	}
}

// ctrl+c cancels from any state. While searching it behaves exactly like
// the q cancellation: terminate, reap, restore, exit 130.
func TestCtrlCWhileSearchingCancels(t *testing.T) {
	reaped := make(chan Result, 1)
	child := newKillChild(Result{Code: -1, Err: errors.New("signal: killed")})
	m := newTestModel(child, options{reap: func(r Result) { reaped <- r }})

	_, cmd := m.Update(keyCtrlC)
	if !m.quitting {
		t.Fatal("ctrl+c while searching did not begin a controlled exit")
	}
	runQuittingCmd(t, cmd)
	if m.status != 130 {
		t.Fatalf("status = %d, want 130", m.status)
	}
	select {
	case <-reaped:
	default:
		t.Fatal("wait/reap path did not run before quit")
	}
}

// ctrl+c on the browse view overrides the fixed search-derived status
// with 130 and cleans up the same way.
func TestCtrlCOnBrowseCancels(t *testing.T) {
	m := newTestModel(newKillChild(Result{Code: 0}), options{})
	m.Update(doneMsg())
	if m.state != stateBrowse {
		t.Fatalf("state = %v, want browse", m.state)
	}
	_, cmd := m.Update(keyCtrlC)
	if !m.quitting {
		t.Fatal("ctrl+c on summary did not begin a controlled exit")
	}
	runQuittingCmd(t, cmd)
	if m.status != 130 {
		t.Fatalf("status = %d, want ctrl+c's 130 override", m.status)
	}
}

// Esc during searching is a no-op: no state change, no command.
func TestEscWhileSearchingNoOp(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Stdout: []byte(happyStream)}}, options{})
	_, cmd := m.Update(keyEsc)
	if cmd != nil {
		t.Fatal("Esc while searching returned a command, want a no-op")
	}
	if m.quitting || m.state != stateSearching {
		t.Fatalf("Esc changed state: quitting=%v state=%v", m.quitting, m.state)
	}
	if v := viewText(m); !strings.Contains(v, "Searching") {
		t.Fatalf("view = %q, want it to keep saying Searching", v)
	}
}

// A search completion arriving after cancellation must not revive the
// UI: the state stays cancelled and the browse view never appears.
func TestLateCompletionAfterCancelDiscarded(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Stdout: []byte(happyStream)}}, options{})
	_, cmd := m.Update(keyQ)
	runQuittingCmd(t, cmd)

	m.Update(doneMsg())
	if m.state == stateBrowse {
		t.Fatal("late completion revived the browse view after cancellation")
	}
	if m.status != 130 {
		t.Fatalf("status = %d, want the cancellation's 130", m.status)
	}
	if v := viewText(m); strings.Contains(v, "─") {
		t.Fatalf("view = %q after cancellation, want no browse frame", v)
	}
}

// q while index preparation is gate-held — rg has already exited and its
// stream is fully collected — is still cancellation (130), not a browse
// quit, and the late completion that lands when the gate releases is
// discarded.
func TestQDuringGateHeldPreparationCancels(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	opts := options{gate: func() {
		close(entered)
		<-release
	}}
	m := newTestModel(fakeChild{res: Result{Stdout: []byte(happyStream)}}, opts)

	done := make(chan tea.Msg, 1)
	go func() { done <- runCollectCmd(t, m) }()
	<-entered // rg exited and its stream is collected; preparation is held

	_, cmd := m.Update(keyQ)
	runQuittingCmd(t, cmd)
	if m.status != 130 {
		t.Fatalf("status = %d, want 130", m.status)
	}
	if !m.quitting {
		t.Fatal("q during held preparation did not begin a controlled exit")
	}

	close(release)
	m.Update(<-done)
	if m.state == stateBrowse {
		t.Fatal("released gate completion revived the browse view after cancellation")
	}
	if v := viewText(m); strings.Contains(v, "─") {
		t.Fatalf("view = %q after cancellation, want no browse frame", v)
	}
}

// An ordinary exit while the child still runs terminates and reaps it:
// the quit command cannot complete without doing both, so a browse quit
// never leaves a running or unreaped child behind.
func TestOrdinaryQuitTerminatesRunningChild(t *testing.T) {
	reaped := make(chan Result, 1)
	child := newKillChild(Result{Code: 0})
	m := newTestModel(child, options{reap: func(r Result) { reaped <- r }})
	m.Update(doneMsg())
	if m.state != stateBrowse {
		t.Fatalf("state = %v, want browse", m.state)
	}
	_, cmd := m.Update(keyQ)
	runQuittingCmd(t, cmd)
	if m.status != 0 {
		t.Fatalf("status = %d, want the ordinary exit's 0", m.status)
	}
	select {
	case r := <-reaped:
		if r.Code != 0 {
			t.Fatalf("reaped code = %d, want the child's 0", r.Code)
		}
	default:
		t.Fatal("ordinary exit did not run the wait/reap path")
	}
}

// An injected controlled failure begins the same cleanup exit while
// recording the error for the post-restoration diagnostic.
func TestFailMsgTriggersCleanup(t *testing.T) {
	reaped := make(chan Result, 1)
	child := newKillChild(Result{Code: -1, Err: errors.New("signal: killed")})
	m := newTestModel(child, options{reap: func(r Result) { reaped <- r }})

	boom := errors.New("boom")
	_, cmd := m.Update(failMsg{err: boom})
	if !errors.Is(m.failErr, boom) {
		t.Fatalf("failErr = %v, want the injected failure", m.failErr)
	}
	if !m.quitting {
		t.Fatal("controlled failure did not begin a controlled exit")
	}
	runQuittingCmd(t, cmd)
	select {
	case <-reaped:
	default:
		t.Fatal("controlled failure did not run the wait/reap path")
	}
}

// With a failure hook installed, Init batches the hook command with
// collection; the hook's non-nil return surfaces as a failMsg.
func TestInitRunsFailureHook(t *testing.T) {
	trigger := make(chan struct{})
	m := newTestModel(
		fakeChild{res: Result{Stdout: []byte(happyStream)}},
		options{fail: func() error {
			<-trigger
			return errors.New("boom")
		}},
	)
	init := m.Init()
	if init == nil {
		t.Fatal("Init returned nil command")
	}
	msg := init()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("Init command produced %T, want a tea.BatchMsg of collection and the hook", msg)
	}
	if len(batch) != 2 {
		t.Fatalf("Init batch = %d commands, want 2", len(batch))
	}
	close(trigger)
	var sawDone, sawFail bool
	for _, c := range batch {
		switch c().(type) {
		case searchDoneMsg:
			sawDone = true
		case failMsg:
			sawFail = true
		}
	}
	if !sawDone || !sawFail {
		t.Fatalf("Init batch produced done=%v fail=%v, want both", sawDone, sawFail)
	}
}

// When the program dies without the model's quit command running — here
// a pre-cancelled context kills it — Run still terminates and reaps the
// child and writes exactly one sanitized diagnostic to stderr.
func TestRunCleansUpOnProgramError(t *testing.T) {
	reaps := make(chan Result, 2)
	child := newKillChild(Result{Code: -1, Err: errors.New("signal: killed")})
	var stderr bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cfg := Config{
		Args: []string{"--json", "--", "x", "."},
		Dir:  t.TempDir(),
		Err:  &stderr,
		Start: func(context.Context, []string, string) (Child, error) {
			return child, nil
		},
	}
	code := Run(ctx, cfg, WithReapReport(func(r Result) { reaps <- r }))
	if code != 2 {
		t.Fatalf("Run = %d, want exit 2", code)
	}
	select {
	case <-reaps:
	default:
		t.Fatal("the wait/reap path never ran for the still-running child")
	}
	select {
	case <-reaps:
		t.Fatal("reap report fired twice; want exactly once")
	default:
	}
	diag := strings.TrimSpace(stderr.String())
	if !strings.HasPrefix(diag, "vrg:") || strings.Contains(diag, "\n") {
		t.Fatalf("diagnostic = %q, want a single vrg: line", diag)
	}
	if strings.ContainsAny(diag, "\x1b\x9b") {
		t.Fatalf("diagnostic contains raw control bytes: %q", diag)
	}
}

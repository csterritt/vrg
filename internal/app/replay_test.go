package app

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/searchindex"
)

// replayOutput runs the common post-restoration stderr writer over the
// model's session diagnostic collection — the bytes Run emits once the
// terminal is restored.
func replayOutput(t *testing.T, m *model) string {
	t.Helper()
	var buf bytes.Buffer
	replayDiags(&buf, m.diags)
	return buf.String()
}

func assertReplayLines(t *testing.T, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("collected diagnostics = %q, want %q", got, want)
	}
}

// The session collection is independent of display: a stderr warning
// shown in the overlay and two load failures never shown all reach the
// replay writer exactly once each, in collection order, on a normal
// exit. The test-only acknowledgement fires once per collected line.
func TestReplayCollectsEveryDiagInOrder(t *testing.T) {
	var acks int
	res := Result{Stdout: []byte(happyStream), Stderr: []byte("warn one\n"), Code: 0}
	m := newTestModel(fakeChild{res: res}, options{diagAck: func() { acks++ }})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.Update(runCollectCmd(t, m))
	if !m.overlayOpen {
		t.Fatal("the stderr warning did not open the overlay")
	}
	if v := viewText(m); !strings.Contains(v, "warn one") {
		t.Fatalf("view = %q, want the stderr warning displayed", v)
	}

	// Two diagnostics that never reach the display: load failures
	// collected without an overlay. Each failure answers an in-flight
	// request — minted here as a visit would have minted it.
	m.Update(fileLoadedMsg{path: []byte("./b.go"), req: mintRequest(m, []byte("./b.go")), err: errors.New("b broke")})
	m.Update(fileLoadedMsg{path: []byte("./c.go"), req: mintRequest(m, []byte("./c.go")), err: errors.New("c broke")})
	m.Update(keyEsc) // dismiss the overlay; the collection is unaffected
	if v := viewText(m); strings.Contains(v, "broke") {
		t.Fatalf("view = %q — a never-displayed diagnostic leaked into the frame", v)
	}

	_, cmd := m.Update(keyQ)
	runQuittingCmd(t, cmd)
	if m.status != 0 {
		t.Fatalf("status = %d, want the ordinary exit's 0", m.status)
	}

	want := []string{
		"warn one",
		"cannot read ./b.go: b broke",
		"cannot read ./c.go: c broke",
	}
	assertReplayLines(t, m.diags, want)
	if acks != len(want) {
		t.Fatalf("collection acks = %d, want one per collected line (%d)", acks, len(want))
	}
	out := replayOutput(t, m)
	last := -1
	for _, w := range want {
		if n := strings.Count(out, w); n != 1 {
			t.Fatalf("replayed diagnostic %q appears %d times, want exactly once: %q", w, n, out)
		}
		i := strings.Index(out, w)
		if i < last {
			t.Fatalf("replayed diagnostics out of collection order: %q", out)
		}
		last = i
	}
}

// Shutdown boundary, ctrl+c route: a diagnostic delivered and processed
// in one update is replayed when ctrl+c lands in the next; a diagnostic
// still in flight — here gated and undelivered — is not waited for and
// is never replayed.
func TestCtrlCReplayBoundary(t *testing.T) {
	m := newTestModel(newKillChild(Result{Code: -1, Err: errors.New("signal: killed")}), options{})
	m.Update(stderrLineMsg{text: "warn one\n"})

	// A gated diagnostic stays in flight past the exit decision.
	gate := make(chan struct{})
	delivered := make(chan tea.Msg, 1)
	go func() {
		<-gate
		delivered <- stderrLineMsg{text: "late warn\n"}
	}()

	_, cmd := m.Update(keyCtrlC)
	runQuittingCmd(t, cmd) // completes while the gate still holds
	if m.status != 130 {
		t.Fatalf("status = %d, want 130", m.status)
	}
	close(gate)
	m.Update(<-delivered) // lands after the exit decision: discarded

	assertReplayLines(t, m.diags, []string{"warn one"})
	out := replayOutput(t, m)
	if strings.Contains(out, "late warn") {
		t.Fatalf("an undelivered diagnostic was replayed: %q", out)
	}
	if !strings.Contains(out, "warn one") {
		t.Fatalf("the processed diagnostic was not replayed: %q", out)
	}
}

// The same boundary on the q route: a diagnostic acknowledged as
// collected before q arrives — while searching or while gate-held
// result preparation is still incomplete — is replayed with exit 130,
// and the exit waits on no undelivered work.
func TestQWhileSearchingReplayBoundary(t *testing.T) {
	for _, tc := range []struct {
		name string
		gate bool // hold index preparation inside the collection command
	}{
		{"while searching", false},
		{"gate-held preparation", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var opts options
			var entered, release chan struct{}
			child := Child(newKillChild(Result{Code: -1, Err: errors.New("signal: killed")}))
			if tc.gate {
				entered, release = make(chan struct{}), make(chan struct{})
				opts.gate = func() {
					close(entered)
					<-release
				}
				// The gate sits inside collection after the child
				// exited: use an already-finished child so Wait
				// returns and the gate is reached.
				child = fakeChild{res: Result{Stdout: []byte(happyStream)}}
			}
			m := newTestModel(child, opts)
			var collected chan tea.Msg
			if tc.gate {
				collected = make(chan tea.Msg, 1)
				go func() { collected <- runCollectCmd(t, m) }()
				<-entered // rg exited, stream collected, preparation held
			}
			m.Update(stderrLineMsg{text: "warn one\n"})

			_, cmd := m.Update(keyQ)
			runQuittingCmd(t, cmd) // no wait on the gate-held collection
			if m.status != 130 {
				t.Fatalf("status = %d, want 130", m.status)
			}

			if tc.gate {
				close(release)
				m.Update(<-collected) // the late completion is discarded
			}
			assertReplayLines(t, m.diags, []string{"warn one"})
			if out := replayOutput(t, m); !strings.Contains(out, "warn one") {
				t.Fatalf("collected diagnostic missing from replay: %q", out)
			}
		})
	}
}

// The controlled-failure diagnostic enters the session collection
// before shutdown and reaches the common post-restoration writer
// through it — earlier diagnostics first, the failure line last, each
// exactly once.
func TestControlledFailureEntersCollection(t *testing.T) {
	m := newTestModel(newKillChild(Result{Code: 0}), options{})
	m.Update(stderrLineMsg{text: "warn one\n"})

	boom := errors.New("injected test failure \x1b[7m")
	_, cmd := m.Update(failMsg{err: boom})
	runQuittingCmd(t, cmd)

	want := []string{"warn one", "vrg: injected test failure ^[[7m"}
	assertReplayLines(t, m.diags, want)
	out := replayOutput(t, m)
	if n := strings.Count(out, "injected test failure"); n != 1 {
		t.Fatalf("failure diagnostic replayed %d times, want exactly once: %q", n, out)
	}
	if strings.Index(out, "warn one") > strings.Index(out, "vrg:") {
		t.Fatalf("failure diagnostic replayed before earlier diagnostics: %q", out)
	}
}

// The shutdown snapshot mirrors the session collection independently
// of the final-model assertion: every line Update processes lands in
// Run's retained slice, so the replay survives a final model that is
// absent or the wrong type (Issue #46).
func TestDiagSinkMirrorsCollection(t *testing.T) {
	var snapshot []string
	m := newTestModel(fakeChild{res: Result{Code: 0}},
		options{diagSink: &snapshot})
	m.Update(stderrLineMsg{text: "warn one\n"})
	m.Update(stderrLineMsg{text: "warn two\n"})
	assertReplayLines(t, snapshot, []string{"warn one", "warn two"})
	assertReplayLines(t, m.diags, snapshot)
}

// A child with the incremental stderr channel collects its stderr
// through stderrLineMsg alone: the completion must not re-collect the
// captured stderr, or the replay would repeat it.
func TestCompletionDoesNotRecollectIncrementalStderr(t *testing.T) {
	ch := make(chan string, 1)
	child := fakeChild{
		res:   Result{Stdout: []byte(happyStream), Stderr: []byte("warn one\n"), Code: 0},
		diags: ch,
	}
	m := newTestModel(child, options{})
	m.Update(stderrLineMsg{text: "warn one\n"})
	m.Update(searchDoneMsg{
		res: Result{Stdout: []byte(happyStream), Stderr: []byte("warn one\n"), Code: 0},
		idx: searchindex.Build([]byte(happyStream), "/wd"),
	})
	assertReplayLines(t, m.diags, []string{"warn one"})
	if out := replayOutput(t, m); strings.Count(out, "warn one") != 1 {
		t.Fatalf("stderr diagnostic replayed more than once: %q", out)
	}
}

// A diagnostic embedding a filename with a newline and an ESC byte is
// escaped and single-lined through the safe-presentation utility: one
// collected line, one replayed line, no raw control bytes.
func TestReplayEscapesEmbeddedFilename(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.Update(fileLoadedMsg{
		path: []byte("bad\nname\x1b.txt"),
		req:  mintRequest(m, []byte("bad\nname\x1b.txt")),
		err:  errors.New("no such file or directory"),
	})
	want := `cannot read bad\nname^[.txt: no such file or directory`
	assertReplayLines(t, m.diags, []string{want})
	out := replayOutput(t, m)
	if !strings.Contains(out, want) {
		t.Fatalf("escaped single-line diagnostic missing from replay: %q", out)
	}
	if strings.Count(strings.TrimSuffix(out, "\n"), "\n") != 0 {
		t.Fatalf("replayed diagnostic spans multiple lines: %q", out)
	}
	if strings.ContainsAny(out, "\x1b") {
		t.Fatalf("replayed diagnostic carries a raw ESC: %q", out)
	}
}

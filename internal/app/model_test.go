package app

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"vrg/internal/searchindex"
)

// fixtureIndex builds a prepared index from decoded fixture records.
func fixtureIndex(t *testing.T, workdir string, records ...string) *searchindex.Index {
	t.Helper()
	ix := searchindex.New(workdir)
	for _, r := range records {
		rec, err := searchindex.DecodeRecord([]byte(r))
		if err != nil {
			t.Fatalf("DecodeRecord(%s): %v", r, err)
		}
		ix.Add(rec)
	}
	ix.Prepare()
	return ix
}

// update drives one message through Update and re-anchors the concrete
// model type.
func update(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	nm, cmd := m.Update(msg)
	return nm.(Model), cmd
}

// awaitMsg runs the Init command's wait on a goroutine and bounds the
// wait: exceeding the budget means collection deadlocked, which fails
// rather than hangs the suite.
func awaitMsg(t *testing.T, m Model, budget time.Duration) tea.Msg {
	t.Helper()
	ch := make(chan tea.Msg, 1)
	go func() { ch <- m.Init()() }()
	select {
	case msg := <-ch:
		return msg
	case <-time.After(budget):
		t.Fatal("collection did not complete within budget; suspected deadlock")
		return nil
	}
}

func keyPress(s string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Text: s, Code: []rune(s)[0]}
}

// ctrlCPress is the message a raw-mode terminal delivers for ctrl+c.
func ctrlCPress() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
}

// escPress is the message a bare Escape keypress delivers.
func escPress() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyEscape}
}

// A completed model for browse-phase tests: the search finished with a
// prepared index and the model shows the browse view.
func completedModel(t *testing.T, files int) Model {
	t.Helper()
	var recs []string
	for f := 0; f < files; f++ {
		p := "f" + string(rune('a'+f))
		recs = append(recs,
			`{"type":"begin","data":{"path":{"text":"`+p+`"}}}`,
			`{"type":"match","data":{"path":{"text":"`+p+`"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
			`{"type":"end","data":{"path":{"text":"`+p+`"},"binary_offset":null}}`,
		)
	}
	recs = append(recs, `{"type":"summary","data":{}}`)
	m := newModel(nil, nil)
	m, _ = update(t, m, searchDoneMsg{index: fixtureIndex(t, "/w", recs...)})
	return m
}

// The searching screen covers collection: while the completion channel
// is silent the model renders "Searching…" and nothing else.
func TestSearchingScreenShownWhileCollecting(t *testing.T) {
	m := newModel(make(chan searchDoneMsg), nil)
	if got := m.View().Content; !strings.Contains(got, "Searching…") {
		t.Fatalf("searching view = %q, want it to contain \"Searching…\"", got)
	}
}

// Resize messages are handled while searching without touching
// collection: the model stores the new dimensions and stays in the
// searching state.
func TestResizeDuringSearching(t *testing.T) {
	m := newModel(make(chan searchDoneMsg), nil)
	m2, _ := update(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	if m2.width != 120 || m2.height != 40 {
		t.Fatalf("dimensions = %dx%d, want 120x40", m2.width, m2.height)
	}
	if got := m2.View().Content; !strings.Contains(got, "Searching…") {
		t.Fatalf("view after resize = %q, still want \"Searching…\"", got)
	}
}

// On the browse view q exits with status 0 through the ordinary cleanup
// path.
func TestQOnBrowseExitsZeroCleanup(t *testing.T) {
	m := completedModel(t, 1)
	m2, cmd := update(t, m, keyPress("q"))
	if cmd == nil {
		t.Fatal("q on browse returned no command, want tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("q on browse command = %T, want tea.QuitMsg", cmd())
	}
	if m2.ExitCode() != 0 {
		t.Fatalf("ExitCode = %d, want 0", m2.ExitCode())
	}
}

// Index preparation is gated independently of rg exit: after the child
// has exited and both pipes are drained, the gate holds preparation and
// the app stays in the searching state until it is released.
func TestGateHoldsSearchingAfterRgExit(t *testing.T) {
	work := t.TempDir()
	out := t.TempDir()
	t.Setenv("VRG_FAKE_RG", "echo")
	t.Setenv("VRG_FAKE_DIR", out)
	drained := make(chan struct{})
	gate := make(chan struct{})
	sess, err := Start(context.Background(), Config{
		Rg:          os.Args[0],
		Argv:        []string{"--json", "--no-config", "--", "foo", "."},
		Workdir:     work,
		Drained:     drained,
		PrepareGate: gate,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	m := sess.Model()

	// The child has fully exited and both pipes are drained; the gate is
	// still holding index preparation.
	select {
	case <-drained:
	case <-time.After(10 * time.Second):
		t.Fatal("child did not exit and drain within budget")
	}
	select {
	case <-m.done:
		t.Fatal("completion delivered while the preparation gate was held")
	default:
	}
	if got := m.View().Content; !strings.Contains(got, "Searching…") {
		t.Fatalf("view after rg exit with gate held = %q, want \"Searching…\"", got)
	}

	close(gate)
	msg := awaitMsg(t, m, 10*time.Second)
	m2, _ := update(t, m, msg)
	if got := m2.View().Content; !strings.Contains(got, "Loading…") {
		t.Fatalf("view after gate release = %q, want the browse placeholder", got)
	}
}

// q while collection is incomplete cancels the search: the model
// signals the child's termination, quits, and settles on exit 130.
func TestQWhileSearchingCancels(t *testing.T) {
	cancelled := false
	m := newModel(make(chan searchDoneMsg), func() { cancelled = true })
	m2, cmd := update(t, m, keyPress("q"))
	if cmd == nil {
		t.Fatal("q while searching returned no command, want tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("q while searching command = %T, want tea.QuitMsg", cmd())
	}
	if m2.ExitCode() != 130 {
		t.Fatalf("ExitCode = %d, want 130", m2.ExitCode())
	}
	if !cancelled {
		t.Fatal("q while searching did not signal child termination")
	}
}

// q pressed after rg has exited but while index preparation is still
// gate-held is cancellation, not a browse quit: exit 130, and the
// collector abandons the held gate promptly rather than leaking.
func TestQDuringGateHeldPreparationCancels(t *testing.T) {
	work := t.TempDir()
	out := t.TempDir()
	t.Setenv("VRG_FAKE_RG", "echo")
	t.Setenv("VRG_FAKE_DIR", out)
	drained := make(chan struct{})
	gate := make(chan struct{})
	sess, err := Start(context.Background(), Config{
		Rg:          fakeRg(),
		Argv:        []string{"--json", "--no-config", "--", "foo", "."},
		Workdir:     work,
		Drained:     drained,
		PrepareGate: gate,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	m := sess.Model()

	// The child has exited and both pipes are drained; preparation is
	// still gate-held, so the app is still searching.
	select {
	case <-drained:
	case <-time.After(10 * time.Second):
		t.Fatal("child did not exit and drain within budget")
	}
	m2, cmd := update(t, m, keyPress("q"))
	if cmd == nil {
		t.Fatal("q during gate-held preparation returned no command, want tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("q during gate-held preparation command = %T, want tea.QuitMsg", cmd())
	}
	if m2.ExitCode() != 130 {
		t.Fatalf("ExitCode = %d, want 130", m2.ExitCode())
	}

	// Cancellation abandons the gate: the collector finishes without the
	// gate ever being released, and a cancelled collection prepares no
	// index.
	select {
	case msg := <-m.done:
		if msg.index != nil {
			t.Fatal("cancelled collection still prepared an index")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("collection did not end promptly after cancellation")
	}
}

// ctrl+c cancels from any state: while searching and in the browse
// view it quits with exit 130 after signalling the child.
func TestCtrlCCancelsFromAnyState(t *testing.T) {
	for _, name := range []string{"searching", "browse"} {
		t.Run(name, func(t *testing.T) {
			cancelled := false
			var m Model
			if name == "browse" {
				m = completedModel(t, 1)
				m.cancel = func() { cancelled = true }
			} else {
				m = newModel(make(chan searchDoneMsg), func() { cancelled = true })
			}
			m2, cmd := update(t, m, ctrlCPress())
			if cmd == nil {
				t.Fatal("ctrl+c returned no command, want tea.Quit")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatalf("ctrl+c command = %T, want tea.QuitMsg", cmd())
			}
			if m2.ExitCode() != 130 {
				t.Fatalf("ExitCode = %d, want 130", m2.ExitCode())
			}
			if !cancelled {
				t.Fatal("ctrl+c did not signal child termination")
			}
		})
	}
}

// Esc during searching is a no-op: no state change, no command, no quit.
func TestEscDuringSearchingIsNoOp(t *testing.T) {
	m := newModel(make(chan searchDoneMsg), nil)
	m2, cmd := update(t, m, escPress())
	if cmd != nil {
		t.Fatalf("Esc while searching produced a command %T, want none", cmd)
	}
	if m2.phase != phaseSearching || m2.quit || m2.ExitCode() != 0 {
		t.Fatalf("Esc while searching changed state: phase=%d quit=%v code=%d",
			m2.phase, m2.quit, m2.ExitCode())
	}
	if got := m2.View().Content; !strings.Contains(got, "Searching…") {
		t.Fatalf("view after Esc = %q, still want \"Searching…\"", got)
	}
}

// A search completion that arrives after cancellation must not revive
// the UI: the model stays quit at exit 130 and never reaches the
// summary.
func TestLateCompletionAfterCancellationDoesNotRevive(t *testing.T) {
	m := newModel(make(chan searchDoneMsg), func() {})
	m2, _ := update(t, m, keyPress("q"))
	m3, cmd := update(t, m2, searchDoneMsg{index: fixtureIndex(t, "/w",
		`{"type":"begin","data":{"path":{"text":"a"}}}`,
		`{"type":"match","data":{"path":{"text":"a"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`,
		`{"type":"end","data":{"path":{"text":"a"},"binary_offset":null}}`,
		`{"type":"summary","data":{}}`,
	)})
	if cmd != nil {
		t.Fatalf("late completion produced a command %T, want none", cmd)
	}
	if !m3.quit || m3.ExitCode() != 130 {
		t.Fatalf("late completion revived the UI: quit=%v code=%d", m3.quit, m3.ExitCode())
	}
	if m3.phase != phaseSearching {
		t.Fatalf("late completion moved phase to %d, want it unchanged", m3.phase)
	}
}

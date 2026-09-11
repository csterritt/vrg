package app_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
)

// keyPress constructs a KeyPressMsg for a simple printable key with no
// modifiers.
func keyPress(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Text: string(code)}
}

// update calls m.Update and type-asserts the returned model back to
// app.Model.
func update(t *testing.T, m app.Model, msg tea.Msg) (app.Model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	am, ok := next.(app.Model)
	if !ok {
		t.Fatalf("Update returned %T, want app.Model", next)
	}
	return am, cmd
}

// ctrlC constructs a KeyPressMsg for ctrl+c.
func ctrlC() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
}

// execCmd executes a tea.Cmd and returns the resulting message, or nil if
// the command is nil.
func execCmd(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	return cmd()
}

// assertQuit requires cmd to produce a tea.QuitMsg.
func assertQuit(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	msg := execCmd(t, cmd)
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T (%v)", msg, msg)
	}
}

// viewContent returns the View's content string.
func viewContent(m app.Model) string {
	return m.View().Content
}

// TestSearchingScreen verifies that a new model starts in the searching
// state and shows "Searching…".
func TestSearchingScreen(t *testing.T) {
	m := app.New([]string{"--json", "--no-config", "--", "foo", "."}, "/work")
	if m.State() != app.StateSearching {
		t.Fatalf("State = %v, want StateSearching", m.State())
	}
	if !strings.Contains(viewContent(m), "Searching") {
		t.Fatalf("View = %q, want it to contain 'Searching'", viewContent(m))
	}
}

// TestGateHeldStaysSearching verifies that the model stays in the searching
// state when no SearchCompleteMsg has been received, even after receiving
// other messages like resize. This simulates the test gate holding index
// preparation independently of rg exit.
func TestGateHeldStaysSearching(t *testing.T) {
	m := app.New([]string{"--json", "--no-config", "--", "foo", "."}, "/work")

	// Simulate resize while searching (rg may have exited, but the gate
	// holds index preparation).
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	if m.State() != app.StateSearching {
		t.Fatalf("after resize, State = %v, want StateSearching (gate held)", m.State())
	}
	if !strings.Contains(viewContent(m), "Searching") {
		t.Fatalf("after resize, View = %q, want 'Searching'", viewContent(m))
	}

	// More messages while gate is still held.
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	if m.State() != app.StateSearching {
		t.Fatalf("after second resize, State = %v, want StateSearching (gate still held)", m.State())
	}
}

// TestCompletionToSummary verifies that a SearchCompleteMsg transitions
// the model from searching to the interim summary screen showing
// "N files, M matched lines".
func TestCompletionToSummary(t *testing.T) {
	m := app.New([]string{"--json", "--no-config", "--", "foo", "."}, "/work")

	m, _ = update(t, m, app.SearchCompleteMsg{Files: 3, Lines: 10})
	if m.State() != app.StateSummary {
		t.Fatalf("State = %v, want StateSummary", m.State())
	}
	view := viewContent(m)
	if !strings.Contains(view, "3") || !strings.Contains(view, "files") {
		t.Fatalf("View = %q, want it to contain '3 files'", view)
	}
	if !strings.Contains(view, "10") || !strings.Contains(view, "matched lines") {
		t.Fatalf("View = %q, want it to contain '10 matched lines'", view)
	}
}

// TestCompletionToSummaryZeroResults verifies the summary with zero files
// and zero lines.
func TestCompletionToSummaryZeroResults(t *testing.T) {
	m := app.New([]string{"--json", "--no-config", "--", "foo", "."}, "/work")
	m, _ = update(t, m, app.SearchCompleteMsg{Files: 0, Lines: 0})
	if m.State() != app.StateSummary {
		t.Fatalf("State = %v, want StateSummary", m.State())
	}
	view := viewContent(m)
	if !strings.Contains(view, "0") {
		t.Fatalf("View = %q, want it to contain '0'", view)
	}
}

// TestQExitsZeroFromSummary verifies that pressing 'q' from the summary
// screen produces a quit command and sets the exit code to 0.
func TestQExitsZeroFromSummary(t *testing.T) {
	m := app.New([]string{"--json", "--no-config", "--", "foo", "."}, "/work")
	m, _ = update(t, m, app.SearchCompleteMsg{Files: 2, Lines: 5})
	if m.State() != app.StateSummary {
		t.Fatalf("State = %v, want StateSummary", m.State())
	}

	m, cmd := update(t, m, keyPress('q'))
	assertQuit(t, cmd)
	if m.ExitCode() != 0 {
		t.Fatalf("ExitCode = %d, want 0", m.ExitCode())
	}
}

// TestQDuringSearchingExits130 verifies that pressing 'q' during the
// searching state exits with code 130 (cancellation). This is the Issue #4
// contract; Issue #3's model test verifies the state is still searching
// before the quit.
func TestQDuringSearchingExits130(t *testing.T) {
	m := app.New([]string{"--json", "--no-config", "--", "foo", "."}, "/work")
	if m.State() != app.StateSearching {
		t.Fatalf("State = %v, want StateSearching", m.State())
	}
	m, cmd := update(t, m, keyPress('q'))
	assertQuit(t, cmd)
	if m.ExitCode() != 130 {
		t.Fatalf("ExitCode = %d, want 130", m.ExitCode())
	}
}

// TestCtrlCExits130 verifies that ctrl+c in any state exits with code 130.
func TestCtrlCExits130(t *testing.T) {
	cases := []struct {
		name string
		msg  tea.Msg
	}{
		{"ctrl+c during searching", ctrlC()},
		{"ctrl+c during summary", ctrlC()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := app.New([]string{"--json", "--no-config", "--", "foo", "."}, "/work")
			if tc.name == "ctrl+c during summary" {
				m, _ = update(t, m, app.SearchCompleteMsg{Files: 1, Lines: 1})
			}
			m, cmd := update(t, m, tc.msg)
			assertQuit(t, cmd)
			if m.ExitCode() != 130 {
				t.Fatalf("ExitCode = %d, want 130", m.ExitCode())
			}
		})
	}
}

// TestResizeDuringSearch verifies that a resize message during searching
// updates the model's dimensions without blocking or transitioning state.
func TestResizeDuringSearch(t *testing.T) {
	m := app.New([]string{"--json", "--no-config", "--", "foo", "."}, "/work")

	// Resize should be handled immediately, no command needed.
	m, cmd := update(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	if m.State() != app.StateSearching {
		t.Fatalf("after resize, State = %v, want StateSearching", m.State())
	}
	if cmd != nil {
		// A resize during search may return a command, but it must not
		// block collection. The command should not be a quit.
		msg := execCmd(t, cmd)
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Fatal("resize during search produced a quit command")
		}
	}
	// Verify dimensions were updated by checking the view renders without
	// error at the new size.
	view := viewContent(m)
	if !strings.Contains(view, "Searching") {
		t.Fatalf("after resize, View = %q, want 'Searching'", view)
	}
}

// TestStartFailure verifies that a SearchFailedMsg transitions the model
// to the start-failed state with exit code 2, a diagnostic, and no TUI
// content.
func TestStartFailure(t *testing.T) {
	m := app.New([]string{"--json", "--no-config", "--", "foo", "."}, "/work")
	m, cmd := update(t, m, app.SearchFailedMsg{Diagnostic: "cannot start rg: not found"})

	if m.State() != app.StateStartFailed {
		t.Fatalf("State = %v, want StateStartFailed", m.State())
	}
	if m.ExitCode() != 2 {
		t.Fatalf("ExitCode = %d, want 2", m.ExitCode())
	}
	if m.Diagnostic() != "cannot start rg: not found" {
		t.Fatalf("Diagnostic = %q, want %q", m.Diagnostic(), "cannot start rg: not found")
	}
	assertQuit(t, cmd)
	// The view must not show TUI content (no "Searching…", no summary).
	view := viewContent(m)
	if strings.Contains(view, "Searching") {
		t.Fatalf("start-failed View shows 'Searching': %q", view)
	}
}

// TestStartFailureSanitizesDiagnostic verifies that the diagnostic stored
// by the model is sanitized (no raw control bytes).
func TestStartFailureSanitizesDiagnostic(t *testing.T) {
	m := app.New([]string{"--json", "--no-config", "--", "foo", "."}, "/work")
	m, _ = update(t, m, app.SearchFailedMsg{Diagnostic: "bad\x1b[31m diagnostic"})
	diag := m.Diagnostic()
	if strings.ContainsAny(diag, "\x1b\x9b") {
		t.Fatalf("Diagnostic contains raw control bytes: %q", diag)
	}
}

// TestGateOption verifies that the WithGate option is accepted by New
// without panicking.
func TestGateOption(t *testing.T) {
	gate := make(chan struct{})
	m := app.New([]string{"--json", "--no-config", "--", "foo", "."}, "/work", app.WithGate(gate))
	if m.State() != app.StateSearching {
		t.Fatalf("State = %v, want StateSearching", m.State())
	}
}

// TestInitReturnsCommand verifies that Init returns nil when no process
// is injected (test mode). The entry point injects a process for
// production; Init then returns the collection command.
func TestInitReturnsCommand(t *testing.T) {
	m := app.New([]string{"--json", "--no-config", "--", "foo", "."}, "/work")
	cmd := m.Init()
	if cmd != nil {
		t.Fatal("Init returned non-nil command without a process, want nil")
	}
}

// TestModelImplementsTeaModel verifies that Model satisfies the tea.Model
// interface.
func TestModelImplementsTeaModel(t *testing.T) {
	var _ tea.Model = app.Model{}
}

// TestSummaryViewDoesNotShowSearching verifies that the summary view
// does not contain "Searching" after completion.
func TestSummaryViewDoesNotShowSearching(t *testing.T) {
	m := app.New([]string{"--json", "--no-config", "--", "foo", "."}, "/work")
	m, _ = update(t, m, app.SearchCompleteMsg{Files: 1, Lines: 1})
	view := viewContent(m)
	if strings.Contains(view, "Searching") {
		t.Fatalf("summary View contains 'Searching': %q", view)
	}
}

// TestMultipleResizesDuringSearch verifies that multiple resize messages
// are handled without blocking.
func TestMultipleResizesDuringSearch(t *testing.T) {
	m := app.New([]string{"--json", "--no-config", "--", "foo", "."}, "/work")
	sizes := []tea.WindowSizeMsg{
		{Width: 80, Height: 24},
		{Width: 20, Height: 3},
		{Width: 200, Height: 50},
		{Width: 1, Height: 1},
		{Width: 120, Height: 40},
	}
	for _, ws := range sizes {
		m, _ = update(t, m, ws)
		if m.State() != app.StateSearching {
			t.Fatalf("after resize to %dx%d, State = %v, want StateSearching", ws.Width, ws.Height, m.State())
		}
	}
}

// TestKeysIgnoredDuringSearch verifies that non-quit keys during searching
// do not transition the state or quit.
func TestKeysIgnoredDuringSearch(t *testing.T) {
	m := app.New([]string{"--json", "--no-config", "--", "foo", "."}, "/work")
	for _, k := range []tea.KeyPressMsg{
		keyPress('n'),
		keyPress('p'),
		keyPress('h'),
		keyPress('?'),
		keyPress('r'),
		keyPress('w'),
	} {
		m, cmd := update(t, m, k)
		if m.State() != app.StateSearching {
			t.Fatalf("after key %q, State = %v, want StateSearching", k.Text, m.State())
		}
		if cmd != nil {
			msg := execCmd(t, cmd)
			if _, ok := msg.(tea.QuitMsg); ok {
				t.Fatalf("key %q during search produced a quit command", k.Text)
			}
		}
	}
}

// TestEscIsNoOpDuringSearch verifies that Esc during searching is a no-op
// (per the PRD: Esc never exits from a base state).
func TestEscIsNoOpDuringSearch(t *testing.T) {
	m := app.New([]string{"--json", "--no-config", "--", "foo", "."}, "/work")
	m, cmd := update(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.State() != app.StateSearching {
		t.Fatalf("after Esc, State = %v, want StateSearching", m.State())
	}
	if cmd != nil {
		msg := execCmd(t, cmd)
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Fatal("Esc during search produced a quit command")
		}
	}
}

// TestEscIsNoOpDuringSummary verifies that Esc during summary is a no-op.
func TestEscIsNoOpDuringSummary(t *testing.T) {
	m := app.New([]string{"--json", "--no-config", "--", "foo", "."}, "/work")
	m, _ = update(t, m, app.SearchCompleteMsg{Files: 1, Lines: 1})
	m, cmd := update(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.State() != app.StateSummary {
		t.Fatalf("after Esc, State = %v, want StateSummary", m.State())
	}
	if cmd != nil {
		msg := execCmd(t, cmd)
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Fatal("Esc during summary produced a quit command")
		}
	}
}

// TestLateCompletionAfterCancellationIgnored verifies that a late
// SearchCompleteMsg arriving after cancellation (q while searching) does
// not revive the UI by transitioning to the summary state. This is the
// Issue #4 late-completion rejection contract.
func TestLateCompletionAfterCancellationIgnored(t *testing.T) {
	m := app.New([]string{"--json", "--no-config", "--", "foo", "."}, "/work")
	if m.State() != app.StateSearching {
		t.Fatalf("State = %v, want StateSearching", m.State())
	}

	// Cancel via q while searching.
	m, cmd := update(t, m, keyPress('q'))
	assertQuit(t, cmd)
	if m.ExitCode() != 130 {
		t.Fatalf("ExitCode = %d, want 130", m.ExitCode())
	}

	// A late SearchCompleteMsg must not transition to summary or
	// otherwise revive the cancelled UI.
	m2, lateCmd := update(t, m, app.SearchCompleteMsg{Files: 3, Lines: 10})
	if m2.State() == app.StateSummary {
		t.Fatal("late SearchCompleteMsg revived the cancelled UI (transitioned to summary)")
	}
	if m2.ExitCode() != 130 {
		t.Fatalf("after late completion, ExitCode = %d, want 130", m2.ExitCode())
	}
	if lateCmd != nil {
		msg := execCmd(t, lateCmd)
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Fatal("late SearchCompleteMsg after cancellation produced a quit command")
		}
	}
}

// TestLateCompletionAfterCtrlCIgnored verifies that a late
// SearchCompleteMsg arriving after ctrl+c cancellation does not revive
// the UI.
func TestLateCompletionAfterCtrlCIgnored(t *testing.T) {
	m := app.New([]string{"--json", "--no-config", "--", "foo", "."}, "/work")
	m, cmd := update(t, m, ctrlC())
	assertQuit(t, cmd)
	if m.ExitCode() != 130 {
		t.Fatalf("ExitCode = %d, want 130", m.ExitCode())
	}

	m2, _ := update(t, m, app.SearchCompleteMsg{Files: 3, Lines: 10})
	if m2.State() == app.StateSummary {
		t.Fatal("late SearchCompleteMsg revived the ctrl+c cancelled UI")
	}
}

// TestQDuringGateHeldExits130 verifies that pressing q while the model is
// in the searching state (representing gate-held index preparation after
// rg has exited) is cancellation with exit 130, not a browse quit. This
// is the Issue #4 post-rg-exit/preparation-window contract.
func TestQDuringGateHeldExits130(t *testing.T) {
	gate := make(chan struct{})
	m := app.New(
		[]string{"--json", "--no-config", "--", "foo", "."},
		"/work",
		app.WithGate(gate),
	)
	// The gate is held (never closed). The model stays in searching
	// even though rg would have exited by this point in a real run.
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	if m.State() != app.StateSearching {
		t.Fatalf("State = %v, want StateSearching (gate held)", m.State())
	}

	m, cmd := update(t, m, keyPress('q'))
	assertQuit(t, cmd)
	if m.ExitCode() != 130 {
		t.Fatalf("ExitCode = %d, want 130 (cancellation, not browse quit)", m.ExitCode())
	}
}

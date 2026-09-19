package app

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// fakeChild is a started-child stand-in whose Wait returns a fixed
// collected result, as if rg had already exited and been drained.
type fakeChild struct{ res Result }

func (f fakeChild) Wait() Result { return f.res }

func (f fakeChild) Terminate() {}

// happyStream is a small valid ripgrep JSON stream: two files, three
// matched lines.
const happyStream = `{"type":"begin","data":{"path":{"text":"./a.go"}}}
{"type":"match","data":{"path":{"text":"./a.go"},"lines":{"text":"alpha\n"},"line_number":2,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
{"type":"match","data":{"path":{"text":"./a.go"},"lines":{"text":"alpha again\n"},"line_number":5,"absolute_offset":20,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
{"type":"end","data":{"path":{"text":"./a.go"},"binary_offset":null,"stats":{}}}
{"type":"begin","data":{"path":{"text":"./b.go"}}}
{"type":"match","data":{"path":{"text":"./b.go"},"lines":{"text":"beta alpha\n"},"line_number":1,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":5,"end":10}]}}
{"type":"end","data":{"path":{"text":"./b.go"},"binary_offset":null,"stats":{}}}
{"type":"summary","data":{"elapsed_total":{},"stats":{}}}
`

func newTestModel(child Child, opts options) *model {
	return newModel(Config{Dir: "/wd"}, opts, child)
}

func runCollectCmd(t *testing.T, m *model) tea.Msg {
	t.Helper()
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init returned nil command; collection must start immediately")
	}
	return cmd()
}

func viewText(m *model) string { return m.View().Content }

// The model opens on the searching screen: collection is underway and the
// interim summary has not yet arrived.
func TestSearchingScreenShownDuringCollection(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Stdout: []byte(happyStream)}}, options{})
	if m.state != stateSearching {
		t.Fatalf("initial state = %v, want searching", m.state)
	}
	if v := viewText(m); !strings.Contains(v, "Searching") {
		t.Fatalf("searching view = %q, want it to say Searching", v)
	}
}

// A collected stream becomes the interim summary once the completion
// message lands.
func TestCompletionTransitionsToSummary(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Stdout: []byte(happyStream)}}, options{})
	m.Update(runCollectCmd(t, m))
	if m.state != stateSummary {
		t.Fatalf("state = %v, want summary after completion", m.state)
	}
	if v := viewText(m); !strings.Contains(v, "2 files, 3 matched lines") {
		t.Fatalf("summary view = %q, want %q", v, "2 files, 3 matched lines")
	}
}

// q on the interim summary quits with the fixed successful status.
func TestQOnSummaryExitsZero(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Stdout: []byte(happyStream)}}, options{})
	m.Update(runCollectCmd(t, m))
	_, cmd := m.Update(tea.KeyPressMsg{Text: "q", Code: 'q'})
	if cmd == nil {
		t.Fatal("q on summary returned nil command, want tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("q command produced %T, want tea.QuitMsg", cmd())
	}
	if m.status != 0 {
		t.Fatalf("status = %d, want 0", m.status)
	}
}

// Resize messages are handled without blocking on collection.
func TestResizeDuringSearching(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Stdout: []byte(happyStream)}}, options{})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if m.width != 100 || m.height != 30 {
		t.Fatalf("size = %dx%d, want 100x30", m.width, m.height)
	}
	if m.state != stateSearching {
		t.Fatalf("state = %v, want still searching after resize", m.state)
	}
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 10})
	if m.width != 40 || m.height != 10 {
		t.Fatalf("size = %dx%d, want 40x10", m.width, m.height)
	}
	if v := viewText(m); !strings.Contains(v, "Searching") {
		t.Fatalf("view = %q, want it to keep saying Searching", v)
	}
}

// The test gate can hold index preparation after rg has exited and its
// stream is fully collected; the app stays on the searching screen until
// preparation is allowed to finish.
func TestGateHeldPreparationStaysSearching(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	opts := options{gate: func() {
		close(entered)
		<-release
	}}
	m := newTestModel(fakeChild{res: Result{Stdout: []byte(happyStream)}}, opts)

	done := make(chan tea.Msg, 1)
	go func() { done <- runCollectCmd(t, m) }()

	// Wait returns: the child "exited" and the stream is collected. The
	// gate then holds preparation; reaching it proves both.
	<-entered
	if m.state != stateSearching {
		t.Fatalf("state = %v while preparation held, want searching", m.state)
	}
	if v := viewText(m); !strings.Contains(v, "Searching") || strings.Contains(v, "matched lines") {
		t.Fatalf("held view = %q, want Searching without a summary", v)
	}
	// The model still answers input while preparation is held.
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	select {
	case msg := <-done:
		t.Fatalf("completion arrived while the gate was held: %#v", msg)
	default:
	}

	close(release)
	msg := <-done
	m.Update(msg)
	if m.state != stateSummary {
		t.Fatalf("state = %v after gate release, want summary", m.state)
	}
	if v := viewText(m); !strings.Contains(v, "2 files, 3 matched lines") {
		t.Fatalf("summary view = %q, want %q", v, "2 files, 3 matched lines")
	}
}

// A start failure maps to the sanitized stderr diagnostic and exit 2 —
// before the TUI is ever entered.
func TestRunStartFailureExit2(t *testing.T) {
	var stderr bytes.Buffer
	cfg := Config{
		Args: []string{"--json", "--no-config", "--", "foo", "."},
		Dir:  t.TempDir(),
		Err:  &stderr,
		Start: func(ctx context.Context, args []string, dir string) (Child, error) {
			return nil, errors.New("exec: \"rg\": executable file not found in $PATH")
		},
	}
	code := Run(t.Context(), cfg)
	if code != 2 {
		t.Fatalf("Run = %d, want exit 2", code)
	}
	diag := stderr.String()
	if !strings.Contains(diag, "vrg:") || !strings.Contains(diag, "rg") {
		t.Fatalf("diagnostic = %q, want a vrg diagnostic naming the start failure", diag)
	}
	if strings.ContainsAny(diag, "\x1b\x9b") {
		t.Fatalf("diagnostic contains raw control bytes: %q", diag)
	}
}

// Hostile bytes in a start error are escaped in the diagnostic.
func TestRunStartFailureSanitizesError(t *testing.T) {
	var stderr bytes.Buffer
	cfg := Config{
		Args: []string{"--json", "--no-config", "--", "foo", "."},
		Dir:  t.TempDir(),
		Err:  &stderr,
		Start: func(ctx context.Context, args []string, dir string) (Child, error) {
			return nil, errors.New("spawn \x1b[31m failed\nsecond line")
		},
	}
	if code := Run(t.Context(), cfg); code != 2 {
		t.Fatalf("Run = %d, want exit 2", code)
	}
	diag := stderr.String()
	if strings.ContainsAny(diag, "\x1b\x9b") {
		t.Fatalf("raw control bytes reached the diagnostic: %q", diag)
	}
	if strings.Count(strings.TrimRight(diag, "\n"), "\n") != 0 {
		t.Fatalf("diagnostic spans multiple lines: %q", diag)
	}
}

package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// replayed runs the replay writer — the sink every controlled exit
// funnels through — and returns the lines the model's session
// collection produced.
func replayed(t *testing.T, m Model) []string {
	t.Helper()
	var buf bytes.Buffer
	m.ReplayTo(&buf)
	s := strings.TrimRight(buf.String(), "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// driveEvent pumps one collector event through the model the way the
// real program does — Init's command and the re-issue each diagnostic
// returns — updating the model with whatever arrives. A silent channel
// past the budget is a deadlock and fails rather than hangs.
func driveEvent(t *testing.T, m Model) Model {
	t.Helper()
	ch := make(chan tea.Msg, 1)
	go func() { ch <- m.awaitEvent() }()
	select {
	case msg := <-ch:
		m2, _ := update(t, m, msg)
		return m2
	case <-time.After(10 * time.Second):
		t.Fatal("no collector event within budget; suspected deadlock")
		return m
	}
}

// Three collected diagnostics — the search's stderr warning that the
// overlay displays plus two load failures that are never displayed —
// reach the replay writer exactly once each, in collection order, on a
// normal exit.
func TestReplayCollectsDisplayedAndUndisplayedInOrder(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "f.txt", "hit\n")
	m := newModel(nil, nil)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	// rg's stderr warning streams in and is collected while the search
	// runs; the completion carries the same bytes for display.
	m, _ = update(t, m, diagMsg{line: "rg: warn one"})
	m, _ = update(t, m, searchDoneMsg{
		index:  fixtureIndex(t, dir, recsOneMatch...),
		stderr: []byte("rg: warn one\n"),
	})
	if m.overlay == nil {
		t.Fatal("the stderr warning did not open the overlay")
	}
	if v := m.View().Content; !strings.Contains(v, "rg: warn one") {
		t.Fatalf("overlay lacks the warning diagnostic: %q", v)
	}
	// Failures on non-current files are collected but never displayed:
	// no overlay, no in-UI indicator. Their loads are in flight — a
	// completion is dropped unless it carries the in-flight request's
	// identity.
	reqA := mintLoad(&m, "late-a.txt")
	m, _ = update(t, m, loadDoneMsg{
		path: []byte("late-a.txt"), req: reqA, err: errors.New("denied")})
	reqB := mintLoad(&m, "late-b.txt")
	m, _ = update(t, m, loadDoneMsg{
		path: []byte("late-b.txt"), req: reqB, err: errors.New("vanished")})

	m, _ = pressKey(t, m, "q") // dismiss the overlay, revealing browse
	m, cmd := pressKey(t, m, "q")
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("q on browse command = %T, want tea.QuitMsg", cmd())
	}

	got := replayed(t, m)
	want := []string{
		"rg: warn one",
		"cannot read late-a.txt: denied",
		"cannot read late-b.txt: vanished",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("replayed diagnostics = %q, want %q", got, want)
	}
}

// The shutdown boundary for ctrl+c: a diagnostic delivered and
// processed before the keypress is replayed; the diagnostic that only
// exists inside the gate-held completion is never delivered, is not
// waited for, and is not replayed.
func TestShutdownBoundaryCtrlC(t *testing.T) {
	dir := t.TempDir()
	// A fake rg that emits one stderr line and a stream with no
	// summary: the stream's "missing summary" diagnostic exists only
	// inside the completion message the preparation gate holds back.
	rg := filepath.Join(dir, "rg")
	script := "#!/bin/sh\n" +
		"echo 'rg: warn one' >&2\n" +
		"printf '%s\\n' " +
		`'{"type":"begin","data":{"path":{"text":"f.txt"}}}' ` +
		`'{"type":"match","data":{"path":{"text":"f.txt"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' ` +
		`'{"type":"end","data":{"path":{"text":"f.txt"},"binary_offset":null}}'` + "\n" +
		"exit 0\n"
	if err := os.WriteFile(rg, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	gate := make(chan struct{})
	sess, err := Start(context.Background(), Config{
		Rg:          rg,
		Argv:        []string{"--json", "--no-config", "--", "foo", "."},
		Workdir:     dir,
		PrepareGate: gate,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	m := sess.Model()

	// The streamed diagnostic is collected while the completion sits
	// behind the gate — processing order is collection order.
	m = driveEvent(t, m)
	m2, cmd := update(t, m, ctrlCPress())
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("ctrl+c command = %T, want tea.QuitMsg", cmd())
	}
	if m2.ExitCode() != 130 {
		t.Fatalf("ExitCode = %d, want 130", m2.ExitCode())
	}

	got := replayed(t, m2)
	if !slices.Equal(got, []string{"rg: warn one"}) {
		t.Fatalf("replayed diagnostics = %q, want only the collected warning", got)
	}
	for _, d := range got {
		if strings.Contains(d, "missing summary") {
			t.Fatalf("gated diagnostic was replayed: %q", got)
		}
	}

	// Cancellation does not wait on the gate: the collector ends
	// promptly and delivers its abandoned result.
	select {
	case done := <-m2.done:
		if done.index != nil {
			t.Fatal("cancelled collection still prepared an index")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("collection did not end promptly after cancellation")
	}
}

// The same boundary for the q cancellation route, in both incomplete
// states: while rg still runs, and after rg's exit while gate-held
// result preparation keeps the app searching. The acknowledged
// diagnostic is replayed with exit 130; undelivered work is not waited
// on.
func TestShutdownBoundaryQ(t *testing.T) {
	// rg emits the diagnostic then runs forever: cancellation while
	// searching with the child still alive.
	t.Run("searching", func(t *testing.T) {
		dir := t.TempDir()
		rg := filepath.Join(dir, "rg")
		script := "#!/bin/sh\n" +
			"echo 'rg: warn one' >&2\n" +
			"exec sleep 3600\n"
		if err := os.WriteFile(rg, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		sess, err := Start(context.Background(), Config{
			Rg:      rg,
			Argv:    []string{"--json", "--no-config", "--", "foo", "."},
			Workdir: dir,
		})
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		m := sess.Model()
		m = driveEvent(t, m) // collect the streamed diagnostic
		m2, cmd := update(t, m, keyPress("q"))
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatalf("q while searching command = %T, want tea.QuitMsg", cmd())
		}
		if m2.ExitCode() != 130 {
			t.Fatalf("ExitCode = %d, want 130", m2.ExitCode())
		}
		if got := replayed(t, m2); !slices.Equal(got, []string{"rg: warn one"}) {
			t.Fatalf("replayed diagnostics = %q, want only the collected warning", got)
		}
		select {
		case <-sess.Reaped():
		case <-time.After(10 * time.Second):
			t.Fatal("child was not reaped within budget after q")
		}
	})

	// rg has already exited but gate-held result preparation keeps the
	// app searching: q is still cancellation, and the completion's
	// diagnostics are never delivered.
	t.Run("gate-held preparation", func(t *testing.T) {
		dir := t.TempDir()
		rg := filepath.Join(dir, "rg")
		script := "#!/bin/sh\n" +
			"echo 'rg: warn one' >&2\n" +
			"printf '%s\\n' " +
			`'{"type":"begin","data":{"path":{"text":"f.txt"}}}' ` +
			`'{"type":"match","data":{"path":{"text":"f.txt"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' ` +
			`'{"type":"end","data":{"path":{"text":"f.txt"},"binary_offset":null}}'` + "\n" +
			"exit 0\n"
		if err := os.WriteFile(rg, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		gate := make(chan struct{})
		sess, err := Start(context.Background(), Config{
			Rg:          rg,
			Argv:        []string{"--json", "--no-config", "--", "foo", "."},
			Workdir:     dir,
			PrepareGate: gate,
		})
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		m := sess.Model()
		m = driveEvent(t, m)
		m2, cmd := update(t, m, keyPress("q"))
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatalf("q during gate-held preparation command = %T, want tea.QuitMsg", cmd())
		}
		if m2.ExitCode() != 130 {
			t.Fatalf("ExitCode = %d, want 130", m2.ExitCode())
		}
		if got := replayed(t, m2); !slices.Equal(got, []string{"rg: warn one"}) {
			t.Fatalf("replayed diagnostics = %q, want only the collected warning", got)
		}
		select {
		case done := <-m2.done:
			if done.index != nil {
				t.Fatal("cancelled collection still prepared an index")
			}
		case <-time.After(10 * time.Second):
			t.Fatal("collection did not end promptly after cancellation")
		}
	})
}

// The controlled-failure diagnostic enters the session collection
// before shutdown and is replayed by the common post-restoration
// writer alongside the earlier diagnostics — once each, in collection
// order, with no separate direct write.
func TestControlledFailureJoinsSessionCollection(t *testing.T) {
	m := newModel(nil, nil)
	m, _ = update(t, m, diagMsg{line: "rg: warn one"})
	// The process boundary's route: the failure diagnostic joins the
	// collection rather than writing to stderr itself.
	m.CollectDiagnostic("vrg: controlled failure")
	got := replayed(t, m)
	want := []string{"rg: warn one", "vrg: controlled failure"}
	if !slices.Equal(got, want) {
		t.Fatalf("replayed diagnostics = %q, want %q", got, want)
	}
}

// A diagnostic embedding a filename with a newline and an ESC escapes
// the filename through the single-line path utility: the replayed text
// stays one line and carries no raw control bytes.
func TestReplayEscapesEmbeddedFilename(t *testing.T) {
	m := newModel(nil, nil)
	req := mintLoad(&m, "f\no\x1b.txt")
	m, _ = update(t, m, loadDoneMsg{
		path: []byte("f\no\x1b.txt"),
		req:  req,
		err:  errors.New("denied"),
	})
	got := replayed(t, m)
	want := []string{`cannot read f\no^[.txt: denied`}
	if !slices.Equal(got, want) {
		t.Fatalf("replayed diagnostics = %q, want %q", got, want)
	}
	var raw bytes.Buffer
	m.ReplayTo(&raw)
	for i := 0; i < raw.Len(); i++ {
		if c := raw.Bytes()[i]; c != '\n' && (c < 0x20 || c == 0x7f) {
			t.Fatalf("replayed diagnostic carries raw control byte 0x%02x: %q", c, raw.String())
		}
	}
}

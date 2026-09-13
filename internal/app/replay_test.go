package app_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
	"vrg/internal/sinkfixtures"
)

// completeBrowseIndex builds a small complete index with one matched file,
// for tests that need a browse state with an overlay.
func completeBrowseIndex(t *testing.T) *searchindex.Index {
	t.Helper()
	return buildIndex(t, "/work",
		textBegin("a.go"),
		textMatch("a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
		endRecord("a.go", nil),
		summaryRecord(),
	)
}

// browseModelWithLoader returns a model configured with a file loader and
// released file-load gate, ready to receive a SearchCompleteMsg.
func browseModelWithLoader() app.Model {
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return makeBuf(nil, 0, 3), nil
	}
	return app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
	)
}

// assertDiagnosticsEq requires m.Diagnostics() to equal want exactly.
func assertDiagnosticsEq(t *testing.T, m app.Model, want []string) {
	t.Helper()
	got := m.Diagnostics()
	if len(got) != len(want) {
		t.Fatalf("Diagnostics() = %q (len %d), want %q (len %d)", got, len(got), want, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Diagnostics()[%d] = %q, want %q (full: %q)", i, got[i], want[i], got)
		}
	}
}

// TestReplayCollectsDisplayedAndNeverDisplayedDiagnostics verifies that
// the session diagnostic collection is independent of what was displayed:
// one diagnostic displayed in an overlay and two never displayed all
// reach the collection exactly once each, in collection order, on a
// normal exit (Issue #11, AC1–AC2).
func TestReplayCollectsDisplayedAndNeverDisplayedDiagnostics(t *testing.T) {
	idx := completeBrowseIndex(t)
	m := browseModelWithLoader()

	// 1. SearchCompleteMsg with stderr → warning overlay (displayed).
	m, cmd := update(t, m, app.SearchCompleteMsg{
		Files: idx.Files(), Lines: idx.Len(), Index: idx,
		Process: app.ProcessResult{ExitCode: 1},
		Stderr:  "displayed warning",
	})
	if cmd != nil {
		execCmd(t, cmd)
	}
	if !m.OverlayOpen() {
		t.Fatalf("overlay not open after SearchCompleteMsg with stderr")
	}

	// 2. Two DiagnosticMsg entries (never displayed).
	m, _ = update(t, m, app.DiagnosticMsg{Diagnostic: "hidden one"})
	m, _ = update(t, m, app.DiagnosticMsg{Diagnostic: "hidden two"})

	// 3. Dismiss the overlay, then quit normally from browse.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	if m.OverlayOpen() {
		t.Fatalf("overlay still open after Esc dismiss")
	}
	m, cmd = update(t, m, keyPress('q'))
	assertQuit(t, cmd)

	// The collection must contain all three, in collection order, once each.
	assertDiagnosticsEq(t, m, []string{"displayed warning", "hidden one", "hidden two"})
}

// TestReplayNeverDisplayedDiagnosticCollected verifies that a diagnostic
// never shown in an overlay is still collected (Issue #11, AC2).
func TestReplayNeverDisplayedDiagnosticCollected(t *testing.T) {
	m := app.New([]string{"--json", "--", "foo", "."}, "/work")
	m, _ = update(t, m, app.DiagnosticMsg{Diagnostic: "never shown"})
	assertDiagnosticsEq(t, m, []string{"never shown"})
}

// TestReplayCtrlCAfterProcessedDiagnosticReplayed verifies the shutdown
// boundary for ctrl+c: a diagnostic delivered and processed before ctrl+c
// in the next update is replayed (Issue #11, AC3).
func TestReplayCtrlCAfterProcessedDiagnosticReplayed(t *testing.T) {
	m := app.New([]string{"--json", "--", "foo", "."}, "/work")
	// Process the diagnostic first.
	m, _ = update(t, m, app.DiagnosticMsg{Diagnostic: "before cancel"})
	// Then ctrl+c in the next update.
	m, cmd := update(t, m, ctrlC())
	assertQuit(t, cmd)
	if m.ExitCode() != 130 {
		t.Fatalf("ExitCode = %d, want 130", m.ExitCode())
	}
	assertDiagnosticsEq(t, m, []string{"before cancel"})
}

// TestReplayCtrlCGatedDiagnosticNotCollected verifies the shutdown
// boundary for ctrl+c: a gated, undelivered diagnostic is not waited for
// and not replayed (Issue #11, AC3).
func TestReplayCtrlCGatedDiagnosticNotCollected(t *testing.T) {
	gate := make(chan struct{}) // held open
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithGate(gate),
	)
	// The SearchCompleteMsg (carrying stderr) is gated/undelivered.
	// Press ctrl+c without delivering it.
	m, cmd := update(t, m, ctrlC())
	assertQuit(t, cmd)
	if m.ExitCode() != 130 {
		t.Fatalf("ExitCode = %d, want 130", m.ExitCode())
	}
	if diags := m.Diagnostics(); len(diags) != 0 {
		t.Fatalf("Diagnostics() = %q, want empty (gated diagnostic not collected)", diags)
	}
}

// TestReplayQAfterProcessedDiagnosticWhileSearching verifies the shutdown
// boundary for q: a diagnostic acknowledged as collected before q arrives
// while searching is still incomplete is replayed with exit 130 (Issue
// #11, AC3).
func TestReplayQAfterProcessedDiagnosticWhileSearching(t *testing.T) {
	gate := make(chan struct{}) // held open: searching/preparation incomplete
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithGate(gate),
	)
	// Process the diagnostic while searching.
	m, _ = update(t, m, app.DiagnosticMsg{Diagnostic: "collected before q"})
	if m.State() != app.StateSearching {
		t.Fatalf("State = %v, want StateSearching (gate held)", m.State())
	}
	// q while searching → cancellation, exit 130.
	m, cmd := update(t, m, keyPress('q'))
	assertQuit(t, cmd)
	if m.ExitCode() != 130 {
		t.Fatalf("ExitCode = %d, want 130", m.ExitCode())
	}
	assertDiagnosticsEq(t, m, []string{"collected before q"})
}

// TestReplayQGatedDiagnosticNotCollected verifies the shutdown boundary
// for q: an undelivered (gated) diagnostic is not waited for and not
// replayed (Issue #11, AC3).
func TestReplayQGatedDiagnosticNotCollected(t *testing.T) {
	gate := make(chan struct{}) // held open
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithGate(gate),
	)
	// The SearchCompleteMsg (carrying stderr) is gated/undelivered.
	// Press q without delivering it.
	m, cmd := update(t, m, keyPress('q'))
	assertQuit(t, cmd)
	if m.ExitCode() != 130 {
		t.Fatalf("ExitCode = %d, want 130", m.ExitCode())
	}
	if diags := m.Diagnostics(); len(diags) != 0 {
		t.Fatalf("Diagnostics() = %q, want empty (gated diagnostic not collected)", diags)
	}
}

// TestReplayControlledFailureCollectedExactlyOnce verifies that the
// Issue #4 controlled-failure diagnostic enters the session collection
// before shutdown and is replayed by the common post-restoration writer
// with no separate direct write, so exactly-once holds across both
// mechanisms (Issue #11, AC4).
func TestReplayControlledFailureCollectedExactlyOnce(t *testing.T) {
	m := app.New([]string{"--json", "--", "foo", "."}, "/work")
	m, cmd := update(t, m, app.ControlledFailureMsg{Diagnostic: "controlled failure"})
	assertQuit(t, cmd)
	if m.ExitCode() != 2 {
		t.Fatalf("ExitCode = %d, want 2", m.ExitCode())
	}
	// The collection must contain exactly one entry — the controlled-
	// failure diagnostic. There is no separate direct write; the entry
	// point replays from the collection.
	assertDiagnosticsEq(t, m, []string{"controlled failure"})
}

// TestReplayControlledFailureWithEarlierDiagnostic verifies that the
// controlled-failure diagnostic appears exactly once alongside earlier
// diagnostics in collection order (Issue #11, AC4).
func TestReplayControlledFailureWithEarlierDiagnostic(t *testing.T) {
	m := app.New([]string{"--json", "--", "foo", "."}, "/work")
	m, _ = update(t, m, app.DiagnosticMsg{Diagnostic: "earlier"})
	m, cmd := update(t, m, app.ControlledFailureMsg{Diagnostic: "controlled failure"})
	assertQuit(t, cmd)
	assertDiagnosticsEq(t, m, []string{"earlier", "controlled failure"})
}

// TestReplayEscapesFilenameInDiagnostic verifies that a diagnostic
// embedding a filename with \n and ESC is escaped and single-lined
// through the Issue #6 utility (Issue #11, AC5).
func TestReplayEscapesFilenameInDiagnostic(t *testing.T) {
	rawFilename := "file\x1bname\nwith\nnewline"
	escapedName := app.EscapePathForDiagnostic(rawFilename)
	diag := "oversized record skipped for " + escapedName
	m := app.New([]string{"--json", "--", "foo", "."}, "/work")
	m, _ = update(t, m, app.DiagnosticMsg{Diagnostic: diag})
	collected := m.Diagnostics()
	if len(collected) != 1 {
		t.Fatalf("Diagnostics() = %q (len %d), want 1 entry", collected, len(collected))
	}
	got := collected[0]
	if strings.Contains(got, "\n") {
		t.Fatalf("collected diagnostic contains a real newline: %q", got)
	}
	if !strings.Contains(got, `\n`) {
		t.Fatalf("collected diagnostic does not contain escaped \\n: %q", got)
	}
	if strings.ContainsAny(got, "\x1b\x9b") {
		t.Fatalf("collected diagnostic contains raw ESC: %q", got)
	}
}

// TestReplaySinkSafetyTable verifies that the replay writer (the
// Diagnostics() collection) passes the Issue #6 sink-safety table: no
// raw control bytes survive in any collected diagnostic for every shared
// hostile fixture (Issue #11, AC5 — adding the replay writer to the
// sink-safety table).
func TestReplaySinkSafetyTable(t *testing.T) {
	for _, fx := range sinkfixtures.Fixtures {
		t.Run(fx.Name, func(t *testing.T) {
			m := app.New([]string{"--json", "--", "foo", "."}, "/work")
			m, _ = update(t, m, app.DiagnosticMsg{Diagnostic: string(fx.Raw)})
			collected := m.Diagnostics()
			if len(collected) != 1 {
				t.Fatalf("Diagnostics() = %q (len %d), want 1 entry", collected, len(collected))
			}
			if !sinkfixtures.NoControlBytes(collected[0]) {
				t.Fatalf("raw control byte in collected diagnostic for %s: %q", fx.Name, collected[0])
			}
		})
	}
}

// TestReplayOnCollectAcknowledgement verifies that the onCollect callback
// fires when a diagnostic is collected, providing the application-side
// acknowledgement side channel (Issue #11, Task 3/4 support).
func TestReplayOnCollectAcknowledgement(t *testing.T) {
	var collected []string
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithOnCollect(func(diag string) {
			collected = append(collected, diag)
		}),
	)
	m, _ = update(t, m, app.DiagnosticMsg{Diagnostic: "ack test"})
	if len(collected) != 1 {
		t.Fatalf("onCollect called %d times, want 1", len(collected))
	}
	if collected[0] != "ack test" {
		t.Fatalf("onCollect received %q, want %q", collected[0], "ack test")
	}
}

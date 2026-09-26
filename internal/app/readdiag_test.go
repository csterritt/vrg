package app

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"vrg/internal/present"
)

// hostileReadNames is the Issue #47 embedded-byte filename set: names
// legal on a Unix filesystem but hostile inside a diagnostic line —
// an embedded newline that PathError.Error() would turn into a
// diagnostic boundary, a tab, an invalid UTF-8 byte, and an ESC
// introducer. Each is created as a real file so the load failure is a
// genuine os.ReadFile *os.PathError, not an injected loader's
// fabrication.
var hostileReadNames = []struct {
	name string
	raw  string
}{
	{"newline", "z\nname.txt"},
	{"tab", "z\tname.txt"},
	{"invalid-utf8", "z\xffname.txt"},
	{"escape", "z\x1bname.txt"},
}

// pathRecs builds a hostile file's begin/match/end records. The path
// travels in the base64 "bytes" form uniformly — JSON strings cannot
// carry several of the fixture names — while the matched line and
// submatch reuse fileWithStops' one-match shape.
func pathRecs(path []byte) []string {
	p := fmt.Sprintf(`{"bytes":"%s"}`, base64.StdEncoding.EncodeToString(path))
	return []string{
		`{"type":"begin","data":{"path":` + p + `}}`,
		`{"type":"match","data":{"path":` + p + `,"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"end","data":{"path":` + p + `,"binary_offset":null}}`,
	}
}

// gatedReadFailure runs one load worker against the real read phase:
// the gate parks it ahead of its os.ReadFile, the fixture's removal
// makes the coming read a genuine missing-file failure — a chmod
// denial could pass under elevated privileges — and the gate's
// release lets the worker complete. The remove-then-release order is
// deterministic: the read cannot slip in before the removal because
// the worker cannot pass the gate.
func gatedReadFailure(t *testing.T, gate chan struct{}, cmd tea.Cmd, victim string) tea.Msg {
	t.Helper()
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case <-done:
		t.Fatal("the gated load completed before its gate released")
	case <-time.After(50 * time.Millisecond):
	}
	if err := os.Remove(victim); err != nil {
		t.Fatalf("remove %q: %v", present.Path([]byte(victim)), err)
	}
	close(gate)
	select {
	case msg := <-done:
		return msg
	case <-time.After(10 * time.Second):
		t.Fatal("the released load never completed")
		return nil
	}
}

// wantReadPathError requires msg to be path's failed loadDoneMsg whose
// error is the real read's *os.PathError — the read genuinely
// attempted resolved and met the missing-file errno — and returns the
// sanitized reason text the diagnostic must carry: the error unwrapped
// to its bare Err rather than PathError.Error(), which embeds the raw
// resolved path.
func wantReadPathError(t *testing.T, msg tea.Msg, path, resolved string) string {
	t.Helper()
	ld, ok := msg.(loadDoneMsg)
	if !ok {
		t.Fatalf("load message = %T %v, want loadDoneMsg", msg, msg)
	}
	if string(ld.path) != path {
		t.Fatalf("load completed for %q, want %q", ld.path, path)
	}
	var pe *os.PathError
	if !errors.As(ld.err, &pe) {
		t.Fatalf("load error = %T %v, want *os.PathError", ld.err, ld.err)
	}
	if !errors.Is(ld.err, os.ErrNotExist) {
		t.Fatalf("load error = %v, want the missing-file errno", ld.err)
	}
	if pe.Path != resolved {
		t.Fatalf("read attempted %q, want the indexed file %q", pe.Path, resolved)
	}
	return pe.Err.Error()
}

// wantSingleLineDiagnostic asserts one read failure surfaced as
// exactly one diagnostic line at every sink: the session collection's
// newest occurrence is want — the Path-escaped path plus the
// sanitized reason, never the raw path repeated — the current file's
// overlay row set gained exactly that line, failLines retains that
// single line for re-entry, and the whole collection replays to
// stderr as collected, one line per occurrence with no re-splitting.
func wantSingleLineDiagnostic(t *testing.T, m Model, raw, reason string, collected []string) {
	t.Helper()
	want := "cannot read " + present.Path([]byte(raw)) + ": " + reason
	if len(m.diags) != len(collected) {
		t.Fatalf("session collection holds %d diagnostics, want %d", len(m.diags), len(collected))
	}
	if got := m.diags[len(m.diags)-1]; got != want {
		t.Fatalf("collected diagnostic = %q, want %q", got, want)
	}
	if m.overlay == nil {
		t.Fatal("the current file's read failure did not open the overlay")
	}
	if got := m.overlay.lines[len(m.overlay.lines)-1]; got != want {
		t.Fatalf("overlay's newest row = %q, want %q", got, want)
	}
	if got := m.failLines[raw]; !slices.Equal(got, []string{want}) {
		t.Fatalf("failLines = %q, want the single line %q", got, want)
	}
	if got := replayed(t, m); !slices.Equal(got, collected) {
		t.Fatalf("replayed diagnostics = %q, want %q", got, collected)
	}
}

// The initial load's read failure — the file indexed by the search,
// the worker parked on the load gate, the fixture removed, the real
// os.ReadFile released to fail — is exactly one diagnostic line per
// hostile name at every sink: the overlay row set, the session
// collection, and the stderr replay.
func TestInitialReadFailureIsOneLine(t *testing.T) {
	for _, fx := range hostileReadNames {
		t.Run(fx.name, func(t *testing.T) {
			dir := t.TempDir()
			resolved := filepath.Join(dir, fx.raw)
			writeWorkFile(t, dir, fx.raw, "hit\n")
			gate := make(chan struct{})
			m, cmd := gatedModel(t, dir, gate, nil,
				append(pathRecs([]byte(fx.raw)), recSummary)...)
			if cmd == nil {
				t.Fatal("search completion returned no load command")
			}

			msg := gatedReadFailure(t, gate, cmd, resolved)
			reason := wantReadPathError(t, msg, fx.raw, resolved)
			m = pump(t, m, msg)

			want := "cannot read " + present.Path([]byte(fx.raw)) + ": " + reason
			wantSingleLineDiagnostic(t, m, fx.raw, reason, []string{want})
			if _, err := os.Stat(resolved); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("hostile fixture survives at %q", resolved)
			}
		})
	}
}

// The r reload's read failure is the same construction: the hostile
// file loads once, a fresh gate arms the reread, the fixture is
// removed while the retry worker waits, and the released real read
// fails — one diagnostic line, every sink.
func TestReloadReadFailureIsOneLine(t *testing.T) {
	for _, fx := range hostileReadNames {
		t.Run(fx.name, func(t *testing.T) {
			dir := t.TempDir()
			resolved := filepath.Join(dir, fx.raw)
			writeWorkFile(t, dir, fx.raw, "hit\n")
			m, cmd := gatedModel(t, dir, nil, nil,
				append(pathRecs([]byte(fx.raw)), recSummary)...)
			m = settle(t, m, cmd)
			if m.bufs[fx.raw] == nil {
				t.Fatal("the initial load did not install a buffer")
			}

			gate := make(chan struct{})
			m.loadGate = gate
			m, cmd = update(t, m, keyPress("r"))
			if cmd == nil {
				t.Fatal("r issued no reload command")
			}
			msg := gatedReadFailure(t, gate, cmd, resolved)
			reason := wantReadPathError(t, msg, fx.raw, resolved)
			m = pump(t, m, msg)

			want := "cannot read " + present.Path([]byte(fx.raw)) + ": " + reason
			wantSingleLineDiagnostic(t, m, fx.raw, reason, []string{want})
		})
	}
}

// The failed-path re-entry retry composes the same single line: the
// hostile file's first visit fails behind the gate, the fixture is
// recreated, and the cross-file re-entry's retry parks on a fresh gate
// for the same remove-then-release failure. Each occurrence lands as
// exactly one line at every sink.
func TestReEntryRetryReadFailureIsOneLine(t *testing.T) {
	for _, fx := range hostileReadNames {
		t.Run(fx.name, func(t *testing.T) {
			dir := t.TempDir()
			resolved := filepath.Join(dir, fx.raw)
			writeWorkFile(t, dir, fx.raw, "hit\n")
			recs := append(fileWithStops(t, dir, "a.txt", 10, 1),
				pathRecs([]byte(fx.raw))...)
			recs = append(recs, recSummary)
			m, cmd := gatedModel(t, dir, nil, nil, recs...)
			m = settle(t, m, cmd) // a.txt loads

			// First visit: the hostile file's load is gate-held, the
			// fixture removed, the real read released to fail.
			gate1 := make(chan struct{})
			m.loadGate = gate1
			m, cmdB := update(t, m, keyPress("n"))
			msg := gatedReadFailure(t, gate1, cmdB, resolved)
			reason := wantReadPathError(t, msg, fx.raw, resolved)
			m = pump(t, m, msg)
			want := "cannot read " + present.Path([]byte(fx.raw)) + ": " + reason
			wantSingleLineDiagnostic(t, m, fx.raw, reason, []string{want})
			m, _ = pressKey(t, m, "esc")
			m, _ = update(t, m, keyPress("p")) // back to a.txt

			// Re-entry: the fixture returns so the retry's gate-held
			// removal is the same recipe, and the prior-failure overlay
			// opens before the retry settles.
			writeWorkFile(t, dir, fx.raw, "hit\n")
			gate2 := make(chan struct{})
			m.loadGate = gate2
			m, retry := update(t, m, keyPress("n"))
			if m.overlay == nil {
				t.Fatal("re-entry did not re-open the prior-failure overlay")
			}
			if retry == nil {
				t.Fatal("re-entry issued no retry load")
			}
			msg = gatedReadFailure(t, gate2, retry, resolved)
			reason = wantReadPathError(t, msg, fx.raw, resolved)
			m = pump(t, m, msg)
			wantSingleLineDiagnostic(t, m, fx.raw, reason, []string{want, want})
		})
	}
}

// A *os.PathError buried under wrapping unwraps the same way: the
// sanitized reason is the bare Err — the wrapped PathError's own raw
// path never reaches the line — while a non-path error keeps its own
// text untouched.
func TestReadFailureReasonUnwrapsPathError(t *testing.T) {
	m := newModel(nil, nil)
	raw := "z\nname.txt"
	req := mintLoad(&m, raw)
	m, _ = update(t, m, loadDoneMsg{
		path: []byte(raw),
		req:  req,
		err: fmt.Errorf("read attempt: %w",
			&os.PathError{Op: "open", Path: raw, Err: os.ErrNotExist}),
	})
	want := `cannot read z\nname.txt: file does not exist`
	if got := replayed(t, m); !slices.Equal(got, []string{want}) {
		t.Fatalf("replayed diagnostics = %q, want %q", got, want)
	}

	req = mintLoad(&m, "plain.txt")
	m, _ = update(t, m, loadDoneMsg{
		path: []byte("plain.txt"),
		req:  req,
		err:  errors.New("denied"),
	})
	got := replayed(t, m)
	if !slices.Equal(got[len(got)-1:], []string{"cannot read plain.txt: denied"}) {
		t.Fatalf("replayed diagnostics = %q, want the plain denial last", got)
	}
}

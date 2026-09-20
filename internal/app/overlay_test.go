package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/clipperhouse/displaywidth"

	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

// overlayModel builds a model that has completed its search with the
// given records, wait status, and captured stderr.
func overlayModel(t *testing.T, records []string, procErr error, stderr string) Model {
	t.Helper()
	b := searchindex.NewBuilder("/w")
	for _, r := range records {
		if err := b.Add([]byte(r)); err != nil {
			t.Fatalf("Add(%q): %v", r, err)
		}
	}
	idx, integrity := b.Finish()
	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, "/w")
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m = feedStderr(t, m, stderr)
	m, _ = update(t, m, searchResult{
		index: idx, integrity: integrity,
		err: procErr,
	})
	return m
}

func validRecords() []string {
	return []string{
		beginRec("a.txt"),
		matchRec("a.txt", "hit\n", 1, 0, 3, "hit"),
		endRec("a.txt", nil),
		summaryRec(),
	}
}

// up and down scroll the overlay's wrapped diagnostic rows, clamped to
// the complete row set; every row is reachable.
func TestOverlayScrollsUpDown(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("head-line\n")
	for i := 0; i < 38; i++ {
		fmt.Fprintf(&sb, "filler %02d\n", i)
	}
	sb.WriteString("tail-line\n")
	m := overlayModel(t, validRecords(), nil, sb.String())
	if m.overlay == nil {
		t.Fatal("no overlay open for a stderr diagnostic")
	}

	v := m.View().Content
	if !strings.Contains(v, "head-line") || strings.Contains(v, "tail-line") {
		t.Fatalf("initial overlay shows the wrong window:\n%s", v)
	}

	// 40 diagnostic rows; the interior shows 22 at 80x24, so the maximum
	// scroll is 18.
	for i := 0; i < 100; i++ {
		m, _ = update(t, m, keyPress("down"))
	}
	if m.overlay.scroll != 18 {
		t.Fatalf("scroll after max down = %d, want 18", m.overlay.scroll)
	}
	v = m.View().Content
	if !strings.Contains(v, "tail-line") || strings.Contains(v, "head-line") {
		t.Fatalf("scrolled-to-bottom overlay lacks the tail:\n%s", v)
	}

	for i := 0; i < 100; i++ {
		m, _ = update(t, m, keyPress("up"))
	}
	if m.overlay.scroll != 0 {
		t.Fatalf("scroll after max up = %d, want 0", m.overlay.scroll)
	}
	v = m.View().Content
	if !strings.Contains(v, "head-line") || strings.Contains(v, "tail-line") {
		t.Fatalf("scrolled-to-top overlay lacks the head:\n%s", v)
	}
}

// q and Esc both dismiss an overlay that has an underlying state —
// browse for the error overlay over results, the no-results screen for a
// warning — and neither press quits.
func TestOverlayDismissKeys(t *testing.T) {
	for _, key := range []string{"q", "esc"} {
		t.Run(key+" over browse", func(t *testing.T) {
			m := overlayModel(t, validRecords(), exitError(2), "boom\n")
			if m.overlay == nil || m.state != stateBrowse {
				t.Fatalf("want browse under an error overlay, state %d overlay %v", m.state, m.overlay != nil)
			}
			m, cmd := update(t, m, keyPress(key))
			if cmd != nil {
				t.Fatalf("%s dismissal returned a command: %v", key, cmd)
			}
			if m.overlay != nil || m.state != stateBrowse {
				t.Fatalf("%s did not dismiss to browse: state %d overlay %v", key, m.state, m.overlay != nil)
			}
			if strings.Contains(m.View().Content, "boom") {
				t.Fatal("dismissed overlay still shows its diagnostic")
			}
		})
		t.Run(key+" over no-results", func(t *testing.T) {
			m := overlayModel(t, []string{summaryRec()}, exitError(1), "warn\n")
			if m.overlay == nil || m.state != stateNoResults {
				t.Fatalf("want no-results under a warning overlay, state %d overlay %v", m.state, m.overlay != nil)
			}
			m, cmd := update(t, m, keyPress(key))
			if cmd != nil {
				t.Fatalf("%s dismissal returned a command: %v", key, cmd)
			}
			if m.overlay != nil || m.state != stateNoResults {
				t.Fatalf("%s did not dismiss to no-results: state %d overlay %v", key, m.state, m.overlay != nil)
			}
			if got := m.View().Content; !strings.Contains(got, "No results found") {
				t.Fatalf("post-dismissal view lost the no-results screen:\n%s", got)
			}
		})
	}
}

// ctrl+c exits 130 even with an overlay open.
func TestOverlayCtrlCExits130(t *testing.T) {
	child := &fakeChild{
		stdout:     strings.NewReader(""),
		stderr:     strings.NewReader(""),
		terminated: make(chan struct{}),
	}
	b := searchindex.NewBuilder("/w")
	for _, r := range validRecords() {
		if err := b.Add([]byte(r)); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	idx, integrity := b.Finish()
	m := New(child, "/w")
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m = feedStderr(t, m, "warn\n")
	m, _ = update(t, m, searchResult{
		index: idx, integrity: integrity,
		err: exitError(2),
	})
	if m.overlay == nil {
		t.Fatal("no overlay open")
	}
	m, cmd := update(t, m, keyPress("ctrl+c"))
	requireQuit(t, cmd, "ctrl+c with an overlay open")
	if m.status != 130 {
		t.Fatalf("exit status = %d, want 130", m.status)
	}
	requireClosed(t, child.terminated, "child termination")
}

// Every key other than up/down, q, Esc, and ctrl+c is ignored while the
// overlay is open — including base-state bindings like c and n.
func TestOverlayIgnoresOtherKeys(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&sb, "line %02d\n", i)
	}
	m := overlayModel(t, validRecords(), nil, sb.String())
	m, _ = update(t, m, keyPress("down"))
	if m.overlay == nil || m.overlay.scroll != 1 {
		t.Fatalf("setup: overlay scroll = %v/%d", m.overlay != nil, m.overlay.scroll)
	}
	for _, key := range []string{"x", "n", "c", "left", "right", "pgdn", "enter", " "} {
		m2, cmd := update(t, m, keyPress(key))
		if cmd != nil {
			t.Fatalf("%s with an open overlay returned a command: %v", key, cmd)
		}
		m = m2
		if m.overlay == nil {
			t.Fatalf("%s dismissed the overlay", key)
		}
		if m.overlay.scroll != 1 {
			t.Fatalf("%s moved the overlay scroll to %d", key, m.overlay.scroll)
		}
		if m.state != stateBrowse {
			t.Fatalf("%s changed state to %d", key, m.state)
		}
	}
}

// A long unbroken diagnostic wraps at the interior width inside the
// border; no row exceeds the box width and the full text is rendered.
func TestOverlayWrapsUnbrokenDiagnostic(t *testing.T) {
	unbroken := strings.Repeat("a", 200)
	m := overlayModel(t, validRecords(), nil, unbroken+"\n")
	m.theme = theme.Plain()
	v := m.View().Content

	full := "│" + strings.Repeat("a", 78) + "│"
	if n := strings.Count(v, full); n != 2 {
		t.Fatalf("wrapped unbroken rows = %d, want 2 full-width interior rows:\n%s", n, v)
	}
	// 200 cells wrap as 78 + 78 + 44: the partial third row is padded
	// out to the border.
	partial := "│" + strings.Repeat("a", 44) + strings.Repeat(" ", 34) + "│"
	if !strings.Contains(v, partial) {
		t.Fatalf("wrapped overlay lacks the partial third row %q:\n%s", partial, v)
	}
	for i, line := range strings.Split(v, "\n") {
		if w := displaywidth.String(line); w > 80 {
			t.Fatalf("rendered line %d is %d cells wide, over the 80-cell terminal:\n%q", i, w, line)
		}
	}
}

// A failed process that supplied no stderr gets a generated diagnostic
// naming its exit code or signal; a failed process with stderr shows the
// stderr instead of a manufactured status line.
func TestFailedProcessGeneratesDiagnostic(t *testing.T) {
	t.Run("exit code", func(t *testing.T) {
		m := overlayModel(t, []string{summaryRec()}, exitError(3), "")
		if m.overlay == nil {
			t.Fatal("no overlay for a fatal no-results outcome")
		}
		if v := m.View().Content; !strings.Contains(v, "exited with code 3") {
			t.Fatalf("overlay does not name the exit code:\n%s", v)
		}
	})
	t.Run("signal", func(t *testing.T) {
		m := overlayModel(t, []string{summaryRec()}, signalError("signal: killed"), "")
		if v := m.View().Content; !strings.Contains(v, "signal") {
			t.Fatalf("overlay does not name the signal:\n%s", v)
		}
	})
	t.Run("stderr instead", func(t *testing.T) {
		m := overlayModel(t, []string{summaryRec()}, exitError(3), "boom\n")
		v := m.View().Content
		if !strings.Contains(v, "boom") {
			t.Fatalf("overlay lacks the captured stderr:\n%s", v)
		}
		if strings.Contains(v, "exited with code") {
			t.Fatalf("overlay manufactured a status line despite stderr:\n%s", v)
		}
	})
}

// Stream-integrity failure produces an explanatory note rather than an
// empty overlay, even when the process exited cleanly with no stderr.
func TestIntegrityFailureProducesDiagnostic(t *testing.T) {
	m := overlayModel(t, []string{
		beginRec("a.txt"),
		matchRec("a.txt", "hit\n", 1, 0, 3, "hit"),
		endRec("a.txt", nil),
	}, nil, "")
	if m.overlay == nil {
		t.Fatal("no overlay for an integrity failure")
	}
	if v := m.View().Content; !strings.Contains(v, "incomplete") {
		t.Fatalf("overlay lacks an integrity diagnostic:\n%s", v)
	}
}

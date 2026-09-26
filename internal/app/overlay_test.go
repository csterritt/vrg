package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"vrg/internal/theme"
)

// overlayModel returns a model whose completed search opened the
// diagnostics overlay: a valid one-match stream, the given child stderr
// and wait error. It fails the test when no overlay opened.
func overlayModel(t *testing.T, stderr string, waitErr error) Model {
	t.Helper()
	dir := t.TempDir()
	writeWorkFile(t, dir, "f.txt", "hit\n")
	m := newModel(nil, nil)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = update(t, m, searchDoneMsg{
		index:   fixtureIndex(t, dir, recsOneMatch...),
		stderr:  []byte(stderr),
		waitErr: waitErr,
	})
	if m.overlay == nil {
		t.Fatalf("no overlay opened (phase %d)", m.phase)
	}
	return m
}

// overlayLines builds a numbered stderr block: "err-00" … "err-(n-1)".
func overlayLines(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "err-%02d\n", i)
	}
	return b.String()
}

// The overlay scrolls its diagnostic lines with up/down: the head is
// visible first, the tail only after scrolling, and scrolling clamps at
// both ends instead of drifting past the content.
func TestOverlayScrollsWithUpDown(t *testing.T) {
	m := overlayModel(t, overlayLines(40), nil)

	v := m.View().Content
	if !strings.Contains(v, "err-00") {
		t.Fatalf("overlay head not visible initially: %q", v)
	}
	if strings.Contains(v, "err-39") {
		t.Fatalf("overlay tail visible before scrolling: %q", v)
	}

	// Up at the top is a clamped no-op.
	m, _ = pressKey(t, m, "up")
	if !strings.Contains(m.View().Content, "err-00") {
		t.Fatal("up at the top scrolled past the head")
	}

	// Enough downs reach the tail; extra downs clamp at the bottom.
	for i := 0; i < 40; i++ {
		m, _ = pressKey(t, m, "down")
	}
	v = m.View().Content
	if !strings.Contains(v, "err-39") {
		t.Fatalf("overlay tail not visible after scrolling: %q", v)
	}
	if strings.Contains(v, "err-00") {
		t.Fatalf("overlay head still visible at the bottom: %q", v)
	}

	// Back up returns the head.
	for i := 0; i < 40; i++ {
		m, _ = pressKey(t, m, "up")
	}
	if !strings.Contains(m.View().Content, "err-00") {
		t.Fatal("up did not scroll back to the head")
	}
}

// q and Esc each dismiss a non-fatal overlay to the underlying screen.
func TestOverlayDismissKeys(t *testing.T) {
	for _, key := range []string{"q", "esc"} {
		t.Run(key, func(t *testing.T) {
			m := overlayModel(t, "rg: warn\n", nil)
			m2, cmd := pressKey(t, m, key)
			if cmd != nil {
				t.Fatalf("%s on a warning overlay produced a command %T, want none", key, cmd)
			}
			if m2.overlay != nil || m2.phase != phaseBrowse || m2.quit {
				t.Fatalf("%s dismissal: overlay=%v phase=%d quit=%v, want browse revealed",
					key, m2.overlay != nil, m2.phase, m2.quit)
			}
		})
	}
}

// ctrl+c inside the overlay takes the cancellation path: quit at 130
// whatever the fixed status would have been.
func TestOverlayCtrlCExits130(t *testing.T) {
	m := overlayModel(t, "rg: warn\n", exitErr(t, 3))
	m2, cmd := pressKey(t, m, "ctrl+c")
	if cmd == nil {
		t.Fatal("ctrl+c on the overlay returned no command, want tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("ctrl+c command = %T, want tea.QuitMsg", cmd())
	}
	if m2.ExitCode() != 130 {
		t.Fatalf("ExitCode = %d, want 130", m2.ExitCode())
	}
}

// Every other key is ignored while the overlay is open: no dismissal,
// no quit, no theme toggle, no state change of any kind.
func TestOverlayIgnoresOtherKeys(t *testing.T) {
	m := overlayModel(t, "rg: warn\n", nil)
	for _, key := range []string{"x", "n", "c", "0", "left", "right", "tab", "enter"} {
		m2, cmd := pressKey(t, m, key)
		if cmd != nil {
			t.Fatalf("%q on the overlay produced a command %T, want none", key, cmd)
		}
		if m2.overlay == nil || m2.quit || m2.phase != phaseBrowse {
			t.Fatalf("%q changed state: overlay=%v quit=%v phase=%d",
				key, m2.overlay != nil, m2.quit, m2.phase)
		}
	}
	if m.theme.Light() {
		t.Fatal("c reached the colour toggle while the overlay was open")
	}
}

// A long unbroken diagnostic wraps inside the border rather than
// overflowing it: every frame row stays within the terminal width and
// the text occupies several interior rows.
func TestOverlayWrapsUnbrokenDiagnostic(t *testing.T) {
	m := overlayModel(t, strings.Repeat("x", 300)+"\n", exitErr(t, 3))
	m.theme = theme.Plain()
	v := m.View().Content
	for i, row := range strings.Split(v, "\n") {
		if w := ansi.StringWidth(row); w > 80 {
			t.Fatalf("frame row %d is %d cells wide, want ≤80: %q", i, w, row)
		}
	}
	interior := 0
	for _, row := range strings.Split(v, "\n") {
		if strings.HasPrefix(row, "│") {
			interior++
		}
	}
	if interior < 4 {
		t.Fatalf("300-cell diagnostic occupied %d interior rows, want wrapping", interior)
	}
}

// A failed process that wrote no stderr still gets a diagnostic naming
// its exit code or its signal — the overlay never opens empty.
func TestFailedProcessWithoutStderrGetsGeneratedDiagnostic(t *testing.T) {
	m := overlayModel(t, "", exitErr(t, 3))
	if v := m.View().Content; !strings.Contains(v, "exit status 3") {
		t.Fatalf("overlay lacks the generated exit-code diagnostic: %q", v)
	}
	m = overlayModel(t, "", sigErr(t))
	if v := m.View().Content; !strings.Contains(v, "signal: killed") {
		t.Fatalf("overlay lacks the generated signal diagnostic: %q", v)
	}
}

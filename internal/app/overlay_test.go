package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/safepresentation"
	"vrg/internal/theme"
)

var (
	keyUp   = tea.KeyPressMsg{Code: tea.KeyUp}
	keyDown = tea.KeyPressMsg{Code: tea.KeyDown}
)

// overlayModel builds a model sitting on the open error overlay: a
// fatal exit-3 outcome with captured stderr over the browse view.
func overlayModel(t *testing.T, stderr string) *model {
	t.Helper()
	res := Result{Stdout: []byte(happyStream), Stderr: []byte(stderr), Code: 3}
	m := newTestModel(fakeChild{res: res}, options{})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.Update(runCollectCmd(t, m))
	if !m.overlayOpen {
		t.Fatalf("overlay did not open; state = %v", m.state)
	}
	return m
}

// up and down scroll the overlay's complete wrapped row set, clamped to
// [0, rows-visible]; pgup, pgdown, and every other key are ignored while
// the modal is open.
func TestOverlayKeyRoutingAndScrolling(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&sb, "diag-%03d\n", i)
	}
	m := overlayModel(t, sb.String())

	// Every other key is inert: no scroll, no dismissal, no command,
	// no theme toggle.
	for _, k := range []tea.KeyPressMsg{
		{Text: "x", Code: 'x'},
		{Text: "c", Code: 'c'},
		{Code: tea.KeyPgUp},
		{Code: tea.KeyPgDown},
		{Code: tea.KeyEnter},
		{Code: tea.KeyRight},
		{Code: tea.KeyLeft},
		{Code: tea.KeyHome},
		{Code: tea.KeyEnd},
	} {
		if _, cmd := m.Update(k); cmd != nil {
			t.Fatalf("key %v returned a command; the overlay ignores it", k)
		}
		if !m.overlayOpen || m.overlayScroll != 0 || m.quitting {
			t.Fatalf("key %v disturbed the overlay: open=%v scroll=%d quitting=%v",
				k, m.overlayOpen, m.overlayScroll, m.quitting)
		}
	}
	if m.theme != theme.Dark() {
		t.Fatal("the c key toggled the theme through the open overlay")
	}

	// up at the top is a clamp, not an error.
	if _, cmd := m.Update(keyUp); cmd != nil || m.overlayScroll != 0 {
		t.Fatalf("up at scroll 0: scroll=%d cmd=%v", m.overlayScroll, cmd)
	}

	m.Update(keyDown)
	if m.overlayScroll != 1 {
		t.Fatalf("scroll after down = %d, want 1", m.overlayScroll)
	}
	m.Update(keyUp)
	if m.overlayScroll != 0 {
		t.Fatalf("scroll after up = %d, want 0", m.overlayScroll)
	}

	// The scrollable set is the complete wrapped diagnostic: scroll
	// past the bottom and the view clamps on the last rows — the
	// head row has left the frame, the tail row is in it.
	for i := 0; i < 40; i++ {
		m.Update(keyDown)
	}
	maxScroll := len(m.overlayRows()) - m.overlayVisible()
	if maxScroll < 1 {
		t.Fatalf("diagnostic did not overflow the frame: rows=%d visible=%d",
			len(m.overlayRows()), m.overlayVisible())
	}
	if m.overlayScroll != maxScroll {
		t.Fatalf("scroll clamped at %d, want %d", m.overlayScroll, maxScroll)
	}
	v := viewText(m)
	if !strings.Contains(v, "diag-029") {
		t.Fatalf("bottom of the scroll range missing the tail row: %q", v)
	}
	if strings.Contains(v, "diag-000") {
		t.Fatalf("scrolled-to-bottom frame still shows the head row: %q", v)
	}
	if !strings.Contains(v, "diag-008") {
		t.Fatalf("frame should begin at the first scrolled row: %q", v)
	}
}

// q and Esc dismiss a nonfatal-positioned overlay back to its base
// state — the browse view here — with no exit.
func TestOverlayDismissKeys(t *testing.T) {
	for _, k := range []string{"q", "esc"} {
		m := overlayModel(t, "boom\n")
		_, cmd := m.Update(outcomeKey(k))
		if cmd != nil {
			t.Fatalf("%s dismissal returned a command, want none", k)
		}
		if m.overlayOpen || m.quitting {
			t.Fatalf("%s: overlayOpen=%v quitting=%v, want dismissed and running",
				k, m.overlayOpen, m.quitting)
		}
		if m.state != stateBrowse {
			t.Fatalf("%s dismissal left state = %v, want browse", k, m.state)
		}
		if v := viewText(m); strings.Contains(v, "boom") {
			t.Fatalf("%s: dismissed overlay still renders: %q", k, v)
		}
	}
}

// ctrl+c inside an open overlay is the global override: cleanup and
// exit 130 whatever the fixed search-derived status was.
func TestOverlayCtrlCExits130(t *testing.T) {
	m := overlayModel(t, "boom\n")
	_, cmd := m.Update(keyCtrlC)
	if !m.quitting {
		t.Fatal("ctrl+c in the overlay did not begin a controlled exit")
	}
	runQuittingCmd(t, cmd)
	if m.status != 130 {
		t.Fatalf("status = %d, want ctrl+c's 130 override", m.status)
	}
}

// A long unbroken diagnostic wraps within the border: every rendered
// row fits the terminal width and the whole text survives across the
// wrapped rows.
func TestOverlayWrapsUnbrokenDiagnostic(t *testing.T) {
	m := overlayModel(t, strings.Repeat("x", 200)+"\n")
	m.theme = theme.Plain()
	v := viewText(m)
	if !strings.Contains(v, "┌") || !strings.Contains(v, "└") {
		t.Fatalf("overlay border missing: %q", v)
	}
	for i, row := range strings.Split(v, "\n") {
		if w := safepresentation.CellWidth(row); w > 80 {
			t.Fatalf("row %d is %d cells wide, over the 80-cell frame: %q", i, w, row)
		}
	}
	if n := strings.Count(v, "x"); n != 200 {
		t.Fatalf("wrapped diagnostic lost text: %d x's, want 200", n)
	}
}

// The scrollable row set is the complete wrapped diagnostic — head to
// tail — so a stderr dump larger than any frame keeps both ends
// reachable rather than compressed away (Issue #41's contract).
func TestOverlayKeepsCompleteDiagnostic(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("stderr head marker\n")
	sb.WriteString(strings.Repeat("e", 1<<20)) // > 1 MiB on one line
	sb.WriteString("\nstderr tail marker\n")
	m := overlayModel(t, sb.String())

	rows := m.overlayRows()
	if rows[0] != "stderr head marker" {
		t.Fatalf("first overlay row = %q, want the head marker", rows[0])
	}
	if got := rows[len(rows)-1]; got != "stderr tail marker" {
		t.Fatalf("last overlay row = %q, want the tail marker", got)
	}
	if v := viewText(m); !strings.Contains(v, "stderr head marker") {
		t.Fatalf("initial frame should show the head of the diagnostic: %q", v)
	}
}

// A failed process with no stderr gets a generated diagnostic naming
// the exit code or signal; a clean process gets no overlay at all.
func TestGeneratedProcessDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		name string
		code int
		err  error
		want string
	}{
		{"exit code", 2, nil, "code 2"},
		{"signal", -1, fmt.Errorf("signal: killed"), "killed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := Result{Stdout: []byte(emptyStream), Code: tc.code, Err: tc.err}
			m := newTestModel(fakeChild{res: res}, options{})
			m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			m.Update(runCollectCmd(t, m))
			if !m.overlayOpen {
				t.Fatal("failed process without results must open the overlay")
			}
			if v := viewText(m); !strings.Contains(v, tc.want) {
				t.Fatalf("generated diagnostic missing %q: %q", tc.want, v)
			}
		})
	}
}

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
	// no theme toggle — u, d, and the page keys are base file-content
	// bindings the modal overlay swallows (Issue #41's unchanged key
	// contract).
	for _, k := range []tea.KeyPressMsg{
		{Text: "x", Code: 'x'},
		{Text: "c", Code: 'c'},
		keyU,
		keyD,
		keyPgUp,
		keyPgDn,
		{Code: tea.KeyEnter},
		{Code: tea.KeyRight},
		{Code: tea.KeyLeft},
		{Code: tea.KeyHome},
		{Code: tea.KeyEnd},
	} {
		if _, cmd := m.Update(k); cmd != nil {
			t.Fatalf("key %v returned a command; the overlay ignores it", k)
		}
		if !m.overlayOpen || m.overlay.scroll != 0 || m.quitting {
			t.Fatalf("key %v disturbed the overlay: open=%v scroll=%d quitting=%v",
				k, m.overlayOpen, m.overlay.scroll, m.quitting)
		}
	}
	if m.theme != theme.Dark() {
		t.Fatal("the c key toggled the theme through the open overlay")
	}

	// up at the top is a clamp, not an error.
	if _, cmd := m.Update(keyUp); cmd != nil || m.overlay.scroll != 0 {
		t.Fatalf("up at scroll 0: scroll=%d cmd=%v", m.overlay.scroll, cmd)
	}

	m.Update(keyDown)
	if m.overlay.scroll != 1 {
		t.Fatalf("scroll after down = %d, want 1", m.overlay.scroll)
	}
	m.Update(keyUp)
	if m.overlay.scroll != 0 {
		t.Fatalf("scroll after up = %d, want 0", m.overlay.scroll)
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
	if m.overlay.scroll != maxScroll {
		t.Fatalf("scroll clamped at %d, want %d", m.overlay.scroll, maxScroll)
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
	// The fixture carries no ellipsis, so any … in the set is an
	// injected compression marker — forbidden by Issue #41.
	for i, r := range rows {
		if strings.Contains(r, "…") {
			t.Fatalf("row %d is an injected ellipsis marker: %q", i, r)
		}
	}
	if v := viewText(m); !strings.Contains(v, "stderr head marker") {
		t.Fatalf("initial frame should show the head of the diagnostic: %q", v)
	}

	// The tail row is reachable: the scroll path's clamp lands the
	// window's last row on the diagnostic's last row — the proof the
	// retired head/tail frame used to owe, without sending thousands
	// of keys.
	vis := m.overlayVisible()
	m.scrollOverlay(len(rows))
	if want := len(rows) - vis; m.overlay.scroll != want {
		t.Fatalf("scroll clamped at %d, want %d", m.overlay.scroll, want)
	}
	m.Update(keyDown)
	if want := len(rows) - vis; m.overlay.scroll != want {
		t.Fatalf("down at the bottom moved scroll to %d, want clamped %d",
			m.overlay.scroll, want)
	}
	if v := viewText(m); !strings.Contains(v, "stderr tail marker") {
		t.Fatalf("scrolled-to-bottom frame missing the tail marker: %q", v)
	}
	if strings.Contains(viewText(m), "stderr head marker") {
		t.Fatal("scrolled-to-bottom frame still shows the head marker")
	}
}

// A diagnostic only a few rows taller than the box walks its window
// across the complete wrapped set: each down moves exactly one row —
// the row that scrolled off leaves the frame while the new top row
// enters it — until the final line renders, and each up walks it back
// to the first line. No ellipsis ever substitutes for content.
func TestOverlayBoundedTraversalReachesEveryRow(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 25; i++ {
		fmt.Fprintf(&sb, "line-%02d\n", i)
	}
	// No trailing newline: every wrapped row carries a unique marker,
	// so presence checks in the frame are unambiguous.
	m := overlayModel(t, strings.TrimSuffix(sb.String(), "\n"))
	m.theme = theme.Plain()

	rows := m.overlayRows()
	vis := m.overlayVisible()
	maxScroll := len(rows) - vis
	if maxScroll < 2 || maxScroll > 6 {
		t.Fatalf("fixture must barely overflow the box: rows=%d visible=%d",
			len(rows), vis)
	}
	for i, r := range rows {
		if strings.Contains(r, "…") {
			t.Fatalf("row %d is an injected ellipsis marker: %q", i, r)
		}
	}

	// Down: each step advances the window exactly one row until the
	// final line is in the frame; further down is a clamp.
	for s := 1; s <= maxScroll; s++ {
		m.Update(keyDown)
		if m.overlay.scroll != s {
			t.Fatalf("down %d: scroll = %d, want %d", s, m.overlay.scroll, s)
		}
		v := viewText(m)
		if !strings.Contains(v, rows[s]) {
			t.Fatalf("down %d: new top row %q missing from %q", s, rows[s], v)
		}
		if strings.Contains(v, rows[s-1]) {
			t.Fatalf("down %d: scrolled-off row %q still renders: %q", s, rows[s-1], v)
		}
	}
	if v := viewText(m); !strings.Contains(v, rows[len(rows)-1]) {
		t.Fatalf("bottom of the traversal missing the final row %q: %q",
			rows[len(rows)-1], v)
	}
	m.Update(keyDown)
	if m.overlay.scroll != maxScroll {
		t.Fatalf("down past the bottom moved scroll to %d, want clamped %d",
			m.overlay.scroll, maxScroll)
	}

	// Up: each step retreats the window exactly one row until the
	// first line tops the frame again; further up is a clamp.
	for s := maxScroll - 1; s >= 0; s-- {
		m.Update(keyUp)
		if m.overlay.scroll != s {
			t.Fatalf("up %d: scroll = %d, want %d", s, m.overlay.scroll, s)
		}
		v := viewText(m)
		if !strings.Contains(v, rows[s]) {
			t.Fatalf("up %d: new top row %q missing from %q", s, rows[s], v)
		}
		if strings.Contains(v, rows[s+vis]) {
			t.Fatalf("up %d: bottom row %q scrolled off yet still renders: %q",
				s, rows[s+vis], v)
		}
	}
	m.Update(keyUp)
	if m.overlay.scroll != 0 {
		t.Fatalf("up past the top moved scroll to %d, want clamped 0",
			m.overlay.scroll)
	}
}

// The render path applies the same clamp the key handler enforces: a
// scroll index outside [0, rows-visible] — stale after a shrink, or
// never reachable through keys at all — renders the clamped window
// rather than panicking or dropping rows. A resize that grows the
// frame past the diagnostic reclamps the position itself.
func TestOverlayRenderClampsScroll(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&sb, "diag-%03d\n", i)
	}
	m := overlayModel(t, sb.String())
	m.theme = theme.Plain()

	rows := m.overlayRows()
	vis := m.overlayVisible()
	maxScroll := len(rows) - vis
	if maxScroll < 1 {
		t.Fatalf("fixture must overflow the box: rows=%d visible=%d", len(rows), vis)
	}

	m.overlay.scroll = maxScroll + 10 // unreachable through keys
	if v := viewText(m); !strings.Contains(v, rows[len(rows)-1]) {
		t.Fatalf("render with scroll past the range lost the tail row: %q", v)
	}
	m.overlay.scroll = -5
	if v := viewText(m); !strings.Contains(v, rows[0]) {
		t.Fatalf("render with scroll below the range lost the head row: %q", v)
	}

	// Growing the frame past the diagnostic collapses the range to 0:
	// the resize's reclamp returns the reader to the first row.
	m.overlay.scroll = maxScroll
	m.Update(tea.WindowSizeMsg{Width: 80, Height: len(rows) + 4})
	if m.overlay.scroll != 0 {
		t.Fatalf("resize left scroll = %d, want reclamped 0", m.overlay.scroll)
	}
	if v := viewText(m); !strings.Contains(v, rows[0]) {
		t.Fatalf("post-resize frame missing the head row: %q", v)
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

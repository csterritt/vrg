package app

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"vrg/internal/present"
)

// overlay is the modal diagnostics box: sanitized diagnostic lines and
// the scroll offset in wrapped rows. It opens over the underlying
// screen — or alone over a blank frame in the fatal no-results case —
// and owns the keyboard while open.
type overlay struct {
	lines  []string
	scroll int
}

// outcome is the pure decision a completed search produces: the
// underlying screen, whether the diagnostics overlay opens over it, the
// fixed exit status, and the diagnostic lines to show. Process result,
// stream integrity, and usable results are assessed independently and
// combined exactly once here — later issues extend the decision through
// this function rather than the update path.
type outcome struct {
	screen  phase
	overlay bool
	code    int
	diags   []string
}

// decideOutcome maps one completed search to its presentation and fixed
// exit status. waitErr is the child's wait error — nil, the benign
// "no matches" exit status 1, or a fatal outcome — failures are the
// stream-integrity diagnostics, usable is the retained-stop count
// assessed after all filtering, recordLoss is the malformed+oversized
// skip tally, and recordDiags are the stream's record-skip diagnostic
// lines. Any diagnostics make the outcome overlay-bearing; a fatal
// outcome — a fatal process result, any integrity failure, or record
// loss that left zero usable results — fixes status 2, usable results
// fix 0, and an intact empty stream fixes 1. Unknown-type warnings are
// diagnostics only and never turn fatal on their own.
func decideOutcome(waitErr error, stderr []byte, failures []string, usable, recordLoss int, recordDiags []string) outcome {
	fatal := processFatal(waitErr) || len(failures) > 0 || (recordLoss > 0 && usable == 0)
	var o outcome
	o.diags = collectDiagnostics(waitErr, stderr, failures, recordDiags)
	o.overlay = len(o.diags) > 0
	switch {
	case fatal:
		o.code = 2
	case usable > 0:
		o.code = 0
	default:
		o.code = 1
	}
	switch {
	case usable > 0:
		o.screen = phaseBrowse
	case fatal:
		o.screen = phaseFatal
	default:
		o.screen = phaseNoResults
	}
	return o
}

// processFatal reports whether the child's wait error is a fatal
// outcome: any failure other than rg's benign "no matches" exit status
// 1. Signal deaths carry ExitCode -1 and are fatal like codes ≥ 2.
func processFatal(waitErr error) bool {
	var ee *exec.ExitError
	return waitErr != nil && !(errors.As(waitErr, &ee) && ee.ExitCode() == 1)
}

// collectDiagnostics assembles the overlay's diagnostic lines: a
// generated line naming the exit code or signal when the process
// outcome was fatal, the child's sanitized stderr, the stream-integrity
// failures, then the record-skip diagnostics — the named oversized-path
// lines and the malformed, oversized, and unrecognised-type tallies.
// Everything passes through the Issue #6 diagnostic utility before it
// can reach the screen.
func collectDiagnostics(waitErr error, stderr []byte, failures, recordDiags []string) []string {
	var diags []string
	if processFatal(waitErr) {
		diags = append(diags, fmt.Sprintf("rg failed: %s", waitErr))
	}
	if s := strings.TrimRight(string(stderr), "\n"); s != "" {
		diags = append(diags, strings.Split(present.Diagnostic(s), "\n")...)
	}
	for _, f := range failures {
		diags = append(diags, strings.Split(present.Diagnostic(f), "\n")...)
	}
	for _, d := range recordDiags {
		diags = append(diags, strings.Split(present.Diagnostic(d), "\n")...)
	}
	return diags
}

// overlayKey handles one key while the overlay is open: up/down scroll,
// q and Esc dismiss — quitting outright when the fatal no-results
// overlay has no underlying state — ctrl+c takes the cancellation path,
// and every other key is ignored.
func (m Model) overlayKey(key string) (Model, tea.Cmd) {
	switch key {
	case "ctrl+c":
		return m.cancelled(), tea.Quit
	case "q", "esc":
		m.overlay = nil
		if m.phase == phaseFatal {
			m.quit = true
			return m, tea.Quit
		}
	case "up":
		if m.overlay.scroll > 0 {
			m.overlay.scroll--
		}
	case "down":
		if m.overlay.scroll < m.overlayMaxScroll() {
			m.overlay.scroll++
		}
	}
	return m, nil
}

// overlayLayout resolves the overlay's geometry for the current frame:
// the diagnostic lines hard-wrapped to the interior width, the interior
// height, and the scroll offset clamped into range.
func (m Model) overlayLayout() (lines []string, interiorH, scroll int) {
	w := m.width - 2
	if w < 1 {
		w = 1
	}
	maxLine := 0
	for _, l := range m.overlay.lines {
		if n := ansi.StringWidth(l); n > maxLine {
			maxLine = n
		}
	}
	if maxLine < w {
		w = maxLine
	}
	if w < 1 {
		w = 1
	}
	for _, l := range m.overlay.lines {
		for _, wl := range strings.Split(ansi.Wrap(l, w, ""), "\n") {
			lines = append(lines, wl)
		}
	}
	interiorH = m.height - 2
	if interiorH < 1 {
		interiorH = 1
	}
	if len(lines) < interiorH {
		interiorH = len(lines)
	}
	scroll = m.overlay.scroll
	if max := len(lines) - interiorH; scroll > max {
		scroll = max
	}
	if scroll < 0 {
		scroll = 0
	}
	return lines, interiorH, scroll
}

// overlayMaxScroll is the largest scroll offset that still shows
// content — the down key's clamp.
func (m Model) overlayMaxScroll() int {
	lines, interiorH, _ := m.overlayLayout()
	return max(0, len(lines)-interiorH)
}

// renderBlank is the empty frame hosting a fatal-only overlay: height
// rows padded to the frame width so the centred box sits on a painted
// screen.
func renderBlank(w, h int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	row := strings.Repeat(" ", w)
	rows := make([]string, h)
	for i := range rows {
		rows[i] = row
	}
	return strings.Join(rows, "\n")
}

// renderOverlay composites the modal box over the base frame: the
// visible slice of wrapped lines inside a single-line border painted in
// the base colours, centred on the frame.
func (m Model) renderOverlay(base string) string {
	lines, interiorH, scroll := m.overlayLayout()
	visible := lines[scroll : scroll+interiorH]
	box := m.theme.Overlay(visible)
	boxRows := strings.Split(box, "\n")
	boxW := ansi.StringWidth(boxRows[0])
	boxH := len(boxRows)

	rows := strings.Split(base, "\n")
	for len(rows) < m.height {
		rows = append(rows, "")
	}
	x := max(0, (m.width-boxW)/2)
	y := max(0, (m.height-boxH)/2)
	for i, br := range boxRows {
		if y+i >= len(rows) {
			break
		}
		head := ansi.Truncate(rows[y+i], x, "")
		head += strings.Repeat(" ", max(0, x-ansi.StringWidth(head)))
		tail := ansi.Cut(rows[y+i], x+boxW, m.width)
		rows[y+i] = head + br + tail
	}
	return strings.Join(rows[:m.height], "\n")
}

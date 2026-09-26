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

// overlay is the shared wrapped-scrollable modal component: sanitized
// text lines and the scroll offset in wrapped rows. The Issue #9
// diagnostics overlay and the Issue #31 help overlay are both
// instances — they differ only in content, key contract, and what a
// dismissal means. It opens over the underlying screen — or alone over
// a blank frame in the fatal no-results case — and owns the keyboard
// while open.
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
	diags := processDiagnostic(waitErr)
	if s := strings.TrimRight(string(stderr), "\n"); s != "" {
		diags = append(diags, strings.Split(present.Diagnostic(s), "\n")...)
	}
	return append(diags, streamDiagnostics(failures, recordDiags)...)
}

// completionDiagnostics returns the diagnostic lines knowable only at
// search completion — the generated process-failure line, the
// integrity failures, and the record tallies — excluding child stderr,
// which the session collection already took line-by-line while the
// search ran.
func completionDiagnostics(waitErr error, failures, recordDiags []string) []string {
	return append(processDiagnostic(waitErr), streamDiagnostics(failures, recordDiags)...)
}

// processDiagnostic is the generated diagnostic for a fatal process
// outcome — one line naming the exit code or signal — or nothing when
// the child's exit was clean or the benign status 1.
func processDiagnostic(waitErr error) []string {
	if processFatal(waitErr) {
		return []string{fmt.Sprintf("rg failed: %s", waitErr)}
	}
	return nil
}

// streamDiagnostics escapes the stream-integrity failures and
// record-skip diagnostic lines for display and collection.
func streamDiagnostics(failures, recordDiags []string) []string {
	var diags []string
	for _, f := range failures {
		diags = append(diags, strings.Split(present.Diagnostic(f), "\n")...)
	}
	for _, d := range recordDiags {
		diags = append(diags, strings.Split(present.Diagnostic(d), "\n")...)
	}
	return diags
}

// overlayKey handles one key while the diagnostics overlay is open:
// up/down scroll, q and Esc dismiss — quitting outright when the fatal
// no-results overlay has no underlying state — ctrl+c takes the
// cancellation path, and every other key is ignored.
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
	case "up", "down":
		scrollOverlay(m.overlay, key, m.width, m.height)
	}
	return m, nil
}

// scrollOverlay applies the shared overlay scroll keys to o: up/down
// move the rendered-row offset, clamped at both ends. Any other key is
// a no-op — each modal's own key handler decides dismissal.
func scrollOverlay(o *overlay, key string, w, h int) {
	switch key {
	case "up":
		if o.scroll > 0 {
			o.scroll--
		}
	case "down":
		if o.scroll < o.maxScroll(w, h) {
			o.scroll++
		}
	}
}

// layout resolves the overlay's geometry for a frame of w×h cells: the
// lines hard-wrapped to the interior width, the interior height, and
// the scroll offset clamped into range.
func (o overlay) layout(w, h int) (lines []string, interiorH, scroll int) {
	iw := w - 2
	if iw < 1 {
		iw = 1
	}
	maxLine := 0
	for _, l := range o.lines {
		if n := ansi.StringWidth(l); n > maxLine {
			maxLine = n
		}
	}
	if maxLine < iw {
		iw = maxLine
	}
	if iw < 1 {
		iw = 1
	}
	for _, l := range o.lines {
		for _, wl := range strings.Split(ansi.Wrap(l, iw, ""), "\n") {
			lines = append(lines, wl)
		}
	}
	interiorH = h - 2
	if interiorH < 1 {
		interiorH = 1
	}
	if len(lines) < interiorH {
		interiorH = len(lines)
	}
	scroll = o.scroll
	if max := len(lines) - interiorH; scroll > max {
		scroll = max
	}
	if scroll < 0 {
		scroll = 0
	}
	return lines, interiorH, scroll
}

// maxScroll is the largest scroll offset that still shows content —
// the down key's clamp.
func (o overlay) maxScroll(w, h int) int {
	lines, interiorH, _ := o.layout(w, h)
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

// openOverlay opens the diagnostics overlay over lines — or appends
// them to the open one, preserving the reader's scroll position — and
// cancels any active pop-up; a dismissed overlay never restores one.
func (m *Model) openOverlay(lines []string) {
	if m.overlay == nil {
		m.overlay = &overlay{lines: lines}
	} else {
		m.overlay.lines = append(m.overlay.lines, lines...)
	}
	m.popupID = 0
}

// renderOverlay composites the modal box over the base frame: the
// visible slice of o's wrapped lines inside a single-line border
// painted in the base colours, centred on the frame.
func (m Model) renderOverlay(base string, o overlay) string {
	lines, interiorH, scroll := o.layout(m.width, m.height)
	visible := lines[scroll : scroll+interiorH]
	return m.composite(base, m.theme.Overlay(visible))
}

// composite paints a box centred over the base frame, clipped to the
// frame: each box row splices over the base row at the centred
// position, preserving the base's head and tail. A box wider than the
// frame truncates rather than overflowing.
func (m Model) composite(base, box string) string {
	boxRows := strings.Split(box, "\n")
	boxW := 0
	for _, r := range boxRows {
		boxW = max(boxW, ansi.StringWidth(r))
	}
	boxW = min(boxW, m.width)

	rows := strings.Split(base, "\n")
	for len(rows) < m.height {
		rows = append(rows, "")
	}
	x := max(0, (m.width-boxW)/2)
	y := max(0, (m.height-len(boxRows))/2)
	for i, br := range boxRows {
		if y+i >= len(rows) {
			break
		}
		br = ansi.Truncate(br, m.width-x, "")
		w := ansi.StringWidth(br)
		head := ansi.Truncate(rows[y+i], x, "")
		head += strings.Repeat(" ", max(0, x-ansi.StringWidth(head)))
		tail := ansi.Cut(rows[y+i], x+w, m.width)
		rows[y+i] = head + br + tail
	}
	return strings.Join(rows[:m.height], "\n")
}

package app

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"vrg/internal/present"
	"vrg/internal/searchindex"
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

// outcomeInput is everything a completed search contributes to the
// outcome decision and the diagnostic composition: the child's wait
// error and collected stderr, the index's structured integrity causes,
// the retained-stop count assessed after all filtering, the malformed
// and oversized skip tallies, the per-path oversized detail lines, and
// the unrecognised-type tally.
type outcomeInput struct {
	waitErr        error
	stderr         []byte
	causes         []searchindex.Cause
	usable         int
	malformed      int
	oversized      int
	oversizedDiags []string
	unknown        int
}

// decideOutcome maps one completed search to its presentation and fixed
// exit status. Any diagnostics make the outcome overlay-bearing; a
// fatal outcome — a fatal process result, any integrity cause, or
// record loss that left zero usable results — fixes status 2, usable
// results fix 0, and an intact empty stream fixes 1. Unknown-type
// warnings are diagnostics only and never turn fatal on their own.
func decideOutcome(in outcomeInput) outcome {
	fatal := processFatal(in.waitErr) || len(in.causes) > 0 ||
		(in.malformed+in.oversized > 0 && in.usable == 0)
	var o outcome
	o.diags = composeDiagnostics(in)
	o.overlay = len(o.diags) > 0
	switch {
	case fatal:
		o.code = 2
	case in.usable > 0:
		o.code = 0
	default:
		o.code = 1
	}
	switch {
	case in.usable > 0:
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

// composeDiagnostics assembles the universal ordered component list
// that feeds both the overlay and, through the session collection, the
// exit replay — one composition, two sinks. The order is the Issue #36
// contract: the process component first — the child's sanitized stderr
// in collection order, or the generated exit-code/signal line only
// when the process result was fatal and stderr carried nothing — then
// the integrity causes, then the record-loss components (malformed
// aggregate, oversized aggregate, per-path oversized details), then
// the unrecognised-type warnings. A clean exit or the benign status 1
// never produces a process-status line.
func composeDiagnostics(in outcomeInput) []string {
	var diags []string
	if s := strings.TrimRight(string(in.stderr), "\n"); s != "" {
		diags = append(diags, strings.Split(present.Diagnostic(s), "\n")...)
	}
	return append(diags, completionDiagnostics(in)...)
}

// completionDiagnostics returns the diagnostic lines knowable only at
// search completion — the same composition minus the child stderr,
// which the session collection already took line-by-line while the
// search ran: the generated process-failure line when a fatal process
// result carried no explanatory stderr, the integrity causes, the
// malformed and oversized aggregates, the per-path oversized details,
// and the unrecognised-type warnings.
func completionDiagnostics(in outcomeInput) []string {
	var diags []string
	if processFatal(in.waitErr) &&
		strings.TrimSpace(string(in.stderr)) == "" {
		diags = append(diags, fmt.Sprintf("rg failed: %s", in.waitErr))
	}
	for _, c := range in.causes {
		diags = append(diags, c.Line())
	}
	if in.malformed > 0 {
		diags = append(diags, fmt.Sprintf("%d malformed record%s skipped",
			in.malformed, plural(in.malformed)))
	}
	if in.oversized > 0 {
		diags = append(diags, fmt.Sprintf("%d oversized record%s skipped",
			in.oversized, plural(in.oversized)))
	}
	diags = append(diags, in.oversizedDiags...)
	if in.unknown > 0 {
		diags = append(diags, fmt.Sprintf("%d unrecognised record types skipped",
			in.unknown))
	}
	return diags
}

// plural is the English plural suffix for a count's noun.
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
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

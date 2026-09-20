package app

import (
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/clipperhouse/displaywidth"
)

// scrollOverlay is the shared wrapped-scrollable modal overlay
// component: lines of safe text — escaped diagnostics for the error
// overlay, the binding table for help — presented in a full-width,
// single-line-bordered box centred over the underlying screen. scroll
// is the index of the first wrapped interior row shown, so the complete
// content is reachable with up/down even when it does not fit the
// terminal.
type scrollOverlay struct {
	lines  []string
	scroll int
}

// updateOverlay applies one key to the open error overlay: up/down
// scroll the wrapped diagnostic rows (clamped to the complete row set),
// q and Esc dismiss — exiting outright when the fatal outcome left no
// underlying state — and every other key is ignored.
func (m Model) updateOverlay(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	o := m.overlay
	switch key := msg.String(); key {
	case "up", "down":
		o.scrollKey(key, m.width, m.height)
	case "q", "esc":
		if m.state == stateFatal {
			// The fatal no-results overlay has no underlying state to
			// return to; dismissal is the exit, at the fixed status.
			return m, tea.Quit
		}
		m.overlay = nil
	}
	return m, nil
}

// scrollKey applies the overlay scroll keys: up moves one row toward
// the head, down one toward the tail, clamped to the complete wrapped
// row set at the current terminal size. Any other key is a no-op.
func (o *scrollOverlay) scrollKey(key string, w, h int) {
	switch key {
	case "up":
		if o.scroll > 0 {
			o.scroll--
		}
	case "down":
		if max := o.maxScroll(w, h); o.scroll < max {
			o.scroll++
		}
	}
}

// append adds one line to the overlay without moving the reader:
// scroll names the first shown wrapped row and a new line extends only
// the tail of the row set, so the position holds. The error overlay
// uses it for appended diagnostics — Issue 26 owns this minimal
// append-preserving-scroll primitive; Issue 32 generalizes it to all
// appended errors.
func (o *scrollOverlay) append(line string) {
	o.lines = append(o.lines, line)
}

// rows is the overlay's complete scrollable row set: every line wrapped
// at interior width w.
func (o *scrollOverlay) rows(w int) []string {
	if w < 1 {
		w = 1
	}
	var out []string
	for _, l := range o.lines {
		out = append(out, wrapCells(l, w)...)
	}
	return out
}

// maxScroll is the largest first-row index whose window still shows
// rows: the wrapped row count minus the interior height the terminal
// affords.
func (o *scrollOverlay) maxScroll(w, h int) int {
	n := len(o.rows(w - 2))
	max := n - interiorHeight(h, n)
	if max < 0 {
		return 0
	}
	return max
}

// interiorHeight is the box's interior row count: the wrapped row count,
// capped by the terminal height minus the two border rows.
func interiorHeight(h, rows int) int {
	ih := h - 2
	if ih > rows {
		ih = rows
	}
	if ih < 1 {
		ih = 1
	}
	return ih
}

// overlayScreen composites overlay o over base: the bordered box
// replaces the centre screen rows wholesale, leaving the rows above and
// below the underlying screen visible. At tiny sizes the box is clipped
// to the terminal — there is no borderless mode — and growth restores
// the normal layout.
func (m Model) overlayScreen(base string, o *scrollOverlay) string {
	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	rows := strings.Split(base, "\n")
	for len(rows) < h {
		rows = append(rows, "")
	}
	if len(rows) > h {
		rows = rows[:h]
	}

	iw := w - 2
	if iw < 1 {
		iw = 1
	}
	all := o.rows(iw)
	ih := interiorHeight(h, len(all))
	scroll := o.scroll
	if max := len(all) - ih; scroll > max {
		scroll = max
	}
	if scroll < 0 {
		scroll = 0
	}

	rule := strings.Repeat("─", iw)
	box := make([]string, 0, ih+2)
	box = append(box, m.theme.Base("┌"+rule+"┐"))
	for _, r := range all[scroll : scroll+ih] {
		box = append(box, m.theme.Base("│"+padCells(r, iw)+"│"))
	}
	box = append(box, m.theme.Base("└"+rule+"┘"))

	top := (h - len(box)) / 2
	if top < 0 {
		top = 0
	}
	for i, r := range box {
		if top+i < len(rows) {
			rows[top+i] = r
		}
	}
	return strings.Join(rows, "\n")
}

// wrapCells splits s into rows of at most w terminal cells, breaking at
// grapheme boundaries — long unbroken diagnostics wrap inside the border
// rather than clipping. An empty line yields one empty row. A grapheme
// wider than w is emitted whole rather than looping forever — only a
// degenerate interior narrower than two cells can produce that.
func wrapCells(s string, w int) []string {
	var rows []string
	for len(s) > 0 {
		take := displaywidth.TruncateString(s, w, "")
		if take == "" {
			_, size := utf8.DecodeRuneInString(s)
			take = s[:size]
		}
		rows = append(rows, take)
		s = s[len(take):]
	}
	if len(rows) == 0 {
		return []string{""}
	}
	return rows
}

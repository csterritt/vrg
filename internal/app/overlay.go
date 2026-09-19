package app

import (
	"strings"

	"github.com/rivo/uniseg"

	"vrg/internal/safepresentation"
)

// scrollBox is the wrapped, scrollable overlay component the error
// overlay (Issue #9) and the help dialog (Issue #31) share: text
// re-wraps into interior-width rows on every render — unbroken strings
// split mid-run so no row exceeds the interior — and scroll is the
// first visible wrapped row, clamped to the complete row set so both
// ends of an oversized body stay reachable.
type scrollBox struct {
	text   string
	scroll int
}

// rows wraps the box text into rows of at most w cells: each input
// line splits at the width on grapheme-cluster boundaries, an unbroken
// string splits mid-run, and an empty input still yields one row.
func (s *scrollBox) rows(w int) []string {
	var rows []string
	for _, line := range strings.Split(s.text, "\n") {
		rows = append(rows, wrapCells(line, w)...)
	}
	return rows
}

// scrollBy moves the first-visible-row index by d, clamped to the
// scrollable range of the complete wrapped body.
func (s *scrollBox) scrollBy(d, w, vis int) {
	s.scroll += d
	s.clamp(w, vis)
}

// clamp keeps the first visible row inside the scrollable range —
// also after a resize changes the row count or visible height.
func (s *scrollBox) clamp(w, vis int) {
	max := len(s.rows(w)) - vis
	if max < 0 {
		max = 0
	}
	if s.scroll > max {
		s.scroll = max
	}
	if s.scroll < 0 {
		s.scroll = 0
	}
}

// overlayInteriorWidth is the overlay's content width in cells: the
// frame width minus the two border columns and their padding.
func (m *model) overlayInteriorWidth() int {
	if w := m.width - 4; w > 1 {
		return w
	}
	return 1
}

// overlayVisible is the number of body rows the box shows at once:
// the frame height minus the two border rows.
func (m *model) overlayVisible() int {
	if v := m.height - 2; v > 0 {
		return v
	}
	return 0
}

// wrapCells splits s into rows of at most w cells on grapheme-cluster
// boundaries; an empty input still yields one (empty) row.
func wrapCells(s string, w int) []string {
	if w < 1 {
		w = 1
	}
	var rows []string
	var b strings.Builder
	col := 0
	rest := s
	state := -1
	for len(rest) > 0 {
		var cl string
		var cw int
		cl, rest, cw, state = uniseg.FirstGraphemeClusterInString(rest, state)
		if col+cw > w && col > 0 {
			rows = append(rows, b.String())
			b.Reset()
			col = 0
		}
		b.WriteString(cl)
		col += cw
	}
	if b.Len() > 0 || len(rows) == 0 {
		rows = append(rows, b.String())
	}
	return rows
}

// openOverlay opens the modal error overlay over the current state —
// or appends a new diagnostic to it, preserving the reader's scroll
// position, when it is already open (the PRD's new-errors-append
// rule). Opening it cancels any file-change pop-up, which never
// returns after the overlay closes.
func (m *model) openOverlay(text string, dismissExits bool) {
	was := m.overlayOpen
	if was {
		m.overlay.text += "\n" + text
	} else {
		m.overlay.text = text
		m.overlay.scroll = 0
	}
	m.overlayOpen = true
	m.overlayExit = dismissExits
	m.popupID = 0
	if was {
		m.ack("overlay:append")
	} else {
		m.ack("overlay:open")
	}
}

// overlayRows is the error overlay's complete wrapped row set.
func (m *model) overlayRows() []string {
	return m.overlay.rows(m.overlayInteriorWidth())
}

// scrollOverlay moves the error overlay's first-visible-row index by
// d, clamped to the scrollable range of the complete wrapped
// diagnostic.
func (m *model) scrollOverlay(d int) {
	m.overlay.scrollBy(d, m.overlayInteriorWidth(), m.overlayVisible())
}

// clampOverlayScroll keeps the error overlay's first visible row
// inside the scrollable range — also after a resize changes the row
// count or visible height.
func (m *model) clampOverlayScroll() {
	m.overlay.clamp(m.overlayInteriorWidth(), m.overlayVisible())
}

// compositeBox draws one scrollBox's open overlay over the base frame:
// the single-line bordered box, centered on the frame, carrying the
// visible window of wrapped rows in the base colours. At tiny sizes
// the box clips to the terminal — there is no borderless mode; growth
// restores the normal layout.
func (m *model) compositeBox(base string, box *scrollBox) string {
	w, h := m.width, m.height
	if w <= 0 || h <= 0 {
		return base
	}
	rows := strings.Split(base, "\n")
	for len(rows) < h {
		rows = append(rows, "")
	}
	if len(rows) > h {
		rows = rows[:h]
	}

	all := box.rows(m.overlayInteriorWidth())
	vis := m.overlayVisible()
	if vis > len(all) {
		vis = len(all)
	}
	scroll := box.scroll
	if max := len(all) - vis; scroll > max {
		scroll = max
	}
	if scroll < 0 {
		scroll = 0
	}
	interior := m.overlayInteriorWidth()
	visible := all[scroll : scroll+vis]
	padded := make([]string, len(visible))
	maxw := 0
	for i, r := range visible {
		padded[i] = padTo(r, interior)
		if cw := safepresentation.CellWidth(padded[i]); cw > maxw {
			maxw = cw
		}
	}
	boxRows := m.theme.Overlay(padded)
	boxW := maxw + 4
	top := 0
	if len(boxRows) < h {
		top = (h - len(boxRows)) / 2
	}
	left := 0
	if boxW < w {
		left = (w - boxW) / 2
	}
	for i, brow := range boxRows {
		r := top + i
		if r >= h {
			break
		}
		right := w - left - boxW
		if right < 0 {
			right = 0
		}
		rows[r] = strings.Repeat(" ", left) + brow + strings.Repeat(" ", right)
	}
	return strings.Join(rows, "\n")
}

package app

import (
	"strings"

	"github.com/rivo/uniseg"

	"vrg/internal/safepresentation"
)

// overlayInteriorWidth is the overlay's content width in cells: the
// frame width minus the two border columns and their padding.
func (m *model) overlayInteriorWidth() int {
	if w := m.width - 4; w > 1 {
		return w
	}
	return 1
}

// overlayVisible is the number of diagnostic rows the box shows at
// once: the frame height minus the two border rows.
func (m *model) overlayVisible() int {
	if v := m.height - 2; v > 0 {
		return v
	}
	return 0
}

// overlayRows wraps the escaped diagnostic text into interior-width
// rows on grapheme boundaries: lines split at the width, and an
// unbroken string splits mid-run so no row exceeds the interior. The
// complete wrapped set stays in the model — scrolling reaches both
// ends of a diagnostic larger than the frame.
func (m *model) overlayRows() []string {
	w := m.overlayInteriorWidth()
	var rows []string
	for _, line := range strings.Split(m.overlayText, "\n") {
		rows = append(rows, wrapCells(line, w)...)
	}
	return rows
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

// scrollOverlay moves the first-visible-row index by d, clamped to the
// scrollable range of the complete wrapped diagnostic.
func (m *model) scrollOverlay(d int) {
	m.overlayScroll += d
	m.clampOverlayScroll()
}

// clampOverlayScroll keeps the first visible row inside the scrollable
// range — also after a resize changes the row count or visible height.
func (m *model) clampOverlayScroll() {
	max := len(m.overlayRows()) - m.overlayVisible()
	if max < 0 {
		max = 0
	}
	if m.overlayScroll > max {
		m.overlayScroll = max
	}
	if m.overlayScroll < 0 {
		m.overlayScroll = 0
	}
}

// compositeOverlay draws the open overlay over the base frame: the
// single-line bordered box, centered on the frame, carrying the visible
// window of wrapped diagnostic rows in the base colours.
func (m *model) compositeOverlay(base string) string {
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

	all := m.overlayRows()
	vis := m.overlayVisible()
	if vis > len(all) {
		vis = len(all)
	}
	scroll := m.overlayScroll
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
	box := m.theme.Overlay(padded)
	boxW := maxw + 4
	top := 0
	if len(box) < h {
		top = (h - len(box)) / 2
	}
	left := 0
	if boxW < w {
		left = (w - boxW) / 2
	}
	for i, brow := range box {
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

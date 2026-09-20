package app

import (
	"fmt"
	"strings"

	"github.com/clipperhouse/displaywidth"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
	"vrg/internal/viewport"
)

// browseScreen composes the two-pane browse view: the raw-path-ordered
// file list on the left and the current file's panel on the right —
// filename rule, then guttered content rows or the load placeholder.
func (m Model) browseScreen() string {
	w, h := m.termSize()
	names := make([]string, len(m.files))
	for i, f := range m.files {
		names[i] = safepresentation.EscapePath(f)
	}
	curIdx := -1
	var curPath []byte
	if stop, ok := m.index.Current(); ok {
		curPath = stop.Path
		curIdx = m.fileIdx[string(curPath)]
	}
	lw := listWidth(names, w)

	// Keep the current entry inside the scrolled list window.
	listTop := m.listTop
	if curIdx >= 0 {
		if curIdx < listTop {
			listTop = curIdx
		}
		if curIdx >= listTop+h {
			listTop = curIdx - h + 1
		}
	}

	panelW := w - lw
	rows := make([]string, h)
	for r := 0; r < h; r++ {
		rows[r] = m.theme.Base(m.listRow(names, listTop+r, curIdx, lw) +
			m.panelRow(r, curPath, names, curIdx, panelW, h-1))
	}
	return strings.Join(rows, "\n")
}

// listWidth is the Issue 5 heuristic for the file-list column: the
// longest escaped path plus padding, capped at 40% of the terminal and
// narrowed further to leave ten cells of panel. Issue 24 owns the real
// formula.
func listWidth(names []string, w int) int {
	if len(names) == 0 {
		return 0
	}
	longest := 0
	for _, n := range names {
		if d := displaywidth.String(n); d > longest {
			longest = d
		}
	}
	lw := longest + 2
	if cap := w * 2 / 5; lw > cap {
		lw = cap
	}
	if w-lw < 10 {
		lw = w - 10
		if lw < 0 {
			lw = 0
		}
	}
	return lw
}

// listRow renders file-list row fi padded to lw cells; the current entry
// is underlined. Rows beyond the list render blank.
func (m Model) listRow(names []string, fi, curIdx, lw int) string {
	if lw <= 0 {
		return ""
	}
	if fi >= len(names) {
		return strings.Repeat(" ", lw)
	}
	name := truncateLeft(names[fi], lw)
	pad := lw - displaywidth.String(name)
	if pad < 0 {
		pad = 0
	}
	if fi == curIdx {
		return m.theme.CurrentFile(name) + strings.Repeat(" ", pad)
	}
	return m.theme.FileList(name + strings.Repeat(" ", pad))
}

// panelRow renders row r of the file panel: the filename rule, then
// content rows of the loaded buffer or the load placeholder.
func (m Model) panelRow(r int, curPath []byte, names []string, curIdx, panelW, contentH int) string {
	if panelW <= 0 {
		return ""
	}
	name := ""
	if curIdx >= 0 {
		name = names[curIdx]
	}
	if r == 0 {
		return m.theme.FilenameRule(filenameRule(name, panelW))
	}
	key := string(curPath)
	src := m.buffers[key]
	if src == nil {
		if r == 1 {
			text := "Loading…"
			if m.failed[key] {
				text = "(unreadable)"
			}
			return padCells(clipCells(text, panelW), panelW)
		}
		return padCells("", panelW)
	}
	// The stored extent is maintained on load and on resize; the painted
	// top is clamped here as well so a stale extent cannot paint
	// avoidable blank rows below EOF. View must not mutate the saved
	// state, so this clamp stays local to the frame.
	top := 0
	if vp := m.vps[key]; vp != nil {
		top = vp.Top()
	}
	if max := src.LineCount() - contentH; top > max {
		top = max
	}
	if top < 0 {
		top = 0
	}
	rowIdx := top + r - 1
	if rowIdx >= src.LineCount() {
		return padCells("", panelW)
	}
	gw := src.GutterWidth()
	line, first := src.RowLine(rowIdx)
	gutter := strings.Repeat(" ", gw)
	if first {
		gutter = fmt.Sprintf("%*d  ", gw-2, line+1)
	}
	if gw >= panelW {
		return padCells(clipCells(gutter, panelW), panelW)
	}
	tw := viewport.TextWidth(panelW, gw, src.Key().Wrap)
	text, painted := m.renderCells(src.Cells(rowIdx), src.Highlights(rowIdx), tw, m.isCurrentLine(line))
	row := m.theme.Gutter(gutter) + text
	if pad := tw - painted; pad > 0 {
		row += strings.Repeat(" ", pad)
	}
	if !src.Key().Wrap {
		// The reserved right-indicator column stays blank until
		// Issue 20 populates it.
		row += " "
	}
	return row
}

// isCurrentLine reports whether 0-based source line i is the cursor's
// matched line; the panel only ever renders the cursor's file.
func (m Model) isCurrentLine(i int) bool {
	stop, ok := m.index.Current()
	return ok && stop.Line == int64(i)+1
}

// renderCells renders up to w terminal cells of one rendered row,
// painting the highlight spans in inverse video — additionally
// underlined when the line is the current matched line — and reports
// the painted cell width so the caller can pad the row. Truncation
// stops before a cell that would overflow the width rather than
// painting half a wide glyph.
func (m Model) renderCells(cells []safepresentation.Cell, spans []filebuffer.Span, w int, current bool) (string, int) {
	n := 0
	painted := 0
	for n < len(cells) {
		cw := displaywidth.String(cells[n].Text)
		if painted+cw > w {
			break
		}
		painted += cw
		n++
	}
	if n <= 0 {
		return "", 0
	}
	hl := make([]bool, n)
	for _, s := range spans {
		for i := s.Start; i < s.End && i < n; i++ {
			if i >= 0 {
				hl[i] = true
			}
		}
	}
	var b strings.Builder
	for i := 0; i < n; {
		j := i + 1
		for j < n && hl[j] == hl[i] {
			j++
		}
		var seg strings.Builder
		for k := i; k < j; k++ {
			seg.WriteString(cells[k].Text)
		}
		text := seg.String()
		if hl[i] {
			if current {
				b.WriteString(m.theme.CurrentMatch(text))
			} else {
				b.WriteString(m.theme.Match(text))
			}
		} else {
			b.WriteString(text)
		}
		i = j
	}
	return b.String(), painted
}

// filenameRule embeds the current file's escaped path in a horizontal
// rule across the panel: "── name " followed by rule fill.
func filenameRule(name string, w int) string {
	const prefix = "── "
	if w <= 0 {
		return ""
	}
	avail := w - displaywidth.String(prefix)
	if avail < 1 {
		return strings.Repeat("─", w)
	}
	name = truncateLeft(name, avail-1) // leave room for the trailing space
	s := prefix + name + " "
	if d := displaywidth.String(s); d < w {
		s += strings.Repeat("─", w-d)
	}
	return s
}

// truncateLeft keeps the widest suffix of s fitting w cells, marked with
// a leading ellipsis; unchanged when s already fits.
func truncateLeft(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if displaywidth.String(s) <= w {
		return s
	}
	runes := []rune(s)
	width := 1 // the ellipsis
	i := len(runes)
	for i > 0 {
		rw := displaywidth.Rune(runes[i-1])
		if width+rw > w {
			break
		}
		width += rw
		i--
	}
	return "…" + string(runes[i:])
}

// clipCells truncates s to w terminal cells.
func clipCells(s string, w int) string {
	if w <= 0 {
		return ""
	}
	return displaywidth.TruncateString(s, w, "")
}

// padCells pads s — a plain, unstyled string — out to w terminal cells.
func padCells(s string, w int) string {
	if d := displaywidth.String(s); d < w {
		s += strings.Repeat(" ", w-d)
	}
	return s
}

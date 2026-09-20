package app

import (
	"fmt"
	"strings"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
	"vrg/internal/viewport"
)

// browseScreen composes the two-pane browse view: the raw-path-ordered
// file list on the left and the current file's panel on the right —
// filename rule, then guttered content rows or the load placeholder.
// The frame queries only the visible window: list entries come through
// the item provider and panel rows through the installed row model.
func (m Model) browseScreen() string {
	w, h := m.termSize()
	curIdx := -1
	var curPath []byte
	if stop, ok := m.index.Current(); ok {
		curPath = stop.Path
		curIdx = m.fileIdx[string(curPath)]
	}
	lw := m.listWidth(w)

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
		rows[r] = m.theme.Base(m.listRow(listTop+r, curIdx, lw) +
			m.panelRow(r, curPath, curIdx, panelW, h-1))
	}
	return strings.Join(rows, "\n")
}

// listItem renders file-list entry i's display name left-truncated to w
// cells: the test-injected provider's answer truncated on the fly, or
// the precomputed measured name's grapheme-safe suffix. Either way the
// work is this one row's — never a pass over the list.
func (m Model) listItem(i, w int) string {
	if m.itemName != nil {
		return safepresentation.TruncateLeftGrapheme(m.itemName(i), w)
	}
	return m.files[i].name.TruncateLeft(w)
}

// listColumn is the Issue 24 file-list column width in cells: zero
// when the list is hidden or has no entries, otherwise the nonnegative
// minimum of the longest sanitized path width plus two, floor(0.40 ×
// terminal width), and the terminal width minus (gutter width + 10 +
// reserved indicator width). The 10 reserves the panel's minimum text
// width; a computed zero draws no cells but is not a hidden list —
// visible is the user's preference, carried separately.
func listColumn(visible bool, widest, w, gutter, reserved int) int {
	if !visible || widest <= 0 {
		return 0
	}
	lw := widest
	if cap := w * 2 / 5; lw > cap {
		lw = cap
	}
	if max := w - (gutter + 10 + reserved); lw > max {
		lw = max
	}
	if lw < 0 {
		lw = 0
	}
	return lw
}

// listWidth applies the Issue 24 formula at the current geometry: the
// widest entry was measured once at browse entry — a frame render
// never rescans the list — while the gutter, reserved column, and
// terminal width are read live so loading, mode, and size changes
// recompute the column.
func (m Model) listWidth(w int) int {
	reserved := 0
	if !m.wrap {
		reserved = 1
	}
	return listColumn(m.listVisible, m.listWidest, w, m.gutterWidth(), reserved)
}

// gutterWidth is the gutter the list-width formula and the panel's
// text width share: the current file's gutter once its content is
// prepared, or the placeholder minimum — one digit slot plus two
// spaces — while it is not.
func (m Model) gutterWidth() int {
	if src := m.sources[m.curKey()]; src != nil {
		return src.GutterWidth()
	}
	return 3
}

// listRow renders file-list row fi padded to lw cells; the current entry
// is underlined. Rows beyond the list render blank.
func (m Model) listRow(fi, curIdx, lw int) string {
	if lw <= 0 {
		return ""
	}
	if fi >= len(m.files) {
		return strings.Repeat(" ", lw)
	}
	name := m.listItem(fi, lw)
	pad := lw - safepresentation.CellWidth(name)
	if pad < 0 {
		pad = 0
	}
	if fi == curIdx {
		return m.theme.CurrentFile(name) + strings.Repeat(" ", pad)
	}
	return m.theme.FileList(name + strings.Repeat(" ", pad))
}

// panelRow renders row r of the file panel: the filename rule, then
// content rows of the installed row model or the load placeholder.
func (m Model) panelRow(r int, curPath []byte, curIdx, panelW, contentH int) string {
	if panelW <= 0 {
		return ""
	}
	key := string(curPath)
	name := ""
	if curIdx >= 0 {
		name = m.files[curIdx].name.String()
	}
	if r == 0 {
		return m.theme.FilenameRule(filenameRule(name, m.notes[key], panelW))
	}
	src := m.buffers[key]
	_, un := m.unsupported[key]
	if src == nil || un {
		if r == 1 {
			// A load in flight reads "Loading…" even for a previously
			// failed or unsupported file: its retry or reload is what
			// settles the placeholder to content, "(unreadable)", or
			// "(unsupported encoding)". The detected buffer's installed
			// layout has no rows, so the placeholder stands in.
			text := "Loading…"
			if _, flying := m.loading[key]; !flying {
				if _, bad := m.failed[key]; bad {
					text = "(unreadable)"
				} else if un {
					text = "(unsupported encoding)"
				}
			}
			return padCells(clipCells(text, panelW), panelW)
		}
		return padCells("", panelW)
	}
	// The installed row model and its saved viewport are reconciled at
	// install time; the painted top is clamped here as well so a stale
	// extent cannot paint avoidable blank rows below EOF. View must not
	// mutate the saved state, so this clamp stays local to the frame.
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
	if gw >= panelW {
		gutter := strings.Repeat(" ", gw)
		if first {
			gutter = fmt.Sprintf("%*d  ", gw-2, line+1)
		}
		return padCells(clipCells(gutter, panelW), panelW)
	}
	tw := viewport.TextWidth(panelW, gw, src.Key().Wrap)
	hoff := 0
	if vp := m.vps[key]; vp != nil {
		hoff = vp.Offset()
	}
	var hid viewport.Hidden
	if !src.Key().Wrap {
		hid = src.Hidden(rowIdx, hoff, tw)
	}
	cells, spans, clusters := src.Clip(rowIdx, hoff, tw)
	current := m.isCurrentLine(line)
	text, painted := m.renderCells(cells, clusters, spans, tw, current)
	row := m.gutterFor(first, line, gw, hid) + text
	if pad := tw - painted; pad > 0 {
		row += strings.Repeat(" ", pad)
	}
	if !src.Key().Wrap {
		// The reserved rightmost column carries an inverse * on the
		// current matched line's row when a match is entirely hidden
		// right. The text width excludes it, so it never overwrites
		// text; a current line scrolled off-screen simply has no row
		// to carry it.
		if current && hid.RightMatch {
			row += m.theme.Indicator("*")
		} else {
			row += " "
		}
	}
	return row
}

// gutterFor renders one rendered row's gutter: the right-justified
// line number followed by two spaces, the first of which is the
// run-off-edge hidden-left indicator — an inverse * when a match or
// marker on the line is entirely hidden left, an inverse _ when text
// is hidden left, blank otherwise. Continuation rows carry a blank
// gutter, and wrap mode reports no hidden content.
func (m Model) gutterFor(first bool, line, gw int, hid viewport.Hidden) string {
	if !first {
		return m.theme.Gutter(strings.Repeat(" ", gw))
	}
	num := fmt.Sprintf("%*d", gw-2, line+1)
	ind := " "
	if hid.LeftMatch {
		ind = "*"
	} else if hid.LeftText {
		ind = "_"
	}
	if ind == " " {
		return m.theme.Gutter(num + "  ")
	}
	return m.theme.Gutter(num) + m.theme.Indicator(ind) + m.theme.Gutter(" ")
}

// isCurrentLine reports whether 0-based source line i is the cursor's
// matched line; the panel only ever renders the cursor's file.
func (m Model) isCurrentLine(i int) bool {
	stop, ok := m.index.Current()
	return ok && stop.Line == int64(i)+1
}

// renderCells renders up to w terminal cells of one clipped row,
// painting the highlight spans in inverse video — additionally
// underlined when the line is the current matched line — and reports
// the painted cell width so the caller can pad the row. Geometry comes
// from the row's clusters, not from re-measuring cell text: a cluster
// that would overflow the width is never painted — never half-drawn —
// and a cluster's cells always share one styling run.
func (m Model) renderCells(cells []safepresentation.Cell, clusters []safepresentation.Cluster, spans []filebuffer.Span, w int, current bool) (string, int) {
	n := 0
	painted := 0
	for _, c := range clusters {
		if painted+c.Width > w {
			break
		}
		painted += c.Width
		n = c.End
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
// rule across the panel: "── name " followed by rule fill, with the
// buffer-status note — when the file has one — closing the row. The
// path truncates to make room for the note where possible, and at
// least one rule cell separates name from note. When the fixed parts
// alone exceed the width the row clips rather than overflows.
func filenameRule(name, note string, w int) string {
	const prefix = "── "
	if w <= 0 {
		return ""
	}
	trailer := ""
	if note != "" {
		trailer = " " + note + " "
	}
	avail := w - safepresentation.CellWidth(prefix) - safepresentation.CellWidth(trailer)
	if avail < 2 {
		return clipCells(prefix+name+trailer, w)
	}
	name = safepresentation.TruncateLeftGrapheme(name, avail-2) // a separator space and ≥1 rule cell
	s := prefix + name + " " + strings.Repeat("─", avail-1-safepresentation.CellWidth(name))
	return s + trailer
}

// clipCells truncates s to w terminal cells on grapheme-cluster
// boundaries.
func clipCells(s string, w int) string {
	return safepresentation.TruncateCells(s, w)
}

// padCells pads s — a plain, unstyled string — out to w terminal cells.
func padCells(s string, w int) string {
	if d := safepresentation.CellWidth(s); d < w {
		s += strings.Repeat(" ", w-d)
	}
	return s
}

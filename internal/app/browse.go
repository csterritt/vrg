package app

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/rivo/uniseg"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
)

// fileLoadedMsg delivers the prepared buffer — or the read error — for
// one requested path. The command performed the read plus decode and
// byte→cell mapping off the update path, so Update only files the
// result; it never does full-file work itself.
type fileLoadedMsg struct {
	path []byte
	buf  *filebuffer.Buffer
	err  error
}

// startLoad issues the current file's load command, or nil when the path
// is already loaded, already in flight, or already failed — a repeated
// request is dropped, never queued. The command runs the whole load
// under the test gate so "Loading…" provably spans disk read plus
// decode/map, then returns the prepared buffer keyed by raw path.
func (m *model) startLoad() tea.Cmd {
	if m.idx == nil || len(m.idx.Files) == 0 {
		return nil
	}
	f := m.idx.Files[m.cur]
	key := string(f.Path)
	if m.loading[key] || m.failed[key] {
		return nil
	}
	if _, ok := m.bufs[key]; ok {
		return nil
	}
	m.loading[key] = true
	path, stops, gate := f.Path, f.Stops, m.opts.loadGate
	return func() tea.Msg {
		if gate != nil {
			gate()
		}
		buf, err := filebuffer.Load(path, stops)
		return fileLoadedMsg{path: path, buf: buf, err: err}
	}
}

// loadDiag composes the single-line diagnostic for a failed file load:
// the path single-line-escaped so hostile name bytes can never forge a
// diagnostic line boundary, and a reason that does not repeat the raw
// path — a *PathError contributes only its cause.
func loadDiag(path []byte, err error) string {
	reason := err.Error()
	var pe *fs.PathError
	if errors.As(err, &pe) {
		reason = pe.Err.Error()
	}
	return "cannot read " + safepresentation.EscapePath(path) + ": " + safepresentation.EscapePath([]byte(reason))
}

// browseView composes the two-pane browse frame at the current
// dimensions: the file list in raw-path order on the left, the filename
// rule and file panel on the right, exactly height rows. Every sink —
// list entries, the rule, and panel content — renders only escaped
// display text from the safe-presentation core.
func (m *model) browseView() string {
	w, h := m.width, m.height
	if w <= 0 || h <= 0 || m.idx == nil {
		return ""
	}
	files := m.idx.Files
	escaped := make([]string, len(files))
	longest := 0
	for i, f := range files {
		escaped[i] = safepresentation.EscapePath(f.Path)
		if cw := safepresentation.CellWidth(escaped[i]); cw > longest {
			longest = cw
		}
	}
	// Issue #5 uses the simple heuristic width: the longest displayed
	// path plus one padding cell, never wider than the terminal.
	// Issue #24 owns the real formula (40% cap, minimum text width,
	// left truncation).
	listW := longest + 1
	if listW > w {
		listW = w
	}
	panelW := w - listW

	// The list scrolls to keep the current entry visible; it shares the
	// h-1 rows below the filename rule with the file panel.
	listTop := 0
	if m.cur >= h-1 {
		listTop = m.cur - h + 2
	}

	var buf *filebuffer.Buffer
	failed := false
	curLine := int64(-1)
	if len(files) > 0 {
		key := string(files[m.cur].Path)
		buf, failed = m.bufs[key], m.failed[key]
		// Until Issue #13's navigation, the current matched line is the
		// current file's first stop.
		if stops := files[m.cur].Stops; len(stops) > 0 {
			curLine = stops[0].Number
		}
	}

	var sb strings.Builder
	sb.WriteString(m.filenameRule(w))
	for r := 1; r < h; r++ {
		sb.WriteByte('\n')
		if listW > 0 {
			sb.WriteString(m.listCell(escaped, listTop+r-1, listW))
		}
		if panelW > 0 {
			sb.WriteString(m.contentCell(r-1, panelW, h-1, buf, failed, curLine))
		}
	}
	return sb.String()
}

// listCell renders the file-list entry at index i padded to width
// cells; the current entry is underlined.
func (m *model) listCell(escaped []string, i, width int) string {
	if i >= len(escaped) {
		return strings.Repeat(" ", width)
	}
	clipped := clipCells(escaped[i], width)
	entry := m.theme.FileList(clipped)
	if i == m.cur {
		entry = m.theme.CurrentFile(clipped)
	}
	return entry + strings.Repeat(" ", width-safepresentation.CellWidth(clipped))
}

// contentCell renders file-panel content row cr padded to width cells:
// gutter plus text for the buffer's visible lines, or the placeholder
// while no buffer is available.
func (m *model) contentCell(cr, width, avail int, buf *filebuffer.Buffer, failed bool, curLine int64) string {
	if buf == nil {
		placeholder := "Loading…"
		if failed {
			placeholder = "(unreadable)"
		}
		if cr == 0 {
			return padTo(clipCells(placeholder, width), width)
		}
		return strings.Repeat(" ", width)
	}
	lines := m.vp.Visible(buf.Lines(), avail)
	if cr >= len(lines) {
		return strings.Repeat(" ", width)
	}
	l := lines[cr]
	gutter := fmt.Sprintf("%*d  ", buf.GutterWidth()-2, l.Number)
	if len(gutter) > width {
		return m.theme.Gutter(gutter[:width])
	}
	return m.theme.Gutter(gutter) + m.contentText(l, width-len(gutter), l.Number == curLine)
}

// filenameRule renders the current file's escaped path embedded in a
// horizontal rule across the full frame width above both panes:
// "─ path ────".
func (m *model) filenameRule(width int) string {
	name := ""
	if m.idx != nil && len(m.idx.Files) > 0 {
		name = safepresentation.EscapePath(m.idx.Files[m.cur].Path)
	}
	if width <= 4 {
		return padTo(clipCells(name, width), width)
	}
	clipped := clipCells(name, width-3)
	rule := "─ " + clipped + " "
	return m.theme.FilenameRule(rule + strings.Repeat("─", width-3-safepresentation.CellWidth(clipped)))
}

// contentText renders one line's escaped cells into at most textW
// terminal cells, wrapping each maximal run of highlighted cells in the
// match style — the true inverse, additionally underlined when the line
// is the current matched line — and padding the rest with blanks.
// Clipping never splits a grapheme's cells: a cell that would cross the
// boundary ends the row.
func (m *model) contentText(l filebuffer.Line, textW int, cur bool) string {
	var sb strings.Builder
	var run strings.Builder
	runHL := false
	flush := func() {
		if run.Len() == 0 {
			return
		}
		if runHL {
			if cur {
				sb.WriteString(m.theme.CurrentMatch(run.String()))
			} else {
				sb.WriteString(m.theme.Match(run.String()))
			}
		} else {
			sb.WriteString(run.String())
		}
		run.Reset()
	}
	highlighted := func(c int) bool {
		for _, s := range l.Highlights {
			if c >= s.Start && c < s.End {
				return true
			}
		}
		return false
	}
	col := 0
	for i, c := range l.Cells {
		text := c.Text
		if text == "" {
			if i > 0 && l.Cells[i-1].Start == c.Start && l.Cells[i-1].End == c.End {
				continue // continuation cell of a wide glyph
			}
			text = string(rune(0xfffd)) // an invalid byte's replacement char
		}
		cw := safepresentation.CellWidth(text)
		if col+cw > textW {
			break
		}
		if hl := highlighted(i); hl != runHL {
			flush()
			runHL = hl
		}
		run.WriteString(text)
		col += cw
	}
	flush()
	if col < textW {
		sb.WriteString(strings.Repeat(" ", textW-col))
	}
	return sb.String()
}

// clipCells returns the longest prefix of s occupying at most w cells,
// never splitting a grapheme cluster.
func clipCells(s string, w int) string {
	if w <= 0 {
		return ""
	}
	var b strings.Builder
	col := 0
	rest := s
	state := -1
	for len(rest) > 0 {
		var cl string
		var cw int
		cl, rest, cw, state = uniseg.FirstGraphemeClusterInString(rest, state)
		if col+cw > w {
			break
		}
		b.WriteString(cl)
		col += cw
	}
	return b.String()
}

// padTo pads s with trailing spaces to width cells.
func padTo(s string, width int) string {
	if n := width - safepresentation.CellWidth(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}

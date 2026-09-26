package app

import (
	"bytes"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"vrg/internal/filebuffer"
	"vrg/internal/present"
	"vrg/internal/searchindex"
	"vrg/internal/theme"
	"vrg/internal/viewport"
)

// loadDoneMsg delivers a prepared buffer for one raw path: the worker
// has already read, split, escaped, and mapped the file, so Update
// stores the result without full-file work. The raw path bytes are the
// cache key — never a displayed form.
type loadDoneMsg struct {
	path []byte
	buf  *filebuffer.Buffer
	err  error
}

// distinctPaths returns the distinct raw paths across the index-ordered
// stops: the file list's order.
func distinctPaths(stops []searchindex.Stop) [][]byte {
	var out [][]byte
	for _, s := range stops {
		if len(out) == 0 || !bytes.Equal(out[len(out)-1], s.Path) {
			out = append(out, s.Path)
		}
	}
	return out
}

// stopsForFile returns the stops belonging to one raw path.
func stopsForFile(ix *searchindex.Index, path []byte) []searchindex.Stop {
	var out []searchindex.Stop
	for _, s := range ix.Stops() {
		if bytes.Equal(s.Path, path) {
			out = append(out, s)
		}
	}
	return out
}

// currentStop returns the cursor's stop — the index owns the
// matched-line navigation position — or false when no index has
// arrived or it holds no stops.
func (m Model) currentStop() (searchindex.Stop, bool) {
	if m.index == nil {
		return searchindex.Stop{}, false
	}
	return m.index.Current()
}

// navigate applies one matched-line cursor step: n advances and p
// retreats, both circularly. The zero- and one-stop indexes are strict
// no-ops — no command, no state change. A step within the same file
// changes only the current-line styling; destination reveal is Issue
// #14's. A step into another file switches the panel: the departing
// file's top row joins its saved state, the viewport reinstalls the
// new file's prepared rows at its own saved top — top of file on a
// first visit — and its load is requested when the file is neither
// cached nor in flight nor already failed. The file list needs no
// wiring: its underlined entry derives from the cursor.
func (m Model) navigate(forward bool) (Model, tea.Cmd) {
	depart := m.currentPath()
	var step searchindex.Step
	if forward {
		step = m.index.Next()
	} else {
		step = m.index.Prev()
	}
	if !step.Moved {
		return m, nil
	}
	if step.FileChanged {
		if depart != nil {
			m.saved[string(depart)] = m.vp.Top()
		}
		m.relayout()
		// The new file starts at its saved vertical state — absent
		// means a first visit, which starts at the top. A load
		// completing for it later applies the same restore, and
		// Issue #14's destination reveal overrides both.
		m.vp.SetTop(m.saved[string(step.Stop.Path)])
	}
	return m, m.ensureLoad()
}

// currentPath returns the raw path of the current file, or nil.
func (m Model) currentPath() []byte {
	if s, ok := m.currentStop(); ok {
		return s.Path
	}
	return nil
}

// ensureLoad starts the current file's load unless it is in flight or
// settled; repeat requests are dropped, not queued. It returns the
// worker command or nil.
func (m *Model) ensureLoad() tea.Cmd {
	s, ok := m.currentStop()
	if !ok {
		return nil
	}
	key := string(s.Path)
	if m.loading[key] || m.bufs[key] != nil || m.failed[key] {
		return nil
	}
	m.loading[key] = true
	return m.loadCmd(s)
}

// loadCmd returns the worker command for one file: read, split, escape,
// and map all happen off the update path, behind the loadGate test seam
// when one is set. The completion message is keyed by the raw path.
func (m Model) loadCmd(s searchindex.Stop) tea.Cmd {
	path := bytes.Clone(s.Path)
	resolved := bytes.Clone(s.ResolvedPath)
	stops := stopsForFile(m.index, path)
	gate := m.loadGate
	return func() tea.Msg {
		if gate != nil {
			<-gate
		}
		buf, err := filebuffer.Load(resolved, stops)
		return loadDoneMsg{path: path, buf: buf, err: err}
	}
}

// gutterDigits returns the current file's gutter digit width: the digit
// width of its largest line number once loaded, one slot otherwise.
func (m Model) gutterDigits() int {
	if s, ok := m.currentStop(); ok {
		if b := m.bufs[string(s.Path)]; b != nil {
			return b.GutterDigits()
		}
	}
	return 1
}

// listWidth is the provisional file-list width — the longest escaped
// path plus padding, capped at 40% of the terminal and by the file
// panel's minimum — pending Issue #24's real formula.
func (m Model) listWidth(gutterW int) int {
	maxW := 0
	for _, f := range m.files {
		if w := ansi.StringWidth(present.Path(f)); w > maxW {
			maxW = w
		}
	}
	return max(0, min(min(maxW+2, m.width*2/5), m.width-(gutterW+10)))
}

// relayout recomputes the viewport's content dimensions after any state
// change that can alter them — resize, search completion, or a load
// changing the gutter — and reinstalls the current file's prepared
// rows, rebuilt for the new layout. Unavailable content installs nil,
// which empties the viewport.
func (m *Model) relayout() {
	listW := m.listWidth(m.gutterDigits() + 2)
	m.vp.Resize(max(0, m.width-listW-m.gutterDigits()-2), max(0, m.height-1))
	if cur := m.currentPath(); cur != nil {
		m.vp.SetRows(m.rows[string(cur)])
	}
}

// scroll applies one vertical scroll key to the current file's
// viewport and records the resulting top row as that file's saved
// vertical state for revisits. Outside browse, and on a file with no
// loaded buffer — the "Loading…" and "(unreadable)" placeholders —
// every scroll key is a no-op.
func (m *Model) scroll(key string) {
	if m.phase != phaseBrowse {
		return
	}
	cur := m.currentPath()
	if cur == nil || m.bufs[string(cur)] == nil {
		return
	}
	switch key {
	case "up":
		m.vp.Up()
	case "down":
		m.vp.Down()
	case "u":
		m.vp.HalfUp()
	case "d":
		m.vp.HalfDown()
	case "pgup":
		m.vp.PageUp()
	case "pgdown":
		m.vp.PageDown()
	}
	m.saved[string(cur)] = m.vp.Top()
}

// bufferRows adapts a loaded buffer to the viewport's prepared-row
// provider: unwrapped mode maps rendered row i to source line i, and
// the buffer's prepared cells and validated spans answer each query,
// so a frame touches only the rows it paints.
type bufferRows struct{ buf *filebuffer.Buffer }

func (r bufferRows) Len() int { return r.buf.LineCount() }

func (r bufferRows) Row(i int) viewport.Row {
	return viewport.Row{Line: i, Cells: r.buf.Cells(i), Spans: r.buf.Spans(i)}
}

// renderBrowse composes the two-pane frame: the file list on the left
// in raw-path order with the current entry underlined and kept visible,
// and on the right the filename rule over the current file's content —
// "Loading…" until its buffer arrives, "(unreadable)" on failure.
func (m Model) renderBrowse() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	cur := m.currentPath()
	buf := m.bufs[string(cur)]
	failed := m.failed[string(cur)]
	vis := m.vp.Visible()
	gutterW := m.gutterDigits() + 2
	listW := m.listWidth(gutterW)
	textW := max(0, m.width-listW-gutterW)

	curIdx := 0
	for i, f := range m.files {
		if bytes.Equal(f, cur) {
			curIdx = i
			break
		}
	}
	// Keep the current entry within the scrolled list window.
	listTop := 0
	if curIdx >= m.height {
		listTop = curIdx - m.height + 1
	}

	rows := make([]string, m.height)
	for r := 0; r < m.height; r++ {
		var sb strings.Builder
		if listW > 0 {
			fi := listTop + r
			entry := ""
			if fi < len(m.files) {
				entry = truncateLeft(present.Path(m.files[fi]), listW)
			}
			w := ansi.StringWidth(entry)
			if fi == curIdx && entry != "" {
				entry = m.theme.CurrentFile(entry)
			} else {
				entry = m.theme.FileList(entry)
			}
			sb.WriteString(entry)
			sb.WriteString(strings.Repeat(" ", max(0, listW-w)))
		}
		if r == 0 {
			sb.WriteString(m.theme.FilenameRule(filenameRule(cur, m.width-listW)))
		} else {
			sb.WriteString(m.contentRow(r-1, cur, buf, failed, vis, gutterW-2, textW))
		}
		row := sb.String()
		// Pad to the frame edge so the base style's background covers
		// the whole row.
		row += strings.Repeat(" ", max(0, m.width-ansi.StringWidth(row)))
		rows[r] = row
	}
	return strings.Join(rows, "\n")
}

// truncateLeft keeps the rightmost n cells of s, marking truncation
// with a leading "…" so basenames stay visible.
func truncateLeft(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if w := ansi.StringWidth(s); w > n {
		return ansi.TruncateLeft(s, w-n+1, "…")
	}
	return s
}

// filenameRule embeds the escaped current path in a horizontal rule: a
// short dash run, the path, then dashes to the panel edge. An oversized
// path is left-truncated so its tail stays visible.
func filenameRule(path []byte, w int) string {
	if w <= 0 {
		return ""
	}
	if w <= 4 || path == nil {
		return strings.Repeat("─", w)
	}
	shown := truncateLeft(present.Path(path), w-4)
	rule := "── " + shown + " "
	if rw := ansi.StringWidth(rule); rw < w {
		rule += strings.Repeat("─", w-rw)
	}
	return rule
}

// contentRow renders one content-area row: row 0 is the first viewport
// row. Until the buffer arrives the first row carries the placeholder
// behind a minimal one-digit gutter; loaded rows carry the
// right-justified line number, two spaces, then the escaped cells with
// matches in inverse video — additionally underlined on the cursor's
// current matched line.
func (m Model) contentRow(row int, cur []byte, buf *filebuffer.Buffer, failed bool, vis []viewport.Row, digits, textW int) string {
	gutter := m.theme.Gutter(strings.Repeat(" ", digits) + "  ")
	if buf == nil {
		if row == 0 && cur != nil {
			if failed {
				return gutter + "(unreadable)"
			}
			return gutter + "Loading…"
		}
		return ""
	}
	if row >= len(vis) {
		return ""
	}
	r := vis[row]
	curLine := -1
	if s, ok := m.currentStop(); ok && bytes.Equal(s.Path, cur) {
		curLine = int(s.Line) - 1
	}
	gutter = m.theme.Gutter(fmt.Sprintf("%*d", digits, r.Line+1) + "  ")
	return gutter + renderCells(r.Cells, r.Spans, textW, m.theme, r.Line == curLine)
}

// renderCells emits a line's display cells clipped to textW columns with
// highlighted spans in inverse video — inverse plus underline when the
// line is the current matched line. Marker spans (Start == End) paint
// one cell at their position without shifting text — replacing the
// cell's glyph, or extending the line by one inverse space at the end.
// A wide cluster split by the clip edge renders its visible cell blank.
func renderCells(cells []present.Cell, spans []present.Span, textW int, th theme.Theme, current bool) string {
	if textW <= 0 {
		return ""
	}
	covered := func(c int) bool {
		for _, s := range spans {
			if s.Start == s.End {
				if c == s.Start {
					return true
				}
				continue
			}
			if c >= s.Start && c < s.End {
				return true
			}
		}
		return false
	}
	marker := func(c int) bool {
		for _, s := range spans {
			if s.Start == s.End && s.Start == c {
				return true
			}
		}
		return false
	}
	match := th.Match
	if current {
		match = th.CurrentMatch
	}
	var sb strings.Builder
	var run strings.Builder
	inv := false
	flush := func() {
		if run.Len() == 0 {
			return
		}
		if inv {
			sb.WriteString(match(run.String()))
		} else {
			sb.WriteString(run.String())
		}
		run.Reset()
	}
	for c := 0; c < len(cells) && c < textW; c++ {
		txt := cells[c].Text
		// A marker paints its cell as one inverse space; the trailing
		// cell of a marked wide glyph paints blank so the row's width
		// accounting holds.
		mk := marker(c) || (cells[c].Cont && c > 0 && marker(c-1))
		if mk || (c+1 == textW && c+1 < len(cells) && cells[c+1].Cont) {
			// The second form is a cluster split at the clip edge.
			txt = " "
		}
		if cov := covered(c) || mk; inv != cov {
			flush()
			inv = cov
		}
		run.WriteString(txt)
	}
	flush()
	// An end-of-line marker extends the line by one inverse space.
	if len(cells) < textW && marker(len(cells)) {
		sb.WriteString(match(" "))
	}
	return sb.String()
}

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
	"vrg/internal/searchindex"
	"vrg/internal/viewport"
)

// rowSource is the prepared rendered-row provider the frame render
// consults: *viewport.Rows in production. Tests substitute a counting
// fake to prove a frame queries only the visible row range.
type rowSource interface {
	Len() int
	At(i int) viewport.Row
	GutterWidth() int
	// TargetRow is the rendered row holding the navigation stop's
	// display target — the row a destination reveal must show.
	TargetRow(st searchindex.Stop) int
}

// curFile is the cursor's current file index — the current file derives
// from the matched-line cursor, never from a separate selection.
func (m *model) curFile() int {
	if m.idx == nil {
		return 0
	}
	if cur, ok := m.idx.Cursor(); ok {
		return cur.File
	}
	return 0
}

// curKey is the current file's raw-path map key — the identity for the
// buffer, row-model, and per-file viewport caches — or false when the
// index is empty.
func (m *model) curKey() (string, bool) {
	if m.idx == nil || len(m.idx.Files) == 0 {
		return "", false
	}
	return string(m.idx.Files[m.curFile()].Path), true
}

// navigate moves the matched-line cursor one stop forward or back,
// applies the destination reveal, and on a file change starts a fresh
// file-change pop-up and returns it batched with the destination
// file's load command when that file is not already cached, in flight,
// or failed. A strict no-op step — an empty or single-stop index — is
// not a transition and triggers no reveal or pop-up. The departing
// file's viewport is already saved by scroll and reveal write-through,
// so the destination's reveal starts from its saved viewport or, on a
// first visit, the top of the file; an uncached destination reveals
// when its load completes instead. The pop-up starts at selection —
// while the destination may still be loading — and load completion
// never restarts it.
func (m *model) navigate(next bool) tea.Cmd {
	if m.idx == nil {
		return nil
	}
	from, _ := m.idx.Cursor()
	var mv searchindex.Move
	if next {
		mv = m.idx.Next()
	} else {
		mv = m.idx.Prev()
	}
	if cur, _ := m.idx.Cursor(); cur == from {
		return nil
	}
	m.reveal()
	if !mv.FileChanged {
		return nil
	}
	return tea.Batch(m.startLoad(), m.startPopup())
}

// reveal applies the vertical destination-reveal rules to the current
// file's viewport: starting from its saved per-file top — or the top
// of the file on a first visit — the rendered row holding the
// destination's display target is left in place when already visible
// and otherwise moved to floor(content height / 3), clamped to valid
// tops. A reveal that moves the viewport replaces the saved vertical
// state; a no-scroll reveal leaves it. While the panel shows a
// placeholder there is no row model and the reveal is a no-op.
func (m *model) reveal() {
	ck, ok := m.curKey()
	if !ok {
		return
	}
	rows := m.rows[ck]
	h := m.contentRows()
	if rows == nil || h < 1 {
		return
	}
	cur, ok := m.idx.Cursor()
	if !ok {
		return
	}
	vp := m.vps[ck]
	if vp.Reveal(rows.TargetRow(m.idx.Files[cur.File].Stops[cur.Stop]), rows.Len(), h) {
		m.vps[ck] = vp
	}
}

// contentRows is the file panel's content height: the frame height
// minus the filename-rule row it shares with the file list.
func (m *model) contentRows() int { return m.height - 1 }

// listWidth is the file list's rendered width under the Issue #5
// heuristic: the longest displayed path plus one padding cell, capped
// at the terminal width. Issue #24 owns the real formula (40% cap,
// minimum text width, left truncation).
func (m *model) listWidth() int {
	longest := 0
	if m.idx != nil {
		for _, f := range m.idx.Files {
			if cw := safepresentation.CellWidth(safepresentation.EscapePath(f.Path)); cw > longest {
				longest = cw
			}
		}
	}
	if w := longest + 1; w < m.width {
		return w
	}
	return m.width
}

// textWidth is a buffer's file-panel text width: the panel width minus
// the line-number gutter and the reserved right-indicator column —
// zero while wrapping, one in run-off-edge mode (Issue #20 populates
// it). All wrapping, clipping, and reveal math uses this width.
func (m *model) textWidth(gutter int) int {
	w := m.width - m.listWidth() - gutter - viewport.ReservedIndicator(m.wrap)
	if w < 0 {
		return 0
	}
	return w
}

// prepareRows builds one buffer's rendered-row model under the current
// layout, keyed by (path, content revision, text width, wrap mode) so a
// later layout change cannot mistake it for current.
func (m *model) prepareRows(key string, buf *filebuffer.Buffer) *viewport.Rows {
	return viewport.Prepare(buf, viewport.Key{
		Path:      key,
		Revision:  m.revs[key],
		TextWidth: m.textWidth(buf.GutterWidth()),
		Wrap:      m.wrap,
	})
}

// rebuildRows swaps in a freshly prepared row model for every cached
// buffer whose layout key changed — after a wrap toggle or a resize —
// then re-clamps each saved viewport to the new row count so no
// position can strand past EOF. A non-*Rows provider (a test fake) is
// left alone.
func (m *model) rebuildRows() {
	for key, buf := range m.bufs {
		k := viewport.Key{
			Path:      key,
			Revision:  m.revs[key],
			TextWidth: m.textWidth(buf.GutterWidth()),
			Wrap:      m.wrap,
		}
		cur, ok := m.rows[key].(*viewport.Rows)
		if !ok {
			continue
		}
		if cur.Key() != k {
			m.rows[key] = viewport.Prepare(buf, k)
		}
	}
	for key, vp := range m.vps {
		if rows := m.rows[key]; rows != nil {
			vp.Clamp(rows.Len(), m.contentRows())
			m.vps[key] = vp
		}
	}
}

// isScrollKey reports whether key is a browse-state vertical scroll key.
func isScrollKey(key string) bool {
	switch key {
	case "up", "down", "u", "d", "pgup", "pgdown":
		return true
	}
	return false
}

// scrollBy applies one vertical scroll key to the current file's saved
// viewport: up/down move one rendered row, u/d a half page
// (max(1, floor(h/2))), and pgup/pgdown a full page of the content
// height — all clamped to valid content. While the panel shows a
// placeholder ("Loading…" or "(unreadable)") there is no row model and
// the keys are no-ops.
func (m *model) scrollBy(key string) {
	ck, ok := m.curKey()
	if !ok {
		return
	}
	rows := m.rows[ck]
	h := m.contentRows()
	if rows == nil || h < 1 {
		return
	}
	var d int
	switch key {
	case "up":
		d = -1
	case "down":
		d = 1
	case "u":
		d = -viewport.HalfPage(h)
	case "d":
		d = viewport.HalfPage(h)
	case "pgup":
		d = -h
	case "pgdown":
		d = h
	}
	vp := m.vps[ck]
	vp.Scroll(d, rows.Len(), h)
	m.vps[ck] = vp
}

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
	f := m.idx.Files[m.curFile()]
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
	for i, f := range files {
		escaped[i] = safepresentation.EscapePath(f.Path)
	}
	listW := m.listWidth()
	panelW := w - listW

	// The list scrolls to keep the current entry visible; it shares the
	// h-1 rows below the filename rule with the file panel.
	cur := m.curFile()
	listTop := 0
	if cur >= h-1 {
		listTop = cur - h + 2
	}

	var rows rowSource
	failed := false
	curLine := int64(-1)
	top := 0
	if len(files) > 0 {
		key := string(files[cur].Path)
		rows, failed = m.rows[key], m.failed[key]
		if rows != nil {
			// The saved top is clamped on every state change; clamp
			// again here so a stale entry can never blank the panel.
			top = m.vps[key].Top()
			if max := viewport.MaxTop(rows.Len(), h-1); top > max {
				top = max
			}
		}
		// The current matched line is the cursor's selected stop.
		if c, ok := m.idx.Cursor(); ok {
			curLine = files[c.File].Stops[c.Stop].Number
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
			sb.WriteString(m.contentCell(r-1, panelW, top, rows, failed, curLine))
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
	if i == m.curFile() {
		entry = m.theme.CurrentFile(clipped)
	}
	return entry + strings.Repeat(" ", width-safepresentation.CellWidth(clipped))
}

// contentCell renders file-panel content row cr padded to width cells:
// gutter plus text for the prepared row at index top+cr — a wrapped
// continuation row carries a blank gutter — or the placeholder while
// no row model is available. In run-off-edge mode the rightmost cell
// is the reserved indicator column, left blank until Issue #20.
func (m *model) contentCell(cr, width, top int, rows rowSource, failed bool, curLine int64) string {
	if rows == nil {
		placeholder := "Loading…"
		if failed {
			placeholder = "(unreadable)"
		}
		if cr == 0 {
			return padTo(clipCells(placeholder, width), width)
		}
		return strings.Repeat(" ", width)
	}
	ri := top + cr
	if ri >= rows.Len() {
		return strings.Repeat(" ", width)
	}
	row := rows.At(ri)
	gw := rows.GutterWidth()
	gutter := fmt.Sprintf("%*d  ", gw-2, row.Line.Number)
	if row.Continuation() {
		gutter = strings.Repeat(" ", gw)
	}
	if len(gutter) > width {
		return m.theme.Gutter(gutter[:width])
	}
	reserved := viewport.ReservedIndicator(m.wrap)
	if n := width - gw; reserved > n {
		reserved = n
	}
	return m.theme.Gutter(gutter) +
		m.contentText(row, width-gw-reserved, row.Line.Number == curLine) +
		strings.Repeat(" ", reserved)
}

// filenameRule renders the current file's escaped path embedded in a
// horizontal rule across the full frame width above both panes:
// "─ path ────".
func (m *model) filenameRule(width int) string {
	name := ""
	if m.idx != nil && len(m.idx.Files) > 0 {
		name = safepresentation.EscapePath(m.idx.Files[m.curFile()].Path)
	}
	if width <= 4 {
		return padTo(clipCells(name, width), width)
	}
	clipped := clipCells(name, width-3)
	rule := "─ " + clipped + " "
	return m.theme.FilenameRule(rule + strings.Repeat("─", width-3-safepresentation.CellWidth(clipped)))
}

// contentText renders one rendered row's escaped cells into at most
// textW terminal cells, wrapping each maximal run of highlighted cells
// in the match style — the true inverse, additionally underlined when
// the line is the current matched line — and padding the rest with
// blanks. Clipping never splits a grapheme's cells: a cell that would
// cross the boundary ends the row.
func (m *model) contentText(row viewport.Row, textW int, cur bool) string {
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
		for _, s := range row.Line.Highlights {
			if c >= s.Start && c < s.End {
				return true
			}
		}
		return false
	}
	cells := row.Line.Cells
	col := 0
	for i := row.Start; i < row.End; i++ {
		c := cells[i]
		text := c.Text
		if text == "" {
			if i > row.Start && cells[i-1].Start == c.Start && cells[i-1].End == c.End {
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

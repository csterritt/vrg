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

// rowSource is the prepared rendered-row provider the frame render and
// position logic consult: *viewport.Rows in production. Tests
// substitute a counting fake to prove a frame queries only the visible
// row range.
type rowSource interface {
	viewport.Extent
	GutterWidth() int
	// TargetRow is the rendered row holding the navigation stop's
	// display target — the row a destination reveal must show.
	TargetRow(st searchindex.Stop) int
	// StopTarget resolves a stop to its display target: the
	// destination line and the first submatch's start cell — the cell
	// the horizontal reveal must paint (Issue #19).
	StopTarget(st searchindex.Stop) viewport.Target
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
	ck, _ := m.curKey()
	if mv.FileChanged {
		// A file change resets the horizontal offset to zero before
		// the reveal runs (Issue #18): a revisited file starts at its
		// left edge, and Issue #19's horizontal reveal will operate
		// on that zero — never on the saved pan.
		vp := m.vps[ck]
		vp.ResetOff()
		m.vps[ck] = vp
	}
	m.reveal()
	if !mv.FileChanged {
		return m.requestLayout(ck)
	}
	if m.failed[ck] {
		// Re-entering a previously failed file from a different file
		// shows its prior failure immediately — the overlay opens
		// with the recorded diagnostic while the panel switches to
		// "Loading…" — and exactly one retry is minted below, before
		// any dismissal (Issue #26's re-entry sequence). A load for
		// the path somehow already in flight drops the request per
		// Issue #25's one-load-per-path rule.
		m.openOverlay(m.failDiag[ck], false)
	}
	return tea.Batch(m.startLoad(), m.requestLayout(ck), m.startPopup())
}

// reveal applies the vertical destination-reveal rules to the current
// file's viewport: starting from its saved per-file top — or the top
// of the file on a first visit — the rendered row holding the
// destination's display target is left in place when already visible
// and otherwise moved to floor(content height / 3), clamped to valid
// tops. A reveal that moves the viewport replaces the saved vertical
// state; a no-scroll reveal leaves it. When no layout matching the
// current parameters is installed — the placeholder case — the reveal
// cannot run, so the intent pends on the path and commits against the
// layout whose matching completion installs.
func (m *model) reveal() {
	ck, ok := m.curKey()
	if !ok {
		return
	}
	// A reveal intent — run now or pended — supersedes a recorded
	// reload-anchor intent for the path: the latest selection's
	// reveal takes precedence (Issue #27).
	delete(m.pendingAnchor, ck)
	rows := m.currentRows(ck)
	h := m.contentRows()
	if rows == nil || h < 1 {
		m.pendingReveals[ck] = true
		return
	}
	cur, ok := m.idx.Cursor()
	if !ok {
		return
	}
	delete(m.pendingReveals, ck)
	vp := m.vps[ck]
	st := m.idx.Files[cur.File].Stops[cur.Stop]
	row := rows.TargetRow(st)
	// The vertical reveal runs first — its movement re-clamps the
	// stored offset against the newly visible rows — then the minimal
	// horizontal reveal adjusts the offset to paint the target's
	// cluster (Issue #19). A same-file n/p triggers both; in wrap
	// mode the horizontal half is a no-op.
	moved := vp.Reveal(row, rows, h)
	if vp.RevealOff(rows.StopTarget(st), row, rows) {
		moved = true
	}
	if moved {
		m.vps[ck] = vp
	}
}

// contentRows is the file panel's content height: the frame height
// minus the filename-rule row it shares with the file list.
func (m *model) contentRows() int { return m.height - 1 }

// fileListWidth is the file list's rendered width under Issue #24's
// three-term formula: the nonnegative minimum of the longest
// displayed path width plus two cells, floor(0.40 × the terminal
// width), and the terminal width minus the file panel's reservation
// (gutter width + 10 text cells + reserved indicator width). The
// third term keeps the panel's minimum content width ahead of the
// 40% cap; it is also the only term that can drive the result to
// zero, and the clamp keeps it there rather than negative.
func fileListWidth(longest, width, gutter, resInd int) int {
	w := longest + 2
	if c := width * 2 / 5; c < w { // floor(0.40 × width)
		w = c
	}
	if c := width - gutter - 10 - resInd; c < w {
		w = c
	}
	if w < 0 {
		return 0
	}
	return w
}

// listWidthFor is the file list's rendered width under a given
// line-number gutter — zero while the list is hidden (Issue #24),
// otherwise the three-term formula. The longest-path term is
// prepared once per index (m.listWBase) so a frame render never
// rescans the list.
func (m *model) listWidthFor(gutter int) int {
	if !m.listVisible {
		return 0
	}
	return fileListWidth(m.listWBase, m.width, gutter, viewport.ReservedIndicator(m.wrap))
}

// listWidth is the file list's rendered width this frame, under the
// current file's gutter — the value the frame layout and its tests
// share.
func (m *model) listWidth() int {
	return m.listWidthFor(m.gutterWidth())
}

// gutterWidth is the line-number gutter the current file's panel
// renders with: the installed row model's when one is current, else
// the cached buffer's — so the list width always matches the gutter
// a rendered frame would show. Three cells is the minimum a real
// buffer produces and the fallback while nothing is loaded.
func (m *model) gutterWidth() int {
	if ck, ok := m.curKey(); ok {
		if rows := m.currentRows(ck); rows != nil {
			return rows.GutterWidth()
		}
		if buf := m.bufs[ck]; buf != nil {
			return buf.GutterWidth()
		}
	}
	return 3
}

// textWidth is a buffer's file-panel text width: the panel width minus
// the line-number gutter and the reserved right-indicator column —
// zero while wrapping, one in run-off-edge mode where Issue #20's
// right-edge star draws. The list width is computed for that buffer's
// own gutter so a key minted for any path is self-consistent. All
// wrapping, clipping, and reveal math uses this width, and the clamp
// keeps pathological dimensions nonnegative.
func (m *model) textWidth(gutter int) int {
	w := m.width - m.listWidthFor(gutter) - gutter - viewport.ReservedIndicator(m.wrap)
	if w < 0 {
		return 0
	}
	return w
}

// layoutKey is the identity the current layout parameters demand for
// path's cached buffer: the raw path, its content revision, the text
// width under the buffer's gutter, and the wrap mode. A prepared row
// model installs only while its key still equals this — a completion
// minted under superseded parameters is obsolete.
func (m *model) layoutKey(path string, buf *filebuffer.Buffer) viewport.Key {
	return viewport.Key{
		Path:      path,
		Revision:  m.revs[path],
		TextWidth: m.textWidth(buf.GutterWidth()),
		Wrap:      m.wrap,
	}
}

// currentRows is path's installed row model when it matches the layout
// the live parameters demand — the only prepared data the frame render,
// scrolling, and reveals may slice. A stale installed model (prepared
// for a superseded key) is invisible: the panel shows the placeholder
// until its replacement installs. So is a model whose path has a load
// in flight: a reload replaces the display with "Loading…" until its
// new revision's layout installs, never presenting the old content
// mid-flight (Issue #27). Test fakes standing in for *Rows are always
// taken as current.
func (m *model) currentRows(path string) rowSource {
	if _, ok := m.loading[path]; ok {
		return nil
	}
	rows := m.rows[path]
	r, ok := rows.(*viewport.Rows)
	if !ok {
		return rows
	}
	buf := m.bufs[path]
	if buf == nil || buf.Unsupported() != "" || r.Key() != m.layoutKey(path, buf) {
		return nil
	}
	return rows
}

// layoutReadyMsg delivers a prepared row model keyed by the exact
// parameters it was built for: path, content revision, text width, and
// wrap mode. The command ran the whole preparation off the update path
// so Update only installs or discards the result.
type layoutReadyMsg struct {
	key  viewport.Key
	rows *viewport.Rows
}

// requestLayout issues the preparation of path's cached buffer under
// the current layout parameters as a command — row-model construction
// stays off the update path so input keeps flowing while it runs. It
// returns nil when the installed model already matches (the fast path),
// when an identical request is already in flight, when no buffer is
// cached yet, or when the buffer is unsupported — an Issue #30
// placeholder has no rows to prepare. The completion carries its key,
// so a superseded or out-of-order delivery is discarded rather than
// installed.
func (m *model) requestLayout(path string) tea.Cmd {
	buf := m.bufs[path]
	if buf == nil || buf.Unsupported() != "" {
		return nil
	}
	k := m.layoutKey(path, buf)
	if rows := m.rows[path]; rows != nil {
		if r, ok := rows.(*viewport.Rows); !ok || r.Key() == k {
			return nil
		}
	}
	if m.layoutReqs[path] == k {
		return nil
	}
	m.layoutReqs[path] = k
	gate := m.opts.layoutGate
	return func() tea.Msg {
		if gate != nil {
			gate()
		}
		return layoutReadyMsg{key: k, rows: viewport.Prepare(buf, k)}
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

// isPanKey reports whether key is a browse-state horizontal pan key.
func isPanKey(key string) bool {
	switch key {
	case ",", ".", "<", ">", "[", "]":
		return true
	}
	return false
}

// panBy applies one horizontal pan key to the current file's saved
// viewport: ,/. move one display cell, </> ten, and [/] half the text
// width — max(1, floor(textW / 2)) — each clamped to the paintable
// boundary of the widest currently rendered line, re-evaluated on
// every keypress. In wrap mode and on the placeholders the keys are
// strict no-ops.
func (m *model) panBy(key string) {
	ck, ok := m.curKey()
	if !ok {
		return
	}
	rows := m.currentRows(ck)
	h := m.contentRows()
	if rows == nil || h < 1 {
		return
	}
	var d int
	switch key {
	case ",":
		d = -1
	case ".":
		d = 1
	case "<":
		d = -10
	case ">":
		d = 10
	case "[":
		d = -viewport.HalfText(rows.Key().TextWidth)
	case "]":
		d = viewport.HalfText(rows.Key().TextWidth)
	}
	vp := m.vps[ck]
	vp.Pan(d, rows, h)
	m.vps[ck] = vp
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
	rows := m.currentRows(ck)
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
	vp.Scroll(d, rows, h)
	m.vps[ck] = vp
}

// fileLoadedMsg answers one requested load, keyed by the raw path and
// the request's identity: a message whose pair does not match a live
// request is discarded, so stale, forged, or superseded completions
// can never touch the cache or the visible panel (Issue #25). reload
// marks an explicit r reread: its completion records the anchor-
// preservation intent rather than the file-change reveal (Issue #27).
// The command performed the read plus decode and byte→cell mapping off
// the update path, so Update only files the result; it never does
// full-file work itself.
type fileLoadedMsg struct {
	path   []byte
	req    int
	reload bool
	buf    *filebuffer.Buffer
	err    error
}

// startLoad issues the current file's load command, or nil when the path
// is already loaded or already in flight — a repeated request is
// dropped, never queued, and there is no load cancellation. A
// previously failed path retries here: minting the retry clears the
// failure record so the panel reads "Loading…" until settlement marks
// it content or "(unreadable)" (Issue #26).
func (m *model) startLoad() tea.Cmd {
	if m.idx == nil || len(m.idx.Files) == 0 {
		return nil
	}
	f := m.idx.Files[m.curFile()]
	key := string(f.Path)
	if _, ok := m.loading[key]; ok {
		return nil
	}
	if _, ok := m.bufs[key]; ok {
		return nil
	}
	return m.mintLoad(f, false)
}

// startReload issues the current file's explicit-r reread command, or
// nil while a load for the path is already in flight — a duplicate r,
// like a re-entry crossing during the load, is dropped and never
// queued (Issue #27). Unlike startLoad it mints unconditionally: the
// cached content is exactly what the reload replaces, and the reread
// never reruns rg or touches the search-derived stops.
func (m *model) startReload() tea.Cmd {
	if m.idx == nil || len(m.idx.Files) == 0 {
		return nil
	}
	f := m.idx.Files[m.curFile()]
	if _, ok := m.loading[string(f.Path)]; ok {
		return nil
	}
	return m.mintLoad(f, true)
}

// mintLoad marks path's request live under a fresh identity and
// returns its worker command: the read runs under the test gate and
// the decode/map phase under its own gate so "Loading…" provably spans
// both, all off the update path. Minting clears a prior failure record
// so the panel reads "Loading…" until settlement marks it content or
// "(unreadable)", and a completion must echo the request identity back
// to be filed.
func (m *model) mintLoad(f searchindex.File, reload bool) tea.Cmd {
	key := string(f.Path)
	delete(m.failed, key)
	m.loadSeq++
	m.loading[key] = m.loadSeq
	path, stops := f.Path, f.Stops
	req, gate, decode := m.loadSeq, m.opts.loadGate, m.opts.decodeGate
	read := m.opts.loader
	if read == nil {
		read = filebuffer.Read
	}
	return func() tea.Msg {
		if gate != nil {
			gate()
		}
		raw, err := read(path)
		if err != nil {
			return fileLoadedMsg{path: path, req: req, reload: reload, err: err}
		}
		if decode != nil {
			decode()
		}
		return fileLoadedMsg{path: path, req: req, reload: reload, buf: filebuffer.Decode(raw, stops)}
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

// encodingDiag composes the single-line diagnostic for a file whose
// BOM declares an encoding vrg does not present (Issue #30): the path
// single-line-escaped so hostile name bytes can never forge a
// diagnostic line boundary, and the detected encoding named.
func encodingDiag(path []byte, enc string) string {
	return "cannot display " + safepresentation.EscapePath(path) + ": unsupported encoding " + enc
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
	placeholder := "Loading…"
	curLine := int64(-1)
	top := 0
	off := 0
	if len(files) > 0 {
		key := string(files[cur].Path)
		rows, placeholder = m.currentRows(key), m.placeholder(key)
		if rows != nil {
			// The saved top is clamped on every state change; clamp
			// again here so a stale entry can never blank the panel.
			top = m.vps[key].Top()
			if max := viewport.MaxTop(rows.Len(), h-1); top > max {
				top = max
			}
			// The horizontal window applies only in run-off-edge
			// mode; wrap mode always renders from cell zero.
			if !m.wrap {
				off = m.vps[key].Off()
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
			sb.WriteString(m.listCell(listTop+r-1, listW))
		}
		if panelW > 0 {
			sb.WriteString(m.contentCell(r-1, panelW, top, off, rows, placeholder, curLine))
		}
	}
	return sb.String()
}

// listCell renders the file-list entry at index i padded to width
// cells; the current entry is underlined. A path wider than its
// cells left-truncates with a leading … so the basename end stays
// visible (Issue #24). Only the visible window's entries are
// escaped — the frame render never queries the provider for
// off-window paths.
func (m *model) listCell(i, width int) string {
	if i >= len(m.idx.Files) {
		return strings.Repeat(" ", width)
	}
	clipped := leftTruncate(m.escapePath(m.idx.Files[i].Path), width)
	entry := m.theme.FileList(clipped)
	if i == m.curFile() {
		entry = m.theme.CurrentFile(clipped)
	}
	return entry + strings.Repeat(" ", width-safepresentation.CellWidth(clipped))
}

// placeholder is the panel's stand-in text while no row model is
// installed for path: "Loading…" while a load is in flight or none
// has settled, "(unreadable)" after a failed load (Issue #26), and
// "(unsupported encoding)" while the cached buffer carries a
// UTF-16/32 BOM (Issue #30). An in-flight load always reads
// Loading… — the unsupported or failed buffer it replaces is never
// presented mid-flight.
func (m *model) placeholder(path string) string {
	if _, ok := m.loading[path]; ok {
		return "Loading…"
	}
	if m.failed[path] {
		return "(unreadable)"
	}
	if buf := m.bufs[path]; buf != nil && buf.Unsupported() != "" {
		return "(unsupported encoding)"
	}
	return "Loading…"
}

// contentCell renders file-panel content row cr padded to width cells:
// gutter plus text for the prepared row at index top+cr — a wrapped
// continuation row carries a blank gutter — or the placeholder while
// no row model is available. In run-off-edge mode the text window
// starts off cells into the line and Issue #20's indicators draw:
// the gutter's first trailing space marks text hidden left ('_'),
// upgraded to '*' when a match or marker is entirely hidden there,
// and the reserved rightmost cell shows '*' on the current matched
// line's row when a match is entirely hidden right — never
// overwriting text. Wrap mode draws neither.
func (m *model) contentCell(cr, width, top, off int, rows rowSource, placeholder string, curLine int64) string {
	if rows == nil {
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
	textW := width - gw - reserved
	cur := row.Line.Number == curLine
	left, right := byte(' '), byte(' ')
	if !m.wrap {
		left = viewport.LeftMark(row, off)
		if cur && reserved > 0 {
			right = viewport.RightMark(row, off, textW)
		}
	}
	head := m.theme.Gutter(gutter)
	if left != ' ' {
		head = m.theme.Gutter(gutter[:gw-2]) +
			m.theme.Indicator(string(left)) +
			m.theme.Gutter(gutter[gw-1:])
	}
	tail := strings.Repeat(" ", reserved)
	if right == '*' {
		tail = m.theme.Indicator("*")
	}
	return head + m.contentText(row, off, textW, cur) + tail
}

// staleNote is the buffer-status note the filename row's slot shows
// while the buffer's recorded submatches fail validation — the file
// changed on disk since the search ran (Issue #29). It persists on
// every display — no timer — until a load validates fully again.
const staleNote = "file changed since search"

// filenameRule renders the current file's escaped path embedded in a
// horizontal rule across the full frame width above both panes:
// "─ path ────". The buffer-status note slot sits inside the rule
// after the path — "─ path note ────" — and the path left-truncates
// with a leading … to make room for the note where possible
// (Issue #24 provides the slot; Issue #29 supplies the note).
func (m *model) filenameRule(width int) string {
	name, note := "", ""
	if m.idx != nil && len(m.idx.Files) > 0 {
		name = m.escapePath(m.idx.Files[m.curFile()].Path)
		note = m.notes[string(m.idx.Files[m.curFile()].Path)]
	}
	if width <= 4 {
		return padTo(leftTruncate(name, width), width)
	}
	noteW := safepresentation.CellWidth(note)
	if note != "" {
		noteW++ // the trailing space closing the note's slot
	}
	if noteW > width-3 {
		// The path yields its cells to the slot first; the note
		// itself clips to whatever the slot leaves.
		note = clipCells(note, width-4)
		noteW = safepresentation.CellWidth(note) + 1
	}
	clipped := leftTruncate(name, width-3-noteW)
	rule := "─ " + clipped + " "
	if note != "" {
		rule += note + " "
	}
	return m.theme.FilenameRule(rule + strings.Repeat("─", width-safepresentation.CellWidth(rule)))
}

// contentText renders one rendered row's escaped cells into at most
// textW terminal cells starting at display cell off, wrapping each
// maximal run of highlighted cells in the match style — the true
// inverse, additionally underlined when the line is the current
// matched line — and padding the rest with blanks. Clipping never
// splits a grapheme's cells: a cluster straddling either boundary
// contributes one blank per in-window cell for its clipped portion —
// never half a glyph, and never a match-styled blank (Issue #21).
// Zero-width markers paint with the match style like any other match
// cell (Issue #23): a marker inside text marks the existing cell it
// lands on — no text shifts — and the end-of-line marker is the
// one-cell unit past the line's last cluster, rendered as an inverse
// space and clipped at the right edge like text.
func (m *model) contentText(row viewport.Row, off, textW int, cur bool) string {
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
	clusters := row.Line.Clusters
	// The painted window starts at the horizontal offset: cells left
	// of off are clipped away. A cluster straddling either clip edge
	// — starting left of the window or ending beyond it — contributes
	// one blank per in-window cell rather than a partial glyph, and a
	// clip blank is never a match cell (Issue #21).
	lo := row.Start
	if off > lo {
		lo = off
	}
	col := 0
	ci := 0 // cursor into the clusters tiling the row's cells
	for i := lo; i < row.End; i++ {
		if i >= len(cells) {
			// The end-of-line marker cell — a one-cell unit past the
			// line's clusters: an inverse space, clipped at the right
			// edge like any other cell.
			if col+1 > textW {
				break
			}
			if hl := row.Line.MarkerAt(i); hl != runHL {
				flush()
				runHL = hl
			}
			run.WriteString(" ")
			col++
			continue
		}
		for ci < len(clusters) && i >= clusters[ci].End {
			ci++
		}
		if ci == len(clusters) {
			break
		}
		cl := clusters[ci]
		c := cells[i]
		text := c.Text
		hl := highlighted(i) || row.Line.MarkerAt(i)
		switch {
		case cl.Start < lo || cl.End-lo > textW:
			text, hl = " ", false // a clip blank — never a match cell
		case text == "" && i > cl.Start:
			continue // continuation cell of a painted wide glyph
		case text == "":
			text = string(rune(0xfffd)) // an invalid byte's replacement char
		}
		cw := safepresentation.CellWidth(text)
		if col+cw > textW {
			break
		}
		if hl != runHL {
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

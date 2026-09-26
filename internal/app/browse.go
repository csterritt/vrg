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
// cache key — never a displayed form — and req is the request identity
// ensureLoad minted; a completion whose identity is not the in-flight
// request's is dropped.
type loadDoneMsg struct {
	path []byte
	req  int
	buf  *filebuffer.Buffer
	err  error
}

// layoutDoneMsg delivers one file's prepared row model from the
// layout worker: Prepare — the row mapping over the whole buffer —
// ran off the update path, so Update only installs the result. The
// (path, content revision, text width, wrap mode) key is what the
// request was minted with; Update installs only while it equals the
// current parameters, so out-of-order and superseded completions are
// inert.
type layoutDoneMsg struct {
	key  viewport.Key
	rows viewport.Rows
}

// installed is one file's cached prepared layout with the key it was
// built for — the value the install guard compares against the current
// parameters.
type installed struct {
	key  viewport.Key
	rows viewport.Rows
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
// no-ops — no command, no state change, no pop-up. A step within the
// same file keeps the viewport and reveals the destination target. A
// step into another file switches the panel — the departing file's top
// row joins its saved state, the viewport reinstalls the new file's
// prepared rows at its own saved top (top of file on a first visit),
// the destination reveal runs over that starting point when content is
// already cached, and its load is requested when the file is neither
// cached nor in flight nor already failed — and opens the file-change
// pop-up: a fresh instance whose one-second expiry command returns
// alongside the load. A previously failed destination instead runs the
// re-entry sequence and shows no pop-up. The file list needs no
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
	var pop, lay tea.Cmd
	if step.FileChanged {
		if depart != nil {
			m.saved[string(depart)] = m.vp.Anchor()
		}
		// The destination's gutter may change the geometry, so the
		// current parameters are recomputed first; a request for the
		// destination's layout joins the batch when its cached rows
		// are missing or stale-keyed.
		lay = m.syncLayout()
		// The destination installs its own rows — only while their
		// key matches — or empties the viewport behind the
		// placeholder; then its saved anchor becomes the reading
		// position, resolving when a matching layout installs (the
		// zero value is the top of the file — a first visit).
		m.vp.SetRows(m.currentRows())
		m.vp.SetAnchor(m.saved[string(step.Stop.Path)])
		// The horizontal offset resets to zero on file change, ahead
		// of the reveal's horizontal half.
		m.vp.SetOffset(0)
		// A failed destination gets the re-entry sequence's
		// prior-failure overlay, not the file-change pop-up.
		if !m.failed[string(step.Stop.Path)] {
			pop = m.openPopup(step.Stop.Path)
		}
	}
	// The destination reveal commits against installed rows or is
	// carried as the pending intent for the newest stop.
	m.reveal()
	return m, tea.Batch(m.entryLoad(step), lay, pop)
}

// pendingIntent is the deferred action owed to the current file's
// viewport while no layout matching the current parameters is
// installed. The model carries it and commits it through the Issue
// #17 installation path when a matching prepared layout arrives;
// superseded and non-current installs leave it untouched.
type pendingIntent int

const (
	// intentNone means no deferred action is owed.
	intentNone pendingIntent = iota
	// intentReveal is the destination reveal for the newest selected
	// stop — the navigation and first-load intent.
	intentReveal
	// intentAnchor keeps the retained logical anchor with no reveal —
	// recorded by an explicit reload's load completion (Issue #27).
	// Issue #28 generalizes this seam into the full
	// reveal-versus-reload arbitration for all load completions.
	intentAnchor
)

// reveal applies the destination reveal for the current stop: the
// display target is the start cell of the first submatch on the
// destination line — the marker cell for a zero-width match — so the
// first surviving validated span supplies it (stale-entry fallbacks
// are Issue #29's). The reveal covers both axes: the vertical
// one-third placement and, in run-off-edge mode, the minimal
// horizontal reveal of the target's start cell. A reveal that moves
// the viewport replaces the file's saved vertical state; a no-scroll
// reveal — an already-visible target — leaves it. With no installed
// layout matching the current parameters the reveal cannot run: it is
// carried as the pending intent and committed when a matching layout
// installs — always for whichever stop is newest at commit time.
func (m *Model) reveal() {
	s, ok := m.currentStop()
	if !ok {
		m.pending = intentNone
		return
	}
	key := string(s.Path)
	buf := m.bufs[key]
	if buf == nil || m.currentRows() == nil {
		m.pending = intentReveal
		return
	}
	m.pending = intentNone
	t := viewport.Target{Line: int(s.Line) - 1}
	for i, sp := range buf.Spans(t.Line) {
		if i == 0 || sp.Start < t.Cell {
			t.Cell = sp.Start
		}
	}
	if m.vp.Reveal(t) {
		m.saved[key] = m.vp.Anchor()
	}
}

// commitIntent discharges the intent owed to the current file after a
// matching layout installs: a deferred reveal runs for the newest
// selected stop, while the reload-anchor intent's work — resolving
// the retained anchor against the new rows — already happened inside
// SetRows, so it only clears. Obsolete and non-current installs never
// reach here: they cannot consume the intent.
func (m *Model) commitIntent() {
	switch m.pending {
	case intentReveal:
		m.reveal()
	case intentAnchor:
		m.pending = intentNone
	}
}

// currentPath returns the raw path of the current file, or nil.
func (m Model) currentPath() []byte {
	if s, ok := m.currentStop(); ok {
		return s.Path
	}
	return nil
}

// entryLoad issues the load a navigation destination is owed. An
// ordinary destination follows the first-load rules; a previously
// failed file re-entered from a different file runs the re-entry
// sequence instead (Issue #26): the prior failure's overlay opens at
// once, the panel returns to "Loading…" for the retry, and exactly one
// retry load starts while the overlay is up — dropped, not queued,
// when that path's load is somehow already in flight, in which case
// the existing load's settlement drives the panel update. A same-file
// step never retries a failed file.
func (m *Model) entryLoad(step searchindex.Step) tea.Cmd {
	key := string(step.Stop.Path)
	if !step.FileChanged || !m.failed[key] {
		return m.ensureLoad()
	}
	if lines := m.failLines[key]; len(lines) > 0 {
		m.openOverlay(lines)
	}
	if m.loading[key] != 0 {
		return nil
	}
	m.loadSeq++
	m.loading[key] = m.loadSeq
	return m.loadCmd(step.Stop, m.loadSeq)
}

// reload issues the explicit r reload: exactly one reread of the
// current file's raw path — never an rg rerun, never a cursor or stop
// change. A press while that path's load is in flight is dropped, not
// queued: the placeholder's change off "Loading…" is the only
// completion signal. The cached buffer is dropped up front so the
// panel reads "Loading…" — scrolling is a placeholder no-op — and a
// failed reread can never present stale content as refreshed. A
// previously failed file's r is its retry route — the only one in a
// one-stop index — and re-opens the prior-failure overlay while the
// retry runs, as in the cross-file re-entry sequence. The request is
// marked so its completion records the anchor intent rather than a
// first load's reveal.
func (m *Model) reload() tea.Cmd {
	s, ok := m.currentStop()
	if !ok {
		return nil
	}
	key := string(s.Path)
	if m.loading[key] != 0 {
		return nil
	}
	delete(m.bufs, key)
	if lines := m.failLines[key]; len(lines) > 0 {
		m.openOverlay(lines)
	}
	m.loadSeq++
	m.loading[key] = m.loadSeq
	m.reloading[key] = true
	return m.loadCmd(s, m.loadSeq)
}

// ensureLoad starts the current file's load unless it is in flight or
// settled; repeat requests are dropped, not queued — at most one load
// is ever in flight per raw path. It mints the request's identity,
// records it as the path's in-flight request, and returns the worker
// command or nil.
func (m *Model) ensureLoad() tea.Cmd {
	s, ok := m.currentStop()
	if !ok {
		return nil
	}
	key := string(s.Path)
	if m.loading[key] != 0 || m.bufs[key] != nil || m.failed[key] {
		return nil
	}
	m.loadSeq++
	m.loading[key] = m.loadSeq
	return m.loadCmd(s, m.loadSeq)
}

// loadCmd returns the worker command for one file: the read, split,
// escape, and map all happen off the update path. The loadGate test
// seam holds the whole worker before the read; mapGate holds the
// decode/map phase alone, after the read — so a test can prove input
// stays actionable while the expensive phase is parked (Issue #25).
// The completion message is keyed by the raw path and the request
// identity.
func (m Model) loadCmd(s searchindex.Stop, req int) tea.Cmd {
	path := bytes.Clone(s.Path)
	resolved := bytes.Clone(s.ResolvedPath)
	stops := stopsForFile(m.index, path)
	gate, mapGate := m.loadGate, m.mapGate
	read := m.readFile
	if read == nil {
		read = filebuffer.ReadFile
	}
	return func() tea.Msg {
		if gate != nil {
			<-gate
		}
		data, err := read(resolved)
		if err != nil {
			return loadDoneMsg{path: path, req: req, err: err}
		}
		if mapGate != nil {
			<-mapGate
		}
		return loadDoneMsg{path: path, req: req, buf: filebuffer.Prepare(data, stops)}
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

// reservedW is the right-indicator column width reserved in the file
// panel: zero in wrap mode, one in run-off-edge mode (Issue #20
// populates the column).
func (m Model) reservedW() int {
	if m.wrap {
		return 0
	}
	return 1
}

// listWidth is the file-list width for the current geometry: zero
// while the list is hidden — a zero-width allocation draws no cells
// and never changes the visibility preference — else the nonnegative
// minimum of the longest escaped path plus two cells of padding,
// floor(0.40 × terminal width), and the terminal width minus the file
// panel's minimum (gutter + ten text cells + the reserved indicator
// column). Scanning the entries for the longest runs on the update
// path only; the frame render uses the cached result.
func (m Model) listWidth(gutterW, res int) int {
	if !m.listShow {
		return 0
	}
	maxW := 0
	for _, f := range m.files {
		if w := ansi.StringWidth(m.listEntry(f)); w > maxW {
			maxW = w
		}
	}
	return max(0, min(min(maxW+2, m.width*2/5), m.width-(gutterW+10+res)))
}

// scrollList keeps the current file's entry inside the visible list
// window with minimal movement: the window shifts only when the entry
// falls outside it, so entries below the active one stay put while
// retreating. It runs inside syncLayout so navigation, resize, and
// every other geometry change re-check it.
func (m *Model) scrollList() {
	rows := m.height
	cur := 0
	if p := m.currentPath(); p != nil {
		cur = m.fileIdx[string(p)]
	}
	switch {
	case cur < m.listTop:
		m.listTop = cur
	case rows > 0 && cur >= m.listTop+rows:
		m.listTop = cur - rows + 1
	}
	m.listTop = min(max(m.listTop, 0), max(0, len(m.files)-rows))
}

// syncLayout recomputes the content geometry after any state change
// that can alter it — resize, wrap toggle, list hide/show, search
// completion, a load changing the gutter, or a file switch — resizes
// the viewport, keeps the list's active entry in view, and caches the
// list and text widths so a frame render never rescans the file list.
// It returns the current file's layout request when its installed rows
// are missing or stale-keyed; preparation runs off this path and
// installs on completion, resolving the retained logical anchor
// against the new row model.
func (m *Model) syncLayout() tea.Cmd {
	res := m.reservedW()
	gutterW := m.gutterDigits() + 2
	m.listW = m.listWidth(gutterW, res)
	m.textW = max(0, m.width-m.listW-gutterW-res)
	m.scrollList()
	m.vp.Resize(m.textW, max(0, m.height-1))
	return m.ensureLayout()
}

// layoutKey is the (path, content revision, text width, wrap mode) a
// prepared layout must carry to be current for path — the request key
// minted at issue time and the install guard's comparison.
func (m Model) layoutKey(path string) viewport.Key {
	return viewport.Key{Path: path, Rev: m.revs[path], Width: m.textW, Wrap: m.wrap}
}

// currentRows returns the current file's installed row model while its
// key matches the current parameters — nil when the layout is missing
// or stale, leaving the viewport empty behind the placeholder until
// the prepared layout installs.
func (m Model) currentRows() viewport.Rows {
	cur := m.currentPath()
	if cur == nil {
		return nil
	}
	if inst, ok := m.rows[string(cur)]; ok && inst.key == m.layoutKey(string(cur)) {
		return inst.rows
	}
	return nil
}

// ensureLayout returns a preparation command for the current file when
// its installed layout does not match the current parameters and no
// request for that key is already in flight; repeat requests are
// dropped, not queued.
func (m *Model) ensureLayout() tea.Cmd {
	cur := m.currentPath()
	if cur == nil {
		return nil
	}
	key := string(cur)
	buf := m.bufs[key]
	if buf == nil {
		return nil
	}
	want := m.layoutKey(key)
	if m.rows[key].key == want || m.reqKey[key] == want {
		return nil
	}
	m.reqKey[key] = want
	return m.layoutCmd(buf, want)
}

// layoutCmd returns the layout worker command for one file: the whole
// row model is prepared off the update path and delivered as a keyed
// completion — like a file load.
func (m Model) layoutCmd(buf *filebuffer.Buffer, key viewport.Key) tea.Cmd {
	return func() tea.Msg {
		return layoutDoneMsg{key: key, rows: viewport.Prepare(buf, key)}
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
	m.saved[string(cur)] = m.vp.Anchor()
}

// pan applies one horizontal pan key to the current file's viewport:
// ,/. one column, </> ten columns, [/] max(1, floor(text width / 2)).
// Panning exists only in run-off-edge mode — the viewport keeps the
// offset dormant under a wrap model — and is a no-op outside browse
// and on the loading/unreadable placeholders, like scroll.
func (m *Model) pan(key string) {
	if m.phase != phaseBrowse {
		return
	}
	cur := m.currentPath()
	if cur == nil || m.bufs[string(cur)] == nil {
		return
	}
	switch key {
	case ",":
		m.vp.Left()
	case ".":
		m.vp.Right()
	case "<":
		m.vp.TenLeft()
	case ">":
		m.vp.TenRight()
	case "[":
		m.vp.HalfLeft()
	case "]":
		m.vp.HalfRight()
	}
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
	listW := m.listW // cached by syncLayout — no per-frame list scan
	textW := m.textW
	curIdx := m.fileIdx[string(cur)]

	rows := make([]string, m.height)
	for r := 0; r < m.height; r++ {
		var sb strings.Builder
		if listW > 0 {
			fi := m.listTop + r
			entry := ""
			if fi < len(m.files) {
				entry = truncateLeft(m.listEntry(m.files[fi]), listW)
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
			note := m.statusNote
			if note == nil {
				note = m.bufferNote
			}
			sb.WriteString(m.theme.FilenameRule(filenameRule(cur, note(cur), m.width-listW)))
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

// truncateLeft keeps the rightmost cells of s within a budget of n
// cells, marking truncation with a leading "…" so basenames stay
// visible. The cut lands only on a grapheme boundary: a cluster
// straddling it is dropped whole, so the result never exceeds n cells
// and never shows half a glyph.
func truncateLeft(s string, n int) string {
	if n <= 0 {
		return ""
	}
	w := ansi.StringWidth(s)
	if w <= n {
		return s
	}
	// The "…" marker takes one cell; the kept suffix fits in n-1.
	// Whole leading clusters drop until the suffix fits — a cluster
	// straddling the cut is dropped rather than split.
	drop := w - n + 1
	acc, i := 0, 0
	for i < len(s) && acc < drop {
		cl, cw := ansi.FirstGraphemeCluster(s[i:], ansi.GraphemeWidth)
		acc += cw
		i += len(cl)
	}
	return "…" + s[i:]
}

// filenameRule embeds the escaped current path in a horizontal rule —
// a short dash run, the path, then dashes to the panel edge — and
// carries the buffer-status note in the slot between the path and the
// trailing dashes. The note wins cells over the path: an oversized
// path is left-truncated, down to nothing, so the note paints whole
// where possible; a note that cannot fit even with an empty path is
// dropped rather than clipped mid-text.
func filenameRule(path []byte, note string, w int) string {
	if w <= 0 {
		return ""
	}
	if w <= 4 || path == nil {
		return strings.Repeat("─", w)
	}
	if ansi.StringWidth(note)+4 > w {
		note = ""
	}
	avail := w - 4 // "── " before the path, " " before the dash run
	if note != "" {
		avail -= ansi.StringWidth(note) + 1
	}
	shown := truncateLeft(present.Path(path), avail)
	rule := "── " + shown
	if shown != "" && note != "" {
		rule += " "
	}
	rule += note + " "
	if rw := ansi.StringWidth(rule); rw < w {
		rule += strings.Repeat("─", w-rw)
	}
	return rule
}

// bufferNote is the real filename-row status provider: the
// "(unreadable)" note while the path sits in the failed state. The
// statusNote seam overrides it in tests; Issues #29 and #30 extend the
// real provider with their own notes.
func (m Model) bufferNote(path []byte) string {
	if m.failed[string(path)] {
		return "(unreadable)"
	}
	return ""
}

// contentRow renders one content-area row: row 0 is the first viewport
// row. Until the buffer arrives the first row carries the placeholder
// behind a minimal one-digit gutter — "(unreadable)" for a failed file
// with no load in flight, "Loading…" while any load runs — clipped to
// the text area so a constrained width cannot overflow; loaded rows
// carry the
// right-justified line number, two spaces, then the escaped cells with
// matches in inverse video — additionally underlined on the cursor's
// current matched line.
func (m Model) contentRow(row int, cur []byte, buf *filebuffer.Buffer, failed bool, vis []viewport.Row, digits, textW int) string {
	gutter := m.theme.Gutter(strings.Repeat(" ", digits) + "  ")
	if buf == nil {
		if row == 0 && cur != nil {
			if failed && m.loading[string(cur)] == 0 {
				return gutter + ansi.Truncate("(unreadable)", textW, "")
			}
			return gutter + ansi.Truncate("Loading…", textW, "")
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
	// A continuation row keeps the gutter blank so its text stays
	// aligned with the row its source line leads with.
	num := strings.Repeat(" ", digits)
	if !r.Cont {
		num = fmt.Sprintf("%*d", digits, r.Line+1)
	}
	// In run-off-edge mode the first trailing gutter space signposts
	// hidden-left content — inverse "_" for text, upgraded to inverse
	// "*" when a match or marker on the line is entirely hidden left.
	// Wrap rows never carry the flags.
	switch {
	case r.MatchHiddenLeft:
		gutter = m.theme.Gutter(num) + m.theme.Indicator("*") + " "
	case r.HiddenLeft:
		gutter = m.theme.Gutter(num) + m.theme.Indicator("_") + " "
	default:
		gutter = m.theme.Gutter(num + "  ")
	}
	cells := renderCells(r.Cells, r.Spans, textW, m.theme, r.Line == curLine)
	if m.reservedW() == 0 {
		return gutter + cells
	}
	// The reserved rightmost column is blank except an inverse "*" on
	// the current matched line's row when a match or marker on that
	// line is entirely hidden right. Painted text stops at textW, so
	// the indicator never overwrites it.
	right := " "
	if r.Line == curLine && r.MatchHiddenRight {
		right = m.theme.Indicator("*")
	}
	return gutter + cells + strings.Repeat(" ", max(0, textW-ansi.StringWidth(cells))) + right
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
		// A wrap or clip blank — marked by the row layer, or a cluster
		// split detected at the clip edge — paints unstyled even when a
		// coverage span crosses it: it is filler, not a match cell.
		bl := cells[c].Blank || (c+1 == textW && c+1 < len(cells) && cells[c+1].Cont)
		if mk || bl {
			txt = " "
		}
		if cov := mk || (covered(c) && !bl); inv != cov {
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

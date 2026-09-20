package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/clipperhouse/displaywidth"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
	"vrg/internal/theme"
	"vrg/internal/viewport"
)

// state identifies the current screen of the search lifecycle.
type state int

const (
	// stateSearching covers collection and post-exit processing: the
	// whole interval until the index is ready, even after rg has exited.
	stateSearching state = iota
	// stateBrowse is the two-pane result browser: file list plus the
	// current file's content panel.
	stateBrowse
	// stateNoResults is the centred outcome screen of a complete,
	// successful search that retained no usable results.
	stateNoResults
	// stateFatal is the fatal no-results outcome: the error overlay is
	// the whole presentation — there is no underlying state — so its
	// dismissal exits at the fixed status 2.
	stateFatal
	// stateCancelled is the terminal cancellation state: the program is
	// quitting with status 130 and late completions must not revive it.
	stateCancelled
)

// Model is the Bubble Tea model for the vrg lifecycle; this slice covers
// the searching screen, the browse view with asynchronous file loading,
// and cancellation.
type Model struct {
	child   Child
	workdir string
	// gate, when non-nil, is awaited between child exit and index
	// preparation so tests can hold preparation independently of rg exit.
	gate <-chan struct{}
	// loadGate, when non-nil, is awaited inside the load worker before a
	// file is read and mapped, so tests can hold a load off the update
	// path while input stays live.
	loadGate <-chan struct{}
	// ctx and cancel drive cancellation of collection and gated loads:
	// cancelling releases a gate-held worker promptly.
	ctx    context.Context
	cancel context.CancelFunc

	width  int
	height int
	state  state

	index   *searchindex.Index
	procErr error
	status  int
	// diags is the session diagnostic collection: every diagnostic the
	// model has processed, in collection order, replayed to stderr after
	// the terminal is restored. It is a shared pointer so Run's cleanup
	// boundary appends the controlled-failure diagnostic to the same
	// collection. stderrLines is the subset sourced from the child's
	// stderr, which the outcome decision shows in an overlay.
	diags       *diagnostics
	stderrLines []string
	// overlay, when non-nil, is the open diagnostic overlay: the modal
	// error/warning presentation of a completed search.
	overlay *errOverlay
	// popup, when non-nil, is the active file-change pop-up; popupSeq
	// mints each pop-up's instance ID, and popupTimer builds the
	// instance's expiry command — a seam so tests drive expiry with
	// injected messages instead of the clock.
	popup      *popup
	popupSeq   int
	popupTimer func(id int) tea.Cmd

	// Browse state: the raw-path-ordered file list, per-path prepared
	// buffers keyed by raw path bytes, and the load bookkeeping that
	// keeps one load in flight per path. The matched-line cursor lives
	// in the index; the current file derives from it. vps holds each
	// visited file's saved vertical viewport under the same key, so a
	// revisited file resumes from its saved top row.
	files   [][]byte
	fileIdx map[string]int
	buffers map[string]rowSource
	loading map[string]bool
	failed  map[string]bool
	listTop int
	vps     map[string]*viewport.Viewport
	theme   theme.Theme
}

// New returns a Model that collects the started child's stream. The
// child must already be running; spawn failures are handled by Run
// before the TUI exists.
func New(child Child, workdir string) Model {
	ctx, cancel := context.WithCancel(context.Background())
	return Model{
		child:      child,
		workdir:    workdir,
		state:      stateSearching,
		ctx:        ctx,
		cancel:     cancel,
		diags:      &diagnostics{},
		buffers:    make(map[string]rowSource),
		loading:    make(map[string]bool),
		failed:     make(map[string]bool),
		vps:        make(map[string]*viewport.Viewport),
		theme:      theme.Styled(),
		popupTimer: popupTick,
	}
}

// Init starts collection and index preparation off the UI update path.
// Drained stderr lines are delivered as stderrMsg values through the
// collection's emit — wired to the running program's Send by the
// default runner — so they are collected as they arrive.
func (m Model) Init() tea.Cmd {
	ctx, child, workdir, gate := m.ctx, m.child, m.workdir, m.gate
	diags := m.diags
	return func() tea.Msg {
		return collect(ctx, child, workdir, gate, diags.emit)
	}
}

// Update applies messages to the model. Collection results arrive as
// searchResult and enter the browse view; file loads arrive as
// loadResult carrying a prepared buffer. Keys act per state — q cancels
// while searching and quits while browsing, and ctrl+c cancels in any
// state. Esc is not an exit key and is a no-op outside overlays.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		// Re-clamp every loaded file's saved viewport: a shorter panel
		// may leave avoidable blank rows below EOF at the old top.
		for key, buf := range m.buffers {
			m.viewportFor(key).SetExtent(buf.LineCount(), m.contentHeight())
		}
		if m.overlay != nil {
			if max := m.overlay.maxScroll(m.width, m.height); m.overlay.scroll > max {
				m.overlay.scroll = max
			}
		}
	case stderrMsg:
		if m.state == stateCancelled {
			// A diagnostic still in flight at the exit decision is never
			// collected.
			return m, nil
		}
		for _, line := range escapeDiagnosticLines(msg.raw) {
			m.stderrLines = append(m.stderrLines, line)
			m.diags.add(line)
		}
	case searchResult:
		if m.state == stateCancelled {
			// A late completion must not revive a cancelled run.
			return m, nil
		}
		m.index = msg.index
		m.procErr = msg.err
		oc := decideOutcome(outcomeInput{
			procErr:   msg.err,
			integrity: msg.integrity,
			report:    msg.report,
			usable:    msg.index.UsableResults(),
			stderr:    m.stderrLines,
		})
		// The exit status is fixed here, at the outcome decision; only
		// ctrl+c overrides it later.
		m.status = oc.status
		// The stderr prefix of the outcome diagnostics was already
		// collected line by line; only the generated diagnostics — the
		// process, record-loss, and integrity lines — join the session
		// collection at the decision.
		m.diags.addAll(oc.diagnostics[len(m.stderrLines):])
		if len(oc.diagnostics) > 0 {
			m.overlay = &errOverlay{lines: oc.diagnostics}
			// An overlay opening cancels any pop-up for good.
			m.popup = nil
		}
		switch oc.state {
		case stateBrowse:
			return m.enterBrowse()
		case stateFatal:
			m.state = stateFatal
		default:
			m.state = stateNoResults
		}
		return m, nil
	case loadResult:
		if m.state == stateCancelled {
			return m, nil
		}
		key := string(msg.path)
		delete(m.loading, key)
		if msg.err != nil {
			m.failed[key] = true
			delete(m.buffers, key)
			line := "cannot read " + safepresentation.EscapePath(msg.path) +
				": " + safepresentation.EscapePath([]byte(msg.err.Error()))
			m.diags.add(line)
			if m.state == stateBrowse && key == m.curKey() {
				// A current-file failure interrupts with the error
				// overlay; opening it cancels any pop-up for good.
				m.popup = nil
				if m.overlay != nil {
					m.overlay.lines = append(m.overlay.lines, line)
				} else {
					m.overlay = &errOverlay{lines: []string{line}}
				}
			}
		} else {
			m.buffers[key] = msg.buf
			delete(m.failed, key)
			// The prepared row data arrives with the buffer; the file's
			// saved viewport is clamped to its extent here and on resize.
			m.viewportFor(key).SetExtent(msg.buf.LineCount(), m.contentHeight())
			if key == m.curKey() {
				// Startup and file-entry loads reveal the cursor's
				// latest target once its rows exist.
				m.revealCurrent()
			}
		}
	case popupExpiredMsg:
		// Only the live instance's own expiry dismisses the pop-up; a
		// stale instance's expiry cannot touch a newer pop-up.
		if m.popup != nil && m.popup.id == msg.id {
			m.popup = nil
		}
	case tea.KeyPressMsg:
		if m.state == stateCancelled {
			return m, nil
		}
		// Any key press dismisses the file-change pop-up; the key still
		// performs its normal action below in the same update.
		m.popup = nil
		if msg.String() == "ctrl+c" {
			return m.cancelRun()
		}
		if m.overlay != nil {
			return m.updateOverlay(msg)
		}
		switch msg.String() {
		case "q":
			switch m.state {
			case stateSearching:
				return m.cancelRun()
			case stateBrowse, stateNoResults, stateFatal:
				return m, tea.Quit
			}
		case "c":
			if m.state == stateBrowse {
				m.theme = m.theme.Toggle()
			}
		case "up", "down", "u", "d", "pgup", "pgdown":
			if m.state == stateBrowse {
				m.scrollCurrent(msg.String())
			}
		case "n", "p":
			if m.state == stateBrowse {
				return m.navigate(msg.String())
			}
		}
	}
	return m, nil
}

// rgSucceeded reports whether the child's wait status is an ordinary rg
// outcome: a clean exit or exit code 1 (no matches). Any other wait
// failure — another exit code, signal death, or a Wait error — is
// fatal.
func rgSucceeded(err error) bool {
	if err == nil {
		return true
	}
	var ex interface{ ExitCode() int }
	return errors.As(err, &ex) && ex.ExitCode() == 1
}

// enterBrowse moves a completed search into the browse state and starts
// loading the current file — the cursor's first stop.
func (m Model) enterBrowse() (tea.Model, tea.Cmd) {
	m.state = stateBrowse
	m.files = m.index.Files()
	m.fileIdx = make(map[string]int, len(m.files))
	for i, f := range m.files {
		m.fileIdx[string(f)] = i
	}
	return m.ensureLoaded()
}

// navigate applies one matched-line navigation key: n advances and p
// retreats the index cursor circularly. Every actual transition reveals
// the destination's target row — immediately when its file is cached,
// otherwise when the load completes. A stop in another file switches
// the panel — the departing file's viewport stays saved under its key —
// and an uncached destination's load is requested. A file change also
// opens the file-change pop-up with a fresh instance-keyed timer.
func (m Model) navigate(key string) (tea.Model, tea.Cmd) {
	if len(m.index.Stops()) < 2 {
		// Zero or one stop is a strict no-op: no reveal, pop-up, or retry.
		return m, nil
	}
	var mv searchindex.Move
	if key == "n" {
		mv = m.index.Next()
	} else {
		mv = m.index.Prev()
	}
	m.revealCurrent()
	if !mv.FileChanged {
		return m, nil
	}
	// The file-change pop-up starts at selection — never at load
	// completion — with a fresh instance keying its own expiry timer.
	stop, _ := m.index.Current()
	m.popupSeq++
	m.popup = &popup{id: m.popupSeq, path: append([]byte(nil), stop.Path...)}
	m, load := m.ensureLoaded()
	return m, tea.Batch(load, m.popupTimer(m.popup.id))
}

// revealCurrent applies the destination reveal to the current stop when
// its file is loaded: the file's saved vertical state — the top of the
// file on a first visit — is the starting point, and the viewport moves
// only when the target row is hidden from it. A file still loading has
// no rows to target; its reveal runs when the load result arrives.
func (m Model) revealCurrent() {
	stop, ok := m.index.Current()
	if !ok {
		return
	}
	key := string(stop.Path)
	src := m.buffers[key]
	if src == nil {
		return
	}
	m.viewportFor(key).Reveal(src.TargetRow(stop))
}

// ensureLoaded starts a load for the current stop's file unless it is
// already loaded or in flight — one load per raw path, never queued.
func (m Model) ensureLoaded() (Model, tea.Cmd) {
	stop, ok := m.index.Current()
	if !ok {
		return m, nil
	}
	key := string(stop.Path)
	if m.buffers[key] != nil || m.loading[key] {
		return m, nil
	}
	m.loading[key] = true
	return m, loadCmd(m.ctx, m.loadGate, stop, stopsForPath(m.index, stop.Path))
}

// stopsForPath collects the file's matched-line stops for the loader.
func stopsForPath(index *searchindex.Index, path []byte) []searchindex.Stop {
	var out []searchindex.Stop
	for _, s := range index.Stops() {
		if bytes.Equal(s.Path, path) {
			out = append(out, s)
		}
	}
	return out
}

// loadResult is the product of one file load, delivered to the model as
// a message: the raw path it belongs to and the prepared buffer or the
// read error.
type loadResult struct {
	path []byte
	buf  rowSource
	err  error
}

// loadCmd reads and maps a file off the update path. A non-nil gate
// holds the read and decode/map phase until it closes; ctx cancellation
// releases a held gate promptly. The completion message carries the
// prepared buffer so Update does no full-file work.
func loadCmd(ctx context.Context, gate <-chan struct{}, stop searchindex.Stop, stops []searchindex.Stop) tea.Cmd {
	resolved := append([]byte(nil), stop.Resolved...)
	path := append([]byte(nil), stop.Path...)
	return func() tea.Msg {
		if gate != nil {
			select {
			case <-gate:
			case <-ctx.Done():
			}
		}
		buf, err := filebuffer.Load(resolved, stops)
		return loadResult{path: path, buf: buf, err: err}
	}
}

// scrollCurrent applies a vertical scroll key to the current file's
// saved viewport. On a "Loading…" or "(unreadable)" placeholder — no
// loaded buffer — scrolling is a no-op.
func (m Model) scrollCurrent(key string) {
	path := m.curKey()
	if m.buffers[path] == nil {
		return
	}
	vp := m.viewportFor(path)
	switch key {
	case "up":
		vp.Up()
	case "down":
		vp.Down()
	case "u":
		vp.HalfUp()
	case "d":
		vp.HalfDown()
	case "pgup":
		vp.PageUp()
	case "pgdown":
		vp.PageDown()
	}
}

// viewportFor returns the file's saved vertical viewport, creating it
// on first visit so a later revisit resumes from the saved top row.
func (m Model) viewportFor(key string) *viewport.Viewport {
	v := m.vps[key]
	if v == nil {
		v = &viewport.Viewport{}
		m.vps[key] = v
	}
	return v
}

// curKey is the per-file map key of the current stop's raw path, or ""
// when the index has no stops.
func (m Model) curKey() string {
	stop, ok := m.index.Current()
	if !ok {
		return ""
	}
	return string(stop.Path)
}

// termSize returns the terminal dimensions, defaulting to 80x24 before
// the first WindowSizeMsg arrives.
func (m Model) termSize() (w, h int) {
	w, h = m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	return w, h
}

// contentHeight is the file panel's scrollable height in rendered rows:
// the panel height minus the filename row.
func (m Model) contentHeight() int {
	_, h := m.termSize()
	return h - 1
}

// cancelRun terminates the child, releases any gate-held collection,
// and quits with the cancellation status 130.
func (m Model) cancelRun() (tea.Model, tea.Cmd) {
	if m.cancel != nil {
		m.cancel()
	}
	if m.child != nil {
		m.child.Terminate()
	}
	m.state = stateCancelled
	m.status = 130
	return m, tea.Quit
}

// View renders the current screen.
func (m Model) View() tea.View {
	v := tea.NewView(m.screen())
	v.AltScreen = true
	return v
}

func (m Model) screen() string {
	var base string
	switch m.state {
	case stateBrowse:
		base = m.browseScreen()
	case stateNoResults:
		base = m.noResultsScreen()
	case stateFatal:
		// The fatal no-results outcome has no underlying state: the
		// overlay is the whole presentation.
		base = m.blankScreen()
	case stateCancelled:
		return ""
	default:
		base = m.theme.Base("Searching…")
	}
	if m.overlay != nil {
		return m.overlayScreen(base)
	}
	if m.popup != nil {
		return m.popupScreen(base)
	}
	return base
}

// blankScreen is h empty rows — the base under a stateless overlay.
func (m Model) blankScreen() string {
	h := m.height
	if h <= 0 {
		h = 24
	}
	return strings.Join(make([]string, h), "\n")
}

// noResultsScreen renders the centred outcome of a successful search
// that retained no usable results, appending the distinct exclusion
// count when matched files were confirmed binary.
func (m Model) noResultsScreen() string {
	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	msg := "No results found"
	if m.index != nil {
		if n := m.index.BinaryExcluded(); n > 0 {
			msg = fmt.Sprintf("%s (%d binary files skipped)", msg, n)
		}
	}
	pad := (w - displaywidth.String(msg)) / 2
	if pad < 0 {
		pad = 0
	}
	rows := make([]string, h)
	rows[h/2] = strings.Repeat(" ", pad) + msg
	return m.theme.Base(strings.Join(rows, "\n"))
}

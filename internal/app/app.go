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
	stderr  []byte
	procErr error
	status  int
	// overlay, when non-nil, is the open diagnostic overlay: the modal
	// error/warning presentation of a completed search.
	overlay *errOverlay

	// Browse state: the raw-path-ordered file list, the matched-line
	// cursor, per-path prepared buffers keyed by raw path bytes, and the
	// load bookkeeping that keeps one load in flight per path.
	files   [][]byte
	fileIdx map[string]int
	cursor  int
	buffers map[string]*filebuffer.Buffer
	loading map[string]bool
	failed  map[string]bool
	listTop int
	vp      viewport.Viewport
	theme   theme.Theme
}

// New returns a Model that collects the started child's stream. The
// child must already be running; spawn failures are handled by Run
// before the TUI exists.
func New(child Child, workdir string) Model {
	ctx, cancel := context.WithCancel(context.Background())
	return Model{
		child:   child,
		workdir: workdir,
		state:   stateSearching,
		ctx:     ctx,
		cancel:  cancel,
		buffers: make(map[string]*filebuffer.Buffer),
		loading: make(map[string]bool),
		failed:  make(map[string]bool),
		theme:   theme.Styled(),
	}
}

// Init starts collection and index preparation off the UI update path.
func (m Model) Init() tea.Cmd {
	ctx, child, workdir, gate := m.ctx, m.child, m.workdir, m.gate
	return func() tea.Msg {
		return collect(ctx, child, workdir, gate)
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
		if m.overlay != nil {
			if max := m.overlay.maxScroll(m.width, m.height); m.overlay.scroll > max {
				m.overlay.scroll = max
			}
		}
	case searchResult:
		if m.state == stateCancelled {
			// A late completion must not revive a cancelled run.
			return m, nil
		}
		m.index = msg.index
		m.stderr = msg.stderr
		m.procErr = msg.err
		oc := decideOutcome(outcomeInput{
			procErr:   msg.err,
			integrity: msg.integrity,
			report:    msg.report,
			usable:    msg.index.UsableResults(),
			stderr:    msg.stderr,
		})
		// The exit status is fixed here, at the outcome decision; only
		// ctrl+c overrides it later.
		m.status = oc.status
		if len(oc.diagnostics) > 0 {
			m.overlay = &errOverlay{lines: oc.diagnostics}
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
		} else {
			m.buffers[key] = msg.buf
			delete(m.failed, key)
		}
	case tea.KeyPressMsg:
		if m.state == stateCancelled {
			return m, nil
		}
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
// loading the current file.
func (m Model) enterBrowse() (tea.Model, tea.Cmd) {
	m.state = stateBrowse
	m.files = m.index.Files()
	m.fileIdx = make(map[string]int, len(m.files))
	for i, f := range m.files {
		m.fileIdx[string(f)] = i
	}
	m.cursor = 0
	return m.ensureLoaded()
}

// ensureLoaded starts a load for the current stop's file unless it is
// already loaded or in flight — one load per raw path, never queued.
func (m Model) ensureLoaded() (Model, tea.Cmd) {
	stops := m.index.Stops()
	if len(stops) == 0 {
		return m, nil
	}
	stop := stops[m.cursor]
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
	buf  *filebuffer.Buffer
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

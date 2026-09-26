package app

import (
	tea "charm.land/bubbletea/v2"
	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
	"vrg/internal/theme"
	"vrg/internal/viewport"
)

// phase is the model's coarse state.
type phase int

const (
	// phaseSearching covers the whole of collection and post-exit
	// processing: parsing, sorting, and index preparation. The app
	// stays here until the prepared index arrives, even after rg has
	// exited.
	phaseSearching phase = iota
	// phaseBrowse is the two-pane browse view: file list on the left,
	// the current file's content on the right.
	phaseBrowse
	// phaseNoResults is the centred "No results found" screen shown
	// after a complete search whose usable results — retained stops
	// after binary exclusion — are zero. Its fixed exit status is 1.
	phaseNoResults
	// phaseFatal is a fatal outcome with no usable results: there is
	// no underlying screen to return to, so the diagnostics overlay
	// stands alone and dismissing it quits.
	phaseFatal
)

// Model is the Bubble Tea application model: it owns the search
// lifecycle state and renders the searching and browse screens.
// cancel terminates the rg child and abandons collection; the process
// boundary confirms the reap before exiting. File loads run off the
// update path behind loadGate — nil in production, a test seam that
// holds the worker's read and decode/map phases.
type Model struct {
	phase    phase
	cancel   func()
	done     <-chan searchDoneMsg
	index    *searchindex.Index
	stops    []searchindex.Stop
	files    [][]byte // distinct raw paths in index order
	cursor   int      // index into stops of the current matched line
	bufs     map[string]*filebuffer.Buffer
	failed   map[string]bool
	loading  map[string]bool
	theme    theme.Theme
	vp       viewport.Viewport
	loadGate <-chan struct{}
	width    int
	height   int
	// binarySkipped is the distinct count of files dropped by binary
	// exclusion, reported on the no-results screen.
	binarySkipped int
	// overlay is the open diagnostics overlay, nil when none is up. It
	// owns the keyboard while open.
	overlay *overlay
	code    int
	quit    bool
}

// newModel returns a searching model awaiting the collection result on
// done; cancel terminates the search's child (nil means none).
func newModel(done <-chan searchDoneMsg, cancel func()) Model {
	if cancel == nil {
		cancel = func() {}
	}
	return Model{
		phase:   phaseSearching,
		cancel:  cancel,
		done:    done,
		bufs:    make(map[string]*filebuffer.Buffer),
		failed:  make(map[string]bool),
		loading: make(map[string]bool),
		theme:   theme.Dark(),
		// Same fallback the process boundary hands Bubble Tea; a real
		// terminal's first resize overrides it.
		width:  80,
		height: 24,
	}
}

// Init returns the command that waits for collection to finish and
// delivers the search-done message. Collection runs off this path; the
// model only receives its result.
func (m Model) Init() tea.Cmd {
	return func() tea.Msg { return <-m.done }
}

// Update applies one message. Resize is handled in any state so the UI
// stays responsive during collection and loads; a search-done message
// resolves the outcome — browse, no-results, or a fatal overlay, with
// the diagnostics overlay opening whenever the completion carries
// diagnostics — and starts the current file's load when browse is the
// underlying screen; a load-done message stores the prepared buffer
// without any full-file work here; q quits a completed state with its
// fixed exit status. An open overlay owns the keyboard: up/down scroll,
// q and Esc dismiss (quitting outright when nothing underlies it),
// other keys are ignored. ctrl+c in any state — and q while searching,
// which covers the post-exit preparation window — cancel the search and
// exit 130. Esc is a base-state no-op. Once the model has committed to
// quitting, late messages (including completions racing cancellation)
// are discarded.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.quit {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.relayout()
	case searchDoneMsg:
		m.index = msg.index
		var failures []string
		if m.index != nil {
			m.stops = m.index.Stops()
			m.binarySkipped = m.index.BinaryExcluded()
			failures = m.index.IntegrityFailures()
		}
		o := decideOutcome(msg.waitErr, msg.stderr, failures, len(m.stops), 0)
		m.code = o.code
		m.phase = o.screen
		if o.overlay {
			m.overlay = &overlay{lines: o.diags}
		}
		if m.phase != phaseBrowse {
			return m, nil
		}
		m.files = distinctPaths(m.stops)
		m.cursor = 0
		m.relayout()
		return m, m.ensureLoad()
	case loadDoneMsg:
		key := string(msg.path)
		delete(m.loading, key)
		if msg.err != nil {
			m.failed[key] = true
		} else {
			m.bufs[key] = msg.buf
		}
		m.relayout()
	case tea.KeyPressMsg:
		if m.overlay != nil {
			return m.overlayKey(msg.String())
		}
		switch msg.String() {
		case "ctrl+c":
			return m.cancelled(), tea.Quit
		case "c":
			// The colour toggle is a browse key; during searching
			// ordinary keys are inert.
			if m.phase == phaseBrowse {
				m.theme = m.theme.Toggle()
			}
		case "q":
			if m.phase == phaseSearching {
				return m.cancelled(), tea.Quit
			}
			m.quit = true
			return m, tea.Quit
		}
	}
	return m, nil
}

// cancelled marks the model quit-by-cancellation: exit 130 once the
// process boundary has terminated and reaped the child and restored the
// terminal.
func (m Model) cancelled() Model {
	m.quit = true
	m.code = 130
	m.cancel()
	return m
}

// View renders the current screen on the alternate screen; on every
// exit Bubble Tea emits the display-restoration sequence (leave alt
// screen, cursor visible). During the whole collection and post-exit
// preparation span the screen is "Searching…"; afterwards it is the
// two-pane browse view or, with no usable results, the centred
// no-results screen — or a blank frame when a fatal outcome left no
// underlying screen — with the diagnostics overlay composited on top
// while open. The theme's base style wraps each frame so the active
// scheme's colours cover the screen.
func (m Model) View() tea.View {
	var base string
	switch m.phase {
	case phaseBrowse:
		base = m.renderBrowse()
	case phaseNoResults:
		base = m.renderNoResults()
	case phaseFatal:
		// No underlying screen: a blank frame hosts the overlay.
		base = renderBlank(m.width, m.height)
	default:
		base = "Searching…\n"
	}
	if m.overlay != nil {
		base = m.renderOverlay(base)
	}
	v := tea.NewView(m.theme.Base(base))
	v.AltScreen = true
	return v
}

// ExitCode is the process exit status the model settled on.
func (m Model) ExitCode() int { return m.code }

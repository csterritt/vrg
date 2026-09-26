package app

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	tea "charm.land/bubbletea/v2"
	"vrg/internal/filebuffer"
	"vrg/internal/present"
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
// holds the worker's read and decode/map phases. diags is the session
// diagnostic collection: every diagnostic the model has processed, in
// collection order, independent of what any screen displayed. diagCh
// carries the collector's incremental stderr lines and diagAck is the
// test-only acknowledgement seam — both nil in plain unit-test models.
type Model struct {
	phase   phase
	cancel  func()
	done    <-chan searchDoneMsg
	diagCh  <-chan string
	diagAck io.Writer
	diags   []string
	index   *searchindex.Index
	stops   []searchindex.Stop
	files   [][]byte // distinct raw paths in index order
	bufs    map[string]*filebuffer.Buffer
	failed  map[string]bool
	loading map[string]bool
	// rows holds each loaded file's prepared rendered-row model — built
	// when its load completes and rebuilt when the layout or wrap mode
	// changes; revs is each path's content revision, bumped on every
	// successful load, and prepW/prepWrap record the layout the cached
	// models were prepared for. saved holds each file's vertical
	// viewport state — the top rendered row — for revisits (Issue #13).
	rows     map[string]viewport.Rows
	revs     map[string]int
	prepW    int
	prepWrap bool
	saved    map[string]int
	// wrap is the wrap-mode flag: on means lines wrap at grapheme
	// boundaries, off means run-off-edge with the reserved indicator
	// column. On initially; w toggles.
	wrap     bool
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
	// popupID is the active file-change pop-up's instance — zero when
	// none is up. popupSeq mints the instance IDs; popupPath is the raw
	// destination path the pop-up displays, captured at selection.
	// popupTimer builds the instance's expiry command — nil selects the
	// real one-second tick; tests substitute a synchronous or nil
	// command so batched navigation commands stay instant.
	popupID    int
	popupSeq   int
	popupPath  []byte
	popupTimer func(id int) tea.Cmd
	code       int
	quit       bool
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
		rows:    make(map[string]viewport.Rows),
		revs:    make(map[string]int),
		saved:   make(map[string]int),
		theme:   theme.Dark(),
		// Wrapping is on initially; w toggles run-off-edge and back.
		wrap: true,
		// Same fallback the process boundary hands Bubble Tea; a real
		// terminal's first resize overrides it.
		width:  80,
		height: 24,
	}
}

// Init returns the command that waits for the collector's next event:
// an incremental diagnostic line or the search-done completion.
// Collection runs off this path; the model only receives its messages.
func (m Model) Init() tea.Cmd {
	return m.awaitEvent
}

// awaitEvent is the collector event wait: the next diagnostic line or
// the search completion, whichever the collector delivers first. Update
// re-issues it for every diagnostic, so collector events are processed
// in emission order — all streamed diagnostics land before the
// completion that follows them.
func (m Model) awaitEvent() tea.Msg {
	select {
	case line := <-m.diagCh:
		return diagMsg{line: line}
	case msg := <-m.done:
		return msg
	}
}

// Update applies one message. Resize is handled in any state so the UI
// stays responsive during collection and loads; a search-done message
// resolves the outcome — browse, no-results, or a fatal overlay, with
// the diagnostics overlay opening whenever the completion carries
// diagnostics — and starts the current file's load when browse is the
// underlying screen; a load-done message stores the prepared buffer
// without any full-file work here; n/p step the matched-line cursor in
// the browse view; q quits a completed state with its fixed exit
// status. An open overlay owns the keyboard: up/down scroll,
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
		var failures, recordDiags []string
		recordLoss := 0
		if m.index != nil {
			m.stops = m.index.Stops()
			m.binarySkipped = m.index.BinaryExcluded()
			failures = m.index.IntegrityFailures()
			recordDiags = m.index.RecordDiagnostics()
			recordLoss = m.index.Malformed() + m.index.Oversized()
		}
		o := decideOutcome(msg.waitErr, msg.stderr, failures, len(m.stops), recordLoss, recordDiags)
		m.code = o.code
		m.phase = o.screen
		// The completion's own diagnostics join the collection. Child
		// stderr is absent here: it was already collected line-by-line
		// while the search ran, and collecting it again would break
		// exactly-once.
		for _, d := range completionDiagnostics(msg.waitErr, failures, recordDiags) {
			m.collect(d)
		}
		if o.overlay {
			m.openOverlay(o.diags)
		}
		if m.phase != phaseBrowse {
			return m, nil
		}
		m.files = distinctPaths(m.stops)
		m.relayout()
		return m, m.ensureLoad()
	case diagMsg:
		// The shutdown boundary: a diagnostic counts as collected once
		// the model has processed the message carrying it.
		m.CollectDiagnostic(msg.line)
		return m, m.awaitEvent
	case loadDoneMsg:
		key := string(msg.path)
		delete(m.loading, key)
		if msg.err != nil {
			m.failed[key] = true
			d := fmt.Sprintf("cannot read %s: %v", present.Path(msg.path), msg.err)
			m.CollectDiagnostic(d)
			// A current-file failure interrupts with the error
			// overlay; a non-current one stays diagnostic-only.
			if bytes.Equal(msg.path, m.currentPath()) {
				m.openOverlay(strings.Split(present.Diagnostic(d), "\n"))
			}
		} else {
			m.bufs[key] = msg.buf
			m.revs[key]++
			// The new content's row model is stale-keyed; relayout
			// rebuilds it for the current layout.
			delete(m.rows, key)
		}
		m.relayout()
		if bytes.Equal(msg.path, m.currentPath()) {
			// A load completing for the current file starts from its
			// saved vertical state — absent means a first visit, which
			// starts at the top — then reveals the latest selected
			// target over that starting point.
			m.vp.SetTop(m.saved[key])
			m.reveal()
		}
	case popupExpiredMsg:
		// Only the instance that scheduled this expiry answers it —
		// a stale instance's expiry cannot dismiss a newer pop-up.
		if msg.id == m.popupID {
			m.popupID = 0
		}
	case tea.KeyPressMsg:
		// Any key press dismisses the pop-up; the key still performs
		// its normal action in this same update.
		m.popupID = 0
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
		case "w":
			// The wrap toggle is a browse key: it flips between wrap
			// and run-off-edge modes, changing the reserved indicator
			// width and therefore the text width every cached row
			// model is rebuilt for.
			if m.phase == phaseBrowse {
				m.wrap = !m.wrap
				m.relayout()
			}
		case "q":
			if m.phase == phaseSearching {
				return m.cancelled(), tea.Quit
			}
			m.quit = true
			return m, tea.Quit
		case "up", "down", "u", "d", "pgup", "pgdown":
			m.scroll(msg.String())
		case "n", "p":
			// Matched-line navigation is a browse key; during
			// searching ordinary keys stay inert.
			if m.phase == phaseBrowse {
				return m.navigate(msg.String() == "n")
			}
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

// collect appends an already-sanitized diagnostic line to the session
// collection and reports the acknowledgement side channel when one is
// wired — one acknowledgement per collected diagnostic, the evidence
// the PTY harness waits for before sending an exit key.
func (m *Model) collect(d string) {
	m.diags = append(m.diags, d)
	if m.diagAck != nil {
		fmt.Fprintln(m.diagAck, "collected")
	}
}

// CollectDiagnostic sanitizes raw diagnostic text through the
// safe-presentation utility and appends it to the session collection.
// Processing the message that carries a diagnostic — never its mere
// arrival in a pipe — is what collects it, so a diagnostic still in
// flight at exit is neither waited for nor replayed.
func (m *Model) CollectDiagnostic(d string) {
	m.collect(present.Diagnostic(d))
}

// ReplayTo writes the session diagnostic collection to w, one line per
// collected occurrence in collection order. The process boundary runs
// it after terminal restoration on every controlled exit — ordinary
// quit, cancellation, controlled failure — and it never waits on
// in-flight work. The lines were sanitized when collected, so replay
// emits them verbatim; no persistent log is written.
func (m Model) ReplayTo(w io.Writer) {
	for _, d := range m.diags {
		fmt.Fprintln(w, d)
	}
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
	if m.popupID != 0 {
		base = m.renderPopup(base)
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

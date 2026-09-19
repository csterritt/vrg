package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
	"vrg/internal/theme"
	"vrg/internal/viewport"
)

// Config is the validated search invocation plus the process's I/O
// wiring.
type Config struct {
	// Args is the protected ripgrep argument vector, excluding the "rg"
	// program name.
	Args []string
	// Dir is the invocation working directory: rg runs there and
	// relative result paths resolve against it.
	Dir string
	// Err receives diagnostics such as the start-failure message; nil
	// means os.Stderr.
	Err io.Writer
	// Start spawns the search child; nil means spawn rg from PATH.
	Start StartFunc
}

// Option configures optional seams.
type Option func(*options)

type options struct {
	// gate, when set, runs after the child's stream is fully collected
	// and before index preparation — the hold proving the searching
	// state covers post-exit processing.
	gate func()
	// collectAck, when set, runs once collection completes, before gate.
	collectAck func()
	// reap, when set, is called once per process with the child's reaped
	// wait status — the side channel proving vrg's wait/reap path ran
	// rather than inferring reaping from a missing pid.
	reap func(Result)
	// fail, when set, is the injectable controlled-failure hook: a
	// non-nil return is a controlled application failure.
	fail func() error
	// diagAck, when set, runs once per diagnostic line after Update has
	// processed it into the session collection — the test-only
	// acknowledgement that a diagnostic is collected before an exit key.
	diagAck func()
	// loadGate, when set, runs inside each file-load command before the
	// read — the hold proving "Loading…" spans the whole load: disk read
	// plus decode and byte→cell mapping, all off the update path.
	loadGate func()
	// popupTimer, when set, builds each file-change pop-up's expiry
	// command in place of the real one-second tick — model tests
	// substitute an instantly resolving command so expiry is driven by
	// injected popupExpireMsg values, never by real time.
	popupTimer func(id int) tea.Cmd
}

// WithGate holds index preparation until fn returns.
func WithGate(fn func()) Option { return func(o *options) { o.gate = fn } }

// WithCollectAck runs fn once the child's output is fully collected.
func WithCollectAck(fn func()) Option { return func(o *options) { o.collectAck = fn } }

// WithReapReport calls fn once with the child's reaped wait status.
func WithReapReport(fn func(Result)) Option { return func(o *options) { o.reap = fn } }

// WithFailFunc installs the controlled-failure hook: a non-nil return
// from fn fails the application under vrg's control.
func WithFailFunc(fn func() error) Option { return func(o *options) { o.fail = fn } }

// WithDiagAck runs fn once per diagnostic line after Update has
// processed it into the session collection.
func WithDiagAck(fn func()) Option { return func(o *options) { o.diagAck = fn } }

// WithLoadGate holds each file load — read and decode/map together —
// until fn returns.
func WithLoadGate(fn func()) Option { return func(o *options) { o.loadGate = fn } }

type state int

const (
	stateSearching state = iota
	// stateNoResults is the centred "No results found" screen: a
	// complete successful search left no usable results.
	stateNoResults
	stateBrowse
	// stateOverlayOnly is the fatal outcome with no usable results: the
	// error overlay is the whole presentation, so dismissing it exits.
	stateOverlayOnly
)

// model is the Bubble Tea model: "Searching…" while collection and index
// preparation run off the update path, then the two-pane browse view, or
// the no-results screen when a complete search retains nothing usable.
type model struct {
	cfg   Config
	opts  options
	child Child

	state         state
	width, height int
	status        int
	// quitting marks that a controlled exit is underway; messages
	// arriving after it — including late search and load completions —
	// are discarded so they cannot revive the UI.
	quitting bool
	// failErr is a controlled application failure; its diagnostic enters
	// the session collection before shutdown and is replayed by the
	// common post-restoration stderr writer in Run.
	failErr error

	// diags is the session diagnostic collection — sanitized lines
	// appended in the order Update processed the messages carrying them,
	// independent of what any overlay displayed. Run replays it to
	// stderr after terminal restoration on every controlled exit.
	diags []string

	// Browse state. idx is the prepared search index; the current file
	// and current matched line derive from its circular matched-line
	// cursor (Issue #13): n advances, p retreats, both wrap, and manual
	// scrolling never moves it. Files load asynchronously: loading marks
	// the in-flight raw paths, bufs caches prepared buffers, and failed
	// records read failures, all keyed by the raw path bytes — never by
	// an escaped display form.
	// vps is the saved vertical viewport per file keyed by raw path:
	// scrolling writes through to it, and a destination reveal that
	// moves the viewport replaces it (Issue #14), so a file revisited
	// later starts from its last position. rows caches each loaded
	// file's prepared
	// rendered-row model — built when its load completes — which the
	// frame render slices instead of rescanning the buffer.
	idx     *searchindex.Index
	loading map[string]bool
	bufs    map[string]*filebuffer.Buffer
	failed  map[string]bool
	vps     map[string]viewport.Viewport
	rows    map[string]rowSource
	theme   theme.Theme

	// Modal error overlay. overlayOpen marks it up — key input routes
	// to it ahead of the base state, and only ctrl+c outranks it.
	// overlayExit marks a fatal overlay with no underlying state:
	// dismissal exits with the fixed status instead of revealing a
	// screen. overlayText is the escaped diagnostic body; it re-wraps
	// to the interior width on every render, and overlayScroll — the
	// first visible wrapped row — clamps to the complete row set.
	overlayOpen   bool
	overlayExit   bool
	overlayText   string
	overlayScroll int

	// File-change pop-up (Issue #15). popupID is the live instance's
	// identity — 0 means none is up; popupSeq mints each new ID so an
	// expiry message matches exactly the instance it was minted for.
	// The pop-up's content derives from the cursor and current
	// terminal size at render time, so a resize recentres and
	// re-truncates it without touching the instance or its timer.
	popupID  int
	popupSeq int
}

func newModel(cfg Config, opts options, child Child) *model {
	return &model{
		cfg:     cfg,
		opts:    opts,
		child:   child,
		state:   stateSearching,
		loading: map[string]bool{},
		bufs:    map[string]*filebuffer.Buffer{},
		failed:  map[string]bool{},
		vps:     map[string]viewport.Viewport{},
		rows:    map[string]rowSource{},
		theme:   theme.Dark(),
	}
}

// searchDoneMsg carries the collected result and prepared index from the
// collection command into Update.
type searchDoneMsg struct {
	res Result
	idx *searchindex.Index
}

// failMsg carries a controlled application failure from the injected
// failure hook into Update.
type failMsg struct{ err error }

// stderrLineMsg carries one drained child-stderr line — terminator
// retained — into the session collection. Incremental delivery is what
// lets a diagnostic be collected while the child still runs, ahead of
// any exit decision.
type stderrLineMsg struct{ text string }

// Init starts collection and, when the failure hook is installed, the
// hook itself. The collection command blocks on the child, runs the
// collection acknowledgement and the preparation gate, then builds the
// index — all off the UI update path so the model stays responsive.
func (m *model) Init() tea.Cmd {
	child := m.child
	opts := m.opts
	dir := m.cfg.Dir
	collect := func() tea.Msg {
		res := child.Wait()
		if opts.collectAck != nil {
			opts.collectAck()
		}
		if opts.gate != nil {
			opts.gate()
		}
		return searchDoneMsg{res: res, idx: searchindex.Build(res.Stdout, dir)}
	}
	if opts.fail == nil {
		return collect
	}
	fail := opts.fail
	return tea.Batch(collect, func() tea.Msg {
		return failMsg{err: fail()}
	})
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.quitting {
		// A controlled exit is underway: discard everything still in
		// flight so late work cannot revive the UI.
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clampOverlayScroll()
		// The new content height can strand a saved top past the last
		// valid position; re-clamp every prepared file's viewport so no
		// revisit can leave avoidable blank rows below EOF.
		for key, vp := range m.vps {
			if rows := m.rows[key]; rows != nil {
				vp.Clamp(rows.Len(), m.contentRows())
				m.vps[key] = vp
			}
		}
	case searchDoneMsg:
		m.idx = msg.idx
		var cmd tea.Cmd
		in := OutcomeInput{
			Result:    msg.res,
			Integrity: msg.idx.Integrity(),
			Usable:    msg.idx.UsableResults(),
			RecordLoss: RecordLoss{
				Malformed: msg.idx.Malformed,
				Oversized: msg.idx.Oversized,
				Paths:     msg.idx.OversizedPaths,
			},
			Warnings: recordWarnings(msg.idx),
		}
		outcome := DecideOutcome(in)
		m.collectSearchDiags(msg.res, in)
		// The search-derived status is fixed once searching completes;
		// only ctrl+c overrides it afterwards.
		m.status = outcome.Status
		switch outcome.Presentation {
		case presentNoResults:
			m.state = stateNoResults
		case presentOverlayOnly:
			m.state = stateOverlayOnly
		default:
			m.state = stateBrowse
			cmd = m.startLoad()
		}
		if len(outcome.Overlay) > 0 {
			m.openOverlay(strings.Join(outcome.Overlay, "\n"), outcome.DismissExits)
		}
		return m, cmd
	case stderrLineMsg:
		m.collectDiags(splitDiagnostic([]byte(msg.text))...)
	case fileLoadedMsg:
		key := string(msg.path)
		delete(m.loading, key)
		if msg.err != nil {
			m.failed[key] = true
			// A load failure is a collected diagnostic whether or not
			// an overlay ever shows it — Issue #26 owns the display
			// side (current-file overlay, non-current silence).
			m.collectDiags(loadDiag(msg.path, msg.err))
		} else {
			m.bufs[key] = msg.buf
			// The prepared row model is what the frame render slices;
			// a saved viewport from an earlier visit re-clamps to the
			// new content.
			m.rows[key] = viewport.Prepare(msg.buf)
			if vp, ok := m.vps[key]; ok {
				vp.Clamp(m.rows[key].Len(), m.contentRows())
				m.vps[key] = vp
			}
			// A load completing for the current file runs the
			// file-change reveal sequence against the latest cursor
			// target — covering the startup file's first visit.
			if ck, ok := m.curKey(); ok && ck == key {
				m.reveal()
			}
		}
	case failMsg:
		if msg.err == nil {
			return m, nil
		}
		m.failErr = msg.err
		// The failure diagnostic enters the session collection before
		// shutdown; the common post-restoration writer replays it
		// exactly once — there is no separate direct write.
		m.collectDiags("vrg: " + safepresentation.EscapePath([]byte(msg.err.Error())))
		m.quitting = true
		return m, m.quitCmd()
	case popupExpireMsg:
		// An expiry dismisses only the pop-up instance it was minted
		// for; a stale instance's expiry is discarded.
		if msg.id == m.popupID {
			m.popupID = 0
		}
	case tea.KeyPressMsg:
		key := msg.Keystroke()
		// Any key press dismisses a file-change pop-up and still
		// performs its normal action in the same update — the pop-up
		// never delays navigation or quitting.
		m.popupID = 0
		switch {
		case key == "ctrl+c":
			// ctrl+c has global precedence: cancellation in every state.
			m.status = 130
			m.quitting = true
			return m, m.quitCmd()
		case m.overlayOpen:
			// The modal overlay takes precedence over base-state keys:
			// up and down scroll the complete wrapped diagnostic,
			// q and Esc dismiss it, and every other key is ignored.
			switch key {
			case "up":
				m.scrollOverlay(-1)
			case "down":
				m.scrollOverlay(1)
			case "q", "esc":
				if m.overlayExit {
					// A fatal overlay has no underlying state:
					// dismissal exits with the fixed status —
					// the one place Esc terminates.
					m.quitting = true
					return m, m.quitCmd()
				}
				m.overlayOpen = false
			}
		case key == "q" && m.state == stateSearching:
			// q while searching — including gate-held index
			// preparation after rg has exited — is cancellation.
			m.status = 130
			m.quitting = true
			return m, m.quitCmd()
		case key == "c":
			// c toggles the colour scheme between dark and light for
			// the session; nothing persists.
			m.theme = m.theme.Toggled()
		case m.state == stateBrowse && (key == "n" || key == "p"):
			// n advances and p retreats the circular matched-line
			// cursor; crossing into another file's stop switches the
			// panel and requests that file's load when uncached.
			return m, m.navigate(key == "n")
		case m.state == stateBrowse && isScrollKey(key):
			// Manual vertical scrolling moves the current file's
			// viewport; it never moves the matched-line cursor, and
			// on a placeholder it is a no-op.
			m.scrollBy(key)
		case key == "q" && m.state == stateNoResults:
			// q dismisses the no-results screen to exit 1 through
			// the same cleanup path; Esc is a no-op here and
			// ctrl+c keeps its 130 override.
			m.quitting = true
			return m, m.quitCmd()
		case key == "q" && m.state == stateBrowse:
			// q in ordinary browsing exits with the fixed
			// search-derived status through the same cleanup path.
			m.quitting = true
			return m, m.quitCmd()
		}
	}
	return m, nil
}

// recordWarnings composes the caller's warning diagnostics from the
// index's counts: the unknown event-type tally is a warning — it never
// independently changes the exit status.
func recordWarnings(ix *searchindex.Index) []string {
	if ix.Unknown > 0 {
		return []string{fmt.Sprintf("%d unrecognised record types skipped", ix.Unknown)}
	}
	return nil
}

// quitCmd is the single cleanup path every controlled exit routes
// through: terminate and reap the child, report the reaped status, then
// quit so the program restores the terminal.
func (m *model) quitCmd() tea.Cmd {
	child, report := m.child, m.opts.reap
	return func() tea.Msg {
		reapChild(child, report)
		return tea.QuitMsg{}
	}
}

// reapChild terminates a still-running child and waits for it, ending
// Issue #3's drainage promptly because termination closes its pipes.
// Wait is idempotent, so running it again for an already-finished or
// already-reaped child is harmless.
func reapChild(c Child, report func(Result)) {
	c.Terminate()
	res := c.Wait()
	if report != nil {
		report(res)
	}
}

// collectDiags appends sanitized diagnostic lines to the session
// collection in the order Update processed the messages carrying them.
// That order is the shutdown boundary: a diagnostic is collected once
// its message is processed and is replayed exactly once after terminal
// restoration; a message still in flight at exit is never waited for.
// The test-only acknowledgement runs once per collected line.
func (m *model) collectDiags(lines ...string) {
	for _, line := range lines {
		m.diags = append(m.diags, line)
		if m.opts.diagAck != nil {
			m.opts.diagAck()
		}
	}
}

// collectSearchDiags collects the search-completion diagnostics in
// outcome order. Child stderr is normally collected line-by-line as it
// drains (stderrLineMsg), so completion collects only the generated
// process line — when a failed child left no stderr — plus the
// integrity, record-loss, and warning tail. A child without incremental
// diagnostics owes its whole captured stderr here.
func (m *model) collectSearchDiags(res Result, in OutcomeInput) {
	switch {
	case len(res.Stderr) > 0:
		if m.child.Diags() == nil {
			m.collectDiags(splitDiagnostic(res.Stderr)...)
		}
	case processFailed(res):
		m.collectDiags(failedProcessLine(res))
	}
	m.collectDiags(tailDiags(in)...)
}

func (m *model) View() tea.View {
	s := center("Searching…", m.width, m.height)
	switch m.state {
	case stateNoResults:
		s = center(m.noResultsText(), m.width, m.height)
	case stateBrowse:
		s = m.browseView()
	case stateOverlayOnly:
		// The fatal overlay has no underlying state: it floats over a
		// blank frame.
		s = ""
	}
	if m.popupID != 0 {
		s = m.compositePopup(s)
	}
	if m.overlayOpen {
		s = m.compositeOverlay(s)
	}
	v := tea.NewView(m.theme.Base(s))
	v.AltScreen = true
	return v
}

// noResultsText is the empty-search screen's single line: "No results
// found", plus the binary-skip suffix when every matched file was
// excluded as binary.
func (m *model) noResultsText() string {
	if m.idx != nil && m.idx.BinaryExcluded > 0 {
		return fmt.Sprintf("No results found (%d binary files skipped)", m.idx.BinaryExcluded)
	}
	return "No results found"
}

// Run is the whole search lifecycle: spawn the child, show "Searching…"
// until the stream is collected and the index prepared, then the
// two-pane browse view. A start failure prints a sanitized diagnostic to
// cfg.Err and
// returns exit 2 without entering the TUI. Every controlled exit —
// ordinary, cancellation, or a controlled application failure —
// terminates and reaps the child and restores the terminal; the session
// diagnostic collection — everything the model processed, whether or
// not an overlay showed it — is then replayed to stderr exactly once
// each, in collection order, through replayDiags. No persistent log is
// written.
func Run(ctx context.Context, cfg Config, opts ...Option) int {
	errOut := cfg.Err
	if errOut == nil {
		errOut = os.Stderr
	}
	start := cfg.Start
	if start == nil {
		start = spawn
	}
	child, err := start(ctx, cfg.Args, cfg.Dir)
	if err != nil {
		fmt.Fprintf(errOut, "vrg: cannot start ripgrep: %s\n", safepresentation.EscapePath([]byte(err.Error())))
		return 2
	}
	var o options
	for _, fn := range opts {
		fn(&o)
	}
	if o.reap != nil {
		// Both the model's quit command and the post-program safety net
		// run the wait/reap path; the report must fire exactly once.
		var once sync.Once
		report := o.reap
		o.reap = func(r Result) { once.Do(func() { report(r) }) }
	}
	m := newModel(cfg, o, child)
	prog := tea.NewProgram(m, tea.WithContext(ctx))
	if ch := child.Diags(); ch != nil {
		// Forward each drained stderr line into the model as it
		// arrives; Send is a no-op once the program has exited, so the
		// forwarder drains to EOF without stalling the child's
		// drainage.
		go func() {
			for line := range ch {
				prog.Send(stderrLineMsg{text: line})
			}
		}()
	}
	final, err := prog.Run()
	// Exits that bypass the model — interrupt, program error, a caught
	// panic — still owe the child termination and reaping.
	reapChild(child, o.reap)
	fm, _ := final.(*model)
	var diags []string
	if fm != nil {
		diags = fm.diags
	}
	if err != nil {
		if errors.Is(err, tea.ErrInterrupted) {
			replayDiags(errOut, diags)
			return 130
		}
		// A runtime failure never reached the session collection; it
		// is appended after the collected diagnostics and emitted by
		// the same post-restoration writer — exactly once.
		replayDiags(errOut, append(diags, "vrg: "+safepresentation.EscapePath([]byte(err.Error()))))
		return 2
	}
	replayDiags(errOut, diags)
	if fm == nil || fm.failErr != nil {
		return 2
	}
	return fm.status
}

// replayDiags is the common post-restoration stderr writer: every
// collected diagnostic line is emitted exactly once, in collection
// order, and it runs only after the program has returned and the
// terminal is restored. Collected lines are already sanitized — child
// stderr through the diagnostic escaper, embedded filenames through the
// path escaper — so the writer adds only the line framing.
func replayDiags(w io.Writer, lines []string) {
	for _, line := range lines {
		fmt.Fprintln(w, line)
	}
}

// center pads s into a w×h field, centered horizontally and vertically.
// Unknown dimensions degrade to the bare string.
func center(s string, w, h int) string {
	if w <= 0 || h <= 0 {
		return s
	}
	pad := 0
	if n := utf8.RuneCountInString(s); n < w {
		pad = (w - n) / 2
	}
	top := 0
	if h > 1 {
		top = (h - 1) / 2
	}
	return strings.Repeat("\n", top) + strings.Repeat(" ", pad) + s
}

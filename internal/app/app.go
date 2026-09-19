package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

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
	// RunProgram, when set, runs the constructed Bubble Tea program in
	// place of Program.Run — the executable's program-runner seam,
	// consumed only by the vrg_testhooks build variant. Nil means call
	// Run directly.
	RunProgram func(*tea.Program) (tea.Model, error)
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
	// decodeGate, when set, runs inside each file-load command after
	// the read and before the decode/map phase — the hold proving that
	// phase is separately off the update path and input stays
	// actionable while it pends (Issue #25).
	decodeGate func()
	// popupTimer, when set, builds each file-change pop-up's expiry
	// command in place of the real one-second tick — model tests
	// substitute an instantly resolving command so expiry is driven by
	// injected popupExpireMsg values, never by real time.
	popupTimer func(id int) tea.Cmd
	// layoutGate, when set, runs inside each layout-preparation command
	// before the row model is built — the hold proving preparation is
	// off the update path and input stays actionable while it pends.
	layoutGate func()
	// escapePath, when set, replaces the safe-presentation path
	// escaper — the render-cost seam proving path display metadata
	// is prepared once at search completion and a frame never
	// re-escapes paths (Issue #40).
	escapePath func([]byte) string
	// loader, when set, replaces filebuffer.Read inside each file-load
	// command — the injected-loader seam making read failures
	// deterministic in model tests rather than depending on
	// filesystem permission bits (Issue #26).
	loader func(path []byte) ([]byte, error)
	// diagSink, when set, is Run's retained snapshot of the session
	// collection: collectDiags mirrors every appended line into it, so
	// the collected diagnostics reach the post-restoration replay even
	// when the program's final model is absent or the wrong type — the
	// final-model assertion is not the collection's only channel
	// (Issue #46). Run always installs it; it is a shutdown contract,
	// not a test seam.
	diagSink *[]string
	// event, when set, records one acknowledgement per processed
	// message and per awaited transition — the Issue #48 PTY
	// handshake seam. It observes production behaviour and timing;
	// it never changes them.
	event func(string)
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

// WithDecodeGate holds each load's decode/map phase — after the read
// and before the buffer is built — until fn returns.
func WithDecodeGate(fn func()) Option { return func(o *options) { o.decodeGate = fn } }

// WithLayoutGate holds each layout preparation until fn returns.
func WithLayoutGate(fn func()) Option { return func(o *options) { o.layoutGate = fn } }

// WithEscapePath substitutes the file-list path escaper.
func WithEscapePath(fn func([]byte) string) Option {
	return func(o *options) { o.escapePath = fn }
}

// WithLoader substitutes the file-load command's read phase.
func WithLoader(fn func([]byte) ([]byte, error)) Option {
	return func(o *options) { o.loader = fn }
}

// WithEventAck installs the acknowledgement recorder: fn is called
// once per Update-processed message and once per awaited transition —
// a key processed, a state entered, an overlay opened or dismissed, a
// load settled, a layout installed, a diagnostic collected — the
// deterministic handshakes the PTY harness waits on (Issue #48).
func WithEventAck(fn func(string)) Option { return func(o *options) { o.event = fn } }

type state int

const (
	// minTermWidth and minTermHeight are the fixed terminal minimum —
	// 20 columns by 3 rows (PRD Layout and indicators). Below either,
	// the model installs the too-small gate (Issue #33).
	minTermWidth  = 20
	minTermHeight = 3
	// tooSmallNote is the message the gate centres on the frame,
	// clipped to what fits.
	tooSmallNote = "Terminal too small"
)

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
	// sized marks that a terminal size has been reported; until then
	// the too-small gate stays off and the model still opens on
	// "Searching…" (Issue #33).
	sized  bool
	status int
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
	// stderr after terminal restoration on every controlled exit,
	// through the diagSink snapshot it installs so the replay survives
	// an absent or wrong-type final model.
	diags []string

	// Browse state. idx is the prepared search index; the current file
	// and current matched line derive from its circular matched-line
	// cursor (Issue #13): n advances, p retreats, both wrap, and manual
	// scrolling never moves it. Files load asynchronously: loading
	// records the in-flight request's identity per raw path — minted
	// from loadSeq, at most one per path — so a completion updates
	// only the request it answers (Issue #25); bufs caches prepared
	// buffers for the session, failed marks paths whose last settled
	// load failed, and failDiag keeps each such path's latest failure
	// diagnostic — the content the re-entry overlay shows (Issue #26)
	// — all keyed by the raw path bytes, never an escaped display form.
	// vps is the saved vertical viewport per file keyed by raw path:
	// scrolling writes through to it, and a destination reveal that
	// moves the viewport replaces it (Issue #14), so a file revisited
	// later starts from its last position. rows caches each loaded
	// file's prepared rendered-row model — installed when a keyed
	// completion arrives still matching the live layout — which the
	// frame render slices instead of rescanning the buffer. revs is the
	// per-path content revision feeding the row model's key: it bumps
	// on every successful load, so a reload's model never aliases the
	// old content's. layoutReqs records the newest in-flight
	// preparation key per path, deduplicating requests; pendingReveals
	// records per-path destination-reveal intents that could not run
	// against a missing or stale layout and commit when a matching
	// completion installs (Issue #17); pendingAnchor records per-path
	// reload-anchor intents — preserve the anchor, no reveal —
	// minted when a reload's load completes and committed through the
	// same installation path (Issue #27, generalized by Issue #28).
	// displayPaths is the per-file display metadata prepared once at
	// search completion (Issue #40) — each file's escaped path text,
	// grapheme-cluster boundaries, and full cell width in index
	// order — so the frame render truncates only the visible entries
	// against the current list width instead of re-escaping or
	// re-segmenting paths per frame. listWBase is the largest
	// prepared path width, kept for the Issue #24 width formula so
	// it never rescans the file list per frame.
	// listVisible is the user's file-list visibility preference
	// (Issue #24): shown at startup, hidden by left/tab, shown by
	// right/shift+tab — a computed zero width never changes it.
	// notes is the per-file buffer-status note the filename row's
	// slot renders — Issue #24 provides the slot; Issue #29 supplies
	// the text.
	// wrap is the session's wrap mode (Issue #16): on initially,
	// toggled by w between wrapped rows and run-off-edge clipping.
	idx            *searchindex.Index
	loading        map[string]int
	loadSeq        int
	bufs           map[string]*filebuffer.Buffer
	failed         map[string]bool
	failDiag       map[string]string
	vps            map[string]viewport.Viewport
	rows           map[string]rowSource
	revs           map[string]int
	layoutReqs     map[string]viewport.Key
	pendingReveals map[string]bool
	pendingAnchor  map[string]bool
	displayPaths   []displayPath
	listWBase      int
	listVisible    bool
	notes          map[string]string
	wrap           bool
	theme          theme.Theme

	// Modal error overlay. overlayOpen marks it up — key input routes
	// to it ahead of the base state, and only ctrl+c outranks it.
	// overlayExit marks a fatal overlay with no underlying state:
	// dismissal exits with the fixed status instead of revealing a
	// screen. overlay is its scrollBox: the escaped diagnostic body
	// re-wraps to the interior width on every render, and scroll —
	// the first visible wrapped row — clamps to the complete row set.
	// helpOpen marks the Issue #31 help dialog up: it sits below the
	// error overlay in the precedence stack — an error opening while
	// help is up suspends it, and dismissing the error restores help
	// at its retained scroll position (Issue #32).
	overlayOpen bool
	overlayExit bool
	overlay     scrollBox
	helpOpen    bool
	help        scrollBox

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
		cfg:            cfg,
		opts:           opts,
		child:          child,
		state:          stateSearching,
		loading:        map[string]int{},
		bufs:           map[string]*filebuffer.Buffer{},
		failed:         map[string]bool{},
		failDiag:       map[string]string{},
		vps:            map[string]viewport.Viewport{},
		rows:           map[string]rowSource{},
		revs:           map[string]int{},
		layoutReqs:     map[string]viewport.Key{},
		pendingReveals: map[string]bool{},
		pendingAnchor:  map[string]bool{},
		listVisible:    true,
		notes:          map[string]string{},
		wrap:           true,
		theme:          theme.Dark(),
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
	m.ack("state:searching")
	collect := func() tea.Msg {
		res := child.Wait()
		if opts.collectAck != nil {
			opts.collectAck()
		}
		m.ack("collected")
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
	res, cmd := m.update(msg)
	// The Issue #48 acknowledgement seam records one event per
	// processed message — the key-processing and model-transition
	// boundaries the PTY harness waits on. A message discarded under
	// quitting earns no record.
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		m.ack("key:" + msg.Keystroke())
	case tea.WindowSizeMsg:
		m.ack("size")
	case searchDoneMsg:
		m.ack("state:" + m.state.name())
	case stderrLineMsg:
		m.ack("stderrline")
	case fileLoadedMsg:
		m.ack("fileloaded")
	case layoutReadyMsg:
		m.ack("layoutready")
	case failMsg:
		if msg.err != nil {
			m.ack("fail")
		} else {
			m.ack("fail:nil")
		}
	case popupExpireMsg:
		m.ack("popup")
	default:
		m.ack("other")
	}
	return res, cmd
}

func (m *model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.sized = true
		if m.tooSmall() {
			// Below the minimum the gate holds: no ordinary
			// layout is installed at the pathological
			// dimensions, and no anchor, viewport, or modal
			// state is touched — the preserved state is what
			// the first size back above the minimum recovers
			// (Issue #33).
			return m, nil
		}
		m.clampOverlayScroll()
		m.clampHelpScroll()
		// The new text width re-keys every prepared layout: the
		// current file's replacement is requested here and prepared
		// off the update path — the retained logical anchor, not the
		// row ordinal, carries the position into it. Saved viewports
		// whose installed layout still matches (a pure height change)
		// re-clamp now; stale ones restore when their fresh layout
		// installs, so no revisit can leave avoidable blank rows
		// below EOF.
		var cmd tea.Cmd
		if ck, ok := m.curKey(); ok {
			cmd = m.requestLayout(ck)
		}
		for key, vp := range m.vps {
			if rows := m.currentRows(key); rows != nil {
				vp.Restore(rows, m.contentRows())
				m.vps[key] = vp
			}
		}
		return m, cmd
	case searchDoneMsg:
		m.idx = msg.idx
		// Every file's width-independent display metadata — the
		// escaped path text, its grapheme-cluster boundaries, and
		// its full cell width — is prepared once here, where the
		// index is finalized: navigation and frame rendering read
		// the shared entries and never re-escape, re-segment, or
		// rescan the file list (Issue #40). The list width's
		// longest-path term is the largest prepared width.
		m.displayPaths = make([]displayPath, len(msg.idx.Files))
		m.listWBase = 0
		for i, f := range msg.idx.Files {
			m.displayPaths[i] = newDisplayPath(m.escapePath(f.Path))
			if m.displayPaths[i].width > m.listWBase {
				m.listWBase = m.displayPaths[i].width
			}
		}
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
		if req, ok := m.loading[key]; !ok || req != msg.req {
			// Not a live request's answer — an unrequested, stale, or
			// already-settled completion: discard it without touching
			// the cache, the failure record, or the diagnostics.
			m.ack("load:stale")
			return m, nil
		}
		delete(m.loading, key)
		if msg.err != nil {
			m.failed[key] = true
			// A failed reload replaces the old display with
			// "(unreadable)": the stale buffer and its layout are
			// dropped so old content is never presented as refreshed
			// (Issue #27). Every load failure is a collected
			// diagnostic and the path's recorded prior failure; only
			// a current file's is also modal — the overlay opens with
			// it or, while an overlay is already up, appends the
			// single new occurrence without moving the reader's
			// scroll position. A non-current failure stays
			// diagnostic-only (Issue #26).
			delete(m.bufs, key)
			delete(m.rows, key)
			delete(m.notes, key)
			diag := loadDiag(msg.path, msg.err)
			m.failDiag[key] = diag
			m.collectDiags(diag)
			if ck, ok := m.curKey(); ok && ck == key {
				m.openOverlay(diag, false)
			}
			m.ack("load:fail")
		} else {
			delete(m.failDiag, key)
			m.bufs[key] = msg.buf
			m.revs[key]++
			// Issue #29's stale mark recomputes on every load: a
			// buffer whose recorded submatches failed validation
			// carries the filename row's "file changed since search"
			// note until a load validates fully again.
			if msg.buf.Stale() {
				m.notes[key] = staleNote
			} else {
				delete(m.notes, key)
			}
			// A UTF-16/32 BOM marks the load unsupported (Issue #30):
			// the panel keeps its "(unsupported encoding)" placeholder
			// — no file text, no highlights — and the explanatory
			// diagnostic is collected once per detection, modal only
			// while the file is current — Issue #26's
			// current/non-current distinction. No layout is prepared:
			// the placeholder has no rows.
			if enc := msg.buf.Unsupported(); enc != "" {
				diag := encodingDiag(msg.path, enc)
				m.collectDiags(diag)
				if ck, ok := m.curKey(); ok && ck == key {
					m.openOverlay(diag, false)
				}
			}
			// Stage one of the two-stage completion is limited to
			// filing the result — no row-based decision runs here:
			// the intent owed to the latest selection is recorded in
			// the model and commits when the matching prepared layout
			// installs (Issue #28). A non-reload completion owes the
			// file-change reveal sequence against the latest cursor
			// target — covering the startup file's first visit; a
			// reload owes no reveal — its intent is anchor
			// preservation (Issue #27). A reveal already pending —
			// navigation during the load, including away-and-back —
			// replaces the reload's anchor intent: navigation intent,
			// never cursor equality, decides the commit (Issue #28).
			if ck, ok := m.curKey(); ok && ck == key {
				if msg.reload {
					if !m.pendingReveals[key] {
						m.pendingAnchor[key] = true
					}
				} else {
					delete(m.pendingAnchor, key)
					m.pendingReveals[key] = true
				}
			}
			m.ack("load:ok")
			return m, m.requestLayout(key)
		}
	case layoutReadyMsg:
		path := msg.key.Path
		if m.layoutReqs[path] == msg.key {
			delete(m.layoutReqs, path)
		}
		if m.tooSmall() {
			// The gate installs no ordinary layout — not even
			// a completion minted before it went up; recovery
			// re-requests at the size that lifts it
			// (Issue #33).
			m.ack("layout:stale")
			return m, nil
		}
		buf := m.bufs[path]
		if buf == nil || msg.key != m.layoutKey(path, buf) {
			// Obsolete: prepared for parameters since superseded, or
			// for content no longer the revision on record — discard
			// it without touching the installed layout, the saved
			// viewports, the anchors, or the pending intents.
			m.ack("layout:stale")
			return m, nil
		}
		_, had := m.rows[path]
		m.rows[path] = msg.rows
		if vp, ok := m.vps[path]; ok {
			vp.Restore(msg.rows, m.contentRows())
			m.vps[path] = vp
		}
		// Intents owed to this file commit against the freshly
		// installed layout when it is still current: a pending
		// reveal — or a first visit's initial reveal — runs the
		// file-change reveal sequence; a pending reload-anchor
		// intent is done by the anchor restore above — preserve the
		// position, no reveal (Issue #27). A pending reveal takes
		// precedence over an anchor intent.
		if ck, ok := m.curKey(); ok && ck == path {
			switch {
			case m.pendingReveals[path]:
				delete(m.pendingReveals, path)
				m.reveal()
			case m.pendingAnchor[path]:
				delete(m.pendingAnchor, path)
			case !had:
				m.reveal()
			}
		}
		m.ack("layout")
		return m, nil
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
		if m.tooSmall() {
			return m.tooSmallKey(key)
		}
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
		case key == "r" && m.state == stateBrowse && !m.helpOpen:
			// r is the explicit reload: reread the current file from
			// disk — never rerunning rg — while preserving the cursor
			// and the viewport anchor. It also works while an error
			// overlay is open — though never under help — and is the
			// only retry route a one-stop index has (Issue #27).
			return m, m.startReload()
		case m.overlayOpen:
			// The modal overlay takes precedence over help and the
			// base-state keys: up and down scroll the complete wrapped
			// diagnostic, q and Esc dismiss it, and every other key is
			// ignored.
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
				m.ack("overlay:dismissed")
			}
		case m.helpOpen:
			// The help dialog is modal over the base state: up and
			// down scroll the complete wrapped table, q, Esc, h, and
			// ? close it, and every other key is ignored — nothing
			// reaches the content behind (Issue #31).
			switch key {
			case "up":
				m.scrollHelp(-1)
			case "down":
				m.scrollHelp(1)
			case "q", "esc", "h", "?":
				m.helpOpen = false
			}
		case (key == "h" || key == "?") && (m.state == stateBrowse || m.state == stateNoResults):
			// h and ? open modal help over ordinary browsing and the
			// no-results screen; closing returns to the underlying
			// base state. Opening cancels any file-change pop-up.
			m.openHelp()
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
		case key == "w" && m.state == stateBrowse:
			// w toggles wrap mode at once: wrapped rows versus
			// run-off-edge clipping. The current file's layout is
			// re-keyed — the text width changes with the reserved
			// indicator column — and prepared off the update path;
			// the retained anchor restores when it installs.
			m.wrap = !m.wrap
			var cmd tea.Cmd
			if ck, ok := m.curKey(); ok {
				cmd = m.requestLayout(ck)
			}
			return m, cmd
		case m.state == stateBrowse && (key == "left" || key == "tab"):
			// left and tab hide the file list (Issue #24). The text
			// width changes, so the current file's layout is
			// re-keyed through the prepared-layout path and the
			// retained anchor carries the reading position across.
			m.listVisible = false
			var cmd tea.Cmd
			if ck, ok := m.curKey(); ok {
				cmd = m.requestLayout(ck)
			}
			return m, cmd
		case m.state == stateBrowse && (key == "right" || key == "shift+tab"):
			// right and shift+tab show the file list again.
			m.listVisible = true
			var cmd tea.Cmd
			if ck, ok := m.curKey(); ok {
				cmd = m.requestLayout(ck)
			}
			return m, cmd
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
		case m.state == stateBrowse && isPanKey(key):
			// , . < > [ ] pan horizontally in run-off-edge mode —
			// one cell, ten cells, or half the text width — clamped
			// to the widest visible line's paintable boundary
			// (Issue #18). In wrap mode and on placeholders they are
			// strict no-ops.
			m.panBy(key)
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

// escapePath is the file-list path escaper — the safe-presentation
// core, or the test seam counting escaper queries. Search completion
// runs it once per file to build the display metadata every later
// consumer shares; the Issue #40 cost guards fail if it is queried
// again on the navigation or render path.
func (m *model) escapePath(p []byte) string {
	if m.opts.escapePath != nil {
		return m.opts.escapePath(p)
	}
	return safepresentation.EscapePath(p)
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

// ack emits one acknowledgement record through the test-only event
// seam; a nil seam — every production build — is inert.
func (m *model) ack(event string) {
	if m.opts.event != nil {
		m.opts.event(event)
	}
}

// name is the state's acknowledgement-event spelling: the record the
// test seam writes when the state is entered.
func (s state) name() string {
	switch s {
	case stateNoResults:
		return "noresults"
	case stateBrowse:
		return "browse"
	case stateOverlayOnly:
		return "overlayonly"
	default:
		return "searching"
	}
}

// tooSmall reports whether the reported terminal size is below the
// fixed minimum — under 20 columns or under 3 rows — so the gate
// replaces the ordinary presentation and key map (Issue #33). Before
// any size is reported the gate stays off: the model still opens on
// "Searching…".
func (m *model) tooSmall() bool {
	return m.sized && (m.width < minTermWidth || m.height < minTermHeight)
}

// tooSmallKey is the too-small screen's entire key map (Issue #33):
// only q and ctrl+c act. ctrl+c keeps its global 130; q exits with the
// state-applicable outcome — 130 while searching, otherwise the status
// the completed search fixed (0 or 2 browsing, 1 on no-results, 2 on
// the fatal overlay) — exiting past a logically open overlay rather
// than merely dismissing it. That precedence beats Issue #32's
// dismissal semantics so the screen can never trap the user behind an
// invisible modal. Esc and every other key are strict no-ops: no
// pop-up dismissal, no state change — a logically open overlay stays
// open for the recovery render.
func (m *model) tooSmallKey(key string) (tea.Model, tea.Cmd) {
	if key != "q" && key != "ctrl+c" {
		return m, nil
	}
	if key == "ctrl+c" || m.state == stateSearching {
		m.status = 130
	}
	m.quitting = true
	return m, m.quitCmd()
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
		if m.opts.diagSink != nil {
			*m.opts.diagSink = append(*m.opts.diagSink, line)
		}
		if m.opts.diagAck != nil {
			m.opts.diagAck()
		}
		m.ack("diag")
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
	var s string
	if m.tooSmall() {
		// The gate's whole presentation is the centred note,
		// clipped to what fits — the state, modal, and pop-up
		// renderings behind it stay suspended, never composited
		// (Issue #33).
		s = center(clipCells(tooSmallNote, m.width), m.width, m.height)
	} else {
		s = center("Searching…", m.width, m.height)
		switch m.state {
		case stateNoResults:
			s = center(m.noResultsText(), m.width, m.height)
		case stateBrowse:
			s = m.browseView()
		case stateOverlayOnly:
			// The fatal overlay has no underlying state: it floats
			// over a blank frame.
			s = ""
		}
		if m.popupID != 0 {
			s = m.compositePopup(s)
		}
		if m.helpOpen {
			s = m.compositeBox(s, &m.help)
		}
		if m.overlayOpen {
			s = m.compositeBox(s, &m.overlay)
		}
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

// invalidFinalModelDiag names the shutdown condition in which
// program.Run returned no usable final model — nil or a foreign type —
// so that return shape's exit is never silent.
const invalidFinalModelDiag = "vrg: program ended without a valid final model"

// Run is the whole search lifecycle: spawn the child, show "Searching…"
// until the stream is collected and the index prepared, then the
// two-pane browse view. A start failure prints a sanitized diagnostic to
// cfg.Err and
// returns exit 2 without entering the TUI. Every controlled exit —
// ordinary, cancellation, or a controlled application failure —
// terminates and reaps the child and restores the terminal; the session
// diagnostic collection — everything the model processed, whether or
// not an overlay showed it — is then replayed to stderr exactly once
// each, in collection order, through replayDiags. The collection reaches
// the replay through Run's own snapshot, so it survives a nil or
// wrong-type final model — that shape instead adds the
// invalid-final-model diagnostic and exits 2 — while a program.Run
// error is appended once after the collection. No persistent log is
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
	// sessionDiags is the shutdown snapshot: collectDiags mirrors every
	// collected line into it as Update runs, so Run replays the session
	// diagnostics even when the program's final model is absent or the
	// wrong type — the post-Run assertion is no longer their only
	// channel (Issue #46).
	var sessionDiags []string
	o.diagSink = &sessionDiags
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
	run := cfg.RunProgram
	if run == nil {
		run = func(p *tea.Program) (tea.Model, error) { return p.Run() }
	}
	final, err := run(prog)
	// Exits that bypass the model — interrupt, program error, a caught
	// panic — still owe the child termination and reaping.
	reapChild(child, o.reap)
	fm, _ := final.(*model)
	// Every Run() return shape converges on the one shutdown replay:
	// the program has returned — the terminal is restored — and the
	// child is terminated and reaped, so now the retained session
	// diagnostics are emitted in collection order, then a diagnostic
	// naming an absent or wrong-type final model (never a silent
	// exit), then the runtime error itself appended exactly once.
	lines := sessionDiags
	if fm == nil {
		lines = append(lines, invalidFinalModelDiag)
	}
	if err != nil && !errors.Is(err, tea.ErrInterrupted) {
		// A runtime failure never reached the session collection; it
		// is appended after the collected diagnostics and emitted by
		// the same post-restoration writer — exactly once.
		lines = append(lines, "vrg: "+safepresentation.EscapePath([]byte(err.Error())))
	}
	replayDiags(errOut, lines)
	if errors.Is(err, tea.ErrInterrupted) {
		return 130
	}
	if err != nil || fm == nil || fm.failErr != nil {
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
	if n := safepresentation.CellWidth(s); n < w {
		pad = (w - n) / 2
	}
	top := 0
	if h > 1 {
		top = (h - 1) / 2
	}
	return strings.Repeat("\n", top) + strings.Repeat(" ", pad) + s
}

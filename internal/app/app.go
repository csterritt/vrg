package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
	"vrg/internal/theme"
	"vrg/internal/viewport"
)

// intent identifies the action a pending layout install commits — the
// model carries it so that obsolete completions can neither consume
// nor mutate it.
type intent int

const (
	// intentNone is no pending action: an install commits nothing.
	intentNone intent = iota
	// intentReveal is the destination reveal owed to the latest cursor
	// selection once a matching layout installs (Issues 14 and 17).
	intentReveal
	// intentReloadAnchor is the explicit-reload contract: preserve the
	// logical viewport anchor, clamped to the new content, with no
	// reveal (Issue 27).
	intentReloadAnchor
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

// resultIndex is the model's read surface over the finalized search
// index: the circular matched-line cursor plus the whole-list
// enumerators. The enumerators belong to the one-time browse
// preparation at search completion; navigation and rendering read the
// precomputed per-file structures instead, so a keystroke or a frame
// never rescans the index (Issue 40). The interface is also the test
// seam that proves it: a double guarding the enumerators fails any
// whole-index access on the transition/render path.
type resultIndex interface {
	Current() (searchindex.Stop, bool)
	Next() searchindex.Move
	Prev() searchindex.Move
	Stops() []searchindex.Stop
	Files() [][]byte
	UsableResults() int
	BinaryExcluded() int
}

// fileEntry is one retained file's immutable browse data, prepared once
// when the search completes: the raw path identity, its measured
// display name — the escaped text, its grapheme-cluster boundaries, and
// its full cell width, all width-independent — and the file's
// navigation-stop group for the loader. Navigation and frame rendering
// share these structures; neither regroups nor re-measures per
// keystroke (Issue 40).
type fileEntry struct {
	raw   []byte
	name  safepresentation.MeasuredText
	stops []searchindex.Stop
}

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
	// layoutGate, when non-nil, is awaited inside the layout worker
	// before a row model is built, so tests can hold a layout off the
	// update path while input stays live.
	layoutGate <-chan struct{}
	// ctx and cancel drive cancellation of collection and gated loads:
	// cancelling releases a gate-held worker promptly.
	ctx    context.Context
	cancel context.CancelFunc

	width  int
	height int
	state  state

	index   resultIndex
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
	// acks is the per-run acknowledgement log, shared across the
	// model's value copies like diags: Update records each processed
	// message and committed transition so the test seam can wait on
	// occurrences rather than elapsed time.
	acks *events
	// overlay, when non-nil, is the open diagnostic overlay: the modal
	// error/warning presentation of a completed search.
	overlay *scrollOverlay
	// help, when non-nil, is the open modal key-binding help overlay.
	// The error overlay takes precedence: an error arriving while help
	// is open suspends help, retaining its scroll position, and closing
	// the error restores it.
	help *scrollOverlay
	// popup, when non-nil, is the active file-change pop-up; popupSeq
	// mints each pop-up's instance ID, and popupTimer builds the
	// instance's expiry command — a seam so tests drive expiry with
	// injected messages instead of the clock.
	popup      *popup
	popupSeq   int
	popupTimer func(id int) tea.Cmd

	// Browse state: the raw-path-ordered file list, per-path loaded
	// content and its installed row model keyed by raw path bytes, and
	// the load bookkeeping that keeps one load in flight per path:
	// loading records each path's in-flight request identity — minted
	// by loadSeq — so a completion can be matched to the request it
	// answers and a stale or unsolicited one discarded. failed holds
	// each path's latest read-failure diagnostic — the prior-failure
	// line a re-entry shows in the overlay — and doubles as the
	// "(unreadable)" placeholder state once no load for the path is in
	// flight. unsupported likewise holds each path's latest
	// encoding-detection diagnostic: the line a re-entry shows in the
	// explanatory overlay and the "(unsupported encoding)" placeholder
	// state once the detected buffer — cached like ordinary content —
	// has settled. The matched-line cursor lives in the index; the current
	// file derives from it. vps holds each visited file's saved
	// vertical viewport under the same key, so a revisited file resumes
	// from its saved position — a width-independent logical anchor
	// inside each Viewport. wrap is the wrap mode — on by default; w
	// toggles it. revs counts each path's content revisions for
	// row-model keying. pendingIntent marks an intent — a navigation
	// or entry reveal, or a reload's anchor preservation — that no
	// current layout could commit; the next matching layout install
	// commits it. reloading marks each path whose in-flight load is an
	// explicit r reload, so its completion records the anchor-preserving
	// intent rather than a destination reveal. listWidest is the file
	// list's widest entry in cells, computed once at browse entry so a
	// frame render never rescans the list. listVisible is the user's
	// show/hide preference — a computed zero-width column draws
	// nothing without touching it. itemName, when non-nil, renders
	// entry i's display name — a test seam proving the frame queries
	// only the visible window. notes holds each path's buffer-status
	// note for the filename-row slot; Issues 26, 29, and 30 populate
	// it. loader is the file-read seam — fileLoader in production, a
	// test substitute for failing and controlled loads.
	files         []fileEntry
	fileIdx       map[string]int
	buffers       map[string]*viewport.RowModel
	sources       map[string]viewport.Source
	revs          map[string]int
	loading       map[string]int
	loadSeq       int
	failed        map[string]string
	unsupported   map[string]string
	notes         map[string]string
	loader        loaderFunc
	listTop       int
	listWidest    int
	listVisible   bool
	itemName      func(i int) string
	vps           map[string]*viewport.Viewport
	wrap          bool
	pendingIntent intent
	reloading     map[string]bool
	theme         theme.Theme
}

// New returns a Model that collects the started child's stream. The
// child must already be running; spawn failures are handled by Run
// before the TUI exists.
func New(child Child, workdir string) Model {
	ctx, cancel := context.WithCancel(context.Background())
	return Model{
		child:       child,
		workdir:     workdir,
		state:       stateSearching,
		ctx:         ctx,
		cancel:      cancel,
		diags:       &diagnostics{},
		acks:        &events{},
		buffers:     make(map[string]*viewport.RowModel),
		sources:     make(map[string]viewport.Source),
		revs:        make(map[string]int),
		loading:     make(map[string]int),
		failed:      make(map[string]string),
		unsupported: make(map[string]string),
		loader:      fileLoader,
		notes:       make(map[string]string),
		vps:         make(map[string]*viewport.Viewport),
		reloading:   make(map[string]bool),
		wrap:        true,
		listVisible: true,
		theme:       theme.Styled(),
		popupTimer:  popupTick,
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

// Update applies messages to the model and records the acknowledgement
// events each processed message commits — one msg record per message,
// a key record per key press, and the transition records — on the
// run's shared acknowledgement log. The records observe the applied
// transition; they never change it.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// A load result counts only while the request it answers is still
	// in flight; the request table is a map shared across the model's
	// value copies, so the admission check is captured before update
	// consumes the entry.
	load, isLoad := msg.(loadResult)
	loadOK := false
	if isLoad {
		req, ok := m.loading[string(load.path)]
		loadOK = ok && req == load.req
	}
	next, cmd := m.update(msg)
	n, ok := next.(Model)
	if !ok {
		return next, cmd
	}
	n.acks.noteUpdate(m, n, msg, loadOK)
	return n, cmd
}

// update applies messages to the model. Collection results arrive as
// searchResult and enter the browse view; file loads arrive as
// loadResult carrying a prepared buffer keyed by raw path and request
// identity, and prepared row layouts arrive as layoutResult keyed by
// the parameters they were built for.
// Keys act per state — q cancels while searching and quits while
// browsing, and ctrl+c cancels in any state. Esc is not an exit key and
// is a no-op outside overlays. Below the minimum terminal size the
// too-small gate narrows input to q and ctrl+c alone; the resize gate
// similarly defers all layout reconciliation until an adequate size.
func (m Model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.tooSmall() {
			// The gate defers all reconciliation: no keyed layout is
			// requested and no saved position — viewport or modal —
			// is clamped at sizes that cannot hold them. Recovery
			// runs once, at the first adequate size.
			return m, nil
		}
		if m.overlay != nil {
			if max := m.overlay.maxScroll(m.width, m.height); m.overlay.scroll > max {
				m.overlay.scroll = max
			}
		}
		if m.help != nil {
			if max := m.help.maxScroll(m.width, m.height); m.help.scroll > max {
				m.help.scroll = max
			}
		}
		return m, m.resizeLayout()
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
			m.overlay = &scrollOverlay{lines: oc.diagnostics}
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
		req, ok := m.loading[key]
		if !ok || req != msg.req {
			// A completion counts only while the request it answers is
			// still in flight: anything else — a stale duplicate, an
			// unsolicited result — is discarded without touching the
			// path's cache, status, or bookkeeping.
			return m, nil
		}
		delete(m.loading, key)
		reload := m.reloading[key]
		delete(m.reloading, key)
		if msg.err != nil {
			line := "cannot read " + safepresentation.EscapePath(msg.path) +
				": " + safepresentation.EscapePath([]byte(readReason(msg.err)))
			m.failed[key] = line
			delete(m.unsupported, key)
			delete(m.buffers, key)
			delete(m.sources, key)
			m.diags.add(line)
			if m.state == stateBrowse && key == m.curKey() {
				// A current-file failure interrupts with the error
				// overlay — an open one gains exactly one appended
				// occurrence without moving the reader — while a
				// non-current failure stays a diagnostic only.
				m.showFailure(line)
			}
		} else {
			m.revs[key]++
			m.sources[key] = msg.src
			delete(m.failed, key)
			// The Issue 29 buffer-status note rides the filename-row
			// slot: recomputed on every completed load — first load and
			// reload alike — it stands while any recorded submatch
			// failed validation and clears only on fully validating
			// content.
			if src, ok := msg.src.(staleNoter); ok && src.Stale() {
				m.notes[key] = staleNote
			} else {
				delete(m.notes, key)
			}
			// Issue 30: a BOM-detected UTF-16/32 file keeps its prepared
			// source cached like ordinary content — the indexed stops
			// stay navigable and nothing reloads until r — but presents
			// the "(unsupported encoding)" placeholder and an
			// explanatory diagnostic under the same current/non-current
			// notification split as a read failure.
			if src, ok := msg.src.(unsupportedNoter); ok && src.Unsupported() != "" {
				line := "cannot display " + safepresentation.EscapePath(msg.path) +
					": unsupported encoding " + src.Unsupported()
				m.unsupported[key] = line
				m.diags.add(line)
				if m.state == stateBrowse && key == m.curKey() {
					m.showFailure(line)
				}
			} else {
				delete(m.unsupported, key)
			}
			if key == m.curKey() {
				// A reload's completion records the anchor-preserving
				// intent — unless a reveal is already owed to a
				// selection made during the load; the newest intent
				// wins.
				if reload && m.pendingIntent != intentReveal {
					m.pendingIntent = intentReloadAnchor
				}
				// The panel's layout is prepared off the update path;
				// the placeholder — or a previous layout — stays on
				// screen until the keyed result lands.
				return m, m.prepareLayout()
			}
		}
	case layoutResult:
		if m.state == stateCancelled {
			return m, nil
		}
		key := msg.key.Path
		cur := m.curKey()
		src := m.sources[key]
		// A prepared layout installs only while its complete key still
		// names the current file and parameters: anything older — a
		// superseded width or wrap mode, a stale content revision, a
		// no-longer-current file — is discarded without touching the
		// display, the anchor, saved per-file state, or a pending
		// intent.
		if key != cur || src == nil || msg.key != m.rowKey(cur, src) {
			return m, nil
		}
		m.buffers[key] = msg.model
		m.viewportFor(key).SetLayout(msg.model, m.contentHeight())
		// The pending intent commits now: a reveal runs the destination
		// rules against the installed rows, while the reload-anchor
		// intent is the anchor-preserving install itself — SetLayout
		// already mapped the anchor, clamped to the new content.
		if m.pendingIntent == intentReveal {
			// The committed reveal is the file-change sequence: the
			// horizontal offset — retained from a superseded layout
			// or the placeholder — resets to zero before the
			// minimal horizontal reveal.
			m.viewportFor(key).ResetOffset()
			m.revealCurrent()
		} else {
			m.pendingIntent = intentNone
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
		if m.tooSmall() {
			return m.updateTooSmall(msg)
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
		if m.help != nil {
			return m.updateHelp(msg)
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
		case "w":
			if m.state == stateBrowse {
				m.wrap = !m.wrap
				return m, m.prepareLayout()
			}
		case "left", "tab":
			if m.state == stateBrowse {
				m.listVisible = false
				return m, m.prepareLayout()
			}
		case "right", "shift+tab":
			if m.state == stateBrowse {
				m.listVisible = true
				return m, m.prepareLayout()
			}
		case "up", "down", "u", "d", "pgup", "pgdown":
			if m.state == stateBrowse {
				m.scrollCurrent(msg.String())
			}
		case ",", ".", "<", ">", "[", "]":
			if m.state == stateBrowse {
				m.panCurrent(msg.String())
			}
		case "n", "p":
			if m.state == stateBrowse {
				return m.navigate(msg.String())
			}
		case "r":
			if m.state == stateBrowse {
				return m.reload()
			}
		case "h", "?":
			if m.state == stateBrowse || m.state == stateNoResults {
				// The pop-up was already dismissed above: opening help
				// cancels it for good.
				m.help = openHelp()
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
// staging the current file — the cursor's first stop. The finalized
// index is enumerated exactly once, here: the ordered file entries —
// each raw path's measured display name and navigation-stop group —
// plus the path→entry map and the list's widest entry are the immutable
// structures every later frame and navigation shares, so neither half
// of a keystroke's update/render rescans the index.
func (m Model) enterBrowse() (tea.Model, tea.Cmd) {
	m.state = stateBrowse
	m.files, m.listWidest = nil, 0
	m.fileIdx = make(map[string]int)
	for _, s := range m.index.Stops() {
		key := string(s.Path)
		i, ok := m.fileIdx[key]
		if !ok {
			i = len(m.files)
			m.fileIdx[key] = i
			m.files = append(m.files, fileEntry{
				raw:  s.Path,
				name: safepresentation.MeasureText(safepresentation.EscapePath(s.Path)),
			})
			if d := m.files[i].name.Width() + 2; d > m.listWidest {
				m.listWidest = d
			}
		}
		m.files[i].stops = append(m.files[i].stops, s)
	}
	// The startup selection's reveal is a pending intent until the
	// file's first layout installs.
	m.pendingIntent = intentReveal
	return m.ensureStaged()
}

// navigate applies one matched-line navigation key: n advances and p
// retreats the index cursor circularly. Every actual transition reveals
// the destination's target row — immediately when its installed layout
// matches the current parameters, otherwise as a pending intent when
// the matching layout installs. A stop in another file switches the
// panel — the departing file's viewport stays saved under its key — and
// an uncached destination's load is requested while a stale cached one
// gets a keyed layout request. A file change also opens the file-change
// pop-up with a fresh instance-keyed timer — except when the
// destination is a previously failed file, whose re-entry opens the
// prior-failure overlay and stages its retry instead.
func (m Model) navigate(key string) (tea.Model, tea.Cmd) {
	if m.index.UsableResults() < 2 {
		// Zero or one stop is a strict no-op: no reveal, pop-up, or retry.
		return m, nil
	}
	var mv searchindex.Move
	if key == "n" {
		mv = m.index.Next()
	} else {
		mv = m.index.Prev()
	}
	stop, _ := m.index.Current()
	if mv.FileChanged {
		// A file change resets the horizontal offset to zero before
		// the destination reveal: stored per-file pan does not carry
		// across files.
		m.viewportFor(string(stop.Path)).ResetOffset()
	}
	m.revealCurrent()
	if !mv.FileChanged {
		return m, nil
	}
	if diag, bad := m.failed[string(stop.Path)]; bad {
		// Re-entering a previously failed file from a different file
		// shows the prior failure immediately — the overlay, not the
		// pop-up — while its retry load is staged. When that path's
		// load is somehow already in flight, the staging request is
		// dropped per Issue 25's one-load-per-path rule and the
		// in-flight load's settlement drives the placeholder.
		m.showFailure(diag)
		m, stage := m.ensureStaged()
		return m, stage
	}
	if diag, un := m.unsupported[string(stop.Path)]; un {
		// Re-entering a detected unsupported file re-shows its
		// explanatory overlay — the same notification a current-file
		// detection gives — but mints no reload: the detected buffer
		// is cached content, stable until r like any other.
		m.showFailure(diag)
		m, stage := m.ensureStaged()
		return m, stage
	}
	// The file-change pop-up starts at selection — never at load
	// completion — with a fresh instance keying its own expiry timer.
	m.popupSeq++
	m.popup = &popup{id: m.popupSeq, path: append([]byte(nil), stop.Path...)}
	m, stage := m.ensureStaged()
	return m, tea.Batch(stage, m.popupTimer(m.popup.id))
}

// showFailure presents one file-read diagnostic on the modal overlay: a
// closed overlay opens with the line and an open one gains it as an
// appended occurrence that leaves the reader's scroll position alone.
// Opening the overlay cancels any pop-up for good.
func (m *Model) showFailure(line string) {
	m.popup = nil
	if m.overlay == nil {
		m.overlay = &scrollOverlay{lines: []string{line}}
		return
	}
	m.overlay.append(line)
}

// revealCurrent applies the destination reveal to the current stop when
// its file's installed layout matches the present parameters: the
// file's saved vertical state — the top of the file on a first visit —
// is the starting point, and the viewport moves only when the target
// row is hidden from it. The same reveal performs the minimal
// horizontal reveal of the target cell's cluster in run-off-edge mode,
// so startup and every navigation action — including same-file n/p —
// reveal both axes. When no layout can serve — the file is loading, or
// its installed layout is stale under the current geometry — the intent
// is marked pending instead and the next matching layout install
// commits it; obsolete completions never consume it.
func (m *Model) revealCurrent() {
	stop, ok := m.index.Current()
	if !ok {
		return
	}
	key := string(stop.Path)
	if !m.layoutCurrent(key) {
		m.pendingIntent = intentReveal
		return
	}
	m.pendingIntent = intentNone
	vp := m.viewportFor(key)
	vp.SetLayout(m.buffers[key], m.contentHeight())
	vp.Reveal(stop)
}

// layoutCurrent reports whether the file's installed layout still
// matches the current terminal geometry, wrap mode, and content
// revision — the only layout a reveal may read.
func (m Model) layoutCurrent(key string) bool {
	src := m.sources[key]
	model := m.buffers[key]
	if src == nil || model == nil {
		return false
	}
	return model.Key() == m.rowKey(key, src)
}

// reload applies the explicit r reload of the current file: one reread
// of the same path, never a rerun of rg and never a change to the
// cursor stops. While the load is in flight the panel reads "Loading…"
// and the filename row still identifies the path; the placeholder's
// change to content or "(unreadable)" is the completion signal. A
// request while that path's load is already in flight is dropped
// whole, not queued — the admission check is the commit point, ahead
// of every reload-state mutation, so a dropped r leaves revision,
// pending intent, and presentation untouched and the in-flight load
// completes under its original classification (Issue 42). On a
// previously failed file r is the retry route, re-showing the
// prior-failure overlay while the retry runs — the same presentation a
// cross-file re-entry gives.
func (m Model) reload() (tea.Model, tea.Cmd) {
	stop, ok := m.index.Current()
	if !ok {
		return m, nil
	}
	key := string(stop.Path)
	if _, ok := m.loading[key]; ok {
		return m, nil
	}
	if diag, bad := m.failed[key]; bad {
		m.showFailure(diag)
	}
	// The cached display is dropped at request time: stale content is
	// never presented as refreshed, and with no installed source a
	// layout prepared for the old revision can never satisfy the
	// install guard over the placeholder.
	delete(m.buffers, key)
	delete(m.sources, key)
	m.reloading[key] = true
	return m.startLoad(stop)
}

// ensureStaged makes the current stop's file displayable at the present
// parameters: an uncached file's load is requested — one per raw path,
// never queued — a cached file whose installed layout is stale gets a
// keyed layout request, and a file already prepared for the current
// parameters needs neither.
func (m Model) ensureStaged() (Model, tea.Cmd) {
	stop, ok := m.index.Current()
	if !ok {
		return m, nil
	}
	key := string(stop.Path)
	if m.sources[key] == nil {
		if _, ok := m.loading[key]; ok {
			return m, nil
		}
		return m.startLoad(stop)
	}
	return m, m.prepareLayout()
}

// startLoad records and launches a fresh load request for the stop's
// path. The caller has already decided admission — one load per raw
// path, never queued — so a minted request is never dropped.
func (m Model) startLoad(stop searchindex.Stop) (Model, tea.Cmd) {
	key := string(stop.Path)
	m.loadSeq++
	m.loading[key] = m.loadSeq
	return m, loadCmd(m.ctx, m.loadGate, m.loader, m.loadSeq, stop, m.fileStops(key))
}

// fileStops returns the raw path's navigation-stop group from the
// per-file structure prepared at browse entry — a shared lookup, never
// a re-derived scan of the index.
func (m Model) fileStops(key string) []searchindex.Stop {
	return m.files[m.fileIdx[key]].stops
}

// loadResult is the product of one file load, delivered to the model as
// a message: the raw path it belongs to, the identity of the request it
// answers, and the prepared source the row model is built from — or the
// read error. The model applies it only while that exact request is
// still in flight for that path.
type loadResult struct {
	path []byte
	req  int
	src  viewport.Source
	err  error
}

// staleNote is the Issue 29 filename-row buffer-status note for a file
// whose recorded submatches no longer all validate against the loaded
// content.
const staleNote = "file changed since search"

// staleNoter is the buffer-status seam for the Issue 29 note: a
// prepared source that reports whether stale-match validation dropped
// any recorded submatch. *filebuffer.Buffer implements it.
type staleNoter interface{ Stale() bool }

// unsupportedNoter is the encoding-status seam for Issue 30: a prepared
// source that reports the name of a BOM-detected encoding the panel
// does not present — "" for supported content. *filebuffer.Buffer
// implements it.
type unsupportedNoter interface{ Unsupported() string }

// loaderFunc reads and prepares one file for display: the resolved raw
// path and the file's stops in, the prepared source or the read error
// out. Model.loader is the injection seam for failing and controlled
// loads — tests substitute it rather than arranging filesystem
// permissions.
type loaderFunc func(resolved []byte, stops []searchindex.Stop) (viewport.Source, error)

// fileLoader is the production loader: filebuffer.Load adapted to the
// seam's Source result — a nil Buffer is a nil source.
func fileLoader(resolved []byte, stops []searchindex.Stop) (viewport.Source, error) {
	buf, err := filebuffer.Load(resolved, stops)
	var src viewport.Source
	if buf != nil {
		src = buf
	}
	return src, err
}

// readReason reduces a file-load error to the reason a diagnostic
// reports: a path error's wrapped cause — the wrapper's own message
// would repeat the raw path the diagnostic already carries in escaped
// form — or the error's own text when it wraps no path. The result is
// escaped at the composition site like any other substituted text.
func readReason(err error) string {
	var pe *fs.PathError
	for errors.As(err, &pe) && pe.Err != nil {
		err = pe.Err
	}
	return err.Error()
}

// loadCmd reads and maps a file off the update path. A non-nil gate
// holds the read and decode/map phase until it closes; ctx cancellation
// releases a held gate promptly. The completion message carries the
// request identity and the prepared source so Update does no full-file
// decoding; the row model is built at install time so it always matches
// the current text width and wrap mode.
func loadCmd(ctx context.Context, gate <-chan struct{}, loader loaderFunc, req int, stop searchindex.Stop, stops []searchindex.Stop) tea.Cmd {
	resolved := append([]byte(nil), stop.Resolved...)
	path := append([]byte(nil), stop.Path...)
	return func() tea.Msg {
		if gate != nil {
			select {
			case <-gate:
			case <-ctx.Done():
			}
		}
		src, err := loader(resolved, stops)
		return loadResult{path: path, req: req, src: src, err: err}
	}
}

// layoutResult is the product of one prepared-layout job, delivered to
// the model as a message: the complete key the row model was built for
// plus the model itself. Installation is guarded on the key still
// matching the current parameters — a superseded result is discarded
// without touching state.
type layoutResult struct {
	key   viewport.RowModelKey
	model *viewport.RowModel
}

// layoutCmd builds a row model off the update path. A non-nil gate
// holds the build until it closes; ctx cancellation releases a held
// gate promptly. The completion carries the key it was built for so a
// superseded result can be discarded on arrival.
func layoutCmd(ctx context.Context, gate <-chan struct{}, key viewport.RowModelKey, src viewport.Source) tea.Cmd {
	return func() tea.Msg {
		if gate != nil {
			select {
			case <-gate:
			case <-ctx.Done():
			}
		}
		return layoutResult{key: key, model: viewport.NewRowModel(key, src)}
	}
}

// prepareLayout requests a prepared row model for the current file at
// the present parameters — or nothing when the installed layout already
// matches. The request is keyed by (path, content revision, text width,
// wrap mode); the completion installs only while that key is current.
func (m Model) prepareLayout() tea.Cmd {
	key := m.curKey()
	src := m.sources[key]
	if src == nil {
		return nil
	}
	rk := m.rowKey(key, src)
	if cur := m.buffers[key]; cur != nil && cur.Key() == rk {
		return nil
	}
	return layoutCmd(m.ctx, m.layoutGate, rk, src)
}

// resizeLayout reconciles the current file's layout with new terminal
// geometry. When the prepared-layout key would not change — a
// height-only resize — the installed model's anchor maps to the same
// rendered row and only the EOF clamp can move the top, applied here;
// a changed key requests keyed preparation instead, and the previous
// layout stays installed until the result lands.
func (m Model) resizeLayout() tea.Cmd {
	if m.state != stateBrowse {
		return nil
	}
	key := m.curKey()
	src := m.sources[key]
	if src == nil {
		return nil
	}
	rk := m.rowKey(key, src)
	if cur := m.buffers[key]; cur != nil && cur.Key() == rk {
		m.viewportFor(key).SetLayout(cur, m.contentHeight())
		return nil
	}
	return layoutCmd(m.ctx, m.layoutGate, rk, src)
}

// rowKey is the row-model key for path's source under the current
// terminal geometry and wrap mode.
func (m Model) rowKey(path string, src viewport.Source) viewport.RowModelKey {
	return viewport.RowModelKey{
		Path:      path,
		Revision:  m.revs[path],
		TextWidth: viewport.TextWidth(m.panelWidth(), src.GutterWidth(), m.wrap),
		Wrap:      m.wrap,
	}
}

// panelWidth is the file panel's width in cells: the terminal width
// minus the file-list column.
func (m Model) panelWidth() int {
	w, _ := m.termSize()
	return w - m.listWidth(w)
}

// panCurrent applies a horizontal pan key to the current file's saved
// viewport: one column for ,/. — ten columns for </> — and half the
// text width for [/]. Panning is a no-op in wrap mode, and on a
// "Loading…" or "(unreadable)" placeholder there is nothing to pan.
func (m Model) panCurrent(key string) {
	if m.wrap {
		return
	}
	path := m.curKey()
	if m.buffers[path] == nil {
		return
	}
	vp := m.viewportFor(path)
	switch key {
	case ",":
		vp.Pan(-1)
	case ".":
		vp.Pan(1)
	case "<":
		vp.Pan(-10)
	case ">":
		vp.Pan(10)
	case "[":
		vp.Pan(-vp.HalfPan())
	case "]":
		vp.Pan(vp.HalfPan())
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
	if m.tooSmall() {
		return m.tooSmallScreen()
	}
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
		return m.overlayScreen(base, m.overlay)
	}
	if m.help != nil {
		return m.overlayScreen(base, m.help)
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
	pad := (w - safepresentation.CellWidth(msg)) / 2
	if pad < 0 {
		pad = 0
	}
	rows := make([]string, h)
	rows[h/2] = strings.Repeat(" ", pad) + msg
	return m.theme.Base(strings.Join(rows, "\n"))
}

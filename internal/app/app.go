// Package app owns the Bubble Tea model, lifecycle, asynchronous work,
// overlays, cleanup, and rendering for vrg.
package app

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
	"vrg/internal/theme"
	"vrg/internal/viewport"
)

// State identifies the current app state.
type State int

const (
	// StateSearching is the initial state while rg is running and results
	// are being collected and indexed.
	StateSearching State = iota
	// StateSummary is the interim summary state shown after collection
	// completes without a browse index (backward-compatible path or no
	// results).
	StateSummary
	// StateBrowse is the two-pane browse state shown after a completed
	// search with results: a file list on the left and a matched-file
	// content panel on the right.
	StateBrowse
	// StateNoResults is the centred no-results screen shown after a
	// complete successful search (rg exit 0 or 1) with no usable
	// results. The optional "(N binary files skipped)" suffix is
	// appended when every matched file was excluded.
	StateNoResults
	// StateStartFailed is the state when rg could not be started. The
	// entry point should print the diagnostic to stderr and exit 2.
	StateStartFailed
	// StateFailed is the state for a controlled application failure
	// after the child has started. The entry point prints the
	// diagnostic to stderr and exits 2.
	StateFailed
	// StateCancelled is the state after cancellation (q while searching
	// or ctrl+c in any state). Late search completions are ignored.
	StateCancelled
)

// OverlayKind classifies the modal overlay presentation.
type OverlayKind int

const (
	// OverlayNone means no overlay is shown.
	OverlayNone OverlayKind = iota
	// OverlayError is the fatal error overlay (exit 2 outcome).
	OverlayError
	// OverlayWarning is the non-fatal warning overlay (stderr on rg 0/1).
	OverlayWarning
)

// ProcessResult captures the ripgrep process exit outcome, kept separate
// from stream integrity so the App can assess them independently.
type ProcessResult struct {
	// ExitCode is the process exit code. SignalDeath is true when the
	// process was killed by a signal; in that case ExitCode is the
	// signal number.
	ExitCode int
	// SignalDeath is true when the process died from a signal.
	SignalDeath bool
}

// RecordLoss holds record-loss counts for the outcome decision.
// Malformed is the count of skipped malformed records. Unknown is the
// count of skipped unknown record types. Oversized is the count of
// records that exceeded the 64 MiB payload limit. When malformed or
// oversized records leave zero usable results, the outcome is a
// record-loss fatal overlay (exit 2). Unknown-only loss never
// independently changes exit status.
type RecordLoss struct {
	// Malformed is the count of skipped malformed records.
	Malformed int
	// Unknown is the count of skipped unknown record types.
	Unknown int
	// Oversized is the count of records that exceeded the 64 MiB
	// payload limit and were discarded.
	Oversized int
}

// OutcomeInput is the input to the pure outcome decision.
type OutcomeInput struct {
	Process       ProcessResult
	Integrity     searchindex.Integrity
	UsableResults int
	RecordLoss    RecordLoss
	// Diagnostics is the captured stderr text used for warning
	// classification and overlay text.
	Diagnostics string
	// RecordLossDiagnostics is the formatted diagnostic text for
	// record-loss counts (malformed, unknown, oversized). It is
	// combined with Diagnostics for the overlay text.
	RecordLossDiagnostics string
}

// Outcome is the result of the pure outcome decision: the initial
// presentation (state + overlay), whether the overlay is fatal (its
// dismissal exits 2), the overlay text, and the fixed exit status.
type Outcome struct {
	// State is the initial presentation state (browse or no-results).
	State State
	// Overlay is the overlay kind to show initially.
	Overlay OverlayKind
	// OverlayFatal is true when the overlay is fatal with no underlying
	// state: dismissal (q or Esc) exits 2.
	OverlayFatal bool
	// OverlayText is the text to render in the overlay (already
	// determined, not yet sanitized).
	OverlayText string
	// ExitStatus is the fixed exit status decided once.
	ExitStatus int
}

// DecideOutcome computes the initial presentation, post-dismissal
// state, and fixed exit status from the search outcome inputs. It is
// pure: it has no side effects and depends only on its inputs. The
// stream is fatal when the process exits with a code other than 0 or 1,
// dies by signal, or the stream integrity fails (incomplete). When
// malformed or oversized records leave zero usable results, the
// outcome is a record-loss fatal overlay (exit 2). Unknown-only loss
// never independently changes exit status; it produces a warning
// overlay. Record-loss diagnostics are combined with stderr
// diagnostics for the overlay text.
func DecideOutcome(in OutcomeInput) Outcome {
	fatal := in.Process.SignalDeath || isFatalExit(in.Process.ExitCode) || !in.Integrity.Complete
	hasResults := in.UsableResults > 0

	// Combine stderr and record-loss diagnostics for overlay text.
	overlayText := in.Diagnostics
	if in.RecordLossDiagnostics != "" {
		if overlayText != "" {
			overlayText += "\n" + in.RecordLossDiagnostics
		} else {
			overlayText = in.RecordLossDiagnostics
		}
	}
	hasOverlayText := overlayText != ""

	if fatal {
		text := in.Diagnostics
		if text == "" {
			text = generatedDiagnostic(in.Process)
		}
		if hasResults {
			// Browse with error overlay; dismiss → browse; q → 2.
			return Outcome{State: StateBrowse, Overlay: OverlayError, OverlayText: text, ExitStatus: 2}
		}
		// Fatal no-results overlay; q/Esc → 2 (no underlying state).
		return Outcome{State: StateNoResults, Overlay: OverlayError, OverlayFatal: true, OverlayText: text, ExitStatus: 2}
	}

	// Record-loss fatal: no usable results due to malformed or
	// oversized records. This is distinct from ordinary no-results
	// because records were received but all were skipped.
	recordLoss := !hasResults && (in.RecordLoss.Malformed > 0 || in.RecordLoss.Oversized > 0)
	if recordLoss {
		return Outcome{State: StateNoResults, Overlay: OverlayError, OverlayFatal: true, OverlayText: overlayText, ExitStatus: 2}
	}

	// Not fatal, no record loss: exit 0 or 1, complete stream.
	if hasResults {
		if hasOverlayText {
			// Browse with warning overlay; dismiss → browse; q → 0.
			return Outcome{State: StateBrowse, Overlay: OverlayWarning, OverlayText: overlayText, ExitStatus: 0}
		}
		// Browse; q → 0.
		return Outcome{State: StateBrowse, Overlay: OverlayNone, ExitStatus: 0}
	}

	// No usable results.
	if hasOverlayText {
		// Warning overlay over no-results; dismiss → no-results; q → 1.
		return Outcome{State: StateNoResults, Overlay: OverlayWarning, OverlayText: overlayText, ExitStatus: 1}
	}
	// No-results; q → 1.
	return Outcome{State: StateNoResults, Overlay: OverlayNone, ExitStatus: 1}
}

// isFatalExit reports whether an exit code is a fatal ripgrep exit
// (anything other than 0 or 1).
func isFatalExit(code int) bool {
	return code != 0 && code != 1
}

// recordLossDiagnostics builds the formatted diagnostic text for
// record-loss counts from the index. It includes malformed count,
// unknown count, and per-record oversized path diagnostics.
func recordLossDiagnostics(idx *searchindex.Index) string {
	var parts []string
	if m := idx.MalformedCount(); m > 0 {
		plural := ""
		if m != 1 {
			plural = "s"
		}
		parts = append(parts, fmt.Sprintf("%d malformed record%s skipped", m, plural))
	}
	if u := idx.UnknownCount(); u > 0 {
		parts = append(parts, fmt.Sprintf("%d unrecognised record types skipped", u))
	}
	parts = append(parts, idx.OversizedDiagnostics()...)
	return strings.Join(parts, "\n")
}

// generatedDiagnostic produces a diagnostic naming the exit code or
// signal when a failed process supplies no stderr.
func generatedDiagnostic(p ProcessResult) string {
	if p.SignalDeath {
		return fmt.Sprintf("ripgrep killed by signal %d", p.ExitCode)
	}
	return fmt.Sprintf("ripgrep exited with code %d", p.ExitCode)
}

// Process holds a running rg subprocess and its stdout/stderr pipes,
// plus lifecycle channels for cancellation and completion.
type Process struct {
	Cmd    *exec.Cmd
	Stdout io.Reader
	Stderr io.Reader

	// OnReap, if set, is called with the child's wait error after
	// Wait completes. It is a test seam for proving the reap path ran
	// rather than inferring it from a missing PID.
	OnReap func(err error)

	// done is closed when the collection goroutine finishes, so the
	// process boundary can wait for full cleanup.
	done chan struct{}

	// cancel is closed to signal the collection goroutine to stop
	// waiting at the gate and finish promptly.
	cancel chan struct{}
}

// NewProcess creates a Process with lifecycle channels initialized.
// The process boundary calls this, injects the result into the model,
// and later calls Cleanup to terminate and reap the child.
func NewProcess(cmd *exec.Cmd, stdout, stderr io.Reader) *Process {
	return &Process{
		Cmd:    cmd,
		Stdout: stdout,
		Stderr: stderr,
		done:   make(chan struct{}),
		cancel: make(chan struct{}),
	}
}

// Cancel signals the collection goroutine to stop waiting at the gate.
// It is safe to call multiple times.
func (p *Process) Cancel() {
	select {
	case <-p.cancel:
		// Already closed.
	default:
		close(p.cancel)
	}
}

// Cleanup terminates the child if still running and waits for the
// collection goroutine to finish, ensuring the child is reaped. It is
// the centralized cleanup boundary called by the process entry point
// on every exit. If the collection goroutine already called Wait,
// the child is already reaped; Cleanup ensures the goroutine has
// finished. A bounded wait prevents deadlock if the goroutine never
// started (e.g., terminal initialization failure).
func (p *Process) Cleanup() {
	if p.Cmd != nil && p.Cmd.Process != nil {
		// Kill the child if still running. Ignore error if already dead.
		_ = p.Cmd.Process.Kill()
	}
	if p.done != nil {
		select {
		case <-p.done:
		case <-time.After(5 * time.Second):
			// Safety net: if the goroutine never started or is stuck,
			// don't block the process exit forever.
		}
	}
}

// FileLoader loads a file's bytes into a prepared buffer. The path is
// the raw path bytes from the search index; the stops are the matched
// lines for this file. The returned buffer is fully decoded and mapped
// so the caller's Update does no full-file work.
type FileLoader func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error)

// Model is the Bubble Tea model for the vrg app.
type Model struct {
	state         State
	width         int
	height        int
	files         int
	lines         int
	excludedFiles int
	diagnostic    string
	exitCode      int
	cancelled     bool

	// diagnostics is the session diagnostic collection, independent of
	// what was displayed. Each entry is sanitized through
	// sanitizeDiagnostic. The entry point replays these to stderr after
	// terminal restoration, exactly once each, in collection order, on
	// every controlled exit (Issue #11).
	diagnostics []string
	// onCollect, if set, is called when a diagnostic is collected into
	// the session collection. It is a test seam for the application-side
	// acknowledgement side channel (Issue #11), in the same mechanism
	// family as Process.OnReap.
	onCollect func(string)

	childArgs []string
	workdir   string
	process   *Process
	gate      chan struct{}
	failSig   <-chan string
	// diagSig, if set, is a channel whose receipt emits a DiagnosticMsg
	// for collection without display (Issue #11). It is a test seam in
	// the same mechanism family as failSig.
	diagSig <-chan string

	// Browse state.
	index      *searchindex.Index
	cursor     *searchindex.Cursor
	buffer     *filebuffer.Buffer
	loading    bool
	theme      theme.Theme
	fileLoader FileLoader
	fileGate   chan struct{}
	loadCancel chan struct{}
	// fileCache retains loaded buffers for the session keyed by raw
	// path, so a revisited file can be shown immediately without a
	// reload (Issue #13). No eviction; the PRD retains successful
	// buffers for the session.
	fileCache map[string]*filebuffer.Buffer

	// viewport is the scrollable content view for the current file.
	// It holds prepared row data and the vertical offset. When nil
	// (loading or no buffer), the render path shows the placeholder.
	viewport *viewport.Viewport
	// rowModel is the prepared, swappable row model for the current
	// file at the current text width and wrap mode (Issue #16). It is
	// rebuilt when a load completes, the wrap mode is toggled, or the
	// layout changes. Keyed by (path, content revision, text width,
	// wrap mode) so Issue #17 can move preparation off the UI update
	// path without restructuring it.
	rowModel *viewport.RowModel
	// wrapMode is the current wrap mode (Issue #16). Wrapping is on by
	// default; 'w' toggles between wrap and run-off-edge modes.
	wrapMode viewport.WrapMode
	// currentPath is the raw path of the currently loaded file, used
	// as the key for per-file viewport state.
	currentPath []byte
	// perFileOffset saves the vertical viewport offset per raw path so
	// a file revisited later can start from its saved position (Issue
	// #12). The key is the string form of the raw path bytes.
	perFileOffset map[string]int
	// needsReveal is true when the next FileLoadCompleteMsg for the
	// current file should apply a destination reveal (Issue #14). It is
	// set for the startup load and for cross-file navigation to an
	// uncached destination; it is cleared once the reveal is applied.
	// Reload (r) does not set it, so a reload preserves the saved
	// viewport anchor without revealing a match (PRD: "Reload by
	// itself does not reveal a match").
	needsReveal bool
	// rowProviderFactory builds a RowProvider from a loaded buffer.
	// When nil, viewport.BufferRows is used. This is a test seam for
	// the render-cost guard: a counting fake proves the render path
	// queries only the visible row range.
	rowProviderFactory RowProviderFactory

	// Overlay state. The modal error/warning overlay sits over the
	// browse or no-results state. When OverlayFatal is true, dismissal
	// (q or Esc) exits 2 because there is no underlying state.
	overlay       OverlayKind
	overlayOpen   bool
	overlayText   string
	overlayFatal  bool
	overlayScroll int

	// File-change pop-up state (Issue #15). The pop-up starts at
	// selection time (when navigation crosses a file boundary), not
	// at load completion. Each pop-up gets a fresh one-second instance;
	// an expiry message dismisses only the matching instance, so a
	// stale timer cannot dismiss a newer pop-up. Any key press dismisses
	// the pop-up and performs its normal action in the same update.
	// Centring and left-truncation are computed from the current
	// terminal size at every render, so a resize recentres and
	// re-truncates without dismissing or restarting the timer. An
	// error overlay cancels the pop-up with no return after dismissal.
	popupOpen     bool
	popupPath     []byte
	popupInstance uint64
	popupDuration time.Duration
}

type config struct {
	gate       chan struct{}
	process    *Process
	failSignal <-chan string
	diagSignal <-chan string
	theme      theme.Theme
	fileLoader FileLoader
	fileGate   chan struct{}
	onCollect  func(string)
	// rowProviderFactory builds a RowProvider from a loaded buffer.
	// When nil, viewport.BufferRows is used. This is a test seam for
	// the render-cost guard.
	rowProviderFactory RowProviderFactory
	// popupDuration is the file-change pop-up lifetime. Zero means
	// use the default (1 second); tests may set a very short duration
	// so the timer fires immediately when executed.
	popupDuration time.Duration
}

// RowProviderFactory builds a viewport.RowProvider from a loaded
// buffer. The default factory (viewport.BufferRows) slices the
// buffer's prepared lines; a test factory can substitute a counting
// fake to prove the render path queries only the visible row range.
type RowProviderFactory func(buf *filebuffer.Buffer) viewport.RowProvider

// Option configures the model.
type Option func(*config)

// WithGate sets a channel that holds index preparation until the channel
// is closed or receives a value. This is a test seam for verifying the
// app stays in the searching state after rg has exited but before the
// index is ready.
func WithGate(ch chan struct{}) Option {
	return func(c *config) { c.gate = ch }
}

// WithProcess injects a running rg process for the model to drain. The
// entry point starts rg and passes the process to the model so that
// start failure is handled before the TUI is entered.
func WithProcess(p *Process) Option {
	return func(c *config) { c.process = p }
}

// WithFailureSignal sets a channel whose receipt triggers a controlled
// application failure. When the channel yields a string, the model
// transitions to the failed state with that string as the diagnostic.
// This is a test seam for injecting failures after the child has started.
func WithFailureSignal(ch <-chan string) Option {
	return func(c *config) { c.failSignal = ch }
}

// WithDiagnosticSignal sets a channel whose receipt emits a
// DiagnosticMsg for collection into the session diagnostic collection
// without display (Issue #11). This is a test seam in the same
// mechanism family as WithFailureSignal: the process boundary wires it
// via an environment-variable-gated side channel so PTY tests can emit
// a diagnostic while the fake rg or preparation gate is still blocked,
// then wait for the collection acknowledgement before sending the exit
// key.
func WithDiagnosticSignal(ch <-chan string) Option {
	return func(c *config) { c.diagSignal = ch }
}

// WithTheme sets the visual theme for the browse view. The no-style
// theme disables all ANSI sequences for sink-safety testing.
func WithTheme(t theme.Theme) Option {
	return func(c *config) { c.theme = t }
}

// WithFileLoader injects a file-loading function for the browse view.
// This is a test seam for verifying responsiveness while a load is held
// by a gate.
func WithFileLoader(loader FileLoader) Option {
	return func(c *config) { c.fileLoader = loader }
}

// WithFileLoadGate sets a channel that holds file loading until the
// channel is closed or receives a value. This is a test seam for
// verifying the browse view stays responsive while a load is pending.
func WithFileLoadGate(ch chan struct{}) Option {
	return func(c *config) { c.fileGate = ch }
}

// WithOnCollect sets a callback called once a diagnostic has been
// processed into the session collection. It is a test seam for the
// application-side acknowledgement side channel (Issue #11), in the
// same mechanism family as Process.OnReap: the process boundary sets
// it via an environment-variable-gated side channel so PTY tests can
// wait for the acknowledgement before sending the exit key. The
// callback receives the sanitized diagnostic string.
func WithOnCollect(f func(string)) Option {
	return func(c *config) { c.onCollect = f }
}

// WithRowProviderFactory sets a factory that builds a RowProvider from
// a loaded buffer. When nil, viewport.BufferRows is used. This is a
// test seam for the render-cost guard (Issue #12): a counting fake
// proves the render path queries only the visible row range, not the
// full buffer.
func WithRowProviderFactory(f RowProviderFactory) Option {
	return func(c *config) { c.rowProviderFactory = f }
}

// WithPopupDuration sets the file-change pop-up lifetime (Issue #15).
// The production default is 1 second. Tests may set a very short
// duration so the timer fires immediately when executed, avoiding
// sleeps in test code.
func WithPopupDuration(d time.Duration) Option {
	return func(c *config) { c.popupDuration = d }
}

// New creates a new app model for a search invocation. The model starts
// in the searching state.
func New(childArgs []string, workdir string, opts ...Option) Model {
	cfg := config{
		popupDuration: time.Second,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return Model{
		state:              StateSearching,
		childArgs:          childArgs,
		workdir:            workdir,
		process:            cfg.process,
		gate:               cfg.gate,
		failSig:            cfg.failSignal,
		diagSig:            cfg.diagSignal,
		theme:              cfg.theme,
		fileLoader:         cfg.fileLoader,
		fileGate:           cfg.fileGate,
		onCollect:          cfg.onCollect,
		rowProviderFactory: cfg.rowProviderFactory,
		perFileOffset:      make(map[string]int),
		fileCache:          make(map[string]*filebuffer.Buffer),
		loadCancel:         make(chan struct{}),
		popupDuration:      cfg.popupDuration,
	}
}

// State returns the current app state.
func (m Model) State() State { return m.state }

// ExitCode returns the process exit code the entry point should use.
func (m Model) ExitCode() int { return m.exitCode }

// Diagnostic returns the sanitized diagnostic for stderr replay. Valid
// after the model has received a SearchFailedMsg or ControlledFailureMsg.
func (m Model) Diagnostic() string { return m.diagnostic }

// Diagnostics returns a copy of the session diagnostic collection in
// collection order (Issue #11). Each entry is sanitized through
// sanitizeDiagnostic. The entry point replays these to stderr after
// terminal restoration, exactly once each, on every controlled exit.
// The collection is independent of what was displayed: diagnostics
// never shown in an overlay are also collected.
func (m Model) Diagnostics() []string {
	out := make([]string, len(m.diagnostics))
	copy(out, m.diagnostics)
	return out
}

// OverlayOpen reports whether the modal overlay is currently open.
func (m Model) OverlayOpen() bool { return m.overlayOpen }

// OverlayKind returns the kind of the currently open overlay
// (OverlayNone when no overlay is open).
func (m Model) OverlayKind() OverlayKind { return m.overlay }

// ViewportOffset returns the current vertical viewport offset (the
// 0-based top row) for the loaded file. Returns 0 when no viewport is
// active (loading or no buffer).
func (m Model) ViewportOffset() int {
	if m.viewport == nil {
		return 0
	}
	return m.viewport.Offset()
}

// ViewportRowCount returns the total number of rendered rows in the
// current viewport's row provider, or 0 when no viewport is active.
func (m Model) ViewportRowCount() int {
	if m.viewport == nil {
		return 0
	}
	return m.viewport.RowCount()
}

// SavedOffset returns the saved per-file vertical viewport offset for
// the given raw path, or 0 if no state is saved (Issue #12). This is
// the per-file state saved for later revisits; a first visit returns 0.
func (m Model) SavedOffset(path []byte) int {
	if m.perFileOffset == nil {
		return 0
	}
	return m.perFileOffset[string(path)]
}

// CursorPosition returns the 0-based position of the matched-line
// cursor in the stop list, or -1 when the index is empty (Issue #13).
// The cursor is the single global navigation anchor; current file and
// current matched line derive from it.
func (m Model) CursorPosition() int {
	if m.cursor == nil {
		return -1
	}
	return m.cursor.Position()
}

// CurrentPath returns the raw path of the cursor's current stop, or
// nil when the index is empty (Issue #13). The current file derives
// from the cursor.
func (m Model) CurrentPath() []byte {
	if m.cursor == nil {
		return nil
	}
	s, ok := m.cursor.Stop()
	if !ok {
		return nil
	}
	return s.RawPath
}

// PopupOpen reports whether the file-change pop-up is currently shown
// (Issue #15). The pop-up starts at selection time when navigation
// crosses a file boundary and is dismissed by its one-second timer
// expiry or any key press.
func (m Model) PopupOpen() bool { return m.popupOpen }

// PopupPath returns the raw path displayed in the file-change pop-up,
// or nil when no pop-up is shown (Issue #15). The path is sanitized
// through safepresentation.EscapePath at render time.
func (m Model) PopupPath() []byte { return m.popupPath }

// PopupInstance returns the instance ID of the current file-change
// pop-up, or 0 when no pop-up is shown (Issue #15). Each pop-up gets
// a fresh instance; an expiry message dismisses only the matching
// instance, so a stale timer cannot dismiss a newer pop-up.
func (m Model) PopupInstance() uint64 { return m.popupInstance }

// EscapePathForDiagnostic escapes a raw filename for safe embedding in a
// diagnostic. It delegates to safepresentation.EscapePath, the shared
// single-line path escaper, so filename newlines become literal \n
// sequences and cannot become diagnostic paragraph breaks. The caller
// then embeds the result in a diagnostic string before
// EscapeDiagnostic processes the whole diagnostic.
func EscapePathForDiagnostic(raw string) string {
	return safepresentation.EscapePath([]byte(raw)).Text
}

// Init returns the initial command. If a process was injected, the
// command drains both pipes, collects results, and prepares the index.
// If a failure signal was injected, a watcher command is also returned.
// Without a process (test mode), Init returns nil.
func (m Model) Init() tea.Cmd {
	if m.process == nil {
		return nil
	}
	cmds := []tea.Cmd{m.collectResults()}
	if m.failSig != nil {
		cmds = append(cmds, m.watchFailure())
	}
	if m.diagSig != nil {
		cmds = append(cmds, m.watchDiagnostic())
	}
	return tea.Batch(cmds...)
}

// Update handles messages and returns the updated model and command.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SearchCompleteMsg:
		// Late search completions after cancellation must not revive
		// the UI.
		if m.cancelled {
			return m, nil
		}
		if msg.Index != nil {
			// Apply the Issue #9 outcome matrix through the pure
			// DecideOutcome function. The fixed exit status is
			// decided once here and never recomputed except by
			// ctrl+c (which overrides to 130).
			oc := DecideOutcome(OutcomeInput{
				Process:               msg.Process,
				Integrity:             msg.Index.Integrity(),
				UsableResults:         msg.Index.Len(),
				Diagnostics:           msg.Stderr,
				RecordLoss:            RecordLoss{Malformed: msg.Index.MalformedCount(), Unknown: msg.Index.UnknownCount(), Oversized: msg.Index.OversizedCount()},
				RecordLossDiagnostics: recordLossDiagnostics(msg.Index),
			})
			m.state = oc.State
			m.exitCode = oc.ExitStatus
			m.files = msg.Files
			m.lines = msg.Lines
			m.index = msg.Index
			m.excludedFiles = msg.Index.ExcludedFiles()
			m.overlay = oc.Overlay
			m.overlayOpen = oc.Overlay != OverlayNone
			m.overlayText = sanitizeDiagnostic(oc.OverlayText)
			m.overlayFatal = oc.OverlayFatal
			m.overlayScroll = 0
			// Issue #15: an error overlay cancels the file-change
			// pop-up. After cancellation the pop-up does not return
			// when the overlay is dismissed.
			if m.overlayOpen {
				m.cancelPopup()
			}
			// Collect the overlay diagnostic into the session collection
			// (Issue #11). The collection is independent of display:
			// the overlay text is collected here regardless of whether
			// the overlay is later dismissed or the user quits while it
			// is open. The shutdown boundary is the message-processing
			// point: the diagnostic is collected once the model has
			// processed this SearchCompleteMsg.
			if oc.OverlayText != "" {
				m.collectDiagnostic(oc.OverlayText)
			}
			if oc.State == StateBrowse {
				// Issue #13: create the circular matched-line
				// cursor. Startup selects the first stop;
				// current file and current matched line derive
				// from the cursor.
				m.cursor = searchindex.NewCursor(m.index)
				m.loading = true
				// Issue #14: the startup file's first load should
				// apply a destination reveal once the content is
				// available.
				m.needsReveal = true
				return m, m.loadFile()
			}
			return m, nil
		}
		// Backward-compatible summary path (Index nil).
		m.state = StateSummary
		m.files = msg.Files
		m.lines = msg.Lines
		return m, nil

	case FileLoadCompleteMsg:
		// Late file-load completions after cancellation must not revive
		// the UI.
		if m.cancelled {
			return m, nil
		}
		if msg.Buffer != nil {
			m.buffer = msg.Buffer
			// Issue #13: cache the loaded buffer for the session
			// so a revisited file can be shown immediately without
			// a reload. No eviction; the PRD retains successful
			// buffers for the session.
			if m.fileCache == nil {
				m.fileCache = make(map[string]*filebuffer.Buffer)
			}
			m.fileCache[string(msg.Path)] = msg.Buffer
		}
		m.loading = false
		// Build the viewport from the prepared row data (Issue #16:
		// swappable row model). The row provider factory (or the
		// default row model) adapts the buffer so the render path
		// queries only the visible range. The per-file saved offset
		// is restored so a revisited file starts from its saved
		// position (Issue #12).
		if msg.Buffer != nil {
			m.currentPath = msg.Path
			m.buildViewport()
			// Issue #14: apply destination reveal after the starting
			// viewport is set. A first visit (including the startup
			// file) starts from the top; a revisit starts from the
			// saved offset. The reveal then adjusts from there.
			// Reload (r) does not set needsReveal, so a reload
			// preserves the saved viewport anchor without revealing
			// a match (PRD: "Reload by itself does not reveal a
			// match").
			if m.needsReveal {
				m.needsReveal = false
				m.revealTarget()
			}
		}
		return m, nil

	case SearchFailedMsg:
		m.state = StateStartFailed
		m.diagnostic = sanitizeDiagnostic(msg.Diagnostic)
		m.exitCode = 2
		// Collect the start-failure diagnostic into the session
		// collection (Issue #11). The entry point replays it from the
		// collection after terminal restoration.
		m.collectDiagnostic(msg.Diagnostic)
		return m, tea.Quit

	case ControlledFailureMsg:
		m.state = StateFailed
		m.diagnostic = sanitizeDiagnostic(msg.Diagnostic)
		m.exitCode = 2
		m.cancelled = true
		m.cancelProcess()
		m.cancelLoad()
		// Route the controlled-failure diagnostic through the session
		// collection instead of a separate direct write (Issue #11).
		// The entry point replays it from the collection after terminal
		// restoration, so exactly-once holds across both the former
		// direct-write path and the replay mechanism.
		m.collectDiagnostic(msg.Diagnostic)
		return m, tea.Quit

	case DiagnosticMsg:
		// Collect the diagnostic into the session collection without
		// displaying it in an overlay (Issue #11). The entry point
		// replays it to stderr after terminal restoration. This path
		// serves diagnostics that are only recoverable through stderr
		// replay: unknown-type warnings, late non-current load
		// failures (Issue #26), and diagnostics collected just before
		// exit.
		m.collectDiagnostic(msg.Diagnostic)
		return m, nil

	case FileChangePopupExpiryMsg:
		// Issue #15: an expiry message dismisses the pop-up only if
		// its instance matches the currently active pop-up. A stale
		// expiry (from an older instance) must not dismiss a newer
		// pop-up.
		if m.popupOpen && m.popupInstance == msg.Instance {
			m.dismissPopup()
		}
		return m, nil

	case tea.KeyPressMsg:
		// ctrl+c always overrides the fixed exit status to 130,
		// regardless of overlay or base state.
		if msg.Code == 'c' && msg.Mod == tea.ModCtrl {
			return m.cancel()
		}
		// When the overlay is open, it captures key routing.
		if m.overlayOpen {
			return m.handleOverlayKey(msg)
		}
		// Issue #15: any key press dismisses the file-change pop-up
		// and performs its normal action in the same update. The
		// dismissal happens before the normal action so a cross-file
		// navigation can open a fresh pop-up in the same update.
		if m.popupOpen {
			m.dismissPopup()
		}
		// Scroll keys are active in the browse state when content is
		// loaded. Scrolling a "Loading…" placeholder is a no-op
		// (Issue #12). Manual scrolling does not move the matched-line
		// cursor.
		if m.state == StateBrowse && m.viewport != nil {
			if m.handleScrollKey(msg) {
				return m, nil
			}
		}
		// Issue #13: n/p move the circular matched-line cursor. The
		// cursor is the single global navigation anchor; current file
		// and current matched line derive from it. Navigation remains
		// active while loading (the viewport may be nil); same-file
		// navigation only changes current-line styling; cross-file
		// navigation switches the panel and requests a load for an
		// uncached destination. Manual scrolling does not move the
		// cursor, so n/p continue from the last selected stop.
		if m.state == StateBrowse {
			if msg.Code == 'n' && msg.Mod == 0 {
				return m.handleNavigate(1)
			}
			if msg.Code == 'p' && msg.Mod == 0 {
				return m.handleNavigate(-1)
			}
		}
		switch {
		case msg.Code == 'w' && msg.Mod == 0:
			if m.state == StateBrowse && m.buffer != nil {
				m.wrapMode = m.wrapMode.Toggle()
				m.buildViewport()
			}
			return m, nil
		case msg.Code == 'c' && msg.Mod == 0:
			if m.state == StateBrowse {
				m.theme = m.theme.Toggle()
			}
			return m, nil
		case msg.Code == 'q' && msg.Mod == 0:
			switch m.state {
			case StateSearching:
				return m.cancel()
			case StateSummary:
				m.exitCode = 0
				return m, tea.Quit
			case StateNoResults:
				// Fixed exit status was decided at completion; q
				// quits with it.
				m.cancelled = true
				m.cancelProcess()
				m.cancelLoad()
				return m, tea.Quit
			case StateBrowse:
				// Fixed exit status was decided at completion; q
				// quits with it.
				m.cancelled = true
				m.cancelProcess()
				m.cancelLoad()
				return m, tea.Quit
			}
		case msg.Code == tea.KeyEscape:
			// Esc never exits from a base state.
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Recompute the viewport layout from the new dimensions and
		// clamp the offset without losing the reading position (Issue
		// #12). Issue #16: rebuild the row model when the text width
		// changes so wrapping reflects the new panel width.
		if m.buffer != nil && m.rowProviderFactory == nil {
			offset := 0
			if m.viewport != nil {
				offset = m.viewport.Offset()
			}
			m.buildViewport()
			if m.viewport != nil {
				m.viewport.SetOffset(offset)
			}
		} else if m.viewport != nil {
			m.viewport.SetPanelHeight(msg.Height)
		}
		return m, nil
	}
	return m, nil
}

// handleScrollKey routes a scroll key press to the viewport. up/down
// scroll one rendered row; u/d scroll half a page (max(1,
// floor(contentHeight/2))); page up/down scroll a full page. The
// viewport clamps to valid content. After scrolling, the per-file
// offset is saved for later revisits (Issue #12). Returns true if the
// key was handled as a scroll key, false otherwise.
func (m Model) handleScrollKey(msg tea.KeyPressMsg) bool {
	switch {
	case msg.Code == tea.KeyDown && msg.Mod == 0:
		m.viewport.ScrollDown()
	case msg.Code == tea.KeyUp && msg.Mod == 0:
		m.viewport.ScrollUp()
	case msg.Code == 'd' && msg.Mod == 0:
		m.viewport.ScrollHalfDown()
	case msg.Code == 'u' && msg.Mod == 0:
		m.viewport.ScrollHalfUp()
	case msg.Code == tea.KeyPgDown:
		m.viewport.ScrollPageDown()
	case msg.Code == tea.KeyPgUp:
		m.viewport.ScrollPageUp()
	default:
		return false
	}
	m.saveOffset()
	return true
}

// saveOffset records the current viewport offset as per-file state for
// the current file's raw path (Issue #12). This is called after every
// scroll action so a revisited file can start from its saved position.
func (m *Model) saveOffset() {
	if m.viewport == nil || m.currentPath == nil {
		return
	}
	if m.perFileOffset == nil {
		m.perFileOffset = make(map[string]int)
	}
	m.perFileOffset[string(m.currentPath)] = m.viewport.Offset()
}

// revealTarget applies the Issue #14 destination reveal to the current
// cursor target. The display target is the start cell of the first
// submatch on the destination line; the reveal adjusts the viewport so
// the rendered row containing that target is visible. If the target
// is already visible, the viewport does not scroll and the saved
// per-file state is left unchanged. If the reveal moves the viewport,
// the new offset replaces the saved per-file state.
func (m *Model) revealTarget() {
	if m.viewport == nil || m.cursor == nil {
		return
	}
	stop, ok := m.cursor.Stop()
	if !ok {
		return
	}
	targetRow := m.targetRow(stop)
	before := m.viewport.Offset()
	m.viewport.Reveal(targetRow)
	// A reveal that moves the viewport replaces the saved vertical
	// state; a no-scroll reveal does not discard it.
	if m.viewport.Offset() != before {
		m.saveOffset()
	}
}

// buildViewport constructs the viewport's row provider from the
// current buffer, wrap mode, and panel dimensions (Issue #16). When a
// row provider factory is set (test seam), it is used directly;
// otherwise a RowModel is built from the buffer at the current text
// width and wrap mode. The per-file saved offset is restored so a
// revisited file starts from its saved position (Issue #12).
func (m *Model) buildViewport() {
	if m.buffer == nil {
		return
	}
	var rows viewport.RowProvider
	if m.rowProviderFactory != nil {
		rows = m.rowProviderFactory(m.buffer)
	} else {
		gw := m.buffer.GutterWidth
		panelWidth := m.width - fileListWidth(m.width) - 1
		tw := viewport.TextWidth(panelWidth, gw, m.wrapMode)
		key := viewport.RowModelKey{
			Path:      string(m.currentPath),
			Revision:  1,
			TextWidth: tw,
			WrapMode:  m.wrapMode,
		}
		m.rowModel = viewport.BuildRowModel(m.buffer, tw, m.wrapMode, key)
		rows = m.rowModel
	}
	offset := m.SavedOffset(m.currentPath)
	m.viewport = viewport.New(rows, m.height)
	m.viewport.SetOffset(offset)
}

// targetRow returns the 0-based rendered row containing the display
// target for the given stop. The display target is the start cell of
// the first submatch on the destination line (the marker cell for a
// zero-width match). Submatches are ordered by byte start then end by
// the search index, so the first submatch identifies the target.
// Issue #16: when wrapping is on, the rendered row is found via the
// row model's RowFromByte, which maps the source line index and byte
// offset to the wrapped row containing the match start. In run-off-
// edge mode, each source line is one rendered row.
func (m *Model) targetRow(stop searchindex.Stop) int {
	if m.rowModel != nil && len(stop.Submatches) > 0 {
		byteOffset := stop.Submatches[0].Start
		if row := m.rowModel.RowFromByte(stop.LineNumber-1, byteOffset); row >= 0 {
			return row
		}
	}
	return stop.LineNumber - 1
}

// handleNavigate moves the matched-line cursor by delta (1 for n, -1
// for p) and wires the consequences (Issue #13). With zero or one stop
// the cursor is a strict no-op: no state change, no load command, no
// pop-up. With two or more stops the cursor moves circularly. On a
// file change: the departing file's viewport offset is saved, the
// content panel switches immediately, and a load is requested for an
// uncached destination; a cached destination is shown immediately with
// its saved viewport restored (first visit starts at the top). On a
// same-file move: only the current matched line changes; destination
// reveal belongs to Issue #14. Manual scrolling does not move the
// cursor, so n/p continue from the last selected stop.
func (m Model) handleNavigate(delta int) (tea.Model, tea.Cmd) {
	if m.cursor == nil {
		return m, nil
	}
	var stop searchindex.Stop
	var moved, fileChanged bool
	if delta > 0 {
		stop, moved, fileChanged = m.cursor.Next()
	} else {
		stop, moved, fileChanged = m.cursor.Prev()
	}
	if !moved {
		// Zero or one stop: strict no-op. No pop-up, no reload.
		return m, nil
	}
	if !fileChanged {
		// Same-file navigation: only the current matched line
		// styling changes. Issue #14: reveal the new target row.
		m.revealTarget()
		return m, nil
	}
	// Cross-file navigation. Save the departing file's viewport.
	m.saveOffset()
	// Issue #15: start the file-change pop-up at selection time,
	// before either destination branch returns. The pop-up begins
	// regardless of whether the destination is cached or loading.
	popupCmd := m.startPopup(stop.RawPath)
	// Switch the panel to the destination file immediately.
	if buf, ok := m.fileCache[string(stop.RawPath)]; ok {
		// Cached destination: show immediately with its saved
		// viewport restored (first visit starts at the top).
		m.buffer = buf
		m.loading = false
		m.currentPath = stop.RawPath
		m.buildViewport()
		// Issue #14: apply destination reveal after the starting
		// viewport is set from the saved offset (or 0 for a first
		// visit).
		m.revealTarget()
		return m, popupCmd
	}
	// Uncached destination: request a load. The panel shows the
	// loading placeholder until the load completes.
	m.buffer = nil
	m.viewport = nil
	m.loading = true
	m.currentPath = stop.RawPath
	// Issue #14: the load completion for this navigation should
	// apply a destination reveal once the content is available.
	m.needsReveal = true
	loadCmd := m.loadFileFor(stop.RawPath)
	// Batch the pop-up timer and the load command so both run.
	return m, tea.Batch(popupCmd, loadCmd)
}

// handleOverlayKey routes a key press to the open overlay. up/down
// scroll; q and Esc dismiss (or exit 2 for a fatal no-results overlay);
// ctrl+c is handled before this is reached; other keys are ignored.
func (m Model) handleOverlayKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Code == tea.KeyUp:
		if m.overlayScroll > 0 {
			m.overlayScroll--
		}
		return m, nil
	case msg.Code == tea.KeyDown:
		m.overlayScroll++
		return m, nil
	case msg.Code == 'q' && msg.Mod == 0:
		if m.overlayFatal {
			// Fatal no-results overlay: dismissal exits 2.
			m.cancelled = true
			m.cancelProcess()
			m.cancelLoad()
			return m, tea.Quit
		}
		// Non-fatal overlay: dismiss to the base state.
		m.overlayOpen = false
		return m, nil
	case msg.Code == tea.KeyEscape:
		if m.overlayFatal {
			// Fatal no-results overlay: Esc exits 2 (the one case
			// where Esc terminates, because there is no underlying
			// state).
			m.cancelled = true
			m.cancelProcess()
			m.cancelLoad()
			return m, tea.Quit
		}
		// Non-fatal overlay: dismiss to the base state.
		m.overlayOpen = false
		return m, nil
	default:
		// Other keys are ignored by the overlay.
		return m, nil
	}
}

// View renders the current state.
func (m Model) View() tea.View {
	var content string
	switch m.state {
	case StateSearching:
		content = "Searching…"
	case StateSummary:
		content = fmt.Sprintf("%d files, %d matched lines", m.files, m.lines)
	case StateNoResults:
		text := "No results found"
		if m.excludedFiles > 0 {
			text += fmt.Sprintf(" (%d binary files skipped)", m.excludedFiles)
		}
		content = centerText(text, m.width, m.height)
	case StateBrowse:
		content = m.renderBrowse()
	default:
		content = ""
	}
	// Render the modal overlay on top of the base view when open.
	if m.overlayOpen {
		content = m.renderOverlay(content)
	} else if m.popupOpen {
		// Issue #15: render the file-change pop-up on top of the
		// browse content. The pop-up is centered and shows a single-
		// line safe path left-truncated to fit. Centring and
		// truncation are computed from the current terminal size at
		// every render, so a resize recentres and re-truncates
		// without dismissing or restarting the timer. The pop-up
		// is not shown when an error overlay is open (the overlay
		// takes precedence and has already cancelled the pop-up).
		content = m.renderPopup(content)
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// renderOverlay renders the modal overlay on top of the base content.
// The overlay text is wrapped to the interior width (accounting for the
// single-line border and side margins), scrolled by overlayScroll, and
// rendered through the theme's Overlay style (base colours + plain
// single-line border). The base content is rendered first so the
// overlay sits on top.
func (m Model) renderOverlay(base string) string {
	// Determine the overlay width: up to 80% of the terminal width,
	// capped to a reasonable maximum. The interior width accounts for
	// the border sides and the single space margin on each side.
	termWidth := m.width
	if termWidth < 20 {
		termWidth = 80
	}
	overlayWidth := termWidth * 4 / 5
	if overlayWidth < 20 {
		overlayWidth = 20
	}
	if overlayWidth > 100 {
		overlayWidth = 100
	}
	interior := overlayWidth - 4 // two border chars + two spaces
	if interior < 1 {
		interior = 1
	}
	// Wrap the overlay text to the interior width, including unbroken
	// strings.
	wrapped := wrapText(m.overlayText, interior)
	lines := strings.Split(wrapped, "\n")
	// Apply vertical scrolling.
	termHeight := m.height
	if termHeight < 5 {
		termHeight = 24
	}
	// Reserve space for the border (2 lines) and a margin.
	maxVisible := termHeight - 4
	if maxVisible < 1 {
		maxVisible = 1
	}
	// When the diagnostic is very large, show both the head and tail
	// so the user sees the beginning and end of the captured stderr.
	if len(lines) > maxVisible {
		headN := maxVisible / 2
		if headN < 1 {
			headN = 1
		}
		tailN := maxVisible - headN - 1
		if tailN < 1 {
			tailN = 1
		}
		head := lines[:headN]
		tail := lines[len(lines)-tailN:]
		lines = append(append(head, "…"), tail...)
	}
	scroll := m.overlayScroll
	if scroll < 0 {
		scroll = 0
	}
	if scroll > len(lines)-maxVisible {
		scroll = len(lines) - maxVisible
		if scroll < 0 {
			scroll = 0
		}
	}
	end := scroll + maxVisible
	if end > len(lines) {
		end = len(lines)
	}
	visible := strings.Join(lines[scroll:end], "\n")
	// Render the overlay through the theme (base colours + border).
	overlay := m.theme.Overlay(visible)
	// Centre the overlay over the base content. The base content is
	// rendered first; the overlay is placed below it with vertical
	// centring. For simplicity, the overlay replaces the visible area
	// by being rendered on top using newlines to position it.
	return overlay
}

// renderPopup renders the file-change pop-up on top of the base
// content (Issue #15). The pop-up shows a single-line safe path
// (sanitized through safepresentation.EscapePath) left-truncated to
// fit the terminal width, centered horizontally and vertically. The
// centring and truncation are computed from the current terminal size
// at every render, so a resize recentres and re-truncates without
// dismissing or restarting the timer.
func (m Model) renderPopup(base string) string {
	escaped := safepresentation.EscapePath(m.popupPath)
	text := escaped.Text
	// Left-truncate to the terminal width if needed, with a leading ….
	width := m.width
	if width < 1 {
		width = 80
	}
	textWidth := visibleWidth(text)
	if textWidth > width {
		// Keep the trailing portion of the path (the filename end is
		// usually more informative than the directory prefix).
		keep := width - 1 // one cell for the leading …
		if keep < 0 {
			keep = 0
		}
		text = truncateLeftCells(text, keep)
		text = "…" + text
	}
	// Center horizontally and vertically over the base content.
	leftPad := 0
	if width > visibleWidth(text) {
		leftPad = (width - visibleWidth(text)) / 2
	}
	height := m.height
	if height < 1 {
		height = 24
	}
	topPad := 0
	if height > 1 {
		topPad = (height - 1) / 2
	}
	baseLines := strings.Split(base, "\n")
	// Pad the base content to the full terminal height so the pop-up
	// can be placed at the vertical centre even when the base content
	// is shorter than the terminal.
	for len(baseLines) < height {
		baseLines = append(baseLines, "")
	}
	var b strings.Builder
	for i, line := range baseLines {
		if i > 0 {
			b.WriteString("\n")
		}
		if i == topPad {
			b.WriteString(strings.Repeat(" ", leftPad))
			b.WriteString(m.theme.Base(text))
		} else {
			b.WriteString(line)
		}
	}
	return b.String()
}

// truncateLeftCells returns the trailing keep cells of s, dropping
// leading cells. ANSI escape sequences are preserved and do not count
// toward the cell budget. Grapheme boundaries are not split: this
// truncates at rune boundaries, which is sufficient for the single-line
// safe path output of safepresentation.EscapePath (no combining marks
// are produced for path text).
func truncateLeftCells(s string, keep int) string {
	if keep <= 0 {
		return ""
	}
	// First strip ANSI sequences into a separate buffer while recording
	// the visible runes, then take the trailing keep runes.
	var runes []rune
	for i := 0; i < len(s); {
		if s[i] == '\x1b' {
			i++
			for i < len(s) && s[i] != 'm' {
				i++
			}
			if i < len(s) {
				i++
			}
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		runes = append(runes, r)
		i += size
	}
	if len(runes) <= keep {
		return s
	}
	return string(runes[len(runes)-keep:])
}

// wrapText wraps s to the given cell width, breaking long unbroken
// strings. Existing newlines are preserved as line boundaries.
func wrapText(s string, width int) string {
	if width <= 0 {
		return s
	}
	var b strings.Builder
	for i, line := range strings.Split(s, "\n") {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(wrapLine(line, width))
	}
	return b.String()
}

// wrapLine wraps a single line (no embedded newlines) to the given cell
// width, breaking long unbroken strings.
func wrapLine(line string, width int) string {
	if width <= 0 || visibleWidth(line) <= width {
		return line
	}
	var b strings.Builder
	col := 0
	for i := 0; i < len(line); {
		if line[i] == '\x1b' {
			// Copy ANSI escape sequences without counting cells.
			b.WriteByte(line[i])
			i++
			for i < len(line) && line[i] != 'm' {
				b.WriteByte(line[i])
				i++
			}
			if i < len(line) {
				b.WriteByte(line[i])
				i++
			}
			continue
		}
		_, size := utf8.DecodeRuneInString(line[i:])
		if col >= width {
			b.WriteString("\n")
			col = 0
		}
		b.WriteString(line[i : i+size])
		col++
		i += size
	}
	return b.String()
}

// centerText pads text with leading newlines and spaces to centre it
// vertically and horizontally within the given dimensions. When width
// or height is zero (no WindowSizeMsg received yet), the text is
// returned without padding.
func centerText(text string, width, height int) string {
	textWidth := visibleWidth(text)
	leftPad := 0
	if width > textWidth {
		leftPad = (width - textWidth) / 2
	}
	topPad := 0
	if height > 1 {
		topPad = (height - 1) / 2
	}
	var b strings.Builder
	for i := 0; i < topPad; i++ {
		b.WriteString("\n")
	}
	b.WriteString(strings.Repeat(" ", leftPad))
	b.WriteString(text)
	return b.String()
}

// SearchCompleteMsg signals that collection and index preparation are
// done. Index is nil for the backward-compatible summary path; non-nil
// with results for the browse path. Process carries the ripgrep exit
// result; Stderr carries the captured stderr text used for warning
// classification and overlay diagnostics.
type SearchCompleteMsg struct {
	Files int
	Lines int
	Index *searchindex.Index
	// Process is the ripgrep process exit result. The zero value
	// (ExitCode 0, no signal) is backward-compatible with pre-Issue #9
	// callers.
	Process ProcessResult
	// Stderr is the captured stderr text.
	Stderr string
}

// FileLoadCompleteMsg signals that an asynchronous file load has
// finished. The Buffer is the fully prepared, decoded, and mapped
// buffer; Update does no full-file work.
type FileLoadCompleteMsg struct {
	Path   []byte
	Buffer *filebuffer.Buffer
	Err    error
}

// SearchFailedMsg signals that rg could not be started.
type SearchFailedMsg struct {
	Diagnostic string
}

// ControlledFailureMsg signals a controlled application failure after
// the child has started. The entry point terminates and reaps the
// child, restores the terminal, and writes the sanitized diagnostic
// to stderr exactly once after restoration, exiting 2.
type ControlledFailureMsg struct {
	Diagnostic string
}

// DiagnosticMsg carries a diagnostic to be collected into the session
// diagnostic collection without being displayed in an overlay (Issue
// #11). This is used for diagnostics that are only recoverable through
// stderr replay: unknown-type warnings, late non-current load failures
// (Issue #26), and diagnostics collected just before exit. The
// diagnostic is sanitized and appended to the collection; the entry
// point replays it to stderr after terminal restoration.
type DiagnosticMsg struct {
	Diagnostic string
}

// FileChangePopupExpiryMsg is the instance-keyed expiry message for
// the file-change pop-up (Issue #15). The Instance identifies which
// pop-up instance this expiry belongs to; an expiry from a stale
// instance must not dismiss a newer pop-up. The pop-up starts at
// selection time with a fresh one-second instance per pop-up.
type FileChangePopupExpiryMsg struct {
	Instance uint64
}

// popupInstanceCounter is the process-wide source of fresh pop-up
// instance IDs. Each cross-file navigation increments it and uses the
// new value as the pop-up instance, so a stale expiry message (carrying
// an older instance) cannot dismiss a newer pop-up.
var popupInstanceCounter uint64

// startPopup opens the file-change pop-up for the given raw path with a
// fresh instance ID and schedules the one-second expiry command (Issue
// #15). The pop-up starts at selection time, not at load completion.
// The duration comes from the model's configured popupDuration
// (production default 1 second; tests may set 0 for an instant timer
// so the expiry message is returned immediately without blocking).
func (m *Model) startPopup(path []byte) tea.Cmd {
	m.popupOpen = true
	m.popupPath = path
	popupInstanceCounter++
	m.popupInstance = popupInstanceCounter
	instance := m.popupInstance
	dur := m.popupDuration
	if dur == 0 {
		// Instant timer: return the expiry message immediately
		// without blocking. Used by tests that inject expiry
		// messages directly rather than waiting for the timer.
		return func() tea.Msg {
			return FileChangePopupExpiryMsg{Instance: instance}
		}
	}
	return tea.Tick(dur, func(time.Time) tea.Msg {
		return FileChangePopupExpiryMsg{Instance: instance}
	})
}

// dismissPopup closes the file-change pop-up without affecting any
// other state (Issue #15).
func (m *Model) dismissPopup() {
	m.popupOpen = false
	m.popupPath = nil
	m.popupInstance = 0
}

// cancelPopup closes the file-change pop-up and clears the instance so
// a stale expiry cannot revive it. Used by the error-overlay
// cancellation path: after cancellation the pop-up does not return.
func (m *Model) cancelPopup() {
	m.popupOpen = false
	m.popupPath = nil
	m.popupInstance = 0
}

// cancel transitions the model to the cancelled state, signals the
// collection goroutine to stop, and returns a quit command. The exit
// code is 130.
func (m Model) cancel() (tea.Model, tea.Cmd) {
	m.state = StateCancelled
	m.cancelled = true
	m.exitCode = 130
	m.cancelProcess()
	m.cancelLoad()
	return m, tea.Quit
}

// cancelProcess signals the collection goroutine to stop waiting at
// the gate. Safe to call when no process is injected.
func (m Model) cancelProcess() {
	if m.process != nil {
		m.process.Cancel()
	}
}

// cancelLoad signals the file-load goroutine to stop waiting at the
// file gate. Safe to call when no load is pending.
func (m Model) cancelLoad() {
	if m.loadCancel != nil {
		select {
		case <-m.loadCancel:
			// Already closed.
		default:
			close(m.loadCancel)
		}
	}
}

// watchFailure returns a command that waits on the failure signal
// channel and emits a ControlledFailureMsg when it fires.
func (m Model) watchFailure() tea.Cmd {
	return func() tea.Msg {
		diag, ok := <-m.failSig
		if !ok {
			return nil
		}
		return ControlledFailureMsg{Diagnostic: diag}
	}
}

// watchDiagnostic returns a command that waits on the diagnostic
// signal channel and emits a DiagnosticMsg when it fires (Issue #11).
// The diagnostic is collected into the session collection without
// display; the entry point replays it to stderr after terminal
// restoration.
func (m Model) watchDiagnostic() tea.Cmd {
	return func() tea.Msg {
		diag, ok := <-m.diagSig
		if !ok {
			return nil
		}
		return DiagnosticMsg{Diagnostic: diag}
	}
}

// loadFile returns a command that asynchronously loads the current
// browse file (the cursor's current stop's file). The command waits at
// the file gate (if set), calls the file loader, and returns a
// FileLoadCompleteMsg with the prepared buffer. The load is
// cancellable via loadCancel.
func (m Model) loadFile() tea.Cmd {
	if m.cursor == nil {
		return nil
	}
	stop, ok := m.cursor.Stop()
	if !ok {
		return nil
	}
	return m.loadFileFor(stop.RawPath)
}

// loadFileFor returns a command that asynchronously loads the file at
// the given raw path. The command waits at the file gate (if set),
// calls the file loader with the stops for that file, and returns a
// FileLoadCompleteMsg with the prepared buffer. The load is
// cancellable via loadCancel. Issue #13 uses this for cross-file
// navigation to an uncached destination.
func (m Model) loadFileFor(path []byte) tea.Cmd {
	return func() tea.Msg {
		if m.index == nil {
			return FileLoadCompleteMsg{}
		}
		stops := m.index.Stops()
		var fileStops []searchindex.Stop
		for _, s := range stops {
			if bytes.Equal(s.RawPath, path) {
				fileStops = append(fileStops, s)
			}
		}

		if m.fileGate != nil {
			select {
			case <-m.fileGate:
			case <-m.loadCancel:
				return nil
			}
		}

		loader := m.fileLoader
		if loader == nil {
			loader = filebuffer.Load
		}
		buf, err := loader(path, fileStops)
		if err != nil {
			return FileLoadCompleteMsg{Path: path, Err: err}
		}
		return FileLoadCompleteMsg{Path: path, Buffer: buf}
	}
}

// fileGroup is a set of stops for one file, identified by raw path.
type fileGroup struct {
	path  []byte
	stops []searchindex.Stop
}

// groupByFile groups stops by raw path. The index returns stops
// ordered by unsigned raw path bytes then line number, so stops for
// the same file are contiguous.
func groupByFile(stops []searchindex.Stop) []fileGroup {
	var groups []fileGroup
	for _, s := range stops {
		if len(groups) == 0 || !bytes.Equal(groups[len(groups)-1].path, s.RawPath) {
			groups = append(groups, fileGroup{path: s.RawPath, stops: []searchindex.Stop{s}})
		} else {
			groups[len(groups)-1].stops = append(groups[len(groups)-1].stops, s)
		}
	}
	return groups
}

// renderBrowse renders the two-pane browse view: file list on the left,
// content panel on the right. Each line is wrapped in the theme's base
// colours; styled spans within (matches, current-file underline) restore
// the base so text after them remains in base. Issue #13: the current
// file and current matched line derive from the cursor; the file list
// underline follows the cursor's current file.
func (m Model) renderBrowse() string {
	if m.index == nil || m.index.Files() == 0 {
		return m.theme.Base("No results")
	}

	groups := groupByFile(m.index.Stops())

	// Issue #13: the current file derives from the cursor. Find the
	// file group index for the cursor's current stop's raw path.
	currentFileIdx := 0
	currentLine := 0
	if m.cursor != nil {
		if stop, ok := m.cursor.Stop(); ok {
			for i, g := range groups {
				if bytes.Equal(g.path, stop.RawPath) {
					currentFileIdx = i
					break
				}
			}
			currentLine = stop.LineNumber
		}
	}

	// File list (left pane).
	listWidth := fileListWidth(m.width)
	var listLines []string
	for i, g := range groups {
		escaped := safepresentation.EscapePath(g.path)
		entry := escaped.Text
		if i == currentFileIdx {
			entry = m.theme.Underline(entry)
		}
		listLines = append(listLines, entry)
	}

	// Content panel (right pane).
	current := groups[currentFileIdx]
	escapedName := safepresentation.EscapePath(current.path)
	panel := m.renderContentPanel(escapedName.Text, currentLine)

	// Join horizontally: pad each file-list line to listWidth, then
	// append the corresponding content-panel line. Each composed line
	// is wrapped in the theme's base colours.
	panelLines := strings.Split(panel, "\n")
	maxLines := len(listLines)
	if len(panelLines) > maxLines {
		maxLines = len(panelLines)
	}
	var b strings.Builder
	for i := 0; i < maxLines; i++ {
		var listEntry, panelLine string
		if i < len(listLines) {
			listEntry = listLines[i]
		}
		if i < len(panelLines) {
			panelLine = panelLines[i]
		}
		// Pad list entry to listWidth.
		if w := visibleWidth(listEntry); w < listWidth {
			listEntry += strings.Repeat(" ", listWidth-w)
		}
		line := listEntry + " " + panelLine
		b.WriteString(m.theme.Base(line))
		if i < maxLines-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// renderContentPanel renders the right pane: filename rule followed by
// content rows or the loading placeholder. The currentLine parameter
// identifies the current matched line for current-match styling. When
// the viewport is active, only the visible row range is queried from
// prepared data (Issue #12); the render path never scans the full
// buffer per frame.
func (m Model) renderContentPanel(escapedName string, currentLine int) string {
	var b strings.Builder
	b.WriteString("── " + escapedName + " ──")
	b.WriteString("\n")
	if m.loading || m.buffer == nil || m.viewport == nil {
		b.WriteString("Loading…")
		return b.String()
	}
	visible := m.viewport.Visible()
	gw := m.buffer.GutterWidth - 2
	if gw < 1 {
		gw = 1
	}
	for _, line := range visible {
		if line.Continuation {
			// Issue #16: continuation rows have a blank gutter
			// aligned with the first row's text.
			b.WriteString(strings.Repeat(" ", gw+2))
		} else {
			b.WriteString(fmt.Sprintf("%*d  ", gw, line.Number))
		}
		b.WriteString(renderLineWithHighlights(line, m.theme, currentLine))
		b.WriteString("\n")
	}
	return b.String()
}

// renderLineWithHighlights escapes the display text through the
// safe-presentation core and applies the true-inverse match style to
// highlighted cell ranges. On the current matched line (line.Number ==
// currentLine), the current-match style adds underline. With the
// no-style theme, no ANSI sequences are produced.
func renderLineWithHighlights(line filebuffer.Line, t theme.Theme, currentLine int) string {
	escaped := safepresentation.EscapeContent([]byte(line.Display))
	display := escaped.Text
	if t.IsNoStyle() || len(line.Highlights) == 0 {
		return display
	}
	isCurrent := line.Number == currentLine
	cellCount := visibleWidth(display)
	var b strings.Builder
	bytePos := 0
	cellPos := 0
	for _, hl := range line.Highlights {
		if hl[0] < cellPos {
			continue
		}
		startCell := hl[0]
		endCell := hl[1]
		if startCell >= cellCount {
			continue
		}
		if endCell > cellCount {
			endCell = cellCount
		}
		startByte := cellToBytePos(display, startCell)
		endByte := cellToBytePos(display, endCell)
		if startByte > bytePos {
			b.WriteString(display[bytePos:startByte])
		}
		if endByte > startByte {
			if isCurrent {
				b.WriteString(t.CurrentMatch(display[startByte:endByte]))
			} else {
				b.WriteString(t.Match(display[startByte:endByte]))
			}
		}
		bytePos = endByte
		cellPos = endCell
	}
	if bytePos < len(display) {
		b.WriteString(display[bytePos:])
	}
	return b.String()
}

// cellToBytePos converts a cell position to a byte position in a
// display string. Each rune is one cell (first-pass mapping pending
// Issue #16's width policy).
func cellToBytePos(s string, cell int) int {
	pos := 0
	for i := 0; i < cell && pos < len(s); i++ {
		_, size := utf8.DecodeRuneInString(s[pos:])
		pos += size
	}
	return pos
}

// visibleWidth returns the number of visible cells in s, excluding
// ANSI escape sequences. Each rune is one cell (first-pass mapping
// pending Issue #16's width policy).
func visibleWidth(s string) int {
	var w int
	for i := 0; i < len(s); {
		if s[i] == '\x1b' {
			// Skip ANSI escape sequence: \x1b[...m.
			i++
			for i < len(s) && s[i] != 'm' {
				i++
			}
			if i < len(s) {
				i++ // skip 'm'
			}
			continue
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		w++
		i += size
	}
	return w
}

// fileListWidth returns a simple fixed width for the file list. Issue
// #24 owns the real formula.
func fileListWidth(termWidth int) int {
	const min = 20
	if termWidth <= 80 {
		return min
	}
	w := termWidth / 4
	if w < min {
		w = min
	}
	return w
}

// collectResults drains both pipes concurrently, parses stdout JSON
// records into a SearchIndex, waits for rg to exit, holds at the gate
// if set (cancellable), prepares the index, and returns a
// SearchCompleteMsg. On cancellation it skips index preparation and
// returns promptly.
func (m Model) collectResults() tea.Cmd {
	return func() tea.Msg {
		p := m.process
		defer func() {
			if p.done != nil {
				close(p.done)
			}
		}()

		// Drain stderr concurrently so it cannot block the child.
		var stderrBuf bytes.Buffer
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			io.Copy(&stderrBuf, p.Stderr)
		}()

		// Parse stdout records into the index builder using the
		// bounded 64 MiB record reader. Oversized records are
		// discarded through the next newline; malformed and unknown
		// records are counted by the builder.
		builder := searchindex.NewBuilder(m.workdir)
		if _, err := builder.ReadFrom(p.Stdout); err != nil {
			// ReadFrom errors are I/O failures from the process pipe;
			// the stream is treated as incomplete. The builder's
			// integrity flags are not set by I/O errors, so we mark
			// the stream as having a trailing malformed record to
			// ensure the outcome is fatal.
			builder.MarkTrailingMalformed()
		}

		// Wait for stderr drain to complete.
		wg.Wait()

		// Wait for rg to exit and reap it.
		waitErr := p.Cmd.Wait()
		if p.OnReap != nil {
			p.OnReap(waitErr)
		}

		// Hold at the gate if set (test seam). Cancellation releases
		// the gate so the goroutine finishes promptly.
		if m.gate != nil {
			select {
			case <-m.gate:
			case <-p.cancel:
				// Cancelled: skip index preparation.
				return SearchCompleteMsg{}
			}
		}

		// Prepare the index.
		idx := builder.Build()

		return SearchCompleteMsg{
			Files:   idx.Files(),
			Lines:   idx.Len(),
			Index:   idx,
			Process: processResult(waitErr),
			Stderr:  stderrBuf.String(),
		}
	}
}

// processResult derives the ProcessResult from the child's wait error.
// A nil error is exit code 0. An exec.ExitError carries the exit code
// or, for signal death, the signal number with SignalDeath set.
func processResult(waitErr error) ProcessResult {
	if waitErr == nil {
		return ProcessResult{}
	}
	if ee, ok := waitErr.(*exec.ExitError); ok {
		if state := ee.Sys(); state != nil {
			return processResultFromSys(state)
		}
		return ProcessResult{ExitCode: ee.ExitCode()}
	}
	return ProcessResult{ExitCode: 1}
}

// processResultFromSys derives the ProcessResult from the platform
// wait status. On Unix, a signaled process reports Signaled() with the
// signal number; otherwise the exit code is used.
func processResultFromSys(state any) ProcessResult {
	if ws, ok := state.(syscall.WaitStatus); ok {
		if ws.Signaled() {
			return ProcessResult{ExitCode: int(ws.Signal()), SignalDeath: true}
		}
		if ws.Exited() {
			return ProcessResult{ExitCode: ws.ExitStatus()}
		}
	}
	return ProcessResult{ExitCode: 1}
}

// sanitizeDiagnostic escapes raw diagnostic bytes for safe display while
// preserving real line boundaries. It delegates to
// safepresentation.EscapeDiagnostic, the shared safe-presentation
// utility for diagnostic escaping: LF is preserved as a line boundary;
// CRLF is normalized to LF; tabs are expanded to eight-column stops;
// other C0 controls and DEL use caret notation; C1 controls use \u00XX
// escapes; invalid UTF-8 bytes use \xNN. Backslashes are not escaped so
// that filenames already escaped through EscapePath can be embedded
// without double-escaping.
func sanitizeDiagnostic(s string) string {
	return safepresentation.EscapeDiagnostic([]byte(s))
}

// collectDiagnostic sanitizes s through sanitizeDiagnostic, appends it to
// the session diagnostic collection, and fires the onCollect callback if
// set (Issue #11). The callback is the application-side acknowledgement
// side channel: it fires once a diagnostic has been processed into the
// collection, in the same mechanism family as Process.OnReap. The
// shutdown boundary is defined here: a diagnostic is "collected" once the
// model has processed the message carrying it. A diagnostic still in
// flight (e.g., a gated SearchCompleteMsg) has not been processed and is
// not collected, not waited for, and not replayed.
func (m *Model) collectDiagnostic(s string) {
	sanitized := sanitizeDiagnostic(s)
	m.diagnostics = append(m.diagnostics, sanitized)
	if m.onCollect != nil {
		m.onCollect(sanitized)
	}
}

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
	browseIdx  int
	buffer     *filebuffer.Buffer
	loading    bool
	theme      theme.Theme
	fileLoader FileLoader
	fileGate   chan struct{}
	loadCancel chan struct{}

	// viewport is the scrollable content view for the current file.
	// It holds prepared row data and the vertical offset. When nil
	// (loading or no buffer), the render path shows the placeholder.
	viewport *viewport.Viewport
	// currentPath is the raw path of the currently loaded file, used
	// as the key for per-file viewport state.
	currentPath []byte
	// perFileOffset saves the vertical viewport offset per raw path so
	// a file revisited later can start from its saved position (Issue
	// #12). The key is the string form of the raw path bytes.
	perFileOffset map[string]int
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

// New creates a new app model for a search invocation. The model starts
// in the searching state.
func New(childArgs []string, workdir string, opts ...Option) Model {
	cfg := config{}
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
		loadCancel:         make(chan struct{}),
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

// SavedOffset returns the saved per-file vertical viewport offset for
// the given raw path, or 0 if no state is saved (Issue #12). This is
// the per-file state saved for later revisits; a first visit returns 0.
func (m Model) SavedOffset(path []byte) int {
	if m.perFileOffset == nil {
		return 0
	}
	return m.perFileOffset[string(path)]
}

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
				m.browseIdx = 0
				m.loading = true
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
		}
		m.loading = false
		// Build the viewport from the prepared row data. The row
		// provider factory (or the default viewport.BufferRows)
		// adapts the buffer so the render path queries only the
		// visible range. The per-file saved offset is restored so a
		// revisited file starts from its saved position (Issue #12).
		if msg.Buffer != nil {
			m.currentPath = msg.Path
			factory := m.rowProviderFactory
			if factory == nil {
				factory = viewport.BufferRows
			}
			rows := factory(msg.Buffer)
			offset := m.SavedOffset(msg.Path)
			m.viewport = viewport.New(rows, m.height)
			m.viewport.SetOffset(offset)
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
		// Scroll keys are active in the browse state when content is
		// loaded. Scrolling a "Loading…" placeholder is a no-op
		// (Issue #12). Manual scrolling does not move the matched-line
		// cursor.
		if m.state == StateBrowse && m.viewport != nil {
			if m.handleScrollKey(msg) {
				return m, nil
			}
		}
		switch {
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
		// #12).
		if m.viewport != nil {
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
// browse file. The command waits at the file gate (if set), calls the
// file loader, and returns a FileLoadCompleteMsg with the prepared
// buffer. The load is cancellable via loadCancel.
func (m Model) loadFile() tea.Cmd {
	return func() tea.Msg {
		if m.index == nil {
			return FileLoadCompleteMsg{}
		}
		stops := m.index.Stops()
		if len(stops) == 0 {
			return FileLoadCompleteMsg{}
		}
		path := stops[0].RawPath
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
// the base so text after them remains in base.
func (m Model) renderBrowse() string {
	if m.index == nil || m.index.Files() == 0 {
		return m.theme.Base("No results")
	}

	groups := groupByFile(m.index.Stops())

	// File list (left pane).
	listWidth := fileListWidth(m.width)
	var listLines []string
	for i, g := range groups {
		escaped := safepresentation.EscapePath(g.path)
		entry := escaped.Text
		if i == m.browseIdx {
			entry = m.theme.Underline(entry)
		}
		listLines = append(listLines, entry)
	}

	// Content panel (right pane).
	current := groups[m.browseIdx]
	escapedName := safepresentation.EscapePath(current.path)
	// The current matched line is the first stop for the current file
	// (the first stop until Issue #13 adds navigation).
	currentLine := 0
	if len(current.stops) > 0 {
		currentLine = current.stops[0].LineNumber
	}
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
		b.WriteString(fmt.Sprintf("%*d  ", gw, line.Number))
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

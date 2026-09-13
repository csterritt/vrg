// Package app owns the Bubble Tea model, lifecycle, asynchronous work,
// overlays, cleanup, and rendering for vrg.
package app

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
	"vrg/internal/theme"
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
	state      State
	width      int
	height     int
	files      int
	lines      int
	diagnostic string
	exitCode   int
	cancelled  bool

	childArgs []string
	workdir   string
	process   *Process
	gate      chan struct{}
	failSig   <-chan string

	// Browse state.
	index      *searchindex.Index
	browseIdx  int
	buffer     *filebuffer.Buffer
	loading    bool
	theme      theme.Theme
	fileLoader FileLoader
	fileGate   chan struct{}
	loadCancel chan struct{}
}

type config struct {
	gate       chan struct{}
	process    *Process
	failSignal <-chan string
	theme      theme.Theme
	fileLoader FileLoader
	fileGate   chan struct{}
}

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

// New creates a new app model for a search invocation. The model starts
// in the searching state.
func New(childArgs []string, workdir string, opts ...Option) Model {
	cfg := config{}
	for _, opt := range opts {
		opt(&cfg)
	}
	return Model{
		state:      StateSearching,
		childArgs:  childArgs,
		workdir:    workdir,
		process:    cfg.process,
		gate:       cfg.gate,
		failSig:    cfg.failSignal,
		theme:      cfg.theme,
		fileLoader: cfg.fileLoader,
		fileGate:   cfg.fileGate,
		loadCancel: make(chan struct{}),
	}
}

// State returns the current app state.
func (m Model) State() State { return m.state }

// ExitCode returns the process exit code the entry point should use.
func (m Model) ExitCode() int { return m.exitCode }

// Diagnostic returns the sanitized diagnostic for stderr replay. Valid
// after the model has received a SearchFailedMsg or ControlledFailureMsg.
func (m Model) Diagnostic() string { return m.diagnostic }

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
		if msg.Index != nil && msg.Index.Files() > 0 {
			m.state = StateBrowse
			m.files = msg.Files
			m.lines = msg.Lines
			m.index = msg.Index
			m.browseIdx = 0
			m.loading = true
			return m, m.loadFile()
		}
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
		return m, nil

	case SearchFailedMsg:
		m.state = StateStartFailed
		m.diagnostic = sanitizeDiagnostic(msg.Diagnostic)
		m.exitCode = 2
		return m, tea.Quit

	case ControlledFailureMsg:
		m.state = StateFailed
		m.diagnostic = sanitizeDiagnostic(msg.Diagnostic)
		m.exitCode = 2
		m.cancelled = true
		m.cancelProcess()
		m.cancelLoad()
		return m, tea.Quit

	case tea.KeyPressMsg:
		switch {
		case msg.Code == 'c' && msg.Mod == tea.ModCtrl:
			return m.cancel()
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
			case StateBrowse:
				m.exitCode = 0
				m.cancelled = true
				m.cancelProcess()
				m.cancelLoad()
				return m, tea.Quit
			}
		case msg.Code == tea.KeyEscape:
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}
	return m, nil
}

// View renders the current state.
func (m Model) View() tea.View {
	switch m.state {
	case StateSearching:
		v := tea.NewView("Searching…")
		v.AltScreen = true
		return v
	case StateSummary:
		v := tea.NewView(fmt.Sprintf("%d files, %d matched lines", m.files, m.lines))
		v.AltScreen = true
		return v
	case StateBrowse:
		v := tea.NewView(m.renderBrowse())
		v.AltScreen = true
		return v
	default:
		return tea.NewView("")
	}
}

// SearchCompleteMsg signals that collection and index preparation are
// done. Index is nil for the backward-compatible summary path; non-nil
// with results for the browse path.
type SearchCompleteMsg struct {
	Files int
	Lines int
	Index *searchindex.Index
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
// identifies the current matched line for current-match styling.
func (m Model) renderContentPanel(escapedName string, currentLine int) string {
	var b strings.Builder
	b.WriteString("── " + escapedName + " ──")
	b.WriteString("\n")
	if m.loading || m.buffer == nil {
		b.WriteString("Loading…")
		return b.String()
	}
	for _, line := range m.buffer.Lines {
		gw := m.buffer.GutterWidth - 2
		if gw < 1 {
			gw = 1
		}
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

		// Parse stdout line by line into the index builder.
		builder := searchindex.NewBuilder(m.workdir)
		scanner := bufio.NewScanner(p.Stdout)
		scanner.Buffer(make([]byte, 64*1024), 64*1024*1024)
		for scanner.Scan() {
			builder.Add(scanner.Bytes())
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
			Files: idx.Files(),
			Lines: idx.Len(),
			Index: idx,
		}
	}
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

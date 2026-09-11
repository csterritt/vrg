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

	tea "charm.land/bubbletea/v2"

	"vrg/internal/searchindex"
)

// State identifies the current app state.
type State int

const (
	// StateSearching is the initial state while rg is running and results
	// are being collected and indexed.
	StateSearching State = iota
	// StateSummary is the interim summary state shown after collection
	// completes.
	StateSummary
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
}

type config struct {
	gate       chan struct{}
	process    *Process
	failSignal <-chan string
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

// New creates a new app model for a search invocation. The model starts
// in the searching state.
func New(childArgs []string, workdir string, opts ...Option) Model {
	cfg := config{}
	for _, opt := range opts {
		opt(&cfg)
	}
	return Model{
		state:     StateSearching,
		childArgs: childArgs,
		workdir:   workdir,
		process:   cfg.process,
		gate:      cfg.gate,
		failSig:   cfg.failSignal,
	}
}

// State returns the current app state.
func (m Model) State() State { return m.state }

// ExitCode returns the process exit code the entry point should use.
func (m Model) ExitCode() int { return m.exitCode }

// Diagnostic returns the sanitized diagnostic for stderr replay. Valid
// after the model has received a SearchFailedMsg or ControlledFailureMsg.
func (m Model) Diagnostic() string { return m.diagnostic }

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
		m.state = StateSummary
		m.files = msg.Files
		m.lines = msg.Lines
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
		return m, tea.Quit

	case tea.KeyPressMsg:
		switch {
		case msg.Code == 'c' && msg.Mod == tea.ModCtrl:
			return m.cancel()
		case msg.Code == 'q' && msg.Mod == 0:
			switch m.state {
			case StateSearching:
				return m.cancel()
			case StateSummary:
				m.exitCode = 0
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
	default:
		return tea.NewView("")
	}
}

// SearchCompleteMsg signals that collection and index preparation are
// done.
type SearchCompleteMsg struct {
	Files int
	Lines int
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
	return m, tea.Quit
}

// cancelProcess signals the collection goroutine to stop waiting at
// the gate. Safe to call when no process is injected.
func (m Model) cancelProcess() {
	if m.process != nil {
		m.process.Cancel()
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
		}
	}
}

// sanitizeDiagnostic removes raw control bytes from a diagnostic string
// so it is safe to write to stderr.
func sanitizeDiagnostic(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == 0x1b || r == 0x9b {
			return ' '
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}

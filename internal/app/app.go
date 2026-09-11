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
)

// Model is the Bubble Tea model for the vrg app.
type Model struct {
	state      State
	width      int
	height     int
	files      int
	lines      int
	diagnostic string
	exitCode   int

	childArgs []string
	workdir   string
	process   *Process
	gate      chan struct{}
}

// Process holds a running rg subprocess and its stdout/stderr pipes for
// the model to drain.
type Process struct {
	Cmd    *exec.Cmd
	Stdout io.Reader
	Stderr io.Reader
}

type config struct {
	gate    chan struct{}
	process *Process
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
func WithProcess(p Process) Option {
	return func(c *config) { c.process = &p }
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
	}
}

// State returns the current app state.
func (m Model) State() State { return m.state }

// ExitCode returns the process exit code the entry point should use.
func (m Model) ExitCode() int { return m.exitCode }

// Diagnostic returns the sanitized diagnostic for stderr replay. Valid
// after the model has received a SearchFailedMsg.
func (m Model) Diagnostic() string { return m.diagnostic }

// Init returns the initial command. If a process was injected, the
// command drains both pipes, collects results, and prepares the index.
// Without a process (test mode), Init returns nil.
func (m Model) Init() tea.Cmd {
	if m.process == nil {
		return nil
	}
	return m.collectResults()
}

// Update handles messages and returns the updated model and command.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SearchCompleteMsg:
		m.state = StateSummary
		m.files = msg.Files
		m.lines = msg.Lines
		return m, nil

	case SearchFailedMsg:
		m.state = StateStartFailed
		m.diagnostic = sanitizeDiagnostic(msg.Diagnostic)
		m.exitCode = 2
		return m, tea.Quit

	case tea.KeyPressMsg:
		switch {
		case msg.Code == 'c' && msg.Mod == tea.ModCtrl:
			m.exitCode = 130
			return m, tea.Quit
		case msg.Code == 'q' && msg.Mod == 0:
			switch m.state {
			case StateSearching:
				m.exitCode = 130
				return m, tea.Quit
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
		return tea.NewView("Searching…")
	case StateSummary:
		return tea.NewView(fmt.Sprintf("%d files, %d matched lines", m.files, m.lines))
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

// collectResults drains both pipes concurrently, parses stdout JSON
// records into a SearchIndex, waits for rg to exit, holds at the gate if
// set, prepares the index, and returns a SearchCompleteMsg.
func (m Model) collectResults() tea.Cmd {
	return func() tea.Msg {
		p := m.process

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

		// Wait for rg to exit.
		_ = p.Cmd.Wait()

		// Hold at the gate if set (test seam).
		if m.gate != nil {
			<-m.gate
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

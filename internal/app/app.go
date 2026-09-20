package app

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/searchindex"
)

// state identifies the current screen of the search lifecycle.
type state int

const (
	// stateSearching covers collection and post-exit processing: the
	// whole interval until the index is ready, even after rg has exited.
	stateSearching state = iota
	// stateSummary is the interim post-search summary screen that later
	// issues replace with browsing.
	stateSummary
	// stateCancelled is the terminal cancellation state: the program is
	// quitting with status 130 and late completions must not revive it.
	stateCancelled
)

// Model is the Bubble Tea model for the vrg lifecycle; this slice covers
// the searching screen, the interim summary, and cancellation.
type Model struct {
	child   Child
	workdir string
	// gate, when non-nil, is awaited between child exit and index
	// preparation so tests can hold preparation independently of rg exit.
	gate <-chan struct{}
	// ctx and cancel drive cancellation of collection: cancelling
	// releases a gate-held preparation promptly.
	ctx    context.Context
	cancel context.CancelFunc

	width  int
	height int
	state  state

	index   *searchindex.Index
	stderr  []byte
	procErr error
	status  int
}

// New returns a Model that collects the started child's stream. The
// child must already be running; spawn failures are handled by Run
// before the TUI exists.
func New(child Child, workdir string) Model {
	ctx, cancel := context.WithCancel(context.Background())
	return Model{child: child, workdir: workdir, state: stateSearching, ctx: ctx, cancel: cancel}
}

// Init starts collection and index preparation off the UI update path.
func (m Model) Init() tea.Cmd {
	ctx, child, workdir, gate := m.ctx, m.child, m.workdir, m.gate
	return func() tea.Msg {
		return collect(ctx, child, workdir, gate)
	}
}

// Update applies messages to the model. Collection results arrive as
// searchResult; keys act per state — q cancels while searching and quits
// the interim summary, and ctrl+c cancels in any state. Esc is not an
// exit key and is a no-op outside overlays.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case searchResult:
		if m.state == stateCancelled {
			// A late completion must not revive a cancelled run.
			return m, nil
		}
		m.index = msg.index
		m.stderr = msg.stderr
		m.procErr = msg.err
		m.state = stateSummary
	case tea.KeyPressMsg:
		if m.state == stateCancelled {
			return m, nil
		}
		switch msg.String() {
		case "ctrl+c":
			return m.cancelRun()
		case "q":
			switch m.state {
			case stateSearching:
				return m.cancelRun()
			case stateSummary:
				return m, tea.Quit
			}
		}
	}
	return m, nil
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
	if m.state == stateSummary && m.index != nil {
		return fmt.Sprintf("%d files, %d matched lines", len(m.index.Files()), len(m.index.Stops()))
	}
	return "Searching…"
}

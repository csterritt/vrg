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
)

// Model is the Bubble Tea model for the vrg lifecycle; this slice covers
// the searching screen and the interim summary.
type Model struct {
	child   Child
	workdir string
	// gate, when non-nil, is awaited between child exit and index
	// preparation so tests can hold preparation independently of rg exit.
	gate <-chan struct{}

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
	return Model{child: child, workdir: workdir, state: stateSearching}
}

// Init starts collection and index preparation off the UI update path.
func (m Model) Init() tea.Cmd {
	child, workdir, gate := m.child, m.workdir, m.gate
	return func() tea.Msg {
		return collect(context.Background(), child, workdir, gate)
	}
}

// Update applies messages to the model. Collection results arrive as
// searchResult; keys act per state — q quits the interim summary.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case searchResult:
		m.index = msg.index
		m.stderr = msg.stderr
		m.procErr = msg.err
		m.state = stateSummary
	case tea.KeyPressMsg:
		if m.state == stateSummary && msg.String() == "q" {
			return m, tea.Quit
		}
	}
	return m, nil
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

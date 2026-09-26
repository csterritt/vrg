package app

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"vrg/internal/searchindex"
)

// phase is the model's coarse state. Later issues add browsing between
// searching and exit.
type phase int

const (
	// phaseSearching covers the whole of collection and post-exit
	// processing: parsing, sorting, and index preparation. The app
	// stays here until the prepared index arrives, even after rg has
	// exited.
	phaseSearching phase = iota
	// phaseSummary is the interim result screen shown once the index is
	// ready; q dismisses it with exit 0.
	phaseSummary
)

// Model is the Bubble Tea application model: it owns the search
// lifecycle state and renders the searching and interim summary screens.
type Model struct {
	phase  phase
	done   <-chan searchDoneMsg
	index  *searchindex.Index
	width  int
	height int
	code   int
	quit   bool
}

// newModel returns a searching model awaiting the collection result on
// done.
func newModel(done <-chan searchDoneMsg) Model {
	return Model{phase: phaseSearching, done: done}
}

// Init returns the command that waits for collection to finish and
// delivers the search-done message. Collection runs off this path; the
// model only receives its result.
func (m Model) Init() tea.Cmd {
	return func() tea.Msg { return <-m.done }
}

// Update applies one message. Resize is handled in any state so the UI
// stays responsive during collection; a search-done message moves the
// model to the interim summary; q quits the summary with exit 0.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case searchDoneMsg:
		m.phase = phaseSummary
		m.index = msg.index
	case tea.KeyPressMsg:
		if m.phase == phaseSummary && msg.String() == "q" {
			m.quit = true
			return m, tea.Quit
		}
	}
	return m, nil
}

// View renders the current screen. During the whole collection and
// post-exit preparation span it is the searching screen; afterwards it
// is the interim "N files, M matched lines" summary.
func (m Model) View() tea.View {
	if m.phase == phaseSummary {
		return tea.NewView(m.summaryLine() + "\n")
	}
	return tea.NewView("Searching…\n")
}

// summaryLine renders the interim result count.
func (m Model) summaryLine() string {
	files, lines := 0, 0
	if m.index != nil {
		files, lines = m.index.FileCount(), m.index.LineCount()
	}
	return fmt.Sprintf("%d %s, %d matched %s", files, plural(files, "file"), lines, plural(lines, "line"))
}

func plural(n int, s string) string {
	if n == 1 {
		return s
	}
	return s + "s"
}

// ExitCode is the process exit status the model settled on.
func (m Model) ExitCode() int { return m.code }

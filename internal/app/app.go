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
// cancel terminates the rg child and abandons collection; the process
// boundary confirms the reap before exiting.
type Model struct {
	phase  phase
	cancel func()
	done   <-chan searchDoneMsg
	index  *searchindex.Index
	width  int
	height int
	code   int
	quit   bool
}

// newModel returns a searching model awaiting the collection result on
// done; cancel terminates the search's child (nil means none).
func newModel(done <-chan searchDoneMsg, cancel func()) Model {
	if cancel == nil {
		cancel = func() {}
	}
	return Model{phase: phaseSearching, cancel: cancel, done: done}
}

// Init returns the command that waits for collection to finish and
// delivers the search-done message. Collection runs off this path; the
// model only receives its result.
func (m Model) Init() tea.Cmd {
	return func() tea.Msg { return <-m.done }
}

// Update applies one message. Resize is handled in any state so the UI
// stays responsive during collection; a search-done message moves the
// model to the interim summary; q quits the summary with exit 0. ctrl+c
// in any state — and q while searching, which covers the post-exit
// preparation window — cancel the search and exit 130. Esc is a base-state
// no-op. Once the model has committed to quitting, late messages
// (including a search completion racing cancellation) are discarded.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.quit {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case searchDoneMsg:
		m.phase = phaseSummary
		m.index = msg.index
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return m.cancelled(), tea.Quit
		case "q":
			if m.phase == phaseSearching {
				return m.cancelled(), tea.Quit
			}
			m.quit = true
			return m, tea.Quit
		}
	}
	return m, nil
}

// cancelled marks the model quit-by-cancellation: exit 130 once the
// process boundary has terminated and reaped the child and restored the
// terminal.
func (m Model) cancelled() Model {
	m.quit = true
	m.code = 130
	m.cancel()
	return m
}

// View renders the current screen on the alternate screen; on every
// exit Bubble Tea emits the display-restoration sequence (leave alt
// screen, cursor visible). During the whole collection and post-exit
// preparation span the screen is "Searching…"; afterwards it is the
// interim "N files, M matched lines" summary.
func (m Model) View() tea.View {
	var v tea.View
	if m.phase == phaseSummary {
		v = tea.NewView(m.summaryLine() + "\n")
	} else {
		v = tea.NewView("Searching…\n")
	}
	v.AltScreen = true
	return v
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

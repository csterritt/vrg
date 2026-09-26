package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// The fixed terminal minimum (Issue #33): below either dimension the
// too-small gate replaces the whole screen.
const (
	minTermCols = 20
	minTermRows = 3
)

// tooSmall reports whether the terminal is below the fixed minimum —
// under 20 columns or under 3 rows — when the "Terminal too small"
// gate owns the screen.
func (m Model) tooSmall() bool {
	return m.width < minTermCols || m.height < minTermRows
}

// tooSmallKey handles one key while the gate is up: only q and ctrl+c
// act. q exits with the state-applicable outcome — the cancellation
// path while searching, else the settled fixed status — and takes
// precedence over Issue #32's dismissal semantics: a logically open
// overlay is exited past, never dismissed, so the hidden modal cannot
// trap the user. ctrl+c exits 130. Esc and every other key are no-ops;
// the modal stack keeps its scroll positions for recovery.
func (m Model) tooSmallKey(key string) (Model, tea.Cmd) {
	switch key {
	case "ctrl+c":
		return m.cancelled(), tea.Quit
	case "q":
		if m.phase == phaseSearching {
			return m.cancelled(), tea.Quit
		}
		m.quit = true
		return m, tea.Quit
	}
	return m, nil
}

// renderTooSmall composes the gate screen: "Terminal too small"
// centred on the middle row, clipped to the frame when the text itself
// is wider than the terminal, every row padded to the frame edge so
// the base style covers the screen.
func (m Model) renderTooSmall() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	line := ansi.Truncate("Terminal too small", m.width, "")
	pad := max(0, (m.width-ansi.StringWidth(line))/2)
	rows := make([]string, m.height)
	for r := range rows {
		row := ""
		if r == m.height/2 {
			row = strings.Repeat(" ", pad) + line
		}
		rows[r] = row + strings.Repeat(" ", max(0, m.width-ansi.StringWidth(row)))
	}
	return strings.Join(rows, "\n")
}

package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/safepresentation"
)

// The fixed terminal minimum: below 20 columns or 3 rows the too-small
// gate installs — the ordinary layout, overlays, and pop-ups all yield
// to the single centred message.
const (
	minWidth  = 20
	minHeight = 3
)

// tooSmall reports whether the terminal is below the fixed minimum,
// gating the display to the "Terminal too small" screen and input to
// the two exit keys.
func (m Model) tooSmall() bool {
	w, h := m.termSize()
	return w < minWidth || h < minHeight
}

// updateTooSmall applies the only live keys under the gate: q exits
// with the state's applicable outcome — cancellation at 130 while
// searching, the fixed status otherwise — taking precedence over the
// Issue 32 overlay-dismissal semantics so the screen never traps the
// user behind an invisible modal, and ctrl+c cancels at 130. Every
// other key, Esc included, is a strict no-op: a logically open overlay
// stays open and a live pop-up is not dismissed.
func (m Model) updateTooSmall(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m.cancelRun()
	case "q":
		if m.state == stateSearching {
			return m.cancelRun()
		}
		return m, tea.Quit
	}
	return m, nil
}

// tooSmallScreen renders the gate screen: the centred "Terminal too
// small" message clipped to whatever space the terminal affords — and
// nothing else: no underlying screen, overlay, or pop-up.
func (m Model) tooSmallScreen() string {
	w, h := m.termSize()
	const msg = "Terminal too small"
	pad := (w - safepresentation.CellWidth(msg)) / 2
	if pad < 0 {
		pad = 0
	}
	rows := make([]string, h)
	rows[h/2] = strings.Repeat(" ", pad) + clipCells(msg, w)
	return m.theme.Base(strings.Join(rows, "\n"))
}

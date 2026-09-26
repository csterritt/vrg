package app

import (
	"bytes"
	"time"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/present"
)

// popupExpiredMsg is the expiry of one file-change pop-up instance:
// id keys it to the instance that scheduled it, so a stale instance's
// expiry cannot dismiss a newer pop-up.
type popupExpiredMsg struct{ id int }

// popupTick is the production expiry command — the pop-up's
// one-second lifetime.
func popupTick(id int) tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return popupExpiredMsg{id: id}
	})
}

// openPopup mints a fresh instance showing the raw path just selected
// and returns its expiry command. The lifetime starts at selection —
// before and independent of the destination's load, whose completion
// never restarts it.
func (m *Model) openPopup(path []byte) tea.Cmd {
	m.popupSeq++
	m.popupID = m.popupSeq
	m.popupPath = bytes.Clone(path)
	timer := m.popupTimer
	if timer == nil {
		timer = popupTick
	}
	return timer(m.popupID)
}

// renderPopup composites the pop-up over the frame: the destination's
// single-line safe path inside a single-line box, centred and
// left-truncated with a leading … against the current terminal size.
// Geometry is computed at every render, so a resize recentres and
// re-truncates without dismissing the instance or restarting its
// timer.
func (m Model) renderPopup(base string) string {
	inner := truncateLeft(present.Path(m.popupPath), max(0, m.width-2))
	return m.composite(base, m.theme.Overlay([]string{inner}))
}

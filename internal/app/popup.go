package app

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/safepresentation"
)

// popupLifetime is each file-change pop-up's on-screen lifetime.
const popupLifetime = time.Second

// popup is the active file-change pop-up: a brief centred box naming
// the file navigation just selected. id keys the pop-up's expiry timer,
// so an expiry message from a stale instance cannot dismiss a newer
// pop-up; path holds the raw path bytes — escaping and truncation are
// render-time concerns.
type popup struct {
	id   int
	path []byte
}

// popupExpiredMsg reports that one pop-up instance's lifetime elapsed;
// it dismisses only the instance it names.
type popupExpiredMsg struct{ id int }

// popupTick is the production expiry command: one second on the clock,
// then the instance-keyed expiry message.
func popupTick(id int) tea.Cmd {
	return tea.Tick(popupLifetime, func(time.Time) tea.Msg {
		return popupExpiredMsg{id: id}
	})
}

// popupScreen composites the pop-up over base: a bordered one-line box
// centred at the current terminal size, holding the destination file's
// single-line safe path left-truncated to fit. Centring and truncation
// recompute from the live size on every render, so a resize recentres
// and re-truncates the pop-up without dismissing it or restarting its
// timer.
func (m Model) popupScreen(base string) string {
	w, h := m.termSize()
	text := safepresentation.TruncateLeftGrapheme(safepresentation.EscapePath(m.popup.path), w-2)
	boxW := safepresentation.CellWidth(text) + 2
	if boxW > w {
		boxW = w
	}
	box := strings.Split(m.theme.Overlay([]string{text}, boxW, 3), "\n")

	rows := strings.Split(base, "\n")
	for len(rows) < h {
		rows = append(rows, "")
	}
	if len(rows) > h {
		rows = rows[:h]
	}
	top := (h - len(box)) / 2
	if top < 0 {
		top = 0
	}
	left := (w - boxW) / 2
	if left < 0 {
		left = 0
	}
	pad := strings.Repeat(" ", left)
	for i, r := range box {
		if top+i < len(rows) {
			rows[top+i] = m.theme.Base(pad + r)
		}
	}
	return strings.Join(rows, "\n")
}

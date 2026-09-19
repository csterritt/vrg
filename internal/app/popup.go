package app

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/safepresentation"
)

// popupLifetime is each file-change pop-up's lifetime: the pop-up
// dismisses on its own instance's expiry message or on any key press,
// whichever comes first.
const popupLifetime = time.Second

// popupExpireMsg is the one-second expiry for pop-up instance id.
// Update dismisses the live pop-up only when the message's id is that
// instance's, so an expiry minted for an older pop-up is stale and
// discarded — a late timer can never dismiss a newer instance.
type popupExpireMsg struct{ id int }

// startPopup opens a fresh file-change pop-up for the current file and
// returns its one-second expiry command. Each instance gets a new ID
// from popupSeq; popupID == 0 means no pop-up is up. The command is
// built through the options seam so tests drive expiry by injecting
// popupExpireMsg rather than waiting on real time.
func (m *model) startPopup() tea.Cmd {
	m.popupSeq++
	m.popupID = m.popupSeq
	id := m.popupID
	timer := m.opts.popupTimer
	if timer == nil {
		timer = func(id int) tea.Cmd {
			return tea.Tick(popupLifetime, func(time.Time) tea.Msg {
				return popupExpireMsg{id: id}
			})
		}
	}
	return timer(id)
}

// compositePopup draws the live file-change pop-up over the base frame:
// the current file's single-line escaped path in a bordered box,
// centred on the current terminal size. Position and truncation are
// recomputed at every render, so a resize recentres and re-truncates
// the pop-up without touching its timer.
func (m *model) compositePopup(base string) string {
	w, h := m.width, m.height
	if w <= 0 || h <= 0 {
		return base
	}
	rows := strings.Split(base, "\n")
	for len(rows) < h {
		rows = append(rows, "")
	}
	if len(rows) > h {
		rows = rows[:h]
	}

	text := ""
	if m.idx != nil && len(m.idx.Files) > 0 {
		// The path fits the box interior — the frame minus the
		// border's four cells — left-truncated with a leading … so
		// the basename end stays visible. Widths are measured on the
		// unstyled text: the styled box rows carry SGR bytes a cell
		// count would mistake for content.
		text = m.entry(m.curFile()).leftTruncate(w - 4)
	}
	box := m.theme.Overlay([]string{text})
	boxW := safepresentation.CellWidth(text) + 4
	top := 0
	if len(box) < h {
		top = (h - len(box)) / 2
	}
	left := 0
	if boxW < w {
		left = (w - boxW) / 2
	}
	for i, brow := range box {
		r := top + i
		if r >= h {
			break
		}
		right := w - left - boxW
		if right < 0 {
			right = 0
		}
		rows[r] = strings.Repeat(" ", left) + brow + strings.Repeat(" ", right)
	}
	return strings.Join(rows, "\n")
}

// leftTruncate returns s clipped to at most w cells from the left with
// a leading …, keeping the tail — the basename end of a path —
// visible. Grapheme clusters are never split.
func leftTruncate(s string, w int) string {
	return newDisplayPath(s).leftTruncate(w)
}

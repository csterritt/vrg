package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/safepresentation"
)

// helpBinding is one row of the key-binding table: the key spellings
// and the action they perform.
type helpBinding struct {
	keys string
	desc string
}

// helpBindings is the single binding-table data source: every key
// binding of the browse UI — navigation, scrolling, panning, wrap,
// colour, list toggle, reload, help, and quit/cancel — in help order.
// The help overlay renders it and Issue 34's documentation test
// iterates it, so the README cannot drift from the implementation.
var helpBindings = []helpBinding{
	{"n / p", "next / previous matched line"},
	{"up / down", "scroll one row"},
	{"u / d", "scroll half a page"},
	{"pgup / pgdown", "scroll a whole page"},
	{", / .", "pan one column"},
	{"< / >", "pan ten columns"},
	{"[ / ]", "pan half the text width"},
	{"w", "toggle line wrapping"},
	{"c", "toggle colour scheme"},
	{"left / tab", "hide the file list"},
	{"right / shift+tab", "show the file list"},
	{"r", "reload the current file"},
	{"h / ?", "open or close this help"},
	{"q / esc", "quit, or dismiss the open overlay"},
	{"ctrl+c", "exit immediately with status 130"},
}

// helpFooter is the help overlay's footer slot: the note rendered after
// the binding table, empty until Issue 34 fills it with the
// scale-and-limits text. It is runtime-substituted text, so it passes
// through the safe-presentation diagnostic policy when the overlay's
// lines are built.
var helpFooter string

// openHelp builds the modal help overlay on the shared scrollable
// component: the binding table followed by the footer slot's escaped
// lines when the slot is filled.
func openHelp() *scrollOverlay {
	var lines []string
	for _, b := range helpBindings {
		lines = append(lines, fmt.Sprintf("%-18s %s", b.keys, b.desc))
	}
	if helpFooter != "" {
		lines = append(lines, "")
		lines = append(lines, strings.Split(safepresentation.EscapeDiagnostic(helpFooter), "\n")...)
	}
	return &scrollOverlay{lines: lines}
}

// updateHelp applies one key to the open help overlay: up/down scroll
// the wrapped rows, q/Esc/h/? close and return to the underlying base
// state, and every other key is ignored — nothing reaches the content
// behind the modal.
func (m Model) updateHelp(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch key := msg.String(); key {
	case "up", "down":
		m.help.scrollKey(key, m.width, m.height)
	case "q", "esc", "h", "?":
		m.help = nil
	}
	return m, nil
}

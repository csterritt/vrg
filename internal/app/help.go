package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/present"
)

// helpBinding is one row of the help overlay's key-binding table: the
// key spelling as the renderer shows it and its description.
type helpBinding struct {
	keys string
	desc string
}

// helpBindings is the single source of key-binding documentation: the
// help renderer consumes it directly and Issue #34's documentation
// test iterates it, so a binding added here lands in both places at
// once. The rows cover navigation, scrolling, panning, wrap, colour,
// the list toggle, reload, help, and quit/cancel.
var helpBindings = []helpBinding{
	{"n / p", "next / previous matched line"},
	{"up / down", "scroll one row"},
	{"u / d", "scroll half a page"},
	{"pgup / pgdown", "scroll one page"},
	{", / .", "pan one column"},
	{"< / >", "pan ten columns"},
	{"[ / ]", "pan half the text width"},
	{"w", "toggle line wrapping"},
	{"c", "toggle colour scheme"},
	{"left / tab", "hide the file list"},
	{"right / shift+tab", "show the file list"},
	{"r", "reload the current file"},
	{"h / ?", "open or close this help"},
	{"q", "close the overlay / quit"},
	{"esc", "close the overlay"},
	{"ctrl+c", "exit immediately"},
}

// limitNotes is the Issue #34 scale, record-limit, and memory note —
// the single structured source for the statements both user-facing
// texts must carry. Each entry is rendered verbatim into the help
// overlay's footer and asserted verbatim against README.md by the
// Issue #34 documentation tests, so neither sink can drift from the
// other.
var limitNotes = []string{
	"Scale examples — independent, not simultaneous capacity guarantees: approximately 10,000 matched files, 100,000 matched lines, or individual files around 50 MB.",
	"The ~50 MB file example assumes UTF-8 content with ordinary line lengths; a line ripgrep must emit as base64 bytes expands by roughly a third, so a single match record can exceed the 64 MiB record limit — oversized records are skipped and reported, naming the file's path when recoverable.",
	"Loaded file buffers are retained for the whole session: no eviction, no aggregate memory bound, and no reliable OOM recovery — large searches or many visited files can exhaust memory, and forced termination cannot guarantee terminal cleanup.",
}

// helpFooter is the help overlay's footer slot, filled by Issue #34
// with the limitNotes scale-and-limits note. Entries are
// substitutions: helpLines routes each through the Issue #6
// diagnostic utility, so runtime text can never emit control bytes
// into the overlay.
var helpFooter = limitNotes

// helpLines composes the help overlay's text: the title, one row per
// helpBindings entry, and the footer slot's lines when it is filled.
// The binding table is static text; the substituted footer passes
// through present.Diagnostic.
func helpLines() []string {
	lines := []string{"Key bindings", ""}
	w := 0
	for _, b := range helpBindings {
		w = max(w, len(b.keys))
	}
	for _, b := range helpBindings {
		lines = append(lines, fmt.Sprintf("%-*s  %s", w, b.keys, b.desc))
	}
	if len(helpFooter) > 0 {
		lines = append(lines, "")
		for _, f := range helpFooter {
			lines = append(lines, strings.Split(present.Diagnostic(f), "\n")...)
		}
	}
	return lines
}

// openHelp opens the modal help overlay — the shared wrapped,
// scrollable overlay component over the binding table's rendered
// lines — and cancels any active pop-up: a dismissed help never
// restores one.
func (m *Model) openHelp() {
	m.help = &overlay{lines: helpLines()}
	m.popupID = 0
}

// helpKey handles one key while the help overlay is open: up/down
// scroll the rendered rows, q/Esc/h/? close to the underlying base
// state, ctrl+c takes the cancellation path, and every other key is
// ignored — nothing reaches the content behind the modal.
func (m Model) helpKey(key string) (Model, tea.Cmd) {
	switch key {
	case "ctrl+c":
		return m.cancelled(), tea.Quit
	case "q", "esc", "h", "?":
		m.help = nil
	case "up", "down":
		scrollOverlay(m.help, key, m.width, m.height)
	}
	return m, nil
}

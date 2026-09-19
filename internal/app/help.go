package app

import (
	"fmt"
	"strings"

	"vrg/internal/safepresentation"
)

// helpBinding is one row of the help dialog's binding table: the key
// or key set and what it does.
type helpBinding struct {
	keys string
	desc string
}

// helpBindings is the single binding-table data source (Issue #31):
// one row per binding — navigation, scrolling, panning, wrap, colour,
// list toggle, reload, help, and quit/cancel — that the help renderer
// consumes and Issue #34's documentation test iterates, so the README
// cannot drift from the keys the model actually routes.
var helpBindings = []helpBinding{
	{"n / p", "next / previous matched line"},
	{"up / down", "scroll one row"},
	{"u / d", "scroll half a page"},
	{"pgup / pgdn", "scroll a full page"},
	{", / .", "pan one column"},
	{"< / >", "pan ten columns"},
	{"[ / ]", "pan half the text width"},
	{"left / tab", "hide the file list"},
	{"right / shift+tab", "show the file list"},
	{"w", "toggle wrapping"},
	{"c", "toggle the colour scheme"},
	{"r", "reload the current file"},
	{"h / ?", "open or close this help"},
	{"q / esc", "dismiss an overlay or quit"},
	{"ctrl+c", "exit immediately"},
}

// helpFooter is the help dialog's footer slot — the text rendered
// under the binding table. Issue #34 fills it with the scale-and-limits
// note; until then it is empty and renders nothing. It is substituted
// text from the renderer's perspective, so the composition routes it
// through the diagnostic escaper like every external string.
var helpFooter string

// helpText composes the help dialog's body from the binding table: a
// title row, one "keys  description" row per binding with the key sets
// padded to a column, then the footer slot's escaped text.
func helpText() string {
	var b strings.Builder
	b.WriteString("Key bindings")
	w := 0
	for _, bind := range helpBindings {
		if n := safepresentation.CellWidth(bind.keys); n > w {
			w = n
		}
	}
	for _, bind := range helpBindings {
		fmt.Fprintf(&b, "\n%s  %s", padTo(bind.keys, w), bind.desc)
	}
	if helpFooter != "" {
		b.WriteString("\n\n" + safepresentation.EscapeDiagnostic(helpFooter))
	}
	return b.String()
}

// openHelp opens the modal help dialog over the current base state:
// the binding table rendered through the shared scrollBox component.
// Opening it cancels any file-change pop-up, which never returns after
// help closes (Issue #15's rule).
func (m *model) openHelp() {
	m.help.text = helpText()
	m.help.scroll = 0
	m.helpOpen = true
	m.popupID = 0
}

// helpRows is the help dialog's complete wrapped row set.
func (m *model) helpRows() []string {
	return m.help.rows(m.overlayInteriorWidth())
}

// scrollHelp moves the help dialog's first-visible-row index by d,
// clamped to the scrollable range of the complete wrapped table.
func (m *model) scrollHelp(d int) {
	m.help.scrollBy(d, m.overlayInteriorWidth(), m.overlayVisible())
}

// clampHelpScroll keeps the help dialog's first visible row inside the
// scrollable range — also after a resize changes the row count or
// visible height.
func (m *model) clampHelpScroll() {
	m.help.clamp(m.overlayInteriorWidth(), m.overlayVisible())
}

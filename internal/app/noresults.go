package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// renderNoResults composes the no-results screen: the message centred
// horizontally on the middle row of the frame, with "(N binary files
// skipped)" appended when binary exclusion emptied the list. Every row
// is padded to the frame edge so the base style covers the screen.
func (m Model) renderNoResults() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	text := "No results found"
	if m.binarySkipped > 0 {
		text = fmt.Sprintf("No results found (%d binary files skipped)", m.binarySkipped)
	}
	line := ansi.Truncate(text, m.width, "")
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

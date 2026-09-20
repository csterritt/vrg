package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/searchindex"
)

// applyLoad delivers a file-load message, then delivers the layout
// result its update scheduled — the two async hops every loaded panel
// goes through.
func applyLoad(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	m, cmd := update(t, m, msg)
	if cmd == nil {
		return m
	}
	m, _ = update(t, m, cmd())
	return m
}

// deliverCmd runs a returned command and feeds its produced message
// back through Update — the runtime's role for one completed job.
func deliverCmd(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	m, _ = update(t, m, cmd())
	return m
}

// A resize preserves the matched-line cursor: the stop selected before
// the terminal geometry changed is still the cursor after the prepared
// layout for the new parameters installs.
func TestResizePreservesCursorSelection(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(100))
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-05\n", 5, 0, 4, "line"))
	addRec(t, idx, matchRec("a.txt", "line-95\n", 95, 0, 4, "line"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())

	m, _ = update(t, m, keyMsg("n"))
	m, layout := update(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	if layout == nil {
		t.Fatal("a width-changing resize requested no layout preparation")
	}
	m, _ = update(t, m, layout())

	stop, _ := m.index.Current()
	if stop.Line != 95 {
		t.Fatalf("cursor after resize = line %d, want the selected stop 95", stop.Line)
	}
}

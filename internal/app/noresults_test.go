package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/searchindex"
)

// exitError is a fake child wait status reporting that rg exited with
// the given code, standing in for *exec.ExitError in outcome tests.
type exitError int

func (e exitError) Error() string { return fmt.Sprintf("exit status %d", int(e)) }
func (e exitError) ExitCode() int { return int(e) }

// endRec builds an end record with text blobs; a nil binaryOffset
// encodes JSON null.
func endRec(path string, binaryOffset any) string {
	off := "null"
	if binaryOffset != nil {
		off = fmt.Sprintf("%v", binaryOffset)
	}
	return fmt.Sprintf(`{"type":"end","data":{"path":{"text":%q},"binary_offset":%s}}`, path, off)
}

// An rg-1 search — a complete stream with no matches — leaves no usable
// results, so the model presents the centred no-results screen rather
// than the browse view, with no binary-skip suffix.
func TestEmptySearchShowsNoResultsScreen(t *testing.T) {
	idx := searchindex.New("/w")
	idx.Finish()

	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, "/w")
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, searchResult{index: idx, integrity: completeStream, err: exitError(1)})
	if cmd != nil {
		t.Fatalf("empty search completion returned a command: %v", cmd)
	}
	if m.state != stateNoResults {
		t.Fatalf("state = %d, want stateNoResults", m.state)
	}

	content := m.View().Content
	lines := strings.Split(content, "\n")
	if len(lines) != 24 {
		t.Fatalf("no-results screen has %d rows, want 24:\n%s", len(lines), content)
	}
	row := -1
	for i, l := range lines {
		if strings.Contains(l, "No results found") {
			row = i
		}
	}
	if row != 12 {
		t.Fatalf("message row = %d, want the centred row 12:\n%s", row, content)
	}
	if got := strings.TrimSpace(lines[row]); got != "No results found" {
		t.Fatalf("message = %q, want %q", got, "No results found")
	}
	if lead := len(lines[row]) - len(strings.TrimLeft(lines[row], " ")); lead != 32 {
		t.Fatalf("message not centred horizontally: %d leading cells, want (80-16)/2 = 32", lead)
	}
	if strings.Contains(content, "binary") {
		t.Fatalf("rg-1 emptiness carries a binary-skip suffix:\n%s", content)
	}
}

// q on the no-results screen quits through the ordinary cleanup path
// with exit status 1 — emptiness is success, not cancellation.
func TestNoResultsQuitExitsOne(t *testing.T) {
	idx := searchindex.New("/w")
	idx.Finish()

	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, "/w")
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = update(t, m, searchResult{index: idx, integrity: completeStream, err: exitError(1)})

	m, cmd := update(t, m, keyMsg("q"))
	if cmd == nil {
		t.Fatal("q on the no-results screen returned no command, want tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("q on the no-results screen returned %T, want tea.QuitMsg", cmd())
	}
	if m.status != 1 {
		t.Fatalf("exit status = %d, want 1", m.status)
	}
}

// An rg-0 stream whose every matched file was confirmed binary still
// lands on the no-results screen, now carrying the distinct exclusion
// count; q exits 1.
func TestAllBinarySearchShowsSkippedCount(t *testing.T) {
	idx := searchindex.New("/w")
	addRec(t, idx, `{"type":"begin","data":{"path":{"text":"a.bin"}}}`)
	addRec(t, idx, matchRec("a.bin", "x\n", 1, 0, 1, "x"))
	addRec(t, idx, endRec("a.bin", 12))
	addRec(t, idx, `{"type":"begin","data":{"path":{"text":"b.bin"}}}`)
	addRec(t, idx, matchRec("b.bin", "x\n", 1, 0, 1, "x"))
	addRec(t, idx, endRec("b.bin", 30))
	addRec(t, idx, `{"type":"summary","data":{}}`)
	idx.Finish()

	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, "/w")
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = update(t, m, searchResult{index: idx, integrity: completeStream}) // rg exit 0, all filtered

	content := m.View().Content
	if !strings.Contains(content, "No results found (2 binary files skipped)") {
		t.Fatalf("no-results screen lacks the exclusion count:\n%s", content)
	}
	m, cmd := update(t, m, keyMsg("q"))
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("q on the all-binary screen returned %T, want tea.QuitMsg", cmd())
	}
	if m.status != 1 {
		t.Fatalf("exit status = %d, want 1", m.status)
	}
}

// A stream mixing a binary-excluded file with a retained one browses:
// usable results are the retained stops, so the excluded file never
// reaches the file list.
func TestMixedBinaryRetentionBrowses(t *testing.T) {
	idx := searchindex.New("/w")
	addRec(t, idx, `{"type":"begin","data":{"path":{"text":"a.bin"}}}`)
	addRec(t, idx, matchRec("a.bin", "x\n", 1, 0, 1, "x"))
	addRec(t, idx, endRec("a.bin", 5))
	addRec(t, idx, `{"type":"begin","data":{"path":{"text":"b.txt"}}}`)
	addRec(t, idx, matchRec("b.txt", "y\n", 9, 0, 1, "y"))
	addRec(t, idx, endRec("b.txt", nil))
	addRec(t, idx, `{"type":"summary","data":{}}`)
	idx.Finish()

	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, "/w")
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, searchResult{index: idx, integrity: completeStream})
	if m.state != stateBrowse {
		t.Fatalf("state = %d, want stateBrowse for the retained file", m.state)
	}
	if cmd == nil {
		t.Fatal("browse entry started no load for the retained file")
	}
	content := m.View().Content
	if !strings.Contains(content, "b.txt") {
		t.Fatalf("browse view lacks the retained file:\n%s", content)
	}
	if strings.Contains(content, "a.bin") {
		t.Fatalf("browse view lists the binary-excluded file:\n%s", content)
	}
}

// Esc on the no-results screen is a base-state no-op: no command, no
// state change, the screen stays.
func TestNoResultsEscIsNoop(t *testing.T) {
	idx := searchindex.New("/w")
	idx.Finish()

	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, "/w")
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = update(t, m, searchResult{index: idx, integrity: completeStream, err: exitError(1)})

	m, cmd := update(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd != nil {
		t.Fatalf("Esc on the no-results screen returned a command: %v", cmd)
	}
	if m.state != stateNoResults {
		t.Fatalf("Esc changed state to %d, want stateNoResults", m.state)
	}
	if got := m.View().Content; !strings.Contains(got, "No results found") {
		t.Fatalf("Esc dismissed the no-results screen:\n%s", got)
	}
}

// ctrl+c on the no-results screen overrides the fixed status: it cancels
// through the shared path and exits 130, not 1.
func TestNoResultsCtrlCExits130(t *testing.T) {
	idx := searchindex.New("/w")
	idx.Finish()

	child := &fakeChild{
		stdout:     strings.NewReader(""),
		stderr:     strings.NewReader(""),
		terminated: make(chan struct{}),
	}
	m := New(child, "/w")
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = update(t, m, searchResult{index: idx, integrity: completeStream, err: exitError(1)})

	m, cmd := update(t, m, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("ctrl+c on the no-results screen returned no command, want tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("ctrl+c on the no-results screen returned %T, want tea.QuitMsg", cmd())
	}
	if m.status != 130 {
		t.Fatalf("exit status = %d, want 130", m.status)
	}
	requireClosed(t, child.terminated, "child termination")
}

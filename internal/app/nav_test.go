package app

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

var (
	keyN = tea.KeyPressMsg{Text: "n", Code: 'n'}
	keyP = tea.KeyPressMsg{Text: "p", Code: 'p'}
)

// navFile is a fixture file carrying several matched lines — the
// multi-stop generalization of fixtureFile.
type navFile struct {
	name    string
	content string
	stops   []navStop
}

// navStop is one matched line's 1-based number and submatch byte range.
type navStop struct {
	line       int64
	start, end int
}

// navIndex writes the fixture files into a fresh temp dir and builds the
// index with one match record per stop.
func navIndex(t *testing.T, files []navFile) *searchindex.Index {
	t.Helper()
	dir := t.TempDir()
	var stream strings.Builder
	b64 := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f.name), []byte(f.content), 0o644); err != nil {
			t.Fatal(err)
		}
		path := "./" + f.name
		fmt.Fprintf(&stream, `{"type":"begin","data":{"path":{"bytes":%q}}}`+"\n", b64(path))
		for _, s := range f.stops {
			line := nthLine(f.content, s.line)
			fmt.Fprintf(&stream, `{"type":"match","data":{"path":{"bytes":%q},"lines":{"bytes":%q},"line_number":%d,"absolute_offset":0,"submatches":[{"match":{"bytes":%q},"start":%d,"end":%d}]}}`+"\n",
				b64(path), b64(line), s.line, b64(line[s.start:s.end]), s.start, s.end)
		}
		fmt.Fprintf(&stream, `{"type":"end","data":{"path":{"bytes":%q},"binary_offset":null,"stats":{}}}`+"\n", b64(path))
	}
	stream.WriteString(`{"type":"summary","data":{"stats":{}}}` + "\n")
	return searchindex.Build([]byte(stream.String()), dir)
}

// navFiles is the standard navigation fixture: a.txt stops at lines 3
// and 8, b.txt a stop at line 2 — three stops across two files.
var navFiles = []navFile{
	{name: "a.txt", content: "l1\nl2\nxx aaa3 yy\nl4\nl5\nl6\nl7\nxx aaa8 yy\n", stops: []navStop{
		{line: 3, start: 3, end: 7},
		{line: 8, start: 3, end: 7},
	}},
	{name: "b.txt", content: "beta one\nzz bbb2 zz\n", stops: []navStop{
		{line: 2, start: 3, end: 7},
	}},
}

// wantCurrentMatch asserts the view renders text in the current matched
// line's style — the true inverse plus underline.
func wantCurrentMatch(t *testing.T, v, text string) {
	t.Helper()
	if !strings.Contains(v, "\x1b[30;47;4m"+text) {
		t.Fatalf("view = %q, want %q underlined-inverse (current matched line)", v, text)
	}
}

// wantPlainMatch asserts the view renders text as an ordinary match —
// inverse without underline — never the current-line style.
func wantPlainMatch(t *testing.T, v, text string) {
	t.Helper()
	if strings.Contains(v, "\x1b[30;47;4m"+text) {
		t.Fatalf("view = %q, %q must not be current-line styled", v, text)
	}
	if !strings.Contains(v, "\x1b[30;47m"+text) {
		t.Fatalf("view = %q, want %q in the plain inverse match style", v, text)
	}
}

// wantUnderlinedEntry asserts the file list underlines exactly the
// given file's entry.
func wantUnderlinedEntry(t *testing.T, m *model, f searchindex.File) {
	t.Helper()
	v := viewText(m)
	want := "\x1b[37;40;4m" + escapedPath(f) + "\x1b[37;40;24m"
	if !strings.Contains(v, want) {
		t.Fatalf("view = %q, want list entry underlined as %q", v, want)
	}
	// The current-line match style also carries the underline SGR, so
	// count the list-entry prefix specifically.
	if n := strings.Count(v, "\x1b[37;40;4m"); n != 1 {
		t.Fatalf("view = %q, want exactly one underlined list entry, got %d", v, n)
	}
}

// Startup selects the first stop in path order: the panel shows the
// first file, its list entry is underlined, and its first matched
// line's match carries the current-line underline while the later
// stop's does not.
func TestStartupCursorAtFirstStop(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, navFiles)
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)

	v := viewText(m)
	wantUnderlinedEntry(t, m, idx.Files[0])
	wantCurrentMatch(t, v, "aaa3")
	wantPlainMatch(t, v, "aaa8")
}

// n within one file moves only the current matched line: no command is
// issued, the list underline and filename rule stay on the same file,
// and the underline moves to the next stop's match.
func TestNAdvancesWithinFile(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, navFiles)
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)

	if _, cmd := m.Update(keyN); cmd != nil {
		t.Fatal("n within one file returned a command, want none")
	}
	v := viewText(m)
	wantCurrentMatch(t, v, "aaa8")
	wantPlainMatch(t, v, "aaa3")
	wantUnderlinedEntry(t, m, idx.Files[0])
}

// Crossing to another file's stop switches the panel immediately: the
// list underline moves, the filename rule names the new file, the
// uncached file's load is requested, and once it lands the panel starts
// from the top with the new stop's match current-line styled.
func TestNCrossingFileBoundarySwitchesPanel(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, navFiles)
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)

	m.Update(keyN) // a.txt line 8
	_, cmd = m.Update(keyN)
	if cmd == nil {
		t.Fatal("n into an uncached file issued no load command")
	}
	v := viewText(m)
	wantUnderlinedEntry(t, m, idx.Files[1])
	if !strings.Contains(v, "─ "+escapedPath(idx.Files[1])+" ") {
		t.Fatalf("view = %q, want the filename rule naming b.txt", v)
	}
	if !strings.Contains(v, "Loading…") {
		t.Fatalf("view = %q, want Loading… for the uncached file", v)
	}

	finishLoad(t, m, cmd)
	v = viewText(m)
	wantCurrentMatch(t, v, "bbb2")
	// First visit starts from the top of the file: line 1 heads the
	// panel content.
	if row := frameRow(t, m, 1); !strings.Contains(row, "beta one") {
		t.Fatalf("first content row = %q, want b.txt's first line", row)
	}
}

// n on the last stop wraps to the first; p on the first wraps to the
// last — circular in both directions across file boundaries.
func TestNavigationWrapsBothEnds(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, navFiles)
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)

	m.Update(keyN)                       // a.txt line 8
	finishLoad(t, m, navCmd(t, m, keyN)) // b.txt line 2

	// The last stop's n wraps to the first: a cached file issues no
	// command and the list underline returns to a.txt.
	if _, cmd := m.Update(keyN); cmd != nil {
		t.Fatal("wrap to a cached file returned a command, want none")
	}
	v := viewText(m)
	wantUnderlinedEntry(t, m, idx.Files[0])
	wantCurrentMatch(t, v, "aaa3")
	if row := frameRow(t, m, 1); !strings.Contains(row, "l1") {
		t.Fatalf("first content row = %q, want a.txt's first line", row)
	}

	// p on the first stop wraps back to the last — b.txt line 2.
	if _, cmd := m.Update(keyP); cmd != nil {
		t.Fatal("wrap-back to a cached file returned a command, want none")
	}
	v = viewText(m)
	wantUnderlinedEntry(t, m, idx.Files[1])
	wantCurrentMatch(t, v, "bbb2")
}

// navCmd sends a navigation key expected to return a load command —
// the destination file is uncached — and returns it.
func navCmd(t *testing.T, m *model, key tea.KeyPressMsg) tea.Cmd {
	t.Helper()
	_, cmd := m.Update(key)
	if cmd == nil {
		t.Fatal("expected a load command, got nil")
	}
	return cmd
}

// A one-stop index makes n and p strict no-ops: no command, no cursor
// movement, no frame change, no reload request.
func TestSingleStopIgnoresNavigation(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, []navFile{
		{name: "solo.txt", content: "one\ntwo hit\nthree\n", stops: []navStop{
			{line: 2, start: 4, end: 7},
		}},
	})
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)
	before := viewText(m)

	for _, k := range []tea.KeyPressMsg{keyN, keyP, keyN} {
		if _, cmd := m.Update(k); cmd != nil {
			t.Fatal("single-stop n/p returned a command, want a strict no-op")
		}
	}
	if got := viewText(m); got != before {
		t.Fatalf("view changed under single-stop n/p: %q → %q", before, got)
	}
	cur, ok := m.idx.Cursor()
	if !ok || cur.File != 0 || cur.Stop != 0 {
		t.Fatalf("cursor = %v ok=%v, want the only stop selected", cur, ok)
	}
}

// Manual scrolling never moves the matched-line cursor: after
// scrolling, n continues from the previously selected stop — the next
// stop in index order, not the stop nearest the viewport.
func TestManualScrollLeavesCursor(t *testing.T) {
	var content strings.Builder
	for i := 1; i <= 60; i++ {
		fmt.Fprintf(&content, "row %02d\n", i)
	}
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, []navFile{
		{name: "a.txt", content: content.String(), stops: []navStop{
			{line: 3, start: 0, end: 3},
			{line: 40, start: 0, end: 3},
		}},
	})
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)
	key := string(idx.Files[0].Path)

	for i := 0; i < 5; i++ {
		m.Update(keyDown)
	}
	if got := m.vps[key].Top(); got != 5 {
		t.Fatalf("top = %d after scrolling, want 5", got)
	}
	if cur, _ := m.idx.Cursor(); cur.Stop != 0 {
		t.Fatalf("scrolling moved the cursor to stop %d, want 0", cur.Stop)
	}

	if _, cmd := m.Update(keyN); cmd != nil {
		t.Fatal("same-file n returned a command, want none")
	}
	if cur, _ := m.idx.Cursor(); cur.Stop != 1 {
		t.Fatalf("n after scrolling selected stop %d, want 1 (line 40)", cur.Stop)
	}
	// Destination reveal is Issue #14's: the viewport stays where the
	// user scrolled.
	if got := m.vps[key].Top(); got != 5 {
		t.Fatalf("n changed the viewport top to %d, want the scrolled 5", got)
	}
}

// Departing a file saves its viewport: navigating away and back resumes
// the saved top rather than restarting from the top of the file.
func TestCrossFileRestoresDepartingViewport(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	idx := navIndex(t, []navFile{
		{name: "a.txt", content: numberedContent("a", 60), stops: []navStop{{line: 1, start: 0, end: 1}}},
		{name: "b.txt", content: numberedContent("b", 60), stops: []navStop{{line: 1, start: 0, end: 1}}},
	})
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)
	keyA := string(idx.Files[0].Path)

	for i := 0; i < 3; i++ {
		m.Update(keyDown)
	}
	finishLoad(t, m, navCmd(t, m, keyN))      // to b.txt
	if _, cmd := m.Update(keyP); cmd != nil { // back to a.txt
		t.Fatal("return to a cached file returned a command, want none")
	}
	if got := m.vps[keyA].Top(); got != 3 {
		t.Fatalf("revisited a.txt top = %d, want the saved 3", got)
	}
	if row := frameRow(t, m, 1); !strings.Contains(row, "a line 4") {
		t.Fatalf("first content row = %q, want line 4 at the saved top", row)
	}
}

// numberedContent is n lines "name line i".
func numberedContent(name string, n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "%s line %d\n", name, i)
	}
	return b.String()
}

// Navigation stays live while a destination file's load is in flight:
// the cursor keeps moving and an already-requested load is not
// reissued.
func TestNavigationWhileLoadInFlight(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, navFiles)
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)

	m.Update(keyN)         // a.txt line 8
	_ = navCmd(t, m, keyN) // b.txt's load stays in flight

	// n again wraps to a.txt's first stop — the cursor moved even
	// though b.txt never finished loading.
	if _, cmd := m.Update(keyN); cmd != nil {
		t.Fatal("wrap to a cached file returned a command, want none")
	}
	wantUnderlinedEntry(t, m, idx.Files[0])
	wantCurrentMatch(t, viewText(m), "aaa3")

	// p returns to b.txt's stop; its in-flight load is deduplicated —
	// no second command is issued.
	if _, cmd := m.Update(keyP); cmd != nil {
		t.Fatal("p to an in-flight load reissued it, want deduplication")
	}
	wantUnderlinedEntry(t, m, idx.Files[1])
}

// The file list is a passive overview: keys beyond n/p never move the
// cursor or change the current file — there is no direct selection
// route.
func TestFileListHasNoDirectSelection(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, navFiles)
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)

	passive := []tea.KeyPressMsg{
		{Code: tea.KeyEnter},
		{Code: tea.KeyTab},
		{Code: tea.KeyRight},
		{Code: tea.KeyLeft},
		{Text: " ", Code: ' '},
		{Text: "l", Code: 'l'},
		{Text: "e", Code: 'e'},
		{Text: "1", Code: '1'},
	}
	for _, k := range passive {
		if _, cmd := m.Update(k); cmd != nil {
			t.Fatalf("key %q returned a command, want none", k.Keystroke())
		}
	}
	cur, ok := m.idx.Cursor()
	if !ok || cur.File != 0 || cur.Stop != 0 {
		t.Fatalf("a passive-list key moved the cursor to %v ok=%v", cur, ok)
	}
	wantUnderlinedEntry(t, m, idx.Files[0])
}

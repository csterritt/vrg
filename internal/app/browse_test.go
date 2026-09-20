package app

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

// writeMatchFile creates a real file the file loader can read.
func writeMatchFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// matchRec builds a match record with text blobs.
func matchRec(path, lines string, line int, start, end int, match string) string {
	return fmt.Sprintf(`{"type":"match","data":{"path":{"text":%q},"lines":{"text":%q},`+
		`"line_number":%d,"submatches":[{"match":{"text":%q},"start":%d,"end":%d}]}}`,
		path, lines, line, match, start, end)
}

// matchRecBytes builds a match record with base64 bytes blobs, the only
// form that can carry invalid-UTF-8 paths and content.
func matchRecBytes(path, lines []byte, line int, start, end int, match []byte) string {
	b64 := base64.StdEncoding.EncodeToString
	return fmt.Sprintf(`{"type":"match","data":{"path":{"bytes":%q},"lines":{"bytes":%q},`+
		`"line_number":%d,"submatches":[{"match":{"bytes":%q},"start":%d,"end":%d}]}}`,
		b64(path), b64(lines), line, b64(match), start, end)
}

// addRec feeds one record into the index, failing on rejection.
func addRec(t *testing.T, idx *searchindex.Index, rec string) {
	t.Helper()
	if err := idx.Add([]byte(rec)); err != nil {
		t.Fatalf("Add: %v\n%s", err, rec)
	}
}

// update applies one message and returns the resulting model.
func update(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	mi, cmd := m.Update(msg)
	return mi.(Model), cmd
}

// A completed search presents the two-pane browse view: every retained
// file in raw-path order on the left, "Loading…" and the filename rule
// on the right until the current file's load completes.
func TestBrowseShowsListAndLoading(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", "hit a\n")
	writeMatchFile(t, dir, "b.txt", "hit b\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("b.txt", "hit b\n", 1, 0, 3, "hit"))
	idx.Finish()

	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, dir)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, searchResult{index: idx, integrity: completeStream})
	if cmd == nil {
		t.Fatal("search completion started no file load")
	}
	v := m.View().Content
	ai, bi := strings.Index(v, "a.txt"), strings.Index(v, "b.txt")
	if ai < 0 || bi < 0 || ai > bi {
		t.Fatalf("file list order wrong in view:\n%s", v)
	}
	if !strings.Contains(v, "Loading…") {
		t.Fatalf("browse view lacks the loading placeholder:\n%s", v)
	}
	if strings.Contains(v, "Searching…") {
		t.Fatalf("browse view still shows the searching screen:\n%s", v)
	}
}

// The load command reads, decodes, and maps the file off the update
// path; delivering its message replaces "Loading…" with guttered content
// and inverse-video matches.
func TestBrowseContentAfterLoad(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", "hit a\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	idx.Finish()

	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, dir)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, searchResult{index: idx, integrity: completeStream})
	m = applyLoad(t, m, cmd())

	v := m.View().Content
	if strings.Contains(v, "Loading…") {
		t.Fatalf("loaded view still shows the placeholder:\n%s", v)
	}
	if !strings.Contains(v, "1  \x1b[37;40m\x1b[4m\x1b[30;47mhit\x1b[24m\x1b[37;40m a") {
		t.Fatalf("loaded view lacks guttered inverse match:\n%q", v)
	}
}

// The filename row embeds the current file's escaped path in a
// horizontal rule, and content rows carry a right-justified gutter
// followed by two spaces.
func TestFilenameRuleAndGutter(t *testing.T) {
	dir := t.TempDir()
	var content strings.Builder
	for i := 1; i <= 12; i++ {
		fmt.Fprintf(&content, "line-%02d\n", i)
	}
	writeMatchFile(t, dir, "a.txt", content.String())
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-01\n", 1, 0, 4, "line"))
	idx.Finish()

	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, dir)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, searchResult{index: idx, integrity: completeStream})
	m = applyLoad(t, m, cmd())

	v := m.View().Content
	first := strings.SplitN(v, "\n", 2)[0]
	if !strings.Contains(first, "── a.txt ") || !strings.Contains(first, "─") {
		t.Fatalf("filename rule missing or malformed in first row: %q", first)
	}
	// Twelve lines need two digit slots; the gutter is right-justified
	// with two trailing spaces.
	if !strings.Contains(v, " 1  ") || !strings.Contains(v, "12  \x1b[37;40mline-12") {
		t.Fatalf("gutter format wrong:\n%s", v)
	}
}

// The current file-list entry is underlined; the others are not.
func TestFileListCurrentUnderlined(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", "hit a\n")
	writeMatchFile(t, dir, "b.txt", "hit b\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("b.txt", "hit b\n", 1, 0, 3, "hit"))
	idx.Finish()

	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, dir)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = update(t, m, searchResult{index: idx, integrity: completeStream})

	v := m.View().Content
	if !strings.Contains(v, "\x1b[4ma.txt\x1b[24m") {
		t.Fatalf("current entry not underlined:\n%q", v)
	}
	if strings.Contains(v, "\x1b[4mb.txt") {
		t.Fatalf("non-current entry underlined:\n%q", v)
	}
}

// While a load — read and decode/map — is held by the worker gate, key
// and resize messages are handled, and ctrl+c exits 130 through the
// cancellation path without waiting on the gate.
func TestGatedLoadKeepsInputResponsive(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", "hit a\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	idx.Finish()

	child := &fakeChild{
		stdout:     strings.NewReader(""),
		stderr:     strings.NewReader(""),
		terminated: make(chan struct{}),
	}
	m := New(child, dir)
	m.loadGate = make(chan struct{})
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, searchResult{index: idx, integrity: completeStream})
	msgs := make(chan tea.Msg, 1)
	go func() { msgs <- cmd() }()

	select {
	case <-msgs:
		t.Fatal("load delivered while the worker gate was held")
	default:
	}

	m, c := update(t, m, tea.WindowSizeMsg{Width: 100, Height: 40})
	if c != nil || m.width != 100 || m.height != 40 {
		t.Fatalf("resize during gated load: cmd %v, size %dx%d", c, m.width, m.height)
	}
	m, c = update(t, m, keyMsg("x"))
	if c != nil {
		t.Fatalf("key during gated load returned a command: %v", c)
	}
	if got := m.View().Content; !strings.Contains(got, "Loading…") {
		t.Fatalf("gated load lost the placeholder:\n%s", got)
	}

	m, quit := update(t, m, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if quit == nil {
		t.Fatal("ctrl+c during gated load returned no command, want tea.Quit")
	}
	if _, ok := quit().(tea.QuitMsg); !ok {
		t.Fatalf("ctrl+c during gated load returned %T, want tea.QuitMsg", quit())
	}
	if m.status != 130 {
		t.Fatalf("exit status = %d, want 130", m.status)
	}
	requireClosed(t, child.terminated, "child termination")

	// Cancellation released the gate-held worker; its late result must
	// not revive the UI.
	var late tea.Msg
	select {
	case late = <-msgs:
	case <-time.After(10 * time.Second):
		t.Fatal("gate-held load did not return promptly after cancellation")
	}
	m, c = update(t, m, late)
	if c != nil {
		t.Fatalf("late load result after cancellation returned a command: %v", c)
	}
	if m.state != stateCancelled || m.status != 130 {
		t.Fatalf("late load result revived the UI: state %d, status %d", m.state, m.status)
	}
}

// c toggles the composed view between the schemes: the dark scheme —
// white on black — is active at startup and cannot change while
// searching, and each c in the browse view flips the styling between
// white-on-black and black-on-white with no persistence. Matches stay
// the true inverse of the active base, the current matched line's match
// stays underlined, and the current file-list entry stays underlined in
// both schemes.
func TestCTogglesColourScheme(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", "hit a\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	idx.Finish()

	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, dir)
	m, c := update(t, m, keyMsg("c"))
	if c != nil {
		t.Fatalf("c while searching returned a command: %v", c)
	}
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, searchResult{index: idx, integrity: completeStream})
	m = applyLoad(t, m, cmd())

	dark := m.View().Content
	if !strings.HasPrefix(dark, "\x1b[37;40m") {
		t.Fatalf("initial browse view is not white on black:\n%q", dark)
	}
	if !strings.Contains(dark, "\x1b[4m\x1b[30;47mhit\x1b[24m") {
		t.Fatalf("dark view lacks the underlined inverse current match:\n%q", dark)
	}
	if !strings.Contains(dark, "\x1b[4ma.txt\x1b[24m") {
		t.Fatalf("dark view lacks the underlined current file:\n%q", dark)
	}

	m, c = update(t, m, keyMsg("c"))
	if c != nil {
		t.Fatalf("c in the browse view returned a command: %v", c)
	}
	light := m.View().Content
	if !strings.HasPrefix(light, "\x1b[30;47m") {
		t.Fatalf("view after c is not black on white:\n%q", light)
	}
	if !strings.Contains(light, "\x1b[4m\x1b[37;40mhit\x1b[24m") {
		t.Fatalf("light view lacks the underlined inverse current match:\n%q", light)
	}
	if !strings.Contains(light, "\x1b[4ma.txt\x1b[24m") {
		t.Fatalf("light view lacks the underlined current file:\n%q", light)
	}

	m, c = update(t, m, keyMsg("c"))
	if c != nil {
		t.Fatalf("second c in the browse view returned a command: %v", c)
	}
	if got := m.View().Content; !strings.HasPrefix(got, "\x1b[37;40m") {
		t.Fatalf("view after the second c did not return to white on black:\n%q", got)
	}
}

// q in the browse view quits with the search-derived exit status 0.
func TestBrowseQExitsZero(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", "hit a\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	idx.Finish()

	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, dir)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = update(t, m, searchResult{index: idx, integrity: completeStream})
	m, cmd := update(t, m, keyMsg("q"))
	if cmd == nil {
		t.Fatal("q in the browse view returned no command, want tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("q in the browse view returned %T, want tea.QuitMsg", cmd())
	}
	if m.status != 0 {
		t.Fatalf("exit status = %d, want 0", m.status)
	}
}

// The hostile fixture set — OSC, CSI, C0, C1, DEL, a standalone CR,
// invalid UTF-8 path bytes, and an embedded filename newline — driven
// through the real composition path via the no-style theme must leave no
// fixture control byte verbatim in the raw output of any browse sink
// (file-list entry, filename rule, panel content).
func TestBrowseSinksNeverEmitFixtureControlBytes(t *testing.T) {
	dir := t.TempDir()
	hostileName := []byte("a\x1bv\x07i\n\xffl.txt")
	content := []byte("ok \x1b]0;pwned\x07 csi \x1b[2J nel \xc2\x85 del \x7f cr \r tab \t bad \xff\n")
	if err := os.WriteFile(filepath.Join(dir, string(hostileName)), content, 0o644); err != nil {
		t.Fatal(err)
	}
	idx := searchindex.New(dir)
	addRec(t, idx, matchRecBytes(hostileName, content, 1, 0, 2, []byte("ok")))
	idx.Finish()

	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, dir)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m, cmd := update(t, m, searchResult{index: idx, integrity: completeStream})
	m = applyLoad(t, m, cmd())
	m.theme = theme.Plain() // no-style composition: no escape byte may legitimately appear

	raw := m.View().Content
	for _, b := range []byte{0x1b, 0x07, 0x08, 0x0d, 0x7f, 0x9b, 0xff} {
		if strings.IndexByte(raw, b) >= 0 {
			t.Fatalf("raw output contains fixture control byte %#02x:\n%q", b, raw)
		}
	}
	if strings.Contains(raw, "\xc2\x85") {
		t.Fatalf("raw output contains a verbatim C1 NEL:\n%q", raw)
	}
	for _, want := range []string{
		`a^[v^Gi\n\xffl.txt`, // escaped path in list and filename rule
		"^[]0;pwned^G",       // OSC content as caret forms
		"^[[2J",              // CSI content
		`nel \u0085`,         // C1 content
		"del ^?",             // DEL content
		"cr ^M",              // standalone CR content
		"tab    bad",         // tab expanded to its structural stop
		"bad \uFFFD",         // invalid UTF-8 content
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("raw output lacks escaped form %q:\n%s", want, raw)
		}
	}
}

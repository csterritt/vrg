package app

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

// fixtureFile is one real file on disk plus the match record the fake
// stream claims for it.
type fixtureFile struct {
	name    string // relative name under the fixture dir
	content string
	line    int64 // matched source line number (1-based)
	start   int   // submatch byte range within the matched line
	end     int
}

// browseIndex writes the fixture files into a fresh temp dir and builds
// the search index from a synthetic record stream claiming each file's
// match. Record payloads use the base64 "bytes" form so fixture paths
// and lines may carry arbitrary bytes.
func browseIndex(t *testing.T, files []fixtureFile) *searchindex.Index {
	t.Helper()
	dir := t.TempDir()
	var stream strings.Builder
	b64 := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f.name), []byte(f.content), 0o644); err != nil {
			t.Fatal(err)
		}
		path := "./" + f.name
		line := nthLine(f.content, f.line)
		fmt.Fprintf(&stream, `{"type":"begin","data":{"path":{"bytes":%q}}}`+"\n", b64(path))
		fmt.Fprintf(&stream, `{"type":"match","data":{"path":{"bytes":%q},"lines":{"bytes":%q},"line_number":%d,"absolute_offset":0,"submatches":[{"match":{"bytes":%q},"start":%d,"end":%d}]}}`+"\n",
			b64(path), b64(line), f.line, b64(line[f.start:f.end]), f.start, f.end)
		fmt.Fprintf(&stream, `{"type":"end","data":{"path":{"bytes":%q},"binary_offset":null,"stats":{}}}`+"\n", b64(path))
	}
	stream.WriteString(`{"type":"summary","data":{"stats":{}}}` + "\n")
	return searchindex.Build([]byte(stream.String()), dir)
}

// nthLine returns content's 1-based nth line including any terminator.
func nthLine(content string, n int64) string {
	lines := strings.SplitAfter(content, "\n")
	return lines[n-1]
}

// startBrowse sends a model through the search-done transition and
// returns it with the load command the transition issued for the
// current file.
func startBrowse(t *testing.T, m *model, idx *searchindex.Index) tea.Cmd {
	t.Helper()
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	_, cmd := m.Update(searchDoneMsg{res: Result{Code: 0}, idx: idx})
	if cmd == nil {
		t.Fatal("browse transition issued no load command")
	}
	return cmd
}

// finishLoad runs the load command and delivers its completion message.
// A file-crossing navigation batches the pop-up's expiry command after
// the load, so batches are unwrapped leaf by leaf until the load lands.
func finishLoad(t *testing.T, m *model, cmd tea.Cmd) {
	t.Helper()
	if msg, ok := fileLoadOf(cmd); ok {
		m.Update(msg)
		return
	}
	t.Fatalf("load command produced no fileLoadedMsg")
}

// fileLoadOf runs cmd — descending into batches — and returns the first
// fileLoadedMsg a leaf produced. Navigation batches the load leaf
// first, so a real pop-up timer leaf is never run.
func fileLoadOf(cmd tea.Cmd) (fileLoadedMsg, bool) {
	if cmd == nil {
		return fileLoadedMsg{}, false
	}
	switch msg := cmd().(type) {
	case fileLoadedMsg:
		return msg, true
	case tea.BatchMsg:
		for _, leaf := range msg {
			if m, ok := fileLoadOf(leaf); ok {
				return m, true
			}
		}
	}
	return fileLoadedMsg{}, false
}

// escapedPath recomputes a file's escaped display path through the real
// safe-presentation core.
func escapedPath(f searchindex.File) string {
	return safepresentation.EscapePath(f.Path)
}

var browseFiles = []fixtureFile{
	{name: "a.go", content: "package a\nfunc alpha() {}\nvar alpha2 int\n", line: 2, start: 5, end: 10},
	{name: "b.go", content: "beta alpha\n", line: 1, start: 5, end: 10},
}

// A completed search presents the two-pane browse view: every retained
// file in raw-path order on the left and the file panel showing
// Loading… until the current file's buffer arrives.
func TestSearchDonePresentsBrowseWithLoading(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	cmd := startBrowse(t, m, browseIndex(t, browseFiles))
	if cmd == nil {
		t.Fatal("no load command")
	}
	if m.state != stateBrowse {
		t.Fatalf("state = %v, want browse", m.state)
	}
	v := viewText(m)
	ia, ib := strings.Index(v, "a.go"), strings.Index(v, "b.go")
	if ia < 0 || ib < 0 || ia > ib {
		t.Fatalf("file list out of order: a.go at %d, b.go at %d in %q", ia, ib, v)
	}
	if !strings.Contains(v, "Loading…") {
		t.Fatalf("view = %q, want the Loading… placeholder while the load is pending", v)
	}
}

// The load completion message carries the prepared buffer; once it
// lands, the panel shows the filename rule, the right-justified gutter,
// content, and matches in inverse video — Update did no full-file work.
func TestLoadCompletionRendersContent(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := browseIndex(t, browseFiles)
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)

	v := viewText(m)
	if strings.Contains(v, "Loading…") {
		t.Fatalf("view = %q, Loading… should be gone after the load completes", v)
	}
	if !strings.Contains(v, "func ") || !strings.Contains(v, "package a") {
		t.Fatalf("view = %q, want the loaded content rows", v)
	}
	// Line 2 is the first stop — the current matched line — so its
	// match renders in the true inverse plus underline.
	if !strings.Contains(v, "\x1b[30;47;4malpha\x1b[37;40;24m") {
		t.Fatalf("view = %q, want the match in inverse video and underlined", v)
	}
	// The filename rule embeds the escaped current path in a horizontal
	// rule above the content.
	want := "─ " + escapedPath(idx.Files[0]) + " "
	if !strings.Contains(v, want) {
		t.Fatalf("view = %q, want filename rule containing %q", v, want)
	}
	// Right-justified gutter followed by two spaces: line 2 shows as
	// "2  " ahead of its text.
	if !strings.Contains(v, "\x1b[37;40m2  \x1b[37;40;24mfunc \x1b[30;47;4malpha") {
		t.Fatalf("view = %q, want gutter %q then text before the highlighted match", v, "2  func ")
	}
}

// The current file's list entry is underlined; other entries are not.
func TestCurrentFileListEntryUnderlined(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := browseIndex(t, browseFiles)
	startBrowse(t, m, idx)

	v := viewText(m)
	underlined := "\x1b[37;40;4m" + escapedPath(idx.Files[0]) + "\x1b[37;40;24m"
	if !strings.Contains(v, underlined) {
		t.Fatalf("view = %q, want the current entry underlined as %q", v, underlined)
	}
	if n := strings.Count(v, ";4m"); n != 1 {
		t.Fatalf("view = %q, want exactly one underlined list entry, got %d", v, n)
	}
}

// A right-justified gutter: single-digit line numbers pad left to the
// digit width of the file's largest line number.
func TestGutterRightJustified(t *testing.T) {
	var content strings.Builder
	for i := 0; i < 10; i++ {
		fmt.Fprintf(&content, "line %d\n", i+1)
	}
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := browseIndex(t, []fixtureFile{
		{name: "a.txt", content: content.String(), line: 10, start: 5, end: 7},
	})
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)

	v := viewText(m)
	if !strings.Contains(v, "\x1b[37;40m 9  \x1b[37;40;24mline 9") {
		t.Fatalf("view = %q, want %q (padded to the 2-digit gutter)", v, " 9  line 9")
	}
	// Line 10 is the file's only stop — the current matched line — so
	// its match is inverse and underlined.
	if !strings.Contains(v, "\x1b[37;40m10  \x1b[37;40;24mline \x1b[30;47;4m10\x1b[37;40;24m") {
		t.Fatalf("view = %q, want %q", v, "10  line 10 with the match inverse and underlined")
	}
}

// While a worker gate holds the load — read and decode/map together —
// the model still answers key and resize messages, and the placeholder
// stays up until the released completion lands.
func TestGatedLoadStaysResponsive(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	opts := options{loadGate: func() {
		close(entered)
		<-release
	}}
	m := newTestModel(fakeChild{res: Result{Code: 0}}, opts)
	cmd := startBrowse(t, m, browseIndex(t, browseFiles))

	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	<-entered // the worker holds the whole load

	if v := viewText(m); !strings.Contains(v, "Loading…") {
		t.Fatalf("view = %q, want Loading… while the load is held", v)
	}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if m.width != 100 || m.height != 30 {
		t.Fatalf("size = %dx%d, want 100x30 handled while the load is held", m.width, m.height)
	}
	if _, cmd := m.Update(tea.KeyPressMsg{Text: "x", Code: 'x'}); cmd != nil {
		t.Fatal("an ignored key during a held load returned a command")
	}
	select {
	case msg := <-done:
		t.Fatalf("completion arrived while the gate was held: %#v", msg)
	default:
	}

	close(release)
	m.Update(<-done)
	if v := viewText(m); strings.Contains(v, "Loading…") || !strings.Contains(v, "func") {
		t.Fatalf("view = %q after release, want content without Loading…", v)
	}
}

// ctrl+c while the load is gate-held exits 130 through the Issue #4
// cleanup path; the released completion can never revive the UI.
func TestCtrlCDuringHeldLoadExits130(t *testing.T) {
	reaped := make(chan Result, 1)
	entered := make(chan struct{})
	release := make(chan struct{})
	opts := options{
		reap: func(r Result) { reaped <- r },
		loadGate: func() {
			close(entered)
			<-release
		},
	}
	m := newTestModel(newKillChild(Result{Code: -1, Err: errors.New("signal: killed")}), opts)
	load := startBrowse(t, m, browseIndex(t, browseFiles))

	done := make(chan tea.Msg, 1)
	go func() { done <- load() }()
	<-entered

	_, cmd := m.Update(keyCtrlC)
	if !m.quitting {
		t.Fatal("ctrl+c during a held load did not begin a controlled exit")
	}
	runQuittingCmd(t, cmd)
	if m.status != 130 {
		t.Fatalf("status = %d, want 130", m.status)
	}
	select {
	case <-reaped:
	default:
		t.Fatal("wait/reap path did not run before quit")
	}

	close(release)
	m.Update(<-done)
	if v := viewText(m); strings.Contains(v, "func") {
		t.Fatalf("view = %q after cancellation, want no content", v)
	}
}

// q in the browse state — loading or loaded — exits 0 through the
// Issue #4 cleanup path.
func TestQOnBrowseExitsZero(t *testing.T) {
	for _, loaded := range []bool{false, true} {
		m := newTestModel(newKillChild(Result{Code: 0}), options{})
		cmd := startBrowse(t, m, browseIndex(t, browseFiles))
		if loaded {
			finishLoad(t, m, cmd)
		}
		_, cmd = m.Update(keyQ)
		if !m.quitting {
			t.Fatalf("q on browse (loaded=%v) did not begin a controlled exit", loaded)
		}
		runQuittingCmd(t, cmd)
		if m.status != 0 {
			t.Fatalf("status = %d, want 0", m.status)
		}
	}
}

// c toggles the composed view's styling between the schemes: the
// session opens dark (white on black), c flips to light (black on
// white) — matches stay inverse and the current matched line's match
// stays underlined, now in the other scheme's colours — and a second c
// returns to the dark frame. Nothing persists beyond the model's theme.
func TestCTogglesColourScheme(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	cmd := startBrowse(t, m, browseIndex(t, browseFiles))
	finishLoad(t, m, cmd)

	dark := viewText(m)
	if !strings.HasPrefix(dark, "\x1b[37;40m") {
		t.Fatalf("initial view = %q, want the dark scheme's white-on-black base", dark)
	}
	if !strings.Contains(dark, "\x1b[30;47;4malpha") {
		t.Fatalf("dark view = %q, want the current-line match in inverse black-on-white plus underline", dark)
	}

	m.Update(keyC)
	light := viewText(m)
	if !strings.HasPrefix(light, "\x1b[30;47m") {
		t.Fatalf("view after c = %q, want the light scheme's black-on-white base", light)
	}
	if !strings.Contains(light, "\x1b[37;40;4malpha") {
		t.Fatalf("light view = %q, want the current-line match in inverse white-on-black plus underline", light)
	}

	m.Update(keyC)
	if got := viewText(m); got != dark {
		t.Fatalf("view after second c = %q, want the dark frame restored", got)
	}
}

// A resize recomposes the browse view at the new dimensions while the
// selection and the loaded content are preserved.
func TestResizeRecomposesBrowse(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	cmd := startBrowse(t, m, browseIndex(t, browseFiles))
	finishLoad(t, m, cmd)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if got := strings.Count(viewText(m), "\n") + 1; got != 30 {
		t.Fatalf("view rows = %d, want 30 after resize", got)
	}
	if v := viewText(m); !strings.Contains(v, "func") {
		t.Fatalf("view = %q, want content preserved through resize", v)
	}
}

// The hostile fixture — OSC, CSI, C0, C1, DEL, a standalone CR, invalid
// UTF-8 path and content bytes, and an embedded filename newline —
// driven through the real composition path on the no-style theme. With
// no decorator emitting bytes, the raw output (before any ANSI
// stripping) may carry no control byte at all: anything present can only
// be a fixture byte that survived verbatim.
func TestHostileFixtureRawOutput(t *testing.T) {
	// bad\nname<ESC>\xff.txt — the newline and invalid byte are raw
	// path bytes; the file is still opened by its original bytes.
	name := "bad\nname\x1b\xff.txt"
	// One matched line carrying the whole fixture set: the OSC and CSI
	// sequences, C0 controls, a C1 control, DEL, a standalone CR, and an
	// invalid UTF-8 byte. The match covers the OSC sequence so its
	// escaped cells all fall inside one highlight.
	content := "ok \x1b]0;pwned\x07 mid \x1b[2J c1\xc2\x85 d\x7f cr\rdel \x00\xff\n"
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain() // the no-style composition path
	idx := browseIndex(t, []fixtureFile{
		{name: name, content: content, line: 1, start: 3, end: 13},
	})
	cmd := startBrowse(t, m, idx)
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 50})
	finishLoad(t, m, cmd)

	v := viewText(m)
	if !utf8.ValidString(v) {
		t.Fatalf("raw view contains invalid UTF-8: %q", v)
	}
	for _, r := range v {
		if r == '\n' {
			continue // row separators are vrg's own framing, not data
		}
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0) {
			t.Fatalf("raw view contains a verbatim control rune %#U: %q", r, v)
		}
	}
	// The embedded filename newline cannot have survived: the frame is
	// still exactly height rows.
	if got := strings.Count(v, "\n") + 1; got != 50 {
		t.Fatalf("view rows = %d, want 50 — a fixture newline would add rows", got)
	}
	// Every escaped form appears, in both path sinks and the content
	// sink. The escaped name shows at least twice: file list and rule.
	escName := `bad\nname^[\xff.txt`
	if n := strings.Count(v, escName); n < 2 {
		t.Fatalf("escaped name %q appears %d times, want list + rule: %q", escName, n, v)
	}
	for _, want := range []string{"^[]0;pwned^G", "^[[2J", `\u0085`, "^?", "^M", "^@", string(rune(0xfffd))} {
		if !strings.Contains(v, want) {
			t.Fatalf("view = %q, want the escaped form %q", v, want)
		}
	}
}

// A load completion for a path that is not current never replaces the
// visible panel.
func TestLateLoadForOtherFileIgnored(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := browseIndex(t, browseFiles)
	cmd := startBrowse(t, m, idx)
	buf, err := filebuffer.Load(idx.Files[1].Path, idx.Files[1].Stops)
	if err != nil {
		t.Fatal(err)
	}
	m.Update(fileLoadedMsg{path: idx.Files[1].Path, buf: buf})
	if v := viewText(m); !strings.Contains(v, "Loading…") {
		t.Fatalf("view = %q, want Loading… — a foreign completion must not settle the current file", v)
	}
	finishLoad(t, m, cmd)
	if v := viewText(m); strings.Contains(v, "beta") {
		t.Fatalf("view = %q, want the current file's content, not the foreign buffer's", v)
	}
}

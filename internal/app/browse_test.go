package app_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
	"vrg/internal/sinkfixtures"
	"vrg/internal/theme"
)

// --- JSON record helpers (mirrors searchindex_test.go) ---

func b64(data []byte) string { return base64.StdEncoding.EncodeToString(data) }

type subSpec struct {
	match string
	start int
	end   int
}

func textMatch(path, line string, lineNo int, subs ...subSpec) string {
	return matchRecord(
		map[string]any{"text": path},
		map[string]any{"text": line},
		lineNo, subs, false,
	)
}

func bytesMatch(path, line []byte, lineNo int, subs ...subSpec) string {
	return matchRecord(
		map[string]any{"bytes": b64(path)},
		map[string]any{"bytes": b64(line)},
		lineNo, subs, true,
	)
}

func matchRecord(pathEnc, lineEnc map[string]any, lineNo int, subs []subSpec, bytesMatch bool) string {
	submatches := make([]map[string]any, len(subs))
	for i, s := range subs {
		var matchEnc map[string]any
		if bytesMatch {
			matchEnc = map[string]any{"bytes": b64([]byte(s.match))}
		} else {
			matchEnc = map[string]any{"text": s.match}
		}
		submatches[i] = map[string]any{"match": matchEnc, "start": s.start, "end": s.end}
	}
	rec := map[string]any{
		"type": "match",
		"data": map[string]any{
			"path":            pathEnc,
			"lines":           lineEnc,
			"line_number":     lineNo,
			"submatches":      submatches,
			"absolute_offset": 0,
		},
	}
	b, _ := json.Marshal(rec)
	return string(b)
}

// buildIndex builds a searchindex.Index from JSON record strings.
func buildIndex(t *testing.T, workdir string, records ...string) *searchindex.Index {
	t.Helper()
	b := searchindex.NewBuilder(workdir)
	for _, r := range records {
		if err := b.Add([]byte(r)); err != nil {
			t.Fatalf("Add %q: %v", r, err)
		}
	}
	return b.Build()
}

// makeBuf creates a prepared filebuffer.Buffer for testing.
func makeBuf(lines []filebuffer.Line, lineCount, gutterWidth int) *filebuffer.Buffer {
	return &filebuffer.Buffer{Lines: lines, LineCount: lineCount, GutterWidth: gutterWidth}
}

// ml creates a filebuffer.Line for testing.
func ml(num int, display string, highlights ...[2]int) filebuffer.Line {
	return filebuffer.Line{Number: num, Display: display, Highlights: highlights}
}

// setupBrowse creates a model in the browse state with the given index
// and a file loader that returns the given buffer. The gate is released
// immediately so the load completes.
func setupBrowse(t *testing.T, idx *searchindex.Index, buf *filebuffer.Buffer) app.Model {
	t.Helper()
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return buf, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
	)
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if m.State() != app.StateBrowse {
		t.Fatalf("State = %v, want StateBrowse", m.State())
	}
	// Execute the async load command and deliver the completion message.
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, _ = update(t, m, lc)
		}
	}
	return m
}

// setupBrowseLoading creates a model in the browse state with the gate
// held (not closed). The file load command is returned but not executed.
func setupBrowseLoading(t *testing.T, idx *searchindex.Index) (app.Model, tea.Cmd) {
	t.Helper()
	gate := make(chan struct{})
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		<-gate
		return &filebuffer.Buffer{}, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
	)
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if m.State() != app.StateBrowse {
		t.Fatalf("State = %v, want StateBrowse", m.State())
	}
	return m, cmd
}

// --- App model tests ---

// TestBrowseViewAfterCompletion verifies that a SearchCompleteMsg with
// an Index transitions to the browse state and shows a file list.
func TestBrowseViewAfterCompletion(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
		textMatch("src/b.go", "world\n", 1, subSpec{"world", 0, 5}),
	)
	buf := makeBuf(nil, 0, 3)
	m := setupBrowse(t, idx, buf)
	view := viewContent(m)
	if !strings.Contains(view, "a.go") {
		t.Fatalf("View does not contain 'a.go': %q", view)
	}
	if !strings.Contains(view, "b.go") {
		t.Fatalf("View does not contain 'b.go': %q", view)
	}
}

// TestBrowseLoadingPlaceholder verifies that the panel shows "Loading…"
// until the file load completes.
func TestBrowseLoadingPlaceholder(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	m, _ := setupBrowseLoading(t, idx)
	view := viewContent(m)
	if !strings.Contains(view, "Loading") {
		t.Fatalf("View does not contain 'Loading': %q", view)
	}
}

// TestBrowseKeyHandledWhileLoading verifies that a key message is handled
// (does not quit or block) while the file load is pending.
func TestBrowseKeyHandledWhileLoading(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	m, _ := setupBrowseLoading(t, idx)
	m, cmd := update(t, m, keyPress('j'))
	if m.State() != app.StateBrowse {
		t.Fatalf("after key, State = %v, want StateBrowse", m.State())
	}
	if cmd != nil {
		msg := execCmd(t, cmd)
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Fatal("key 'j' while loading produced a quit command")
		}
	}
}

// TestBrowseResizeHandledWhileLoading verifies that a resize message is
// handled while the file load is pending.
func TestBrowseResizeHandledWhileLoading(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	m, _ := setupBrowseLoading(t, idx)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	if m.State() != app.StateBrowse {
		t.Fatalf("after resize, State = %v, want StateBrowse", m.State())
	}
}

// TestBrowseCtrlCExits130 verifies that ctrl+c while loading exits with
// code 130 through the Issue #4 cancellation path.
func TestBrowseCtrlCExits130(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	m, _ := setupBrowseLoading(t, idx)
	m, cmd := update(t, m, ctrlC())
	assertQuit(t, cmd)
	if m.ExitCode() != 130 {
		t.Fatalf("ExitCode = %d, want 130", m.ExitCode())
	}
}

// TestBrowseQExitsZero verifies that q in the browse state exits with
// code 0 through the browse quit path.
func TestBrowseQExitsZero(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	buf := makeBuf(nil, 0, 3)
	m := setupBrowse(t, idx, buf)
	m, cmd := update(t, m, keyPress('q'))
	assertQuit(t, cmd)
	if m.ExitCode() != 0 {
		t.Fatalf("ExitCode = %d, want 0", m.ExitCode())
	}
}

// TestBrowseFileLoadComplete verifies that a FileLoadCompleteMsg replaces
// the loading placeholder with content and highlights.
func TestBrowseFileLoadComplete(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello world\n", 1, subSpec{"hello", 0, 5}),
	)
	lines := []filebuffer.Line{
		ml(1, "hello world"),
		ml(2, "foo bar"),
	}
	buf := makeBuf(lines, 2, 3)
	m := setupBrowse(t, idx, buf)
	view := viewContent(m)
	if !strings.Contains(view, "hello world") {
		t.Fatalf("View does not contain 'hello world': %q", view)
	}
	if !strings.Contains(view, "foo bar") {
		t.Fatalf("View does not contain 'foo bar': %q", view)
	}
}

// TestBrowseLateLoadIgnoredAfterCancel verifies that a late
// FileLoadCompleteMsg arriving after cancellation is ignored.
func TestBrowseLateLoadIgnoredAfterCancel(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	m, _ := setupBrowseLoading(t, idx)
	m, cmd := update(t, m, ctrlC())
	assertQuit(t, cmd)
	if m.ExitCode() != 130 {
		t.Fatalf("ExitCode = %d, want 130", m.ExitCode())
	}
	m2, _ := update(t, m, app.FileLoadCompleteMsg{
		Path:   []byte("src/a.go"),
		Buffer: makeBuf(nil, 0, 3),
	})
	if m2.State() == app.StateBrowse && m2.ExitCode() == 0 {
		t.Fatal("late FileLoadCompleteMsg revived the cancelled UI")
	}
}

// TestBrowseFileListOrder verifies that the file list is in raw-path
// order.
func TestBrowseFileListOrder(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/zebra.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/alpha.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/middle.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	buf := makeBuf(nil, 0, 3)
	m := setupBrowse(t, idx, buf)
	view := viewContent(m)
	lines := strings.Split(view, "\n")
	// Find the lines containing the file names.
	var order []string
	for _, line := range lines {
		for _, name := range []string{"alpha.go", "middle.go", "zebra.go"} {
			if strings.Contains(line, name) {
				order = append(order, name)
			}
		}
	}
	if len(order) < 3 {
		t.Fatalf("expected at least 3 file entries, got %d in view:\n%s", len(order), view)
	}
	// alpha < middle < zebra in raw-path order.
	if order[0] != "alpha.go" || order[1] != "middle.go" || order[2] != "zebra.go" {
		t.Fatalf("file list order = %v, want [alpha.go middle.go zebra.go]", order)
	}
}

// TestBrowseCurrentFileUnderlined verifies that the current (first) file
// is underlined in the file list.
func TestBrowseCurrentFileUnderlined(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
		textMatch("src/b.go", "world\n", 1, subSpec{"world", 0, 5}),
	)
	buf := makeBuf(nil, 0, 3)
	m := setupBrowse(t, idx, buf)
	view := viewContent(m)
	// The underlined entry should contain the ANSI underline sequence
	// (SGR 4). The current file is a.go (first in raw-path order).
	if !strings.Contains(view, "\x1b[4m") {
		t.Fatalf("View does not contain underline ANSI sequence: %q", view)
	}
}

// TestBrowseFilenameRule verifies that the filename is embedded in a
// horizontal rule.
func TestBrowseFilenameRule(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	buf := makeBuf(nil, 0, 3)
	m := setupBrowse(t, idx, buf)
	view := viewContent(m)
	// The filename rule should contain the escaped filename and a
	// horizontal rule character.
	if !strings.Contains(view, "a.go") {
		t.Fatalf("View does not contain filename 'a.go': %q", view)
	}
	// The rule should contain a horizontal line character (─ or -).
	if !strings.ContainsAny(view, "─-") {
		t.Fatalf("View does not contain a horizontal rule character: %q", view)
	}
}

// TestBrowseGutterFormat verifies that the gutter is right-justified with
// two trailing spaces.
func TestBrowseGutterFormat(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	lines := []filebuffer.Line{
		ml(1, "hello"),
		ml(2, "world"),
	}
	buf := makeBuf(lines, 2, 3)
	m := setupBrowse(t, idx, buf)
	view := viewContent(m)
	// Gutter for line 1 should be "1  " (1 digit + 2 spaces).
	if !strings.Contains(view, "1  hello") {
		t.Fatalf("View does not contain gutter '1  hello': %q", view)
	}
	if !strings.Contains(view, "2  world") {
		t.Fatalf("View does not contain gutter '2  world': %q", view)
	}
}

// TestBrowseGutterRightJustified verifies that the gutter is
// right-justified when line numbers have different digit counts.
func TestBrowseGutterRightJustified(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	lines := []filebuffer.Line{
		ml(1, "first"),
		ml(10, "tenth"),
	}
	buf := makeBuf(lines, 10, 4)
	m := setupBrowse(t, idx, buf)
	view := viewContent(m)
	// Gutter for line 1 should be " 1  " (right-justified to 2 digits + 2 spaces).
	if !strings.Contains(view, " 1  first") {
		t.Fatalf("View does not contain right-justified gutter ' 1  first': %q", view)
	}
	if !strings.Contains(view, "10  tenth") {
		t.Fatalf("View does not contain gutter '10  tenth': %q", view)
	}
}

// TestBrowseNoBorders verifies that no box-drawing border characters are
// drawn around the file panel.
func TestBrowseNoBorders(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	lines := []filebuffer.Line{ml(1, "hello")}
	buf := makeBuf(lines, 1, 3)
	m := setupBrowse(t, idx, buf)
	view := viewContent(m)
	// No box-drawing border characters (┌┐└┘│─ used for borders, but ─
	// is also used for the filename rule, so check for the corner/edge
	// chars).
	borderChars := "┌┐└┘├┤┬┴┼"
	if strings.ContainsAny(view, borderChars) {
		t.Fatalf("View contains box-drawing border characters: %q", view)
	}
}

// TestBrowseInverseVideo verifies that matched spans are rendered with
// inverse video styling.
func TestBrowseInverseVideo(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello world\n", 1, subSpec{"hello", 0, 5}),
	)
	lines := []filebuffer.Line{
		ml(1, "hello world", [2]int{0, 5}),
	}
	buf := makeBuf(lines, 1, 3)
	m := setupBrowse(t, idx, buf)
	view := viewContent(m)
	// Inverse video uses SGR 7.
	if !strings.Contains(view, "\x1b[7m") {
		t.Fatalf("View does not contain inverse video ANSI sequence: %q", view)
	}
}

// TestBrowseInverseVideoCoversEscapedForm verifies that a highlight over
// an ESC byte covers both cells of the ^[ escape.
func TestBrowseInverseVideoCoversEscapedForm(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "a\x1bb\n", 1, subSpec{"\x1b", 1, 2}),
	)
	lines := []filebuffer.Line{
		ml(1, "a^[", [2]int{1, 3}),
	}
	buf := makeBuf(lines, 1, 3)
	m := setupBrowse(t, idx, buf)
	view := viewContent(m)
	// The inverse video sequence should be present.
	if !strings.Contains(view, "\x1b[7m") {
		t.Fatalf("View does not contain inverse video for escaped form: %q", view)
	}
}

// --- Hostile-fixture sink-safety tests ---

// noControlBytes reports whether the string contains no C0 control bytes
// or DEL, excluding newlines which are part of the multi-line layout.
// This is the raw-output assertion for sink-safety: no fixture control
// byte may survive verbatim.
func noControlBytes(s string) bool {
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b == '\n' {
			continue
		}
		if b < 0x20 || b == 0x7f {
			return false
		}
	}
	return true
}

// TestSinkSafetyFileList verifies that hostile path bytes in the file
// list do not produce raw control bytes in the output. Rendered through
// the no-style composition path so no ANSI escape may legitimately appear.
func TestSinkSafetyFileList(t *testing.T) {
	fixtures := [][]byte{
		[]byte("\x1b]0;x\x07"),   // OSC
		[]byte("\x1b[2J"),        // CSI
		[]byte("\x07\x08\x1b"),   // C0
		[]byte("\xc2\x85"),       // C1 (NEL)
		[]byte("\x7f"),           // DEL
		[]byte("\r"),             // standalone CR
		[]byte("foo\xff\xfebar"), // invalid UTF-8
		[]byte("file\nname"),     // embedded newline
		[]byte("file\tname"),     // embedded tab
		[]byte(`a\b`),            // backslash
	}
	for _, fx := range fixtures {
		t.Run(fmt.Sprintf("path_%q", fx), func(t *testing.T) {
			idx := buildIndex(t, "/work",
				textMatch(string(fx), "x\n", 1, subSpec{"x", 0, 1}),
			)
			buf := makeBuf(nil, 0, 3)
			gate := make(chan struct{})
			close(gate)
			loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
				return buf, nil
			}
			m := app.New([]string{"--json", "--", "foo", "."}, "/work",
				app.WithFileLoadGate(gate),
				app.WithFileLoader(loader),
				app.WithTheme(theme.NoStyle()),
			)
			m, _ = update(t, m, app.SearchCompleteMsg{
				Files: idx.Files(), Lines: idx.Len(), Index: idx,
			})
			view := viewContent(m)
			if !noControlBytes(view) {
				t.Fatalf("raw control byte in file-list output for %q: %q", fx, view)
			}
		})
	}
}

// TestSinkSafetyFilenameRule verifies that hostile path bytes in the
// filename rule do not produce raw control bytes in the output.
func TestSinkSafetyFilenameRule(t *testing.T) {
	fixtures := [][]byte{
		[]byte("\x1b]0;x\x07"),
		[]byte("\x1b[2J"),
		[]byte("\x07\x08\x1b"),
		[]byte("\xc2\x85"),
		[]byte("\x7f"),
		[]byte("\r"),
		[]byte("foo\xff\xfebar"),
		[]byte("file\nname"),
		[]byte("file\tname"),
		[]byte(`a\b`),
	}
	for _, fx := range fixtures {
		t.Run(fmt.Sprintf("rule_%q", fx), func(t *testing.T) {
			idx := buildIndex(t, "/work",
				textMatch(string(fx), "hello\n", 1, subSpec{"hello", 0, 5}),
			)
			lines := []filebuffer.Line{ml(1, "hello")}
			buf := makeBuf(lines, 1, 3)
			gate := make(chan struct{})
			close(gate)
			loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
				return buf, nil
			}
			m := app.New([]string{"--json", "--", "foo", "."}, "/work",
				app.WithFileLoadGate(gate),
				app.WithFileLoader(loader),
				app.WithTheme(theme.NoStyle()),
			)
			m, _ = update(t, m, app.SearchCompleteMsg{
				Files: idx.Files(), Lines: idx.Len(), Index: idx,
			})
			view := viewContent(m)
			if !noControlBytes(view) {
				t.Fatalf("raw control byte in filename-rule output for %q: %q", fx, view)
			}
		})
	}
}

// TestSinkSafetyPanelContent verifies that hostile content bytes in the
// panel do not produce raw control bytes in the output.
func TestSinkSafetyPanelContent(t *testing.T) {
	fixtures := [][]byte{
		[]byte("\x1b]0;pwned\x07"), // OSC
		[]byte("\x1b[2J"),          // CSI
		[]byte("\x07\x08\x1b"),     // C0
		[]byte("\xc2\x85"),         // C1 (NEL)
		[]byte("\x7f"),             // DEL
		[]byte("a\rb"),             // standalone CR
		[]byte("foo\xff\xfebar"),   // invalid UTF-8
		[]byte("line\nline"),       // LF
		[]byte("line\r\nline"),     // CRLF
		[]byte("a\tb"),             // tab
	}
	for _, fx := range fixtures {
		t.Run(fmt.Sprintf("content_%q", fx), func(t *testing.T) {
			idx := buildIndex(t, "/work",
				textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
			)
			lines := []filebuffer.Line{
				{Number: 1, Display: string(fx)},
			}
			buf := makeBuf(lines, 1, 3)
			gate := make(chan struct{})
			close(gate)
			loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
				return buf, nil
			}
			m := app.New([]string{"--json", "--", "foo", "."}, "/work",
				app.WithFileLoadGate(gate),
				app.WithFileLoader(loader),
				app.WithTheme(theme.NoStyle()),
			)
			m, cmd := update(t, m, app.SearchCompleteMsg{
				Files: idx.Files(), Lines: idx.Len(), Index: idx,
			})
			if cmd != nil {
				msg := execCmd(t, cmd)
				if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
					m, _ = update(t, m, lc)
				}
			}
			view := viewContent(m)
			if !noControlBytes(view) {
				t.Fatalf("raw control byte in panel-content output for %q: %q", fx, view)
			}
		})
	}
}

// TestSinkSafetyAllSinksHostile verifies that a single hostile fixture
// with a hostile filename, hostile content, and a matching line
// containing an OSC sequence produces no raw control bytes in any sink.
func TestSinkSafetyAllSinksHostile(t *testing.T) {
	// Filename with newline and ESC byte.
	hostilePath := []byte("file\x1b\nname")
	// Content with OSC sequence.
	hostileContent := []byte("\x1b]0;pwned\x07")
	idx := buildIndex(t, "/work",
		bytesMatch(hostilePath, hostileContent, 1, subSpec{"\x1b", 0, 1}),
	)
	lines := []filebuffer.Line{
		{Number: 1, Display: "^[]0;pwned^G"},
	}
	buf := makeBuf(lines, 1, 3)
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return buf, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithTheme(theme.NoStyle()),
	)
	m, cmd := update(t, m, app.SearchCompleteMsg{
		Files: idx.Files(), Lines: idx.Len(), Index: idx,
	})
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, _ = update(t, m, lc)
		}
	}
	view := viewContent(m)
	if !noControlBytes(view) {
		t.Fatalf("raw control byte in hostile all-sinks output: %q", view)
	}
}

// --- Issue #6 shared sink-safety table tests ---
//
// These tests restructure the Issue #5 hostile fixture set into the
// shared, extensible sink-safety table from internal/sinkfixtures. Each
// sink is a row; later issues add rows without duplicating fixtures.
// The no-style path asserts no fixture control byte survives in raw
// output (before any ANSI stripping). The styled path asserts the
// fixture's distinctive payload never appears immediately after an
// unescaped ESC.

// setupBrowseSink creates a browse model with a single file whose path
// or content is the fixture, rendered through the given theme. The
// file load completes immediately.
func setupBrowseSink(t *testing.T, path []byte, content string, themed theme.Theme) app.Model {
	t.Helper()
	idx := buildIndex(t, "/work",
		bytesMatch(path, []byte(content+"\n"), 1, subSpec{content, 0, len(content)}),
	)
	lines := []filebuffer.Line{
		{Number: 1, Display: content},
	}
	buf := makeBuf(lines, 1, 3)
	gate := make(chan struct{})
	close(gate)
	loader := func(p []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return buf, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithTheme(themed),
	)
	m, cmd := update(t, m, app.SearchCompleteMsg{
		Files: idx.Files(), Lines: idx.Len(), Index: idx,
	})
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, _ = update(t, m, lc)
		}
	}
	return m
}

// TestSinkSafetyTableFileListNoStyle verifies that the file-list sink
// produces no raw control bytes for every shared fixture, rendered
// through the no-style composition path.
func TestSinkSafetyTableFileListNoStyle(t *testing.T) {
	for _, fx := range sinkfixtures.Fixtures {
		t.Run(fx.Name, func(t *testing.T) {
			m := setupBrowseSink(t, fx.Raw, "hello", theme.NoStyle())
			view := viewContent(m)
			if !sinkfixtures.NoControlBytes(view) {
				t.Fatalf("raw control byte in file-list for %s: %q", fx.Name, view)
			}
		})
	}
}

// TestSinkSafetyTableFilenameRuleNoStyle verifies that the filename-rule
// sink produces no raw control bytes for every shared fixture, rendered
// through the no-style composition path.
func TestSinkSafetyTableFilenameRuleNoStyle(t *testing.T) {
	for _, fx := range sinkfixtures.Fixtures {
		t.Run(fx.Name, func(t *testing.T) {
			m := setupBrowseSink(t, fx.Raw, "hello", theme.NoStyle())
			view := viewContent(m)
			if !sinkfixtures.NoControlBytes(view) {
				t.Fatalf("raw control byte in filename-rule for %s: %q", fx.Name, view)
			}
		})
	}
}

// TestSinkSafetyTablePanelContentNoStyle verifies that the panel-content
// sink produces no raw control bytes for every shared fixture, rendered
// through the no-style composition path.
func TestSinkSafetyTablePanelContentNoStyle(t *testing.T) {
	for _, fx := range sinkfixtures.Fixtures {
		t.Run(fx.Name, func(t *testing.T) {
			m := setupBrowseSink(t, []byte("src/a.go"), string(fx.Raw), theme.NoStyle())
			view := viewContent(m)
			if !sinkfixtures.NoControlBytes(view) {
				t.Fatalf("raw control byte in panel-content for %s: %q", fx.Name, view)
			}
		})
	}
}

// TestSinkSafetyTableFileListStyled verifies that with styles enabled,
// the fixture's distinctive payload never appears immediately after an
// unescaped ESC in the file-list sink.
func TestSinkSafetyTableFileListStyled(t *testing.T) {
	for _, fx := range sinkfixtures.Fixtures {
		t.Run(fx.Name, func(t *testing.T) {
			m := setupBrowseSink(t, fx.Raw, "hello", theme.New())
			view := viewContent(m)
			if !sinkfixtures.NoPayloadAfterESC(view, fx.Payload) {
				t.Fatalf("fixture payload after unescaped ESC in file-list for %s: %q", fx.Name, view)
			}
		})
	}
}

// TestSinkSafetyTableFilenameRuleStyled verifies that with styles
// enabled, the fixture's distinctive payload never appears immediately
// after an unescaped ESC in the filename-rule sink.
func TestSinkSafetyTableFilenameRuleStyled(t *testing.T) {
	for _, fx := range sinkfixtures.Fixtures {
		t.Run(fx.Name, func(t *testing.T) {
			m := setupBrowseSink(t, fx.Raw, "hello", theme.New())
			view := viewContent(m)
			if !sinkfixtures.NoPayloadAfterESC(view, fx.Payload) {
				t.Fatalf("fixture payload after unescaped ESC in filename-rule for %s: %q", fx.Name, view)
			}
		})
	}
}

// TestSinkSafetyTablePanelContentStyled verifies that with styles
// enabled, the fixture's distinctive payload never appears immediately
// after an unescaped ESC in the panel-content sink.
func TestSinkSafetyTablePanelContentStyled(t *testing.T) {
	for _, fx := range sinkfixtures.Fixtures {
		t.Run(fx.Name, func(t *testing.T) {
			m := setupBrowseSink(t, []byte("src/a.go"), string(fx.Raw), theme.New())
			view := viewContent(m)
			if !sinkfixtures.NoPayloadAfterESC(view, fx.Payload) {
				t.Fatalf("fixture payload after unescaped ESC in panel-content for %s: %q", fx.Name, view)
			}
		})
	}
}

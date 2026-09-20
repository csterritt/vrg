package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/cli"
	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

// hostileFixture is one payload of the shared Issue 5 hostile fixture
// set. data is injected at the sink's substitution point; payload is the
// fixture's distinctive byte sequence that must never appear immediately
// after an unescaped ESC when styles are enabled; wantPath is the
// fixture's single-line path-escaped form (the name sinks and usage
// operands); wantContent holds the escaped fragments the panel-content
// sink must show.
type hostileFixture struct {
	name        string
	data        []byte
	payload     string
	wantPath    string
	wantContent []string
}

// hostileFixtures is the sink-safety fixture set, defined once so every
// sink row — including the rows later issues add for sinks they own —
// runs against the same payloads without duplicating them.
var hostileFixtures = []hostileFixture{
	{"osc", []byte("\x1b]0;pwned\x07"), "]0;pwned", `^[]0;pwned^G`, []string{"^[]0;pwned^G"}},
	{"csi", []byte("\x1b[2J"), "[2J", `^[[2J`, []string{"^[[2J"}},
	{"c0 bell", []byte("\x07"), "\x07", `^G`, []string{"^G"}},
	{"c0 backspace", []byte("\x08"), "\x08", `^H`, []string{"^H"}},
	{"c0 escape", []byte("\x1b"), "\x1b", `^[`, []string{"^["}},
	{"c1 nel", []byte("\xc2\x85"), "\xc2\x85", `\u0085`, []string{`\u0085`}},
	{"delete", []byte("\x7f"), "\x7f", `^?`, []string{"^?"}},
	{"standalone cr", []byte("\r"), "\r", `\r`, []string{"^M"}},
	{"invalid utf-8 path bytes", []byte("\xff\xfe"), "\xff\xfe", `\xff\xfe`, []string{"\uFFFD\uFFFD"}},
	{"embedded filename newline", []byte("evil\nname.txt"), "\nname", `evil\nname.txt`, []string{"evil", "name.txt"}},
	{"embedded tab", []byte("\t"), "\t", `\t`, nil},
}

// sinkRow is one output sink in the safety table: render drives a
// hostile fixture through the sink's real composition path and returns
// the sink's raw output — before any ANSI stripping — and expect lists
// the escaped fragments that must appear in the no-style render (nil
// when the sink carries no substitution point). Later issues add rows
// here for the sinks they own without touching hostileFixtures.
type sinkRow struct {
	name   string
	render func(t *testing.T, f hostileFixture, styled bool) string
	expect func(f hostileFixture) []string
}

var sinkSafetyTable = []sinkRow{
	{"file-list entry", browseFilenameSink, pathExpect},
	{"filename rule", browseFilenameSink, ruleExpect},
	{"panel content", browseContentSink, func(f hostileFixture) []string { return f.wantContent }},
	{"usage-error stderr", usageErrorSink, pathExpect},
	{"error overlay", errorOverlaySink, overlayExpect},
	{"file-change pop-up", popupSink, popupExpect},
	{"stderr replay", stderrReplaySink, pathExpect},
	// The generated command-line help has no substitution point — the
	// application name and every description are fixed — so the row
	// pins the sink's clean-output contract itself.
	{"cli-help stdout", cliHelpSink, func(hostileFixture) []string { return nil }},
}

func pathExpect(f hostileFixture) []string { return []string{f.wantPath} }
func ruleExpect(f hostileFixture) []string { return []string{"── " + f.wantPath} }

// browseFilenameSink injects the fixture as the matched file's raw path
// bytes, so it reaches both the file-list entry and the filename rule.
func browseFilenameSink(t *testing.T, f hostileFixture, styled bool) string {
	t.Helper()
	return browseSinkFrame(t, f.data, []byte("hit\n"), styled)
}

// browseContentSink injects the fixture as the matched file's content.
func browseContentSink(t *testing.T, f hostileFixture, styled bool) string {
	t.Helper()
	return browseSinkFrame(t, []byte("fixture.txt"), f.data, styled)
}

// browseSinkFrame composes a real browse frame — index record, load,
// View() — over a one-file fixture on disk.
func browseSinkFrame(t *testing.T, name, content []byte, styled bool) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, string(name)), content, 0o644); err != nil {
		t.Fatal(err)
	}
	idx := searchindex.New(dir)
	addRec(t, idx, matchRecBytes(name, content, 1, 0, 1, content[:1]))
	idx.Finish()

	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, dir)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m, cmd := update(t, m, searchResult{index: idx, integrity: completeStream})
	m, _ = update(t, m, cmd())
	if !styled {
		m.theme = theme.Plain()
	}
	return m.View().Content
}

// usageErrorSink injects the fixture as the root operand; the classified
// usage error's Diagnostic is the single line destined for stderr.
func usageErrorSink(t *testing.T, f hostileFixture, _ bool) string {
	t.Helper()
	res := cli.Parse([]string{"p", string(f.data)}, &bytes.Buffer{}, cli.Env{Stat: os.Stat})
	if res.Kind != cli.KindUsageError {
		t.Fatalf("fixture operand %q produced kind %v, want KindUsageError", f.data, res.Kind)
	}
	return res.Diagnostic
}

// errorOverlaySink injects the fixture as the failed child's stderr on
// a fatal run with no results: the fixture reaches the error overlay's
// interior through the real composition path.
func errorOverlaySink(t *testing.T, f hostileFixture, styled bool) string {
	t.Helper()
	b := searchindex.NewBuilder("/w")
	if err := b.Add([]byte(summaryRec())); err != nil {
		t.Fatalf("Add: %v", err)
	}
	idx, integrity := b.Finish()
	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, "/w")
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m = feedStderr(t, m, string(f.data))
	m, _ = update(t, m, searchResult{
		index: idx, integrity: integrity,
		err: exitError(2),
	})
	if !styled {
		m.theme = theme.Plain()
	}
	return m.View().Content
}

// overlayExpect lists the escaped form each fixture takes in the error
// overlay's diagnostic text. Diagnostics use the multi-line
// EscapeDiagnostic form: invalid UTF-8 escapes per byte as \xNN (the
// wantPath rendering), a tab expands to spaces with no assertable
// fragment, and an embedded newline stays a real line boundary.
func overlayExpect(f hostileFixture) []string {
	switch f.name {
	case "invalid utf-8 path bytes":
		return []string{f.wantPath}
	case "embedded tab":
		return nil
	default:
		return f.wantContent
	}
}

// popupSink drives the fixture through the file-change pop-up's real
// composition path: the fixture is the raw path navigation selects, so
// its escaped single-line form must appear inside the pop-up's bordered
// interior.
func popupSink(t *testing.T, f hostileFixture, styled bool) string {
	t.Helper()
	dir := t.TempDir()
	idx := searchindex.New(dir)
	addRec(t, idx, matchRecBytes(f.data, []byte("hit x\n"), 1, 0, 3, []byte("hit")))
	addRec(t, idx, matchRec("companion.txt", "hit c\n", 1, 0, 3, "hit"))
	idx.Finish()

	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, dir)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = update(t, m, searchResult{index: idx, integrity: completeStream})

	// The pop-up fires on a file change: when the fixture sorts first
	// the cursor leaves and returns to it, otherwise one step lands on
	// it.
	steps := 1
	if bytes.Equal(idx.Stops()[0].Path, f.data) {
		steps = 2
	}
	for i := 0; i < steps; i++ {
		m, _ = update(t, m, keyMsg("n"))
	}
	if !styled {
		m.theme = theme.Plain()
	}
	return m.View().Content
}

// popupExpect requires the fixture's escaped single-line path inside
// the pop-up's bordered interior.
func popupExpect(f hostileFixture) []string {
	return []string{"│" + f.wantPath + "│"}
}

// stderrReplaySink collects a load-failure diagnostic embedding the
// fixture filename and emits it through the replay writer — the
// post-restoration sink that writes every collected diagnostic to the
// user's stderr.
func stderrReplaySink(t *testing.T, f hostileFixture, _ bool) string {
	t.Helper()
	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, "/w")
	m, _ = update(t, m, loadResult{path: f.data, err: errors.New("denied")})
	var out bytes.Buffer
	m.diags.replay(&out)
	return out.String()
}

// cliHelpSink renders the Issue 1 generated command-line help, the
// stdout sink — distinct from the Issue 31 TUI help dialog, which adds
// its own row when it lands.
func cliHelpSink(t *testing.T, _ hostileFixture, _ bool) string {
	t.Helper()
	var out bytes.Buffer
	res := cli.Parse([]string{"--help"}, &out, cli.Env{})
	if res.Kind != cli.KindHelp {
		t.Fatalf("help request produced kind %v, want KindHelp", res.Kind)
	}
	return out.String()
}

// assertNoControlBytes is the raw-output sink contract: rendered through
// the no-style composition path, no escape byte may legitimately appear,
// so the output must contain no C0 control byte other than the real line
// boundary, no DEL, no C1 control, and no invalid UTF-8 — checked on the
// raw bytes before any ANSI stripping, because stripping would erase the
// very evidence sought.
func assertNoControlBytes(t *testing.T, sink, raw string) {
	t.Helper()
	for i := 0; i < len(raw); i++ {
		if b := raw[i]; (b < 0x20 && b != '\n') || b == 0x7f {
			t.Fatalf("%s output contains control byte %#02x at offset %d:\n%q", sink, b, i, raw)
		}
	}
	if !utf8.ValidString(raw) {
		t.Fatalf("%s output is not valid UTF-8:\n%q", sink, raw)
	}
	for _, r := range raw {
		if r >= 0x80 && r < 0xa0 {
			t.Fatalf("%s output contains C1 control %#04x:\n%q", sink, r, raw)
		}
	}
}

// Every hostile fixture, driven through every sink's real composition
// path, leaves no fixture control byte in the sink's raw output: the
// no-style render may contain no control bytes at all, and the styled
// render never places the fixture's distinctive payload immediately
// after an unescaped ESC.
func TestSinkSafety(t *testing.T) {
	for _, sink := range sinkSafetyTable {
		t.Run(sink.name, func(t *testing.T) {
			for _, f := range hostileFixtures {
				t.Run(f.name, func(t *testing.T) {
					raw := sink.render(t, f, false)
					assertNoControlBytes(t, sink.name, raw)
					for _, want := range sink.expect(f) {
						if !strings.Contains(raw, want) {
							t.Fatalf("%s output lacks escaped form %q for fixture %q:\n%s",
								sink.name, want, f.name, raw)
						}
					}
					styled := sink.render(t, f, true)
					if strings.Contains(styled, "\x1b"+f.payload) {
						t.Fatalf("%s styled output emits fixture payload %q after an unescaped ESC:\n%q",
							sink.name, f.payload, styled)
					}
				})
			}
		})
	}
}

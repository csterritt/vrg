package app

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"vrg/internal/cli"
	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

// hostileFixture is one payload from the shared hostile-fixture set —
// the Issue #5 fixture set generalized for every sink. inject carries
// the hostile bytes; sinks embed them in whichever input they consume
// (a filename for path-bearing sinks, a matched content line for the
// panel sink, a root operand for the usage-error sink, stderr text for
// the diagnostics-overlay sink). forbidden lists raw bytes that must
// never survive verbatim in the sink's output; payload is the fixture's
// distinctive post-ESC form, asserted never to follow an unescaped ESC
// under styles. wantPath, wantText, and wantDiag are the escaped
// substrings filename-bearing, content, and diagnostic sinks must show.
type hostileFixture struct {
	name      string
	inject    string
	forbidden []string
	payload   string
	wantPath  []string
	wantText  []string
	wantDiag  []string
}

// hostileFixtures is the shared fixture set every sink row drives: OSC,
// CSI, C0 controls, C1, DEL, a standalone CR, invalid UTF-8 path bytes,
// and an embedded filename newline. The escaped expectations are written
// out independently of the utility under test.
var hostileFixtures = []hostileFixture{
	{
		name:      "osc title set",
		inject:    "\x1b]0;pwned\x07",
		forbidden: []string{"\x1b", "\x07"},
		payload:   "]0;pwned",
		wantPath:  []string{`f^[]0;pwned^G.txt`},
		wantText:  []string{`1  pre ^[]0;pwned^G post`},
		wantDiag:  []string{`pre^[]0;pwned^Gpost`},
	},
	{
		name:      "csi erase",
		inject:    "\x1b[2J",
		forbidden: []string{"\x1b"},
		payload:   "[2J",
		wantPath:  []string{`f^[[2J.txt`},
		wantText:  []string{`1  pre ^[[2J post`},
		wantDiag:  []string{`pre^[[2Jpost`},
	},
	{
		name:      "c0 controls",
		inject:    "\x07\x08\x1b",
		forbidden: []string{"\x07", "\x08", "\x1b"},
		wantPath:  []string{`f^G^H^[.txt`},
		wantText:  []string{`1  pre ^G^H^[ post`},
		wantDiag:  []string{`pre^G^H^[post`},
	},
	{
		name:      "c1 nel",
		inject:    "\xc2\x85",
		forbidden: []string{"\xc2\x85", "\x85"},
		wantPath:  []string{`f\u0085.txt`},
		wantText:  []string{`1  pre \u0085 post`},
		wantDiag:  []string{`pre\u0085post`},
	},
	{
		name:      "delete",
		inject:    "\x7f",
		forbidden: []string{"\x7f"},
		wantPath:  []string{`f^?.txt`},
		wantText:  []string{`1  pre ^? post`},
		wantDiag:  []string{`pre^?post`},
	},
	{
		name:      "standalone carriage return",
		inject:    "\r",
		forbidden: []string{"\r"},
		wantPath:  []string{`f\r.txt`},
		wantText:  []string{`1  pre ^M post`},
		wantDiag:  []string{`pre^Mpost`},
	},
	{
		name:      "invalid utf-8 path bytes",
		inject:    "\xff\xfe",
		forbidden: []string{"\xff", "\xfe"},
		wantPath:  []string{`f\xff\xfe.txt`},
		wantText:  []string{"1  pre \ufffd\ufffd post"},
		wantDiag:  []string{`pre\xff\xfepost`},
	},
	{
		name:   "embedded filename newline",
		inject: "\n",
		// A raw newline is legitimate frame and stderr structure, so the
		// fixture forbids no byte: the escaped-form and structure checks
		// prove it never survived inside a name. In a diagnostic the
		// newline is a real line boundary, splitting the text in two.
		wantPath: []string{`f\n.txt`},
		wantText: []string{"1  pre ", "2   post"},
		wantDiag: []string{"pre", "post"},
	},
}

// sinkRow is one output sink's row in the shared sink-safety table.
// render drives one fixture through the sink's real composition path
// and returns the raw, unstripped output; check adds the sink-specific
// escaped-form assertions. tui marks sinks whose output is a whole
// frame, so its row count itself proves an embedded newline stayed
// single-lined. styled requests a second render under real styling.
// Later issues add their new sinks as rows here — the fixture set is
// shared and must not be duplicated per sink.
type sinkRow struct {
	name   string
	tui    bool
	styled bool
	render func(t *testing.T, fx hostileFixture, styled bool) string
	check  func(t *testing.T, fx hostileFixture, raw string)
}

// sinkSafetySinks is the table of every output sink existing at this
// point: the three browse sinks, the Issue #15 file-change pop-up, the
// usage-error stderr composition, and the generated command-line help
// on stdout (distinct from the Issue #31 TUI help dialog, which adds
// its own row later).
var sinkSafetySinks = []sinkRow{
	{
		name:   "file-list entry",
		tui:    true,
		styled: true,
		render: func(t *testing.T, fx hostileFixture, styled bool) string {
			return renderBrowseSink(t, fx, true, styled)
		},
		check: func(t *testing.T, fx hostileFixture, raw string) {
			// The escaped entry occupies a list row: it starts the
			// frame or a row of it, never shares its row with a break.
			for _, w := range fx.wantPath {
				if !strings.HasPrefix(raw, w) && !strings.Contains(raw, "\n"+w) {
					t.Fatalf("file-list row lacks escaped form %q: %q", w, raw)
				}
			}
		},
	},
	{
		name:   "filename rule",
		tui:    true,
		styled: true,
		render: func(t *testing.T, fx hostileFixture, styled bool) string {
			return renderBrowseSink(t, fx, true, styled)
		},
		check: func(t *testing.T, fx hostileFixture, raw string) {
			for _, w := range fx.wantPath {
				if !strings.Contains(raw, "── "+w) {
					t.Fatalf("filename rule lacks escaped form %q: %q", w, raw)
				}
			}
		},
	},
	{
		name:   "panel content",
		tui:    true,
		styled: true,
		render: func(t *testing.T, fx hostileFixture, styled bool) string {
			return renderBrowseSink(t, fx, false, styled)
		},
		check: func(t *testing.T, fx hostileFixture, raw string) {
			for _, w := range fx.wantText {
				if !strings.Contains(raw, w) {
					t.Fatalf("panel content lacks escaped form %q: %q", w, raw)
				}
			}
		},
	},
	{
		name:   "file-change pop-up",
		tui:    true,
		styled: true,
		render: func(t *testing.T, fx hostileFixture, styled bool) string {
			return renderPopupSink(t, fx, styled)
		},
		check: func(t *testing.T, fx hostileFixture, raw string) {
			// The hostile bytes name the destination file; the pop-up
			// shows their Path-escaped single-line form inside the box.
			for _, w := range fx.wantPath {
				if !strings.Contains(raw, "│"+w+"│") {
					t.Fatalf("pop-up lacks escaped form %q: %q", w, raw)
				}
			}
		},
	},
	{
		name:   "error overlay",
		tui:    true,
		styled: true,
		render: func(t *testing.T, fx hostileFixture, styled bool) string {
			return renderOverlaySink(t, fx, styled)
		},
		check: func(t *testing.T, fx hostileFixture, raw string) {
			// The hostile bytes ride in as captured stderr; the overlay
			// shows their Diagnostic-escaped form inside the border.
			for _, w := range fx.wantDiag {
				if !strings.Contains(raw, w) {
					t.Fatalf("overlay diagnostic lacks escaped form %q: %q", w, raw)
				}
			}
		},
	},
	{
		name: "usage-error stderr",
		render: func(t *testing.T, fx hostileFixture, _ bool) string {
			return renderUsageErrorSink(t, fx)
		},
		check: func(t *testing.T, fx hostileFixture, raw string) {
			// The diagnostic is the first line; an embedded filename
			// stays single-lined within it, followed by a blank line
			// and the generated help.
			diag, rest, found := strings.Cut(raw, "\n")
			if !found || !strings.HasPrefix(diag, "vrg: ") {
				t.Fatalf("usage-error stderr lacks a diagnostic line: %q", raw)
			}
			for _, w := range fx.wantPath {
				if !strings.Contains(diag, w) {
					t.Fatalf("usage diagnostic lacks escaped form %q: %q", w, diag)
				}
			}
			if !strings.HasPrefix(rest, "\nUsage: vrg") {
				t.Fatalf("usage-error stderr lacks the following help block: %q", raw)
			}
		},
	},
	{
		name: "cli-help stdout",
		render: func(t *testing.T, fx hostileFixture, _ bool) string {
			return renderCLIHelpSink(t, fx)
		},
		check: func(t *testing.T, fx hostileFixture, raw string) {
			if !strings.HasPrefix(raw, "Usage: vrg") {
				t.Fatalf("cli-help stdout lacks the generated help: %q", raw)
			}
		},
	},
	{
		name: "stderr replay",
		render: func(t *testing.T, fx hostileFixture, _ bool) string {
			return renderReplaySink(t, fx)
		},
		check: func(t *testing.T, fx hostileFixture, raw string) {
			// The replayed diagnostics carry the escaped stderr text
			// and the escaped embedded filename — the filename's
			// single-line form proves its newline could not become a
			// diagnostic line break.
			for _, w := range fx.wantDiag {
				if !strings.Contains(raw, w) {
					t.Fatalf("replayed diagnostic lacks escaped form %q: %q", w, raw)
				}
			}
			for _, w := range fx.wantPath {
				if !strings.Contains(raw, w) {
					t.Fatalf("replayed diagnostic lacks escaped filename %q: %q", w, raw)
				}
			}
		},
	},
}

// The shared sink-safety table: every fixture is driven through every
// sink's real composition path — browse sinks rendered through
// theme.Plain, the no-style composition path where no escape byte may
// legitimately appear — and the raw output, before any ANSI stripping,
// must contain no fixture control byte verbatim and none of the
// universal set (ESC, BEL, raw CSI, C1 NEL, bare CR). A styled pass
// then asserts the fixture's distinctive payload never follows an
// unescaped ESC.
func TestSinkSafetyTable(t *testing.T) {
	for _, sink := range sinkSafetySinks {
		for _, fx := range hostileFixtures {
			t.Run(sink.name+"/"+fx.name, func(t *testing.T) {
				raw := sink.render(t, fx, false)
				assertNoEscapeBytes(t, sink.name, raw)
				for _, bad := range fx.forbidden {
					if strings.Contains(raw, bad) {
						t.Fatalf("%s: raw output contains fixture byte %q: %q", sink.name, bad, raw)
					}
				}
				if sink.tui {
					if n := strings.Count(raw, "\n"); n != 23 {
						t.Fatalf("%s: output rows = %d newlines, want 23 for a 24-row frame: %q",
							sink.name, n, raw)
					}
				}
				if sink.check != nil {
					sink.check(t, fx, raw)
				}
				if !sink.styled {
					return
				}
				styled := sink.render(t, fx, true)
				if fx.payload != "" && strings.Contains(styled, "\x1b"+fx.payload) {
					t.Fatalf("%s: fixture payload %q follows an unescaped ESC: %q",
						sink.name, fx.payload, styled)
				}
				for _, bad := range fx.forbidden {
					if bad == "\x1b" {
						continue // trusted SGR sequences carry ESC legitimately
					}
					if strings.Contains(styled, bad) {
						t.Fatalf("%s: styled output contains fixture byte %q: %q", sink.name, bad, styled)
					}
				}
			})
		}
	}
}

// renderBrowseSink drives fx through the real browse composition at
// 80x24 and returns the raw View() content. inName selects the injected
// input: true names the matched file "f<inject>.txt", false loads
// "f.txt" with the matched line "pre <inject> post". styled selects
// real styling; false renders through theme.Plain, the no-style
// composition path.
func renderBrowseSink(t *testing.T, fx hostileFixture, inName bool, styled bool) string {
	t.Helper()
	dir := t.TempDir()
	name, content, sub := "f.txt", "pre "+fx.inject+" post\n", []byte(fx.inject)
	start, end := 4, 4+len(fx.inject)
	if inName {
		name, content = "f"+fx.inject+".txt", "hit\n"
		sub, start, end = []byte("hit"), 0, 3
	} else if strings.Contains(fx.inject, "\n") {
		// The injection is a real line boundary in content; highlight
		// the leading word of the first line instead.
		sub, start, end = []byte("pre"), 0, 3
	}
	writeWorkFile(t, dir, name, content)
	line := content
	if i := strings.IndexByte(content, '\n'); i >= 0 {
		line = content[:i+1]
	}
	ix := searchindex.New(dir)
	ix.Add(searchindex.Record{Kind: searchindex.KindBegin, Path: []byte(name)})
	ix.Add(searchindex.Record{
		Kind:       searchindex.KindMatch,
		Path:       []byte(name),
		LineNumber: 1,
		Line:       []byte(line),
		Submatches: []searchindex.Submatch{{Start: start, End: end, Bytes: sub}},
	})
	ix.Add(searchindex.Record{Kind: searchindex.KindEnd, Path: []byte(name)})
	ix.Add(searchindex.Record{Kind: searchindex.KindSummary})
	ix.Prepare()

	m := newModel(nil, nil)
	if !styled {
		m.theme = theme.Plain()
	}
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, searchDoneMsg{index: ix})
	m = settle(t, m, cmd)
	return m.View().Content
}

// renderPopupSink drives the fixture through the file-change pop-up's
// real composition path at 80x24: the hostile bytes name the second
// matched file, n crosses the file boundary, and the raw View()
// content is returned with the pop-up composited on top. styled
// selects real styling; false renders through theme.Plain.
func renderPopupSink(t *testing.T, fx hostileFixture, styled bool) string {
	t.Helper()
	dir := t.TempDir()
	hostile := "f" + fx.inject + ".txt"
	writeWorkFile(t, dir, "a.txt", "hit\n")
	writeWorkFile(t, dir, hostile, "hit\n")
	ix := searchindex.New(dir)
	for _, name := range []string{"a.txt", hostile} {
		ix.Add(searchindex.Record{Kind: searchindex.KindBegin, Path: []byte(name)})
		ix.Add(searchindex.Record{
			Kind:       searchindex.KindMatch,
			Path:       []byte(name),
			LineNumber: 1,
			Line:       []byte("hit\n"),
			Submatches: []searchindex.Submatch{{Start: 0, End: 3, Bytes: []byte("hit")}},
		})
		ix.Add(searchindex.Record{Kind: searchindex.KindEnd, Path: []byte(name)})
	}
	ix.Add(searchindex.Record{Kind: searchindex.KindSummary})
	ix.Prepare()

	m := newModel(nil, nil)
	if !styled {
		m.theme = theme.Plain()
	}
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = update(t, m, searchDoneMsg{index: ix})
	m, _ = update(t, m, keyPress("n"))
	return m.View().Content
}

// renderOverlaySink drives the fixture through the diagnostics
// overlay's real composition path at 80x24: a valid one-match stream, a
// fatal child exit, and stderr carrying "pre<inject>post" — the overlay
// opens over browse and the raw View() content is returned. styled
// selects real styling; false renders through theme.Plain.
func renderOverlaySink(t *testing.T, fx hostileFixture, styled bool) string {
	t.Helper()
	dir := t.TempDir()
	writeWorkFile(t, dir, "f.txt", "hit\n")
	ix := fixtureIndex(t, dir, recsOneMatch...)

	m := newModel(nil, nil)
	if !styled {
		m.theme = theme.Plain()
	}
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = update(t, m, searchDoneMsg{
		index:   ix,
		stderr:  []byte("pre" + fx.inject + "post\n"),
		waitErr: exitErr(t, 3),
	})
	return m.View().Content
}

// renderUsageErrorSink runs a hostile root operand through cli.Parse
// and returns the stderr composition cmd/vrg's run writes: the
// sanitized diagnostic line, a blank line, then the generated help.
func renderUsageErrorSink(t *testing.T, fx hostileFixture) string {
	t.Helper()
	res := cli.Parse([]string{"pat", "f" + fx.inject + ".txt"}, io.Discard, cli.Env{Stat: os.Stat})
	if res.Kind != cli.KindUsageError || res.ErrorKind != cli.ErrInvalidRoot {
		t.Fatalf("Parse of hostile root = kind %v err %v, want invalid-root usage error",
			res.Kind, res.ErrorKind)
	}
	return res.Diagnostic + "\n\n" + cli.HelpText()
}

// renderReplaySink drives the fixture through the stderr-replay sink:
// one collected diagnostic line carrying the injection as child stderr
// text and one diagnostic embedding it as a filename, replayed through
// the model's post-restoration writer.
func renderReplaySink(t *testing.T, fx hostileFixture) string {
	t.Helper()
	m := newModel(nil, nil)
	m, _ = update(t, m, diagMsg{line: "pre" + fx.inject + "post"})
	path := "f" + fx.inject + ".txt"
	req := mintLoad(&m, path)
	m, _ = update(t, m, loadDoneMsg{
		path: []byte(path),
		req:  req,
		err:  errors.New("denied"),
	})
	var buf bytes.Buffer
	m.ReplayTo(&buf)
	return buf.String()
}

// renderCLIHelpSink drives a hostile operand through cli.Parse on a
// help path — the generated command-line help's stdout sink — and
// returns the raw help output.
func renderCLIHelpSink(t *testing.T, fx hostileFixture) string {
	t.Helper()
	var out bytes.Buffer
	res := cli.Parse([]string{"f" + fx.inject + ".txt", "--help"}, &out, cli.Env{Stat: os.Stat})
	if res.Kind != cli.KindHelp {
		t.Fatalf("Parse hostile operand + --help = kind %v, want KindHelp", res.Kind)
	}
	return out.String()
}

// assertNoEscapeBytes requires the no-style raw output to carry no byte
// that could command a terminal: no ESC, BEL, raw CSI lead, C1 NEL, or
// bare CR. Asserted on the raw output before any ANSI stripping —
// stripping would erase the very evidence sought.
func assertNoEscapeBytes(t *testing.T, sink, raw string) {
	t.Helper()
	for _, bad := range []string{"\x1b", "\x07", "\x9b", "\xc2\x85", "\r"} {
		if strings.Contains(raw, bad) {
			t.Fatalf("%s: raw output contains byte %q: %q", sink, bad, raw)
		}
	}
}

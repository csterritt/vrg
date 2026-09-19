package app

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/cli"
	"vrg/internal/safepresentation"
	"vrg/internal/safepresentation/sinktest"
	"vrg/internal/theme"
)

// sinkSafetySinks is the shared sink-safety table (Issue #6): one row
// per output sink existing at this point — file-list entry, filename
// rule, panel content, the Issue #9 error overlay, usage-error stderr,
// and the Issue #1 generated command-line help on stdout (a sink
// distinct from the Issue #31 TUI help dialog, which adds its own row
// later). Every row renders through the no-style composition path so no
// escape byte may legitimately appear, and the TUI rows repeat under
// the styled theme so a fixture's payload can be checked against
// legitimate style sequences.
//
// Later issues that introduce sinks — stderr replay (#11), the pop-up
// (#15), the TUI help dialog (#31), generated documentation (#34) — add
// rows here, or in their own package's test file via sinktest.Run,
// reusing sinktest.Fixtures rather than duplicating them.
var sinkSafetySinks = []sinktest.Sink{
	{
		Name:         "file-list entry",
		Render:       func(t *testing.T, fx sinktest.Fixture) string { return pathFixtureView(t, fx, theme.Plain(), false) },
		RenderStyled: func(t *testing.T, fx sinktest.Fixture) string { return pathFixtureView(t, fx, theme.Dark(), false) },
	},
	{
		Name:         "filename rule",
		Render:       func(t *testing.T, fx sinktest.Fixture) string { return pathFixtureView(t, fx, theme.Plain(), true) },
		RenderStyled: func(t *testing.T, fx sinktest.Fixture) string { return pathFixtureView(t, fx, theme.Dark(), true) },
	},
	{
		Name:         "panel content",
		Render:       func(t *testing.T, fx sinktest.Fixture) string { return contentFixtureView(t, fx, theme.Plain()) },
		RenderStyled: func(t *testing.T, fx sinktest.Fixture) string { return contentFixtureView(t, fx, theme.Dark()) },
	},
	{
		Name:         "error overlay",
		Render:       func(t *testing.T, fx sinktest.Fixture) string { return overlayFixtureView(t, fx, theme.Plain()) },
		RenderStyled: func(t *testing.T, fx sinktest.Fixture) string { return overlayFixtureView(t, fx, theme.Dark()) },
	},
	{
		Name:   "usage-error stderr",
		Render: usageErrorStderr,
	},
	{
		Name:   "CLI-help stdout",
		Render: cliHelpStdout,
	},
	{
		Name:   "stderr replay",
		Render: replayFixtureOutput,
	},
}

// TestSinkSafetyTable drives the hostile fixture set through every sink
// row; sinktest.Run asserts on the raw output — before any ANSI
// stripping — that no fixture control byte survives, and under styles
// that no fixture payload follows an unescaped ESC.
func TestSinkSafetyTable(t *testing.T) {
	sinktest.Run(t, sinkSafetySinks)
}

// missingStat makes every root validation fail with fs.ErrNotExist so
// the usage-error row is deterministic whatever the fixture bytes form.
func missingStat(string) (fs.FileInfo, error) { return nil, fs.ErrNotExist }

// errFixtureRead is the fixed read failure behind the stderr-replay
// sink row.
var errFixtureRead = errors.New("read failed")

// pathFixtureView renders the browse view for a one-file index whose
// path carries the fixture bytes, on the given theme. The escaped name
// must appear in the filename rule (row 0) and in the file-list body —
// rule selects which region proves the sink carried the fixture.
func pathFixtureView(t *testing.T, fx sinktest.Fixture, th theme.Theme, rule bool) string {
	t.Helper()
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = th
	idx := browseIndex(t, []fixtureFile{
		{name: string(fx.Bytes), content: "x\n", line: 1, start: 0, end: 1},
	})
	cmd := startBrowse(t, m, idx)
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 50})
	finishLoad(t, m, cmd)
	v := viewText(m)
	row0, rest, _ := strings.Cut(v, "\n")
	region := rest
	if rule {
		region = row0
	}
	if want := safepresentation.EscapePath(fx.Bytes); !strings.Contains(region, want) {
		t.Fatalf("path fixture %q did not reach the sink: %q missing from %q", fx.Bytes, want, v)
	}
	return v
}

// contentFixtureView renders the browse view for a file whose matched
// content line carries the fixture bytes, on the given theme.
func contentFixtureView(t *testing.T, fx sinktest.Fixture, th theme.Theme) string {
	t.Helper()
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = th
	idx := browseIndex(t, []fixtureFile{
		{name: "target.txt", content: "pre" + string(fx.Bytes) + "post\n", line: 1, start: 0, end: 3},
	})
	cmd := startBrowse(t, m, idx)
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 50})
	finishLoad(t, m, cmd)
	v := viewText(m)
	for _, want := range contentFixtureForms(fx) {
		if !strings.Contains(v, want) {
			t.Fatalf("content fixture %q did not reach the sink: %q missing from %q", fx.Bytes, want, v)
		}
	}
	return v
}

// contentFixtureForms returns the display strings the panel must show
// for the fixture's bytes: each \n-separated piece is mapped like a
// source line, with an invalid byte's empty cell rendering U+FFFD — the
// same presentation contentText applies.
func contentFixtureForms(fx sinktest.Fixture) []string {
	var want []string
	for _, piece := range bytes.Split(fx.Bytes, []byte{'\n'}) {
		if len(piece) == 0 {
			continue
		}
		m := safepresentation.MapContent(piece)
		var s strings.Builder
		for i, c := range m.Cells {
			if c.Text != "" {
				s.WriteString(c.Text)
			} else if i == 0 || m.Cells[i-1].Start != c.Start || m.Cells[i-1].End != c.End {
				s.WriteString(string(rune(0xfffd)))
			}
		}
		want = append(want, s.String())
	}
	return want
}

// usageErrorStderr drives the fixture as the root operand through
// cli.Parse and returns the stderr block the entry point emits: the
// diagnostic line plus the appended generated help.
func usageErrorStderr(t *testing.T, fx sinktest.Fixture) string {
	t.Helper()
	res := cli.Parse([]string{"pat", string(fx.Bytes)}, io.Discard, cli.Env{Stat: missingStat})
	if res.Kind != cli.KindUsageError {
		t.Fatalf("Parse(pat %q).Kind = %v, want usage error", fx.Bytes, res.Kind)
	}
	out := res.Diagnostic + "\n\n" + cli.HelpText()
	if want := safepresentation.EscapePath(fx.Bytes); !strings.Contains(out, want) {
		t.Fatalf("usage-error fixture %q did not reach stderr: %q missing from %q", fx.Bytes, want, out)
	}
	return out
}

// cliHelpStdout renders the Issue #1 generated help on stdout while a
// hostile operand sits in argv. Generated help carries no external
// substitutions — the fixed name and descriptions cannot reflect the
// operand — so the row proves the sink's composition stays clean; the
// hostile-argv0 boundary test likewise proves argv0 cannot reach it.
func cliHelpStdout(t *testing.T, fx sinktest.Fixture) string {
	t.Helper()
	var out bytes.Buffer
	res := cli.Parse([]string{string(fx.Bytes), "-h"}, &out, cli.Env{Stat: missingStat})
	if res.Kind != cli.KindHelp {
		t.Fatalf("Parse(%q -h).Kind = %v, want help", fx.Bytes, res.Kind)
	}
	return out.String()
}

// replayFixtureOutput drives the fixture bytes through the
// stderr-replay sink's real composition path: a diagnostic embedding
// the fixture as a filename is collected into the session diagnostics
// and written by the common post-restoration writer.
func replayFixtureOutput(t *testing.T, fx sinktest.Fixture) string {
	t.Helper()
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.Update(fileLoadedMsg{path: fx.Bytes, err: errFixtureRead})
	var buf bytes.Buffer
	replayDiags(&buf, m.diags)
	out := buf.String()
	if want := safepresentation.EscapePath(fx.Bytes); !strings.Contains(out, want) {
		t.Fatalf("replay fixture %q did not reach the sink: %q missing from %q", fx.Bytes, want, out)
	}
	return out
}

// overlayFixtureView drives the fixture bytes through the error
// overlay's real composition path — captured child stderr on a failed
// process, escaped by the diagnostic utility — and returns the rendered
// frame, failing when no escaped piece of the fixture reached it.
func overlayFixtureView(t *testing.T, fx sinktest.Fixture, th theme.Theme) string {
	t.Helper()
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = th
	idx := browseIndex(t, []fixtureFile{
		{name: "target.txt", content: "x\n", line: 1, start: 0, end: 1},
	})
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 50})
	m.Update(searchDoneMsg{
		res: Result{Code: 3, Stderr: fx.Bytes},
		idx: idx,
	})
	if !m.overlayOpen {
		t.Fatalf("overlay did not open for the failed process")
	}
	v := viewText(m)
	for _, piece := range strings.Split(safepresentation.EscapeDiagnostic(string(fx.Bytes)), "\n") {
		if piece == "" {
			continue
		}
		if !strings.Contains(v, piece) {
			t.Fatalf("overlay fixture %q did not reach the sink: %q missing from %q", fx.Bytes, piece, v)
		}
	}
	return v
}

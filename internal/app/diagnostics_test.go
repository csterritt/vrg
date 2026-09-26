package app

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
	"vrg/internal/searchindex"
)

// diagCase is one exact row of the Issue #36 universal diagnostic
// composition: the structured outcome inputs a completed search
// presents, the complete ordered line list composeDiagnostics must
// produce for the overlay, and the complete ordered list
// completionDiagnostics must produce for the session collection — the
// same composition minus the child stderr the live collection already
// took.
type diagCase struct {
	name       string
	in         outcomeInput
	want       []string
	wantCollect []string
}

// TestComposedDiagnostics pins the universal component order shared by
// the overlay and the stderr replay: the process component first —
// collected stderr in collection order, or the generated exit-code /
// signal line only when a fatal process result carried no stderr, and
// never a process line for the clean 0 or benign 1 — then the
// integrity causes, then the record-loss components (malformed
// aggregate, oversized aggregate, per-path oversized details), then
// the unrecognised-type warnings.
func TestComposedDiagnostics(t *testing.T) {
	cases := []diagCase{
		{
			name: "clean search composes nothing",
		},
		{
			// A warning under a clean exit: the stderr component
			// alone, and nothing further to collect at completion —
			// the live collection already holds the line.
			name: "stderr warning under exit 0",
			in:   outcomeInput{stderr: []byte("rg: warn: odd\n")},
			want: []string{"rg: warn: odd"},
		},
		{
			// The benign no-matches status never earns a process
			// line; its stderr still composes as the process
			// component.
			name: "stderr under benign exit 1",
			in:   outcomeInput{stderr: []byte("rg: warn: odd\n")},
			want: []string{"rg: warn: odd"},
		},
		{
			// Explanatory stderr takes precedence over the generated
			// line: a fatal code with stderr composes the stderr
			// alone.
			name: "fatal code with stderr composes the stderr only",
			in: outcomeInput{
				waitErr: exitErr(t, 3),
				stderr:  []byte("rg: something broke\nrg: giving up\n"),
			},
			want: []string{"rg: something broke", "rg: giving up"},
		},
		{
			// Without explanatory stderr the generated line names
			// the exit code — the only process-status line the
			// composition can produce.
			name: "fatal code without stderr generates the exit line",
			in:   outcomeInput{waitErr: exitErr(t, 3)},
			want: []string{"rg failed: exit status 3"},
			wantCollect: []string{"rg failed: exit status 3"},
		},
		{
			name: "signal death without stderr generates the signal line",
			in:   outcomeInput{waitErr: sigErr(t)},
			want: []string{"rg failed: signal: killed"},
			wantCollect: []string{"rg failed: signal: killed"},
		},
		{
			// A clean exit with a damaged stream: no process-status
			// line — never "ripgrep exited with code 0" — just the
			// cause.
			name: "integrity failure under exit 0 names only the cause",
			in: outcomeInput{
				causes: []searchindex.Cause{{Kind: searchindex.CauseMissingSummary}},
			},
			want:        []string{"missing summary"},
			wantCollect: []string{"missing summary"},
		},
		{
			// Nor "ripgrep exited with code 1": the benign status is
			// not a process component.
			name: "integrity failure under exit 1 names only the cause",
			in: outcomeInput{
				waitErr: exitErr(t, 1),
				causes:  []searchindex.Cause{{Kind: searchindex.CauseMissingEnd, Path: []byte("f.txt")}},
			},
			want:        []string{"missing end for f.txt"},
			wantCollect: []string{"missing end for f.txt"},
		},
		{
			// Every component in one composition: stderr lines in
			// collection order, the causes in cause order, the
			// malformed aggregate, the oversized aggregate, the
			// per-path oversized details, then the unknown-type
			// warning.
			name: "the universal order across all components",
			in: outcomeInput{
				stderr: []byte("rg: one\nrg: two\n"),
				causes: []searchindex.Cause{
					{Kind: searchindex.CauseOrphanedMatch, Path: []byte("a.txt")},
					{Kind: searchindex.CauseOrphanedEnd, Path: []byte("b.txt")},
				},
				malformed: 2,
				oversized: 2,
				oversizedDiags: []string{
					"oversized record skipped for big.bin",
					"oversized record skipped for huge.bin",
				},
				unknown: 3,
			},
			want: []string{
				"rg: one",
				"rg: two",
				"orphaned match for a.txt",
				"orphaned end for b.txt",
				"2 malformed records skipped",
				"2 oversized records skipped",
				"oversized record skipped for big.bin",
				"oversized record skipped for huge.bin",
				"3 unrecognised record types skipped",
			},
			wantCollect: []string{
				"orphaned match for a.txt",
				"orphaned end for b.txt",
				"2 malformed records skipped",
				"2 oversized records skipped",
				"oversized record skipped for big.bin",
				"oversized record skipped for huge.bin",
				"3 unrecognised record types skipped",
			},
		},
		{
			// The post-summary dual representation: each offending
			// record carries only its record-after-summary cause while
			// the independent malformed, oversized, and unknown
			// tallies still compose their own components — the
			// recovered oversized path detail included.
			name: "post-summary records keep their independent tallies",
			in: outcomeInput{
				causes: []searchindex.Cause{
					{Kind: searchindex.CauseRecordAfterSummary},
					{Kind: searchindex.CauseRecordAfterSummary},
					{Kind: searchindex.CauseRecordAfterSummary},
				},
				malformed:      1,
				oversized:      1,
				oversizedDiags: []string{"oversized record skipped for big.txt"},
				unknown:        1,
			},
			want: []string{
				"record after summary",
				"record after summary",
				"record after summary",
				"1 malformed record skipped",
				"1 oversized record skipped",
				"oversized record skipped for big.txt",
				"1 unrecognised record types skipped",
			},
			wantCollect: []string{
				"record after summary",
				"record after summary",
				"record after summary",
				"1 malformed record skipped",
				"1 oversized record skipped",
				"oversized record skipped for big.txt",
				"1 unrecognised record types skipped",
			},
		},
		{
			// One cause per offending record — repeated identical
			// violations are neither aggregated nor capped.
			name: "repeated violations compose one line each",
			in: outcomeInput{
				causes: []searchindex.Cause{
					{Kind: searchindex.CauseOrphanedMatch, Path: []byte("a.txt")},
					{Kind: searchindex.CauseOrphanedMatch, Path: []byte("a.txt")},
					{Kind: searchindex.CauseOrphanedMatch, Path: []byte("a.txt")},
				},
			},
			want: []string{
				"orphaned match for a.txt",
				"orphaned match for a.txt",
				"orphaned match for a.txt",
			},
			wantCollect: []string{
				"orphaned match for a.txt",
				"orphaned match for a.txt",
				"orphaned match for a.txt",
			},
		},
		{
			// An embedded newline in a cause's path cannot forge a
			// paragraph break: the path utility's single-line escaped
			// form is composed verbatim.
			name: "a newline path composes one escaped line",
			in: outcomeInput{
				causes: []searchindex.Cause{
					{Kind: searchindex.CauseMissingEnd, Path: []byte("a\nb.txt")},
				},
			},
			want:        []string{`missing end for a\nb.txt`},
			wantCollect: []string{`missing end for a\nb.txt`},
		},
		{
			name: "unknown-type warnings alone",
			in:   outcomeInput{unknown: 2},
			want: []string{"2 unrecognised record types skipped"},
			wantCollect: []string{"2 unrecognised record types skipped"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := composeDiagnostics(tc.in); !slices.Equal(got, tc.want) {
				t.Fatalf("composeDiagnostics = %q, want %q", got, tc.want)
			}
			if got := completionDiagnostics(tc.in); !slices.Equal(got, tc.wantCollect) {
				t.Fatalf("completionDiagnostics = %q, want %q", got, tc.wantCollect)
			}
			// The overlay carries the composed list verbatim —
			// overlay and replay are one composition.
			if o := decideOutcome(tc.in); !slices.Equal(o.diags, tc.want) {
				t.Fatalf("decideOutcome diags = %q, want %q", o.diags, tc.want)
			}
		})
	}
}

// The structured input fixes the same outcome the string inputs did:
// fatal process result, any integrity cause, or record loss with zero
// usable results is status 2; usable results are 0; an intact empty
// stream is 1. Unknown-type warnings and stderr alone are diagnostics
// only — never fatal.
func TestOutcomeCodesFromInput(t *testing.T) {
	cases := []struct {
		name    string
		in      outcomeInput
		code    int
		screen  phase
		overlay bool
	}{
		{
			name:   "clean stream with usable results",
			in:     outcomeInput{usable: 3},
			code:   0, screen: phaseBrowse,
		},
		{
			name:   "anomalous rg 1 with usable results",
			in:     outcomeInput{waitErr: exitErr(t, 1), usable: 1},
			code:   0, screen: phaseBrowse,
		},
		{
			name:   "complete stream with no usable results",
			in:     outcomeInput{},
			code:   1, screen: phaseNoResults,
		},
		{
			name:   "stderr warning alone is not fatal",
			in:     outcomeInput{stderr: []byte("rg: warn\n")},
			code:   1, screen: phaseNoResults, overlay: true,
		},
		{
			name:   "fatal code with usable results",
			in:     outcomeInput{waitErr: exitErr(t, 3), usable: 2},
			code:   2, screen: phaseBrowse, overlay: true,
		},
		{
			name:   "fatal code without usable results",
			in:     outcomeInput{waitErr: exitErr(t, 3)},
			code:   2, screen: phaseFatal, overlay: true,
		},
		{
			name: "integrity cause with usable results",
			in: outcomeInput{
				usable: 1,
				causes: []searchindex.Cause{{Kind: searchindex.CauseMissingSummary}},
			},
			code: 2, screen: phaseBrowse, overlay: true,
		},
		{
			name: "record loss with usable results warns only",
			in:   outcomeInput{usable: 1, malformed: 1},
			code: 0, screen: phaseBrowse, overlay: true,
		},
		{
			name:   "malformed loss leaving no usable results is fatal",
			in:     outcomeInput{malformed: 1},
			code:   2, screen: phaseFatal, overlay: true,
		},
		{
			name:   "oversized loss leaving no usable results is fatal",
			in:     outcomeInput{oversized: 1},
			code:   2, screen: phaseFatal, overlay: true,
		},
		{
			name:   "unknown types alone never turn fatal",
			in:     outcomeInput{unknown: 2},
			code:   1, screen: phaseNoResults, overlay: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := decideOutcome(tc.in)
			if o.code != tc.code {
				t.Fatalf("code = %d, want %d", o.code, tc.code)
			}
			if o.screen != tc.screen {
				t.Fatalf("screen = %d, want %d", o.screen, tc.screen)
			}
			if o.overlay != tc.overlay {
				t.Fatalf("overlay = %v, want %v", o.overlay, tc.overlay)
			}
		})
	}
}

// The composition consumes the built index's own accessors: two files
// left open report their missing ends ordered by unsigned raw path
// bytes — a.txt before the first-opened z.txt — and the order is
// stable across repeated builds.
func TestComposedDiagnosticsFromIndexOrdering(t *testing.T) {
	dir := t.TempDir()
	want := []string{
		"missing end for a.txt",
		"missing end for z.txt",
	}
	for i := 0; i < 20; i++ {
		ix := fixtureIndex(t, dir,
			`{"type":"begin","data":{"path":{"text":"z.txt"}}}`,
			`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
			recSummary,
		)
		in := outcomeInput{
			causes:         ix.IntegrityCauses(),
			usable:         len(ix.Stops()),
			malformed:      ix.Malformed(),
			oversized:      ix.Oversized(),
			oversizedDiags: ix.OversizedDiagnostics(),
			unknown:        ix.Unknown(),
		}
		if got := composeDiagnostics(in); !slices.Equal(got, want) {
			t.Fatalf("build %d: composeDiagnostics = %q, want %q", i, got, want)
		}
	}
}

// One composition feeds both sinks: the overlay's lines and the exit
// replay carry the same text — here the live-collected stderr warning
// plus the completion's missing-summary cause, with no generated
// process line for the clean exit.
func TestOverlayAndReplayShareComposedDiagnostics(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "f.txt", "hit\n")
	m := newModel(nil, nil)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = update(t, m, diagMsg{line: "rg: warn: odd"})
	m, _ = update(t, m, searchDoneMsg{
		index:  fixtureIndex(t, dir, recsMissingSummary...),
		stderr: []byte("rg: warn: odd\n"),
	})
	want := []string{"rg: warn: odd", "missing summary"}
	if m.overlay == nil {
		t.Fatal("the completion diagnostics did not open the overlay")
	}
	if !slices.Equal(m.overlay.lines, want) {
		t.Fatalf("overlay lines = %q, want %q", m.overlay.lines, want)
	}

	m, _ = pressKey(t, m, "q") // dismiss over browse
	m, cmd := pressKey(t, m, "q")
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("q on browse command = %T, want tea.QuitMsg", cmd())
	}
	if got := replayed(t, m); !slices.Equal(got, want) {
		t.Fatalf("replayed diagnostics = %q, want %q", got, want)
	}
}

// The fatal overlay keeps every component: real child stderr, the
// integrity causes, and the record-loss tallies appear together —
// the fatal process result suppresses nothing and earns no generated
// line while the stderr explains the failure.
func TestFatalOverlayKeepsStderrCausesAndRecordLoss(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "f.txt", "hit\n")
	stream := streamLines(
		`{"type":"begin","data":{"path":{"text":"f.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"f.txt"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`this is not json`,
		`{"type":"end","data":{"path":{"text":"f.txt"},"binary_offset":null}}`,
		recSummary,
		`{"type":"weird","data":{"x":1}}`,
	)
	m := newModel(nil, nil)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = update(t, m, diagMsg{line: "rg: real stderr"})
	m, _ = update(t, m, searchDoneMsg{
		index:   fixtureStream(t, dir, stream),
		stderr:  []byte("rg: real stderr\n"),
		waitErr: exitErr(t, 3),
	})
	want := []string{
		"rg: real stderr",
		"record after summary",
		"1 malformed record skipped",
		"1 unrecognised record types skipped",
	}
	if m.overlay == nil {
		t.Fatal("the fatal completion did not open the overlay")
	}
	if !slices.Equal(m.overlay.lines, want) {
		t.Fatalf("overlay lines = %q, want %q", m.overlay.lines, want)
	}
	for _, line := range m.overlay.lines {
		if line == "rg failed: exit status 3" {
			t.Fatalf("generated process line appeared alongside stderr: %q", m.overlay.lines)
		}
	}
}

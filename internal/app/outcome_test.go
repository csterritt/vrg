package app

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/searchindex"
)

// completeStream is the stream-integrity value a directly-built fixture
// index stands in for: fixtures never pass through a stream, so tests
// declare the stream complete explicitly.
var completeStream = searchindex.Integrity{Complete: true}

// signalError is a fake wait status for a child that died by signal:
// *exec.ExitError reports ExitCode -1 in that case.
type signalError string

func (e signalError) Error() string { return string(e) }
func (e signalError) ExitCode() int { return -1 }

func beginRec(path string) string {
	return fmt.Sprintf(`{"type":"begin","data":{"path":{"text":%q}}}`, path)
}

func summaryRec() string {
	return `{"type":"summary","data":{}}`
}

// keyPress maps a key name to its press message: "q" and other text keys
// through the text form, named keys through their code.
func keyPress(key string) tea.KeyPressMsg {
	switch key {
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	default:
		return keyMsg(key)
	}
}

func requireQuit(t *testing.T, cmd tea.Cmd, what string) {
	t.Helper()
	if cmd == nil {
		t.Fatalf("%s returned no command, want tea.Quit", what)
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("%s returned %T, want tea.QuitMsg", what, cmd())
	}
}

// outcomeRow is one row of the outcome/transition matrix: the stream
// records fed through the Builder, the child's wait status and captured
// stderr, then the expected initial presentation, overlay dismissal
// behavior, and final exit status.
type outcomeRow struct {
	name    string
	records []string
	procErr error
	stderr  string

	wantState   state
	wantOverlay bool
	contains    []string
	absent      []string

	// dismiss is the key that dismisses the initial overlay ("q" or
	// "esc"); dismissQuits marks the fatal no-results overlay whose
	// dismissal exits because there is no underlying state.
	dismiss      string
	dismissQuits bool
	postContains []string
	postAbsent   []string

	// final is the key that ends a still-running row; empty when the
	// dismissal already quit.
	final      string
	wantStatus int
}

// Record-loss and unknown-type accounting composes into overlay
// diagnostic lines: the pluralized malformed and oversized aggregates,
// each recoverable oversized path escaped to a single line, and the
// unknown-type warning.
func TestRecordLossDiagnostics(t *testing.T) {
	in := outcomeInput{
		integrity: completeStream,
		report: searchindex.Report{
			Malformed:    1,
			Oversized:    2,
			UnknownTypes: 3,
			OversizedPaths: [][]byte{
				[]byte("big.txt"),
				[]byte("a\nb.txt"),
			},
		},
	}
	want := []string{
		"1 malformed record skipped",
		"2 oversized records skipped",
		"oversized record skipped for big.txt",
		`oversized record skipped for a\nb.txt`,
		"3 unrecognised record types skipped",
	}
	if got := diagnosticLines(in); !slices.Equal(got, want) {
		t.Fatalf("diagnosticLines() = %q, want %q", got, want)
	}
}

// TestOutcomeMatrix is the single table-driven outcome matrix of Issue
// 9: every outcome-table row asserting initial presentation, dismissal
// behavior, and final exit status. Later issues extend it with rows.
func TestOutcomeMatrix(t *testing.T) {
	valid := []string{
		beginRec("a.txt"),
		matchRec("a.txt", "hit\n", 1, 0, 3, "hit"),
		endRec("a.txt", nil),
		summaryRec(),
	}
	summaryOnly := []string{summaryRec()}
	allBinary := []string{
		beginRec("a.bin"),
		matchRec("a.bin", "x\n", 1, 0, 1, "x"),
		endRec("a.bin", 7),
		summaryRec(),
	}

	cases := []outcomeRow{
		{
			name:      "rg 0 clean stream browses and exits 0",
			records:   valid,
			wantState: stateBrowse,
			contains:  []string{"a.txt"},
			absent:    []string{"┌"},
			final:     "q", wantStatus: 0,
		},
		{
			name:    "rg 1 with retained results and complete stream browses and exits 0",
			records: valid, procErr: exitError(1),
			wantState: stateBrowse,
			contains:  []string{"a.txt"},
			absent:    []string{"┌"},
			final:     "q", wantStatus: 0,
		},
		{
			name:    "rg 1 empty shows no results and exits 1",
			records: summaryOnly, procErr: exitError(1),
			wantState: stateNoResults,
			contains:  []string{"No results found"},
			absent:    []string{"┌"},
			final:     "q", wantStatus: 1,
		},
		{
			name:    "rg 2 with usable results browses under the error overlay",
			records: valid, procErr: exitError(2), stderr: "boom\n",
			wantState: stateBrowse, wantOverlay: true,
			contains:     []string{"boom", "a.txt"},
			dismiss:      "esc",
			postContains: []string{"a.txt"},
			postAbsent:   []string{"boom"},
			final:        "q", wantStatus: 2,
		},
		{
			name:    "rg 2 without usable results dismissed with q exits 2",
			records: summaryOnly, procErr: exitError(2), stderr: "boom\n",
			wantState: stateFatal, wantOverlay: true,
			contains: []string{"boom"},
			absent:   []string{"No results found"},
			dismiss:  "q", dismissQuits: true,
			wantStatus: 2,
		},
		{
			name:    "rg 2 without usable results dismissed with esc exits 2",
			records: summaryOnly, procErr: exitError(2), stderr: "boom\n",
			wantState: stateFatal, wantOverlay: true,
			contains: []string{"boom"},
			absent:   []string{"No results found"},
			dismiss:  "esc", dismissQuits: true,
			wantStatus: 2,
		},
		{
			name:    "signal death with usable results browses under the error overlay",
			records: valid, procErr: signalError("signal: killed"),
			wantState: stateBrowse, wantOverlay: true,
			contains:     []string{"killed"},
			dismiss:      "q",
			postContains: []string{"a.txt"},
			final:        "q", wantStatus: 2,
		},
		{
			name:    "signal death without usable results exits 2 on dismissal",
			records: summaryOnly, procErr: signalError("signal: killed"),
			wantState: stateFatal, wantOverlay: true,
			contains: []string{"signal"},
			dismiss:  "esc", dismissQuits: true,
			wantStatus: 2,
		},
		{
			name: "missing summary with valid matches is fatal",
			records: []string{
				beginRec("a.txt"),
				matchRec("a.txt", "hit\n", 1, 0, 3, "hit"),
				endRec("a.txt", nil),
			},
			wantState: stateBrowse, wantOverlay: true,
			contains:     []string{"incomplete", "a.txt"},
			dismiss:      "esc",
			postContains: []string{"a.txt"},
			final:        "q", wantStatus: 2,
		},
		{
			name: "orphaned end with valid matches is fatal",
			records: []string{
				beginRec("a.txt"),
				matchRec("a.txt", "hit\n", 1, 0, 3, "hit"),
				endRec("a.txt", nil),
				endRec("b.txt", nil), // orphaned: never opened
				summaryRec(),
			},
			wantState: stateBrowse, wantOverlay: true,
			contains: []string{"a.txt"},
			dismiss:  "esc",
			final:    "q", wantStatus: 2,
		},
		{
			name:    "stderr warning with results warns over browse and exits 0",
			records: valid, stderr: "warn\n",
			wantState: stateBrowse, wantOverlay: true,
			contains:     []string{"warn", "a.txt"},
			dismiss:      "esc",
			postContains: []string{"a.txt"},
			postAbsent:   []string{"warn"},
			final:        "q", wantStatus: 0,
		},
		{
			name:    "stderr warning with zero results and a complete stream warns then exits 1",
			records: summaryOnly, procErr: exitError(1), stderr: "warn\n",
			wantState: stateNoResults, wantOverlay: true,
			// The centred box covers the centred no-results line; the
			// state field and the post-dismissal view carry the
			// warning-vs-fatal distinction.
			contains:     []string{"warn"},
			dismiss:      "esc",
			postContains: []string{"No results found"},
			postAbsent:   []string{"warn"},
			final:        "q", wantStatus: 1,
		},
		{
			name:    "all binary after a warning warns then shows the skipped count",
			records: allBinary, stderr: "warn\n",
			wantState: stateNoResults, wantOverlay: true,
			contains:     []string{"warn"},
			dismiss:      "esc",
			postContains: []string{"No results found (1 binary files skipped)"},
			final:        "q", wantStatus: 1,
		},
		{
			name:      "unknown-type warnings alone with zero results warn then exit 1",
			records:   []string{`{"type":"weird","data":{}}`, `{"type":"stats","data":{}}`, summaryRec()},
			wantState: stateNoResults, wantOverlay: true,
			contains:     []string{"2 unrecognised record types skipped"},
			dismiss:      "esc",
			postContains: []string{"No results found"},
			postAbsent:   []string{"unrecognised"},
			final:        "q", wantStatus: 1,
		},
		{
			name: "skipped malformed record with usable results browses with the overlay and exits 0",
			records: []string{
				beginRec("a.txt"),
				matchRec("a.txt", "hit\n", 1, 0, 3, "hit"),
				`{not json`,
				endRec("a.txt", nil),
				summaryRec(),
			},
			wantState: stateBrowse, wantOverlay: true,
			contains:     []string{"1 malformed record skipped", "a.txt"},
			dismiss:      "esc",
			postContains: []string{"a.txt"},
			final:        "q", wantStatus: 0,
		},
		{
			name: "skipped malformed record with zero usable results is fatal record loss dismissed with q",
			records: []string{
				`{not json`,
				summaryRec(),
			},
			wantState: stateFatal, wantOverlay: true,
			contains: []string{"1 malformed record skipped"},
			absent:   []string{"No results found"},
			dismiss:  "q", dismissQuits: true,
			wantStatus: 2,
		},
		{
			name: "skipped malformed record with zero usable results is fatal record loss dismissed with esc",
			records: []string{
				`{not json`,
				summaryRec(),
			},
			procErr:   exitError(1),
			wantState: stateFatal, wantOverlay: true,
			contains: []string{"1 malformed record skipped"},
			dismiss:  "esc", dismissQuits: true,
			wantStatus: 2,
		},
		{
			name: "skipped record and binary exclusion leaving zero retained stops is fatal record loss",
			records: []string{
				beginRec("a.bin"),
				matchRec("a.bin", "x\n", 1, 0, 1, "x"),
				`{not json`,
				endRec("a.bin", 7),
				summaryRec(),
			},
			wantState: stateFatal, wantOverlay: true,
			contains: []string{"1 malformed record skipped"},
			dismiss:  "esc", dismissQuits: true,
			wantStatus: 2,
		},
		{
			name: "missing end with retained matches browses with the overlay and exits 2",
			records: []string{
				beginRec("a.txt"),
				matchRec("a.txt", "hit\n", 1, 0, 3, "hit"),
				summaryRec(),
			},
			wantState: stateBrowse, wantOverlay: true,
			contains:     []string{"incomplete", "a.txt"},
			dismiss:      "esc",
			postContains: []string{"a.txt"},
			final:        "q", wantStatus: 2,
		},
		{
			name: "missing end with no matches is fatal",
			records: []string{
				beginRec("a.txt"),
				summaryRec(),
			},
			wantState: stateFatal, wantOverlay: true,
			contains: []string{"incomplete"},
			dismiss:  "q", dismissQuits: true,
			wantStatus: 2,
		},
		{
			name:         "esc in a base state never exits",
			records:      valid,
			wantState:    stateBrowse,
			contains:     []string{"a.txt"},
			dismiss:      "esc", // no overlay open: the key is a no-op
			postContains: []string{"a.txt"},
			final:        "q", wantStatus: 0,
		},
		{
			name:      "ctrl+c after completion in browse exits 130",
			records:   valid,
			wantState: stateBrowse,
			final:     "ctrl+c", wantStatus: 130,
		},
		{
			name:    "ctrl+c after completion on no-results exits 130",
			records: summaryOnly, procErr: exitError(1),
			wantState: stateNoResults,
			final:     "ctrl+c", wantStatus: 130,
		},
		{
			name:    "ctrl+c with the error overlay open exits 130",
			records: summaryOnly, procErr: exitError(2), stderr: "boom\n",
			wantState: stateFatal, wantOverlay: true,
			final: "ctrl+c", wantStatus: 130,
		},
		{
			name:    "ctrl+c with the warning overlay open exits 130",
			records: valid, stderr: "warn\n",
			wantState: stateBrowse, wantOverlay: true,
			final: "ctrl+c", wantStatus: 130,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := searchindex.NewBuilder("/w")
			var stream strings.Builder
			for _, r := range tc.records {
				stream.WriteString(r)
				stream.WriteByte('\n')
			}
			b.Consume(strings.NewReader(stream.String()))
			idx, integrity := b.Finish()

			m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, "/w")
			m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
			m = feedStderr(t, m, tc.stderr)
			m, _ = update(t, m, searchResult{
				index: idx, integrity: integrity, report: b.Report(),
				err: tc.procErr,
			})

			if m.state != tc.wantState {
				t.Fatalf("initial state = %d, want %d", m.state, tc.wantState)
			}
			if (m.overlay != nil) != tc.wantOverlay {
				t.Fatalf("overlay open = %v, want %v", m.overlay != nil, tc.wantOverlay)
			}
			v := m.View().Content
			for _, want := range tc.contains {
				if !strings.Contains(v, want) {
					t.Fatalf("initial view lacks %q:\n%s", want, v)
				}
			}
			for _, not := range tc.absent {
				if strings.Contains(v, not) {
					t.Fatalf("initial view contains %q:\n%s", not, v)
				}
			}

			if tc.dismiss != "" {
				var cmd tea.Cmd
				m, cmd = update(t, m, keyPress(tc.dismiss))
				if tc.dismissQuits {
					requireQuit(t, cmd, tc.dismiss+" dismissing the fatal overlay")
					if m.status != tc.wantStatus {
						t.Fatalf("exit status = %d, want %d", m.status, tc.wantStatus)
					}
					return
				}
				if cmd != nil {
					t.Fatalf("%s dismissal returned a command: %v", tc.dismiss, cmd)
				}
				if m.overlay != nil {
					t.Fatal("overlay still open after dismissal")
				}
				v = m.View().Content
				for _, want := range tc.postContains {
					if !strings.Contains(v, want) {
						t.Fatalf("post-dismissal view lacks %q:\n%s", want, v)
					}
				}
				for _, not := range tc.postAbsent {
					if strings.Contains(v, not) {
						t.Fatalf("post-dismissal view contains %q:\n%s", not, v)
					}
				}
			}

			if tc.final == "" {
				return
			}
			var cmd tea.Cmd
			m, cmd = update(t, m, keyPress(tc.final))
			requireQuit(t, cmd, tc.final)
			if m.status != tc.wantStatus {
				t.Fatalf("exit status = %d, want %d", m.status, tc.wantStatus)
			}
		})
	}
}

package app

import (
	"os/exec"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// sigErr returns a child wait error for a signal death, produced by a
// real process killing itself with SIGKILL so the error is the
// *exec.ExitError collect delivers — ExitCode -1, ProcessState naming
// the signal.
func sigErr(t *testing.T) error {
	t.Helper()
	err := exec.Command("sh", "-c", "kill -9 $$").Run()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != -1 {
		t.Fatalf("signal fixture = %v, want a signal death", err)
	}
	return err
}

// Stream fixtures shared by the outcome matrix. Every match stream is
// lifecycle-valid: begin, matches, end — then the final summary.
var (
	recSummary = `{"type":"summary","data":{}}`

	recsOneMatch = []string{
		`{"type":"begin","data":{"path":{"text":"f.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"f.txt"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"end","data":{"path":{"text":"f.txt"},"binary_offset":null}}`,
		recSummary,
	}
	recsSummaryOnly = []string{recSummary}
	// Missing summary: well-formed records but the stream never closed.
	recsMissingSummary = recsOneMatch[:3]
	// Orphaned end: g.txt closes without ever opening.
	recsOrphanedEnd = []string{
		`{"type":"begin","data":{"path":{"text":"f.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"f.txt"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"end","data":{"path":{"text":"f.txt"},"binary_offset":null}}`,
		`{"type":"end","data":{"path":{"text":"g.txt"},"binary_offset":null}}`,
		recSummary,
	}
	// A file left open when the stream ends.
	recsOpenAtEnd = []string{
		`{"type":"begin","data":{"path":{"text":"f.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"f.txt"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		recSummary,
	}
	// Every matched file excluded as binary: usable results zero.
	recsAllBinary = []string{
		`{"type":"begin","data":{"path":{"text":"b.bin"}}}`,
		`{"type":"match","data":{"path":{"text":"b.bin"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"end","data":{"path":{"text":"b.bin"},"binary_offset":4}}`,
		recSummary,
	}
)

// outcomeRow is one row of the Issue #9 outcome-transition matrix: the
// inputs a completed search presents — stream records, the child's wait
// status, and its stderr — and the full contract that follows: the
// initial presentation (underlying screen plus whether the diagnostics
// overlay opens), the substrings the overlay must carry, which key
// dismisses it, the post-dismissal state, the key that ends the
// session, and the final exit status.
type outcomeRow struct {
	name      string
	recs      []string
	code      int      // child exit status; -1 selects the signal-death fixture
	stderr    string   // captured child stderr
	overlay   bool     // the diagnostics overlay opens on completion
	screen    phase    // the underlying screen while the overlay is open, or alone
	dismiss   string   // key that dismisses the overlay; "" when none opens
	after     phase    // underlying screen after dismissal (ignored when afterQuit)
	afterQuit bool     // dismissal itself ended the session — the fatal no-results case
	quitKey   string   // key that ends the session once no overlay is open
	exit      int      // final process exit status
	diags     []string // substrings the open overlay must carry
}

// The outcome matrix: one table owning every Issue #9 outcome row so
// later issues extend it by adding rows rather than duplicating the
// decision logic. Process result, stream integrity, and usable results
// are assessed independently; their combination fixes the presentation
// and the exit status exactly once, with ctrl+c the only override.
var outcomeMatrix = []outcomeRow{
	{
		name: "rg 0 clean complete stream browses to exit 0",
		recs: recsOneMatch, code: 0,
		screen:  phaseBrowse,
		quitKey: "q", exit: 0,
	},
	{
		name: "rg 0 summary-only stream is no-results exit 1",
		recs: recsSummaryOnly, code: 0,
		screen:  phaseNoResults,
		quitKey: "q", exit: 1,
	},
	{
		name: "anomalous rg 1 with retained results browses to exit 0",
		recs: recsOneMatch, code: 1,
		screen:  phaseBrowse,
		quitKey: "q", exit: 0,
	},
	{
		name: "rg 1 empty complete stream is no-results exit 1",
		recs: recsSummaryOnly, code: 1,
		screen:  phaseNoResults,
		quitKey: "q", exit: 1,
	},
	{
		name: "fatal code with usable results overlays browse, q dismissal, exit 2",
		recs: recsOneMatch, code: 3,
		overlay: true, screen: phaseBrowse,
		dismiss: "q", after: phaseBrowse,
		quitKey: "q", exit: 2,
		diags: []string{"exit status 3"},
	},
	{
		name: "fatal code with usable results overlays browse, Esc dismissal, exit 2",
		recs: recsOneMatch, code: 3,
		overlay: true, screen: phaseBrowse,
		dismiss: "esc", after: phaseBrowse,
		quitKey: "q", exit: 2,
		diags: []string{"exit status 3"},
	},
	{
		name: "fatal code without usable results is overlay-only, q exits 2",
		recs: recsSummaryOnly, code: 3,
		overlay: true, screen: phaseFatal,
		dismiss: "q", afterQuit: true, exit: 2,
		diags: []string{"exit status 3"},
	},
	{
		name: "fatal code without usable results is overlay-only, Esc exits 2",
		recs: recsSummaryOnly, code: 3,
		overlay: true, screen: phaseFatal,
		dismiss: "esc", afterQuit: true, exit: 2,
		diags: []string{"exit status 3"},
	},
	{
		name: "signal death with usable results overlays browse, exit 2",
		recs: recsOneMatch, code: -1,
		overlay: true, screen: phaseBrowse,
		dismiss: "esc", after: phaseBrowse,
		quitKey: "q", exit: 2,
		diags: []string{"signal: killed"},
	},
	{
		name: "signal death without usable results is overlay-only, exit 2",
		recs: recsSummaryOnly, code: -1,
		overlay: true, screen: phaseFatal,
		dismiss: "q", afterQuit: true, exit: 2,
		diags: []string{"signal: killed"},
	},
	{
		name: "missing summary with valid matches overlays browse, exit 2",
		recs: recsMissingSummary, code: 0,
		overlay: true, screen: phaseBrowse,
		dismiss: "q", after: phaseBrowse,
		quitKey: "q", exit: 2,
		diags: []string{"missing summary"},
	},
	{
		name: "orphaned end with valid matches overlays browse, exit 2",
		recs: recsOrphanedEnd, code: 0,
		overlay: true, screen: phaseBrowse,
		dismiss: "esc", after: phaseBrowse,
		quitKey: "q", exit: 2,
		diags: []string{"orphaned end"},
	},
	{
		name: "file left open at stream end overlays browse, exit 2",
		recs: recsOpenAtEnd, code: 0,
		overlay: true, screen: phaseBrowse,
		dismiss: "q", after: phaseBrowse,
		quitKey: "q", exit: 2,
		diags: []string{"missing end"},
	},
	{
		name: "stderr warning with results under rg 0 overlays browse, exit 0",
		recs: recsOneMatch, code: 0, stderr: "rg: warn: something odd\n",
		overlay: true, screen: phaseBrowse,
		dismiss: "esc", after: phaseBrowse,
		quitKey: "q", exit: 0,
		diags: []string{"something odd"},
	},
	{
		name: "stderr warning with results under rg 1 overlays browse, exit 0",
		recs: recsOneMatch, code: 1, stderr: "rg: warn: something odd\n",
		overlay: true, screen: phaseBrowse,
		dismiss: "q", after: phaseBrowse,
		quitKey: "q", exit: 0,
		diags: []string{"something odd"},
	},
	{
		name: "stderr warning with zero results under rg 0 overlays no-results, exit 1",
		recs: recsSummaryOnly, code: 0, stderr: "rg: warn: something odd\n",
		overlay: true, screen: phaseNoResults,
		dismiss: "q", after: phaseNoResults,
		quitKey: "q", exit: 1,
		diags: []string{"something odd"},
	},
	{
		name: "stderr warning with zero results under rg 1 overlays no-results, exit 1",
		recs: recsSummaryOnly, code: 1, stderr: "rg: warn: something odd\n",
		overlay: true, screen: phaseNoResults,
		dismiss: "esc", after: phaseNoResults,
		quitKey: "q", exit: 1,
		diags: []string{"something odd"},
	},
	{
		name: "all-binary after a warning lands on no-results with the count, exit 1",
		recs: recsAllBinary, code: 1, stderr: "rg: warn: something odd\n",
		overlay: true, screen: phaseNoResults,
		dismiss: "q", after: phaseNoResults,
		quitKey: "q", exit: 1,
		diags: []string{"something odd"},
	},
	{
		name: "ctrl+c in browse overrides the fixed status with 130",
		recs: recsOneMatch, code: 0,
		screen:  phaseBrowse,
		quitKey: "ctrl+c", exit: 130,
	},
	{
		name: "ctrl+c on no-results overrides the fixed status with 130",
		recs: recsSummaryOnly, code: 1,
		screen:  phaseNoResults,
		quitKey: "ctrl+c", exit: 130,
	},
	{
		name: "ctrl+c with the overlay open overrides the fixed status with 130",
		recs: recsOneMatch, code: 3,
		overlay: true, screen: phaseBrowse,
		quitKey: "ctrl+c", exit: 130,
		diags: []string{"exit status 3"},
	},
}

// pressKey delivers one named key through Update: the dismissal and
// quit keys the matrix drives, mapped to the messages a raw terminal
// sends.
func pressKey(t *testing.T, m Model, key string) (Model, tea.Cmd) {
	t.Helper()
	switch key {
	case "ctrl+c":
		return update(t, m, ctrlCPress())
	case "esc":
		return update(t, m, escPress())
	case "up":
		return update(t, m, tea.KeyPressMsg{Code: tea.KeyUp})
	case "down":
		return update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	default:
		return update(t, m, keyPress(key))
	}
}

// waitFixture builds the row's child wait error: nil for a clean exit,
// the exit-code fixture for a positive code, the signal fixture for -1.
func waitFixture(t *testing.T, row outcomeRow) error {
	t.Helper()
	switch {
	case row.code < 0:
		return sigErr(t)
	case row.code == 0:
		return nil
	default:
		return exitErr(t, row.code)
	}
}

// The single table-driven outcome matrix: each row drives the full
// completion → presentation → dismissal → exit sequence and asserts the
// initial screen and overlay state, the overlay's diagnostics, the
// post-dismissal state (and which key dismissed), and the final exit
// status — including that Esc only ever exits when the fatal overlay
// has no underlying state, and that ctrl+c overrides any fixed status.
func TestOutcomeMatrix(t *testing.T) {
	for _, row := range outcomeMatrix {
		t.Run(row.name, func(t *testing.T) {
			dir := t.TempDir()
			writeWorkFile(t, dir, "f.txt", "hit\n")

			m := newModel(nil, nil)
			m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
			m, _ = update(t, m, searchDoneMsg{
				index:   fixtureIndex(t, dir, row.recs...),
				stderr:  []byte(row.stderr),
				waitErr: waitFixture(t, row),
			})

			// Initial presentation: underlying screen and overlay state.
			if m.phase != row.screen {
				t.Fatalf("initial phase = %d, want %d", m.phase, row.screen)
			}
			if got := m.overlay != nil; got != row.overlay {
				t.Fatalf("overlay open = %v, want %v", got, row.overlay)
			}
			if row.overlay {
				v := m.View().Content
				if !strings.Contains(v, "┌") || !strings.Contains(v, "└") {
					t.Fatalf("overlay view lacks the single-line border: %q", v)
				}
				for _, d := range row.diags {
					if !strings.Contains(v, d) {
						t.Fatalf("overlay lacks diagnostic %q: %q", d, v)
					}
				}
			}

			// Dismissal step: the named key must reach the state the row
			// promises — the revealed underlying screen, or process exit
			// when the fatal overlay had nothing beneath it.
			cur := m
			if row.dismiss != "" {
				m2, cmd := pressKey(t, cur, row.dismiss)
				if row.afterQuit {
					if cmd == nil {
						t.Fatalf("%s on a fatal-only overlay returned no command, want tea.Quit", row.dismiss)
					}
					if _, ok := cmd().(tea.QuitMsg); !ok {
						t.Fatalf("%s command = %T, want tea.QuitMsg", row.dismiss, cmd())
					}
					if !m2.quit {
						t.Fatal("fatal-only overlay dismissal did not quit")
					}
					if m2.ExitCode() != row.exit {
						t.Fatalf("ExitCode = %d, want %d", m2.ExitCode(), row.exit)
					}
					return
				}
				if m2.overlay != nil {
					t.Fatalf("%s did not dismiss the overlay", row.dismiss)
				}
				if m2.phase != row.after {
					t.Fatalf("post-dismissal phase = %d, want %d", m2.phase, row.after)
				}
				cur = m2
			}

			// Final exit: the named key quits with the row's status.
			m2, cmd := pressKey(t, cur, row.quitKey)
			if cmd == nil {
				t.Fatalf("%s returned no command, want tea.Quit", row.quitKey)
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatalf("%s command = %T, want tea.QuitMsg", row.quitKey, cmd())
			}
			if m2.ExitCode() != row.exit {
				t.Fatalf("ExitCode = %d, want %d", m2.ExitCode(), row.exit)
			}
		})
	}
}

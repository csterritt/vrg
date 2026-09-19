package app

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// missingSummaryStream closes its one file but never terminates the
// stream — an integrity failure whatever the child's exit code says.
const missingSummaryStream = `{"type":"begin","data":{"path":{"text":"./a.go"}}}
{"type":"match","data":{"path":{"text":"./a.go"},"lines":{"text":"alpha\n"},"line_number":2,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
{"type":"end","data":{"path":{"text":"./a.go"},"binary_offset":null,"stats":{}}}
`

// orphanEndStream closes its file cleanly, then reports an end for a
// path that never opened — an orphaned-record integrity failure with
// usable results retained.
const orphanEndStream = `{"type":"begin","data":{"path":{"text":"./a.go"}}}
{"type":"match","data":{"path":{"text":"./a.go"},"lines":{"text":"alpha\n"},"line_number":2,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
{"type":"end","data":{"path":{"text":"./a.go"},"binary_offset":null,"stats":{}}}
{"type":"end","data":{"path":{"text":"./stray.go"},"binary_offset":null,"stats":{}}}
{"type":"summary","data":{"elapsed_total":{},"stats":{}}}
`

// orphanEndOnlyStream is an integrity failure with nothing retained:
// the stream's only file event is an end for a path that never opened.
const orphanEndOnlyStream = `{"type":"end","data":{"path":{"text":"./stray.go"},"binary_offset":null,"stats":{}}}
{"type":"summary","data":{"elapsed_total":{},"stats":{}}}
`

// stateGone marks an outcome-matrix step whose key exits outright: the
// fatal overlay-only presentation has no underlying state to reveal.
const stateGone = state(-1)

// outcomeCase is one row of the Issue #9 outcome matrix: the completed
// search in — a process result and an event stream — and the expected
// presentation, dismissal behavior, and fixed exit status out.
type outcomeCase struct {
	name   string
	stream string
	code   int    // rg's exit code; -1 with err models signal death
	err    error  // the wait error
	stderr string // captured stderr diagnostics

	// Initial presentation: the base state beneath any overlay,
	// whether the modal overlay is open, and frame content.
	state   state
	overlay bool
	shows   []string // must appear in the rendered frame
	omits   []string // must not appear in the rendered frame

	// dismiss is the key pressed next — "q" or "esc" — asserted to
	// dismiss the overlay (or to be a base-state no-op when no overlay
	// is open). after is the presentation the dismissal reveals;
	// stateGone means the dismissal itself exits with status.
	dismiss    string
	after      state
	showsAfter []string

	// close is the session-ending key — "q" or "ctrl+c" — run after any
	// dismissal; status is the fixed exit status it must produce.
	close  string
	status int
}

// TestOutcomeMatrix is the PRD's outcome table: every combination of
// process result, stream integrity, usable results, and diagnostics —
// asserting the initial presentation, the dismissal key's effect, and
// the fixed exit status that only ctrl+c can override.
func TestOutcomeMatrix(t *testing.T) {
	errKilled := errors.New("signal: killed")
	for _, tc := range []outcomeCase{
		{
			name:   "rg 0 clean stream browses",
			stream: happyStream, code: 0,
			state: stateBrowse, shows: []string{"a.go"},
			close: "q", status: 0,
		},
		{
			name:   "anomalous rg 1 with retained results browses",
			stream: happyStream, code: 1,
			state: stateBrowse, shows: []string{"a.go"},
			close: "q", status: 0,
		},
		{
			name:   "rg 1 empty complete stream shows no results",
			stream: emptyStream, code: 1,
			state: stateNoResults, shows: []string{"No results found"},
			close: "q", status: 1,
		},
		{
			name:   "Esc on the no-results base state never exits",
			stream: emptyStream, code: 1,
			state:   stateNoResults,
			dismiss: "esc", after: stateNoResults,
			showsAfter: []string{"No results found"},
			close:      "q", status: 1,
		},
		{
			name:   "Esc on the browse base state never exits",
			stream: happyStream, code: 0,
			state:   stateBrowse,
			dismiss: "esc", after: stateBrowse,
			showsAfter: []string{"a.go"},
			close:      "q", status: 0,
		},
		{
			name:   "fatal exit code with results browses under the overlay",
			stream: happyStream, code: 3, stderr: "boom\n",
			state: stateBrowse, overlay: true,
			shows:   []string{"boom", "a.go"},
			dismiss: "esc", after: stateBrowse,
			showsAfter: []string{"a.go"},
			close:      "q", status: 2,
		},
		{
			name:   "fatal exit code with results dismissed by q",
			stream: happyStream, code: 3, stderr: "boom\n",
			state: stateBrowse, overlay: true,
			dismiss: "q", after: stateBrowse,
			close: "q", status: 2,
		},
		{
			name:   "fatal exit code without results exits on q dismissal",
			stream: emptyStream, code: 3, stderr: "boom\n",
			state: stateOverlayOnly, overlay: true,
			shows:   []string{"boom"},
			omits:   []string{"No results found", "a.go"},
			dismiss: "q", after: stateGone, status: 2,
		},
		{
			name:   "fatal exit code without results exits on Esc dismissal",
			stream: emptyStream, code: 3, stderr: "boom\n",
			state: stateOverlayOnly, overlay: true,
			shows:   []string{"boom"},
			dismiss: "esc", after: stateGone, status: 2,
		},
		{
			name:   "failed process without stderr names the exit code",
			stream: emptyStream, code: 2,
			state: stateOverlayOnly, overlay: true,
			shows:   []string{"code 2"},
			dismiss: "q", after: stateGone, status: 2,
		},
		{
			name:   "signal death with results names the signal",
			stream: happyStream, code: -1, err: errKilled,
			state: stateBrowse, overlay: true,
			shows:   []string{"killed"},
			dismiss: "esc", after: stateBrowse,
			close: "q", status: 2,
		},
		{
			name:   "signal death without results names the signal",
			stream: emptyStream, code: -1, err: errKilled,
			state: stateOverlayOnly, overlay: true,
			shows:   []string{"killed"},
			dismiss: "q", after: stateGone, status: 2,
		},
		{
			name:   "missing summary with valid matches is fatal",
			stream: missingSummaryStream, code: 0,
			state: stateBrowse, overlay: true,
			shows:   []string{"incomplete"},
			dismiss: "esc", after: stateBrowse,
			showsAfter: []string{"a.go"},
			close:      "q", status: 2,
		},
		{
			name:   "orphaned end with valid matches is fatal",
			stream: orphanEndStream, code: 0,
			state: stateBrowse, overlay: true,
			shows:   []string{"incomplete"},
			dismiss: "q", after: stateBrowse,
			close: "q", status: 2,
		},
		{
			name:   "integrity failure without usable results",
			stream: orphanEndOnlyStream, code: 0,
			state: stateOverlayOnly, overlay: true,
			shows:   []string{"incomplete"},
			dismiss: "esc", after: stateGone, status: 2,
		},
		{
			name:   "stderr on rg 0 warns over browse",
			stream: happyStream, code: 0, stderr: "warn\n",
			state: stateBrowse, overlay: true,
			shows:   []string{"warn", "a.go"},
			dismiss: "esc", after: stateBrowse,
			showsAfter: []string{"a.go"},
			close:      "q", status: 0,
		},
		{
			name:   "stderr on rg 1 empty warns to no results",
			stream: emptyStream, code: 1, stderr: "warn\n",
			state: stateNoResults, overlay: true,
			shows:   []string{"warn"},
			dismiss: "esc", after: stateNoResults,
			showsAfter: []string{"No results found"},
			close:      "q", status: 1,
		},
		{
			name:   "all binary after a warning keeps the skip count",
			stream: allBinaryStream, code: 0, stderr: "warn\n",
			state: stateNoResults, overlay: true,
			shows:   []string{"warn"},
			dismiss: "esc", after: stateNoResults,
			showsAfter: []string{"No results found (2 binary files skipped)"},
			close:      "q", status: 1,
		},
		{
			name:   "ctrl+c after completion in browse exits 130",
			stream: happyStream, code: 0,
			state: stateBrowse,
			close: "ctrl+c", status: 130,
		},
		{
			name:   "ctrl+c after completion in no results exits 130",
			stream: emptyStream, code: 1,
			state: stateNoResults,
			close: "ctrl+c", status: 130,
		},
		{
			name:   "ctrl+c in an open overlay exits 130",
			stream: happyStream, code: 3, stderr: "boom\n",
			state: stateBrowse, overlay: true,
			close: "ctrl+c", status: 130,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := Result{
				Stdout: []byte(tc.stream),
				Stderr: []byte(tc.stderr),
				Code:   tc.code,
				Err:    tc.err,
			}
			m := newTestModel(fakeChild{res: res}, options{})
			m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			m.Update(runCollectCmd(t, m))
			assertOutcomeFrame(t, m, tc.state, tc.overlay, tc.shows, tc.omits)

			if tc.dismiss != "" {
				_, cmd := m.Update(outcomeKey(tc.dismiss))
				if tc.after == stateGone {
					if !m.quitting {
						t.Fatalf("%s did not exit the overlay-only outcome", tc.dismiss)
					}
					runQuittingCmd(t, cmd)
					if m.status != tc.status {
						t.Fatalf("status = %d, want %d", m.status, tc.status)
					}
					return
				}
				if cmd != nil {
					t.Fatalf("%s dismissal returned a command; want none", tc.dismiss)
				}
				assertOutcomeFrame(t, m, tc.after, false, tc.showsAfter, nil)
			}
			if tc.close == "" {
				return
			}
			_, cmd := m.Update(outcomeKey(tc.close))
			runQuittingCmd(t, cmd)
			if m.status != tc.status {
				t.Fatalf("status = %d, want %d", m.status, tc.status)
			}
		})
	}
}

func outcomeKey(s string) tea.KeyPressMsg {
	switch s {
	case "q":
		return keyQ
	case "esc":
		return keyEsc
	case "ctrl+c":
		return keyCtrlC
	}
	return tea.KeyPressMsg{Text: s, Code: rune(s[0])}
}

// assertOutcomeFrame checks the model's base state, whether the modal
// overlay is open, and the frame's rendered content.
func assertOutcomeFrame(t *testing.T, m *model, want state, overlay bool, shows, omits []string) {
	t.Helper()
	if m.state != want {
		t.Fatalf("state = %v, want %v", m.state, want)
	}
	if m.overlayOpen != overlay {
		t.Fatalf("overlayOpen = %v, want %v", m.overlayOpen, overlay)
	}
	v := viewText(m)
	for _, s := range shows {
		if !strings.Contains(v, s) {
			t.Errorf("view missing %q; view = %q", s, v)
		}
	}
	for _, s := range omits {
		if strings.Contains(v, s) {
			t.Errorf("view unexpectedly contains %q; view = %q", s, v)
		}
	}
}

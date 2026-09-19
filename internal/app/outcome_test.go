package app

import (
	"errors"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
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

// unknownOnlyStream is a complete stream whose only oddity is an
// unrecognized record type — a warning diagnostic, never a failure.
const unknownOnlyStream = `{"type":"weird","data":{"x":1}}
{"type":"summary","data":{"elapsed_total":{},"stats":{}}}
`

// malformedResultsStream keeps usable results beside one skipped
// garbage record: browse with the record-loss overlay, exit 0.
const malformedResultsStream = `{"type":"begin","data":{"path":{"text":"./a.go"}}}
{"type":"match","data":{"path":{"text":"./a.go"},"lines":{"text":"alpha\n"},"line_number":2,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
not json at all
{"type":"end","data":{"path":{"text":"./a.go"},"binary_offset":null,"stats":{}}}
{"type":"summary","data":{"elapsed_total":{},"stats":{}}}
`

// malformedOnlyStream is a complete stream that lost its only record to
// skipping: no usable results, so record loss is fatal.
const malformedOnlyStream = `not json at all
{"type":"summary","data":{"elapsed_total":{},"stats":{}}}
`

// skippedThenBinaryStream loses one record to skipping and its only
// retained file to binary exclusion: usable results are assessed after
// all filtering, so zero retained stops is the record-loss fatal row,
// not the no-results row.
const skippedThenBinaryStream = `{"type":"begin","data":{"path":{"text":"./b.bin"}}}
{"type":"match","data":{"path":{"text":"./b.bin"},"lines":{"text":"x y\n"},"line_number":1,"absolute_offset":0,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}
{"type":"end","data":{"path":{"text":"./b.bin"},"binary_offset":1,"stats":{}}}
not json either
{"type":"summary","data":{"elapsed_total":{},"stats":{}}}
`

// missingEndStream retains its match while the file's end never
// arrived: incomplete metadata is an integrity failure, so retained
// results still browse under the overlay at exit 2.
const missingEndStream = `{"type":"begin","data":{"path":{"text":"./a.go"}}}
{"type":"match","data":{"path":{"text":"./a.go"},"lines":{"text":"alpha\n"},"line_number":2,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
{"type":"summary","data":{"elapsed_total":{},"stats":{}}}
`

// missingEndEmptyStream is the same integrity failure with nothing
// retained: the overlay alone, exiting on dismissal.
const missingEndEmptyStream = `{"type":"begin","data":{"path":{"text":"./a.go"}}}
{"type":"summary","data":{"elapsed_total":{},"stats":{}}}
`

// stateGone marks an outcome-matrix step whose key exits outright: the
// fatal overlay-only presentation has no underlying state to reveal.
const stateGone = state(-1)

// staleContent is the injected stale payload for the Issue #29 rows:
// its bytes never equal the fixtures' recorded submatch bytes, so
// every submatch drops and every buffer decodes stale.
var staleContent = []byte("stale changed content\n")

// staleAllLoader is the injected loader returning the stale payload
// for every read — the Issue #29 outcome-matrix seam.
func staleAllLoader([]byte) ([]byte, error) { return staleContent, nil }

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

	// failLoads lists index file positions whose loads fail once the
	// search has settled — the Issue #26 rows. The current file's
	// failure runs the transition's own load command under the
	// injected failing loader; other positions are minted and failed
	// by injected completions. Load failures touch only presentation
	// and diagnostics: the fixed status must survive all of them.
	failLoads []int

	// staleLoads lists index file positions whose loads return stale
	// content — the Issue #29 rows. The current file's load command
	// runs the injected stale-returning loader; other positions are
	// minted and completed by injected stale-decoded buffers. Stale
	// validation touches only presentation: the fixed status must
	// survive all of it.
	staleLoads []int

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
			// Unknown-type warnings never change the exit status,
			// even with zero results: the warning overlay precedes
			// the no-results screen, where q exits 1.
			name:   "unknown types with zero results warn to no results",
			stream: unknownOnlyStream, code: 0,
			state: stateNoResults, overlay: true,
			shows:   []string{"1 unrecognised record types skipped"},
			dismiss: "esc", after: stateNoResults,
			showsAfter: []string{"No results found"},
			close:      "q", status: 1,
		},
		{
			name:   "malformed skipped with usable results browses under overlay",
			stream: malformedResultsStream, code: 0,
			state: stateBrowse, overlay: true,
			shows:   []string{"1 malformed record skipped", "a.go"},
			dismiss: "esc", after: stateBrowse,
			showsAfter: []string{"a.go"},
			close:      "q", status: 0,
		},
		{
			name:   "malformed skipped with zero usable results exits on q",
			stream: malformedOnlyStream, code: 0,
			state: stateOverlayOnly, overlay: true,
			shows:   []string{"1 malformed record skipped"},
			omits:   []string{"No results found"},
			dismiss: "q", after: stateGone, status: 2,
		},
		{
			name:   "malformed skipped with zero usable results exits on Esc",
			stream: malformedOnlyStream, code: 0,
			state: stateOverlayOnly, overlay: true,
			shows:   []string{"1 malformed record skipped"},
			dismiss: "esc", after: stateGone, status: 2,
		},
		{
			// A skipped record plus a binary exclusion leaves zero
			// retained stops — assessed after all filtering, so the
			// record-loss fatal row applies, not the no-results row.
			name:   "skipped record plus binary exclusion is fatal",
			stream: skippedThenBinaryStream, code: 0,
			state: stateOverlayOnly, overlay: true,
			shows:   []string{"1 malformed record skipped"},
			dismiss: "q", after: stateGone, status: 2,
		},
		{
			name:   "missing end with retained matches browses under overlay",
			stream: missingEndStream, code: 0,
			state: stateBrowse, overlay: true,
			shows:   []string{"incomplete", "a.go"},
			dismiss: "esc", after: stateBrowse,
			showsAfter: []string{"a.go"},
			close:      "q", status: 2,
		},
		{
			name:   "missing end with no matches is fatal",
			stream: missingEndEmptyStream, code: 0,
			state: stateOverlayOnly, overlay: true,
			shows:   []string{"incomplete"},
			dismiss: "q", after: stateGone, status: 2,
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
		{
			// Issue #26: every retained file fails to load — the
			// ordinary status stays 0; the failures touch only file
			// presentation and the diagnostic collection.
			name:   "all loads fail with fixed status 0 still exits 0",
			stream: happyStream, code: 0,
			state: stateBrowse, shows: []string{"a.go"},
			failLoads: []int{0, 1},
			dismiss:   "esc", after: stateBrowse,
			showsAfter: []string{"(unreadable)"},
			close:      "q", status: 0,
		},
		{
			// A current-file read failure cannot reopen the
			// fatal-search outcome already fixed at 2.
			name:   "current-file failure with fixed status 2 still exits 2",
			stream: happyStream, code: 3, stderr: "boom\n",
			state: stateBrowse, overlay: true,
			shows:     []string{"boom", "a.go"},
			failLoads: []int{0},
			dismiss:   "esc", after: stateBrowse,
			showsAfter: []string{"(unreadable)"},
			close:      "q", status: 2,
		},
		{
			// The composed row: usable results with fixed status 2
			// where every retained file subsequently fails to load —
			// the ordinary status remains 2, the load failures affect
			// only file presentation and diagnostics, and the
			// already-fixed fatal-search outcome is not recomputed.
			name:   "all loads fail with fixed status 2 still exits 2",
			stream: missingEndStream, code: 0,
			state: stateBrowse, overlay: true,
			shows:     []string{"incomplete", "a.go"},
			failLoads: []int{0},
			dismiss:   "esc", after: stateBrowse,
			showsAfter: []string{"(unreadable)"},
			close:      "q", status: 2,
		},
		{
			// Issue #29: every retained stop validates stale — the
			// files changed since the search — yet the fixed status
			// stays the search-derived one; the stale note rides the
			// filename row, touching only presentation.
			name:   "all stops stale with fixed status 0 still exits 0",
			stream: happyStream, code: 0,
			state: stateBrowse, shows: []string{"a.go"},
			staleLoads: []int{0, 1},
			dismiss:    "esc", after: stateBrowse,
			showsAfter: []string{"file changed since search"},
			close:      "q", status: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := Result{
				Stdout: []byte(tc.stream),
				Stderr: []byte(tc.stderr),
				Code:   tc.code,
				Err:    tc.err,
			}
			var opts options
			if len(tc.failLoads) > 0 {
				opts.loader = failAllLoader
			}
			if len(tc.staleLoads) > 0 {
				opts.loader = staleAllLoader
			}
			m := newTestModel(fakeChild{res: res}, opts)
			m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			_, loadCmd := m.Update(runCollectCmd(t, m))
			assertOutcomeFrame(t, m, tc.state, tc.overlay, tc.shows, tc.omits)

			// Issue #26 rows: fail the listed files' loads — the
			// current file's through the transition's own load
			// command, the rest by minted injected completions — then
			// prove the fixed status survived all of them.
			for _, fi := range tc.failLoads {
				if fi == m.curFile() {
					msg, ok := fileLoadOf(loadCmd)
					if !ok {
						t.Fatal("the browse transition's load command produced no completion")
					}
					m.Update(msg)
					continue
				}
				p := m.idx.Files[fi].Path
				m.Update(fileLoadedMsg{path: p, req: mintRequest(m, p), err: errUnreadable})
			}
			if len(tc.failLoads) > 0 {
				if m.status != tc.status {
					t.Fatalf("load failures changed the fixed status to %d, want %d", m.status, tc.status)
				}
				if !m.overlayOpen || !strings.Contains(m.overlayText, "cannot read") {
					t.Fatalf("overlay open=%v text=%q, want the current file's load failure shown",
						m.overlayOpen, m.overlayText)
				}
			}

			// Issue #29 rows: stale the listed files' loads — the
			// current file's through the transition's own load command
			// under the injected stale loader, the rest by minted
			// completions carrying stale-decoded buffers — then prove
			// the fixed status survived all of it.
			for _, fi := range tc.staleLoads {
				p := m.idx.Files[fi].Path
				if fi == m.curFile() {
					msg, ok := fileLoadOf(loadCmd)
					if !ok {
						t.Fatal("the browse transition's load command produced no completion")
					}
					_, lc := m.Update(msg)
					deliverLayout(t, m, lc)
					continue
				}
				m.Update(fileLoadedMsg{path: p, req: mintRequest(m, p),
					buf: filebuffer.Decode(staleContent, m.idx.Files[fi].Stops)})
			}
			for _, fi := range tc.staleLoads {
				key := string(m.idx.Files[fi].Path)
				if buf := m.bufs[key]; buf == nil || !buf.Stale() {
					t.Fatalf("file %d did not install a stale buffer", fi)
				}
			}
			if len(tc.staleLoads) > 0 && m.status != tc.status {
				t.Fatalf("stale loads changed the fixed status to %d, want %d", m.status, tc.status)
			}

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

// TestRecordLossDiagnostics pins the composed overlay lines for the
// record-loss counters: the malformed and oversized counts, plus one
// sanitized "oversized record skipped for <path>" line per recovered
// path — the only evidence of a file whose every record was discarded.
func TestRecordLossDiagnostics(t *testing.T) {
	out := DecideOutcome(OutcomeInput{
		Result:    Result{Code: 0},
		Integrity: searchindex.Integrity{Complete: true},
		Usable:    3,
		RecordLoss: RecordLoss{
			Malformed: 2,
			Oversized: 1,
			Paths:     [][]byte{[]byte("big\t.txt")},
		},
		Warnings: []string{"1 unrecognised record types skipped"},
	})
	want := []string{
		"2 malformed records skipped",
		"1 oversized record skipped",
		`oversized record skipped for big\t.txt`,
		"1 unrecognised record types skipped",
	}
	if !slices.Equal(out.Overlay, want) {
		t.Fatalf("Overlay = %q, want %q", out.Overlay, want)
	}
	if out.Presentation != presentBrowse || out.Status != 0 {
		t.Fatalf("record loss with usable results = (%v, %d), want browse + exit 0",
			out.Presentation, out.Status)
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

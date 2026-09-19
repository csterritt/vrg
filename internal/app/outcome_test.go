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

// duplicateBeginStream repeats a begin for the open file — a
// mid-stream lifecycle violation with usable results retained.
const duplicateBeginStream = `{"type":"begin","data":{"path":{"text":"./a.go"}}}
{"type":"begin","data":{"path":{"text":"./a.go"}}}
{"type":"match","data":{"path":{"text":"./a.go"},"lines":{"text":"alpha\n"},"line_number":2,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
{"type":"end","data":{"path":{"text":"./a.go"},"binary_offset":null,"stats":{}}}
{"type":"summary","data":{"elapsed_total":{},"stats":{}}}
`

// orphanMatchStream is a match for a path that never opened — retained
// with incomplete metadata.
const orphanMatchStream = `{"type":"match","data":{"path":{"text":"./a.go"},"lines":{"text":"alpha\n"},"line_number":2,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
{"type":"summary","data":{"elapsed_total":{},"stats":{}}}
`

// matchAfterEndStream is a match arriving after its file's end —
// retained with incomplete metadata like any orphaned match.
const matchAfterEndStream = `{"type":"begin","data":{"path":{"text":"./a.go"}}}
{"type":"match","data":{"path":{"text":"./a.go"},"lines":{"text":"alpha\n"},"line_number":2,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
{"type":"end","data":{"path":{"text":"./a.go"},"binary_offset":null,"stats":{}}}
{"type":"match","data":{"path":{"text":"./a.go"},"lines":{"text":"alpha\n"},"line_number":9,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
{"type":"summary","data":{"elapsed_total":{},"stats":{}}}
`

// matchAfterBinaryEndStream is a match arriving after a binary-
// excluding end: the orphaned-match cause under the binary-exclusion
// precedence, the late match not retained.
const matchAfterBinaryEndStream = `{"type":"begin","data":{"path":{"text":"./a.go"}}}
{"type":"match","data":{"path":{"text":"./a.go"},"lines":{"text":"alpha\n"},"line_number":2,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
{"type":"end","data":{"path":{"text":"./a.go"},"binary_offset":3,"stats":{}}}
{"type":"match","data":{"path":{"text":"./a.go"},"lines":{"text":"alpha\n"},"line_number":9,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
{"type":"summary","data":{"elapsed_total":{},"stats":{}}}
`

// duplicateEndStream closes its file twice — the second end is
// orphaned.
const duplicateEndStream = `{"type":"begin","data":{"path":{"text":"./a.go"}}}
{"type":"end","data":{"path":{"text":"./a.go"},"binary_offset":null,"stats":{}}}
{"type":"end","data":{"path":{"text":"./a.go"},"binary_offset":null,"stats":{}}}
{"type":"summary","data":{"elapsed_total":{},"stats":{}}}
`

// secondSummaryStream terminates twice — the second summary's sole
// cause is the extra summary, not a record after summary.
const secondSummaryStream = `{"type":"summary","data":{"elapsed_total":{},"stats":{}}}
{"type":"summary","data":{"elapsed_total":{},"stats":{}}}
`

// matchAfterSummaryStream is a record arriving after the summary —
// never dispatched to the lifecycle parsers.
const matchAfterSummaryStream = `{"type":"summary","data":{"elapsed_total":{},"stats":{}}}
{"type":"match","data":{"path":{"text":"./a.go"},"lines":{"text":"alpha\n"},"line_number":2,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
`

// beginAfterSummaryStream is a post-summary begin: it cannot open the
// file, so no missing-end cause can ever name it.
const beginAfterSummaryStream = `{"type":"summary","data":{"elapsed_total":{},"stats":{}}}
{"type":"begin","data":{"path":{"text":"./q.go"}}}
`

// contextAfterSummaryStream is a post-summary context record — an
// integrity failure like any other record after the summary since
// Issue #36 removed the exemption.
const contextAfterSummaryStream = `{"type":"begin","data":{"path":{"text":"./a.go"}}}
{"type":"match","data":{"path":{"text":"./a.go"},"lines":{"text":"alpha\n"},"line_number":2,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
{"type":"end","data":{"path":{"text":"./a.go"},"binary_offset":null,"stats":{}}}
{"type":"summary","data":{"elapsed_total":{},"stats":{}}}
{"type":"context","data":{"path":{"text":"./a.go"},"lines":{"text":"nearby\n"},"line_number":1,"absolute_offset":0,"submatches":[]}}
`

// malformedAfterSummaryStream is the dual-representation row: the
// garbage record after the summary is both the after-summary integrity
// cause and a malformed record in the count.
const malformedAfterSummaryStream = `{"type":"summary","data":{"elapsed_total":{},"stats":{}}}
not json at all
`

// unknownAfterSummaryStream is the same dual representation for an
// unknown-type record: the after-summary cause plus the unrecognised
// record-type warning.
const unknownAfterSummaryStream = `{"type":"summary","data":{"elapsed_total":{},"stats":{}}}
{"type":"frobnicate","data":{"x":1}}
`

// postSummaryTailStream ends in an unterminated fragment after a valid
// summary: post-summary precedence gives the fragment the after-
// summary cause — not a second unterminated-final-record cause —
// while the missing termination still counts malformed.
const postSummaryTailStream = `{"type":"begin","data":{"path":{"text":"./a.go"}}}
{"type":"match","data":{"path":{"text":"./a.go"},"lines":{"text":"alpha\n"},"line_number":2,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
{"type":"end","data":{"path":{"text":"./a.go"},"binary_offset":null,"stats":{}}}
{"type":"summary","data":{"elapsed_total":{},"stats":{}}}
{"type":"match","data":{"path":{"text":"./b.go"},"lines":{"text":"beta\n"},"line_number":1,"absolute_offset":0,"submatches":[{"match":{"text":"beta"},"start":0,"end":4}]}}`

// unterminatedTailStream ends in an unterminated fragment with no
// valid summary: the fragment is the unterminated final record —
// reported after the missing summary in the mandated end-of-stream
// order — and still counts malformed.
const unterminatedTailStream = `{"type":"begin","data":{"path":{"text":"./a.go"}}}
{"type":"match","data":{"path":{"text":"./a.go"},"lines":{"text":"alpha\n"},"line_number":2,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
{"type":"end","data":{"path":{"text":"./a.go"},"binary_offset":null,"stats":{}}}
{"type":"summ`

// repeatedViolationsStream repeats one orphaned match once per
// physical record — uncapped, unaggregated, undeduplicated.
const repeatedViolationsStream = `{"type":"match","data":{"path":{"text":"./a.go"},"lines":{"text":"alpha\n"},"line_number":2,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
{"type":"match","data":{"path":{"text":"./a.go"},"lines":{"text":"alpha\n"},"line_number":4,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
{"type":"match","data":{"path":{"text":"./a.go"},"lines":{"text":"alpha\n"},"line_number":7,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
{"type":"summary","data":{"elapsed_total":{},"stats":{}}}
`

// twoOpenFilesStream leaves two files open at stream end: their
// missing-end causes order by unsigned raw-path bytes — the \xff.bin
// begin arrived first yet its cause sorts last.
const twoOpenFilesStream = `{"type":"begin","data":{"path":{"bytes":"/y5iaW4="}}}
{"type":"begin","data":{"path":{"text":"a.go"}}}
{"type":"summary","data":{"elapsed_total":{},"stats":{}}}
`

// hostilePathStream leaves open a file whose raw path bytes carry a
// newline: the missing-end diagnostic shows it only through the
// EscapePath single-line escape, which can never forge a paragraph
// break.
const hostilePathStream = `{"type":"begin","data":{"path":{"text":"bad\nname.txt"}}}
{"type":"match","data":{"path":{"text":"bad\nname.txt"},"lines":{"text":"alpha\n"},"line_number":2,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
{"type":"summary","data":{"elapsed_total":{},"stats":{}}}
`

// allComponentsStream exercises every diagnostic component on one
// fatal stream: real child stderr, an after-summary integrity cause, a
// malformed record, and an unknown-type warning.
const allComponentsStream = `{"type":"begin","data":{"path":{"text":"./a.go"}}}
{"type":"match","data":{"path":{"text":"./a.go"},"lines":{"text":"alpha\n"},"line_number":2,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
{"type":"end","data":{"path":{"text":"./a.go"},"binary_offset":null,"stats":{}}}
not json at all
{"type":"frobnicate","data":{"x":1}}
{"type":"summary","data":{"elapsed_total":{},"stats":{}}}
{"type":"end","data":{"path":{"text":"./stray.go"},"binary_offset":null,"stats":{}}}
`

// nonFatalComponentsStream is the same coverage on a complete stream:
// the warning components keep their relative order — stderr, then
// record loss, then the unknown-type warning.
const nonFatalComponentsStream = `{"type":"begin","data":{"path":{"text":"./a.go"}}}
{"type":"match","data":{"path":{"text":"./a.go"},"lines":{"text":"alpha\n"},"line_number":2,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
{"type":"end","data":{"path":{"text":"./a.go"},"binary_offset":null,"stats":{}}}
not json at all
{"type":"frobnicate","data":{"x":1}}
{"type":"summary","data":{"elapsed_total":{},"stats":{}}}
`

// oversizedAfterSummaryStream is the post-summary oversized row: one
// physical record carrying both the after-summary integrity cause and
// the oversized record-loss representation — aggregate plus the
// recovered per-path detail.
func oversizedAfterSummaryStream() string {
	return `{"type":"summary","data":{"elapsed_total":{},"stats":{}}}` + "\n" +
		`{"type":"match","data":{"path":{"text":"q.txt"},"lines":{"text":"` +
		strings.Repeat("a", searchindex.MaxRecordBytes) +
		`"},"line_number":1,"submatches":[{"match":{"text":"a"},"start":0,"end":1}]}}` + "\n"
}

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

// utf16Content is the injected UTF-16 LE payload for the Issue #30
// rows: its FF FE signature classifies every decoded buffer as
// unsupported.
var utf16Content = []byte(utf16LEContent)

// unsupportedAllLoader is the injected loader returning the UTF-16
// payload for every read — the Issue #30 outcome-matrix seam.
func unsupportedAllLoader([]byte) ([]byte, error) { return utf16Content, nil }

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

	// unsupportedLoads lists index file positions whose loads detect
	// an unsupported encoding — the Issue #30 rows. The current
	// file's load command runs the injected BOM-payload loader; other
	// positions are minted and completed by injected
	// unsupported-decoded buffers. Encoding detection touches only
	// presentation and diagnostics: the fixed status must survive
	// all of it.
	unsupportedLoads []int

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
			shows:   []string{"missing summary record"},
			dismiss: "esc", after: stateBrowse,
			showsAfter: []string{"a.go"},
			close:      "q", status: 2,
		},
		{
			name:   "orphaned end with valid matches is fatal",
			stream: orphanEndStream, code: 0,
			state: stateBrowse, overlay: true,
			shows:   []string{"orphaned end for ./stray.go"},
			dismiss: "q", after: stateBrowse,
			close: "q", status: 2,
		},
		{
			name:   "integrity failure without usable results",
			stream: orphanEndOnlyStream, code: 0,
			state: stateOverlayOnly, overlay: true,
			shows:   []string{"orphaned end for ./stray.go"},
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
			shows:   []string{"missing end for ./a.go", "a.go"},
			dismiss: "esc", after: stateBrowse,
			showsAfter: []string{"a.go"},
			close:      "q", status: 2,
		},
		{
			name:   "missing end with no matches is fatal",
			stream: missingEndEmptyStream, code: 0,
			state: stateOverlayOnly, overlay: true,
			shows:   []string{"missing end for ./a.go"},
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
			shows:     []string{"missing end for ./a.go", "a.go"},
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
		{
			// Issue #30: every retained file opens with a UTF-16/32
			// BOM — the panel presents only placeholders and the
			// detections are collected diagnostics; the fixed status
			// stays the search-derived 0.
			name:   "all files unsupported with fixed status 0 still exits 0",
			stream: happyStream, code: 0,
			state: stateBrowse, shows: []string{"a.go"},
			unsupportedLoads: []int{0, 1},
			dismiss:          "esc", after: stateBrowse,
			showsAfter: []string{"(unsupported encoding)"},
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
			if len(tc.unsupportedLoads) > 0 {
				opts.loader = unsupportedAllLoader
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
				if !m.overlayOpen || !strings.Contains(m.overlay.text, "cannot read") {
					t.Fatalf("overlay open=%v text=%q, want the current file's load failure shown",
						m.overlayOpen, m.overlay.text)
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

			// Issue #30 rows: mark the listed files' loads
			// unsupported — the current file's through the
			// transition's own load command under the injected BOM
			// loader, the rest by minted completions carrying
			// unsupported-decoded buffers — then prove the fixed
			// status survived all of it.
			for _, fi := range tc.unsupportedLoads {
				p := m.idx.Files[fi].Path
				if fi == m.curFile() {
					msg, ok := fileLoadOf(loadCmd)
					if !ok {
						t.Fatal("the browse transition's load command produced no completion")
					}
					m.Update(msg)
					continue
				}
				m.Update(fileLoadedMsg{path: p, req: mintRequest(m, p),
					buf: filebuffer.Decode(utf16Content, m.idx.Files[fi].Stops)})
			}
			for _, fi := range tc.unsupportedLoads {
				key := string(m.idx.Files[fi].Path)
				if buf := m.bufs[key]; buf == nil || buf.Unsupported() == "" {
					t.Fatalf("file %d did not install an unsupported buffer", fi)
				}
			}
			if len(tc.unsupportedLoads) > 0 {
				if m.status != tc.status {
					t.Fatalf("unsupported loads changed the fixed status to %d, want %d",
						m.status, tc.status)
				}
				if !m.overlayOpen || !strings.Contains(m.overlay.text, "unsupported encoding") {
					t.Fatalf("overlay open=%v text=%q, want the current file's encoding diagnostic shown",
						m.overlayOpen, m.overlay.text)
				}
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

// integrityDiagCase is one composed-diagnostic row (Issue #36): the
// finished search in, and the complete ordered diagnostic line slice
// out — asserted identically against the overlay text and the session
// collection replayed to stderr, so a dropped, reordered, or
// rephrased component fails the test.
type integrityDiagCase struct {
	name   string
	stream string
	code   int    // rg's exit code; -1 with err models signal death
	err    error  // the wait error
	stderr string // captured stderr diagnostics
	want   []string
}

// TestIntegrityDiagnostics pins the complete composed diagnostic for
// every Issue #36 integrity cause and every overlap: each cause's
// stable user-facing line, the one-cause-per-physical-record
// precedence, the uncapped multiplicity, the dual representation of
// post-summary record loss, the unsigned raw-path ordering of missing
// ends, EscapePath escaping, the universal component order, and the
// absence of a process-status line for a 0/1 exit.
func TestIntegrityDiagnostics(t *testing.T) {
	errKilled := errors.New("signal: killed")
	for _, tc := range []integrityDiagCase{
		{
			name:   "duplicate begin names the open path",
			stream: duplicateBeginStream, code: 0,
			want: []string{"duplicate begin for ./a.go"},
		},
		{
			name:   "orphaned match names the never-opened path",
			stream: orphanMatchStream, code: 0,
			want: []string{"orphaned match for ./a.go"},
		},
		{
			name:   "match after end is the orphaned match",
			stream: matchAfterEndStream, code: 0,
			want: []string{"orphaned match for ./a.go"},
		},
		{
			name:   "match after binary end is the orphaned match",
			stream: matchAfterBinaryEndStream, code: 0,
			want: []string{"orphaned match for ./a.go"},
		},
		{
			name:   "orphaned end names the never-opened path",
			stream: orphanEndOnlyStream, code: 0,
			want: []string{"orphaned end for ./stray.go"},
		},
		{
			name:   "duplicate end is the orphaned end",
			stream: duplicateEndStream, code: 0,
			want: []string{"orphaned end for ./a.go"},
		},
		{
			name:   "missing end names the still-open path",
			stream: missingEndStream, code: 0,
			want: []string{"missing end for ./a.go"},
		},
		{
			// The missing summary is the explanation — never a
			// manufactured process-status line for a 0 exit.
			name:   "missing summary names itself, not an exit code",
			stream: missingSummaryStream, code: 0,
			want: []string{"missing summary record"},
		},
		{
			// An exit-1 child leaves no process line either.
			name:   "exit 1 emits no process-status line",
			stream: orphanEndOnlyStream, code: 1,
			want: []string{"orphaned end for ./stray.go"},
		},
		{
			// The second summary's sole integrity line is the extra
			// summary — never a record after summary.
			name:   "second summary reports only the extra summary",
			stream: secondSummaryStream, code: 0,
			want: []string{"extra summary record"},
		},
		{
			name:   "match after summary reports only its position",
			stream: matchAfterSummaryStream, code: 0,
			want: []string{"record after summary"},
		},
		{
			name:   "context after summary reports only its position",
			stream: contextAfterSummaryStream, code: 0,
			want: []string{"record after summary"},
		},
		{
			// The post-summary begin cannot open q.go, so no
			// end-of-stream missing end names it: the sole line is
			// the record's position.
			name:   "post-summary begin cannot open the file",
			stream: beginAfterSummaryStream, code: 0,
			want: []string{"record after summary"},
		},
		{
			// Dual representation: the post-summary fragment is the
			// after-summary cause and still counts malformed — never
			// a second unterminated-final-record cause.
			name:   "post-summary tail is after-summary plus malformed",
			stream: postSummaryTailStream, code: 0,
			want: []string{"record after summary", "1 malformed record skipped"},
		},
		{
			// The same fragment without a summary is the
			// unterminated final record, listed after the missing
			// summary in the mandated end-of-stream order.
			name:   "unterminated tail follows the missing summary",
			stream: unterminatedTailStream, code: 0,
			want: []string{
				"missing summary record",
				"unterminated final record",
				"1 malformed record skipped",
			},
		},
		{
			// Dual representation for a post-summary malformed
			// record: position cause and count both remain.
			name:   "post-summary malformed keeps both representations",
			stream: malformedAfterSummaryStream, code: 0,
			want: []string{"record after summary", "1 malformed record skipped"},
		},
		{
			// Dual representation for a post-summary oversized
			// record: the after-summary cause plus the oversized
			// aggregate and the recovered per-path detail.
			name:   "post-summary oversized keeps both representations",
			stream: oversizedAfterSummaryStream(), code: 0,
			want: []string{
				"record after summary",
				"1 oversized record skipped",
				"oversized record skipped for q.txt",
			},
		},
		{
			// Dual representation for a post-summary unknown-type
			// record: the after-summary cause plus the
			// unrecognised-type warning in the warning slot.
			name:   "post-summary unknown keeps both representations",
			stream: unknownAfterSummaryStream, code: 0,
			want: []string{
				"record after summary",
				"1 unrecognised record types skipped",
			},
		},
		{
			// Universal order on a fatal stream: the child's real
			// stderr precedes the integrity causes — neither
			// suppresses the other.
			name:   "stderr precedes integrity causes",
			stream: duplicateBeginStream, code: 0, stderr: "boom\n",
			want: []string{"boom", "duplicate begin for ./a.go"},
		},
		{
			// A failed child with stderr: the real stderr, never a
			// generated process line, then the integrity causes.
			name:   "fatal exit with stderr reports both",
			stream: missingSummaryStream, code: 3, stderr: "boom\n",
			want: []string{"boom", "missing summary record"},
		},
		{
			// A failed child without stderr owes the generated
			// exit-code line before the integrity causes.
			name:   "fatal exit without stderr generates the code line",
			stream: missingSummaryStream, code: 3,
			want: []string{"ripgrep exited with code 3", "missing summary record"},
		},
		{
			// Signal death names the signal, then the integrity
			// causes.
			name:   "signal death names the signal then the causes",
			stream: missingSummaryStream, code: -1, err: errKilled,
			want: []string{"ripgrep died: signal: killed", "missing summary record"},
		},
		{
			// Every component on one fatal stream, universal order:
			// process, integrity, record loss, unknown-type warning.
			name:   "all components compose in universal order",
			stream: allComponentsStream, code: 0, stderr: "warn\n",
			want: []string{
				"warn",
				"record after summary",
				"1 malformed record skipped",
				"1 unrecognised record types skipped",
			},
		},
		{
			// The non-fatal composition keeps the same relative
			// order with the integrity component absent.
			name:   "non-fatal components keep the universal order",
			stream: nonFatalComponentsStream, code: 0, stderr: "warn\n",
			want: []string{
				"warn",
				"1 malformed record skipped",
				"1 unrecognised record types skipped",
			},
		},
		{
			// One line per offending record, uncapped: three
			// identical orphaned matches, three identical lines.
			name:   "repeated violations emit one line each",
			stream: repeatedViolationsStream, code: 0,
			want: []string{
				"orphaned match for ./a.go",
				"orphaned match for ./a.go",
				"orphaned match for ./a.go",
			},
		},
		{
			// Missing ends order by unsigned raw-path bytes: 0xff
			// sorts after 'a' regardless of arrival order.
			name:   "missing ends order by unsigned raw-path bytes",
			stream: twoOpenFilesStream, code: 0,
			want: []string{
				"missing end for a.go",
				`missing end for \xff.bin`,
			},
		},
		{
			// A newline in the raw path is EscapePath-escaped: the
			// diagnostic stays one line and cannot forge a break.
			name:   "embedded path escapes through EscapePath",
			stream: hostilePathStream, code: 0,
			want: []string{`missing end for bad\nname.txt`},
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
			if !m.overlayOpen {
				t.Fatal("the composed diagnostic did not open the overlay")
			}
			if got := m.overlay.text; got != strings.Join(tc.want, "\n") {
				t.Fatalf("overlay text = %q, want %q", got, strings.Join(tc.want, "\n"))
			}
			// The same composed lines — same text, same order —
			// are what the post-restoration stderr replay emits.
			assertReplayLines(t, m.diags, tc.want)
		})
	}
}

// TestIntegrityDiagnosticsDeterministic proves the missing-end
// ordering survives repeated builds: the map-iteration order of
// still-open files can never leak into the composed diagnostic.
func TestIntegrityDiagnosticsDeterministic(t *testing.T) {
	var first []string
	for i := 0; i < 8; i++ {
		res := Result{Stdout: []byte(twoOpenFilesStream), Code: 0}
		m := newTestModel(fakeChild{res: res}, options{})
		m.Update(runCollectCmd(t, m))
		if first == nil {
			first = slices.Clone(m.diags)
			continue
		}
		assertReplayLines(t, m.diags, first)
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

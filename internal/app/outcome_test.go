package app

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"vrg/internal/searchindex"
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
	// A file left open when the stream ends, contributing no matches.
	recsOpenNoMatches = []string{
		`{"type":"begin","data":{"path":{"text":"f.txt"}}}`,
		recSummary,
	}
	// Two matched files: f.txt precedes g.txt in index order, so
	// f.txt is the current file at startup.
	recsTwoMatches = []string{
		`{"type":"begin","data":{"path":{"text":"f.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"f.txt"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"end","data":{"path":{"text":"f.txt"},"binary_offset":null}}`,
		`{"type":"begin","data":{"path":{"text":"g.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"g.txt"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"end","data":{"path":{"text":"g.txt"},"binary_offset":null}}`,
		recSummary,
	}
	// Unknown event types only: a complete stream, zero results.
	recsUnknownOnly = []string{
		`{"type":"weird","data":{"x":1}}`,
		recSummary,
	}
)

// streamLines joins records into a collected stdout stream — the form
// Index.Feed consumes, so malformed bytes can ride along.
func streamLines(records ...string) string {
	return strings.Join(records, "\n") + "\n"
}

// Issue #10 stream fixtures: each carries a record that cannot survive
// decoding, so the index must be built through Feed.
var (
	// One garbage line inside a valid file lifecycle: malformed skip
	// with usable results remaining.
	streamMalformedWithResults = streamLines(
		`{"type":"begin","data":{"path":{"text":"f.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"f.txt"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`this is not json`,
		`{"type":"end","data":{"path":{"text":"f.txt"},"binary_offset":null}}`,
		recSummary,
	)
	// The file's only match record is malformed: zero usable results.
	streamMalformedNoResults = streamLines(
		`{"type":"begin","data":{"path":{"text":"f.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"f.txt"},"lines":`,
		`{"type":"end","data":{"path":{"text":"f.txt"},"binary_offset":null}}`,
		recSummary,
	)
	// A skipped match plus a binary-excluding end leave zero retained
	// stops: usable results is assessed after all filtering, so this
	// is the record-loss fatal row, not no-results.
	streamMalformedThenBinary = streamLines(
		`{"type":"begin","data":{"path":{"text":"b.bin"}}}`,
		`{"type":"match","data":{"path":{"text":"b.bin"},"lines":`,
		`{"type":"match","data":{"path":{"text":"b.bin"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"end","data":{"path":{"text":"b.bin"},"binary_offset":7}}`,
		recSummary,
	)
)

// recordLimit is the PRD's maximum JSON record payload: 64 MiB
// excluding its newline delimiter. Written out so the oversized
// fixtures pin the contract itself.
const recordLimit = 64 << 20

// oversizedMatch builds a match record for path whose payload is
// exactly size bytes: a run of 'x' inside the lines text pads it out
// while the (0,1) submatch stays within the decoded line.
func oversizedMatch(path string, size int) string {
	pre := `{"type":"match","data":{"path":{"text":"` + path + `"},"lines":{"text":"`
	suf := `"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`
	return pre + strings.Repeat("x", size-len(pre)-len(suf)) + suf
}

// oversizedAnonMatch builds a match record whose giant lines member
// precedes data.path: the 64 MiB cut lands inside the lines value, so
// the record's path is never parsed — the anonymous oversized case.
func oversizedAnonMatch(size int) string {
	pre := `{"type":"match","data":{"lines":{"text":"`
	suf := `"},"path":{"text":"late.txt"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`
	return pre + strings.Repeat("x", size-len(pre)-len(suf)) + suf
}

// Issue #37 stream fixtures: oversized records inside a valid file
// lifecycle, named and anonymous.
var (
	// An anonymous oversized record — the limit hit before its path
	// was parsed — with no usable results: the aggregate alone must
	// carry the record-loss fatal row.
	streamOversizedAnonymousNoResults = streamLines(
		`{"type":"begin","data":{"path":{"text":"f.txt"}}}`,
		oversizedAnonMatch(recordLimit+200),
		`{"type":"end","data":{"path":{"text":"f.txt"},"binary_offset":null}}`,
		recSummary,
	)
	// A valid match plus an anonymous oversized record: usable
	// results remain, so the aggregate is a warning, not fatal.
	streamOversizedAnonymousWithResults = streamLines(
		`{"type":"begin","data":{"path":{"text":"f.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"f.txt"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		oversizedAnonMatch(recordLimit+200),
		`{"type":"end","data":{"path":{"text":"f.txt"},"binary_offset":null}}`,
		recSummary,
	)
	// A named oversized record beside a retained match.
	streamOversizedNamedWithResults = streamLines(
		`{"type":"begin","data":{"path":{"text":"f.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"f.txt"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		oversizedMatch("big.txt", recordLimit+1),
		`{"type":"end","data":{"path":{"text":"f.txt"},"binary_offset":null}}`,
		recSummary,
	)
)

// outcomeRow is one row of the Issue #9 outcome-transition matrix: the
// inputs a completed search presents — stream records, the child's wait
// status, and its stderr — and the full contract that follows: the
// initial presentation (underlying screen plus whether the diagnostics
// overlay opens), the substrings the overlay must carry, which key
// dismisses it, the post-dismissal state, the key that ends the
// session, and the final exit status. stream, when set, is fed through
// Index.Feed instead of recs so rows can carry malformed records.
type outcomeRow struct {
	name      string
	recs      []string
	stream    string
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
	// Issue #26 row extensions. failAll fails every retained file's
	// load after the outcome is fixed: the current file's own worker
	// through the injected loader, each other file's in-flight request
	// by injected completion. absent lists substrings the open overlay
	// must NOT carry — a non-current failure stays diagnostic-only —
	// viewHas lists substrings the composed frame must carry, and
	// replayHas lists substrings the exit replay must contain.
	failAll   bool
	absent    []string
	viewHas   []string
	replayHas []string
	// Issue #29 row extension: fileData replaces the f.txt fixture's
	// bytes so its load can validate stale, and loadCurrent settles
	// the current file's load before the assertions run so the stale
	// state exists. Stale state can never move the fixed exit status.
	fileData    string
	loadCurrent bool
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
	// Issue #10 rows: record-loss inputs and the after-filtering
	// usable-results assessment.
	{
		// Unknown-type warnings alone never change the exit status:
		// warning overlay first, then the no-results screen at 1.
		name: "unknown-type-only warnings with zero results warn then no-results, exit 1",
		recs: recsUnknownOnly, code: 0,
		overlay: true, screen: phaseNoResults,
		dismiss: "q", after: phaseNoResults,
		quitKey: "q", exit: 1,
		diags: []string{"1 unrecognised record types skipped"},
	},
	{
		name:    "malformed skip with usable results overlays browse, exit 0",
		code:    0,
		stream:  streamMalformedWithResults,
		overlay: true, screen: phaseBrowse,
		dismiss: "esc", after: phaseBrowse,
		quitKey: "q", exit: 0,
		diags: []string{"1 malformed record skipped"},
	},
	{
		// rg 0/1, complete stream, records skipped, no usable results:
		// the record-loss fatal row — the overlay stands alone and
		// either dismissal key exits 2.
		name:    "malformed skip with zero usable results is record-loss fatal, q exits 2",
		code:    0,
		stream:  streamMalformedNoResults,
		overlay: true, screen: phaseFatal,
		dismiss: "q", afterQuit: true, exit: 2,
		diags: []string{"1 malformed record skipped"},
	},
	{
		name:    "malformed skip with zero usable results is record-loss fatal, Esc exits 2",
		code:    0,
		stream:  streamMalformedNoResults,
		overlay: true, screen: phaseFatal,
		dismiss: "esc", afterQuit: true, exit: 2,
		diags: []string{"1 malformed record skipped"},
	},
	{
		// A skipped record plus binary exclusion leave zero retained
		// stops: assessed after all filtering, the record-loss fatal
		// row applies rather than the no-results row.
		name:    "skipped record plus binary exclusion is record-loss fatal, exit 2",
		code:    0,
		stream:  streamMalformedThenBinary,
		overlay: true, screen: phaseFatal,
		dismiss: "q", afterQuit: true, exit: 2,
		diags: []string{"1 malformed record skipped"},
	},
	// Issue #37 rows: the oversized aggregate is emitted whenever the
	// count is positive, regardless of path recovery — an anonymous
	// oversized record is never silent.
	{
		// No usable results and no recoverable path: the aggregate
		// alone fills the fatal overlay — never an empty one.
		name:    "anonymous oversized skip with zero usable results is record-loss fatal, exit 2",
		code:    0,
		stream:  streamOversizedAnonymousNoResults,
		overlay: true, screen: phaseFatal,
		dismiss: "q", afterQuit: true, exit: 2,
		diags: []string{"1 oversized record skipped"},
	},
	{
		// Usable results demote the same anonymous record to a
		// warning: the overlay opens over browse and the run exits 0.
		name:    "anonymous oversized skip with usable results overlays browse, exit 0",
		code:    0,
		stream:  streamOversizedAnonymousWithResults,
		overlay: true, screen: phaseBrowse,
		dismiss: "esc", after: phaseBrowse,
		quitKey: "q", exit: 0,
		diags: []string{"1 oversized record skipped"},
	},
	{
		// A recovered path earns its detail line after the aggregate.
		name:    "named oversized skip with usable results overlays browse, exit 0",
		code:    0,
		stream:  streamOversizedNamedWithResults,
		overlay: true, screen: phaseBrowse,
		dismiss: "q", after: phaseBrowse,
		quitKey: "q", exit: 0,
		diags: []string{"1 oversized record skipped", "oversized record skipped for big.txt"},
	},
	{
		// Missing end with retained matches is covered by the
		// open-at-end row above; with no matches at all the integrity
		// failure is fatal with nothing beneath the overlay.
		name: "missing end with no matches is fatal overlay, exit 2",
		recs: recsOpenNoMatches, code: 0,
		overlay: true, screen: phaseFatal,
		dismiss: "q", afterQuit: true, exit: 2,
		diags: []string{"missing end"},
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
	// Issue #26 rows: load failures after the outcome is fixed can
	// never move the exit status — not when every retained file fails,
	// not on top of an already-fatal fixed status, and not both at
	// once. The failures affect only file presentation and the
	// diagnostic collection.
	{
		name: "every retained file failing to load keeps the fixed status 0",
		recs: recsTwoMatches, code: 0, failAll: true,
		overlay: true, screen: phaseBrowse,
		dismiss: "esc", after: phaseBrowse,
		quitKey: "q", exit: 0,
		diags:     []string{"cannot read f.txt"},
		absent:    []string{"cannot read g.txt"},
		viewHas:   []string{"(unreadable)"},
		replayHas: []string{"cannot read f.txt", "cannot read g.txt"},
	},
	{
		name: "current-file load failure keeps the fixed status 2",
		recs: recsOneMatch, code: 3, failAll: true,
		overlay: true, screen: phaseBrowse,
		dismiss: "esc", after: phaseBrowse,
		quitKey: "q", exit: 2,
		diags:     []string{"exit status 3", "cannot read f.txt"},
		viewHas:   []string{"(unreadable)"},
		replayHas: []string{"cannot read f.txt"},
	},
	{
		// The composed row: usable results under a fatal search where
		// every retained file then fails to load. The ordinary status
		// stays 2 — the already-fixed fatal-search outcome is not
		// recomputed — and the non-current failure reaches only the
		// diagnostics and the replay, never the overlay.
		name: "fatal search with usable results then all files fail keeps 2",
		recs: recsTwoMatches, code: 3, failAll: true,
		overlay: true, screen: phaseBrowse,
		dismiss: "q", after: phaseBrowse,
		quitKey: "q", exit: 2,
		diags:     []string{"exit status 3", "cannot read f.txt"},
		absent:    []string{"cannot read g.txt"},
		viewHas:   []string{"(unreadable)"},
		replayHas: []string{"cannot read f.txt", "cannot read g.txt"},
	},
	// Issue #29: every retained stop validating stale changes only the
	// presentation — the dropped highlights and the filename-row note —
	// never the fixed search-derived exit status.
	{
		name: "every retained stop validating stale keeps the fixed status 0",
		recs: recsOneMatch, code: 0, fileData: "zzz\n", loadCurrent: true,
		screen:  phaseBrowse,
		quitKey: "q", exit: 0,
		viewHas: []string{"file changed since search"},
	},
	// Issue #30: every retained file detecting an unsupported encoding
	// changes only the presentation — the "(unsupported encoding)"
	// placeholder and its explanatory overlay — never the fixed
	// search-derived exit status.
	{
		name: "every retained file unsupported keeps the fixed status 0",
		recs: recsOneMatch, code: 0,
		fileData:    "\xff\xfeh\x00i\x00t\x00\n\x00",
		loadCurrent: true,
		overlay:     true, screen: phaseBrowse,
		dismiss: "esc", after: phaseBrowse,
		quitKey: "q", exit: 0,
		diags:     []string{"unsupported encoding"},
		viewHas:   []string{"(unsupported encoding)"},
		replayHas: []string{"unsupported encoding"},
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
	case "pgup":
		return update(t, m, codePress(tea.KeyPgUp))
	case "pgdown":
		return update(t, m, codePress(tea.KeyPgDown))
	default:
		return update(t, m, keyPress(key))
	}
}

// fixtureStream builds a prepared index by feeding a raw collected
// stream through Index.Feed — the route malformed bytes take, since
// they never reach a decoded Record.
func fixtureStream(t *testing.T, workdir, stream string) *searchindex.Index {
	t.Helper()
	ix := searchindex.New(workdir)
	ix.Feed([]byte(stream))
	ix.Prepare()
	return ix
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
			data := row.fileData
			if data == "" {
				data = "hit\n"
			}
			writeWorkFile(t, dir, "f.txt", data)

			var ix *searchindex.Index
			if row.stream != "" {
				ix = fixtureStream(t, dir, row.stream)
			} else {
				ix = fixtureIndex(t, dir, row.recs...)
			}

			m := newModel(nil, nil)
			if row.failAll {
				m.readFile = failLoader(errors.New("denied"))
			}
			m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
			m, load := update(t, m, searchDoneMsg{
				index:   ix,
				stderr:  []byte(row.stderr),
				waitErr: waitFixture(t, row),
			})
			if row.failAll || row.loadCurrent {
				m = settle(t, m, load)
			}
			if row.failAll {
				// The current file's own worker fails through the
				// injected loader; every other retained file's
				// in-flight request fails by injected completion.
				// Load failures land after the outcome is fixed and
				// can never move it.
				for _, f := range m.files {
					if bytes.Equal(f, m.currentPath()) {
						continue
					}
					m, _ = update(t, m, loadDoneMsg{
						path: f,
						req:  mintLoad(&m, string(f)),
						err:  errors.New("denied"),
					})
				}
			}

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
				for _, d := range row.absent {
					if strings.Contains(v, d) {
						t.Fatalf("overlay carries a diagnostic it must not: %q", d)
					}
				}
			}
			for _, d := range row.viewHas {
				if v := m.View().Content; !strings.Contains(v, d) {
					t.Fatalf("composed frame lacks %q: %q", d, v)
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
			for _, d := range row.replayHas {
				found := false
				for _, line := range replayed(t, m2) {
					if strings.Contains(line, d) {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("exit replay lacks %q", d)
				}
			}
		})
	}
}

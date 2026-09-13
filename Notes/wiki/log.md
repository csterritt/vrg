# Wiki log

Chronological, append-only record. Entries use `## [YYYY-MM-DD] <operation> | <subject>`.

## [2026-09-10] ingest | Issue #1 CLI foundation and scaffold

Ingested the completed Issue #1 implementation: the `vrg` module
(`go 1.27.1`), `cmd/vrg`, the six `internal/` package boundaries, and the
`internal/cli` mow.cli adapter with emission-prevention output strategy,
shared option/argument declarations, ordered raw-token preflight,
generated help, root validation, and the `Escape` sanitizer. Created
[project-overview](project-overview.md), [cli-foundation](cli-foundation.md),
[source-code](source-code.md), and [unit-tests](unit-tests.md); created
the index. Sources: `Notes/issues/001-go-scaffold-cli-positionals-and-root.md`,
`Notes/decisions/001-cli-scaffold-and-output-architecture.md`,
`Notes/PRD-vrg.md` (Invocation and child arguments, Module Design → CLI,
Testing Decisions → CLI), `cmd/vrg/main.go`, `internal/cli/cli.go`,
`internal/cli/cli_test.go`, `internal/cli/internal_test.go`,
`cmd/vrg/main_test.go`.

## [2026-09-10] ingest | Issue #2 CLI flag allow-list and child argv

Ingested the completed Issue #2 implementation: the shared `optionDecls`
table extended with the allow-listed no-argument search flags (`search`,
`unrestricted` role bits); the ordered `scanArgs`/`scanOption` preflight
recording exact spellings in encounter order, expanding combined shorts,
counting cumulative `-u` occurrences, and lexically rejecting `=`
assignment forms (truthy help assignments still pass through to the
parsed-value help seam); `Result.ChildArgs` carrying
`--json --no-config <flags> -- <pattern> <root>`; and the stub printing
`search stub: argv=rg …`. Created
[cli-flag-forwarding](cli-flag-forwarding.md); updated cli-foundation,
source-code, unit-tests, and index. Sources:
`Notes/issues/002-cli-flag-allow-list-and-child-argv.md`,
`Notes/PRD-vrg.md` (Invocation and child arguments), `internal/cli/cli.go`,
`internal/cli/cli_test.go`, `internal/cli/internal_test.go`,
`cmd/vrg/main.go`, `cmd/vrg/main_test.go`.

## [2026-09-10] ingest | Issue #3 search collection path

Ingested the completed Issue #3 implementation: ripgrep subprocess
execution (`cmd/vrg/main.go` `runSearch`), dual-pipe stdout/stderr
drainage, `internal/searchindex` JSON stream parsing (`begin`, `match`,
`end`, `summary`, `context`) with text/bytes encoding, same-line
merging, range normalization, raw-byte preservation, and unsigned byte
path ordering, and `internal/app` Bubble Tea model with searching,
summary, and start-failed states. Created
[search-collection-path](search-collection-path.md); updated
source-code, unit-tests, and index. Sources:
`Notes/issues/003-spawn-rg-collect-results-searching-screen.md`,
`Notes/PRD-vrg.md` (Module Design → SearchIndex / App, Testing
Decisions), `cmd/vrg/main.go`, `cmd/vrg/search_test.go`,
`internal/app/app.go`, `internal/app/app_test.go`,
`internal/searchindex/searchindex.go`,
`internal/searchindex/searchindex_test.go`.

## [2026-09-11] ingest | Issue #4 cancellation, child cleanup, terminal restore

Ingested the completed Issue #4 implementation: cancellation (q while
searching, ctrl+c in any state) with exit 130, late-completion
rejection via `StateCancelled` and a `cancelled` flag, child
termination/reaping via `Process.Cancel`/`Process.Cleanup` and
process-group kill, terminal restoration (alt screen exit, cursor
show) by Bubble Tea on `tea.Quit`, single post-restoration stderr
writer for diagnostics, exactly-once diagnostic behavior, injectable
controlled-failure hook (`WithFailureSignal`), and a reusable
fake-rg/PTY harness with blocked-child readiness/completion
handshakes, reap-evidence side channel (`VRG_TEST_REAP`), termios
snapshot/restore assertions, display-restoration sequence checks,
gate injection (`VRG_TEST_GATE`), and controlled-failure injection
(`VRG_TEST_FAIL_TRIGGER` / `VRG_TEST_FAIL_DIAGNOSTIC`). Updated
[search-collection-path](search-collection-path.md) with
cancellation, controlled-failure, and terminal-restoration sections;
updated source-code, unit-tests, and index. Sources:
`Notes/tasks/004-cancellation-child-cleanup-terminal-restore.md`,
`Notes/issues/004-cancellation-child-cleanup-terminal-restore.md`,
`Notes/PRD-vrg.md` (Outcome and exit-status contract, Cancellation
precedence, Cleanup, Terminal restoration, Subprocess-boundary
testing, Responsiveness boundaries), `cmd/vrg/main.go`,
`cmd/vrg/cancel_test.go`, `internal/app/app.go`,
`internal/app/app_test.go`.

## [2026-09-11] ingest | Issue #5 browse tracer

Ingested the completed Issue #5 implementation: the two-pane browse
view (file list on the left, content panel on the right with
inverse-video highlights), the safe-presentation core for paths and
content with byte→cell mappings, async file loading with a
`Loading…` placeholder and prepared buffers delivered off the update
path, the filename rule, right-justified gutter, no borders, browse
`q` exit 0 and `ctrl+c` exit 130 through the Issue #4 cleanup path,
late-load rejection after cancellation, responsive key/resize while
loading, and the hostile-fixture sink-safety method via a no-style
composition path. Created [browse-tracer](browse-tracer.md); updated
source-code, unit-tests, and index. Sources:
`Notes/tasks/005-browse-tracer-file-list-and-file-panel.md`,
`Notes/issues/005-browse-tracer-file-list-and-file-panel.md`,
`Notes/PRD-vrg.md` (File list and layout, Text, graphemes, and safe
presentation, Module Design → FileBuffer / Viewport / Theme / App),
`internal/safepresentation/safepresentation.go`,
`internal/safepresentation/safepresentation_test.go`,
`internal/filebuffer/filebuffer.go`,
`internal/filebuffer/filebuffer_test.go`,
`internal/viewport/viewport.go`, `internal/theme/theme.go`,
`internal/app/app.go`, `internal/app/browse_test.go`.

## [2026-09-12] ingest | Issue #6 shared safe-presentation utility

Ingested the completed Issue #6 implementation: the shared
safe-presentation utility generalized from the Issue #5 path/content
core, the new `EscapeDiagnostic` escaper (preserves LF line boundaries,
normalizes CRLF to LF, expands tabs to eight-column stops, escapes
other controls with caret notation / `\u00XX` / `\xNN`, does not escape
backslashes so embedded filenames escaped through `EscapePath` are not
double-escaped), the replacement of the Issue #1 `cli.Escape` escaper
with a one-line wrapper around `safepresentation.EscapePath`, the
replacement of the `internal/app` `sanitizeDiagnostic` (which only
mapped ESC and C1 CSI to spaces) with a delegate to
`EscapeDiagnostic`, the `app.EscapePathForDiagnostic` helper for
embedding filenames in diagnostics, and the shared
`internal/sinkfixtures` package with the hostile-fixture set and
sink-safety assertion helpers (`NoControlBytes`, `NoDangerousControls`,
`NoPayloadAfterESC`). Created [safe-presentation](safe-presentation.md);
updated cli-foundation, source-code, unit-tests, and index. Sources:
`Notes/tasks/006-safe-presentation-utility-for-all-sinks.md`,
`Notes/PRD-vrg.md` (Text, graphemes, and safe presentation),
`internal/safepresentation/safepresentation.go`,
`internal/safepresentation/safepresentation_test.go`,
`internal/safepresentation/doc.go`,
`internal/sinkfixtures/sinkfixtures.go`,
`internal/cli/cli.go`, `internal/cli/cli_test.go`,
`internal/app/app.go`, `internal/app/app_test.go`,
`internal/app/browse_test.go`,
`cmd/vrg/main.go`, `cmd/vrg/main_test.go`.

## [2026-09-11] ingest | Issue #7 theme colour toggle and match styles

Ingested the completed Issue #7 implementation: the theme module now
owns the active colour scheme (dark/light) and the full PRD style set.
`New()` starts dark (white on black); `Toggle()` flips between dark
and light with no persistence; `NoStyle()` disables all ANSI sequences
for sink-safety testing. The style set: `Base`, `Gutter`, `Match`
(true inverse of base), `CurrentMatch` (true inverse + underline),
`Indicator` (inverse), `Underline`, `FileList`, `FilenameRule`,
`Overlay` (base colours + plain single-line border). Match,
CurrentMatch, Indicator, and Underline restore the base colours after
the styled span. The App handles the `c` keypress in browse state by
calling `theme.Toggle()`. Browse rendering wraps each composed line in
`theme.Base()` and uses `theme.Match`/`theme.CurrentMatch` for
highlights depending on whether the line is the current matched line
(the first stop for the current file until Issue #13 adds
navigation). Issue #5's `theme.Reverse` (SGR 7) is replaced by explicit
true-inverse colour pairs. Created [theme-module](theme-module.md);
updated [browse-tracer](browse-tracer.md) (Theme section, Rendering
section, key handling), [source-code](source-code.md) (internal/theme),
[unit-tests](unit-tests.md) (internal/theme, Issue #7 app tests),
[safe-presentation](safe-presentation.md) (styled assertion method),
and [index](index.md). Sources:
`Notes/tasks/007-theme-colour-toggle-and-match-styles.md`,
`Notes/PRD-vrg.md` (Colours, overlays, and key precedence; Module
Design → Theme), `internal/theme/theme.go`,
`internal/theme/theme_test.go`, `internal/app/app.go`,
`internal/app/browse_test.go`.

## [2026-09-12] ingest | Issue #8 no-results screen and binary exclusion

Ingested the completed Issue #8 implementation: binary-file exclusion
during ripgrep result indexing and a distinct no-results TUI outcome.
`internal/searchindex` now drops a file and all its previously collected
matches when an `end` event carries a non-null `binary_offset`, counts
the file as a distinct excluded file, and drops later matches for the
same file. `Index.ExcludedFiles()` exposes the distinct excluded-file
count. `internal/app` added `StateNoResults`: a centred no-results
screen shown after a complete successful search (rg exit 0 or 1) with
no usable results, with the optional `(N binary files skipped)` suffix
when every matched file was excluded. The outcome logic consumes the
retained stop count after binary filtering as the single usable-results
value. `q` from no-results exits 1 through the Issue #4 cleanup path;
`Esc` is a no-op; `ctrl+c` exits 130. Mixed streams (one file excluded,
one retained) transition to ordinary browsing. Updated
[search-collection-path](search-collection-path.md),
[source-code](source-code.md), [unit-tests](unit-tests.md), and the
index. Sources: `Notes/tasks/008-no-results-screen-and-binary-exclusion.md`,
`Notes/PRD-vrg.md` (Result index contract, Outcome and exit-status
contract), `internal/searchindex/searchindex.go`,
`internal/searchindex/searchindex_test.go`, `internal/app/app.go`,
`internal/app/browse_test.go`, `cmd/vrg/search_test.go`,
`cmd/vrg/cancel_test.go`.

## [2026-09-13] ingest | Issue #9 error overlay and fatal outcomes

Ingested the completed Issue #9 implementation: stream-integrity
accounting, separate process-success and stream-integrity assessment,
the fatal/warning outcome matrix, the modal error overlay, and exit
status 2 for fatal process or stream-integrity outcomes.
`internal/searchindex` now tracks per-path open/closed state,
summary-seen state, after-summary state, trailing-malformed state, and
an integrity-failed flag. `Index.Integrity()` exposes the
`Integrity{Complete}` assessment, kept separate from process success.
`Stop.Incomplete` marks retained matches whose lifecycle metadata is
incomplete. `Builder.MarkTrailingMalformed` signals a trailing
unterminated record. `internal/app` added `ProcessResult` (exit code +
signal death), `OutcomeInput`/`Outcome`, the pure `DecideOutcome`
function, `OverlayKind` (none/error/warning), `OverlayOpen()`/
`OverlayKind()` accessors, `SearchCompleteMsg.Process`/`Stderr`
fields, and `processResult`/`processResultFromSys` to derive the
process result from the wait error. The fixed exit status is decided
once at completion and never recomputed except by `ctrl+c` (which
overrides to 130). The overlay is modal: up/down scroll, q/Esc dismiss
(non-fatal) or exit 2 (fatal no-results), other keys ignored. Large
diagnostics show both head and tail. Created
[outcome-contract](outcome-contract.md). Updated
[source-code](source-code.md), [unit-tests](unit-tests.md), and the
index. Sources:
`Notes/tasks/009-error-overlay-and-fatal-outcomes.md`,
`Notes/issues/009-error-overlay-and-fatal-outcomes.md`,
`Notes/PRD-vrg.md` (Result index contract, Outcome and exit-status
contract, Colours/overlays/key precedence), `internal/searchindex/searchindex.go`,
`internal/searchindex/lifecycle_test.go`, `internal/app/app.go`,
`internal/app/outcome_test.go`, `internal/app/overlay_test.go`,
`internal/app/browse_test.go`, `cmd/vrg/outcome_test.go`,
`cmd/vrg/search_test.go`.

## [2026-09-12] ingest | Issue #10 record robustness — malformed, oversized, unknown

Ingested the completed Issue #10 implementation: robust handling of
malformed, oversized, and unknown-type records in ripgrep JSON streams
with separate counters, bounded 64 MiB record parsing with
discard-and-resynchronize behavior, sanitized oversized-record
diagnostics, and record-loss outcome rows. `internal/searchindex`
added `Index.MalformedCount()`, `Index.OversizedCount()`,
`Index.UnknownCount()`, and `Index.OversizedDiagnostics()` accessors;
`Builder.ReadFrom` is the bounded 64 MiB record reader that discards
oversized records through the next newline and resynchronizes on the
following record; `Builder.recordOversized` counts oversized records
and recovers sanitized path diagnostics via token-based JSON parsing
through the Issue #6 `safepresentation.EscapePath` utility; unknown
string event types are counted separately from malformed and never
independently alter exit status; a trailing oversized record without
a newline is counted oversized and malformed and marks the stream
incomplete. `internal/app` extended `RecordLoss` with `Oversized`,
added `RecordLossDiagnostics` to `OutcomeInput`, the
`recordLossDiagnostics` helper, and new outcome-matrix rows for
malformed, oversized, and unknown record loss; `collectResults` now
uses `Builder.ReadFrom` instead of `bufio.Scanner`. Created
[record-robustness](record-robustness.md); updated
[search-collection-path](search-collection-path.md),
[outcome-contract](outcome-contract.md), [source-code](source-code.md),
[unit-tests](unit-tests.md), and the index. Sources:
`Notes/tasks/010-record-robustness-malformed-oversized-unknown.md`,
`Notes/issues/010-record-robustness-malformed-oversized-unknown.md`,
`Notes/PRD-vrg.md` (Result index, records, and stream integrity;
Outcome and exit-status contract; Resources and responsiveness),
`internal/searchindex/searchindex.go`,
`internal/searchindex/malformed_test.go`,
`internal/searchindex/oversized_test.go`,
`internal/app/app.go`, `internal/app/outcome_test.go`,
`internal/app/browse_test.go`.

## [2026-09-11] ingest | Issue #11 stderr replay of collected diagnostics

Ingested the completed Issue #11 implementation: the session
diagnostic collection independent of display, the
processed-versus-in-flight shutdown boundary, and post-restoration
stderr replay of every collected diagnostic exactly once in collection
order. `internal/app` added a `diagnostics []string` field on `Model`,
a `collectDiagnostic` helper (sanitizes through
`safepresentation.EscapeDiagnostic`, appends to the collection, fires
the `onCollect` callback), a `Diagnostics()` accessor (returns a copy
in collection order), a `DiagnosticMsg` type (collects without
display), `WithDiagnosticSignal` and `WithOnCollect` options (test
seams), and a `watchDiagnostic` command. The `SearchCompleteMsg`,
`SearchFailedMsg`, `ControlledFailureMsg`, and `DiagnosticMsg`
handlers now collect into the session collection. The
controlled-failure diagnostic is routed through the collection instead
of a separate direct write, so exactly-once holds across both the
former direct-write path and the replay mechanism. The shutdown
boundary is defined at message-processing time: a diagnostic is
collected once the model has processed the message carrying it;
in-flight diagnostics are not awaited or replayed. `cmd/vrg/main.go`
replays every collected diagnostic from `m.Diagnostics()` to stderr
exactly once each in collection order after terminal restoration, and
wires the `VRG_TEST_COLLECT_ACK` acknowledgement side channel and the
`VRG_TEST_DIAGNOSTIC_TRIGGER`/`VRG_TEST_DIAGNOSTIC_TEXT` diagnostic
emission trigger. Updated
[search-collection-path](search-collection-path.md),
[outcome-contract](outcome-contract.md),
[safe-presentation](safe-presentation.md) (replay writer added to the
sink-safety table), [source-code](source-code.md),
[unit-tests](unit-tests.md), and the index. Sources:
`Notes/tasks/011-stderr-replay-of-collected-diagnostics.md`,
`Notes/issues/011-stderr-replay-of-collected-diagnostics.md`,
`Notes/PRD-vrg.md` (Colours, overlays, and key precedence — replay
bullet; Outcome and exit-status contract), `internal/app/app.go`,
`internal/app/replay_test.go`, `cmd/vrg/main.go`,
`cmd/vrg/replay_test.go`.

## [2026-09-11] ingest | Issue #12 manual vertical scrolling and per-file viewport

Ingested Issue #12: manual vertical scrolling with per-file saved
viewport state and prepared-row rendering. `internal/viewport` was
expanded from the Issue #5 minimal seam into the full scroll module:
`RowProvider` interface (`RowCount`/`Rows`) for on-demand prepared rows,
`Viewport` with `panelHeight`/`offset`/`RowProvider`, content height as
`panelHeight - 1` (filename row), `maxOffset = max(0, rowCount -
contentHeight)` for no avoidable blank rows below EOF, scroll methods
(one-row `ScrollDown`/`ScrollUp`, half-page `ScrollHalfDown`/
`ScrollHalfUp` as `max(1, floor(contentHeight/2))`, full-page
`ScrollPageDown`/`ScrollPageUp` as `contentHeight`), `SetOffset`/
`SetPanelHeight`/`SetRows` with re-clamping, `Visible()` querying only
the visible `[offset, offset+contentHeight)` range, and `BufferRows`
adapter. `internal/app` integrated the viewport: `viewport` field (nil
while loading), `perFileOffset map[string]int` for per-file saved
offsets keyed by raw path, `currentPath []byte`, `RowProviderFactory`
type and `WithRowProviderFactory` option (test seam for the render-cost
guard), `ViewportOffset()`/`SavedOffset(path)` accessors,
`handleScrollKey` routing `up`/`down`/`u`/`d`/`pgup`/`pgdn` to the
viewport, `saveOffset` recording per-file state after every scroll,
`FileLoadCompleteMsg` building the viewport from prepared row data and
restoring the saved offset (0 for a first visit), `WindowSizeMsg`
calling `viewport.SetPanelHeight`, and `renderContentPanel` querying
`viewport.Visible()` for the visible range only instead of scanning
the full buffer per frame. Scroll keys are no-ops while the viewport is
nil (loading placeholder). Tests added: `internal/viewport/
viewport_test.go` (scroll units, clamps, odd heights, file lengths,
empty file, render-cost guard, SetOffset/SetPanelHeight clamping) and
`internal/app/scroll_test.go` (placeholder no-op, per-file state
saved/restored/first-visit, scroll key behavior, BOF/EOF clamps,
render-cost guard before/after scroll, visible-rows-only rendering).
Existing browse tests updated to set a terminal size before search
completion so the viewport has a usable content height. Created
[manual-vertical-scrolling](manual-vertical-scrolling.md), updated
[browse-tracer](browse-tracer.md) (Viewport section, Browse model
fields, Update flow, key handling, Rendering),
[source-code](source-code.md) (internal/viewport and internal/app
entries), [unit-tests](unit-tests.md) (internal/viewport and
internal/app scroll_test.go catalogs), and the index. Sources:
`Notes/tasks/012-manual-vertical-scrolling-and-per-file-viewport.md`,
`Notes/issues/012-manual-vertical-scrolling-and-per-file-viewport.md`,
`Notes/PRD-vrg.md` (Navigation, viewport, and logical anchors;
Module Design), `internal/viewport/viewport.go`,
`internal/viewport/viewport_test.go`, `internal/app/app.go`,
`internal/app/scroll_test.go`, `internal/app/browse_test.go`.

## [2026-09-13] ingest | Issue #13 match navigation n/p circular cursor

Ingested the completed Issue #13 implementation: the single global
matched-line cursor and App wiring for circular `n`/`p` navigation.
`internal/searchindex` added `Cursor`, a circular cursor over
`Index.stops` (already ordered by unsigned raw path bytes then
ascending line number and deduplicated by `(raw path, line number)`, so
multiple submatches on one line are one stop). `NewCursor(idx)` starts
at the first stop (position 0) or -1 when empty/nil.
`Stop()`/`Position()`/`Len()` report state. `Next()`/`Prev()` move
circularly with wrap at both ends and return the new stop, whether the
cursor moved, and whether the file changed (raw path differs via
`bytes.Equal`). With zero or one stop both are strict no-ops: no move,
no file change. `internal/app` replaced the Issue #5 `browseIdx` int
with a `cursor *searchindex.Cursor` field, created on
`SearchCompleteMsg` via `searchindex.NewCursor`, selecting the first
stop at startup. `handleNavigate(delta)` is called for `n` (delta 1)
and `p` (delta -1): with zero or one stop it is a strict no-op; on a
same-file move only the current matched line styling changes
(destination reveal belongs to Issue #14); on a cross-file move the
departing file's viewport offset is saved, the content panel switches
immediately, a cached destination is shown with its saved viewport
restored (first visit starts at the top), and an uncached destination
requests a load via `loadFileFor(path)`. A `fileCache
map[string]*filebuffer.Buffer` session cache keyed by raw path (no
eviction) lets a revisited file be shown immediately without a reload.
`loadFile` now delegates to `loadFileFor` with the cursor's current
stop's raw path. `CursorPosition()`/`CurrentPath()` accessors expose
cursor state. `renderBrowse` derives the current file and current
matched line from the cursor so the file list underline and
current-match styling follow cursor selection. Manual scrolling does
not move the cursor, so `n`/`p` continue from the last selected stop.
The file list remains passive with no direct selection route. Tests
added: `internal/searchindex/cursor_test.go` (startup selection,
position, Next/Prev advance/retreat, wrap at both ends, file-change
flag, single-stop no-op, empty no-op, multiple-submatches-one-stop,
Len, nil index, full cycle, bytes-equal file change) and
`internal/app/navigation_test.go` (startup selection, cross-file
switching with load request, same-file no-load, wrap cross-file,
one-stop no-op, list underline follows cursor, current-match underline
moves, manual-scroll independence, departing viewport save, saved
viewport restore, first-visit top, passive file list, n while loading,
multi-file multi-stop sequence). Updated
[browse-tracer](browse-tracer.md) (Navigation cursor section, Browse
model fields, Update flow, key handling, Rendering),
[source-code](source-code.md) (internal/searchindex and internal/app
entries), [unit-tests](unit-tests.md) (internal/searchindex cursor
tests and internal/app navigation tests catalogs), and the index.
Sources: `Notes/tasks/013-match-navigation-n-p-circular-cursor.md`,
`Notes/PRD-vrg.md` (Navigation, viewport, and logical anchors),
`internal/searchindex/searchindex.go`,
`internal/searchindex/cursor_test.go`, `internal/app/app.go`,
`internal/app/navigation_test.go`.

## [2026-09-12] ingest | Issue #14 vertical destination reveal

Implemented vertical destination reveal (Issue #14): navigating to a
match adjusts the viewport so the rendered row containing the match's
display target is visible, without unnecessary scrolling. The display
target is the start cell of the first submatch on the destination line
(the marker cell for a zero-width match); submatches are ordered by
byte start, so the first identifies the target. The reveal targets the
rendered row containing this target, not merely the source-line
ordinal. Without wrapping (Issue #16 pending), the rendered row is the
0-based source line index. If the target row is already visible, the
viewport does not scroll. If hidden, the viewport is moved so the
target lands at zero-based row `floor(contentHeight / 3)`, clamped to
`[0, maxOffset]` so BOF and EOF available content takes precedence
over one-third placement. On a file change, the starting viewport is
the saved per-file offset (revisit) or 0 (first visit, including the
startup file), then the reveal is applied. A reveal that moves the
viewport replaces the saved per-file vertical state; a no-scroll reveal
preserves the retained state. Reveal triggers: startup after the
initial file load completes (owned by Issue #14; broader
load-completion reveal owned by Issue #28), same-file `n`/`p`,
cross-file `n`/`p` to a cached destination (immediately), and
cross-file `n`/`p` to an uncached destination (when the load
completes). Reload does not trigger a reveal. Horizontal reveal is
owned by Issue #19. Added `viewport.Reveal(targetRow int)` and
`app.Model.revealTarget()`/`targetRow(stop)` with a `needsReveal` flag
gating the startup-after-load and uncached-cross-file-navigation
triggers. Tests added: `internal/viewport/viewport_test.go` (visible
no-scroll, hidden one-third placement, BOF clamp, EOF clamp, saved
viewport visible/hidden, first-visit visible/hidden, exact one-third
boundary, empty file) and `internal/app/reveal_test.go` (startup
reveal after load, startup visible no-scroll, startup BOF/EOF clamp,
same-file navigation reveal/back/visible no-scroll, cross-file
navigation reveal cached/uncached, reveal moves replaces saved state,
reveal no-scroll leaves saved state, saved viewport starting point,
first visit starts at top, first submatch identification). Updated
`TestNavigationRestoresSavedViewport` to reflect the new contract
(saved offset is the starting point, then reveal applies). Updated
[source-code](source-code.md) (internal/app and internal/viewport
entries), [unit-tests](unit-tests.md) (viewport reveal tests and
reveal_test.go catalog), and the index. Created
[destination-reveal](destination-reveal.md). Sources:
`Notes/tasks/014-vertical-destination-reveal.md`,
`Notes/PRD-vrg.md` (Navigation, viewport, and logical anchors),
`internal/viewport/viewport.go`, `internal/viewport/viewport_test.go`,
`internal/app/app.go`, `internal/app/reveal_test.go`,
`internal/app/navigation_test.go`.

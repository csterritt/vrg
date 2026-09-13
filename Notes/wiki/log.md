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

## [2026-09-10] ingest | Issue #15 file-change pop-up

Implemented the transient centred file-change pop-up (Issue #15): when
`n`/`p` navigation crosses a file boundary, a centred single-line
pop-up displays the selected file's escaped path for one second or
until a keypress. The pop-up starts at selection time (not load
completion), uses instance-keyed one-second timers so stale expiry
messages cannot dismiss newer pop-ups, and is dismissed by any
keypress (the same key then performs its normal action: q still
quits, scroll keys still scroll, Esc is a no-op besides dismissal).
Error-overlay opening cancels the pop-up permanently; it does not
return after overlay dismissal, expiry, or keypress dismissal. Resize
recentres and retruncates without restarting the timer. The path is
escaped through `safepresentation.EscapePath` and left-truncated
with a leading `…` to fit the terminal width. Added `popupOpen`
/`popupPath`/`popupInstance`/`popupDuration` fields; a process-wide
`popupInstanceCounter`; `FileChangePopupExpiryMsg{Instance}`;
`startPopup`/`dismissPopup`/`cancelPopup` helpers;
`WithPopupDuration` option (test seam; production default 1 second,
0 for instant test timer); `PopupOpen`/`PopupPath`/`PopupInstance`
accessors; `handleNavigate` calls `startPopup` on cross-file
navigation before the cached/uncached branch (uncached batches the
timer and load via `tea.Batch`); `Update` dismisses on
matching-instance expiry; keypress dismissal before normal key
routing; `SearchCompleteMsg` error-overlay path calls `cancelPopup`;
`View` renders the pop-up via `renderPopup` below the error overlay;
`truncateLeftCells` helper. Tests added:
`internal/app/popup_test.go` (open on cross-file n, no open on
same-file/one-stop, matching/stale expiry, fresh instance, keypress
dismissal with normal action, q still quits, scroll still scrolls,
Esc dismisses, resize recentres/retruncates, error-overlay
cancellation, load completion does not restart timer, centred
rendering, long-path truncation, sink-safety across all hostile
fixtures including invalid UTF-8 via bytes-encoded begin/end
records, no return after expiry, no return after keypress). Updated
`deliverLoad` in `navigation_test.go` to handle `tea.BatchMsg`
with a short timeout so blocking timer commands don't stall tests.
Updated [browse-tracer](browse-tracer.md) (file-change pop-up
section), [source-code](source-code.md) (internal/app entry),
[unit-tests](unit-tests.md) (popup_test.go catalog), and the index.
Sources: `Notes/tasks/015-file-change-popup.md`,
`Notes/PRD-vrg.md` (File-change pop-up), `internal/app/app.go`,
`internal/app/popup_test.go`, `internal/app/navigation_test.go`.

## [2026-09-11] ingest | Issue #16 wrap mode and grapheme policy

Ingested the completed Issue #16 implementation: wrapping on by
default with `w` toggling between wrap and run-off-edge modes; the
shared grapheme segmentation and cell-width policy in
`internal/safepresentation` (`Cluster{StartByte, EndByte, Width}` and
`GraphemeClusters` using `github.com/rivo/uniseg` and
`uniseg.StringWidth`); `filebuffer.Cluster` alias and
`Line.Clusters`/`StartByte`/`Continuation` populated by `Load`;
eight-column tab expansion in `EscapeContent` replacing Issue #5's
`→` placeholder; the prepared swappable `RowModel` keyed by
`RowModelKey{Path, Revision, TextWidth, WrapMode}`; `BuildRowModel`
wrapping at grapheme-cluster boundaries (two-cell clusters that
don't fit move to the next row, leaving a blank); `RowFromByte`
mapping a source line and byte offset to the wrapped row;
`ReservedWidth` (0 in wrap, 1 in run-off-edge for the Issue #20
indicator); `TextWidth`; `Viewport.RowCount()`; the `w` key toggle
in browse mode with viewport rebuild; `WindowSizeMsg` row-model
rebuild when the text width changes; `targetRow(stop)` using
`RowModel.RowFromByte` for the wrapped destination reveal; and
`renderContentPanel` showing a blank gutter for continuation rows.
Created [wrap-mode-and-grapheme-policy](wrap-mode-and-grapheme-policy.md);
updated [source-code](source-code.md) (safepresentation, filebuffer,
viewport, and app entries), [unit-tests](unit-tests.md) (wrap_test.go
in viewport and app, grapheme_test.go, safepresentation_test.go
additions), and the index. Sources:
`Notes/tasks/016-wrap-mode-and-toggle.md`,
`Notes/PRD-vrg.md` (Wrap mode and grapheme policy),
`internal/safepresentation/safepresentation.go`,
`internal/safepresentation/safepresentation_test.go`,
`internal/filebuffer/filebuffer.go`,
`internal/filebuffer/grapheme_test.go`,
`internal/viewport/viewport.go`,
`internal/viewport/wrap_test.go`,
`internal/app/app.go`,
`internal/app/wrap_test.go`,
`internal/app/browse_test.go`.

## [2026-09-11] ingest | Issue #16 walkthrough bug fixes

During the Issue #16 code walkthrough, two bugs were found and fixed:

1. **Continuation flag for intermediate wrapped rows**: `wrapLine` in
   `internal/viewport/viewport.go` only set `Continuation=true` for the
   last wrapped row, not all continuation rows. Intermediate wrapped
   rows were incorrectly marked as non-continuation, causing them to
   show the line number in the gutter instead of a blank gutter. Fixed
   by passing `len(result) > 0` instead of `false` when breaking a row.
   Added `TestWrapContinuationRowsThreePlus` to catch the bug with 3+
   wrapped rows.

2. **Text width calculation in buildViewport**: `buildViewport` in
   `internal/app/app.go` used `m.width` (terminal width) instead of
   the content panel width (`m.width - fileListWidth(m.width) - 1`)
   for the `TextWidth` calculation. This caused wrapping to use a
   wider width than the visible content area, clipping text off the
   right edge. Fixed by subtracting the file list width and separator.
   Updated `TestWrappedTargetRevealLongLine` to use terminal width 50
   (panel width 29, text width 25) with corrected expectations.

Updated [wrap-mode-and-grapheme-policy](wrap-mode-and-grapheme-policy.md)
(continuation rows and text width sections), [source-code](source-code.md)
(viewport and app entries), and [unit-tests](unit-tests.md)
(`TestWrapContinuationRowsThreePlus`).

## [2026-09-11] ingest | Issue #17 logical anchor and layout preparation

Ingested the completed Issue #17 implementation: the width-independent
logical viewport anchor (`viewport.Anchor{LineIndex, Column}`) with
`RowModel.RowFromCell`/`RowModel.RowAnchor` and anchor-aware
`Viewport.SetRows`/`SetPanelHeight`/`SetOffset`; anchor retention through
rewrap, wrap toggle, and resize; anchor replacement by user scrolling
and moving reveals; intentionally lossy EOF-clamp anchor replacement;
off-UI layout preparation via `buildViewport()` returning a `tea.Cmd`
that emits `LayoutReadyMsg`; keyed installation guards (installs only
when the key matches current parameters, discards out-of-order/stale
completions); model-carried pending reveal intent (`pendingReveal`)
committed on layout installation; cached-file stale-layout navigation
with matching-layout fast path (`layoutCache`); AC6 responsiveness
during gated preparation (`WithLayoutGate` test seam); render-cost
guard for visible file-list entries (`WithFileListProvider` test seam,
`renderBrowse` limits iteration to `min(fileCount, terminalHeight)`);
and app integration (`prevBuildPath`, `LayoutKey()`, `WrapMode()`,
`HasPendingLayout()`, `PendingLayoutKey()`, `HasPendingReveal()`
accessors, `WithRowModelFactory`/`WithFileListProvider` options,
cross-file navigation sets `viewport = nil` for fresh-load anchor
restoration). Created
[logical-anchor-and-layout-preparation](logical-anchor-and-layout-preparation.md);
updated [wrap-mode-and-grapheme-policy](wrap-mode-and-grapheme-policy.md)
(`w` toggle and app integration section), [index](index.md),
[source-code](source-code.md) (viewport and app entries), and
[unit-tests](unit-tests.md) (anchor_test.go and layout_test.go entries).
Sources: `Notes/tasks/017-logical-anchor-through-rewrap-and-resize.md`,
`Notes/PRD-vrg.md` (Navigation, viewport, and logical anchors;
Resources and responsiveness), `internal/viewport/viewport.go`,
`internal/viewport/anchor_test.go`, `internal/app/app.go`,
`internal/app/anchor_test.go`, `internal/app/layout_test.go`.

## [2026-09-11] ingest | Issue #18 horizontal panning

Ingested the completed Issue #18 implementation: horizontal panning in
run-off-edge mode with `,`/`.` (one column), `<`/`>` (ten columns), and
`[`/`]` (half text width) pan units; the paintable-boundary maximum
(`max(0, S)` where `S` is the largest cluster start of the widest
visible line whose width fits the text width); three distinct width
definitions (content extent, extent policy over visible rows only,
maximum valid offset); visible-set re-clamping on vertical scroll,
reveal, resize, list hide/show, gutter growth, and wrap-toggle
re-entry; no restoration after destructive leftward clamping (lossy,
mirroring the Issue #17 vertical EOF clamp); offset retention through
wrap toggles with re-entry clamping; reset to zero on file change;
grapheme-safe clipping with blank cells for split clusters at the clip
edge; forward-compatible extent definition for the future Issue #23
end-of-line marker cells; the visible-row render-cost guard (extent
computation queries only `Viewport.Visible`); and app integration
(`handlePanKey` routing, `SetLayout` on every viewport installation,
`SetHOffset`/`ResetHorizontal` on rebuilds and file changes,
`ClipLine` in `renderContentPanel`, `ViewportHOffset()` accessor).
Created [horizontal-panning](horizontal-panning.md); updated
[index](index.md), [source-code](source-code.md) (viewport and app
entries), and [unit-tests](unit-tests.md) (pan_test.go viewport and
app entries). Sources:
`Notes/tasks/018-horizontal-panning.md`,
`Notes/issues/018-horizontal-panning.md`,
`Notes/PRD-vrg.md` (Navigation, viewport, and logical anchors;
Layout and indicators), `internal/viewport/viewport.go`,
`internal/viewport/pan_test.go`, `internal/app/app.go`,
`internal/app/pan_test.go`.

## [2026-09-12] ingest | Issue #19 minimal horizontal reveal

Ingested the completed Issue #19 implementation: minimal horizontal
reveal of the first-submatch start cell in run-off-edge mode. The
viewport `RevealHorizontal(line, targetCell)` adjusts the horizontal
offset so the target's grapheme cluster is painted — no-op in wrap
mode, no-op when already painted (fully within the window, not split
by either clip edge), right-edge arithmetic for right-side reveal
(`offset = target + clusterWidth - textWidth`), left-edge for
left-side (`offset = target`), geometric fallback for unpaintable
clusters (offset = target, no clamp, idempotent to avoid panning
loops). `ClusterWidthAtCell(clusters, cell)` derives the target
cluster width. The app's `revealTarget` calls `RevealHorizontal`
after the vertical reveal in run-off-edge mode, deriving the target
cell from `line.ByteCells[sm.Start][0]`. The reveal triggers on
startup after the initial file loads and on every actual
match-navigation transition, after the new viewport/layout is
installed and after the Issue #18 file-change horizontal reset.
`WithWrapMode` option starts the app in run-off-edge mode for tests.
Created [horizontal-reveal](horizontal-reveal.md); updated
[index](index.md), [source-code](source-code.md) (viewport and app
entries), and [unit-tests](unit-tests.md) (reveal_horizontal_test.go
viewport and app entries). Sources:
`Notes/tasks/019-minimal-horizontal-reveal.md`,
`Notes/PRD-vrg.md` (Navigation, viewport, and logical anchors),
`internal/viewport/viewport.go`,
`internal/viewport/reveal_horizontal_test.go`,
`internal/app/app.go`, `internal/app/reveal_horizontal_test.go`.

## [2026-09-12] ingest | Issue #20 hidden-content indicators

Ingested the completed Issue #20 implementation: run-off-edge
hidden-content indicators populating the column reserved by Issue #16.
`renderContentPanel` (in `internal/app/app.go`) now emits, in
run-off-edge mode only, a left `_`/`*` in the first trailing gutter
space (text hidden left vs. match entirely hidden left) and a right
`*` in the reserved rightmost column on the current matched line's
visible row when a match is entirely hidden right. Helpers:
`leftIndicator` (left `_`/`*`/blank), `hasHiddenMatchRight` (right
`*` predicate), `highlightHasVisibleCells` (visibility over fully
visible non-split clusters only), `lineHasContent` (non-zero-width
cluster check), `clusterCellWidth` (text-area padding so the right
indicator never overwrites text). Visibility is derived from the
actually rendered cells after grapheme clipping — a split wide glyph
rendered as blanks does not count as visible match content — and the
reserved column is excluded from visibility calculations. Partial
visibility on a side produces no hidden-match indicator for that side.
The right indicator is absent when the current matched line is
vertically off-screen; other lines' left indicators remain. Both
indicators use `Theme.Indicator` (inverse match colours restored to
base). Wrap mode draws neither indicators nor a reserved right
column. Tests in `internal/app/indicator_test.go` cover all contracts
through observable Bubble Tea `Update` and rendered output. Created
[hidden-content-indicators](hidden-content-indicators.md); updated
[index](index.md), [source-code](source-code.md) (internal/app entry),
and [unit-tests](unit-tests.md) (indicator_test.go entry). Sources:
`Notes/tasks/020-hidden-content-indicators.md`,
`Notes/PRD-vrg.md` (Layout and indicators section),
`internal/app/app.go`, `internal/app/indicator_test.go`.

## [2026-04-21] ingest | Issue #21 grapheme cluster highlight expansion

Issue #21: match highlights now expand to grapheme-cluster boundaries so
they never split a cluster. `filebuffer.Load` remaps `ByteCells` via
`expandedByteCells` so every byte in a cluster (including combining
marks and ZWJ joiners) maps to the cluster's full cell range, and
converts each submatch to a display-cell span via `expandedHighlights`
that expands outward to the enclosing clusters' cell boundaries.
Combining-only matches highlight the whole base cluster; standalone
zero-width clusters receive a visible fallback cell; wide glyphs and
ZWJ sequences are never split; multi-cell escaped forms (ESC → `^[`)
preserve their original cell range. The expanded `Highlights` and
`ByteCells` are the single source for Viewport, App, Issue #19 reveal,
and Issue #20 indicators. `safepresentation.ContentDisplay` gained
`ByteOffsets` (display byte offset per raw byte) so `filebuffer` can
map raw bytes to grapheme clusters by display byte range (a combining
mark's cell is outside its cluster's cell range). `viewport.clipLineToWindow`
now tracks `paintableRanges` (fully-visible non-split cluster cell
ranges) and `clipHighlightsToPaintable` intersects highlights with them
so split-blank filler cells are never painted as match cells (replacing
`clipHighlights`). Tests in `internal/filebuffer/grapheme_highlight_test.go`,
`internal/viewport/grapheme_highlight_test.go`, and
`internal/app/grapheme_indicator_test.go` cover the expansion,
wrap/clip blank exclusion, and indicator behavior through the
production `filebuffer.Load` path. Created
[grapheme-cluster-highlight-expansion](grapheme-cluster-highlight-expansion.md);
updated [index](index.md), [source-code](source-code.md)
(internal/safepresentation, internal/filebuffer, internal/viewport
entries), and [unit-tests](unit-tests.md) (grapheme_highlight_test.go
entries for filebuffer and viewport, grapheme_indicator_test.go entry
for app). Sources: `Notes/tasks/021-grapheme-cluster-highlight-expansion.md`,
`Notes/PRD-vrg.md` (Text, graphemes, and safe presentation),
`internal/filebuffer/filebuffer.go`,
`internal/safepresentation/safepresentation.go`,
`internal/viewport/viewport.go`,
`internal/filebuffer/grapheme_highlight_test.go`,
`internal/viewport/grapheme_highlight_test.go`,
`internal/app/grapheme_indicator_test.go`.

## [2026-09-12] ingest | Issue #22 structural line handling

Issue #22: FileBuffer structural line handling with raw/search/display
coordinate separation. `Load` now detects and strips a leading UTF-8 BOM
(`EF BB BF`) before `splitLines` so rg-line and raw-file coordinates
are kept separate; `Buffer.BOMOffset` records the strip length (0 or 3)
for converting back to raw-file coordinates (e.g. Issue #29 stale
validation). A first-line match at rg offset 0 maps to raw byte 3 and
highlights the correct display cell. The BOM adjustment applies only to
line 1; line 2 and beyond have no adjustment. A BOM-only file produces
zero lines; a BOM plus a newline produces one empty line. Non-leading
U+FEFF is ordinary content (displayed, counted, matchable). LF and CRLF
terminate lines and are not displayed; original line bytes including
terminators are retained in `Line.ByteCells` for byte-coordinate mapping
and later validation. A standalone CR (not followed by LF) is escaped as
`^M` by the safe-presentation core. A missing final newline yields a
final line; a trailing newline does not invent an extra empty line; an
empty file has zero lines with a three-cell gutter (one digit slot plus
two spaces) and no source rows. Terminator bytes and zero-width
positions map to the display end-of-line column (byte 4 of `hit\r\n` →
display column 3). A span covering visible text plus terminator
highlights only the visible text: `expandedHighlights` now uses the end
byte's original cell end when the end byte has no cluster (a terminator
mapping to the zero-width end-of-line position), instead of falling
back to the start cluster's cell end. The end-of-line marker for
terminator-only matches is owned by Issue #23; the stale validation
that consumes the retained bytes is owned by Issue #29. Tests in
`internal/filebuffer/structural_line_test.go` cover mixed terminators,
CRLF/LF not displayed, terminator-to-EOL mapping, text-plus-terminator
spans, standalone CR escape, leading BOM invisible with first-line
match at rg offset 0 → raw byte 3, BOM-only file, BOM plus newline,
non-leading U+FEFF as content, empty file, unterminated final line, no
phantom trailing line, and retained original bytes. Created
[structural-line-handling](structural-line-handling.md); updated
[index](index.md), [source-code](source-code.md) (internal/filebuffer
entry), and [unit-tests](unit-tests.md) (structural_line_test.go entry).
Sources: `Notes/tasks/022-line-terminators-final-line-empty-file-utf8-bom.md`,
`Notes/PRD-vrg.md` (Text, graphemes, and safe presentation; Encodings
and stale-content validation), `internal/filebuffer/filebuffer.go`,
`internal/filebuffer/structural_line_test.go`.

## [2026-09-13] ingest | Issue #23 zero-width match markers

Issue #23: zero-width regex submatches (`Start == End`) render as one
inverse-video cell at their mapped display location, underlined on
the current matched line, marking an existing cell without shifting
following text. A marker at end of line extends the effective line
width by one cell, so an empty matched line has width one and a marker
after a completely full wrap row occupies another row. A zero-width
position inside a grapheme cluster maps to the cluster start with no
split wide glyph. A terminator-only `$` match on `hit\r\n` (byte 4)
produces a single marker cell at display column 3 following exactly
the same reveal, wrap, clip, horizontal-extent, and indicator rules as
any other marker, including counting as entirely hidden for the
Issue #20 gutter `*` and right `*` indicators. Marker cells are
navigable reveal targets. Markers participate in horizontal extent
and pan clamping so a marker-only line has extent 1 and maximum offset
0 under Issue #18's paintable-boundary maximum.

`filebuffer.Load` produces the markers:
`markerCellsForStops(stops, byteCells, clusters)` returns the display
cell positions of zero-width submatches, mapping each through the
expanded `ByteCells` (cluster-start mapping via Issue #21) and
deduplicating; `clusterContentWidth(clusters)` returns the sum of
cluster widths (the content extent before any EOL marker extension);
for each marker cell at the content width (EOL), `Load` appends a
space to `Display` and a 1-cell `safepresentation.Cluster` to
`Clusters` so the marker has a paintable cell; each marker cell is
appended to `Highlights` as `[cell, cell+1)` and `Highlights` are
sorted by start cell so rendering processes them in cell order. The
Viewport needs no marker-specific changes: the existing
cluster-driven wrap model, clipping, extent, pan clamping, and
reveal arithmetic operate on the marker's virtual cluster and
one-cell highlight like any other cluster and highlight. The App's
`renderLineWithHighlights` paints the marker space with the match or
current-match style, and the indicator functions consume the
one-cell highlight like any other.

Tests in `internal/filebuffer/marker_test.go` cover BOL/EOL/empty-line
markers, LF and CRLF terminator-only markers (column 3), wide and
combining cluster-start mapping, no text shifting, mid-line no-width-
extension, coexistence with non-zero-width highlights, and the EOL
cluster being last with width 1. Tests in
`internal/viewport/marker_test.go` cover wrap-row occupation,
marker-only-line extent 1, max pan offset 0, reveal targeting,
hidden-left/right clip exclusion, visible clip inclusion, pan
clamping participation, and same-row wrap behavior. Tests in
`internal/app/marker_indicator_test.go` cover hidden-left `*`,
hidden-right `*`, visible no-indicator, terminator-only `*`, and
marker-only-line always-visible behavior. Created
[zero-width-match-markers](zero-width-match-markers.md); updated
[index](index.md), [source-code](source-code.md) (internal/filebuffer
entry), and [unit-tests](unit-tests.md) (marker_test.go entries for
filebuffer and viewport, marker_indicator_test.go entry for app).
Sources: `Notes/tasks/023-zero-width-match-markers.md`,
`Notes/issues/023-zero-width-match-markers.md`,
`Notes/PRD-vrg.md` (Text, graphemes, and safe presentation),
`internal/filebuffer/filebuffer.go`,
`internal/filebuffer/marker_test.go`,
`internal/viewport/marker_test.go`,
`internal/app/marker_indicator_test.go`.

## [2026-09-13] ingest | Issue #24 file list layout, truncation, and toggle

Issue #24: replaced the Issue #5 placeholder file-list width with a
responsive formula and added hide/show toggles, grapheme-safe left
truncation, active-entry auto-scroll, and a filename-row buffer-status
note slot. `ComputeListWidth(termWidth, longestPathWidth, gutterWidth,
reservedIndicator, visible)` returns the nonnegative minimum of
`longestPathWidth+2`, `floor(0.40×termWidth)`, and
`termWidth−(gutterWidth+10+reservedIndicator)`; 0 when not visible.
`longestPathWidth` is computed from sanitized index paths measured
through the shared grapheme policy. The visibility preference
(`listVisible`, initially true) is distinct from the computed width:
a computed zero width draws no list cells but does not change the
preference or auto-toggle. Toggles: `left`/`tab` hide, `right`/
`shift+tab` show. `toggleListVisible` rebuilds via `buildViewport` so
the relayout goes through Issue #17's prepared-layout path with anchor
preservation. `TruncateLeftGrapheme(s, maxCells)` left-truncates with
a leading `…` via `safepresentation.GraphemeClusters`, never splitting
grapheme clusters. `updateListOffset`/`currentFileIndex` keep the
active entry visible on navigation and render. `renderBrowse` queries
only the visible file-list window. `renderFilenameRow` adds a
buffer-status note slot (path truncated to fit the panel width), with
the real note texts owned by Issues #26/#29/#30 and a synthetic
`WithStatusNote` test seam. `LayoutKey()` uses `m.ListWidth()` so
every text-width change flows through Issue #17. Removed the old
placeholder `fileListWidth` function. Updated
`TestViewportAnchorOnResize` and `TestWrappedTargetRevealLongLine` for
the new width formula. Tests in `internal/app/filelist_layout_test.go`
cover the formula (each term winning, 40% floor rounding, zero when
hidden, nonnegative clamp), grapheme-safe truncation (ASCII, wide,
combining, no-truncate-when-fits), toggles (tab/left hide,
shift+tab/right show), anchor preservation through toggle and resize,
zero-width visibility preference, active-entry auto-scroll,
visible-window render-cost guard, filename-row status-note slot, and
list-entry/filename-row truncation. Created
[file-list-layout](file-list-layout.md); updated [index](index.md),
[source-code](source-code.md) (internal/app entry), and
[unit-tests](unit-tests.md) (filelist_layout_test.go entry). Sources:
`Notes/tasks/024-file-list-layout-width-truncation-toggle.md`,
`Notes/PRD-vrg.md` (Layout and indicators; Navigation, viewport, and
logical anchors; Resources and responsiveness; Text, graphemes, and
safe presentation), `internal/app/app.go`,
`internal/app/filelist_layout_test.go`,
`internal/app/anchor_test.go`, `internal/app/wrap_test.go`.

## [2026-09-12] ingest | Issue #25 asynchronous load isolation

Ingested the completed Issue #25 implementation: keyed asynchronous
load isolation so the TUI stays navigable while file loads are
pending, late completions update only their own file, and at most one
load is in flight per raw path. `FileLoadCompleteMsg` now carries a
`RequestID uint64` identifying the load request that produced it.
`Model.loadRequestID` is the monotonically increasing request identity
counter. `Model.loadingPaths map[string]uint64` tracks in-flight loads
by raw path, mapping to the active request ID. `startLoad(path)
(Model, tea.Cmd)` is the single entry point for starting a load: it
enforces one-load-per-path (checks `loadingPaths`, returns nil if a
load is already in flight), assigns a fresh request ID, records the
in-flight request, and delegates to `loadFileFor(path, requestID)`.
The `FileLoadCompleteMsg` handler validates the request ID against
`loadingPaths` before mutating state (stale completions from
cancelled or superseded requests are discarded), removes the path
from `loadingPaths`, caches the buffer regardless of whether the
path is current, and updates the visible panel only when the
completion's path is still the current path. The `SearchCompleteMsg`
handler sets `m.currentPath` to the startup file's raw path before
starting the load so the keyed completion handler can identify it as
current. `handleNavigate` calls `startLoad` instead of `loadFileFor`
directly so the one-load-per-path rule is enforced on re-entry of a
loading path. Tests in `internal/app/load_isolation_test.go` cover
navigation during loads, placeholder scroll no-op, `w`/`c`/resize
normal meaning while loading, keyed late completions for non-current
files (A→B→C scenario), keyed completion for current path,
one-load-per-path with dropped re-entry, cached revisit without
reload, cache retention across multiple visits, post-cancellation
rejection (ctrl+c and q), and input responsiveness during the
decode/map phase (n, p, w, c, resize, ctrl+c all actionable while
the file gate is held). Created
[async-load-isolation](async-load-isolation.md); updated [index](index.md),
[browse-tracer](browse-tracer.md) (FileLoadCompleteMsg, model fields,
loadFile/loadFileFor, navigation cursor, FileLoadCompleteMsg handler),
[source-code](source-code.md) (internal/app Issue #25 entry), and
[unit-tests](unit-tests.md) (load_isolation_test.go entry). Sources:
`Notes/tasks/025-async-load-isolation.md`,
`Notes/PRD-vrg.md` (File loading, cache, reload, and selection
consistency), `internal/app/app.go`,
`internal/app/load_isolation_test.go`.

## [2026-10-19] ingest | Issue #26 read failures and retry rules

Ingested the completed Issue #26 implementation: read-failure state
and the `(unreadable)` placeholder, the non-fatal read-failure
overlay with sanitized diagnostics, non-current diagnostic-only
collection, the same-file-step no-retry versus cross-file
exactly-one retry distinction, the five-step re-entry sequence,
composed-view robustness, and the fixed-status guarantee. The
`Model` gained `readFailed bool` (current file's last load attempt
failed; rendering shows `(unreadable)` instead of `Loading…`),
`failedPaths map[string]string` (failed raw paths to sanitized
diagnostics; a successful load removes the path), and
`overlayReadFailure bool` (the open overlay is a read-failure
overlay). `openReadFailureOverlay` opens a fresh non-fatal
`OverlayError` for a current-file load failure (or appends to an
existing read-failure overlay on a re-entry retry failure without
resetting `overlayScroll`), with search-complete overlays taking
precedence and the file-change pop-up cancelled on open. The
`FileLoadCompleteMsg` handler now distinguishes errors from
successful buffers while retaining the Issue #25 request-ID and
current-path isolation: current-file errors mark the file
unreadable and collect the diagnostic; non-current errors record
the diagnostic in `failedPaths` and collect it without disturbing
the visible panel; the fixed `ExitCode` is not recomputed. Overlay
dismissal clears `overlayReadFailure`. The filename row uses
`(unreadable)` as the default status note when the current file
has failed and no `WithStatusNote` callback overrides it;
`truncateRightCells` truncates the status note when it would
overflow the panel (reserving at least one cell for the path).
`handleNavigate` reopens the prior-failure overlay immediately on
re-entry into a previously failed file (clearing `readFailed`,
setting `loading`, and starting exactly one retry load while the
overlay is open, reusing the Issue #25 one-load-per-path rule).
Public accessors `OverlayFatal()`, `OverlayText()`,
`OverlayScroll()`, `IsLoading()`, and `ReadFailed()` were added.
Tests in `internal/app/read_failure_test.go` cover the current-file
overlay and placeholder, non-current diagnostic-only collection,
same-file versus cross-file retry, composed-view robustness, and
the outcome-matrix rows (fixed-0 all-fail, fixed-2 current-file,
composed all-fail-with-fixed-2). Tests in
`internal/app/reentry_test.go` cover the five-step re-entry
sequence: immediate prior-failure overlay reopen with `Loading…`,
exactly one in-flight retry, Esc-without-disturbance, settlement
presentation, append-preserving-scroll on second failure,
navigation away during retry, and one-load-per-path drop on
re-entry while in flight. Created
[read-failures-and-retry](read-failures-and-retry.md); updated
[index](index.md), [source-code](source-code.md) (internal/app
Issue #26 entry), and [unit-tests](unit-tests.md)
(read_failure_test.go and reentry_test.go entries). Sources:
`Notes/tasks/026-read-failures-unreadable-retry-rules.md`,
`Notes/issues/026-read-failures-unreadable-retry-rules.md`,
`Notes/PRD-vrg.md` (File loading, cache, reload, and selection
consistency), `internal/app/app.go`,
`internal/app/read_failure_test.go`,
`internal/app/reentry_test.go`.

## [2026-05-20] ingest | Issue #27 explicit reload (r)

Issue #27 added the `r` explicit reload: pressing `r` rereads the
current file exactly once without rerunning ripgrep, without changing
cursor stops, and without revealing a match. The reload shows the
`Loading…` placeholder while pending, replaces stale content on
failure with `(unreadable)`, supports retries and the one-stop index,
and discards superseded layouts safely through content revisions and
the Issue #17 installation guard. Implementation in
`internal/app/app.go`: `handleReload` records the path in
`reloadingPaths` so the `FileLoadCompleteMsg` handler increments the
per-path content revision in `revisions` (default `1`, incremented
only for reloads); `LayoutKey()` now uses `contentRevision(path)`
instead of the hardcoded `1` so stale cached layouts from a prior
revision are discarded by the Issue #17 installation guard; a
`pendingReloadAnchor` model field carries the reload-anchor intent
(preserve the anchor, no reveal) from load completion to the matching
layout installation; `handleOverlayKey` routes `r` through a
read-failure overlay so the user can retry without dismissing it;
`HasPendingReloadAnchor()` exposes the intent for tests. Tests in
`internal/app/reload_test.go` cover the reload lifecycle, dropped
duplicate reloads, dropped re-entry while the same path is loading,
the `Loading…` placeholder, exactly one reread, no cursor-stop
changes, anchor preservation after the matching new layout installs,
clamping on content shrink, failure replacement with
`(unreadable)`, the current-file failure overlay, second-failure
append with overlay scroll preserved, the one-stop index route, no
reload on simulated disk change, the filename row retaining the
path, content revision advancement, and the Issue #17
revision-supersession discard of a gated pre-reload layout released
after reload completion. Created [explicit-reload](explicit-reload.md);
updated [index](index.md), [source-code](source-code.md)
(internal/app Issue #27 entry), and [unit-tests](unit-tests.md)
(reload_test.go entry). Sources:
`Notes/tasks/027-explicit-reload-r.md`,
`Notes/issues/027-explicit-reload-r.md`,
`Notes/PRD-vrg.md` (File loading, cache, reload, and selection
consistency), `internal/app/app.go`,
`internal/app/reload_test.go`.

## [2026-09-10] ingest | Issue #28 two-stage load completion

Ingested the completed Issue #28 implementation: the two-stage
load-completion contract separating file-load completion (stage one)
from viewport reveal/reload-anchor decisions (stage two). Added the
`LoadIntent` enum (`IntentNone`, `IntentReveal`, `IntentReloadAnchor`)
and `loadIntent` model field, replacing the Issue #14 `needsReveal`
flag, the Issue #17 `pendingReveal` flag, and the Issue #27
`pendingReloadAnchor` flag. Added `commitLoadIntent()` to commit the
pending intent against the installed rows at stage two. Navigation
during a pending reload replaces `IntentReloadAnchor` with
`IntentReveal` so the latest selection wins. Stale layouts are
discarded without consuming or mutating the intent. Created
[load-completion-two-stage](load-completion-two-stage.md); updated
[destination-reveal](destination-reveal.md),
[logical-anchor-and-layout-preparation](logical-anchor-and-layout-preparation.md),
[explicit-reload](explicit-reload.md), and the index. Sources:
`Notes/tasks/028-load-completion-reveal-latest-target.md`,
`Notes/PRD-vrg.md` (File loading, cache, reload, and selection
consistency), `internal/app/app.go`,
`internal/app/two_stage_test.go`,
`internal/app/reload_intent_test.go`.

## [2026-09-12] ingest | Issue #29 stale-match validation and file-changed note

Ingested the completed Issue #29 implementation: best-effort
per-submatch stale-match validation in `filebuffer.Load`, a
`Buffer.Stale` flag, per-line survivor and fallback metadata
(`Line.RawBytes`, `ContentWidth`, `HasMarker`, `ValidStarts`,
`FirstRecordedStart`), a `Buffer.RevealTarget(stop)` method exposing
the validated reveal target (first surviving submatch, clamped
recorded start with end-of-line cell clamping, or last source line),
integration of the `file changed since search` note into the Issue #24
filename-row status slot (persistent, no timer, cleared only by a
reload that fully validates), and integration of the validated reveal
target into the Issue #28 two-stage reveal path (`revealTarget` and
`targetRow` now use the buffer's `RevealTarget` instead of
`stop.Submatches[0]`). Validation checks line existence, range
validity, and byte equality against the original line bytes
(including terminators, BOM-adjusted per Issue #22); invalid
submatches are dropped individually, surviving submatches keep their
highlights, and the buffer is marked stale. The fixed search-derived
exit status is never altered by stale content (all-stale
outcome-matrix row keeps exit 0). UTF-16/UTF-32 are excluded (Issue
#30). Created [stale-match-validation](stale-match-validation.md);
updated [index](index.md), [source-code](source-code.md)
(internal/filebuffer and internal/app Issue #29 entries), and
[unit-tests](unit-tests.md) (stale_validation_test.go and
stale_note_test.go entries). Sources:
`Notes/tasks/029-stale-match-validation-and-file-changed-note.md`,
`Notes/PRD-vrg.md` (Encodings and stale-content validation),
`internal/filebuffer/filebuffer.go`,
`internal/filebuffer/stale_validation_test.go`,
`internal/app/app.go`,
`internal/app/stale_note_test.go`.

## [2026-09-12] ingest | Issue #30 unsupported encodings UTF-16/UTF-32

Ingested the completed Issue #30 implementation: UTF-16/UTF-32 BOM
detection in `filebuffer.Load` via `detectUnsupportedBOM(data)`, with
longer-before-shorter overlap ordering so `FF FE 00 00` classifies as
UTF-32 LE rather than UTF-16 LE (UTF-32 LE/BE checked before UTF-16
LE/BE); a placeholder `Buffer` with `UnsupportedEncoding = true`,
`EncodingDiagnostic` set, no lines, no highlights, and `Stale = false`
(stale-match guard never runs against raw encoded bytes); a leading
UTF-8 BOM (`EF BB BF`) is not unsupported and falls through to the
existing Issue #22 strip path. App integration: an
`unsupportedEncoding bool` model flag mirrors `readFailed`; a
current-file unsupported buffer opens the Issue #9 overlay with the
encoding diagnostic (non-fatal), shows `(unsupported encoding)` in the
panel and the Issue #24 status slot, collects the diagnostic for
replay, and builds no viewport; a non-current unsupported buffer is
diagnostic-only; `handleReload` clears the flag (showing `Loading…`
then re-detecting); cached unsupported destinations open the overlay
through the cached-destination path in `handleNavigate`. The
`(unsupported encoding)` placeholder is truncated to the panel width
so the composed view never overflows at constrained widths. The
fixed search-derived exit status is never altered by unsupported
encodings (all-unsupported outcome-matrix row keeps exit 0). Ripgrep
invocation is unchanged (no forced encoding flag; default BOM
detection remains enabled). Created
[unsupported-encodings](unsupported-encodings.md); updated
[index](index.md), [source-code](source-code.md) (internal/filebuffer
and internal/app Issue #30 entries), and [unit-tests](unit-tests.md)
(encoding_test.go entries for both packages). Sources:
`Notes/tasks/030-unsupported-encodings-utf16-utf32.md`,
`Notes/PRD-vrg.md` (Encodings, Stale-content validation, Invocation),
`internal/filebuffer/filebuffer.go`,
`internal/filebuffer/encoding_test.go`,
`internal/app/app.go`,
`internal/app/encoding_test.go`.

## [2026-09-12] ingest | Issue #31 help overlay h/? with wrapped scrollable key bindings

Ingested the completed Issue #31 implementation: the modal help overlay
opened with `h`/`?` from ordinary browsing and the no-results screen.
The help overlay uses the same `renderOverlay` component as the Issue
#9 error overlay (base colours, plain single-line border, wrapped text
including unbroken strings, vertical scrolling) but skips the head/tail
compression so all rows are reachable by scrolling. Key routing while
open: `up`/`down` scroll, `q`/`Esc`/`h`/`?` close (returning to the
underlying base state), `ctrl+c` exits 130, and every other key
(including `n`/`p`/`w`/`c`/`r`) is ignored with the state behind
unchanged. The `OverlayHelp` kind was added to `OverlayKind`. The
key-binding list is defined once as data: `KeyBinding{Key, Description}`
struct, `KeyBindings()` returning the single binding-table data source
covering navigation, scrolling, panning, wrap, colour, list toggle,
reload, help, and quit/cancel bindings, and `HelpFooter()` returning the
footer slot reserved for Issue #34. `helpText()` builds the overlay text
from the binding table plus footer. `openHelp()` opens the overlay and
cancels any active Issue #15 file-change pop-up with no return on close.
`handleOverlayKey` was extended to close the help overlay on `h`/`?`
(ignored by error/warning overlays). At tiny sizes the overlay is
clipped to the terminal without a borderless mode (overlay width capped
to terminal width, visible height derived from terminal height) and
restored on growth; rendering does not panic at 25×8. The help overlay
passes the Issue #6 sink-safety check (no dangerous control bytes in
the no-style path; no fixture payload after unescaped ESC with styles).
Created [help-overlay](help-overlay.md); updated
[index](index.md), [source-code](source-code.md) (internal/app Issue
#31 entry), and [unit-tests](unit-tests.md) (help_overlay_test.go
entries). Sources:
`Notes/tasks/031-help-overlay.md`,
`Notes/PRD-vrg.md` (Colours, overlays, and key precedence),
`internal/app/app.go`,
`internal/app/help_overlay_test.go`.

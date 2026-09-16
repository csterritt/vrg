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

## [2026-09-12] ingest | Issue #32 overlay precedence and Esc semantics

Ingested the completed Issue #32 implementation: the full overlay
precedence stack (`ctrl+c` over modal error over help over pop-up over
base keys), error-suspends-help with scroll position restoration,
generalized append-preserving-scroll for all appended errors, pop-up
cancellation by help and error with no return, `Esc` no-op with no
overlay, and the dismissal-outcome table for both `q` and `Esc`.
`internal/app` added `suspendedHelp`/`suspendedHelpScroll` fields and
the `HelpSuspended()` accessor. `openReadFailureOverlay` was
generalized: a read failure while help is open suspends help (saving
its scroll position) and opens the error overlay; a read failure while
any error/warning overlay is open appends to it without resetting the
scroll position and marks the overlay as a read-failure overlay so `r`
(Issue #27) can retry (replacing the Issue #26 rule that a
search-complete overlay takes precedence over read failures).
`handleOverlayKey` was refactored to delegate `q`/`Esc` dismissal to a
new `dismissOverlay()` helper: a fatal no-results overlay exits 2; a
non-fatal error overlay suspended over help restores help at its saved
scroll position; any other non-fatal overlay dismisses to the base
state. `openHelp` clears any suspended-help state. Tests added in
`internal/app/overlay_precedence_test.go`: error-suspends-help with
scroll restoration for `q` and `Esc`; append-preserving-scroll via `r`
retry; error cancels pop-up with no return; `Esc` no-overlay dismisses
pop-up; the dismissal-outcome table for `q` and `Esc` (browse + error,
browse + help, browse + error-over-help, no-results + warning,
no-results + help, fatal no-results, record-loss no-results);
error-first key routing; the full three-key error-over-help sequence
for `q` and `Esc`; and `Esc` no-op in no-results. Created
[overlay-precedence](overlay-precedence.md); updated
[help-overlay](help-overlay.md) (cross-reference),
[outcome-contract](outcome-contract.md) (Issue #32 generalization
note and cross-reference),
[read-failures-and-retry](read-failures-and-retry.md) (generalized
append and error-suspends-help notes, out-of-scope update),
[index](index.md) (overlay-precedence entry),
[source-code](source-code.md) (internal/app Issue #32 entry), and
[unit-tests](unit-tests.md) (overlay_precedence_test.go entries).
Sources: `Notes/tasks/032-overlay-precedence-esc-semantics.md`,
`Notes/PRD-vrg.md` (Colours, overlays, and key precedence; Outcome and
exit-status contract), `internal/app/app.go`,
`internal/app/overlay_precedence_test.go`.

## [2026-09-12] ingest | Issue #33 terminal too-small screen with state recovery

Ingested the completed Issue #33 implementation: the 20×3 minimum
terminal size gate showing centred "Terminal too small", with only `q`
and `ctrl+c` active. `q` exits with the state-applicable outcome (130
if searching, else the fixed status) taking precedence over Issue
#32's dismissal semantics — `q` exits even if a modal overlay is
logically open, rather than dismissing it. `Esc` and every other key
are no-ops; a logically open overlay remains open after recovery.
`internal/app` added a `tooSmall` field set by the `WindowSizeMsg`
handler from the current dimensions, a `TooSmall()` accessor, a
`Theme()` accessor, a `SuspendedHelpScroll()` accessor, and a
`quitTooSmall()` helper. The `View` renders the centred message via
`centerText` when too-small, suppressing the overlay and pop-up. The
`KeyPressMsg` handler gates all keys after `ctrl+c`: `q` calls
`quitTooSmall()` and every other key is a no-op. The `WindowSizeMsg`
handler skips `buildViewport` while too-small, preserving all state
(cursor, viewport, anchors, list visibility, wrap, colour, horizontal
offset, modal state with scroll positions, pop-up timer); resizes
wholly within too-small defer recovery to the final dimensions. Tests
added in `internal/app/too_small_test.go`: threshold and display
(width below 20, height below 3, 20×3 boundary, recovery); exit
semantics (q during searching → 130, browsing → fixed status,
no-results → 1, fatal no-results overlay → 2, browse error overlay →
exits program, ctrl+c → 130); Esc and other-key no-ops with overlay
preservation; round-trip restoration of cursor, viewport/anchor,
horizontal offset, list visibility, wrap, colour, scrolled help
overlay, scrolled error overlay, and error-over-help stack; resize
wholly within too-small (19×2 → 10×1 → 25×8) with state preservation;
and pop-up timer continuation with expiry dismissal and no display.
Two existing tests updated for the 20×3 minimum:
`TestPopupResizeRecentresAndRetruncates` (width 15 → 20) and
`TestStaleNoteNoNegativeDimensions` (width 10 → 20). Created
[too-small-screen](too-small-screen.md); updated
[overlay-precedence](overlay-precedence.md) (cross-reference via
precedence rule), [index](index.md) (too-small-screen entry),
[source-code](source-code.md) (internal/app Issue #33 entry), and
[unit-tests](unit-tests.md) (too_small_test.go entries). Sources:
`Notes/tasks/033-terminal-too-small-with-state-recovery.md`,
`Notes/PRD-vrg.md` (Layout and indicators — minimum-size bullet),
`internal/app/app.go`, `internal/app/too_small_test.go`.

## [2026-09-12] ingest | Issue #34 documentation scale and memory limits

Ingested the completed Issue #34 implementation: the user-facing
`README.md` at the repository root (invocation, flags, key bindings,
exit-status table, help-only behaviour, ripgrep 15.x and `--no-config`,
scale/record-limit/memory statements); the filled help overlay footer
(`app.HelpFooter()` returns the shared `scaleLimitsText` constant —
the same text as the README's scale section — so neither sink drifts);
the `internal/docs/docs_test.go` synchronization tests (binding table,
allow-list, local help options, shared declarations, exit-status
agreement with `app.DecideOutcome`, help-only path, flags-only error,
ripgrep 15.x, `--no-config`, scale examples, record limit, memory
limits, footer carries the same tokens); and the
`internal/app/help_footer_test.go` Issue #6 sink-safety row for the
rendered help footer (no-style raw output, styled no-payload-after-ESC,
footer rendered in overlay). Created
[documentation-sync](documentation-sync.md); updated
[help-overlay](help-overlay.md) (footer slot filled by Issue #34),
[source-code](source-code.md) (internal/app Issue #34 footer entry,
internal/docs section), [unit-tests](unit-tests.md) (help footer
sink-safety row, internal/docs section), and [index](index.md)
(documentation-sync entry). Sources:
`Notes/tasks/034-documentation-scale-and-memory-limits.md`,
`Notes/PRD-vrg.md` (Resources and responsiveness, Out of Scope,
Outcome and exit-status contract), `README.md`,
`internal/app/app.go`, `internal/docs/docs_test.go`,
`internal/app/help_footer_test.go`.

## [2026-09-12] ingest | Issue #35 final integration verification

Ingested the completed Issue #35 closing verification pass. From a
clean checkout with Go build and test caches cleared, `go build ./...`,
`go vet ./...`, and `go test ./...` all passed (every package green,
including `cmd/vrg`, all six `internal/` packages, and `internal/docs`).
The PTY/subprocess boundary suites for Issues #4, #9, and #11 (all in
`cmd/vrg`) were re-run with caching disabled (`go test -count=1 -v
./cmd/vrg/ -timeout 120s`) so every critical boundary test executed:
Issue #4 search/subprocess tests, Issue #9 outcome matrix tests, Issue
#11 stderr-replay tests, Issue #4 cancellation/reap tests, and the
help-only process-boundary tests — all PASS. The final `vrg` binary was
built and smoke-run through the five representative outcomes with the
Issue #4 fake-rg harness under a real PTY: (1) successful browse with
`q` exiting 0; (2) no-results search with `q` exiting 1 and the
collected `warn` diagnostic replayed to stderr after terminal
restoration; (3) fatal fake-rg with no usable results exiting 2 on
dismissal with both `q` and `Esc`; (4) cancellation while searching
exiting 130 with the child terminated and reaped (reap evidence
present, child PID gone), terminal restoration verified (cursor-show
sequence, alt-screen exit, termios restored), for both `q` and
`ctrl+c`; (5) help-only invocation (bare `vrg`, `-h`, `--help`) printing
exactly one help copy to stdout with empty stderr and exit 0, run with
ripgrep unavailable on `PATH` and a sentinel fake rg available but
never invoked (marker file never appeared). No regressions were found;
no production code was changed; no focused test was patched, weakened,
or deleted. Created [final-verification](final-verification.md);
updated [index](index.md) (final-verification entry). Sources:
`Notes/tasks/035-final-integration-verification.md`,
`Notes/PRD-vrg.md` (Testing Decisions, Outcome and exit-status
contract), `cmd/vrg/main.go`, `cmd/vrg/search_test.go`,
`cmd/vrg/outcome_test.go`, `cmd/vrg/replay_test.go`,
`cmd/vrg/cancel_test.go`, `cmd/vrg/main_test.go`.

## [2026-09-14] ingest | Issue #49 dependency removal decision

Recorded the product owner's decision to remove unused Bubbles and Lip Gloss requirements. The active TUI stack now uses Bubble Tea directly with VRG's existing theme and rendering primitives; current PRD, project-overview, wiki-scope, and coding-skill instructions no longer prescribe the removed libraries. Updated the dependency manifests through `go mod tidy`; token imports are explicitly rejected. Sources: `Notes/issues/049-tidy-dependency-manifests.md`, `Notes/tasks/049-tidy-dependency-manifests.md`, `Notes/PRD-vrg.md`, `go.mod`, `go.sum`, `Notes/wiki/project-overview.md`, `Notes/wiki/AGENTS.md`, `Notes/skills/code-writing/styling-tui.md`, `Notes/skills/AGENTS.md`.

## [2026-09-15] ingest | Issue #36 stream-integrity fatal diagnostics

Ingested the completed Issue #36 implementation and tests. SearchIndex
`Integrity` now carries `Causes`: one structured `IntegrityCause`
(stable `IntegrityCauseKind` plus raw path) per offending physical
record, recorded at the violation site (`parseBegin`/`parseMatch`/
`parseEnd`/`parseSummary`/`Add` post-summary dispatch/`Build`) with
fixed overlap precedence — second `summary` contributes only
`extra summary`, other post-`summary` records contribute only
`record after summary` without lifecycle dispatch (Issue #36 removed
the `context` exemption and corrected the superseded lifecycle-matrix
row; Issue #44 builds on that boundary), and post-`summary`
unterminated fragments contribute `record after summary` rather than
`unterminated final record`. `Build` appends end-of-stream causes
deterministically (missing `end` sorted by unsigned raw-path bytes,
then `missing summary`, then the trailing-fragment cause).
`DecideOutcome` now composes a universal process → integrity →
record-loss → unknown-type diagnostic shared by overlay and stderr
replay, with no process-status line for 0/1 exits and `EscapePath`
escaping; `recordLossDiagnostics` was reordered so oversized
components precede unknown-type warnings (Issue #37's aggregate slot).
Created [stream-integrity-diagnostics](stream-integrity-diagnostics.md);
updated [outcome-contract](outcome-contract.md),
[record-robustness](record-robustness.md),
[search-collection-path](search-collection-path.md),
[source-code](source-code.md), [unit-tests](unit-tests.md), and
[index](index.md). Sources: `internal/searchindex/searchindex.go`,
`internal/searchindex/integrity_test.go`,
`internal/searchindex/lifecycle_test.go`, `internal/app/app.go`,
`internal/app/outcome_test.go`, `Notes/issues/036-stream-integrity-fatal-diagnostics.md`,
`Notes/tasks/036-stream-integrity-fatal-diagnostics.md`,
`Notes/PRD-vrg.md`.

## [2026-09-15] ingest | Issue #37 oversized-record aggregate and anonymous diagnostics

Ingested the completed Issue #37 implementation and tests.
`recordLossDiagnostics` in `internal/app/app.go` now leads the
oversized component with the pluralized aggregate built from
`Index.OversizedCount()` — exactly `1 oversized record skipped` for one
record and `N oversized records skipped` for every other count —
emitted whenever the count is positive, followed by one
`oversized record skipped for <sanitized path>` detail per distinct raw
path deduplicated in deterministic first-occurrence order; the
per-record aggregate count itself is never deduplicated. An anonymous
oversized record (path not recovered) now always surfaces: with zero
usable results the record-loss fatal overlay contains exactly the
aggregate line and exits 2 — never an empty overlay — and with usable
results the loss produces a warning overlay and the aggregate reaches
the post-restoration stderr replay instead of passing silently. The
component keeps Issue #36's universal order: after the malformed
aggregate and before unknown-type warnings. The Issue #36
post-`summary` oversized fixtures in `TestDecideOutcomeComposedOrder`
and `TestOutcomeIntegrityDiagnosticsFlow` were updated to
`[record after summary, 1 oversized record skipped, oversized record
skipped for <escaped Q>]`, and `TestOutcomeOversizedAggregateDiagnostics`
was added covering singular/plural aggregates, per-path deduplication,
both anonymous cases, mixed recoverability, and component ordering —
all through real `Builder.ReadFrom` oversized streams. Updated
[record-robustness](record-robustness.md),
[outcome-contract](outcome-contract.md),
[stream-integrity-diagnostics](stream-integrity-diagnostics.md),
[search-collection-path](search-collection-path.md),
[source-code](source-code.md), [unit-tests](unit-tests.md), and
[index](index.md). Sources: `internal/app/app.go`,
`internal/app/outcome_test.go`,
`internal/searchindex/searchindex.go`,
`Notes/issues/037-oversized-record-aggregate-anonymous-diagnostics.md`,
`Notes/tasks/037-oversized-record-aggregate-anonymous-diagnostics.md`,
`Notes/PRD-vrg.md` (Result index, records, and stream integrity —
oversized bullets; Resources and responsiveness — 64 MiB limit).

## [2026-09-15] ingest | Issue #38 viewport installed with the layout key's text width

Ingested the completed Issue #38 implementation and tests.
`internal/app/app.go` no longer recomputes
`viewport.TextWidth(m.width, …)` from the raw terminal width at
viewport install sites: the `LayoutReadyMsg` handler installs
`msg.Key.TextWidth`, the synchronous row-provider factory seam in
`buildViewport` installs `m.LayoutKey().TextWidth`, the
`WindowSizeMsg` resize path (factory-seam viewport retained) installs
`m.LayoutKey().TextWidth`, and the cache-hit fast path already used
`key.TextWidth`. The single `LayoutKey` computation — terminal width
minus `ListWidth()` minus the one-cell separator = panel width; panel
width minus gutter minus `ReservedWidth(mode)` = text width — now
governs row construction and viewport behaviour alike, so reveal,
pan, clip, padding, and the hidden-content indicators are measured
against the content panel with the file list present, and no composed
row exceeds the terminal width.
`internal/app/reveal_horizontal_test.go` replaced the terminal-width
`textWidthAt80` helper with `textWidthFor` (terminal minus actual
`ListWidth()` minus separator minus gutter minus `ReservedWidth`) and
added list-hidden, wrap-mode zero-reservation, resize, factory-seam,
and composed-view boundary coverage; `pan_test.go`,
`indicator_test.go`, `grapheme_indicator_test.go`, and `wrap_test.go`
were recalibrated to the same chain (text width 65 at 80×24 with the
`src/a.go` fixture list). Created
[viewport-text-width](viewport-text-width.md); updated
[file-list-layout](file-list-layout.md),
[wrap-mode-and-grapheme-policy](wrap-mode-and-grapheme-policy.md),
[logical-anchor-and-layout-preparation](logical-anchor-and-layout-preparation.md),
[horizontal-panning](horizontal-panning.md),
[horizontal-reveal](horizontal-reveal.md),
[hidden-content-indicators](hidden-content-indicators.md),
[source-code](source-code.md), [unit-tests](unit-tests.md), and
[index](index.md). Sources: `internal/app/app.go`,
`internal/app/reveal_horizontal_test.go`,
`internal/app/pan_test.go`, `internal/app/indicator_test.go`,
`internal/app/grapheme_indicator_test.go`,
`internal/app/wrap_test.go`,
`Notes/issues/038-viewport-content-panel-width.md`,
`Notes/tasks/038-viewport-content-panel-width.md`,
`Notes/PRD-vrg.md` (File list and layout; Layout and indicators;
Navigation, viewport, and logical anchors).

## [2026-09-15] ingest | Issue #39 render from the shared grapheme/cell model

Ingested the completed Issue #39 implementation and tests.
`internal/safepresentation/cellwidth.go` is the new shared ANSI-aware
grapheme/cell helper: `GraphemeClustersANSI` segments styled text into
clusters with ANSI CSI sequences carried as zero-width clusters,
`CellWidth` sums terminal cells ignoring ANSI bytes, and
`TruncateLeftCells` keeps trailing cells without splitting wide,
combining, or ZWJ clusters — the sole allow-listed production file for
`utf8.DecodeRuneInString`. `internal/app/app.go` routed every
final-render display-geometry consumer through the helper:
`renderLineWithHighlights` renders each clipped row directly from
`line.Clusters` cell spans (a cluster whose cells intersect a
highlight is styled whole, so two-cell CJK no longer swallows the
following character, combining sequences stay with their base, and
ZWJ sequences are never split); `visibleWidth` delegates to
`CellWidth`; the pop-up left-truncation and centreing use
`TruncateLeftCells`/`CellWidth`; `wrapLine` wraps overlay/help text on
cluster boundaries; `computeLongestPathWidth`, file-list entry
padding, the filename-row note slot, and the indicator geometry
measure through `CellWidth`. `internal/theme/theme.go`'s private
`cellWidth` delegates to `safepresentation.CellWidth` so overlay
borders align under wide and combining content. New tests:
`internal/safepresentation/cellwidth_test.go` (helper units),
`internal/theme/theme_test.go` (overlay alignment under
wide/combining/mixed rows), `internal/app/cell_render_test.go`
(composed `View()` CJK/combining/ZWJ, clip, list padding, filename
row, pop-up, overlay assertions), and
`internal/app/decode_guard_test.go` (the recursive non-test source
scan failing on `utf8.DecodeRuneInString` outside
`internal/safepresentation/cellwidth.go`). Created
[shared-cell-model-render](shared-cell-model-render.md); updated
[safe-presentation](safe-presentation.md),
[wrap-mode-and-grapheme-policy](wrap-mode-and-grapheme-policy.md),
[theme-module](theme-module.md),
[file-list-layout](file-list-layout.md),
[source-code](source-code.md), [unit-tests](unit-tests.md), and
[index](index.md). Sources: `internal/safepresentation/cellwidth.go`,
`internal/safepresentation/cellwidth_test.go`,
`internal/app/app.go`, `internal/app/cell_render_test.go`,
`internal/app/decode_guard_test.go`, `internal/theme/theme.go`,
`internal/theme/theme_test.go`,
`Notes/issues/039-render-from-shared-grapheme-cell-model.md`,
`Notes/tasks/039-render-from-shared-grapheme-cell-model.md`,
`Notes/PRD-vrg.md` (Text, graphemes, and safe presentation; Layout
and indicators).

## [2026-09-15] ingest | Issue #40 bounded browse rendering from prepared file groups

Ingested the completed Issue #40 implementation and tests.
`internal/app/app.go` moved all whole-index work off the frame path:
`prepareFileGroups(idx)` runs once in the `SearchCompleteMsg` handler
and produces `m.fileGroups` (immutable per-file stop groupings, each
carrying the `safepresentation.EscapePath` text, its
`GraphemeClusters` table, and its full cell width),
`m.fileIndexByPath` (raw path → group index), and `longestPathWidth`
(absorbing `computeLongestPathWidth`). `renderBrowse` now reads
`m.fileGroups`, iterates only the visible `[listOffset,
listOffset+visibleRows)` window, truncates each visible entry via the
new `truncateFileEntry` → `safepresentation.TruncateLeftCellsFrom`
(the cluster-table variant added to
`internal/safepresentation/cellwidth.go`), finds the current file via
the `currentFileIndex` map lookup, and tests emptiness with
`len(m.fileGroups)` instead of the per-frame `index.Files()`
distinct-path scan; `loadFileFor` takes its per-file stop range from
the group map instead of copying and scanning `index.Stops()`.
Per-keystroke `Update()`+`View()` allocation fell from ~42 MB to
under 512 KiB over a 45,000-stop index. `internal/app/layout_test.go`
gained the combined Update+View cost guard
(`measureUpdateViewAllocs`, spanning both halves without a counter
reset, so whole-index work fails even when moved into navigation
handling) with `TestRenderCostGuardNavigateViewBounded`,
`TestRenderCostGuardNavigatePrevBounded`,
`TestRenderCostGuardResizeRetruncates`, and
`TestRenderCostGuardGutterGrowthRetruncates`. Created
[bounded-browse-render](bounded-browse-render.md); updated
[shared-cell-model-render](shared-cell-model-render.md),
[file-list-layout](file-list-layout.md),
[source-code](source-code.md), [unit-tests](unit-tests.md), and
[index](index.md). Sources: `internal/app/app.go`,
`internal/app/layout_test.go`,
`internal/safepresentation/cellwidth.go`,
`Notes/issues/040-browse-render-no-whole-index-scan.md`,
`Notes/tasks/040-browse-render-no-whole-index-scan.md`,
`Notes/PRD-vrg.md` (Resources and responsiveness; File list and
layout).
## [2026-09-15] ingest | Issue #41 overlay full scroll — no head/tail compression

Removed the head-plus-ellipsis-plus-tail compression `renderOverlay`
applied to non-help diagnostics longer than the visible height: the
scrollable row set is now the complete wrapped diagnostic, so every
row of a long captured stderr stream is reachable by scrolling
(`internal/app/app.go`). `overlayScroll` clamps to
`[0, max(0, rows−maxVisible)]` in `handleOverlayKey` (stale positions
snap into range on the next scroll key) and in the render path; new
shared helpers `overlayInteriorWidth`, `overlayMaxVisible`,
`overlayRows`, and `overlayMaxScroll` compute the geometry, and the
new `overlayWrap`/`overlayWrapCache` field memoizes the wrapped rows
keyed on (text, interior) so the per-keypress clamp does not re-wrap
a ~1 MiB diagnostic. The modal key contract is unchanged (up/down
scroll, q/Esc dismiss, `u`/`d`/page up/page down ignored) and
render-time clipping at tiny sizes remains; model-level elision is
forbidden. The ≥1 MiB `TestStderrContentFixture` keeps its drainage,
complete-stdout, captured-stderr, and completion checks but no longer
requires simultaneous head/tail rendering — superseding the Issue #9
contract; it scrolls a few rows and dismisses with `q` twice because
a bare `Esc`+`q` can coalesce into `Alt+q` while an expensive update
is in flight. New `internal/app/overlay_full_scroll_test.go` proves
the complete-row set, both-end clamps, bounded traversal, the ≥1 MiB
content shape, and append-extends-set-with-preserved-position at the
model level; `TestOverlayKeyOtherIgnored` now covers `u`, `d`, page
up, and page down; `gatedFailingLoader.load` reads its outcome after
the per-path gate (fixing a pre-existing race the slower key path
exposed). Created
[overlay-full-scroll](overlay-full-scroll.md); updated
[outcome-contract](outcome-contract.md),
[help-overlay](help-overlay.md),
[source-code](source-code.md), [unit-tests](unit-tests.md), and
[index](index.md). Sources: `internal/app/app.go`,
`internal/app/overlay_full_scroll_test.go`,
`internal/app/overlay_test.go`,
`internal/app/overlay_precedence_test.go`,
`internal/app/read_failure_test.go`, `cmd/vrg/outcome_test.go`,
`Notes/issues/041-overlay-full-scroll-no-head-tail-compression.md`,
`Notes/tasks/041-overlay-full-scroll-no-head-tail-compression.md`,
`Notes/PRD-vrg.md` (Colours, overlays, and key precedence; Outcome
and exit-status contract).
## [2026-09-15] ingest | Issue #42 atomic reload admission

Ingested the completed Issue #42 fix. `handleReload`
(`internal/app/app.go`) previously recorded `reloadingPaths`,
switched to `Loading…` presentation, and set `IntentReloadAnchor`
*before* `startLoad` checked whether a load was already in flight for
the path — so a dropped `r` still committed reload state, and the
in-flight startup or navigation load's completion was misclassified
as a reload (extra revision bump, pending destination reveal
supplanted by anchor preservation, suppressing the required
first-match reveal). `handleReload` now checks `loadingPaths` first:
a dropped `r` returns the model unchanged (single decision point, no
intermediate committed state), while an accepted `r` applies the
reload flags, `Loading…` presentation, `IntentReloadAnchor`, and
exactly one revision increment unchanged. Navigation re-entry is
deliberately ungated: selection, placeholder presentation, and
`IntentReveal` still update even when `startLoad` drops a duplicate
load. New `internal/app/reload_admission_test.go` proves the
dropped-`r` intent/revision/presentation preservation during startup
and navigation loads (completion reveals the latest target instead of
anchor-preserving), the accepted-`r` single revision increment, the
rapid-press single-in-flight rule, and ungated re-entry. Created
[reload-admission](reload-admission.md); updated
[explicit-reload](explicit-reload.md),
[async-load-isolation](async-load-isolation.md),
[load-completion-two-stage](load-completion-two-stage.md),
[source-code](source-code.md), [unit-tests](unit-tests.md), and
[index](index.md). Sources: `internal/app/app.go`,
`internal/app/reload_admission_test.go`,
`Notes/issues/042-dropped-reload-no-intent-mutation.md`,
`Notes/tasks/042-dropped-reload-no-intent-mutation.md`,
`Notes/PRD-vrg.md` (File loading, cache, reload, and selection
consistency; Navigation, viewport, and logical anchors).
## [2026-09-15] ingest | Issue #43 standalone combining cluster fallback cell

Ingested the completed Issue #43 implementation. The recorded decision
(HITL gate, `Notes/decisions/043-combining-cluster-fallback-cell.md`)
selects Candidate A: a standalone zero-width cluster displays `U+25CC ◌`
followed by the cluster's original combining-mark bytes, expected to
segment as one width-1 cluster under `rivo/uniseg`. Previously
`expandedByteCells`/`expandedHighlights` only annotated a synthetic
`[start, start+1)` range while `cellPos` still advanced by zero, so a
following cluster overlapped the fallback and the Issue #39 renderer
had no real cell to paint (the mark rendered unstyled, the next
character took the highlight). `filebuffer.Load` now runs
`standaloneClusterFallback` between `GraphemeClusters` and the Issue
#21 expansion: it inserts ◌ before every zero-width cluster's bytes,
records the `◌`+marks unit as one width-1 cluster (one-cell
normalization by construction, immune to an unexpected width-library
report), and shifts `ByteOffsets` past the inserted bytes so
byte-to-cell mapping still resolves the fallback cell to the original
source bytes. The fallback propagates through `expandedByteCells`,
`expandedHighlights`, `ContentWidth`, wrapping, clipping, panning,
indicators, and `renderLineWithHighlights` like any other cell;
`cellPos` in both expansion functions now advances by the effective
width as a defensive residue. New
`internal/filebuffer/cluster_fallback_test.go` asserts the recorded
display bytes, ByteCells, content width, exact highlight spans,
multiple-fallback non-overlap, and one-cell normalization; new
`internal/app/fallback_cell_test.go` asserts the composed view paints
exactly `◌́` styled for one cell with `x` unstyled after it, and that
wrap and pan/clip count the fallback cell. Created
[cluster-fallback-cell](cluster-fallback-cell.md); updated
[grapheme-cluster-highlight-expansion](grapheme-cluster-highlight-expansion.md),
[shared-cell-model-render](shared-cell-model-render.md),
[source-code](source-code.md), [unit-tests](unit-tests.md), and
[index](index.md). Sources: `internal/filebuffer/filebuffer.go`,
`internal/filebuffer/cluster_fallback_test.go`,
`internal/app/fallback_cell_test.go`,
`Notes/issues/043-combining-cluster-fallback-cell.md`,
`Notes/tasks/043-combining-cluster-fallback-cell.md`,
`Notes/decisions/043-combining-cluster-fallback-cell.md`,
`Notes/PRD-vrg.md` (Text, graphemes, and safe presentation).
## [2026-09-15] ingest | Issue #44 post-`summary` `context` integrity failure

Ingested the completed Issue #44 coverage and documentation. Issue #36
had already removed the `context` exemption in `Builder.Add` and
corrected the superseded `"context after summary has no lifecycle
effect"` lifecycle-matrix row as part of its post-summary-precedence
GREEN; Issue #44 began from that green boundary and added dedicated
regression coverage only — no second parser change.
`internal/searchindex/lifecycle_test.go` gained a neighbouring
`context while open` row; `TestIntegrityCauseMatrix`
(`internal/searchindex/integrity_test.go`) gained neighbouring
pre-`summary` `context` rows (before `begin`, while open, after `end`
before `summary`) asserting an empty cause list beside the dedicated
`summary` → `context` row asserting exactly the single
`record after summary` cause; `internal/app/outcome_test.go` gained
`TestContextAfterSummaryOutcome`, asserting the `summary` → `context`
stream takes the outcome matrix's fatal path (exit 2) in both the
zero-results fatal-overlay and retained-results browse-overlay
dispositions, with the complete composed `record after summary`
diagnostic identical in the overlay and the collected stderr replay.
The wiki now records the amended contract: Issue #9's former
"`context` in any position" row covers only pre-`summary` positions,
and *any* record after `summary` — including `context` — is a
stream-integrity failure per the summary-is-final contract. Updated
[stream-integrity-diagnostics](stream-integrity-diagnostics.md),
[outcome-contract](outcome-contract.md),
[search-collection-path](search-collection-path.md),
[unit-tests](unit-tests.md), and [index](index.md). Sources:
`internal/searchindex/lifecycle_test.go`,
`internal/searchindex/integrity_test.go`,
`internal/app/outcome_test.go`,
`Notes/issues/044-post-summary-context-integrity-failure.md`,
`Notes/tasks/044-post-summary-context-integrity-failure.md`,
`Notes/PRD-vrg.md` (Result index, records, and stream integrity;
Outcome and exit-status contract).

## [2026-09-15] ingest | Issue #45 test-hook build topology

Ingested the completed Issue #45 implementation, which moves every
test-only seam out of the released `vrg` binary behind a
`vrg_testhooks` build variant. `cmd/vrg` now carries two
build-constrained boundaries joined by unconditional common call sites
in `runSearch`: the option/process wiring `testSeamOptions(proc)`
(`seams.go` returns nil untagged; `seams_testhooks.go` reads the
option-related `VRG_TEST_*` manifest and returns the `app.Option`s and
`proc.OnReap` wiring, with file watchers paced by a sleep-polled
`watchForFile` instead of the former tight `os.Stat` spin) and the
program-runner wrapper `runProgram(model, stdout)`
(`runner.go` delegates to `tea.NewProgram(...).Run()` untagged;
`runner_testhooks.go` runs the real program and substitutes the
returned final-model/error tuple selected by `VRG_TEST_RUN_FINAL_MODEL`
and `VRG_TEST_RUN_ERROR` for Issue #46's return-shape matrix).
`TestMain` builds the binary under test with
`go build -tags vrg_testhooks`, so the whole subprocess suite
exercises the hooked variant automatically; `testhooks_test.go` adds
the two-sided boundary proof — the untagged artifact ignores the
entire explicit hook manifest and contains none of its names, while
the tagged runner delivers every required tuple at the real
`program.Run()` return site. The explicit manifest excludes fake-rg
fixture variables (`VRG_TEST_ARGV`/`CWD`/`HANDSHAKE`/`READY`/`PID`),
which Issue #50 renames `FAKE_RG_*`; Issues #46 and #48 extend these
same tagged boundaries rather than adding production hooks. Added
[test-hook-topology](test-hook-topology.md); updated
[source-code](source-code.md), [unit-tests](unit-tests.md), and
[index](index.md). Sources: `cmd/vrg/main.go`, `cmd/vrg/seams.go`,
`cmd/vrg/seams_testhooks.go`, `cmd/vrg/runner.go`,
`cmd/vrg/runner_testhooks.go`, `cmd/vrg/main_test.go`,
`cmd/vrg/testhooks_test.go`,
`Notes/issues/045-remove-test-hooks-from-production-binary.md`,
`Notes/tasks/045-remove-test-hooks-from-production-binary.md`,
`Notes/PRD-vrg.md` (Outcome and exit-status contract; Testing
Decisions).

## [2026-09-15] ingest | Issue #46 unified runtime-error shutdown/replay

Ingested the completed Issue #46 implementation, which routes every
`program.Run()` return shape through the single ordered
shutdown/diagnostic-replay sequence instead of writing a `Run()` error
directly and returning before the replay — or exiting silently on a
failed final-model type assertion. `runSearch` now owns an
`app.Diagnostics` snapshot (a mutex-guarded string list with
`add`/`Lines()`) wired into the model through the new
`app.WithDiagnostics` option — production wiring in the same option
family as the `WithOnCollect` test seam — and `collectDiagnostic`
appends each collected diagnostic to it alongside the session
collection, so session diagnostics reach stderr even when the final
model `Run()` returns is absent or has the wrong type. After `Run()`
returns (terminal already restored by Bubble Tea) and centralized
cleanup terminates and reaps the child, the unified sequence replays:
the snapshot's session diagnostics in collection order, then
`vrg: program returned no usable final model` when the final model is
absent or wrong-typed (never a silent exit), then the `Run()` runtime
error appended exactly once with no direct-write/replay duplicate.
Every failing shape exits 2, matching the startup-failure convention.
`cmd/vrg/runshape_test.go` adds `TestRunReturnShapeUnifiedShutdown`,
driving the five-shape matrix (valid/nil/invalid final model ×
injected/nil `Run()` error) through the Issue #45 tagged
`runProgram` seam in a real PTY lifecycle with two diagnostics
collected in deterministic order before a clean browse quit; each
subtest asserts exit 2, termios and display restoration, child
termination/reap, and the ordered exactly-once replay. The issue added
no new hook-manifest names or key-sending helpers, so Issue #48's
reciprocal adaptation rule applies. Updated
[outcome-contract](outcome-contract.md),
[search-collection-path](search-collection-path.md),
[test-hook-topology](test-hook-topology.md),
[source-code](source-code.md), [unit-tests](unit-tests.md), and
[index](index.md). Sources: `cmd/vrg/main.go`, `internal/app/app.go`,
`cmd/vrg/runshape_test.go`, `cmd/vrg/runner_testhooks.go`,
`Notes/issues/046-runtime-error-common-diagnostic-replay.md`,
`Notes/tasks/046-runtime-error-common-diagnostic-replay.md`,
`Notes/PRD-vrg.md` (Outcome and exit-status contract).

## [2026-09-15] ingest | Issue #47 single-line read-failure diagnostics

Ingested the completed Issue #47 fix, which composes every file-load
failure diagnostic as the `safepresentation.EscapePath`-escaped path
plus a sanitized reason that never repeats the raw path, instead of
passing `err.Error()` to `EscapeDiagnostic`. `os.ReadFile` returns a
`*fs.PathError` whose message embeds the raw filename, and
`EscapeDiagnostic` preserves LF as a diagnostic boundary, so a
filename containing a newline split one failed read into multiple
diagnostic rows — the escaping sanitized control bytes but did not
preserve the single-line diagnostic structure. The new
`readFailureDiagnostic(path, err)` helper unwraps a `*fs.PathError`
to its `Op` and `Err` (`open <escaped>: no such file or directory`)
and falls back to `<escaped>: <err>` for non-PathError loader
errors. The same construction feeds `failedPaths` (re-entry retry
overlay), the session diagnostic collection (stderr replay), and
the current-file overlay, so initial load, `r` reload, and
failed-path re-entry retry all produce exactly one diagnostic line
whatever bytes the filename contains. `read_failure_test.go` gained
the Issue #47 suite: a table-driven test over hostile filename kinds
(newline, tab, invalid UTF-8, ESC) creates real fixture files in a
disposable `os.MkdirTemp` directory, holds the load at the file
gate, removes the fixture, and releases the gate so the production
`filebuffer.Load` fails with a genuine ENOENT `*fs.PathError`;
assertions pin the single escaped-path-plus-reason line through the
overlay text, the rendered overlay row set, and the replay
collection. Reload and re-entry tests drive the same construction
through `r` and the failed-path retry, where the appended second
diagnostic is another identical single line.
`TestOverlayAppendExtendsScrollableSet` now expects the appended
row's escaped-path prefix (`src/a.go: APPENDED-MARKER`). Updated
[read-failures-and-retry](read-failures-and-retry.md),
[safe-presentation](safe-presentation.md),
[source-code](source-code.md), [unit-tests](unit-tests.md), and
[index](index.md). Sources: `internal/app/app.go`,
`internal/app/read_failure_test.go`,
`internal/app/overlay_full_scroll_test.go`,
`Notes/issues/047-read-failure-single-line-filenames.md`,
`Notes/tasks/047-read-failure-single-line-filenames.md`,
`Notes/PRD-vrg.md` (Text, graphemes, and safe presentation; File
loading, cache, reload, and selection consistency).

## [2026-09-15] ingest | Issue #48 deterministic PTY handshakes

Ingested the completed Issue #48 implementation, which replaces every
fixed settling and inter-key `time.Sleep` in the `cmd/vrg`
PTY/subprocess helpers with deterministic, application-side
acknowledgements riding the Issue #45 `vrg_testhooks` seam — never the
production binary. `internal/app/app.go` gains the observational
`UpdateAck` record (message kind, key label, post-update state,
open-overlay kind, `OverlayDismissed` flag) and the `WithUpdateAck`
option in the same test-seam family as `WithOnCollect`; `Model.Update`
is now a wrapper that runs the real `update` transition and then fires
the callback, so production behaviour and timing are unchanged.
`seams_testhooks.go` reads the new `VRG_TEST_UPDATE_ACK` manifest name
and appends one `<seq> msg=… key=… state=… overlay=… dismissed=…`
record per Update-processed message through the mutex-serialized
`updateAckLog` sink; the untagged build returns no options and the
name is absent from the production artifact, proven by the extended
`TestUntaggedBinaryIgnoresHookManifest` probe (provocative path, no
`update-ack` side-effect file, no name bytes in the binary).
`cmd/vrg/handshake_test.go` pins the finite
helper/action/postcondition/acknowledgement matrix and owns the shared
`runVrgPTY`/`ptyDriver` scaffold: `waitMsg`/`waitAck` block on the
`ackLog` per-occurrence cursor (an earlier same-kind record can never
satisfy a later wait), `sendKey` blocks on that key's own event before
the next send — which also prevents input-reader coalescing of
consecutive sends — and `waitForOutput` polls rendered PTY output for
tests asserting on transient painted content (the Update
acknowledgement proves the model transition while the renderer can
still coalesce the frame, the ordering bug the removed sleeps hid).
`runVrgWithKeys`, `runVrgKillChild`, `runVrgWithQuit`, `runVrgReplay`,
and `runVrgCancel` are thin wrappers over the scaffold; every
`runVrgReplay` and `runshape` trigger callback, the Issue #41 overlay
tests, and the revised `TestStderrContentFixture` now wait on their
matrix rows, overlay dismissal is acknowledged before a following `q`,
and `runVrgWithQuit` sends a second `q` only when the first `q`'s
event reports a dismissal. `TestNormalExitReapsChild` moved off the
fixture-only handshake, which raced the model transition. Every wait
is bounded (`ackTimeout`, abort on process exit) and fails naming the
awaited condition; the only remaining `time.Sleep` calls pace bounded
condition polls (`waitForFile`, `waitForAckLines`, `waitEvent`,
`waitForOutput`), enforced by `TestNoFixedSleepsInPtyHelpers`' static
AST check. Contract tests prove the seam contract, per-occurrence
correlation with repeated same-kind events, overlay-dismissal-before-
quit ordering, and bounded-timeout failure. Verified with
`go test ./cmd/vrg -count=10`, `CGO_ENABLED=1 go test -race
-count=1 ./cmd/vrg`, and the repository-wide build/vet/test/race
suite. Created [pty-handshake-tests](pty-handshake-tests.md); updated
[test-hook-topology](test-hook-topology.md),
[source-code](source-code.md), [unit-tests](unit-tests.md), and
[index](index.md). Sources: `internal/app/app.go`,
`cmd/vrg/seams_testhooks.go`, `cmd/vrg/handshake_test.go`,
`cmd/vrg/outcome_test.go`, `cmd/vrg/search_test.go`,
`cmd/vrg/replay_test.go`, `cmd/vrg/cancel_test.go`,
`cmd/vrg/runshape_test.go`, `cmd/vrg/testhooks_test.go`,
`cmd/vrg/main_test.go`,
`Notes/issues/048-pty-tests-deterministic-handshakes.md`,
`Notes/tasks/048-pty-tests-deterministic-handshakes.md`,
`Notes/PRD-vrg.md` (Testing Decisions — subprocess boundary).
## [2026-09-15] ingest | Issue #49 dependency-manifest tidy — Bubbles/Lip Gloss removal

Recorded the product owner's 2026-09-14 removal decision:
`charm.land/bubbles/v2` and `charm.land/lipgloss/v2` are absent from
`go.mod` and `go.sum` because the implementation imports neither
library, and no token import may be added to retain a manifest entry.
The remaining TUI dependency is `charm.land/bubbletea/v2`; VRG's own
theme and rendering primitives continue to own presentation.
`go mod tidy -diff` reports no drift on the committed manifests and
`go mod verify`, `go build ./...`, `go vet ./...`, and `go test ./...`
are green. Current stated-stack and prescriptive references agree:
the `Notes/PRD-vrg.md` *Further Notes* stack line,
[project-overview](project-overview.md), this schema's Scope
paragraph, `Notes/skills/code-writing/styling-tui.md`, and the
`code-writing/styling-tui` entry in `Notes/skills/AGENTS.md` all
describe Bubble Tea without Bubbles or Lip Gloss; historical
audit/issue/critique references are preserved as decision context.
Sources: `go.mod`, `go.sum`,
`Notes/issues/049-tidy-dependency-manifests.md`,
`Notes/tasks/049-tidy-dependency-manifests.md`, `Notes/PRD-vrg.md`.

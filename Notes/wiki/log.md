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

## [2026-09-16] ingest | Issue #2 CLI flag allow-list and child argv

Ingested the completed Issue #2 implementation: `optionDecls` extended to
all eleven allow-listed no-argument search flags (single source for
mow.cli config, scan recognition, ordered flag records, and generated
help); `scanArgs`/`scanOption` record accepted spellings in encounter
order with combined-short expansion, lexically reject every `=`
assignment form (closing Issue #1's `--help=true` parsed-value seam), and
count cumulative `-u`/`--unrestricted` with a two-occurrence cap;
`Result.ChildArgs` carries the exact protected argv
`--json --no-config <flags> -- <pattern> <root>`; the `cmd/vrg` stub
prints `search stub: argv=rg …`. Created
[cli-flags-child-argv](cli-flags-child-argv.md); updated
[cli-foundation](cli-foundation.md), [source-code](source-code.md),
[unit-tests](unit-tests.md), and the index. Sources:
`Notes/issues/002-cli-flag-allow-list-and-child-argv.md`,
`Notes/tasks/002-cli-flag-allow-list-and-child-argv.md`,
`Notes/PRD-vrg.md` (Invocation and child arguments, Module Design → CLI),
`internal/cli/cli.go`, `internal/cli/cli_test.go`,
`internal/cli/internal_test.go`, `cmd/vrg/main.go`,
`cmd/vrg/main_test.go`.

## [2026-09-16] ingest | Issue #3 spawn rg, collect results, searching screen

Ingested the completed Issue #3 implementation: `cmd/vrg` now hands the
protected `ChildArgs` vector and the invocation working directory to
`app.Run`; `internal/app` spawns `rg` from `PATH` with `cmd.Dir` set to
that directory, drains both stdout and stderr concurrently for the whole
child lifetime, and runs collection plus index preparation as a single
command off the Bubble Tea update path; the model holds "Searching…"
until the index is ready (the `WithGate`/`WithCollectAck` seams, wired in
`main` from `VRG_TEST_GATE`/`VRG_TEST_COLLECT_ACK`, let tests hold
preparation after rg exits), then shows the interim
`N files, M matched lines` summary where `q` exits 0; start failure is
detected before the TUI with a sanitized diagnostic and exit 2.
`internal/searchindex` gained the per-record parser (`text`/base64
`bytes` decoding, the five known events plus `KindUnknown`/
`KindMalformed`, the required-field/range schema matrix) and the index
itself (same-path/same-line `Stop` merging, `(start, end)` submatch
order, union `Highlights`, unsigned raw-path ordering, working-directory
resolution without canonicalization). The `cmd/vrg` stub line is gone.
Created [search-collection](search-collection.md); updated
[cli-flags-child-argv](cli-flags-child-argv.md),
[project-overview](project-overview.md), [source-code](source-code.md),
[unit-tests](unit-tests.md), and the index. Sources:
`Notes/issues/003-spawn-rg-collect-results-searching-screen.md`,
`Notes/tasks/003-spawn-rg-collect-results-searching-screen.md`,
`Notes/PRD-vrg.md` (Invocation and child arguments; Result index,
records, and stream integrity; Module Design → SearchIndex / App),
`cmd/vrg/main.go`, `cmd/vrg/main_test.go`, `cmd/vrg/search_test.go`,
`internal/app/app.go`, `internal/app/rg.go`, `internal/app/app_test.go`,
`internal/app/rg_test.go`, `internal/searchindex/record.go`,
`internal/searchindex/index.go`, `internal/searchindex/index_test.go`.

## [2026-09-16] ingest | Issue #4 cancellation, child cleanup, terminal restore

Ingested the completed Issue #4 implementation: `ctrl+c` in any state
and `q` while searching (including gate-held index preparation after rg
has exited) begin a controlled exit at status 130; `Esc` while searching
is a no-op; every controlled exit routes through `model.quitCmd` →
`reapChild` (`Child.Terminate` + idempotent `Wait`) so the child is
terminated and reaped before quit, with `app.Run` repeating the same
cleanup after `program.Run` as a safety net for exits that bypass the
model (`InterruptMsg` → 130, program errors → 2). The `quitting` flag
discards late `searchDoneMsg`s so completions cannot revive the UI.
Views set `AltScreen`, giving the leave-alt-screen/cursor-visible
restoration sequence plus termios restoration on every controlled exit.
The injectable `WithFailFunc` hook produces `failMsg`; `Run` reports it
through `writeFailureDiag`, the single post-restoration stderr writer,
exactly once, exit 2. New seams: `WithReapReport`/`VRG_TEST_REAP`
(reaped wait-status side channel) and `WithFailFunc`/`VRG_TEST_FAIL`
(trigger-file failure injection); `cmd/vrg/cancel_test.go` adds the
termios-capturing PTY harness (`startVrgTermPTY`, `blockedRG` fake rg).
Created [cancellation-cleanup](cancellation-cleanup.md); updated
[search-collection](search-collection.md), [source-code](source-code.md),
[unit-tests](unit-tests.md), and the index. Sources:
`Notes/issues/004-cancellation-child-cleanup-terminal-restore.md`,
`Notes/tasks/004-cancellation-child-cleanup-terminal-restore.md`,
`Notes/PRD-vrg.md` (Outcome and exit-status contract; Colours, overlays,
and key precedence; Module Design → App; Testing Decisions),
`cmd/vrg/main.go`, `cmd/vrg/cancel_test.go`, `cmd/vrg/search_test.go`,
`internal/app/app.go`, `internal/app/rg.go`, `internal/app/app_test.go`,
`internal/app/cancel_test.go`.

## [2026-09-16] ingest | Issue #5 browse tracer: file list, file panel, safe presentation

Ingested the completed Issue #5 implementation: the interim summary is
replaced by `stateBrowse`, a two-pane view — full-width filename rule on
row 0, fixed-width file list (longest escaped path + 1, Issue #24 owns
the real formula) with the current entry underlined, and a file panel
with a right-justified gutter plus inverse-video match runs. The current
file loads asynchronously through a `tea.Cmd` that runs the whole
`filebuffer.Load` (read + decode/map) so `fileLoadedMsg` carries a
prepared buffer; "Loading…" and "(unreadable)" are the panel
placeholders, in-flight loads are deduplicated, and late completions for
a non-current path update only their cache slot. `internal/filebuffer`
splits LF/CRLF, maps each line through the new
`internal/safepresentation` escaping core (path `\n`/`\r`/`\t`/caret/
`\uXXXX`/`\xNN` rules; content caret forms, standalone-CR `^M`,
provisional single-cell `→` tab, U+FFFD invalid bytes) and converts stop
byte ranges to display-cell ranges via `CellsCovering`. `internal/viewport`
and `internal/theme` land minimal seams (top-of-file window; `Dark()` +
the `Plain()` no-style composition path). New seam:
`WithLoadGate`/`VRG_TEST_LOAD_GATE`. `TestHostileFixtureRawOutput` drives
the hostile fixture through the real composition path and asserts on raw
output before ANSI stripping. Created [browse-tracer](browse-tracer.md);
updated [project-overview](project-overview.md),
[source-code](source-code.md), [unit-tests](unit-tests.md),
[search-collection](search-collection.md),
[cancellation-cleanup](cancellation-cleanup.md), and the index. Sources:
`Notes/issues/005-browse-tracer-file-list-and-file-panel.md`,
`Notes/tasks/005-browse-tracer-file-list-and-file-panel.md`,
`Notes/PRD-vrg.md` (File list and layout; File loading, cache, reload;
Layout and indicators; Text, graphemes, and safe presentation; Module
Design), `cmd/vrg/main.go`, `internal/app/app.go`,
`internal/app/browse.go`, `internal/app/browse_test.go`,
`internal/filebuffer/filebuffer.go`, `internal/filebuffer/filebuffer_test.go`,
`internal/safepresentation/safepresentation.go`,
`internal/safepresentation/cellwidth.go`,
`internal/safepresentation/safepresentation_test.go`,
`internal/viewport/viewport.go`, `internal/theme/theme.go`.

## [2026-09-16] ingest | Issue #6 shared safe-presentation utility and sink-safety table

Ingested the completed Issue #6 implementation: `internal/safepresentation`
is now the single shared utility every output sink routes through —
`EscapePath` (paths/filenames, single-line) and `MapContent` (content
lines with byte→cell maps) unchanged from Issue #5, plus the new
`EscapeDiagnostic` (`internal/safepresentation/diagnostic.go`): real
diagnostic line boundaries preserved (LF kept, CRLF normalized to LF),
tabs expanded to eight-column stops, other controls escaped (standalone
CR → `^M`, caret/C1 `\uXXXX`/invalid `\xNN`, literal backslash), with
embedded external strings single-line-escaped via `EscapePath` first.
The Issue #1 `cli.Escape` is removed: `internal/cli` usage diagnostics
embed `EscapePath`-escaped operands and pass through `EscapeDiagnostic`
(`usageErrorf`), `renderHelp` output passes through `EscapeDiagnostic`
(expanding its layout tabs — routing the CLI-help stdout sink), and the
`internal/app`/`cmd/vrg` stderr diagnostics escape embedded error text
via `EscapePath`. The new test-support package
`internal/safepresentation/sinktest` holds the restructured hostile
fixture set (OSC, CSI, C0 run, C1, DEL, standalone CR, invalid UTF-8,
embedded filename newline) plus `AssertRawOutput` (no-style raw-output
cleanliness before ANSI stripping) and `AssertPayloadNotEscaped`
(styled payload-after-ESC); `internal/app/sinksafety_test.go` hosts the
five-row table — file-list entry, filename rule, panel content,
usage-error stderr, CLI-help stdout — that Issues #9/#11/#15/#31/#34
extend for their sinks. Issue #5 escaping tests, the browse
sink-safety test, and the Issue #1 CLI output tests pass unchanged.
Created [safe-presentation](safe-presentation.md); updated
[cli-foundation](cli-foundation.md), [browse-tracer](browse-tracer.md),
[search-collection](search-collection.md),
[cancellation-cleanup](cancellation-cleanup.md),
[source-code](source-code.md), [unit-tests](unit-tests.md), and the
index. Sources:
`Notes/issues/006-safe-presentation-utility-for-all-sinks.md`,
`Notes/tasks/006-safe-presentation-utility-for-all-sinks.md`,
`Notes/PRD-vrg.md` (Text, graphemes, and safe presentation),
`cmd/vrg/main.go`, `internal/app/app.go`,
`internal/app/sinksafety_test.go`, `internal/cli/cli.go`,
`internal/safepresentation/safepresentation.go`,
`internal/safepresentation/diagnostic.go`,
`internal/safepresentation/cellwidth.go`,
`internal/safepresentation/diagnostic_test.go`,
`internal/safepresentation/sinktest/sinktest.go`.

## [2026-09-16] ingest | Issue #7 theme: colour toggle, inverse matches, underlines

Ingested the completed Issue #7 implementation: `internal/theme` is now
the full style set from Module Design — `Dark()`/`Light()` schemes
(white-on-black `37;40` initially, black-on-white `30;47`), the pure
`Toggled()` flip behind the App's base-state `c` keypress with no
persistence, and the decorators `Base` (the frame's outermost wrap so
padding cells carry the background), `Gutter`, `FilenameRule`,
`FileList`, `Match` (the true inverse — the scheme's pair swapped by
SGR 3x↔4x, not SGR 7 reverse video), `CurrentMatch` (inverse + `;4`,
the current matched line being the current file's first stop until
Issue #13), `Indicator` (the inverse style), `CurrentFile` (base +
underline), and `Overlay` (rows framed in a plain single-line border,
all base colours, still drawn on the no-style path). Every styled run
closes by re-asserting base and clearing underline. `Plain()` and the
sink-safety contract are unchanged. `internal/app` routes the whole
`View()` through `Base` and each browse region through its style.
Created [theme](theme.md); updated [browse-tracer](browse-tracer.md),
[source-code](source-code.md), [unit-tests](unit-tests.md),
[project-overview](project-overview.md), and the index. Sources:
`Notes/issues/007-theme-colour-toggle-and-match-styles.md`,
`Notes/tasks/007-theme-colour-toggle-and-match-styles.md`,
`Notes/PRD-vrg.md` (Colours, overlays, and key precedence; Module
Design → Theme), `internal/theme/theme.go`,
`internal/theme/theme_test.go`, `internal/theme/doc.go`,
`internal/app/app.go`, `internal/app/browse.go`,
`internal/app/browse_test.go`, `internal/app/cancel_test.go`,
`cmd/vrg/search_test.go`.

## [2026-09-16] ingest | Issue #8 no-results screen and binary exclusion

Ingested the completed Issue #8 implementation: `internal/searchindex`'s
`Index.Feed` now drops a file and all its previously collected matches
when a valid `end` event reports a non-null `binary_offset`, counting
distinct excluded files in `Index.BinaryExcluded`, and
`Index.UsableResults()` exposes retained stops after filtering as the
single value the outcome logic consumes. `internal/app` gained
`stateNoResults`: a completed search with zero usable results — the
rg-1 empty stream and the rg-0/rg-1 all-filtered stream alike — shows
the centred "No results found" screen, appending "(N binary files
skipped)" when every matched file was excluded; `q` exits 1 through the
Issue #4 `quitCmd`/`reapChild` cleanup path, `Esc` is a no-op, and
`ctrl+c` keeps the 130 override. New `internal/app/noresults_test.go`
drives the empty, all-binary, and mixed streams through the real
collection command. Created
[no-results-screen](no-results-screen.md); updated
[search-collection](search-collection.md),
[project-overview](project-overview.md), [source-code](source-code.md),
[unit-tests](unit-tests.md), and the index. Sources:
`Notes/issues/008-no-results-screen-and-binary-exclusion.md`,
`Notes/tasks/008-no-results-screen-and-binary-exclusion.md`,
`Notes/PRD-vrg.md` (Result index, records, and stream integrity —
binary bullet; Outcome and exit-status contract — last row),
`internal/searchindex/index.go`, `internal/searchindex/index_test.go`,
`internal/app/app.go`, `internal/app/noresults_test.go`.

## [2026-09-16] ingest | Issue #9 error overlay and fatal outcomes

Ingested Issue #9 ([issue](../issues/009-error-overlay-and-fatal-outcomes.md),
[task](../tasks/009-error-overlay-and-fatal-outcomes.md)). `internal/searchindex`
gained the lifecycle transition matrix: per-path open tracking on decoded raw
bytes, retained-with-`Incomplete` orphaned matches, binary-exclusion precedence
over retention (excluded paths cannot reopen; a well-formed `binary_offset` end
excludes even when orphaned), summary-is-final with the Issue #9 context
exemption (Issues #36/#44 correct it), the `FeedTail` unterminated-fragment
double disposition, and `Integrity().Complete` assessed separately from the
child's process result. `internal/app` gained `outcome.go` — the pure
`DecideOutcome` mapping process result × integrity × usable results to
presentation, overlay lines, dismiss-exits, and the fixed status — and
`overlay.go`, the modal single-line-bordered overlay: grapheme-boundary
wrapping to interior width, complete-row-set scrolling clamped to
`[0, rows − visible]`, `q`/`Esc` dismissal (both exiting when there is no
underlying state), `ctrl+c` → 130 precedence, all other keys ignored. stderr is
diagnostic regardless of exit code; a silent failed process gets a generated
code-or-signal line. The search-derived status is fixed at completion; only
`ctrl+c` overrides it. Created
[error-overlay-and-outcomes](error-overlay-and-outcomes.md); updated
[search-collection](search-collection.md),
[no-results-screen](no-results-screen.md), [source-code](source-code.md),
[unit-tests](unit-tests.md), and the index. Sources:
`Notes/issues/009-error-overlay-and-fatal-outcomes.md`,
`Notes/tasks/009-error-overlay-and-fatal-outcomes.md`,
`Notes/PRD-vrg.md` (Result index, records, and stream integrity; Outcome and
exit-status contract; Colours, overlays, and key precedence),
`internal/searchindex/index.go`, `internal/searchindex/lifecycle_test.go`,
`internal/app/app.go`, `internal/app/outcome.go`, `internal/app/overlay.go`,
`internal/app/outcome_test.go`, `internal/app/overlay_test.go`,
`internal/app/sinksafety_test.go`, `cmd/vrg/outcome_test.go`,
`cmd/vrg/search_test.go`.

## [2026-09-16] ingest | Issue #10 record robustness — malformed, oversized, unknown

Ingested Issue #10 ([issue](../issues/010-record-robustness-malformed-oversized-unknown.md),
[task](../tasks/010-record-robustness-malformed-oversized-unknown.md)).
`internal/searchindex` gained the deterministic skip counters —
`Index.Malformed` and `Index.Unknown` classified per record and
unqualified by position (a malformed or unknown record after `summary`
counts *and* fails integrity; a skipped match never breaks intact
lifecycle metadata) — plus `oversized.go`: `MaxRecordBytes` (64 MiB
excluding the newline), `Build`'s bounded scan discarding an oversized
record through its next newline and resynchronizing on the following
one, `feedOversized`/`recoverOversizedPath` counting separately with
best-effort `type`+`data.path` recovery into `OversizedPaths`, and the
unterminated oversized tail's triple disposition (oversized + malformed
+ incomplete). `internal/app` filled `RecordLoss` (`Malformed`,
`Oversized`, `Paths`), added the fatal record-loss row to
`DecideOutcome` — complete stream, records skipped, zero usable results
assessed after all filtering → overlay-only exit 2 — and
`recordLossLines`/`recordWarnings` compose the count, per-path, and
"N unrecognised record types skipped" diagnostics. New tests:
`disposition_test.go` (schema-matrix and lifecycle-matrix disposition
tables plus both composite rows), `oversized_test.go` (boundary,
resync, named/anonymous paths, absent oversized-only file, triple
disposition, unknown-not-summary), and six new outcome-matrix rows.
Created [record-robustness](record-robustness.md); updated
[search-collection](search-collection.md),
[error-overlay-and-outcomes](error-overlay-and-outcomes.md),
[source-code](source-code.md), [unit-tests](unit-tests.md), and the
index. Sources:
`Notes/issues/010-record-robustness-malformed-oversized-unknown.md`,
`Notes/tasks/010-record-robustness-malformed-oversized-unknown.md`,
`Notes/PRD-vrg.md` (Result index, records, and stream integrity;
Outcome and exit-status contract; Resources and responsiveness),
`internal/searchindex/index.go`, `internal/searchindex/oversized.go`,
`internal/searchindex/disposition_test.go`,
`internal/searchindex/oversized_test.go`, `internal/app/app.go`,
`internal/app/outcome.go`, `internal/app/outcome_test.go`.

## [2026-09-16] ingest | Issue #11 stderr replay of collected diagnostics

Implemented the session diagnostic collection and post-restoration
stderr replay. `internal/app/rg.go` gained incremental stderr delivery:
`drainStderr` line-splits stderr into a mutex-guarded pending queue so
drainage never blocks on a consumer, and `feedDiags` (started lazily by
the new `Child.Diags()`) forwards the queue onto a channel closed at
EOF; `Run` forwards each line into the model as `stderrLineMsg` via
`prog.Send`. `internal/app/app.go` gained `model.diags` —
`collectDiags` appends sanitized lines in the order `Update` processes
the carrying message (`stderrLineMsg`, `searchDoneMsg` via
`collectSearchDiags` — which skips captured stderr when the incremental
route already delivered it — `fileLoadedMsg` failures via the new
`loadDiag` in `browse.go`, and `failMsg`) — plus `replayDiags`, the
common post-restoration stderr writer that replaced `writeFailureDiag`:
after `program.Run` returns and `reapChild` runs, every collected line
is emitted exactly once, in collection order, on ordinary quit,
cancellation (130), and controlled failure (2) alike; a `program.Run`
error appends through the same writer. `outcome.go` split
`outcomeDiagnostics` into `processDiags`/`tailDiags` so collection and
display share composition. New option `WithDiagAck` fires once per
collected line, wired to `VRG_TEST_DIAG_ACK` in `cmd/vrg/main.go`.
New tests: `internal/app/replay_test.go` (ordered exactly-once
collection, never-displayed diagnostics, the `ctrl+c`/`q`/gate-held
shutdown boundaries, incremental-vs-completion deduplication,
controlled-failure collection, hostile embedded filenames) and
`cmd/vrg/replay_test.go` (PTY acknowledgement via `waitForAcks`,
replay-after-restoration ordering and exactly-once on cancel,
gate-held, normal-quit, and controlled-failure routes, escaped hostile
filename); `sinksafety_test.go` gained the stderr-replay row (seven
sinks). Created [stderr-replay](stderr-replay.md); updated
[cancellation-cleanup](cancellation-cleanup.md),
[error-overlay-and-outcomes](error-overlay-and-outcomes.md),
[safe-presentation](safe-presentation.md), [source-code](source-code.md),
[unit-tests](unit-tests.md), and the index. Sources:
`Notes/issues/011-stderr-replay-of-collected-diagnostics.md`,
`Notes/tasks/011-stderr-replay-of-collected-diagnostics.md`,
`Notes/PRD-vrg.md` (Outcome and exit-status contract; Colours,
overlays, and key precedence; Testing Decisions → Subprocess boundary),
`internal/app/app.go`, `internal/app/rg.go`, `internal/app/browse.go`,
`internal/app/outcome.go`, `internal/app/replay_test.go`,
`internal/app/sinksafety_test.go`, `cmd/vrg/main.go`,
`cmd/vrg/replay_test.go`.

## [2026-09-16] ingest | Issue #12 manual vertical scrolling and per-file viewport

Ingested Issue #12 ([issue](../issues/012-manual-vertical-scrolling-and-per-file-viewport.md),
[task](../tasks/012-manual-vertical-scrolling-and-per-file-viewport.md)).
`internal/viewport` replaced the Issue #5 `Visible` seam with the real
scroll surface: `Viewport.Scroll`/`Clamp` move the top rendered row under
the valid-content bound `MaxTop` (`max(0, rows − height)` — never below 0,
never leaving avoidable blank rows below EOF, shorter-than-viewport files
clamped to 0), `HalfPage` is the `u`/`d` unit `max(1, floor(height/2))`,
and `Rows`/`Prepare` build the prepared rendered-row model (one row per
source line in the still-unwrapped panel, plus the gutter width) that the
frame render slices. `internal/app` gained the `rowSource` seam
(`Len`/`At`/`GutterWidth`) so a frame queries only `[top, top + content
height)`, the `vps` per-file saved-viewport map written through on every
scroll and re-clamped on resize and load completion, and `scrollBy` behind
`stateBrowse && isScrollKey` — `up`/`down` one row, `u`/`d` half page,
`pgup`/`pgdown` a page of the content height (frame − filename row), a
strict no-op on "Loading…"/"(unreadable)" placeholders, never moving the
matched-line cursor. New tests: `internal/viewport/viewport_test.go`
(units, odd-height half pages, both clamps across file-length cases,
lossy clamp, `Prepare`) and `internal/app/scroll_test.go` (key-driven
movement, EOF/BOF stops, short-file blank rows, placeholder no-ops,
per-file save/restore, the `countingRows` render-cost guard, resize
re-clamp). Created [viewport-scrolling](viewport-scrolling.md); updated
[browse-tracer](browse-tracer.md), [source-code](source-code.md),
[unit-tests](unit-tests.md), and the index. Sources:
`Notes/issues/012-manual-vertical-scrolling-and-per-file-viewport.md`,
`Notes/tasks/012-manual-vertical-scrolling-and-per-file-viewport.md`,
`Notes/PRD-vrg.md` (Navigation, viewport, and logical anchors; Module
Design → Viewport; Resources and responsiveness),
`internal/viewport/viewport.go`, `internal/viewport/viewport_test.go`,
`internal/app/app.go`, `internal/app/browse.go`,
`internal/app/scroll_test.go`.
## [2026-09-17] ingest | Issue #13 n/p circular matched-line navigation

Ingested Issue #13 ([issue](../issues/013-match-navigation-n-p-circular-cursor.md),
[task](../tasks/013-match-navigation-n-p-circular-cursor.md)).
`internal/searchindex` gained `cursor.go`: `Cursor{File, Stop}` positions
into `Files`/`Stops` (the zero value is the startup selection — first stop
in path-then-line order), `Index.Cursor()` reporting it (absent on an
empty index), and `Next`/`Prev` stepping one stop circularly with wrap at
both ends, returning `Move{Wrapped, FileChanged}` — the flags independent
so a one-file wrap reports `Wrapped` alone; the prepared `stops` total
makes zero- and one-stop indexes strict no-ops, and a matched line is one
stop however many submatches it holds. `internal/app` dropped `model.cur`:
`model.curFile()` derives the current file from the cursor everywhere
(`curKey`, `startLoad`, list underline and scroll-keeping, filename rule)
and `browseView`'s `curLine` is the cursor's stop — the Issue #7
inverse-plus-underline style follows the selection. The browse `n`/`p`
route calls `model.navigate`: a `FileChanged` move issues `startLoad` for
the destination (deduplicated against cached/in-flight/failed), the panel
switches immediately — reveal is Issue #14's, the stale-layout request is
Issue #17's — and the departing file's viewport is already saved by scroll
write-through, so destinations resume their saved top or start at the top.
Manual scrolling never moves the cursor; the file list stays passive with
no direct selection route. New tests:
`internal/searchindex/cursor_test.go` (startup, both-direction order and
wrap, flag reports, within-one-file wrap, single-stop and empty no-ops,
submatches sharing a stop, prepared over stream order) and
`internal/app/nav_test.go` (startup selection, same-file underline-only
move, cross-file switch with load request and top-of-file start, wrap at
both ends, single-stop no-op, scroll independence, saved-viewport restore,
in-flight-load navigation with dedup, the passive list). Created
[match-navigation](match-navigation.md); updated
[search-collection](search-collection.md),
[browse-tracer](browse-tracer.md),
[viewport-scrolling](viewport-scrolling.md), [source-code](source-code.md),
[unit-tests](unit-tests.md), and the index. Sources:
`Notes/issues/013-match-navigation-n-p-circular-cursor.md`,
`Notes/tasks/013-match-navigation-n-p-circular-cursor.md`,
`Notes/PRD-vrg.md` (Navigation, viewport, and logical anchors — first
three bullets; Module Design → SearchIndex / App; File loading, cache,
reload — navigation-active-while-loading bullet),
`internal/searchindex/cursor.go`, `internal/searchindex/cursor_test.go`,
`internal/searchindex/index.go`, `internal/app/app.go`,
`internal/app/browse.go`, `internal/app/nav_test.go`,
`internal/app/scroll_test.go`.
## [2026-09-17] ingest | Issue #14 vertical destination reveal

Ingested Issue #14 ([issue](../issues/014-vertical-destination-reveal.md),
[task](../tasks/014-vertical-destination-reveal.md)).
`internal/viewport` gained `reveal.go`: `Target{Line, Cell}` — the
display target as the start cell of the first submatch on the
destination line, not a source-line ordinal — resolved by
`Rows.StopTarget` through the line's byte→cell map (escaped bytes widen,
so the cell is not the byte offset; a no-cell submatch lands on the
marker cell one past the last), `Rows.TargetRow` mapping the target to
its rendered row (one row per source line until Issue #16's wrap), and
`Viewport.Reveal` — visible-target no-scroll, else top =
`row − floor(h/3)` clamped to `[0, MaxTop]` so BOF/EOF content beats
one-third placement, reporting whether the viewport moved.
`internal/app` extended `rowSource` with `TargetRow`, added
`model.reveal` — saved `vps` top or first-visit top-of-file as the
start, written back only on a move — and wired it into `navigate` on
every actual cursor transition (strict no-ops detected by cursor
comparison trigger none) and into `fileLoadedMsg` when the completed
path is current — the startup-after-load trigger, always revealing the
latest cursor target. Issue #19 owns horizontal reveal and Issue #28
the two-stage prepared-layout commit. New tests:
`internal/viewport/reveal_test.go` (start-cell identification through
escapes, zero-width marker cell, target-row clamps, visible no-scroll,
one-third both directions, BOF/EOF precedence, saved-vs-first-visit
start) and `internal/app/reveal_test.go` (startup reveal after load,
visible-target stays top with no state recorded, n/p round trip with
BOF clamp, no-scroll between on-screen stops, no reveal on no-op, moving
reveal replacing saved state, saved-start revisit, cached first visit,
reveal on load completion); Issue #12/#13 tests updated where they
encoded the pre-reveal contract. Created
[destination-reveal](destination-reveal.md); updated
[match-navigation](match-navigation.md),
[viewport-scrolling](viewport-scrolling.md),
[source-code](source-code.md), [unit-tests](unit-tests.md), and the
index. Sources:
`Notes/issues/014-vertical-destination-reveal.md`,
`Notes/tasks/014-vertical-destination-reveal.md`,
`Notes/PRD-vrg.md` (Navigation, viewport, and logical anchors —
target-row, placement, and file-change bullets; File loading —
load-completion reveal bullet; Testing Decisions → Viewport),
`internal/viewport/reveal.go`, `internal/viewport/reveal_test.go`,
`internal/app/browse.go`, `internal/app/app.go`,
`internal/app/reveal_test.go`, `internal/app/nav_test.go`,
`internal/app/scroll_test.go`.

## [2026-09-17] ingest | Issue #15 file-change pop-up

The centred path pop-up on `n`/`p` file crossings, implemented in
`internal/app/popup.go` (`popupExpireMsg{id}`, `startPopup`,
`compositePopup`, `leftTruncate`/`tailCells`) with `Update`/`View`
wiring in `internal/app/app.go` (`popupID`/`popupSeq`, the
stale-instance-rejecting expiry case, any-key dismissal before the key
switch, pop-up composited under the error overlay) and `openOverlay`
extracted in `internal/app/overlay.go` to clear the live pop-up on
open-or-append. `navigate` in `internal/app/browse.go` now returns
`tea.Batch(startLoad(), startPopup())` on `Move.FileChanged` — the
pop-up starts at selection while the destination may still be loading,
load completion never restarts it, and a cached/in-flight/failed
destination contributes only the pop-up leaf. Contracts: fresh
one-second instance per crossing, expiry dismisses only its own
instance, any key dismisses and still performs its normal action,
render-time centring and `…`-led left-truncation recomputed on resize
without touching the timer, overlay cancellation with no return, and
the single-line `EscapePath`-sanitized path added to the sink-safety
table. New tests: `internal/app/popup_test.go` (selection-time start
over "Loading…", centring, stale-instance rejection,
dismissal-plus-action including `q`/`Esc`, resize recentring without
restart, overlay cancellation, truncation, hostile path) under the
`popupStubTicks` options seam with `leafMsgs`/`navLeafMsgs`/
`deliverLoad` batch helpers; `sinksafety_test.go` gained the pop-up
row via `popupFixtureView`; `browse_test.go`'s `finishLoad` unwraps
`tea.BatchMsg` (`fileLoadOf`); the pre-#15 "cached file returns no
command" assertions in `nav_test.go`/`reveal_test.go` became
no-load-leaf checks (`navSendsNoLoad`). Created
[file-change-popup](file-change-popup.md); updated
[match-navigation](match-navigation.md),
[error-overlay-and-outcomes](error-overlay-and-outcomes.md),
[safe-presentation](safe-presentation.md), [source-code](source-code.md),
[unit-tests](unit-tests.md), and the index. Sources:
`Notes/issues/015-file-change-popup.md`,
`Notes/tasks/015-file-change-popup.md`,
`Notes/PRD-vrg.md` (Colours, overlays, and key precedence — the pop-up
bullet; File loading — selection-time-start bullet; user stories
58–59), `internal/app/popup.go`, `internal/app/popup_test.go`,
`internal/app/app.go`, `internal/app/browse.go`,
`internal/app/overlay.go`, `internal/app/sinksafety_test.go`,
`internal/app/browse_test.go`, `internal/app/nav_test.go`,
`internal/app/reveal_test.go`.

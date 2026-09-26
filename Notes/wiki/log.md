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

## [2026-09-23] ingest | Issue #2 flag allow-list and child argv

Ingested the completed Issue #2 implementation: `optionDecls` gained
`forward`/`unrestricted` fields and the eleven allow-listed no-argument
search flags; `scanArgs`/`scanOption`/`expandOption` record accepted
spellings in encounter order (combined shorts expand left to right),
enforce the cumulative two-`-u` cap, and reject every `=` assignment
spelling lexically — including help assignments, which are no longer
help (`--help=true` now errors too; the parsed-value check is a
defensive unreachable seam). `Result.ChildArgv` carries the exact
`--json --no-config <flags> -- pattern root` vector and the stub prints
it. Created [cli-flags-and-child-argv](cli-flags-and-child-argv.md);
updated [cli-foundation](cli-foundation.md) (stale `=`-assignment and
forward-reference notes), [source-code](source-code.md),
[unit-tests](unit-tests.md), [project-overview](project-overview.md),
and the index. Tests using `-x` as an unsupported example moved to `-z`
(`-x` is now `--line-regexp`). Sources:
`Notes/issues/002-cli-flag-allow-list-and-child-argv.md`,
`Notes/tasks/002-cli-flag-allow-list-and-child-argv.md`,
`Notes/PRD-vrg.md` (Invocation and child arguments),
`internal/cli/cli.go`, `internal/cli/cli_test.go`,
`internal/cli/internal_test.go`, `cmd/vrg/main.go`,
`cmd/vrg/main_test.go`.

## [2026-09-23] ingest | Issue #3 rg spawn, collection, and searching screen

Ingested the completed Issue #3 implementation: `internal/searchindex`
gained `DecodeRecord` (the five rg JSON events plus `KindUnknown`, the
`{"text"}`/`{"bytes"}` value union, `ErrMalformed` schema/range checks)
and the `Index` (stops keyed by raw path bytes + line, submatch sorting
and union highlights, unsigned-byte ordering, non-canonicalizing
workdir resolution). `internal/app` gained `Start`/`Config`/`collect`
(`exec.CommandContext` in the invocation working directory, concurrent
stdout/stderr drainage for the whole child lifetime, `Drained`/
`PrepareGate` test seams) and the Bubble Tea `Model` (`Searching…`
across collection and post-exit preparation, `N files, M matched lines`
interim summary, `q` → exit 0). `cmd/vrg` replaced the `search stub:`
line with `runSearch` (rg start failure → sanitized `vrg:` diagnostic,
exit 2, no TUI; `WithInput(os.Stdin)` and a `WithWindowSize` fallback so
piped stdio still renders). Created
[searchindex-records-and-stops](searchindex-records-and-stops.md) and
[search-spawn-and-searching-screen](search-spawn-and-searching-screen.md);
updated [source-code](source-code.md), [unit-tests](unit-tests.md),
[project-overview](project-overview.md), [cli-foundation](cli-foundation.md)
(stale stub references), [cli-flags-and-child-argv](cli-flags-and-child-argv.md)
(stub → execution), and the index. The `TestChildArgvStub` boundary test
became `TestSearchLifecycleAtBoundary` plus `TestDualPipeDrainageAtBoundary`
and `TestStartFailureNoRipgrep`. Sources:
`Notes/issues/003-spawn-rg-collect-results-searching-screen.md`,
`Notes/tasks/003-spawn-rg-collect-results-searching-screen.md`,
`Notes/PRD-vrg.md` (Result index, records, and stream integrity; Module
Design), `internal/searchindex/record.go`, `internal/searchindex/index.go`,
`internal/searchindex/searchindex_test.go`, `internal/app/app.go`,
`internal/app/search.go`, `internal/app/model_test.go`,
`internal/app/subprocess_test.go`, `cmd/vrg/main.go`,
`cmd/vrg/main_test.go`.

## [2026-09-23] ingest | Issue #4 cancellation, child cleanup, and terminal restoration

Ingested the completed Issue #4 implementation: `internal/app` gained
`Session` (`Model`/`Cancel`/`Reaped`, returned by `Start`) and
`Config.ReapReport` (the reap-evidence side channel written after
`cmd.Wait`); `collect` now reports the wait status, abandons a held
`PrepareGate`, and skips index preparation on cancellation. The `Model`
gains `cancelled`/`quit` handling: `ctrl+c` in any state and `q` while
searching (including the post-exit gate-held window) kill the child
context and exit 130, `q` on the summary still exits 0, `Esc` stays a
searching no-op, and `quit` rejects late completions so a cancelled UI
cannot revive; every `View` sets `AltScreen` for the exit restoration
sequence. `cmd/vrg` gained `hooks.go` (`VRG_TEST_REAP_FILE`,
`VRG_TEST_GATE_FIFO`, `VRG_TEST_FAIL_FIFO`) and `runSearch` now routes
every controlled exit through one boundary: `Cancel`, `<-Reaped()`, then
status selection (`ErrInterrupted` → 130, `Run` error → single
post-restoration sanitized `vrg:` diagnostic + exit 2, else the model's
`ExitCode`). Created
[cancellation-and-cleanup](cancellation-and-cleanup.md); updated
[source-code](source-code.md), [unit-tests](unit-tests.md) (new
cancellation model tests, `TestCancelTerminatesAndReapsChild`, the
`TestPTY*` harness group), [search-spawn-and-searching-screen](
search-spawn-and-searching-screen.md) (drainage and boundary updates),
and the index. New test files: `cmd/vrg/pty_test.go` (PTY harness,
fake-rg block/stream scripts, termios equality). Sources:
`Notes/issues/004-cancellation-child-cleanup-terminal-restore.md`,
`Notes/tasks/004-cancellation-child-cleanup-terminal-restore.md`,
`Notes/PRD-vrg.md` (Outcome and exit-status contract; Module Design →
App), `internal/app/app.go`, `internal/app/search.go`,
`internal/app/model_test.go`, `internal/app/subprocess_test.go`,
`cmd/vrg/main.go`, `cmd/vrg/hooks.go`, `cmd/vrg/pty_test.go`.

## [2026-09-23] ingest | Issue #5 browse tracer and safe-presentation core

Ingested the completed Issue #5 implementation: the interim summary is
replaced by the two-pane browse view. `internal/filebuffer` gained the
safe-presentation core (`present.go`: `EscapePath` path rules —
`\n`/`\r`/`\t`, `\\`, `\xNN`, caret notation, `\uXXXX` C1 — and
`presentLine` content rules — U+FFFD, caret notation, invisible
LF/CRLF, `^M` standalone CR, provisional `→` tab — with per-byte
`lo`/`hi` byte→cell maps and marker spans) and `filebuffer.go`
(`Buffer`/`Load`: read, split, escape, map, submatch validation,
`GutterWidth`). `internal/viewport` and `internal/theme` gained their
minimal seams (`Range` clamp; `Dark`/`Plain` styles). `internal/app`
gained `browse.go` (`loadDoneMsg` prepared-buffer completion,
`ensureLoad`/`loadCmd` async workers behind `loadGate`,
`renderBrowse`/`filenameRule`/`contentRow`/`renderCells` with
inverse-video matches) and `app.go`'s `phaseBrowse` state. Created
[browse-tracer](browse-tracer.md); updated
[source-code](source-code.md), [unit-tests](unit-tests.md) (new
filebuffer and browse test catalogs; summary-era entries retargeted),
[project-overview](project-overview.md), and the index. Boundary and
PTY tests now watch for the browse filename marker instead of the
interim summary. Sources:
`Notes/issues/005-browse-tracer-file-list-and-file-panel.md`,
`Notes/tasks/005-browse-tracer-file-list-and-file-panel.md`,
`Notes/PRD-vrg.md` (File list and layout; Text, graphemes, and safe
presentation; Module Design), `internal/filebuffer/present.go`,
`internal/filebuffer/filebuffer.go`, `internal/viewport/viewport.go`,
`internal/theme/theme.go`, `internal/app/browse.go`,
`internal/app/app.go`, `internal/filebuffer/present_test.go`,
`internal/filebuffer/filebuffer_test.go`, `internal/app/browse_test.go`,
`internal/app/model_test.go`, `cmd/vrg/main_test.go`,
`cmd/vrg/pty_test.go`.

## [2026-09-23] ingest | Issue #6 safe-presentation utility for all sinks

Ingested the completed Issue #6 implementation: the Issue #5 escaping
core moved out of `internal/filebuffer` into the new shared
`internal/present` package — `Path` (was `EscapePath`), `LineOf`/`Line`
(was `presentLine`/`presented`), `Cell`, `Span` — and gained
`Diagnostic` (real LF/CRLF line boundaries preserved, tabs expanded to
8-column stops, other controls escaped, `Path`-escaped filenames embed
single-lined). `cli.Escape` is removed: `internal/cli` escapes hostile
substitutions through `present.Path`, `cmd/vrg` renders boundary error
text through `present.Diagnostic`, and `internal/app` routes the file
list, filename rule, and panel content through the package.
`internal/app/sinksafety_test.go` restructured the Issue #5 hostile
fixture set into `TestSinkSafetyTable`: shared fixtures × sink rows
(file-list entry, filename rule, panel content, usage-error stderr,
CLI-help stdout) rendered through `theme.Plain` with raw-output
assertions, a styled payload-after-ESC pass, and an extensible
`sinkSafetySinks` table later issues (#9, #11, #15, #31, #34) extend
with their own sink rows. The Issue #5 core cases moved with the
utility unchanged (mechanical renames only); the Issue #1 CLI output
tests pass unmodified. Created
[safe-presentation](safe-presentation.md); updated
[source-code](source-code.md), [unit-tests](unit-tests.md),
[browse-tracer](browse-tracer.md) (core relocation, table restructure),
[cli-foundation](cli-foundation.md) (escaper replacement),
[project-overview](project-overview.md), and the index. Sources:
`Notes/issues/006-safe-presentation-utility-for-all-sinks.md`,
`Notes/tasks/006-safe-presentation-utility-for-all-sinks.md`,
`Notes/PRD-vrg.md` (Text, graphemes, and safe presentation; Module
Design), `internal/present/doc.go`, `internal/present/present.go`,
`internal/present/line.go`, `internal/present/present_test.go`,
`internal/present/line_test.go`, `internal/app/sinksafety_test.go`,
`internal/app/browse.go`, `internal/filebuffer/filebuffer.go`,
`internal/cli/cli.go`, `cmd/vrg/main.go`.

## [2026-09-23] ingest | Issue #7 theme module and colour toggle

Ingested the completed Issue #7 implementation: `internal/theme` is now
the real Theme module — the `dark`/`light` scheme pairs (`37;40` white
on black, `30;47` black on white, dark initially active), `Toggle`/`Light`
(in-memory only, no persistence), and the full named style set: `Base`
(frame wrap), `Gutter`, `FileList`, `FilenameRule`, `Match` and
`Indicator` (the scheme's true inverse pair via `scheme.inverse`),
`CurrentMatch` (inverse + underline), `CurrentFile` (underline), and
`Overlay` (base colours, plain single-line border). Styles emit explicit
colour pairs rather than bare SGR 7 and restore the base pair so nested
runs compose inside a `Base` frame; `Plain` remains the identity
no-style path for the sink-safety table. `internal/app` handles `c` in
`phaseBrowse` (`m.theme.Toggle()`), wraps each `View` frame in `Base`,
pads each browse row to the frame edge, and routes the file list,
filename rule, gutter, and match spans through the new styles with a
`current`-line flag selecting `CurrentMatch`. The Issue #5 `\x1b[7m`
assertions moved to explicit-pair codes; new tests cover the toggle
round trip, both schemes' pairs, true-inverse matches/indicators, the
current-match and current-file underlines, the overlay style, `c`
flipping the frame's base pair, and the current-line underline
distinction. Created
[theme-and-colour-toggle](theme-and-colour-toggle.md); updated
[source-code](source-code.md), [unit-tests](unit-tests.md),
[browse-tracer](browse-tracer.md), and the index. Sources:
`Notes/issues/007-theme-colour-toggle-and-match-styles.md`,
`Notes/tasks/007-theme-colour-toggle-and-match-styles.md`,
`Notes/PRD-vrg.md` (Colours, overlays, and key precedence; Module
Design → Theme), `internal/theme/theme.go`,
`internal/theme/theme_test.go`, `internal/app/app.go`,
`internal/app/browse.go`, `internal/app/browse_test.go`.

## [2026-09-23] ingest | Issue #8 no-results screen and binary exclusion

Ingested the completed Issue #8 implementation: `internal/searchindex`
gained binary exclusion — `Index.Add` now dispatches end records, a
non-null `binary_offset` drops the file's collected stops and marks its
raw path in the `excluded` set (idempotent distinct-file tally via
`BinaryExcluded()`; later matches for an excluded path drop too), and
`LineCount()` is documented as the usable-results value — retained
stops after filtering — that the outcome logic consumes. `internal/app`
gained `phaseNoResults` and `renderNoResults` (`noresults.go`): a done
message with zero usable results fixes `ExitCode` at 1 and shows the
centred "No results found" screen, appending "(N binary files skipped)"
when exclusion emptied the list — identical for rg-1 empty and rg-0
all-filtered streams — with `q` quitting 1 through the ordinary path,
`Esc` a no-op, and `ctrl+c` overriding to 130. `waitErr`/`stderr` remain
unconsumed cargo pending Issue #9's outcome function. Created
[no-results-and-binary-exclusion](no-results-and-binary-exclusion.md);
updated [searchindex-records-and-stops](searchindex-records-and-stops.md)
(binary exclusion section), [search-spawn-and-searching-screen](
search-spawn-and-searching-screen.md) (stale interim-summary section
replaced by the completion-destination branch),
[source-code](source-code.md), [unit-tests](unit-tests.md), and the
index. Sources:
`Notes/issues/008-no-results-screen-and-binary-exclusion.md`,
`Notes/tasks/008-no-results-screen-and-binary-exclusion.md`,
`Notes/PRD-vrg.md` (Result index, records, and stream integrity;
Outcome and exit-status contract), `internal/searchindex/index.go`,
`internal/searchindex/searchindex_test.go`, `internal/app/app.go`,
`internal/app/noresults.go`, `internal/app/noresults_test.go`.

## [2026-09-23] ingest | Issue #9 error overlay and fatal outcomes

Ingested the completed Issue #9 implementation. `internal/searchindex`
gained lifecycle validation: `Index.Feed` consumes the collected stream
(skipping schema-failing records — their counting stays Issue #10's —
and flagging a trailing unterminated record), per-path `open`/`incomplete`
state tracks the begin/end matrix over decoded raw path bytes with
binary exclusion taking precedence over orphan retention, `seal`
applies the end-of-stream rules once inside `Prepare`, and
`IntegrityFailures()` reports stream integrity separately from process
success; `Stop.Incomplete` marks stops with damaged file metadata.
`internal/app` gained `overlay.go`: the pure `decideOutcome` function
(wait error × stderr × integrity failures × usable results, record-loss
count passed through unused), `collectDiagnostics` (generated
code-or-signal line + sanitized stderr + integrity failures through
`present.Diagnostic`), the modal `overlay` (`up`/`down` scroll,
`q`/`Esc` dismissal — `Esc` exits 2 only in the fatal no-results case
where nothing underlies it — `ctrl+c` → 130, other keys ignored), and
the centred `theme.Overlay` compositing. The fixed status is decided
once at completion — 2 fatal, 0 usable results, 1 intact empty — and
overridden only by `ctrl+c`. Created
[error-overlay-and-fatal-outcomes](error-overlay-and-fatal-outcomes.md);
updated
[searchindex-records-and-stops](searchindex-records-and-stops.md)
(lifecycle validation section), [no-results-and-binary-exclusion](
no-results-and-binary-exclusion.md) (exclusion precedence, the
`decideOutcome` branch, warning overlay),
[safe-presentation](safe-presentation.md) (the `error overlay` sink
row, `wantDiag` expectations), [source-code](source-code.md),
[unit-tests](unit-tests.md), and the index. Sources:
`Notes/issues/009-error-overlay-and-fatal-outcomes.md`,
`Notes/tasks/009-error-overlay-and-fatal-outcomes.md`,
`Notes/PRD-vrg.md` (Result index, records, and stream integrity;
Outcome and exit-status contract; Colours, overlays, and key
precedence), `internal/searchindex/index.go`,
`internal/searchindex/lifecycle_test.go`, `internal/app/overlay.go`,
`internal/app/app.go`, `internal/app/search.go`,
`internal/app/outcome_test.go`, `internal/app/overlay_test.go`,
`internal/app/sinksafety_test.go`, `cmd/vrg/pty_test.go`,
`cmd/vrg/main_test.go`.

## [2026-09-23] ingest | Issue #10 record robustness

Ingested the completed Issue #10 implementation. `internal/searchindex`
gained deterministic record dispositions: `Feed` enforces
`maxRecordPayload` (64 MiB) on each split line so oversized records are
discarded through the newline and the stream resynchronizes, counts
schema-failing records into `Malformed()` without conflating them with
integrity failures, and gives the unterminated oversized final record
its triple disposition (oversized + malformed + `unterminated trailing
record`); `Add` tallies `KindUnknown` into `Unknown()` — never a
substitute for `summary`, counted plus separately flagged after
`summary`. `record.go` gained `recoverRecordPath`/`recoverDataPath`,
token-streamed `data.path` recovery over an oversized record's consumed
prefix so the diagnostic names the file (`oversized record skipped for
<path>`, via `present.Path`) whenever the path decoded before the
limit, else the anonymous tally only. `RecordDiagnostics()` assembles
the skip lines — per-path oversized lines, then the malformed/
oversized/unknown tallies. `internal/app`: `decideOutcome` now consumes
the record-loss count (malformed + oversized) and the record-skip
diagnostics — fatal when record loss left zero usable results assessed
after all filtering (binary exclusion included), a warning overlay over
browse at 0 otherwise, unknown-only warnings still no-results at 1.
Created [record-robustness](record-robustness.md); updated
[searchindex-records-and-stops](searchindex-records-and-stops.md),
[error-overlay-and-fatal-outcomes](error-overlay-and-fatal-outcomes.md)
(decideOutcome inputs, four new outcome rows, diagnostics order),
[source-code](source-code.md), [unit-tests](unit-tests.md), and the
index. Sources:
`Notes/issues/010-record-robustness-malformed-oversized-unknown.md`,
`Notes/tasks/010-record-robustness-malformed-oversized-unknown.md`,
`Notes/PRD-vrg.md` (Result index, records, and stream integrity;
Outcome and exit-status contract; Resources and responsiveness),
`internal/searchindex/index.go`, `internal/searchindex/record.go`,
`internal/searchindex/disposition_test.go`,
`internal/searchindex/oversized_test.go`, `internal/app/overlay.go`,
`internal/app/app.go`, `internal/app/outcome_test.go`.

## [2026-09-23] ingest | Issue #11 stderr replay of collected diagnostics

Ingested the completed Issue #11 implementation. `internal/app` gained
the session diagnostic collection: `Model.diags` holds every diagnostic
the model processed, independent of display; `collect`'s stderr drain
forwards each child stderr line to the model as a `diagMsg` over an
unbuffered channel (every send pairs with the model's `awaitEvent`, so
the completion can never overtake a diagnostic; cancellation stops
delivery while drainage continues); `Update`'s `diagMsg` branch and the
`searchDoneMsg` branch append through `collect`, which also writes the
`Config.DiagAck` acknowledgement — processing the message is the
collection boundary, so in-flight work is never waited for or replayed.
`overlay.go` split `collectDiagnostics` (the display list, stderr
included) from `completionDiagnostics` (the collection subset, stderr
excluded to preserve exactly-once). `ReplayTo` emits the collection in
order on the common post-restoration writer; `runSearch` calls it on
every controlled exit and routes controlled failures through
`CollectDiagnostic` first, retiring the separate direct write.
`cmd/vrg` gained the `VRG_TEST_DIAG_ACK_FILE` seam. Created
[stderr-replay](stderr-replay.md); updated
[cancellation-and-cleanup](cancellation-and-cleanup.md) (unified
failure write, new seam),
[error-overlay-and-fatal-outcomes](error-overlay-and-fatal-outcomes.md)
(display/collection split), [safe-presentation](safe-presentation.md)
(the delivered `stderr replay` sink row), [source-code](source-code.md),
[unit-tests](unit-tests.md), and the index. New test files:
`internal/app/replay_test.go`, `cmd/vrg/pty_replay_test.go`; the
lifecycle and dash-file boundary fixtures gained `f.txt` and the
dual-pipe boundary test now expects the replayed flood. Sources:
`Notes/issues/011-stderr-replay-of-collected-diagnostics.md`,
`Notes/tasks/011-stderr-replay-of-collected-diagnostics.md`,
`Notes/PRD-vrg.md` (Colours, overlays, and key precedence; Outcome and
exit-status contract), `internal/app/app.go`, `internal/app/search.go`,
`internal/app/overlay.go`, `internal/app/replay_test.go`,
`internal/app/model_test.go`, `internal/app/subprocess_test.go`,
`internal/app/sinksafety_test.go`, `cmd/vrg/main.go`,
`cmd/vrg/hooks.go`, `cmd/vrg/main_test.go`, `cmd/vrg/pty_replay_test.go`.

## [2026-09-23] ingest | Issue #12 manual vertical scrolling and per-file viewport

Ingested the completed Issue #12 implementation. `internal/viewport`
grew from the Issue #5 seam into the real reading position: `Row`
(source line + cells + spans) and the `Rows` prepared-row provider
interface; `Viewport` now holds the prepared rows and the clamped top
rendered row — `[0, max(0, count − height)]`, lossy on EOF per the PRD
— with `Up`/`Down` (one rendered row), `HalfUp`/`HalfDown`
(`max(1, floor(h/2))`), `PageUp`/`PageDown` (the content height),
`SetTop` for saved-state restore, and `Visible`, which queries the
provider once per shown row only. `internal/app` gained the `rows`
(prepared `viewport.Rows` per path, built on load completion and
reinstalled by `relayout`) and `saved` (per-path top row) maps, the
`up`/`down`/`u`/`d`/`pgup`/`pgdown` key case routed through
`Model.scroll` — a no-op outside browse and on `Loading…`/`(unreadable)`
placeholders — and the saved-state `SetTop` restore when a load
completes for the current path; `bufferRows` adapts `*filebuffer.Buffer`
(unwrapped row i = source line i) and `contentRow` renders from the
`Visible()` slice. Created
[viewport-scrolling](viewport-scrolling.md); updated
[browse-tracer](browse-tracer.md) (the viewport seam description and
file list), [source-code](source-code.md),
[unit-tests](unit-tests.md), and the index. New test files:
`internal/viewport/viewport_test.go` (scroll units incl. odd heights,
BOF/EOF clamps, short/equal/long content, empty-content inertness,
counting-fake visible-range queries, resize and SetRows clamps) and
`internal/app/scroll_test.go` (key-level scroll units over content
height = panel minus filename row, EOF/BOF clamps, short-file and
placeholder no-ops, per-file saved state saved and restored, the
render-cost counting-fake guard, the `bufferRows` adapter). Sources:
`Notes/issues/012-manual-vertical-scrolling-and-per-file-viewport.md`,
`Notes/tasks/012-manual-vertical-scrolling-and-per-file-viewport.md`,
`Notes/PRD-vrg.md` (Navigation, viewport, and logical anchors; Module
Design → Viewport), `internal/viewport/viewport.go`,
`internal/app/app.go`, `internal/app/browse.go`,
`internal/viewport/viewport_test.go`, `internal/app/scroll_test.go`.

## [2026-09-23] ingest | Issue #13 circular matched-line cursor and navigation wiring

Ingested the completed Issue #13 implementation. `internal/searchindex`
gained `cursor.go`: the `Index` owns the single global matched-line
cursor (`cursor` field — an index into the sorted `order`, zero value
selecting the first stop at startup, clamped in `Prepare`), `Current()`
returns the selected stop (false when empty), and `Next()`/`Prev()`
step circularly returning a `Step` — the destination `Stop` plus
`Moved` (false for the strict zero- and one-stop no-ops), `FileChanged`
(raw-path comparison of departed and destination stops), and `Wrapped`
(the step passed an index end); `export` now shares the `stop`→`Stop`
projection between `Stops()` and the cursor methods. `internal/app`
dropped its own `cursor` field: `currentStop`/`currentPath` read
`Index.Current()` so the panel, filename rule, current-line
`CurrentMatch` underline, and file-list underline all derive from the
cursor; `Update` routes `n`/`p` to `Model.navigate` in `phaseBrowse`
(strict no-op when `Moved` is false, styling-only same-file steps — the
viewport stays put pending Issue #14's reveal — and on `FileChanged`
the departing file's top row is saved, the destination's prepared rows
install at its saved-or-top state via `relayout` + `SetTop`, and
`ensureLoad` requests the load when uncached). The file list stays
passive — no direct selection route. Created
[match-navigation](match-navigation.md); updated
[searchindex-records-and-stops](searchindex-records-and-stops.md)
(cursor section), [viewport-scrolling](viewport-scrolling.md) (the
saved-state restore now driven by `n`/`p` crossings),
[browse-tracer](browse-tracer.md) (passive list, cursor ownership),
[theme-and-colour-toggle](theme-and-colour-toggle.md) (the
current-line match is the cursor's stop),
[source-code](source-code.md), [unit-tests](unit-tests.md), and the
index. New test files: `internal/searchindex/cursor_test.go` (startup
selection, next/prev step tables with `FileChanged`/`Wrapped` flags,
one-stop and empty strict no-ops, submatches sharing one stop) and
`internal/app/nav_test.go` (startup selection in path order, same-file
underline move without scrolling, cross-file panel switch with load
request and `Loading…`, wrap at both ends, one-stop `n`/`p` no-op,
manual-scroll independence, departing-viewport save/restore, passive
file list); `scroll_test.go`'s saved-state test now drives the real `n`
mechanism. Sources:
`Notes/issues/013-match-navigation-n-p-circular-cursor.md`,
`Notes/tasks/013-match-navigation-n-p-circular-cursor.md`,
`Notes/PRD-vrg.md` (Navigation, viewport, and logical anchors; Module
Design → SearchIndex), `internal/searchindex/cursor.go`,
`internal/searchindex/index.go`,
`internal/searchindex/cursor_test.go`, `internal/app/app.go`,
`internal/app/browse.go`, `internal/app/nav_test.go`,
`internal/app/scroll_test.go`.

## [2026-09-23] ingest | Issue #14 vertical destination reveal

Ingested the completed Issue #14 implementation: `internal/viewport`
gained `Target` (the zero-based `(line, cell)` display location — the
destination line's first submatch start, the marker cell for zero-width
matches), `Rows.RowOf(Target)` so the prepared-row provider resolves
the target to its rendered row (the Issue #16 wrap seam;
`bufferRows.RowOf` returns `Target.Line` unwrapped), and
`Viewport.Reveal` — inert on empty content, no-scroll on a visible
target row, `row − floor(height/3)` on a hidden one, both directions
clamped through the existing `[0, maxTop]` clamp so BOF/EOF content
wins over exact placement, reporting whether the top moved.
`internal/app` gained `Model.reveal` (the smallest span `Start` of the
destination line as the target cell; `saved[path]` replaced only when
`Reveal` reports a move) and wired the two triggers: `navigate` calls
it after every actual cursor transition — a file crossing installs the
destination's saved-or-top start first — and the `loadDoneMsg` path
runs `SetTop(saved)` + `reveal` for the current path, covering both
revisits and the startup-after-load reveal (and reading the live
cursor, so a load landing after further navigation reveals the latest
stop, Issue #28's seam). Horizontal reveal stays Issue #19's. Created
[destination-reveal](destination-reveal.md); updated
[viewport-scrolling](viewport-scrolling.md) (reveal section, inert
placeholder, saved-state replacement rules),
[match-navigation](match-navigation.md) (same-file steps now reveal;
stale "Issue #14's" notes),
[browse-tracer](browse-tracer.md) (reveal on entry),
[source-code](source-code.md), [unit-tests](unit-tests.md), and the
index. New test files: `internal/viewport/reveal_test.go` (`mappingRows`
programmable-`RowOf` fake, visible-target no-scroll, one-third
placement both directions, BOF/EOF precedence, out-of-range clamp,
inert empty content, the saved/first-visit sequence) and
`internal/app/reveal_test.go` (`fileWithStops` fixture; startup reveal
including the gate-held completion, `n`/`p` placement and saved-state
replacement, on-screen `n` no-scroll, saved-viewport revisit, reveal
overriding a hiding saved top, first-visit top-then-reveal);
`nav_test.go`/`scroll_test.go` expectations updated for the reveal.
Sources:
`Notes/issues/014-vertical-destination-reveal.md`,
`Notes/tasks/014-vertical-destination-reveal.md`,
`Notes/PRD-vrg.md` (Navigation, viewport, and logical anchors; Testing
Decisions → Viewport), `internal/viewport/viewport.go`,
`internal/viewport/reveal_test.go`, `internal/app/app.go`,
`internal/app/browse.go`, `internal/app/reveal_test.go`,
`internal/app/nav_test.go`, `internal/app/scroll_test.go`.

## [2026-09-23] ingest | Issue #15 file-change pop-up

Ingested the completed Issue #15 implementation: `internal/app` gained
`popup.go` — `popupExpiredMsg` (the expiry carrying its instance ID),
`popupTick` (the one-second `tea.Tick` behind the `popupTimer` seam),
`openPopup` (fresh instance per pop-up, raw destination path stored),
and `renderPopup` (centred `theme.Overlay` box over a
`present.Path`-escaped, left-truncated-with-`…` path — geometry
recomputed per render, so resize recentres and re-truncates the same
instance without restarting its timer). `navigate` opens the pop-up in
the `FileChanged` branch — at selection, before the destination loads —
and `tea.Batch`es the expiry with the load command; the
`popupExpiredMsg` case dismisses only on an instance-ID match (stale
timers inert); the `KeyPressMsg` branch clears the pop-up before normal
routing so any key dismisses and still performs its action;
`openOverlay` (new single overlay-opening route — appends when already
open) cancels the pop-up permanently, and `View` composites pop-up
under overlay. `composite` was factored out of `renderOverlay` for
shared use. A `loadDoneMsg` failure for the current path now opens the
diagnostics overlay (the first slice of Issue #26's read-failure
rules), which is also the injected cancellation trigger.
`cmd/vrg/pty_replay_test.go`'s embedded-filename replay test gained a
second `q` since the current-file load failure now opens the overlay.
Created [file-change-popup](file-change-popup.md); updated
[match-navigation](match-navigation.md) (crossings open the pop-up),
[error-overlay-and-fatal-outcomes](error-overlay-and-fatal-outcomes.md)
(`openOverlay`, `composite`, current-file failure route, pop-up
cancellation), [safe-presentation](safe-presentation.md) (the
`file-change pop-up` sink row, delivered), [source-code](source-code.md),
[unit-tests](unit-tests.md), and the index. New test file:
`internal/app/popup_test.go` (`instantPopupTimer` injected-seam helper;
selection-time centred opening over `Loading…` with the batched
load + expiry, load completion never restarting, fresh-instance minting
with stale-expiry rejection, dismissal keys performing their actions,
resize recentring without restart, left-truncation at both sizes,
error-overlay cancellation with no return); `sinksafety_test.go` gained
the `file-change pop-up` row via `renderPopupSink`; `browse_test.go`'s
`browseModel` installs a nil-command `popupTimer` keeping navigation
commands synchronous. Sources:
`Notes/issues/015-file-change-popup.md`,
`Notes/tasks/015-file-change-popup.md`,
`Notes/PRD-vrg.md` (Colours, overlays, and key precedence; File loading,
cache, reload, and selection consistency; Navigation, viewport, and
logical anchors), `internal/app/popup.go`, `internal/app/app.go`,
`internal/app/browse.go`, `internal/app/overlay.go`,
`internal/app/popup_test.go`, `internal/app/sinksafety_test.go`,
`internal/app/browse_test.go`, `cmd/vrg/pty_replay_test.go`.

## [2026-09-23] ingest | Issue #16 wrap mode and `w` toggle

Ingested the completed Issue #16 implementation: wrapping is on
initially and `w` toggles run-off-edge mode in browse. `present.Cell`
gained `Lead` — a cluster's first cell, the only legal wrap boundary —
alongside `Cont` (a multi-cell unit's trailing cells), making
`internal/present` the single grapheme-segmentation and cell-width
policy; `*filebuffer.Buffer` satisfies the new `viewport.Source`
(`LineCount`/`Cells`/`Spans`) so `internal/viewport` never re-derives
segmentation. Tabs now expand structurally with space cells to the
next multiple of eight source-display columns as one cluster,
replacing Issue #5's provisional `→` and asserting the deferred cell
positions. New `internal/viewport/rows.go`: `Key{Path, Rev, Width,
Wrap}` (the staleness contract for Issue #17's async preparation) and
`Prepare`/`Model` — wrap mode packs clusters greedily into text-width
rows, moving an unfit cluster whole (blank remainder) and splitting an
over-wide one as a last resort with the clipped lead blanked; an
end-of-line marker past a full final row occupies its own continuation
row. `Row.Cont` marks continuations, painted behind a blank gutter.
The reserved right-indicator column is zero in wrap mode and one in
run-off-edge (Issue #20 populates it), so text width is panel minus
gutter minus reserved. `relayout` rebuilds stale models per
path/revision/layout; `Reveal` resolves wrapped targets through
`Model.RowOf` (boundary positions belong to the next row). Created
[wrap-mode](wrap-mode.md); updated
[viewport-scrolling](viewport-scrolling.md) (the `bufferRows` adapter
is gone — `viewport.Model` is the provider),
[destination-reveal](destination-reveal.md) (`RowOf` now wrap-aware),
[safe-presentation](safe-presentation.md) (tab expansion, `Lead`/`Cont`),
[browse-tracer](browse-tracer.md) (tab contract, file list),
[source-code](source-code.md), [unit-tests](unit-tests.md), and the
index. New test files: `internal/viewport/wrap_test.go` (wrap table,
continuation/run-off-edge models, key semantics, per-row span
translation, end-of-line marker rows, wrapped-target reveal at
`floor(h/3)`, the `lineSource` counting-fake render-cost guard),
`internal/filebuffer/cluster_test.go` (`Lead`/`Cont` boundaries, tab
stops), `internal/app/wrap_test.go` (default wrap with blank
continuation gutters, `w` toggle changing the reserved column and
re-wrapping, `n`/`p` reveal inside a screen-tall wrapped line);
`line_test.go`'s provisional tab test became `TestLineTabStops` and
`scroll_test.go`'s adapter test became `TestBufferPreparesAsRowSource`.
Sources: `Notes/issues/016-wrap-mode-and-toggle.md`,
`Notes/tasks/016-wrap-mode-and-toggle.md`, `Notes/PRD-vrg.md` (Text,
graphemes, and safe presentation; Layout and indicators; Navigation,
viewport, and logical anchors; Wrapping, indicators, and text
display), `internal/present/line.go`, `internal/viewport/rows.go`,
`internal/viewport/viewport.go`, `internal/app/app.go`,
`internal/app/browse.go`, `internal/viewport/wrap_test.go`,
`internal/filebuffer/cluster_test.go`, `internal/app/wrap_test.go`,
`internal/present/line_test.go`, `internal/app/scroll_test.go`.

## [2026-09-23] ingest | Issue #17 logical anchor and off-UI layout preparation

Ingested the completed Issue #17 implementation: the reading position
is now a width-independent logical anchor — `Viewport.anchor`, a
`(source line, display-column offset)` `Target` — and `Resize`/
`SetRows`/`SetAnchor` resolve the effective top through
`Rows.RowOf(anchor)` instead of keeping a row ordinal. `Row` gained
`Start` (the display-column offset where a rendered row begins), so a
wrapped continuation row maps back to logical coordinates; a scroll
that moves replaces the anchor with the new top row's location while a
clamped no-move keeps it, a `Reveal` that moves replaces it while a
no-scroll reveal preserves the retained column, and the EOF clamp is
deliberately lossy — when it pulls the top off the anchor's row the
anchor rewrites to the clamped row, so a later shrink does not restore
the pre-clamp position. `Model.saved` became `map[string]viewport.
Target`, so per-file state survives rewraps while a file is away, and
resize preserves the cursor selection since only the derived top
moves. Preparation moved off the update path: `layoutCmd` workers run
`viewport.Prepare` and deliver `layoutDoneMsg{key, rows}`; a
completion installs only while its `(path, revision, text width, wrap
mode)` key equals the model's current `layoutKey`, with `reqKey`
deduplicating in-flight requests — out-of-order and superseded
completions are discarded without touching the panel, anchor, saved
state, or the pending reveal intent. `reveal()` now pends
(`pendingReveal`) when no matching layout is installed and commits for
the newest selected stop via `commitReveal`; `navigate` installs a
destination's cached rows only while their key matches (the fast path
commits immediately, stale-keyed caches get a fresh request). A
`listEntry` provider seam plus cached `listW`/`textW`/`fileIdx` keep
frame rendering to the visible rows and visible list window — the
`relayout`/`prepW`/`prepWrap` machinery is gone. Created
[logical-anchor-and-layout](logical-anchor-and-layout.md); updated
[viewport-scrolling](viewport-scrolling.md) (anchor semantics, anchor-
keyed saved state, `Row.Start`),
[destination-reveal](destination-reveal.md) (pending intent, newest-
stop commit),
[wrap-mode](wrap-mode.md) (async `w` toggle, keyed install guard),
[match-navigation](match-navigation.md) (the anchor handoff and
carried intent on crossings),
[browse-tracer](browse-tracer.md) (layout workers, visible-window
list),
[cancellation-and-cleanup](cancellation-and-cleanup.md) (`ctrl+c`/`q`
actionable while a layout worker is held), [source-code](source-code.md),
[unit-tests](unit-tests.md), and the index. New test files:
`internal/viewport/anchor_test.go` (rewrap round trips, wrap-toggle
column retention, scroll/reveal anchor replacement, lossy EOF clamp),
`internal/app/anchor_test.go` (cursor preservation and anchor-text-at-
top through resize), `internal/app/layout_test.go` (off-path resize
requests, held-worker input actionability, out-of-order/superseded
discards, departed-file caching, pending-intent preservation,
stale-layout and matching-layout navigation, list render-cost guard).
Sources: `Notes/issues/017-logical-anchor-through-rewrap-and-resize.md`,
`Notes/tasks/017-logical-anchor-through-rewrap-and-resize.md`,
`Notes/PRD-vrg.md` (Navigation, viewport, and logical anchors;
Resources and responsiveness), `internal/viewport/viewport.go`,
`internal/viewport/rows.go`, `internal/app/app.go`,
`internal/app/browse.go`, `internal/viewport/anchor_test.go`,
`internal/app/anchor_test.go`, `internal/app/layout_test.go`,
`internal/app/model_test.go`, `internal/app/scroll_test.go`.
## [2026-09-23] ingest | Issue #18 horizontal panning in run-off-edge mode

Ingested the completed Issue #18 implementation: `Viewport` gained
`off`, a horizontal pan offset in display cells, driven by the six pan
keys — `,`/`.` one column, `<`/`>` ten, `[`/`]`
`max(1, floor(text width / 2))` — clamped to `[0, max(0, S)]` under
the visible-lines extent policy, where S is the paintable boundary of
the widest *currently rendered* source line: the largest cell index
whose grapheme cluster also fits the text width, so the maximum always
leaves one whole cluster painted and never half a glyph (a widest line
with no fitting cluster reports 0 — the documented exception renders
clipping blanks). `clampOff` runs inside `clamp` and `resolve`, so
scrolling, reveals, resizes (including list hide/show and gutter
growth reaching the viewport as width changes), row-model swaps, and
wrap-toggle re-entry all re-clamp, and every pan recomputes the
maximum from the visible rows — the loss is permanent, never restored
when a wider line returns. `Rows` gained `Wrap()`, gating panning to
run-off-edge models: under wrap the offset is dormant and retained,
re-clamped on re-entry. `navigate`'s `FileChanged` branch and the
current-path `loadDoneMsg` reset the offset to zero before the
pending/Issue #19 reveal. `Visible()` clips each run-off-edge row to
`[off, off+width)` via `clipRow` — out-of-window cells dropped, spans
translated into window cells (markers following their cell), and a
cluster split by either clip edge painting its in-window cells blank,
the same policy as wrap-mode's split-cluster blanking. Created
[horizontal-panning](horizontal-panning.md); updated
[viewport-scrolling](viewport-scrolling.md) (`Rows.Wrap`, clipped
`Visible`), [wrap-mode](wrap-mode.md) (panning lands, dormant offset,
re-entry clamp), [destination-reveal](destination-reveal.md) (the
offset reset joins the entry sequence),
[match-navigation](match-navigation.md) (file-crossing reset),
[logical-anchor-and-layout](logical-anchor-and-layout.md) (re-clamp on
every resolve/clamp), [browse-tracer](browse-tracer.md) (clipped rows
feed `contentRow`), [source-code](source-code.md),
[unit-tests](unit-tests.md), and the index. New test files:
`internal/viewport/pan_test.go` (pan units, extent policy,
retention/reset, paintable boundary, re-clamp triggers, split-cluster
blanking, span translation, hidden-left geometry, visible-rows-only
cost guard) and `internal/app/pan_test.go` (key wiring, wrap-mode
no-op, `w w` retention, `n`/`p` reset, half-clipped CJK blank,
maximum leaves the final cluster whole).
Sources: `Notes/issues/018-horizontal-panning.md`,
`Notes/tasks/018-horizontal-panning.md`,
`Notes/PRD-vrg.md` (Navigation, viewport, and logical anchors; Layout
and indicators), `internal/viewport/viewport.go`,
`internal/viewport/rows.go`, `internal/app/app.go`,
`internal/app/browse.go`, `internal/viewport/pan_test.go`,
`internal/app/pan_test.go`.

## [2026-09-24] ingest | Issue #19 minimal horizontal reveal

Ingested the completed Issue #19 implementation: `Viewport.Reveal`
gained a horizontal half via `revealCell`, which runs after vertical
placement (including its visible-set re-clamp of `off`) in run-off-edge
models only. Visibility means *painted cells* — a target start cell
geometrically inside `[off, off+width)` but blanked by a clip edge
counts as hidden and is revealed. The target unit is its grapheme
cluster (`Lead`/`Cont` scan from the start cell, forward through
continuation cells) or a one-cell marker for a zero-width span
(`markedAt`); the offset moves minimally — `s` when hidden left,
`s + cw − width` when hidden right or clipped — or stays put when the
unit already paints. A match wider than the text area reveals its
start cell alone; a cluster wider than the whole text area takes
`off = s` (the closest achievable position, deliberately beyond the
paintable-boundary maximum `revealCell` bypasses), renders in-window
clipping blanks, and counts as geometrically revealed so repeated
navigation cannot loop — Issue #20's indicators still treat it as not
visible. The triggers ride the existing reveal seam: startup (the
pending intent commits on the startup layout's install), every `n`/`p`
including same-file steps, and file crossings where the Issue #18
`SetOffset(0)` reset runs first. Created
[minimal-horizontal-reveal](minimal-horizontal-reveal.md); updated
[destination-reveal](destination-reveal.md) (the horizontal half on
the same `Reveal` seam), [horizontal-panning](horizontal-panning.md)
(reveal operates on the clamped offset; its writes bypass the
boundary), [source-code](source-code.md) (`revealCell`/`markedAt`),
[unit-tests](unit-tests.md), and the index. New test files:
`internal/viewport/hreveal_test.go` (right/left reveal arithmetic,
wide-cluster whole painting, painted-target no-ops, clipped-blank
hidden, oversized-span start-cell rule, unpaintable-cluster fallback
with no loop, marker cells, both-axes move, wrap-mode dormancy) and
`internal/app/hreveal_test.go` (same-file `n`/`p` minimal scrolls,
visible-target no-op, startup reveal, file-change reset-then-reveal,
CJK two-cell painting). `internal/viewport/pan_test.go`'s
`moving_reveal` subtest retargeted to cell 9 so it stays painted at
the clamped offset.
Sources: `Notes/issues/019-minimal-horizontal-reveal.md`,
`Notes/tasks/019-minimal-horizontal-reveal.md`,
`Notes/PRD-vrg.md` (Navigation, viewport, and logical anchors; Layout
and indicators), `internal/viewport/viewport.go`,
`internal/app/browse.go`, `internal/viewport/hreveal_test.go`,
`internal/app/hreveal_test.go`, `internal/viewport/pan_test.go`.

## [2026-09-24] ingest | Issue #20 hidden-content indicators

Ingested the completed Issue #20 implementation: run-off-edge frames
now signpost clipped content. `viewport.Row` gained three flags —
`HiddenLeft` (any of the line's text hidden left), `MatchHiddenLeft`
(a match or marker entirely hidden left), and `MatchHiddenRight`
(entirely hidden right) — computed by `hiddenMarks` inside `clipRow`,
which sees the unclipped line and the same split-cluster blanked
regions `[off, lb)`/`[rb, off+w)` the clip produces. Visibility is the
Issue #19 painted-cell rule: a marker is one painted cell wherever it
sits, a partially painted match counts as visible for that side, a
match on a clip-edge-split cluster's blanked cells counts as entirely
hidden on that side, and an entirely hidden match attributes to the
side its hidden cells stand on (both only when a blank-filling cluster
straddles both edges). `contentRow` renders them: the first trailing
gutter space of every visible source line is blank, an inverse `_`, or
an inverse `*` upgrade, and the reserved rightmost column is blank
except an inverse `*` on the current matched line's visible row — the
indicator is appended after text padded to the text width, so it never
overwrites a match ending on the last text cell, and it vanishes when
the current line scrolls off-screen. Wrap mode draws neither
indicators nor the column. Created
[hidden-content-indicators](hidden-content-indicators.md); updated
[source-code](source-code.md) (`Row` flags + `hiddenMarks`,
`contentRow` indicator composition), [unit-tests](unit-tests.md),
[horizontal-panning](horizontal-panning.md) and
[wrap-mode](wrap-mode.md) and
[minimal-horizontal-reveal](minimal-horizontal-reveal.md) (Issue #20
links), and the index. New test files:
`internal/viewport/indicators_test.go` (per-side flags, marker cells,
partial visibility, split-cluster blanks both directions,
unpaintable-cluster hidden-right, wrap-mode absence) and
`internal/app/indicators_test.go` (`_`/`*`/blank gutters per line
including empty and uniform lines, inverse styling, current-line-only
right `*` with off-screen absence, both stars together, no-overwrite
last-cell case, split-glyph stars, wrap-mode absence).
Sources: `Notes/issues/020-hidden-content-indicators.md`,
`Notes/tasks/020-hidden-content-indicators.md`,
`Notes/PRD-vrg.md` (Layout and indicators),
`internal/viewport/viewport.go`, `internal/app/browse.go`,
`internal/viewport/indicators_test.go`,
`internal/app/indicators_test.go`.
## [2026-09-24] ingest | Issue #21 grapheme-cluster highlight expansion and wide-glyph safety

Ingested the completed Issue #21 implementation: FileBuffer now hands
down cluster-expanded highlight spans and wrap/clip filler cells are
never painted as match cells. `filebuffer.Load` runs each validated
submatch's mapped span through `clusterSpan`, which walks a nonempty
span's endpoints outward to the `Lead`-marked cluster boundaries —
covering the start-inside, end-inside, and strictly interior cases the
byte→cell map alone leaves mid-cluster: zero-width clusters joined
into a host cell (a combining mark on a `^A` escape's trailing cell,
on a tab expansion cell, or on a wide glyph's trailing cell) expand to
the whole host cluster, a combining-only match highlights the whole
base cluster (the visible `é` glyph for decomposed `e` + `́`),
interior bytes of a wide pair or an emoji ZWJ sequence cover both
cells, and a standalone combining cluster keeps the provisional
`Lead` cell `emit` gives it at line start on a `◌` (U+25CC)
dotted-circle base — the visible fallback that keeps its highlight
one cell, never zero. `present.Cell` gained the
`Blank` mark: `Model.Row` marks the lead cell it substitutes when a
wrap row ends inside a split cluster and `clipRow` marks the in-window
cells it blanks for an edge-split cluster; `renderCells` paints a
`Blank` (or clip-edge-split) cell's space unstyled even under a
covering span while a marker still paints its own position. The
expanded span is the sole span source: `reveal` takes its target cell
from it (a mid-cluster match reveals from the cluster start) and
`hiddenMarks` classifies it (the cluster-start-hidden match counts
entirely hidden left) — neither re-derives the recorded bytes.
Created
[grapheme-highlight-expansion](grapheme-highlight-expansion.md);
updated [source-code](source-code.md) (`Cell.Blank`, `clusterSpan` in
`Load`, blank marking in `Row`/`clipRow`, `renderCells` blank
safety), [unit-tests](unit-tests.md) (new `expand_test.go`,
`blanks_test.go`, `cluster_test.go`, and the updated
indicator/hreveal entries), [safe-presentation](safe-presentation.md)
(`Blank` and the provisional-cell fallback),
[hidden-content-indicators](hidden-content-indicators.md) (expanded
spans classified), [minimal-horizontal-reveal](minimal-horizontal-reveal.md),
[wrap-mode](wrap-mode.md), and
[horizontal-panning](horizontal-panning.md) (Issue #21 links), and the
index. New test files: `internal/filebuffer/expand_test.go` (the
`clusterSpan` boundary table and the `Load`-level expansion table),
`internal/viewport/blanks_test.go` (`Blank` marks on wrap and clip
filler), `internal/app/cluster_test.go` (`renderCells` blank safety);
updated `internal/app/indicators_test.go` (mid-cluster match counting
hidden-left, plus the `matchRecJSON` helper for records carrying raw
control bytes) and `internal/app/hreveal_test.go` (a mid-cluster match
revealing and painting the whole cluster).
Sources: `Notes/issues/021-grapheme-cluster-highlight-expansion.md`,
`Notes/tasks/021-grapheme-cluster-highlight-expansion.md`,
`Notes/PRD-vrg.md` (Text, graphemes, and safe presentation; Testing
Decisions → FileBuffer), `internal/filebuffer/filebuffer.go`,
`internal/present/line.go`, `internal/viewport/rows.go`,
`internal/viewport/viewport.go`, `internal/app/browse.go`,
`internal/filebuffer/expand_test.go`, `internal/viewport/blanks_test.go`,
`internal/app/cluster_test.go`, `internal/app/indicators_test.go`,
`internal/app/hreveal_test.go`.

## [2026-09-24] ingest | Issue #22 line terminators, empty file, and UTF-8 BOM

Ingested the completed Issue #22 implementation: FileBuffer's
structural line handling with raw-file and rg-line coordinate
separation. LF and CRLF terminate lines without being displayed while
every line's original bytes — terminators included — stay in the raw
view for byte-coordinate mapping and Issue #29's later validation; a
missing final newline still yields the final line, a trailing newline
invents no phantom line, and an empty file has zero source lines
behind the minimum one-digit gutter slot (three cells). A standalone
CR is not a terminator and escapes as `^M`. Removed terminator bytes
and zero-width positions map to the display end-of-line position
(byte 4 of `hit\r\n` → column 3), a terminator-only match yields an
end-of-line marker position, and a span covering text plus terminator
highlights the visible text only. For a leading UTF-8 BOM, `Load`
detects `EF BB BF` at file start, escapes line one through the new
`present.LineOfBOM` — `lineOf(raw, hidden)` keeps the BOM's three
bytes in `Raw` while painting nothing and mapping them to the
line-start position — and shifts that line's rg submatch offsets by
three into the raw view before validating and mapping, so rg offset 0
names raw byte 3; later lines and non-leading U+FEFF are unaffected.
Created [line-terminators-and-bom](line-terminators-and-bom.md);
updated [source-code](source-code.md) (`lineOf` hidden prefix /
`LineOfBOM`, `Load`'s BOM detection and offset shift),
[unit-tests](unit-tests.md) (new `lines_test.go` catalog),
[safe-presentation](safe-presentation.md) (`LineOfBOM`), and the
index. New test file: `internal/filebuffer/lines_test.go` (the
structural-line and BOM contract tables).
Sources: `Notes/issues/022-line-terminators-final-line-empty-file-utf8-bom.md`,
`Notes/tasks/022-line-terminators-final-line-empty-file-utf8-bom.md`,
`Notes/PRD-vrg.md` (Text, graphemes, and safe presentation; Encodings
and stale-content validation; Testing Decisions → FileBuffer),
`internal/filebuffer/filebuffer.go`, `internal/present/line.go`,
`internal/filebuffer/lines_test.go`.
## [2026-09-24] ingest | Issue #23 zero-width match markers

Ingested the completed Issue #23 implementation: the one-cell
inverse-video marker for zero-width submatches. Most marker machinery
predated the issue — `present.Line.Span` has emitted `Start == End`
marker positions since Issue #22, `filebuffer.Load` validates empty
submatches against raw bytes and `clusterSpan` passes the positions
through, `wrapLine` gives an end-of-line marker an overflow row after
a completely full wrap row, `clipRow` translates markers like cells,
`hiddenMarks` counts a marker as painted wherever it sits,
`revealCell`/`markedAt` reveal marker targets as one-cell units, and
`renderCells` paints the inverse space in place or appended at line
end under `Match`/`CurrentMatch`. The remaining gap was
`lineExtent`, which now counts markers as one-cell units: an
end-of-line marker extends the content extent by one cell (a
marker-only line has extent 1, maximum offset 0) and every marker is
a paintable boundary at its position, so the Issue #18 maximum can
land on it even past an unfittable final cluster. The terminator-only
`$` marker on `hit\r\n` at display column 3 is an ordinary marker in
every rule — no special case. Created
[zero-width-match-markers](zero-width-match-markers.md); updated
[source-code](source-code.md) (`lineExtent`'s marker counting and the
`Spans` marker channel), [unit-tests](unit-tests.md) (both new
`marker_test.go` files), [horizontal-panning](horizontal-panning.md)
(marker extents in the paintable-boundary maximum),
[line-terminators-and-bom](line-terminators-and-bom.md),
[hidden-content-indicators](hidden-content-indicators.md),
[minimal-horizontal-reveal](minimal-horizontal-reveal.md),
[destination-reveal](destination-reveal.md), and the index.
New test files: `internal/filebuffer/marker_test.go`,
`internal/viewport/marker_test.go`.
Sources: `Notes/issues/023-zero-width-match-markers.md`,
`Notes/tasks/023-zero-width-match-markers.md`,
`Notes/PRD-vrg.md` (Text, graphemes, and safe presentation —
zero-width bullets; Testing Decisions → FileBuffer / Viewport),
`internal/viewport/viewport.go`, `internal/filebuffer/filebuffer.go`,
`internal/filebuffer/marker_test.go`, `internal/viewport/marker_test.go`.

## [2026-09-24] ingest | Issue #24 file-list layout — width formula, visibility toggle, and status slot

Ingested the completed Issue #24 implementation: the real file-list
width contract replacing the provisional one. `listWidth` now returns
the nonnegative minimum of longest sanitized path plus two,
`floor(0.40 × terminal width)` (as `width*2/5`), and the terminal
width minus the panel's gutter-plus-ten-plus-reserved-indicator
minimum, recomputed in `syncLayout` on resize, `w`, load (gutter
growth), search completion, file crossings, and the list's own
toggle. `listShow` is the requested-visible preference — `left`/`tab`
hide, `right`/`shift+tab` show, inert while searching — and a
zero-width allocation (the `W=20`/gutter 9/reserved 1 example) draws
no cells without touching it. Every hide/show is a text-width change
routed through the Issue #17 keyed prepared-layout path, so the
logical anchor's text stays at the top through `tab`/`shift+tab` and
gutter-growth round trips. `truncateLeft` was rewritten as a
cluster-boundary cut that never exceeds its budget
(`ansi.TruncateLeft` could overflow by a cell on a wide cluster
straddling the cut), `filenameRule` gained the buffer-status note
slot — the note wins cells over the path, which truncates to nothing
so it paints whole; real notes are owned by Issues #26, #29, and #30
and tests use a synthetic `statusNote` — and `listTop`/`scrollList`
keep the active entry in the visible window with minimal movement.
Created [file-list-layout](file-list-layout.md); updated
[source-code](source-code.md) (`app.go`'s `listShow`/`listTop`/
`statusNote` and the toggle key cases, `browse.go`'s `listWidth`,
`scrollList`, `truncateLeft`, and `filenameRule` slot),
[unit-tests](unit-tests.md) (the new `filelist_test.go` catalog),
[browse-tracer](browse-tracer.md) (the provisional-width note and the
status slot),
[logical-anchor-and-layout](logical-anchor-and-layout.md) (the
hide/show relayout cause), and the index.
New test file: `internal/app/filelist_test.go`.
Sources: `Notes/issues/024-file-list-layout-width-truncation-toggle.md`,
`Notes/tasks/024-file-list-layout-width-truncation-toggle.md`,
`Notes/PRD-vrg.md` (File list and layout; Layout and indicators —
the width bullets), `internal/app/app.go`, `internal/app/browse.go`,
`internal/app/filelist_test.go`.
## [2026-09-24] ingest | Issue #25 asynchronous load isolation — request-keyed completions and responsiveness

Ingested the completed Issue #25 implementation: `loading` is now a
raw-path → request-identity map (`loadSeq` mints), `loadDoneMsg`
carries `req`, and the `Update` case installs a completion only while
its identity matches the path's in-flight request — stale and
unsolicited results touch nothing, and a late completion after
cancellation hits the `quit` guard. A completion accepted for a
non-current path updates only that path's `bufs`/`revs` and drops its
stale `rows`/`reqKey` entries without touching the visible panel;
current-path completions still reset the offset and drive the
Issue #17 layout install plus pending reveal. `ensureLoad` drops
re-entry to an in-flight path — one load per raw path, nothing queued
— and `bufs` retains successful buffers for the session with no
eviction. `filebuffer.Load` split into `ReadFile` (read) and `Prepare`
(decode/map) so the new `mapGate` seam can hold the expensive phase
alone — a read-phase failure completes without reaching it — while the
gated tests prove resize, `w`, `c`, `n`/`p`, and `ctrl+c` stay
actionable. Created [async-load-isolation](async-load-isolation.md);
updated [source-code](source-code.md) (`app.go`'s
`loading`/`loadSeq`/`mapGate`, `browse.go`'s `req`-keyed completion
and two-phase worker, `filebuffer.go`'s `ReadFile`/`Prepare` split),
[unit-tests](unit-tests.md) (the new `load_test.go` catalog and the
request-minting injector updates),
[browse-tracer](browse-tracer.md) (the async-loading section rewritten
for keyed completions and the two gates),
[match-navigation](match-navigation.md) (the crossing's load link),
and the index.
New test file: `internal/app/load_test.go`.
Sources: `Notes/issues/025-async-load-isolation.md`,
`Notes/tasks/025-async-load-isolation.md`,
`Notes/PRD-vrg.md` (File loading, cache, reload, and selection
consistency; Resources and responsiveness), `internal/app/app.go`,
`internal/app/browse.go`, `internal/app/load_test.go`,
`internal/filebuffer/filebuffer.go`.
## [2026-09-24] ingest | Issue #26 read failures — "(unreadable)", notification split, and the gated re-entry retry

Ingested the completed Issue #26 implementation: a `loadDoneMsg` error
marks `failed`, retains its sanitized lines in `failLines`, collects one
`cannot read <safe path>: <err>` occurrence, and opens the overlay only
when the failed path is current — non-current failures stay
diagnostic-only for the exit replay with a byte-identical frame. The
filename row's Issue #24 note slot gained its first real provider,
`bufferNote` → `(unreadable)`, and `contentRow` splits its placeholders
— `Loading…` while a request is in flight (including a failed path's
retry), `(unreadable)` once settled — each clipped to the text width.
`navigate` routes destinations through `entryLoad`: same-file steps
request nothing, while a cross-file entry into a failed path skips the
pop-up, re-opens the retained overlay, and mints exactly one retry —
dropped not queued when a request is in flight per Issue #25. `Esc`
dismisses without disturbing the load; settlement updates the panel
independent of dismissal; a second failure appends one occurrence via
`openOverlay`'s scroll-preserving append; a success collects nothing
and leaves the prior overlay up. `loadCmd` consults the new `readFile`
seam (nil → `filebuffer.ReadFile`) so tests inject deterministic
loaders rather than rely on permissions, and the outcome matrix gained
`failAll`/`absent`/`viewHas`/`replayHas` plus the three fixed-status
rows (all-fail-0, current-fail-2, composed all-fail-2). Created
[read-failures](read-failures.md); updated
[source-code](source-code.md) (`app.go`'s `failLines`/`readFile` and
the current-split failure branch, `browse.go`'s `entryLoad`,
`bufferNote`, and placeholder rules), [unit-tests](unit-tests.md) (the
new `failures_test.go` catalog and the outcome-row extensions),
[browse-tracer](browse-tracer.md) (the `readFile` seam and the real
status note), [match-navigation](match-navigation.md) (the
`entryLoad` routing on crossings),
[async-load-isolation](async-load-isolation.md) (the three seams and
the failed-path re-entry exception),
[stderr-replay](stderr-replay.md) (the current/non-current display
split), [error-overlay-and-fatal-outcomes](error-overlay-and-fatal-outcomes.md)
(the retained-lines reopen, second-failure append, and new matrix
rows), [file-list-layout](file-list-layout.md) (`bufferNote` behind
the `statusNote` seam), and the index.
New test file: `internal/app/failures_test.go`.
Sources: `Notes/issues/026-read-failures-unreadable-retry-rules.md`,
`Notes/tasks/026-read-failures-unreadable-retry-rules.md`,
`Notes/PRD-vrg.md` (File loading, cache, reload, and selection
consistency; Outcome and exit-status contract),
`internal/app/app.go`, `internal/app/browse.go`,
`internal/app/failures_test.go`, `internal/app/outcome_test.go`.

## [2026-09-24] ingest | Issue #27 explicit reload — `r` reread, revisions, and the pending-intent seam

Ingested the completed Issue #27 implementation: `r` in browse routes to
`reload()`, which drops the cached buffer immediately (the panel shows
`Loading…` for the duration and a failure can never present stale
content as refreshed), re-opens the retained `failLines` overlay when
retrying a failed path — mirroring `entryLoad` — and mints exactly one
request through the shared `loadSeq`/`loading[path]` bookkeeping plus a
new `reloading` mark; rg is never rerun and the stops/cursor are
untouched. Duplicate `r` and cross-file re-entry while the path loads
are dropped not queued per Issue #25, and the placeholder's settlement
is the only completion signal — making `r` the one-stop index's only
retry route. A successful reload bumps `revs[path]`, stale-keying the
installed row model and any in-flight pre-reload layout request (the
revision-superseded discard Issue #17 designed for is now exercised);
the current-path completion `SetRows(nil)`s the viewport so nothing
superseded can paint, while the logical `anchor Target` survives and
resolves — clamped — against the new revision's layout. `pendingReveal`
became `pending pendingIntent` (`intentNone`/`intentReveal`/
`intentAnchor`): `reveal` still carries the newest-stop intent, a reload
completion records `intentAnchor` only when no reveal is already
carried, and `commitIntent` discharges whichever survives at matching
layout install — the seam Issue #28 generalizes into full
reveal-versus-reload arbitration. Failed reloads flow through the Issue
#26 current-file path (`(unreadable)` + overlay, one collected
occurrence, second-failure append preserving scroll). Created
[explicit-reload](explicit-reload.md); updated
[source-code](source-code.md) (`app.go`'s `reloading`/`pending`/`r`
key and the success branch, `browse.go`'s `reload`/`pendingIntent`/
`commitIntent`), [unit-tests](unit-tests.md) (the new `reload_test.go`
catalog), [read-failures](read-failures.md) (the `r` retry route),
[logical-anchor-and-layout](logical-anchor-and-layout.md) (the
widened pending intent and the exercised revision supersession),
[async-load-isolation](async-load-isolation.md) (reload sharing the
one-load-per-path rule), and the index.
New test file: `internal/app/reload_test.go`.
Sources: `Notes/issues/027-explicit-reload-r.md`,
`Notes/tasks/027-explicit-reload-r.md`,
`Notes/PRD-vrg.md` (File loading, cache, reload, and selection
consistency), `internal/app/app.go`, `internal/app/browse.go`,
`internal/app/reload_test.go`.

## [2026-09-24] ingest | Issue #28 load completion — two-stage contract and the latest-target commit

Ingested the completed Issue #28 work: the two-stage load-completion
contract is pinned end to end on the Issue #27 `pendingIntent` seam.
Stage 1 — the `loadDoneMsg` success branch for the current path —
installs the buffer, bumps the content revision, drops the viewport's
rows and stale-keys the old revision's layout and requests, resets the
horizontal offset, and recomputes the text width (grown gutter plus
Issue #24 list visibility participating) before requesting the keyed
layout — performing no row-based decision itself. Stage 2: the intent
lives in the model, so obsolete `layoutDoneMsg`s (wrong width, mode,
revision, or path) are discarded without consuming it, and
`commitIntent` runs only on a matching install — `intentReveal`
revealing the newest selected stop's final display target
(cluster-expanded first-submatch start cell, or the zero-width /
terminator-only marker cell) read live from `currentStop()` and the
newest buffer; `intentAnchor` resolving the retained anchor with no
reveal. Navigation during a load — same-file or cross-file
away-and-back ending on the starting cursor — leaves the reveal intent
standing, so navigation intent rather than cursor equality decides the
commit. Starting viewports follow the file-change sequence: first
visits (including startup) from top 0, revisits from the saved anchor,
visible targets never scrolling. The file-change pop-up is untouched
by either stage. Created
[load-completion-reveal](load-completion-reveal.md); updated
[explicit-reload](explicit-reload.md) (the completed arbitration and
the new transition tests),
[logical-anchor-and-layout](logical-anchor-and-layout.md) (the
pinned two-stage contract and the new test file),
[destination-reveal](destination-reveal.md) (the commit-time target
and obsolete-layout survival),
[async-load-isolation](async-load-isolation.md) (the current-path
completion's stage-1 boundary),
[match-navigation](match-navigation.md) (cross-reference),
[source-code](source-code.md) (`app.go`'s two-stage branch and
`browse.go`'s arbitration),
[unit-tests](unit-tests.md) (the `completion_test.go` catalog and the
new `reload_test.go` cases), and the index.
New test file: `internal/app/completion_test.go`.
Sources: `Notes/issues/028-load-completion-reveal-latest-target.md`,
`Notes/tasks/028-load-completion-reveal-latest-target.md`,
`Notes/PRD-vrg.md` (File loading, cache, reload, and selection
consistency; Navigation, viewport, and logical anchors),
`internal/app/app.go`, `internal/app/browse.go`,
`internal/app/completion_test.go`, `internal/app/reload_test.go`.
## [2026-09-24] ingest | Issue #29 stale-match validation — per-submatch drops, the "file changed since search" note, and fallback landings

Ingested the completed Issue #29 implementation: `filebuffer.Prepare`
validates every recorded submatch on first load and every reload —
line existence, range, and byte equality against the line's *raw*
bytes (terminator included; the leading UTF-8 BOM's three-byte shift
applies to line one's rg offsets; escaped display text is never
consulted) — dropping each failing submatch individually and marking
the buffer `stale` while survivors keep their cluster-expanded
highlights. `Buffer.Stale()` carries the verdict and
`Buffer.RevealTarget` answers a stop's reveal location: the first
surviving span's start cell, else the earliest recorded start
BOM-shifted and clamped to the line's bytes with end-of-line mappings
falling back to the last rendered cell, else the last source line's
start — fallbacks invent no highlights or markers, and an empty file
stays a zero-line panel. The app's new `stale` map records each
completed load's verdict, so `bufferNote` shows "file changed since
search" in the Issue #24 status slot on every display (below
`(unreadable)` in precedence), persists through a reread's
placeholder, and clears only on a fully validating completion.
`reveal()` consults `RevealTarget`, so the stale fallback rides the
Issue #28 two-stage commit — a navigation landing mid-reload reveals
the newest stop's survivor-or-fallback computed against the new
content. Stale state never moves the fixed exit status; the outcome
matrix gained the all-stale row (new `fileData`/`loadCurrent`
fields). Created
[stale-match-validation](stale-match-validation.md); updated
[destination-reveal](destination-reveal.md) (RevealTarget supplies
the target, fallbacks delivered),
[load-completion-reveal](load-completion-reveal.md) (the commit-time
target can be a stale fallback),
[explicit-reload](explicit-reload.md) (r recomputes the note),
[file-list-layout](file-list-layout.md) (Issue #29's real note in the
status slot),
[match-navigation](match-navigation.md) (stale stops stay stops),
[line-terminators-and-bom](line-terminators-and-bom.md) (the BOM
shift and raw-byte view the validation uses),
[source-code](source-code.md) (`filebuffer.go`'s verdict/`RevealTarget`,
`app.go`'s `stale` map, `browse.go`'s `bufferNote` branch and
`reveal`), [unit-tests](unit-tests.md) (the two new `stale_test.go`
catalogs and the outcome-matrix row), and the index.
New test files: `internal/filebuffer/stale_test.go`,
`internal/app/stale_test.go`.
Sources: `Notes/issues/029-stale-match-validation-and-file-changed-note.md`,
`Notes/tasks/029-stale-match-validation-and-file-changed-note.md`,
`Notes/PRD-vrg.md` (Encodings and stale-content validation;
Navigation, viewport, and logical anchors; Exit statuses; Testing
Decisions → FileBuffer), `internal/filebuffer/filebuffer.go`,
`internal/filebuffer/stale_test.go`, `internal/app/app.go`,
`internal/app/browse.go`, `internal/app/stale_test.go`,
`internal/app/outcome_test.go`.

## [2026-09-24] ingest | Issue #30 unsupported encodings — UTF-16/UTF-32 BOM detection, "(unsupported encoding)" placeholder, and the notification split

Ingested the completed Issue #30 implementation: `filebuffer.Prepare`
classifies the raw bytes through the longest-first `unsupportedBOMs`
table before any splitting or validation — `FF FE 00 00` UTF-32 LE,
`00 00 FE FF` UTF-32 BE, `FF FE` UTF-16 LE, `FE FF` UTF-16 BE, the
UTF-32 LE mark checked before the UTF-16 LE mark its prefix overlaps —
while the leading UTF-8 BOM keeps its Issue #22 supported path and
non-leading marks stay ordinary content. A classified buffer carries
only `enc` (`Unsupported()`): no lines, no spans, no stale verdict,
an inert `(0,0)` reveal target — the encoded bytes never reach
`present.LineOf` and never face Issue #29's raw-byte validation, the
PRD's explicit exclusion applied where every load funnels through.
The app presents `contentRow`'s third placeholder `(unsupported
encoding)` behind the minimal gutter plus `bufferNote`'s third
filename-row note, collects `cannot display <path>: unsupported
encoding <name>` once per detection into the session collection, and
follows the Issue #26 split — overlay when the path is current,
collection alone when it is not — with the retained `encLines`
re-opening the overlay on a later visit in place of the file-change
pop-up. `r` remains the reload route (swallowed by the open overlay,
then `Loading…`, exactly one reread, placeholder and fresh overlay on
a still-encoded completion), the file stays an indexed cursor stop,
and the fixed exit status never moves — the outcome matrix gained
the all-unsupported row. Created
[unsupported-encodings](unsupported-encodings.md); updated
[stale-match-validation](stale-match-validation.md) (detection landed;
encoded bytes skip validation),
[line-terminators-and-bom](line-terminators-and-bom.md) (the UTF-16/32
marks versus the supported UTF-8 BOM),
[read-failures](read-failures.md) (the shared notification split),
[explicit-reload](explicit-reload.md) (the rereading placeholder),
[match-navigation](match-navigation.md) (unsupported files stay
stops),
[error-overlay-and-fatal-outcomes](error-overlay-and-fatal-outcomes.md)
(the new fixed-status row),
[file-list-layout](file-list-layout.md) (the third real status note),
[source-code](source-code.md) (`filebuffer.go`'s classification,
`app.go`'s `encLines` and completion split, `browse.go`'s placeholder,
note, and overlay route), [unit-tests](unit-tests.md) (the two new
`encoding_test.go` catalogs and the outcome-matrix row), and the
index.
New test files: `internal/filebuffer/encoding_test.go`,
`internal/app/encoding_test.go`.
Sources: `Notes/issues/030-unsupported-encodings-utf16-utf32.md`,
`Notes/tasks/030-unsupported-encodings-utf16-utf32.md`,
`Notes/PRD-vrg.md` (Encodings and stale-content validation; Invocation
and child process; Exit statuses; Testing Decisions → FileBuffer),
`internal/filebuffer/filebuffer.go`,
`internal/filebuffer/encoding_test.go`, `internal/app/app.go`,
`internal/app/browse.go`, `internal/app/encoding_test.go`,
`internal/app/outcome_test.go`.

## [2026-09-24] ingest | Issue #31 help overlay — `h`/`?` modal key bindings on the shared overlay component

Ingested the completed Issue #31 implementation: `h` and `?` open the
modal help overlay over ordinary browsing and the no-results screen
(inert during searching like every other ordinary key), closing with
`q`/`Esc`/`h`/`?` back to the underlying base state — a subsequent `q`
on no-results still exits 1. While open, `up`/`down` scroll the
rendered rows clamped at both ends, `ctrl+c` exits 130, and every
other key — including `n`/`p`/`w`/`c`/`r` — is ignored with the state
behind unchanged. The Issue #9 `overlay` struct is now the shared
wrapped-scrollable component both modals instantiate: its geometry
helpers became value methods `layout(w, h)` and `maxScroll(w, h)`,
`scrollOverlay` is the shared clamped scroll handler, and
`renderOverlay(base, o)` composites whichever instance is passed —
`View` paints pop-up, then help, then the diagnostics overlay, so an
error arriving over help suspends it at its retained scroll position
(Issue #32 owns the combined precedence matrix). The binding list is
the single `helpBindings` table (key spelling → description) covering
navigation, scrolling, panning, wrap, colour, list toggle, reload,
help, and quit/cancel, rendered by `helpLines` behind the `Key
bindings` title with the `helpFooter` slot reserved for Issue #34 —
the substitution point routed through `present.Diagnostic`. Opening
help cancels a live file-change pop-up with no return. Text hard-wraps
to the interior width including unbroken strings, scrolling reaches
every row at usable sizes, and at tiny sizes above the 20×3 minimum
the box is clipped to the terminal without a borderless mode and
restored on growth. Created [help-overlay](help-overlay.md); updated
[error-overlay-and-fatal-outcomes](error-overlay-and-fatal-outcomes.md)
(the shared component and renamed methods),
[file-change-popup](file-change-popup.md) (`openHelp` cancellation),
[no-results-and-binary-exclusion](no-results-and-binary-exclusion.md)
(help over the no-results screen),
[safe-presentation](safe-presentation.md) (the delivered `help
overlay` sink row), [source-code](source-code.md) (`help.go`, the
`app.go` help field/routing, the `overlay.go` refactor),
[unit-tests](unit-tests.md) (the `help_test.go` catalog and the new
sink row), and the index.
New files: `internal/app/help.go`, `internal/app/help_test.go`.
Sources: `Notes/issues/031-help-overlay.md`,
`Notes/tasks/031-help-overlay.md`, `Notes/PRD-vrg.md` (Colours,
overlays, and key precedence), `internal/app/help.go`,
`internal/app/help_test.go`, `internal/app/overlay.go`,
`internal/app/app.go`, `internal/app/sinksafety_test.go`.

## [2026-09-24] ingest | Issue #32 overlay precedence — error suspends help, append preserves scroll, `Esc`/`q` dismissal semantics

Ingested the completed Issue #32 implementation: the `KeyPressMsg`
routing in `Update` is the `ctrl+c` → modal error → help → pop-up →
base precedence stack, `openOverlay` suspends an open help at its
retained scroll position (either dismissal key restores it) and is the
single open-or-append route every error takes — appending while the
reader's scroll position holds — and `openHelp`/`openOverlay` each
cancel a live pop-up with no return. `Esc` is dismissal-only: a no-op
in every base state (its one base-state effect is pop-up dismissal),
while dismissing the fatal no-results overlay with either `q` or `Esc`
exits 2 because there is no underlying state. The composed semantics
needed no production changes — they were already assembled from the
Issue #9/#15/#26/#31 primitives — so the issue's deliverable is the
new `precedence_test.go` pinning them: the gated
error-while-help-open suspension at scroll 5 for both dismissal keys
with error-first key routing, the generalized append-preserving-scroll
case (an unsupported-encoding diagnostic appended to a read-failure
overlay), and the full dismissal-outcome table run for `q` and `Esc`
with the state-specific follow-ups — second `q` exits the fixed
status, second `Esc` no-ops, and the error-over-help row's three-key
sequence. Created
[overlay-precedence](overlay-precedence.md); updated
[error-overlay-and-fatal-outcomes](error-overlay-and-fatal-outcomes.md)
(the `Esc` note cross-reference),
[help-overlay](help-overlay.md) (the suspension note's
cross-reference), [file-change-popup](file-change-popup.md) (the
cancellation note's cross-reference),
[source-code](source-code.md) (the `app.go` precedence routing),
[unit-tests](unit-tests.md) (the `precedence_test.go` catalog), and
the index.
New files: `internal/app/precedence_test.go`.
Sources: `Notes/issues/032-overlay-precedence-esc-semantics.md`,
`Notes/tasks/032-overlay-precedence-esc-semantics.md`,
`Notes/PRD-vrg.md` (Colours, overlays, and key precedence; Outcome and
exit-status contract), `internal/app/app.go`, `internal/app/overlay.go`,
`internal/app/help.go`, `internal/app/popup.go`,
`internal/app/precedence_test.go`.

## [2026-09-24] ingest | Issue #33 terminal-too-small gate with full state recovery

Ingested the completed Issue #33 implementation: `toosmall.go` adds a
gate ahead of the whole key-precedence stack — a window under 20
columns or under 3 rows renders a centred "Terminal too small"
message (clipped, never overflowing, at pathological sizes) while the
authoritative model keeps every semantic position. The gate's own
key map is `ctrl+c` (exit 130) and `q` (the state-applicable outcome
— 130 with cancellation while searching, the fixed search-derived
status while browsing, 1 on no-results, 2 under a fatal overlay, and
past a browse-error overlay with the fixed browse status), which
takes precedence over Issue #32's dismissal semantics; `Esc` and
every other key are no-ops that leave a logically open modal
untouched. `syncLayout` short-circuits while gated, so interior
too-small resizes install no layout, mutate no anchors or saved
viewport tops, and issue no preparation requests — recovery replays
`syncLayout` once against the first viable dimensions. Pop-up timers
continue under the gate: the instance is never painted, an expiry
arriving there dismisses it for good, and a live instance reappears
after recovery. The new `toosmall_test.go` pins the threshold
(including the exact 20×3 boundary), centred/clipped rendering, the
per-state `q`/`ctrl+c` table, the no-op keys, browse and modal
round-trips (scrolled help, scrolled error, error-over-help),
interior-resize deferral, search completion arriving behind the
gate, and the pop-up timer cases; `scroll_test.go`'s pathological
half-page row moved to height 3, the smallest viable size.
Created [terminal-too-small](terminal-too-small.md); updated
[overlay-precedence](overlay-precedence.md) (the dedicated-rule
note), [file-change-popup](file-change-popup.md) (the hidden-but-
ticking timer note), [help-overlay](help-overlay.md) (the
below-minimum clause on the clipping paragraph),
[logical-anchor-and-layout](logical-anchor-and-layout.md) (the
`syncLayout` guard note), [source-code](source-code.md) (the
`toosmall.go` catalog and gate routing),
[unit-tests](unit-tests.md) (the `toosmall_test.go` catalog), and
the index.
New files: `internal/app/toosmall.go`, `internal/app/toosmall_test.go`,
`Notes/wiki/terminal-too-small.md`.
Sources: `Notes/issues/033-terminal-too-small-with-state-recovery.md`,
`Notes/tasks/033-terminal-too-small-with-state-recovery.md`,
`Notes/PRD-vrg.md` (Layout and indicators; Outcome and exit-status
contract), `internal/app/toosmall.go`, `internal/app/app.go`,
`internal/app/browse.go`, `internal/app/toosmall_test.go`,
`internal/app/scroll_test.go`.

## [2026-09-24] ingest | Issue #34 documentation — README, footer note, synchronization tests

Ingested the completed Issue #34 implementation: `README.md` at the
repository root is now the single user-facing documentation artifact —
invocation syntax and option-placement rules, the embedded verbatim
generated help (the shared `optionDecls` declarations' own output,
keeping the allow-listed flags' exact no-argument spellings and the
local `-h`/`--help` options incapable of drifting from parsing), the
full `helpBindings` table, the four-row exit-status table covering
both exit-0 reasons and the `vrg -i` flags-only usage error, the three
independent scale examples with their not-simultaneous qualification,
the 64 MiB record limit with the base64 `bytes` expansion caveat and
the path-naming oversized diagnostic, the session-long
retention/no-eviction/no-aggregate-bound/no-OOM-recovery/no-cleanup
statements, and the ripgrep 15.x reference family with VRG's supplied
`--no-config`. `internal/app/help.go` gained `limitNotes` — the
structured source for the three scale/record-limit/memory statements —
and `helpFooter` is filled with it, each entry still routed through
`present.Diagnostic`; the README carries the same entries verbatim and
`TestHelpFooterMatchesREADME` pins the equivalence. New tests:
`internal/app/readme_test.go` (binding-table iteration, the
exit-status table's agreement with `decideOutcome`, the help-only
path's distinction from the `h`/`?` dialog, ripgrep/scale/record/
memory statements, footer↔README equivalence including the rendered
overlay) and `internal/cli/readme_test.go` (per-declaration spellings
and option lines, verbatim `HelpText()`, the invocation contract);
`sinksafety_test.go` gained Issue #34's `help footer note` row —
`renderHelpFooterSink` substitutes each hostile fixture at every
runtime-substitution point of the rendered footer and asserts the
escaped forms plus the real note text inside the border.
`TestHelpScrollsWithUpDown` now ends on the footer at the tail, and
the footer-slot tests save/restore `helpFooter`.
Created [documentation](documentation.md); updated
[help-overlay](help-overlay.md) (filled footer slot, second sink row,
tests/files),
[safe-presentation](safe-presentation.md) (the new row and the Issue
#34 later-sink ownership delivered),
[source-code](source-code.md) (the `help.go` entry's `limitNotes`/
filled `helpFooter`),
[unit-tests](unit-tests.md) (both `readme_test.go` catalogs, the new
sink row, the scroll-test tail), and the index.
New files: `README.md`, `internal/app/readme_test.go`,
`internal/cli/readme_test.go`, `Notes/wiki/documentation.md`.
Sources: `Notes/issues/034-documentation-scale-and-memory-limits.md`,
`Notes/tasks/034-documentation-scale-and-memory-limits.md`,
`Notes/PRD-vrg.md` (Resources and responsiveness; Out of Scope;
Further Notes), `README.md`, `internal/app/help.go`,
`internal/app/readme_test.go`, `internal/cli/readme_test.go`,
`internal/app/sinksafety_test.go`, `internal/app/help_test.go`.
## [2026-09-24] ingest | Issue #35 final integration verification — clean-checkout gates, PTY/subprocess re-runs, five smoke outcomes

Ingested the completed Issue #35 verification pass. From a clean
worktree checkout of the composed repository: `go build ./...`,
`go vet ./...`, and `go test ./...` all pass; the Issues #4/#9/#11
PTY/subprocess packages were re-run uncached (`go test -count=1
./cmd/vrg` and the subprocess-boundary tests in `./internal/app`) with
every test executing; and `scripts/smoke.py` verified all five final-
binary outcomes — browse exit 0 after `q`, warning-overlay dismissal
then no-results exit 1, the fatal composed diagnostic exiting 2 under
both `q` and `Esc` with stderr replay, cancellation exiting 130 with
the child and its process group externally gone and termios/display
restored, and the help-only invocation (bare, `-h`, `--help`) printing
one `Usage:` copy with empty stderr and exit 0 while a sentinel fake rg
is never invoked. The pass caught one real regression: cancellation
killed only the direct child PID, so a scripted rg whose blocking
payload ran as a grandchild left the drained pipes held open and the
process hung post-restoration — fixed in `internal/app/search.go` by
spawning the child as a process-group leader and SIGKILLing the group
in `cmd.Cancel`, with `TestCancelTerminatesChildProcessGroup` (new
`forkblock` fake-rg mode) as the regression test. The seeded smoke
harness's stale marker strings were aligned to the composed diagnostic
wording the focused tests pin (`exit status N`, `missing summary`) and
to the fragmented repaint of the centred no-results text; no behavioral
assertion changed.
Created [final-verification](final-verification.md); updated
[cancellation-and-cleanup](cancellation-and-cleanup.md) (process-group
kill),
[source-code](source-code.md) (`search.go` `Setpgid`/`Cancel`),
[unit-tests](unit-tests.md) (`forkblock` mode, the new test), and the
index.
New files: `Notes/wiki/final-verification.md`.
Sources: `Notes/issues/035-final-integration-verification.md` (task
file `Notes/tasks/035-final-integration-verification.md`),
`Notes/PRD-vrg.md` (Testing Decisions), `internal/app/search.go`,
`internal/app/subprocess_test.go`, `scripts/smoke.py`, the clean
worktree gate output, and the smoke run.
## [2026-09-24] ingest | Issue #36 stream-integrity fatal diagnostics — structured causes, post-summary precedence, universal composition

Ingested the completed Issue #36 implementation. `internal/searchindex`
gained `cause.go`: the exported `Cause`/`CauseKind` model — nine stable
kinds, each carrying the offending record's raw path bytes where it
names one, rendered by `Cause.Line()` through `present.Path` — with
`IntegrityCauses()` returning the ordered list and `IntegrityFailures()`
deriving one line per cause. `index.go` replaced the string `failures`
with the `causes` slice plus the `unterminated` flag, added the `Add`
post-summary gate (a second summary is only `extra summary record`;
every other post-summary record is only `record after summary` and is
never lifecycle-processed — a `begin` cannot open, a `match` is dropped
unretained, `context` lost its exemption, unknown types still tally),
split the trailing fragment on summary state (post-summary →
`record after summary` + malformed count; otherwise the deferred
`unterminated final record` cause + malformed count), and made `seal`
append end-of-stream causes in the mandated order (missing ends sorted
by unsigned raw path bytes, missing summary, unterminated final).
Post-summary malformed/oversized/unknown records keep the dual
representation — sole cause plus independent tallies — and
`OversizedDiagnostics()` exposes the per-path detail lines.
`internal/app/overlay.go` replaced `collectDiagnostics`/
`processDiagnostic`/`streamDiagnostics` with `outcomeInput` and the
universal `composeDiagnostics` order — process component (collected
stderr, or the generated `rg failed:` line only for a fatal result
without stderr; never a line for exit 0/1), integrity causes, malformed
and oversized aggregates, per-path oversized details, unknown warnings —
shared by the overlay and the `completionDiagnostics` collection subset
replayed verbatim; fatal branches can no longer suppress causes or
record-loss tallies. `app.go` gathers the index's structured accessors
into `outcomeInput`.
Created
[stream-integrity-fatal-diagnostics](stream-integrity-fatal-diagnostics.md);
updated
[error-overlay-and-fatal-outcomes](error-overlay-and-fatal-outcomes.md)
(lifecycle rows, cause model, stderr classification),
[record-robustness](record-robustness.md) (dual representation,
component order), [source-code](source-code.md) (`cause.go`,
`index.go`, `overlay.go` entries), [unit-tests](unit-tests.md)
(`causes_test.go`, `diagnostics_test.go`, corrected row descriptions),
and the index.
New files: `internal/searchindex/cause.go`,
`internal/searchindex/causes_test.go`, `internal/app/diagnostics_test.go`,
`Notes/wiki/stream-integrity-fatal-diagnostics.md`.
Sources: `Notes/tasks/036-stream-integrity-fatal-diagnostics.md`,
`Notes/PRD-vrg.md` (*Result index, records, and stream integrity*,
*Outcome and exit-status contract*), `internal/searchindex/index.go`,
`internal/searchindex/cause.go`, `internal/app/overlay.go`,
`internal/app/app.go`, the new and corrected test files.
## [2026-09-25] ingest | Issue #37 oversized-record aggregate and anonymous diagnostics

Ingested the completed Issue #37 implementation. `internal/searchindex`
gained the `oversizedSeen` raw-path set: `countOversized` still counts
every oversized record in `Oversized()` but appends a recovered path to
`oversizedPaths` only on its first occurrence, so
`OversizedDiagnostics()` — and the `RecordDiagnostics()` view sharing
the same storage — emits one `oversized record skipped for <escaped
path>` line per **distinct raw path** in first-occurrence stream order
(the `text` and `bytes` encodings of one path agree). `internal/app`'s
composition was already the Issue #36 universal order — malformed
aggregate, oversized aggregate, per-path oversized details, unknown
warnings — so the oversized component now leads with the always-emitted
pluralized aggregate (`1 oversized record skipped` / `N oversized
records skipped`) whenever the count is positive, even when no detail
exists: an anonymous oversized record (the limit cut before
`type`/`data.path` were parsed) produces a fatal overlay containing
exactly the aggregate and exit 2 with zero usable results, or the
warning overlay plus stderr replay with usable results — never silent,
never an empty overlay. `internal/searchindex/oversized_test.go`
gained the dedup tests (repeated path, `text`/`bytes` raw-path
agreement, mixed recoverability in first-occurrence order) plus the
`matchSizedPath` helper; `internal/app/diagnostics_test.go` gained the
post-summary exact slice (`record after summary`,
`1 oversized record skipped`, the recovered detail), the anonymous and
plural composition rows, and the real-64 MiB model-level tests
(anonymous fatal/non-fatal, dedup and mixed-recoverability overlays);
`internal/app/outcome_test.go` gained the `recordLimit`/
`oversizedMatch`/`oversizedAnonMatch` fixtures and three oversized
outcome-matrix rows.
Updated
[record-robustness](record-robustness.md) (aggregate rule and exact
strings, raw-path dedup and first-occurrence ordering, anonymous-record
fatal/non-fatal guarantees, component position),
[stream-integrity-fatal-diagnostics](stream-integrity-fatal-diagnostics.md)
(the oversized component inside the universal order),
[error-overlay-and-fatal-outcomes](error-overlay-and-fatal-outcomes.md)
(the composition sentence and never-empty-fatal guarantee),
[source-code](source-code.md) (`oversizedSeen`/`countOversized`),
[unit-tests](unit-tests.md) (the new searchindex and app coverage),
and the index.
New files: none — `internal/searchindex/index.go` gained one field and
a guarded append; `Notes/walkthroughs/037-04/` holds the walkthrough.
Sources: `Notes/tasks/037-oversized-record-aggregate-anonymous-diagnostics.md`,
`Notes/PRD-vrg.md` (*Result index, records, and stream integrity*
oversized bullets, *Resources and responsiveness* 64 MiB bullets),
`internal/searchindex/index.go`, `internal/app/overlay.go`,
`internal/app/app.go`, the new and updated test files.
## [2026-09-25] ingest | Issue #38 viewport content-panel text width

Issue #38 (`Notes/tasks/038-viewport-content-panel-width.md`) pinned
the terminal→panel→text width chain: the panel width is the terminal
width minus the file list's allocated cells, and the text width —
the width `layoutKey` carries, `viewport.Prepare` wraps and clips
against, and every viewport install site (resize, wrap toggle,
load/search completion, file crossing, list hide/show, cache hits)
uses — is the panel width minus the buffer gutter minus the mode's
reserved right-indicator column (one run-off-edge, zero wrap). The
audit confirmed `syncLayout`'s single computation already governs all
install sites; `internal/app/browse.go` now names `panelW` explicitly
and `internal/app/app.go`'s field comment distinguishes the three
widths. The horizontal-reveal tests moved from
`internal/app/hreveal_test.go` to `internal/app/reveal_horizontal_test.go`,
where the `wantTextW()` helper recomputes expectations from the
layout chain rather than the cached `m.textW`; new coverage includes
list-hidden re-measure, wrap mode's zero reservation, resize
re-measurement, and `TestComposedViewContentStaysInsidePanel` (no
rendered row exceeds the terminal width, panned content stays inside
the panel, the reserved indicator sits at the panel's right edge).
`pan_test.go`, `layout_test.go`'s `wantKey`, and
`completion_test.go`'s offset expectations consume `wantTextW()` too.
Updated [file-list-layout](file-list-layout.md) (new *Terminal, panel,
and text widths* section),
[logical-anchor-and-layout](logical-anchor-and-layout.md) (the key's
`Width` contract), [minimal-horizontal-reveal](minimal-horizontal-reveal.md)
and [unit-tests](unit-tests.md) (the renamed file and new tests),
[hidden-content-indicators](hidden-content-indicators.md) (the
reserved column's placement basis),
[grapheme-highlight-expansion](grapheme-highlight-expansion.md) (the
renamed test file), [source-code](source-code.md) (`syncLayout`'s
chain and the `textW` field), and the index.
New files: `internal/app/reveal_horizontal_test.go` (replacing
`hreveal_test.go`); `Notes/walkthroughs/038-04/` holds the
walkthrough.
Sources: `Notes/tasks/038-viewport-content-panel-width.md`,
`Notes/PRD-vrg.md` (*File list and layout*, *Layout and indicators*,
*Navigation, viewport, and logical anchors*),
`internal/app/browse.go`, `internal/app/app.go`,
`internal/app/reveal_horizontal_test.go`, `internal/app/pan_test.go`,
`internal/app/layout_test.go`, `internal/app/completion_test.go`.

## [2026-09-24] ingest | Issue #39 shared grapheme/cell model end to end

Issue #39 (`Notes/tasks/039-render-from-shared-grapheme-cell-model.md`)
made `internal/present/cellwidth.go` the single authority for terminal
display geometry: `CellWidth`, `TruncateLeft`, `Truncate`, `Cut`, and
`Wrap` implement one ANSI-aware grapheme policy — text segments into
grapheme clusters (a base plus its combining marks is one cluster, an
emoji ZWJ sequence is one two-cell cluster), each cluster occupies the
cells of its widest glyph, and escape/control sequences paint no
cells. Every display-width and truncation consumer now routes through
the helper: `renderCells` emits `present.Line` cells directly (the
file panel never re-derives positions from runes or bytes),
`Diagnostic`'s tab-stop column counting, `listWidth`'s longest-entry
measure and list-entry padding, indicator-column sizing,
`filenameRule`, the file-change pop-up's truncation and centring, the
diagnostics overlay's wrap and box splice, `Theme.Overlay`'s border
sizing and padding, and the centred no-results/too-small screens.
Issue #24's `truncateLeft` moved into the package as
`TruncateLeft`, becoming ANSI-aware — sequences are skipped and
preserved so kept text retains its styling state. A source-scanning
test (`internal/app/guard_test.go`) walks every non-test production
`.go` file under `internal/` and `cmd/` and permits
`utf8.DecodeRuneInString` in `internal/present/cellwidth.go` alone;
`Diagnostic`'s non-geometry decoding now uses `utf8.DecodeRune`. The
composed-view tests (`internal/app/cellmodel_test.go`) pin
cluster-exact highlights for two-cell CJK, combining-only matches,
and interior bytes of a ZWJ sequence; blanked split-cluster cells,
wide/combining list entries and filename-rule fitting, indicator
visibility for a split wide cluster, and the pop-up's width,
truncation, and centring under decomposed combining marks.
Updated [safe-presentation](safe-presentation.md) (new *Display
geometry* section and the `cellwidth.go` file entry),
[file-list-layout](file-list-layout.md) (the shared `TruncateLeft`),
[file-change-popup](file-change-popup.md) (shared-helper truncation
and centring), [theme-and-colour-toggle](theme-and-colour-toggle.md)
(`Overlay`'s measured-cell sizing), [source-code](source-code.md)
(the new helper plus its consumers), [unit-tests](unit-tests.md) (the
new test files), and the index.
New files: `internal/present/cellwidth.go`,
`internal/present/cellwidth_test.go`, `internal/theme/cellwidth_test.go`,
`internal/app/cellmodel_test.go`, `internal/app/guard_test.go`;
`Notes/walkthroughs/039-04/` holds the walkthrough.
Sources: `Notes/tasks/039-render-from-shared-grapheme-cell-model.md`,
`Notes/PRD-vrg.md` (*Text, graphemes, and safe presentation*,
*Navigation, viewport, and logical anchors*),
`internal/present/cellwidth.go`, `internal/present/present.go`,
`internal/app/browse.go`, `internal/app/overlay.go`,
`internal/app/popup.go`, `internal/theme/theme.go`,
`internal/app/cellmodel_test.go`, `internal/app/guard_test.go`.
## [2026-09-24] ingest | Issue #40 bounded browse render — completion-time groups, no whole-index scans

Issue #40 (`Notes/tasks/040-browse-render-no-whole-index-scan.md`)
removed the last whole-index work from the keystroke and frame paths.
The `searchDoneMsg` browse branch now builds the immutable per-file
structures once — `m.stops` (the single `Index.Stops()`
materialization), `files`/`fileIdx`, `fileStops` (raw path → its stop
group), and `longestEntryW` (the widest list entry's painted cell
width via `listEntry` + `present.CellWidth`). `loadCmd` hands each
load worker the destination's precomputed `fileStops` group instead of
re-filtering `Index.Stops()` per issued load, and `listWidth` reads
`longestEntryW` as formula term 1 instead of re-measuring every entry
inside `syncLayout`. `m.index` narrows to the new `stopIndex` read
seam (`Current`/`Next`/`Prev`/`Stops`) so `countingIndex` can tally
whole-stop materializations; `stopsForFile` moved to
`browse_test.go`, test-only since nothing in production needs it.
Per-frame work stays as before: `renderBrowse` queries `listEntry`
only for `files[listTop : listTop+rows]` and left-truncates each
visible entry against the *current* `listW` through
`present.TruncateLeft` — the truncated text cannot be precomputed
because the allotted width moves with resize, gutter growth, wrap
mode, and the list toggle. The new guard
(`internal/app/rendercost_test.go`) counts provider queries and
`Stops()` calls across a navigation `Update()` plus the resulting
`View()` without reset, bounding the combined transition by the
visible row count; the resize and gutter-growth case asserts the
visible entries re-truncate at grapheme boundaries inside the same
bound. Rationale: the PRD's *Resources and responsiveness* scale
examples (~10,000 files, ~100,000 matched lines) demand no unbounded
work on the UI update path.
Updated [file-list-layout](file-list-layout.md) (the `listWidth`
formula term and a new *Bounded transition and render cost — Issue
#40* section), [match-navigation](match-navigation.md) (the
crossing's bounded per-keystroke cost), [browse-tracer](browse-tracer.md)
(the completion-time precompute and the `stopIndex` seam),
[source-code](source-code.md) (the `fileStops`/`longestEntryW`
fields, `stopIndex`, `loadCmd`'s group read, `stopsForFile`'s move),
[unit-tests](unit-tests.md) (`rendercost_test.go` and the
`TestListWidthFormula` seam), and the index.
New files: `internal/app/rendercost_test.go`;
`Notes/walkthroughs/040-04/` holds the walkthrough.
Sources: `Notes/tasks/040-browse-render-no-whole-index-scan.md`,
`Notes/PRD-vrg.md` (*Resources and responsiveness*, *File list and
layout*), `internal/app/app.go`, `internal/app/browse.go`,
`internal/app/rendercost_test.go`, `internal/app/filelist_test.go`,
`internal/app/browse_test.go`.
## [2026-09-24] ingest | Issue #41 full-scroll overlays — no head/tail compression

Issue #41 (`Notes/tasks/041-overlay-full-scroll-no-head-tail-compression.md`)
pinned the complete-scrollable-row contract for non-help overlays. The
model already kept every wrapped row scrollable — `layout` wraps every
diagnostic line and `renderOverlay` slices the clamped visible window —
so the work was contractual rather than a removal: new tests in
`internal/app/overlay_test.go` assert the scrollable row set is the
complete wrapped diagnostic (joining `layout`'s rows reproduces every
line in order for a ≥ 1 MiB stderr shape, first and last markers
intact, no elision row injected), that `scroll` clamps exactly to
`[0, max(0, rows − interiorH)]` in both `scrollOverlay` (the key
handler) and `layout` (the render path), that a bounded row-by-row
traversal reaches both ends on a slightly-oversized fixture, that an
appended error extends the set's tail without moving the reader, and
that `u`/`d`/`pgup`/`pgdown` stay inert while an error overlay is open
(the `pressKey` helper gained `pgup`/`pgdown` mappings).
`cmd/vrg/pty_test.go`'s 1 MiB stderr-content fixture was revised to
`TestPTYStderrContentFixture` at an ordinary 80×24 PTY: it keeps the
dual-pipe drainage (`writes-done` handshake), complete-stdout
(`f0.txt` browse), captured-stderr inclusion (`ERRHEAD-MARKER` in the
overlay), and exit-0 assertions while dropping the simultaneous
head/tail-in-one-frame requirement — tail reachability now proven by
the model-level tests. Render-time clipping at tiny sizes
(`composite`, story 83) and help-overlay scrolling are unchanged.
Updated [error-overlay-and-fatal-outcomes](error-overlay-and-fatal-outcomes.md)
(the *Complete scrollable row set (Issue #41)* contract paragraph and
the tests section), [unit-tests](unit-tests.md) (the four new
`overlay_test.go` cases, the extended ignored-keys list, and the
revised PTY fixture entry), and the index.
New files: none — tests added to `internal/app/overlay_test.go`,
`internal/app/outcome_test.go` (helper), and `cmd/vrg/pty_test.go`;
`Notes/walkthroughs/041-04/` holds the walkthrough.
Sources: `Notes/tasks/041-overlay-full-scroll-no-head-tail-compression.md`,
`Notes/PRD-vrg.md` (*Colours, overlays, and key precedence*),
`internal/app/overlay.go`, `internal/app/overlay_test.go`,
`internal/app/outcome_test.go`, `cmd/vrg/pty_test.go`.
## [2026-09-24] ingest | Issue #42 atomic reload admission — a dropped `r` commits nothing

Issue #42 (`Notes/tasks/042-dropped-reload-no-intent-mutation.md`)
pinned the load-admission boundary: `reload()`'s in-flight check was
already the single decision point — evaluated before the reread mark,
the buffer drop, the prior-failure overlay, and the identity mint — so
the work was contractual rather than a restructure. New tests in
`internal/app/admission_test.go` assert a dropped `r` during a startup
or navigation load leaves the in-flight request's identity, the
content revision, the pending `intentReveal`, and the frame untouched,
with the load then completing under its own classification (the
one-third destination reveal, never `intentAnchor`); that an admitted
`r` lands the request, mark, and `Loading…` together with exactly one
revision increment and the anchor intent at completion; that rapid
repeats keep one reread in flight with the placeholder → content /
`(unreadable)` settlement; and that navigation re-entry is
deliberately ungated — selection, placeholder, and reveal intent all
update while its duplicate load drops. `reload()`'s doc comment now
records the Issue #42 atomicity.
Updated [explicit-reload](explicit-reload.md) (the atomic-admission
contract and the new tests),
[load-completion-reveal](load-completion-reveal.md) (the reread mark
exists only on an admitted request),
[async-load-isolation](async-load-isolation.md) (the atomic drop and
the ungated re-entry), [unit-tests](unit-tests.md) (the
`admission_test.go` catalog), and the index.
New files: `internal/app/admission_test.go`;
`Notes/walkthroughs/042-04/` holds the walkthrough.
Sources: `Notes/tasks/042-dropped-reload-no-intent-mutation.md`,
`Notes/PRD-vrg.md` (*File loading, cache, reload, and selection
consistency*; *Navigation, viewport, and logical anchors*),
`internal/app/browse.go`, `internal/app/admission_test.go`.

## [2026-09-25] ingest | Issue #43 standalone combining-cluster fallback cell

Issue #43 (`Notes/tasks/043-combining-cluster-fallback-cell.md`;
recorded representation in
`Notes/decisions/043-combining-cluster-fallback-cell.md`) generalized
the Issue #21 line-start provisional fallback into a permanent rule
for every standalone zero-width grapheme cluster: such a cluster
paints one real terminal cell holding U+25CC DOTTED CIRCLE followed by
the cluster's original bytes. The one-cell occupancy is structural —
`emit` appends exactly one `Cell` regardless of what a width call
returns — and the `◌` prefix is display-only, so the byte→cell map
still resolves the cell to the cluster's original source bytes and a
mark-only match highlights exactly that cell.

The distinguishing fix lives in `lineOf`'s segmentation boundary: a
printable ASCII byte followed by a non-ASCII byte now takes the
grapheme-cluster path, so `e`+U+0301 emits as the single cluster the
shared policy reports and a `lead` zero-width emit is standalone by
construction — after a caret escape, a tab expansion, a `^M`, or
another standalone cluster. The eager `\ufffd` fast-path emit was
removed so an invalid byte flows through its real cluster, keeping a
mark the policy attaches to it composing onto the U+FFFD base. A
mid-line U+FEFF and a zero-width U+2028 separator now take fallback
cells of their own.

Everything downstream consumes the fallback as an ordinary `Lead`
cell: `Line.Text`/`Width`/`Span`, FileBuffer's validated-submatch
mapping and `clusterSpan` (a mark match has nothing to expand into),
viewport wrap/clip/pan, and the app's `renderCells`, indicators, and
minimal reveal. Tests formerly encoding the borrowed-cell behaviour
were rewritten to the fallback geometry.

New page: [standalone-cluster-fallback](standalone-cluster-fallback.md).
Updated [safe-presentation](safe-presentation.md) (the generalized
zero-width rule), [grapheme-highlight-expansion](grapheme-highlight-expansion.md)
(standalone clusters no longer borrow hosts; the fallback section and
test roster), [line-terminators-and-bom](line-terminators-and-bom.md)
(mid-line FEFF takes the fallback cell), [unit-tests](unit-tests.md)
(present/filebuffer/viewport/app catalogs incl. `fallback_test.go`),
[source-code](source-code.md) (`line.go`), and the index.
New files: `internal/filebuffer/fallback_test.go`;
`Notes/walkthroughs/043-05/` holds the walkthrough.
Sources: `Notes/tasks/043-combining-cluster-fallback-cell.md`,
`Notes/decisions/043-combining-cluster-fallback-cell.md`,
`Notes/PRD-vrg.md` (*Text, graphemes, and safe presentation*),
`internal/present/line.go`, `internal/filebuffer/fallback_test.go`,
`internal/app/cellmodel_test.go`.

## [2026-09-25] ingest | Issue #44 post-summary context regression coverage

Issue #44 (`Notes/tasks/044-post-summary-context-integrity-failure.md`)
added the dedicated regression net for the summary-is-final contract —
a test-only change on top of Issue #36's parser work, which owned
removing the `context` exemption in `Index.Add` and correcting the
contradictory lifecycle row. Any record after `summary`, `context`
included, is a stream-integrity failure carrying the sole
`record after summary` cause; pre-`summary` `context` records remain
ignored for match/lifecycle purposes.

`internal/searchindex/causes_test.go`'s `TestIntegrityCauses` gained
the dedicated row — an intact begin/match/end/`summary` stream then a
`context` record asserting exactly `{Kind: CauseRecordAfterSummary}`
with the retained stop unaffected — plus neighbouring rows proving
`context` before `begin` and before `summary` contributes no cause and
touches no lifecycle state. `lifecycle_test.go`'s former "context in
any position" row was renamed "context before the summary has no
lifecycle effect" to make the pre-`summary`-only scope explicit.
`internal/app/diagnostics_test.go` gained
`TestPostSummaryContextIsFatalIntegrity`: the same stream through
`searchDoneMsg` under a clean exit 0 is the fatal integrity outcome —
overlay over browse carrying exactly `record after summary`, dismissal
then `q` exiting 2, the identical line retained for stderr replay.

Updated [searchindex-records-and-stops](searchindex-records-and-stops.md)
(the positional `context` exemption),
[stream-integrity-fatal-diagnostics](stream-integrity-fatal-diagnostics.md)
(the Issue #44 coverage under Tests),
[error-overlay-and-fatal-outcomes](error-overlay-and-fatal-outcomes.md)
(the summary-is-final regression net under Tests),
[unit-tests](unit-tests.md) (the lifecycle/causes/diagnostics
catalogs), and the index.
New files: none — test-only change;
`Notes/walkthroughs/044-03/` holds the walkthrough.
Sources: `Notes/tasks/044-post-summary-context-integrity-failure.md`,
`Notes/PRD-vrg.md` (*Result index, records, and stream integrity*;
*Outcome and exit-status contract*),
`internal/searchindex/causes_test.go`,
`internal/searchindex/lifecycle_test.go`,
`internal/app/diagnostics_test.go`.

## [2026-09-25] ingest | Issue #45 test-hook build topology — `vrg_testhooks` variant

Issue #45 (`Notes/tasks/045-remove-test-hooks-from-production-binary.md`)
moved every test hook out of the production binary behind two
`//go:build` boundaries in `cmd/vrg`. `seams.go` (`!vrg_testhooks`)
holds the inert halves — `wireTestHooks` returns a bare context and
`runProgram` delegates directly to `tea.NewProgram(...).Run()` — while
`seams_testhooks.go` (`vrg_testhooks`) holds every env-var read, fifo
trigger, and watcher. The env names moved to the explicit hook
manifest: `VRG_TEST_REAP`, `VRG_TEST_GATE`, `VRG_TEST_COLLECT_ACK`
(formerly `VRG_TEST_REAP_FILE`/`VRG_TEST_GATE_FIFO`/`VRG_TEST_DIAG_ACK_FILE`),
the new `VRG_TEST_FAIL_TRIGGER`/`VRG_TEST_FAIL_DIAGNOSTIC` and
`VRG_TEST_DIAGNOSTIC_TRIGGER`/`VRG_TEST_DIAGNOSTIC_TEXT` injection
pairs, and the `VRG_TEST_RUN_FINAL_MODEL`/`VRG_TEST_RUN_ERROR`
program-runner controls that substitute the `Run()` return tuple at the
executable's real boundary for Issue #46. `Config` gained
`DiagInject`, forwarded onto the diagnostic channel by `injectDiags`.
`TestMain` now builds the binary under test with `-tags vrg_testhooks`,
so the whole suite exercises the hooked variant automatically.
`testhooks_test.go` proves both directions: the untagged artifact
ignores every manifest name and contains none of their strings, and the
tagged build answers every return-shape tuple and injection seam.
Fixture-owned variables (`VRG_CAPTURE_DIR`) are excluded from the
manifest by construction; Issues #46/#48 extend this mechanism rather
than adding production hooks.

Created [test-hook-build-topology](test-hook-build-topology.md).
Updated [cancellation-and-cleanup](cancellation-and-cleanup.md) (the
renamed, tagged seam list), [stderr-replay](stderr-replay.md)
(`VRG_TEST_COLLECT_ACK`), [source-code](source-code.md) (the seam
files, `runProgram`, `Config.DiagInject`), [unit-tests](unit-tests.md)
(the `TestMain` tagged build, renamed env vars, the `testhooks_test.go`
catalog), and the index.
Sources: `Notes/tasks/045-remove-test-hooks-from-production-binary.md`,
`Notes/PRD-vrg.md` (*Outcome and exit-status contract*; *Testing
Decisions*), `cmd/vrg/seams.go`, `cmd/vrg/seams_testhooks.go`,
`cmd/vrg/main.go`, `cmd/vrg/testhooks_test.go`,
`internal/app/search.go`.

## [2026-09-25] ingest | Issue #46 unified runtime-error shutdown and diagnostic replay

Issue #46 (`Notes/tasks/046-runtime-error-common-diagnostic-replay.md`)
routes every `program.Run()` return shape through one shutdown sequence
in `cmd/vrg/main.go`. `Config.OnCollect` (new `internal/app` seam,
wired to `Model.onCollect` invoked from `collect`) feeds the process
boundary's `diagSnapshot` as each diagnostic is collected, so session
diagnostics reach stderr independently of the final-model type
assertion — the nil/wrong-type model shapes that previously dropped the
collection and could exit 0. The ordered sequence: `Run()` returns with
the terminal restored → `sess.Cancel()` + `<-sess.Reaped()` → snapshot
replay (session diagnostics in collection order, then
`finalModelDiagnostic`'s invalid-final-model line when applicable, then
the runtime error exactly once). Every failing shape exits 2, extending
the controlled-failure convention to runtime errors. Tests:
`cmd/vrg/pty_returnshape_test.go` (`TestPTYRunReturnShapes`, the full
matrix on the real PTY lifecycle through the Issue #45
`VRG_TEST_RUN_*` runner seam) plus the updated
`TestTaggedRunnerSeamSelectsReturnShape` expectations; no new hook
names joined the manifest.

Created [runtime-error-shutdown](runtime-error-shutdown.md). Updated
[stderr-replay](stderr-replay.md) (the snapshot replacing the
model-carried replay), [cancellation-and-cleanup](cancellation-and-cleanup.md)
(the unified return-shape sequence and exit-2 extension),
[test-hook-build-topology](test-hook-build-topology.md) (Issue #46's
runner-seam consumption), [source-code](source-code.md) (`main.go`
shutdown contract, `Config.OnCollect`), [unit-tests](unit-tests.md)
(the `pty_returnshape_test.go` matrix, updated seam-test contract), and
the index.
Sources: `Notes/tasks/046-runtime-error-common-diagnostic-replay.md`,
`Notes/PRD-vrg.md` (*Outcome and exit-status contract*),
`cmd/vrg/main.go`, `cmd/vrg/pty_returnshape_test.go`,
`cmd/vrg/testhooks_test.go`, `internal/app/app.go`,
`internal/app/search.go`.

## [2026-09-25] ingest | Issue #47 single-line read-failure diagnostics

Issue #47 (`Notes/tasks/047-read-failure-single-line-filenames.md`)
fixes read-failure diagnostics on hostile filenames: the `loadDoneMsg`
error branch in `internal/app/app.go` now composes `cannot read
<present.Path(path)>: <reason>` where the reason comes from the new
`readReason` in `internal/app/browse.go` — a `*os.PathError` unwraps
through `errors.As` to its bare `Err` (the errno text carries no path)
instead of `PathError.Error()`, which embeds the raw resolved path; a
newline in the filename previously survived `present.Diagnostic` as a
real line boundary, splitting one failure into two diagnostic lines.
Non-path errors keep their own text. The construction is uniform —
initial load, `r` reload, and the failed-path re-entry retry all share
the branch — so one failed read is exactly one diagnostic line in the
overlay row set, `failLines`, the session collection, and the stderr
replay. Tests: `internal/app/readdiag_test.go` drives real
`os.ReadFile` failures (index the file, park the worker on `loadGate`,
remove it, release to a genuine `*os.PathError`) for filenames with
embedded newline, tab, invalid UTF-8, and ESC bytes at all three load
sites, plus a wrapped-PathError/plain-error unit pin.

Updated [read-failures](read-failures.md) (the sanitized-reason
construction and the new test file), [safe-presentation](safe-presentation.md)
(the Diagnostics contract — escaping the embedded filename is not
enough when the reason re-embeds the raw path),
[stderr-replay](stderr-replay.md) (the load-failure line's sanitized
reason), [source-code](source-code.md) (`readReason` on `browse.go`,
the `loadDoneMsg` error branch on `app.go`), [unit-tests](unit-tests.md)
(the `readdiag_test.go` block), and the index.
Sources: `Notes/tasks/047-read-failure-single-line-filenames.md`,
`Notes/PRD-vrg.md` (*Text, graphemes, and safe presentation*; *File
loading, cache, reload, and selection consistency*),
`internal/app/app.go`, `internal/app/browse.go`,
`internal/app/readdiag_test.go`.

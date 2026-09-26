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

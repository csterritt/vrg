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

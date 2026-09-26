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

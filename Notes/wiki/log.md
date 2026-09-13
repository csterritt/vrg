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

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

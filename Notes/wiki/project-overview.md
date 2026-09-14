# Project overview

`vrg` is a terminal UI for browsing ripgrep results, invoked as
`vrg [flags] pattern [root]` with the root defaulting to `.`. It runs
ripgrep, collects its JSON results, and presents a file list beside the
current file's contents with matches highlighted.

- **Specification**: `Notes/PRD-vrg.md` (revision 6)
- **Issues**: `Notes/issues/001`–`035`; **tasks**: `Notes/tasks/`
- **Architecture decisions**: `Notes/decisions/`

## Stack

- Go (`go 1.27.1`), module path `vrg`
- `github.com/jawher/mow.cli v1.2.0` for command-line parsing
- Bubble Tea v2 pinned for the TUI: `charm.land/bubbletea/v2 v2.0.9`; Bubbles and Lip Gloss are intentionally not dependencies because the implementation does not import them
- ripgrep 15.x is the reference search child (spawned in later issues)

## Layout

Six internal packages mirror the PRD Module Design: `cli`,
`searchindex`, `filebuffer`, `viewport`, `theme`, `app`, `docs` — all under
`internal/`, plus the thin entry point `cmd/vrg`. The user-facing
documentation lives in the repository-root `README.md` (Issue #34),
synchronized with the implementation by `internal/docs` tests. See
[documentation-sync](documentation-sync.md).

See [source-code.md](source-code.md) for the file catalog and
[unit-tests.md](unit-tests.md) for the test catalog.

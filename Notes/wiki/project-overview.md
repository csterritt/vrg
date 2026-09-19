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
- Charm v2 stack pinned for the TUI: `charm.land/bubbletea/v2 v2.0.9`,
  `charm.land/bubbles/v2 v2.2.1`, `charm.land/lipgloss/v2 v2.0.6`
- ripgrep 15.x is the reference search child (spawned since Issue #3)

## Layout

Six internal packages mirror the PRD Module Design: `cli`,
`searchindex`, `filebuffer`, `viewport`, `theme`, `app` — all under
`internal/`, plus the thin entry point `cmd/vrg`. `cli` and
`searchindex` cover Issues #1–3 (the CLI contract and the record
parser/navigation index); `app` carries Issues #3–5 (spawn/drain,
searching screen, cancellation/cleanup, and the two-pane browse view);
`filebuffer`, `viewport`, and `theme` gained their first Issue #5
implementations (prepared file loading, the top-of-file window seam,
and the initial Dark/Plain styles); `safepresentation` holds the Issue
#5 escaping core.

See [source-code.md](source-code.md) for the file catalog and
[unit-tests.md](unit-tests.md) for the test catalog.

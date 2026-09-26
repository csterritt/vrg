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
- `charm.land/bubbletea/v2 v2.0.9` for the TUI; Bubbles and Lip Gloss
  are not used (removed from the manifest under Issue #49) — the
  `theme` and `present` packages own presentation
- ripgrep 15.x is the reference search child, spawned with the
  protected `--json` argv from Issue #3 onward

## Layout

The internal packages mirror the PRD Module Design: `cli`,
`searchindex`, `filebuffer`, `viewport`, `theme`, `app`, plus
`present` — the shared safe-presentation utility every output sink
routes through — all under `internal/`, plus the thin entry point
`cmd/vrg`. Issues #1–#6 are live: invocation, flag allow-list, the
protected child argv, rg spawn with dual-pipe drainage,
cancellation/cleanup/terminal restoration, the `Searching…` state, the
`searchindex` record/stop model, the two-pane browse tracer — file
list, filename rule, guttered content with inverse-video matches,
async prepared-buffer loads — and the all-sink `present` utility
(`Path`/`LineOf`/`Diagnostic`) with its extensible sink-safety table.

See [source-code.md](source-code.md) for the file catalog and
[unit-tests.md](unit-tests.md) for the test catalog.

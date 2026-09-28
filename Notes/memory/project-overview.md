---
uuid: 1f1bb46e-4f0c-61a0-8876-adcb3a225448
created: '2026-09-28T14:14:18Z'
updated: '2026-09-28T14:14:18Z'
title: vrg project overview
summary: Repository layout, module facts, VCS, and implementation status of the vrg
  codebase.
---
# vrg project overview

`vrg` is invoked as `vrg [flags] pattern [root]`: it runs ripgrep, collects JSON
results, and presents matched files in a terminal UI (file list left, file
content right, matches highlighted). Spec: `Notes/PRD-vrg.md` (revision 6).

## Module facts

- Module `vrg`, Go 1.27.1.
- One direct dependency: `github.com/jawher/mow.cli` v1.2.0 (CLI parsing).
- `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`, `charm.land/bubbles/v2`
  and their transitive deps are in `go.mod` as indirect requirements reserved
  for the upcoming TUI issues.

## Layout

- `cmd/vrg/` — entry point; sole owner of exit statuses and output streams.
- `internal/cli/` — argument parsing and generated help (implemented).
- `internal/app/` — lifecycle, input, async work, overlays (skeleton).
- `internal/filebuffer/` — file loading, display lines, highlights (skeleton).
- `internal/searchindex/` — ripgrep data, stream integrity, match cursor (skeleton).
- `internal/theme/` — colour scheme and styles (skeleton).
- `internal/viewport/` — reading position and row visibility (skeleton).
- `scripts/` — repo tooling; `verify.sh` is the permanent verification gate.
- `Notes/` — PRD, issues, tasks, skills (agent rules), walkthroughs, memory.

## Status and VCS

Issue 1 (CLI positionals and root) is implemented; `main.go` still prints a
`search stub:` line instead of running ripgrep. Later issues (2–50 in
`Notes/issues/`) fill in the rest. The repo is versioned with Jujutsu (`.jj`),
not plain git.

Sources: `/home/chris/vrg/go.mod`, `cmd/vrg/main.go`, `internal/*/doc.go`,
`Notes/issues/`, `scripts/verify.sh`, `jj log`

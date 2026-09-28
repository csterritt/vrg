---
uuid: 1f1bb46f-632a-64cb-bdb2-585b13c3cccb
created: '2026-09-28T14:14:47Z'
updated: '2026-09-28T14:14:47Z'
title: cmd/vrg entry point
summary: 'main.go: the sole process boundary mapping cli result kinds to exit statuses
  and streams, plus its subprocess tests.'
---
# cmd/vrg entry point

`cmd/vrg/main.go` is deliberately thin. `main()` calls
`run(os.Args[1:], os.Stdout, os.Stderr)` and exits with its return value.

## run() — the process boundary

`run` is the only place that chooses exit statuses and output streams; the
`internal/` library never exits the process and never writes to stdout/stderr.
It maps `cli.Parse` result kinds:

- `KindHelp` → exit 0 (help was already written to stdout by `Parse`).
- `KindSearch` → prints `search stub: pattern=<escaped> root=<escaped>` to
  stdout, exit 0. This stub proves the end-to-end slice; later issues replace
  it with the real search and TUI.
- default (`KindUsageError`) → sanitized one-line diagnostic plus
  `cli.HelpText()` to stderr, exit 2.

Stub substitutions go through `cli.Escape`, so hostile operand bytes can never
reach the terminal raw.

## Tests

`main_test.go` builds the real binary once per run in `TestMain` (`binPath`)
and asserts at the true subprocess boundary (`runVrg`, `runVrgIn`):

- Every help spelling: exactly one `Usage:` copy on stdout, exit 0, empty
  stderr, no stub output, no terminal control bytes.
- Help runs without ripgrep on PATH and never execs an `rg` binary.
- A hostile `argv[0]` containing escape sequences cannot reach output — the
  app name is the fixed constant `"vrg"`.
- Usage errors exit 2, keep stdout empty, and emit `vrg: <diagnostic>` plus
  one usage block — never the library's `Error:`/`incorrect usage` text.
- Root acceptance: dirs, regular files, symlinks to either; `-` (stdin)
  rejected with a diagnostic naming stdin; `./-` still addresses a real file.

Sources: `/home/chris/vrg/cmd/vrg/main.go`,
`/home/chris/vrg/cmd/vrg/main_test.go`

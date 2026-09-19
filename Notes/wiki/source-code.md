# Source code

Catalog of Go source under `cmd/` and `internal/`. Module path: `vrg`.

## cmd/

- `cmd/vrg/main.go` — thin process boundary. `run` calls `cli.Parse` with
  `os.Stat` injected and maps the explicit result kind to stream/status:
  help → exit 0 (help already on stdout), usage error → sanitized
  diagnostic + usage on stderr, exit 2, search → `app.Run` with the
  protected child argv and the invocation working directory. It also
  wires the `VRG_TEST_*` seam env vars (`VRG_TEST_GATE`,
  `VRG_TEST_COLLECT_ACK`, `VRG_TEST_REAP`, `VRG_TEST_FAIL`) into
  `app.Option`s — see [search-collection.md](search-collection.md) and
  [cancellation-cleanup.md](cancellation-cleanup.md).

## internal/cli

- `internal/cli/cli.go` — the CLI module and sole `mow.cli` v1.2.0
  adapter. Contains the shared `optionDecls`/`argDecls` table (parser
  config + raw-token recognition + ordered flag records + generated
  help), the ordered `scanArgs` preflight with combined-short expansion
  and cumulative unrestricted counting, `renderHelp`, `checkRoot`, the
  `Escape` sanitizer, and the `Result`/`Kind`/`ErrorKind`/`Env` contract
  including `Result.ChildArgs` (the protected `rg` argv). See
  [cli-foundation.md](cli-foundation.md) and
  [cli-flags-child-argv.md](cli-flags-child-argv.md).

## internal/searchindex

- `internal/searchindex/record.go` — the per-record JSON parser and the
  Issue #3 schema matrix: `Kind` classification (`begin`/`match`/`end`/
  `summary`/`context`/`KindUnknown`/`KindMalformed`), `text`/base64
  `bytes` decoding, required-field and range validation.
- `internal/searchindex/index.go` — the navigation index: `Index.Feed` /
  `Prepare` / `Build`, same-path/same-line `Stop` merging, submatch
  `(start, end)` ordering, union `Highlights`, unsigned raw-path ordering,
  and working-directory path resolution without canonicalization.
- `internal/searchindex/doc.go` — package comment.

## internal/app

- `internal/app/rg.go` — the child-process seam: `spawn` runs `rg` from
  `PATH` with `cmd.Dir` set to the invocation working directory and
  drains both stdout and stderr concurrently for the child's whole
  lifetime; `Child`/`Result`/`StartFunc` are the boundary types, with
  `Child.Terminate` and an idempotent `Wait` backing the cleanup path.
- `internal/app/app.go` — the Bubble Tea model and `app.Run`: the
  "Searching…" state covering collection and post-exit index
  preparation, the `WithGate`/`WithCollectAck` test seams, the interim
  `N files, M matched lines` summary with `q` → exit 0, the sanitized
  start-failure diagnostic with exit 2 before the TUI, and the Issue #4
  surface: `ctrl+c`/`q` cancellation to 130, the `quitCmd`/`reapChild`
  cleanup boundary, `WithFailFunc`/`WithReapReport`, the `quitting`
  discard of late completions, alt-screen views, `ErrInterrupted` → 130,
  and `writeFailureDiag` (the single post-restoration stderr writer) →
  exit 2. See [cancellation-cleanup.md](cancellation-cleanup.md).
- `internal/app/doc.go` — package comment.

## Package boundaries awaiting their issues

Each is a documented empty package mirroring PRD Module Design:

- `internal/filebuffer` — per-file loading/classification into safe
  display-ready lines and validated highlights.
- `internal/viewport` — logical reading position and rendered-row
  visibility.
- `internal/theme` — active colour scheme and styles.

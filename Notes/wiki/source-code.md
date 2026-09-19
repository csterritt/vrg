# Source code

Catalog of Go source under `cmd/` and `internal/`. Module path: `vrg`.

## cmd/

- `cmd/vrg/main.go` — thin process boundary. `run` calls `cli.Parse` with
  `os.Stat` injected and maps the explicit result kind to stream/status:
  help → exit 0 (help already on stdout), search → escaped stub line
  (`search stub: argv=rg …`, the exact protected child argv), exit 0,
  usage error → sanitized diagnostic on stderr, exit 2.

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

## Package boundaries awaiting their issues

Each is a documented empty package mirroring PRD Module Design:

- `internal/searchindex` — parsed result data, stream integrity,
  exclusions, circular matched-line cursor.
- `internal/filebuffer` — per-file loading/classification into safe
  display-ready lines and validated highlights.
- `internal/viewport` — logical reading position and rendered-row
  visibility.
- `internal/theme` — active colour scheme and styles.
- `internal/app` — lifecycle/input/async/overlay/cleanup coordination
  (Bubble Tea model).

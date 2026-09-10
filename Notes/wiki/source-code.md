# Source code

Catalog of Go source under `cmd/` and `internal/`. Module path: `vrg`.

## cmd/

- `cmd/vrg/main.go` — thin process boundary. `run` calls `cli.Parse` with
  `os.Stat` injected and maps the explicit result kind to stream/status:
  help → exit 0 (help already on stdout), search → escaped stub line
  (`search stub: pattern=… root=…`), exit 0, usage error → sanitized
  diagnostic on stderr, exit 2.

## internal/cli

- `internal/cli/cli.go` — the Issue #1 CLI foundation and the sole
  `mow.cli` v1.2.0 adapter. Contains the shared `optionDecls`/`argDecls`
  table (parser config + raw-token recognition + generated help), the
  ordered `scanArgs` preflight, `renderHelp`, `checkRoot`, the `Escape`
  sanitizer, and the `Result`/`Kind`/`ErrorKind`/`Env` contract.
  See [cli-foundation.md](cli-foundation.md).

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

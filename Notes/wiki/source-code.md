# Source code

Catalog of Go source under `cmd/` and `internal/`. Module path: `vrg`.

## cmd/

- `cmd/vrg/main.go` — thin process boundary. `run` calls `cli.Parse` with
  `os.Stat` injected and maps the explicit result kind to stream/status:
  help → exit 0 (help already on stdout), search → escaped stub line
  printing the child argv (`search stub: rg --json --no-config … --
  pattern root`), exit 0, usage error → sanitized diagnostic plus the
  generated usage block on stderr, exit 2.

## internal/cli

- `internal/cli/cli.go` — the Issue #1 CLI foundation and the sole
  `mow.cli` v1.2.0 adapter, extended by Issue #2. Contains the shared
  `optionDecls`/`argDecls` table (parser config + raw-token recognition +
  generated help; the search-flag allow-list lives only here), the
  ordered `scanArgs`/`scanOption`/`expandOption` preflight (encounter-
  order flag records, combined-short expansion, cumulative `-u` cap,
  lexical `=` rejection), `renderHelp`, `checkRoot`, the `Escape`
  sanitizer, and the `Result`/`Kind`/`ErrorKind`/`Env` contract —
  `Result.ChildArgv` is the exact rg argument vector. See
  [cli-foundation.md](cli-foundation.md) and
  [cli-flags-and-child-argv.md](cli-flags-and-child-argv.md).

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

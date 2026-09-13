# Source code

Catalog of Go source under `cmd/` and `internal/`. Module path: `vrg`.

## cmd/

- `cmd/vrg/main.go` — thin process boundary. `run` calls `cli.Parse` with
  `os.Stat` injected and maps the explicit result kind to stream/status:
  help → exit 0 (help already on stdout), search → `runSearch` (Issue #3:
  starts ripgrep with the protected child argv, runs the Bubble Tea
  program, propagates exit codes; Issue #4: centralized cleanup kills the
  child process group and reaps via `proc.Cleanup`, single
  post-restoration stderr writer for diagnostics, test seams for reap
  evidence/gate/failure injection via `VRG_TEST_*` env vars), usage error
  → sanitized diagnostic on stderr, exit 2. See
  [search-collection-path](search-collection-path.md).

## internal/cli

- `internal/cli/cli.go` — the Issue #1 CLI foundation, the Issue #2
  search-flag allow-list and child-argv contract, and the sole `mow.cli`
  v1.2.0 adapter. Contains the shared `optionDecls`/`argDecls` table
  (parser config + raw-token recognition + generated help), the ordered
  `scanArgs`/`scanOption` preflight with ordered flag records and the
  cumulative `-u` count, `renderHelp`, `checkRoot`, the `Escape`
  sanitizer, and the `Result`/`Kind`/`ErrorKind`/`Env` contract.
  See [cli-foundation.md](cli-foundation.md) and
  [cli-flag-forwarding.md](cli-flag-forwarding.md).

## internal/searchindex

- `internal/searchindex/searchindex.go` — Issue #3: parses ripgrep
  `--json` stream events (`begin`, `match`, `end`, `summary`,
  `context`) into a navigable `Index` of `Stop` entries. Supports both
  `text` and `bytes` encodings with identical logical values, retains
  raw non-UTF-8 path and line bytes, merges same-line matches, sorts and
  merges overlapping/adjacent submatch ranges, resolves relative paths
  against the working directory without canonicalization, and orders
  files by unsigned byte ordering of raw path bytes. See
  [search-collection-path](search-collection-path.md).

## internal/app

- `internal/app/app.go` — Issue #3: the Bubble Tea model owning the
  search lifecycle. States: searching, summary, start-failed. Issue #4
  added cancelled and failed states, cancellation (q while searching,
  ctrl+c anywhere) with child termination/reaping, late-completion
  rejection, an injectable controlled-failure hook
  (`WithFailureSignal`), a `Process` type with lifecycle channels
  (`done`, `cancel`) and `Cleanup`/`Cancel` methods, alt-screen-enabled
  views, and a cancellable gate select. `Init` returns a batch of
  `collectResults` and `watchFailure`. `collectResults` drains stdout
  and stderr concurrently, parses stdout into a `searchindex.Builder`,
  waits for ripgrep to exit (reaping via `Wait` with optional `OnReap`
  callback), holds at the cancellable gate if set, builds the index, and
  returns a `SearchCompleteMsg`. `Update` handles completion, failure,
  controlled failure, key (q, Ctrl-C, escape), and resize. `View`
  renders the searching and summary screens with alt screen enabled.
  Issue #5 added `StateBrowse` with a two-pane browse view (file list +
  content panel), `SearchCompleteMsg.Index`, `FileLoadCompleteMsg`,
  `WithTheme`/`WithFileLoader`/`WithFileLoadGate` options, async file
  loading off the update path, inverse-video highlights, the
  safe-presentation core for all visible strings, and browse `q` exit 0
  through the Issue #4 cleanup path. See
  [search-collection-path](search-collection-path.md) and
  [browse-tracer](browse-tracer.md).

## internal/safepresentation

- `internal/safepresentation/safepresentation.go` — Issue #5: the
  focused safe-presentation core for paths and content. `EscapePath`
  escapes raw path bytes for safe single-line display (backslash
  escapes for `\n`/`\r`/`\t`/`\\`, `\xNN` for invalid UTF-8, caret
  notation for C0/DEL, `\u00XX` for C1, valid printable Unicode
  preserved). `EscapeContent` escapes raw content bytes for safe display
  (U+FFFD for invalid UTF-8, caret notation for C0/DEL, `\u00XX` for C1,
  LF/CRLF as terminators, `^M` for standalone CR, `→` for tab). Both
  expose `ByteCells` byte→cell mappings so highlight rendering can cover
  all cells of an escaped form. Issue #6 will unify this with the
  Issue #1 `cli.Escape` escaper. See
  [browse-tracer](browse-tracer.md).

## internal/filebuffer

- `internal/filebuffer/filebuffer.go` — Issue #5: loads, decodes, and
  maps a file's bytes into a display-ready `Buffer`. `Load(path, stops)`
  reads the file, splits lines (LF/CRLF terminators, standalone CR is
  content), escapes each line through `safepresentation.EscapeContent`,
  and maps `Stop.Submatches` to display cell ranges via the byte→cell
  map. `Buffer` carries `Lines`, `LineCount`, and `GutterWidth`. `Line`
  carries `Number`, `Display`, `ByteCells`, and `Highlights`. The
  completion message carries the fully prepared buffer so `Update` does
  no full-file work. See [browse-tracer](browse-tracer.md).

## internal/viewport

- `internal/viewport/viewport.go` — Issue #5: minimal scrollable content
  view seam. `Viewport` with `Lines`, `Height`, and `Offset` (always 0
  for Issue #5). `Visible()` returns lines at the current offset. Later
  issues add scrolling, cursor tracking, and reveal-on-match.

## internal/theme

- `internal/theme/theme.go` — Issue #5: visual style configuration.
  `New()` returns a default theme; `NoStyle()` disables all ANSI
  sequences for sink-safety testing. `Underline(s)` and `Reverse(s)`
  wrap strings in ANSI sequences (no-op with no-style theme). See
  [browse-tracer](browse-tracer.md).

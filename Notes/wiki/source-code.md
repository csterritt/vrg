# Source code

Catalog of Go source under `cmd/` and `internal/`. Module path: `vrg`.

## cmd/

- `cmd/vrg/main.go` — thin process boundary. `run` calls `cli.Parse` with
  `os.Stat` injected and maps the explicit result kind to stream/status:
  help → exit 0 (help already on stdout), usage error → sanitized
  diagnostic + usage on stderr, exit 2, search → `app.Run` with the
  protected child argv and the invocation working directory. It also
  wires the `VRG_TEST_*` seam env vars (`VRG_TEST_GATE`,
  `VRG_TEST_COLLECT_ACK`, `VRG_TEST_REAP`, `VRG_TEST_FAIL`,
  `VRG_TEST_LOAD_GATE`) into `app.Option`s — see
  [search-collection.md](search-collection.md),
  [cancellation-cleanup.md](cancellation-cleanup.md), and
  [browse-tracer.md](browse-tracer.md).

## internal/cli

- `internal/cli/cli.go` — the CLI module and sole `mow.cli` v1.2.0
  adapter. Contains the shared `optionDecls`/`argDecls` table (parser
  config + raw-token recognition + ordered flag records + generated
  help), the ordered `scanArgs` preflight with combined-short expansion
  and cumulative unrestricted counting, `renderHelp`, `checkRoot`, and
  the `Result`/`Kind`/`ErrorKind`/`Env` contract including
  `Result.ChildArgs` (the protected `rg` argv). Sanitization routes
  through `internal/safepresentation` (the Issue #1 `Escape` is
  replaced): usage diagnostics embed `EscapePath`-escaped operands and
  pass through `EscapeDiagnostic`, as does generated help. See
  [cli-foundation.md](cli-foundation.md),
  [cli-flags-child-argv.md](cli-flags-child-argv.md), and
  [safe-presentation.md](safe-presentation.md).

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
  preparation, the `WithGate`/`WithCollectAck` test seams, the
  `stateBrowse` transition on completion with `q` → exit 0, the
  sanitized start-failure diagnostic with exit 2 before the TUI, and the
  Issue #4 surface: `ctrl+c`/`q` cancellation to 130, the
  `quitCmd`/`reapChild` cleanup boundary, `WithFailFunc`/
  `WithReapReport`, the `quitting` discard of late completions,
  alt-screen views, `ErrInterrupted` → 130, `writeFailureDiag` (the
  single post-restoration stderr writer) → exit 2, and the Issue #7
  `c` key toggling `m.theme` between the dark and light schemes. See
  [cancellation-cleanup.md](cancellation-cleanup.md) and
  [theme.md](theme.md).
- `internal/app/browse.go` — the Issue #5 browse view: async
  `filebuffer.Load` commands gated by `WithLoadGate`, the
  `loading`/`bufs`/`failed` caches keyed by raw path, `fileLoadedMsg`,
  the full-width filename rule, the fixed-width file list with the
  current entry underlined, the right-justified gutter, inverse-video
  match runs, and the "Loading…"/"(unreadable)" placeholders — all
  styled through `internal/theme` since Issue #7 (base-wrapped frame,
  true-inverse matches, underlined current match and current file). See
  [browse-tracer.md](browse-tracer.md) and [theme.md](theme.md).
- `internal/app/doc.go` — package comment.

## internal/filebuffer, internal/viewport, internal/theme, internal/safepresentation

- `internal/filebuffer/filebuffer.go` — `Load` reads the file by raw
  path bytes, splits LF/CRLF, maps each line through
  `safepresentation.MapContent`, and produces `Buffer`/`Line` records
  with `GutterWidth`, source `Raw` bytes, and `Highlights` mapped to
  display cells. See [browse-tracer.md](browse-tracer.md).
- `internal/viewport/viewport.go` — the minimal top-of-file vertical
  window seam (`Viewport`, `Visible`, `Top`); scrolling and reveal are
  later issues.
- `internal/theme/theme.go` — the active scheme's style set: `Dark()`
  (white on black, initially active), `Light()` (black on white), the
  pure `Toggled` flip behind the `c` key, `Plain()` (the no-style
  composition path for sink-safety tests), and the decorators `Base`,
  `Gutter`, `Match` (true-inverse colours), `CurrentMatch` (inverse +
  underline), `Indicator`, `FilenameRule`, `FileList`, `CurrentFile`
  (base + underline), and `Overlay` (base colours inside a plain
  single-line border). See [theme.md](theme.md).
- `internal/safepresentation/safepresentation.go` — `EscapePath`,
  `MapContent`, `Mapped`/`Cell`/`CellsCovering` byte→cell maps.
- `internal/safepresentation/diagnostic.go` — `EscapeDiagnostic`:
  real line boundaries preserved (CRLF normalizes to LF), tabs expand
  to eight-column stops, other controls escaped; embedded external
  strings are `EscapePath`-escaped first.
- `internal/safepresentation/cellwidth.go` — `CellWidth` and the
  package-internal rune decoders (Issue #39 boundary).
- `internal/safepresentation/sinktest/sinktest.go` — test-support
  package (imported only by `_test.go` files): the shared hostile
  `Fixtures`, the `Sink` row type, the `AssertRawOutput`/
  `AssertPayloadNotEscaped` raw-output assertions, and the `Run`
  table driver. See [safe-presentation.md](safe-presentation.md).

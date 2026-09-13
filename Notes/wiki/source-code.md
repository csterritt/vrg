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
  cumulative `-u` count, `renderHelp`, `checkRoot`, and the `Escape`
  sanitizer (Issue #6: now a one-line wrapper around
  `safepresentation.EscapePath`, eliminating the duplicated escaper),
  and the `Result`/`Kind`/`ErrorKind`/`Env` contract.
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
  through the Issue #4 cleanup path. Issue #6 replaced the local
  `sanitizeDiagnostic` (which only mapped ESC and C1 CSI to spaces)
  with a delegate to `safepresentation.EscapeDiagnostic`, satisfying
  the full diagnostic contract: line preservation, tab expansion, and
  complete control escaping. See
  [search-collection-path](search-collection-path.md) and
  [browse-tracer](browse-tracer.md).

## internal/safepresentation

- `internal/safepresentation/safepresentation.go` — the shared
  safe-presentation utility for every output sink. Issue #5 landed the
  path and content rules; Issue #6 added the diagnostic escaper and
  unified `cli.Escape` onto `EscapePath`. `EscapePath(raw []byte)
  PathDisplay` escapes raw path bytes for safe single-line display
  (backslash escapes for `\n`/`\r`/`\t`/`\\`, `\xNN` for invalid
  UTF-8, caret notation for C0/DEL, `\u00XX` for C1, valid printable
  Unicode preserved). `EscapeContent(raw []byte) ContentDisplay`
  escapes raw content bytes for safe display (U+FFFD for invalid UTF-8,
  caret notation for C0/DEL, `\u00XX` for C1, LF/CRLF as terminators,
  `^M` for standalone CR, `→` for tab). `EscapeDiagnostic(raw []byte)
  string` escapes raw diagnostic bytes for safe display while
  preserving real line boundaries (LF preserved, CRLF normalized to LF,
  tabs expanded to eight-column stops, other controls escaped).
  `PathDisplay`/`ContentDisplay` expose `ByteCells` byte→cell mappings
  so highlight rendering can cover all cells of an escaped form. See
  [safe-presentation](safe-presentation.md) and
  [browse-tracer](browse-tracer.md).

## internal/sinkfixtures

- `internal/sinkfixtures/sinkfixtures.go` — Issue #6: the shared
  hostile-fixture set and sink-safety assertion helpers. `Fixtures` is
  the shared slice covering OSC, CSI, C0, C1, DEL, standalone CR,
  invalid UTF-8, and embedded filename newline. Each fixture has a
  `Name`, `Raw`, and `Payload`. `NoControlBytes` asserts no C0/DEL
  survive (excluding newlines). `NoDangerousControls` asserts no
  dangerous C0/DEL survive (excluding newlines and tabs, for fixed-text
  sinks like generated help). `NoPayloadAfterESC` is the styled
  assertion: the fixture's payload must never appear immediately after
  an unescaped ESC. See [safe-presentation](safe-presentation.md).

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
  Issue #7: expanded to own the active colour scheme and full PRD
  style set. `New()` returns a theme with the dark scheme active
  (white on black); `NoStyle()` disables all ANSI sequences for
  sink-safety testing. `Scheme()` reports the active scheme;
  `Toggle()` flips between dark and light with no persistence.
  `Base(s)` wraps in the base colour pair; `Match(s)` wraps in the
  true-inverse match colours; `CurrentMatch(s)` adds underline;
  `Indicator(s)` uses the inverse style; `Underline(s)` wraps in SGR
  4; `Gutter(s)` and `FileList(s)` use base colours;
  `FilenameRule(name)` embeds the name in a base-coloured horizontal
  rule; `Overlay(s)` wraps with base colours and a plain single-line
  border. Match, CurrentMatch, Indicator, and Underline restore the
  base colours after the styled span. See
  [theme-module](theme-module.md) and [browse-tracer](browse-tracer.md).

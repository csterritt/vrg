# Source code

Catalog of Go source under `cmd/` and `internal/`. Module path: `vrg`.

## cmd/

- `cmd/vrg/main.go` — thin process boundary. `run` calls `cli.Parse` with
  `os.Stat` injected and maps the explicit result kind to stream/status:
  help → exit 0 (help already on stdout), search → `runSearch` starts an
  `app.Session` and runs the Bubble Tea program (`WithInput(os.Stdin)`,
  `WithWindowSize(80, 24)` fallback for piped output). After `Run`
  returns, every controlled exit funnels through one cleanup boundary:
  `sess.Cancel()` terminates a still-running child and `<-sess.Reaped()`
  waits for its reap. Status selection: `tea.ErrInterrupted` → 130,
  other `Run` errors → sanitized `vrg:` diagnostic on stderr after
  terminal restoration and exit 2, otherwise the final model's
  `ExitCode` (0 summary quit, 130 cancellation); rg start failure →
  sanitized diagnostic, exit 2, no TUI; usage error → sanitized
  diagnostic plus the generated usage block, exit 2.
- `cmd/vrg/hooks.go` — the env-var test seams applied to `app.Config`:
  `VRG_TEST_REAP_FILE` (reap-evidence side channel → `ReapReport`),
  `VRG_TEST_GATE_FIFO` (boundary `PrepareGate`), `VRG_TEST_FAIL_FIFO`
  (controlled-failure hook → `Program` context cancellation). No-op
  when unset.

## internal/app

- `internal/app/search.go` — the subprocess seam: `Config` (rg
  executable, protected argv, invocation working directory, `Drained`/
  `PrepareGate`/`ReapReport` test hooks), `Start` (spawns
  `exec.CommandContext` in the working directory, fails synchronously
  before the TUI, returns a `Session` owning the model plus `Cancel`/
  `Reaped`), `collect` (concurrent stdout/stderr drainage for the whole
  child lifetime, then `Wait` with reap reporting, then cancellation-
  aware gated index preparation — a cancelled gate abandons the build),
  and `prepareIndex` (decode + index build). See
  [search-spawn-and-searching-screen.md](search-spawn-and-searching-screen.md)
  and [cancellation-and-cleanup.md](cancellation-and-cleanup.md).
- `internal/app/app.go` — the Bubble Tea `Model`: `phaseSearching` renders
  `Searching…` until the prepared index arrives (spanning post-exit
  preparation), `phaseBrowse` renders the two-pane browse view and `q`
  quits with `ExitCode` 0; `ctrl+c` in any state or `q` while searching
  cancels (kills the child context, `ExitCode` 130), `Esc` is a no-op,
  and once `quit` is set `Update` discards all messages so a late
  completion cannot revive a cancelled UI; every view sets `AltScreen`
  for the exit restoration sequence; resize handled in any state.
  Issue #5 state: `stops`/`files`/`cursor`, the `bufs`/`loading`/
  `failed` buffer maps, `theme`, `vp`, and the `loadGate` test seam.
- `internal/app/browse.go` — the Issue #5 browse composition:
  `loadDoneMsg` (the worker's prepared-buffer completion keyed by raw
  path), `ensureLoad`/`loadCmd` (one async load per file; read, split,
  escape, and map all off the update path, behind `loadGate` in tests),
  `renderBrowse` (raw-path-ordered file list with underlined current
  entry scrolled into view, filename rule, `Loading…`/`(unreadable)`
  placeholders), `filenameRule`, `contentRow` (right-justified gutter +
  two spaces), and `renderCells` (inverse-video spans over escaped
  cells, marker spans, clip-edge wide clusters). See
  [browse-tracer.md](browse-tracer.md).

## internal/searchindex

- `internal/searchindex/record.go` — `DecodeRecord` per-record schema
  validation for the five rg JSON events (`begin`/`match`/`end`/
  `summary`/`context`) plus `KindUnknown`; the `{"text"}`/`{"bytes"}`
  value union; `ErrMalformed` range/required-field errors. See
  [searchindex-records-and-stops.md](searchindex-records-and-stops.md).
- `internal/searchindex/index.go` — the `Index`: match records merge
  into navigation stops keyed by (raw path bytes, line), submatches
  sort by `(start, end)`, `Prepare` sorts stops by unsigned raw path
  bytes then line and computes union `Highlights`; `ResolvedPath` joins
  relative paths onto the working directory without canonicalization.

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

## internal/filebuffer

- `internal/filebuffer/present.go` — the Issue #5 safe-presentation
  core: `EscapePath` for path bytes (`\n`/`\r`/`\t` forms, `\\`,
  `\xNN` invalid UTF-8, caret notation for C0/DEL, `\uXXXX` for C1,
  printable Unicode preserved) and `presentLine`/`presented` for
  content lines (U+FFFD for invalid UTF-8, caret notation, `\uXXXX`,
  LF/CRLF never displayed, standalone CR → `^M`, provisional
  single-cell `→` tab pending Issue #16) with per-byte `lo`/`hi`
  byte→cell maps and `Span` range→cell mapping including zero-width
  markers. Grapheme-aware via `x/ansi`. See
  [browse-tracer.md](browse-tracer.md).
- `internal/filebuffer/filebuffer.go` — `Buffer`, one file's prepared
  display-ready content: `Load` reads, splits, escapes, and maps the
  file (all inside the worker command, never on `Update`), validates
  each stop's submatches against the line's raw bytes, and exposes
  `LineCount`, `GutterWidth`/`GutterDigits` (largest line number's
  digit width + two spaces, minimum one slot), `Text`, `Cells`, and
  `Spans`.

## internal/viewport

- `internal/viewport/viewport.go` — the minimal Issue #5 seam:
  content dimensions plus `Range`, the visible window clamped to the
  loaded line count. Top-of-file only; scrolling/reveal are Issues
  12–19.

## internal/theme

- `internal/theme/theme.go` — the minimal Issue #5 seam: `Dark` emits
  real SGR (inverse `\x1b[7m`, underline `\x1b[4m`); `Plain` is the
  no-style composition path whose identity styles let sink-safety
  tests assert no escape bytes may legitimately appear.

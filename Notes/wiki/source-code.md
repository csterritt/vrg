# Source code

Catalog of Go source under `cmd/` and `internal/`. Module path: `vrg`.

## cmd/

- `cmd/vrg/main.go` — thin process boundary. `run` calls `cli.Parse` with
  `os.Stat` injected and maps the explicit result kind to stream/status:
  help → exit 0 (help already on stdout), search → `runSearch` starts an
  `app.Session` and runs the Bubble Tea program (`WithInput(os.Stdin)`,
  `WithWindowSize(80, 24)` fallback for piped output). After `Run`
  returns, every controlled exit funnels through one cleanup boundary:
  `sess.Cancel()` terminates a still-running child, `<-sess.Reaped()`
  waits for its reap, then `model.ReplayTo(stderr)` writes the session
  diagnostic collection — every diagnostic the final model processed —
  to stderr exactly once, after terminal restoration (Issue #11).
  Status selection: `tea.ErrInterrupted` → 130, other `Run` errors →
  the `vrg:` diagnostic joins the collection via `CollectDiagnostic`
  before `ReplayTo` (no separate direct write) and exit 2, otherwise the
  final model's
  `ExitCode` (the fixed search status — 0/1/2 per the Issue #9 outcome
  table — or 130 cancellation); rg start failure →
  sanitized diagnostic, exit 2, no TUI; usage error → sanitized
  diagnostic plus the generated usage block, exit 2. Boundary error
  text renders through `present.Diagnostic` since Issue #6.
- `cmd/vrg/hooks.go` — the env-var test seams applied to `app.Config`:
  `VRG_TEST_REAP_FILE` (reap-evidence side channel → `ReapReport`),
  `VRG_TEST_GATE_FIFO` (boundary `PrepareGate`), `VRG_TEST_FAIL_FIFO`
  (controlled-failure hook → `Program` context cancellation), and —
  Issue #11 — `VRG_TEST_DIAG_ACK_FILE` (one acknowledgement line per
  collected diagnostic → `DiagAck`). No-op
  when unset.

## internal/app

- `internal/app/search.go` — the subprocess seam: `Config` (rg
  executable, protected argv, invocation working directory, `Drained`/
  `PrepareGate`/`ReapReport` test hooks, and — Issue #11 — `DiagAck`,
  the collection-acknowledgement side channel), `Start` (spawns
  `exec.CommandContext` in the working directory, fails synchronously
  before the TUI, returns a `Session` owning the model plus `Cancel`/
  `Reaped`), `collect` (concurrent stdout/stderr drainage for the whole
  child lifetime, then `Wait` with reap reporting, then cancellation-
  aware gated index preparation — a cancelled gate abandons the build),
  and `prepareIndex` (feeds the stream through `Index.Feed` so decoding
  and lifecycle validation happen inside the index, then `Prepare`).
  Issue #11: the stderr drain also forwards each line to the model as a
  `diagMsg` over an unbuffered channel — pairing every send with the
  model's event wait so the completion can never overtake a diagnostic —
  while cancellation stops delivery and keeps draining.
  The delivered `searchDoneMsg` carries the index, captured stderr, and
  the wait error the model's outcome decision consumes. See
  [search-spawn-and-searching-screen.md](search-spawn-and-searching-screen.md),
  [cancellation-and-cleanup.md](cancellation-and-cleanup.md), and
  [stderr-replay.md](stderr-replay.md).
- `internal/app/app.go` — the Bubble Tea `Model`: `phaseSearching` renders
  `Searching…` until the prepared index arrives (spanning post-exit
  preparation); a done message resolves the Issue #9 `decideOutcome`
  decision — `phaseBrowse` (two-pane browse, load command issued),
  `phaseNoResults` (centred empty screen), or `phaseFatal` (no
  underlying screen) — fixes `ExitCode` once, and opens the diagnostics
  `overlay` whenever the completion carries diagnostics; an open
  overlay owns the keyboard via `overlayKey` (`up`/`down` scroll,
  `q`/`Esc` dismiss — quitting outright under `phaseFatal` — `ctrl+c`
  cancels, other keys ignored); `ctrl+c` in any state or `q` while
  searching cancels (kills the child context, `ExitCode` 130), `Esc` is
  a base-state no-op, `c` in browse toggles the theme between its dark
  and light schemes (Issue #7), `n`/`p` in browse step the matched-line
  cursor through `navigate` (Issue #13), and once `quit` is set `Update`
  discards all messages so a late completion cannot revive a cancelled
  UI; every view wraps the frame in `theme.Base`, composites the
  overlay while open, and sets `AltScreen` for the exit restoration
  sequence; resize handled in any state.
  Issue #5 state: `stops`/`files`, the `bufs`/`loading`/
  `failed` buffer maps, `theme`, `vp`, and the `loadGate` test seam —
  the Issue #5 `cursor` field is gone since Issue #13, the matched-line
  cursor living in `searchindex.Index` instead;
  Issue #12 adds `rows` (per-path prepared `viewport.Rows`, built on
  load completion) and `saved` (per-path top row for revisits), the
  `up`/`down`/`u`/`d`/`pgup`/`pgdown` key case, and the saved-state
  `SetTop` restore when a load completes for the current path —
  Issue #14 runs `reveal` immediately after that restore, covering the
  startup-after-load trigger;
  Issue #8 adds `binarySkipped`, the distinct excluded-file count shown
  on the no-results screen; Issue #9 adds `overlay`, the open
  diagnostics box; Issue #11 adds `diags` (the session diagnostic
  collection), `diagCh`/`diagAck` (the event channel and
  acknowledgement seam), `awaitEvent` (the `Init` command selecting
  between diagnostic lines and the completion, re-issued per
  `diagMsg`), `collect`/`CollectDiagnostic` (collection plus
  sanitization and acknowledgement), and `ReplayTo` (the
  post-restoration replay writer the boundary runs on every controlled
  exit).
- `internal/app/overlay.go` — the Issue #9 outcome contract:
  `decideOutcome` (the pure function of wait error, stderr, integrity
  failures, usable results, and — consumed since Issue #10 — the
  record-loss count and record-skip diagnostics, returning screen,
  overlay flag, fixed status, and diagnostic lines),
  `processFatal` (any wait error but benign exit 1),
  `collectDiagnostics` (generated code-or-signal line, sanitized
  stderr, integrity failures, record-skip lines — all through
  `present.Diagnostic` — the display list), and — Issue #11 —
  `completionDiagnostics`/`processDiagnostic`/`streamDiagnostics` (the
  collection subset, child stderr excluded because the session
  collection already took it incrementally); the
  modal `overlay` state, `overlayKey`, `overlayLayout` (hard-wrapped
  interior sized from the frame), `renderOverlay` (the centred
  `theme.Overlay` box composited over the base frame by cell-exact
  `ansi.Truncate`/`ansi.Cut` splicing), and `renderBlank` (the frame
  under a fatal-only overlay). See
  [error-overlay-and-fatal-outcomes.md](error-overlay-and-fatal-outcomes.md).
- `internal/app/noresults.go` — `renderNoResults` (Issue #8): the
  centred "No results found" message on the frame's middle row, with
  "(N binary files skipped)" appended when exclusion emptied the list,
  clipped to the frame width, every row padded so `Base` covers the
  screen. See
  [no-results-and-binary-exclusion.md](no-results-and-binary-exclusion.md).
- `internal/app/browse.go` — the Issue #5 browse composition:
  `loadDoneMsg` (the worker's prepared-buffer completion keyed by raw
  path), `ensureLoad`/`loadCmd` (one async load per file; read, split,
  escape, and map all off the update path, behind `loadGate` in tests),
  `renderBrowse` (raw-path-ordered file list — `FileList` entries with
  `CurrentFile` underline on the current one — scrolled into view,
  `FilenameRule`, `Loading…`/`(unreadable)` placeholders — path sinks
  via `present.Path`; each row padded to the frame edge so `Base`'s
  background covers it), `filenameRule`, `contentRow` (`Gutter`-styled
  right-justified number + two spaces over the viewport's visible-row
  slice), and `renderCells` (inverse-video
  spans over escaped `present.Cell`s via `Match`/`CurrentMatch`,
  marker spans, clip-edge wide clusters). Issue #12 adds `scroll`
  (the six scroll keys routed to `Viewport` units, then saving the top
  row per path — a no-op on placeholders and outside browse) and
  `bufferRows` (the prepared `viewport.Rows` adapter over
  `*filebuffer.Buffer`: unwrapped row i = source line i — Issue #14
  adds its `RowOf`, which returns `Target.Line` until wrap arrives),
  with
  `relayout` reinstalling the current file's prepared rows on every
  layout change. Issue #13 adds `navigate` (the `n`/`p` cursor step:
  strict no-op on zero/one stops, restyle within a file, and on a
  file crossing the departing top row is saved, the destination's
  prepared rows and saved-or-top state installed, and `ensureLoad`
  requests the load when uncached) and moves `currentStop`/
  `currentPath` onto `Index.Current()`. Issue #14 adds `reveal` (the
  target = the destination line's smallest span start cell, reported to
  `Viewport.Reveal`; the `saved` entry is replaced only when the
  viewport moved) and calls it at the end of every moved `navigate`. See
  [browse-tracer.md](browse-tracer.md),
  [match-navigation.md](match-navigation.md),
  [destination-reveal.md](destination-reveal.md),
  [viewport-scrolling.md](viewport-scrolling.md),
  [safe-presentation.md](safe-presentation.md), and
  [theme-and-colour-toggle.md](theme-and-colour-toggle.md).

## internal/searchindex

- `internal/searchindex/record.go` — `DecodeRecord` per-record schema
  validation for the five rg JSON events (`begin`/`match`/`end`/
  `summary`/`context`) plus `KindUnknown`; the `{"text"}`/`{"bytes"}`
  value union; `ErrMalformed` range/required-field errors; and —
  Issue #10 — `recoverRecordPath`/`recoverDataPath`, the token-streamed
  best-effort `data.path` recovery over an oversized record's consumed
  prefix. See
  [searchindex-records-and-stops.md](searchindex-records-and-stops.md)
  and [record-robustness.md](record-robustness.md).
- `internal/searchindex/index.go` — the `Index`: match records merge
  into navigation stops keyed by (raw path bytes, line), submatches
  sort by `(start, end)`, `Prepare` sorts stops by unsigned raw path
  bytes then line and computes union `Highlights`; `ResolvedPath` joins
  relative paths onto the working directory without canonicalization.
  Issue #8 adds binary exclusion: an `end` with non-null
  `binary_offset` drops the file's stops and marks its raw path in the
  `excluded` set (later matches for it drop too), `BinaryExcluded()`
  returns the distinct-file tally, and `LineCount()` is the
  usable-results value — retained stops after filtering. Issue #9 adds
  lifecycle validation: `Feed` consumes the collected stream (skipping
  schema-failing records, flagging a trailing unterminated record),
  `open`/`incomplete` track per-path lifecycle over raw path bytes,
  `seal` applies the end-of-stream rules once inside `Prepare`, and
  `IntegrityFailures()` returns the diagnostics kept separate from
  process success; `Stop.Incomplete` marks stops with damaged file
  metadata. Issue #10 adds record robustness: `maxRecordPayload` (the
  64 MiB per-record bound enforced on each split line), the `malformed`/
  `oversized`/`unknown` tallies exposed by `Malformed()`/`Oversized()`/
  `Unknown()`, `oversizedPaths` recovered per skipped oversized record,
  and `RecordDiagnostics()` assembling the nonfatal record-skip lines.
  Issue #13 adds the `cursor` field — an index into the sorted `order`
  clamped in `Prepare` — and the `export` helper sharing the
  `stop`→`Stop` projection between `Stops()` and the cursor methods.
  See
  [no-results-and-binary-exclusion.md](no-results-and-binary-exclusion.md),
  [error-overlay-and-fatal-outcomes.md](error-overlay-and-fatal-outcomes.md),
  and [record-robustness.md](record-robustness.md).
- `internal/searchindex/cursor.go` — Issue #13's matched-line cursor:
  `Step` (the selected `Stop` plus `Moved`/`FileChanged`/`Wrapped`
  transition flags), `Current()` (the first stop at startup, false when
  empty), and `Next()`/`Prev()` — circular steps where zero- and
  one-stop indexes are strict no-ops. See
  [match-navigation.md](match-navigation.md).

## internal/cli

- `internal/cli/cli.go` — the Issue #1 CLI foundation and the sole
  `mow.cli` v1.2.0 adapter, extended by Issue #2. Contains the shared
  `optionDecls`/`argDecls` table (parser config + raw-token recognition +
  generated help; the search-flag allow-list lives only here), the
  ordered `scanArgs`/`scanOption`/`expandOption` preflight (encounter-
  order flag records, combined-short expansion, cumulative `-u` cap,
  lexical `=` rejection), `renderHelp`, `checkRoot`, and the
  `Result`/`Kind`/`ErrorKind`/`Env` contract — `Result.ChildArgv` is
  the exact rg argument vector. Every hostile substitution (option
  tokens, excess operand, root) escapes through `present.Path`; the
  Issue #1 `cli.Escape` is gone since Issue #6. See
  [cli-foundation.md](cli-foundation.md) and
  [cli-flags-and-child-argv.md](cli-flags-and-child-argv.md).

## internal/present

- `internal/present/present.go` — `Path`, the canonical single-line
  path/filename escaper (`\n`/`\r`/`\t` forms, `\\` doubling, `\xNN`
  invalid UTF-8, caret notation for C0, `^?` for DEL, `\uXXXX` for C1,
  printable Unicode preserved), and `Diagnostic`, the
  line-boundary-preserving diagnostic escaper (LF/CRLF are real
  boundaries, tabs expand to 8-column stops, other controls escaped,
  `Path`-escaped filenames embed single-lined).
- `internal/present/line.go` — `LineOf`/`Line` for content lines
  (U+FFFD for invalid UTF-8, caret notation, `\uXXXX`, LF/CRLF never
  displayed, standalone CR → `^M`, provisional single-cell `→` tab
  pending Issue #16) with per-byte `lo`/`hi` byte→cell maps and `Span`
  range→cell mapping including zero-width markers; `Cell`. Grapheme-
  aware via `x/ansi`. See [safe-presentation.md](safe-presentation.md).
- `internal/present/doc.go` — the shared all-sink utility contract.

## internal/filebuffer

- `internal/filebuffer/filebuffer.go` — `Buffer`, one file's prepared
  display-ready content: `Load` reads, splits, escapes, and maps the
  file (all inside the worker command, never on `Update`) through
  `internal/present`, validates each stop's submatches against the
  line's raw bytes, and exposes `LineCount`,
  `GutterWidth`/`GutterDigits` (largest line number's digit width +
  two spaces, minimum one slot), `Text`, `Cells`, and `Spans`.

## internal/viewport

- `internal/viewport/viewport.go` — Issue #12's reading position:
  `Row` (source line + cells + spans of one rendered row), the `Rows`
  prepared-row provider interface built at load or layout time —
  Issue #14 adds `RowOf(Target)` so the provider resolves a display
  target to its rendered row — and `Viewport` — content dimensions, the
  clamped top rendered row
  (`[0, max(0, count − height)]`, lossy on EOF), the scroll units
  (`Up`/`Down` one row, `HalfUp`/`HalfDown` `max(1, floor(h/2))`,
  `PageUp`/`PageDown` the content height), `SetTop` for saved-state
  restore, Issue #14's `Reveal` (visible target → no scroll; hidden
  target → `row − floor(height/3)` top, clamped; reports whether the
  top moved), and `Visible`, which queries the provider only for the
  shown range. Wrap, anchors, and panning remain Issues 16–21.
  See [viewport-scrolling.md](viewport-scrolling.md) and
  [destination-reveal.md](destination-reveal.md).

## internal/theme

- `internal/theme/theme.go` — the Issue #7 module: the `dark`/`light`
  schemes (SGR pairs `37;40` / `30;47`, dark initially active),
  `Toggle`/`Light` (in-memory only), and the style set — `Base` (frame
  wrap), `Gutter`, `FileList`, `FilenameRule`, `Match` and `Indicator`
  (the scheme's true inverse pair via `scheme.inverse`),
  `CurrentMatch` (inverse + underline), `CurrentFile` (underline), and
  `Overlay` (base colours, plain single-line border). `Plain` remains
  the no-style composition path whose identity styles let sink-safety
  tests assert no escape bytes may legitimately appear. See
  [theme-and-colour-toggle.md](theme-and-colour-toggle.md).

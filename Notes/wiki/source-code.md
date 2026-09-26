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
  cursor through `navigate` (Issue #13), `,`/`.`/`<`/`>`/`[`/`]` in
  browse pan horizontally through `Model.pan` (Issue #18),
  `left`/`tab` hide and `right`/`shift+tab` show the file list
  (Issue #24), and once
  `quit` is set `Update`
  discards all messages so a late completion cannot revive a cancelled
  UI; every view wraps the frame in `theme.Base`, composites the
  overlay while open, and sets `AltScreen` for the exit restoration
  sequence; resize handled in any state.
  Issue #5 state: `stops`/`files`, the `bufs`/`loading`/
  `failed` buffer maps, `theme`, `vp`, and the `loadGate` test seam —
  the Issue #5 `cursor` field is gone since Issue #13, the matched-line
  cursor living in `searchindex.Index` instead; Issue #25 keys `loading`
  by a request identity `loadSeq` mints (one in-flight load per raw
  path, repeats dropped not queued; the `loadDoneMsg` case installs only
  on a matching identity and updates a non-current path's entries
  without touching the panel) and adds `mapGate`, the seam holding the
  decode/map phase alone after the read;
  Issue #12 adds `saved` (per-path vertical state for revisits —
  `viewport.Target` logical anchors since Issue #17) and the
  `up`/`down`/`u`/`d`/`pgup`/`pgdown` key case; Issue #14's `reveal`
  runs after the saved-state restore, covering the startup-after-load
  trigger; Issue #16 adds `wrap` (on initially) and the `w` browse key
  flipping between wrap and run-off-edge modes; Issue #17 adds the
  prepared-layout bookkeeping — `rows` (per-path `installed{key,
  rows}`), `revs` (per-path content revision bumped on each successful
  load), `reqKey` (latest requested key per path) — plus
  `pendingReveal` (the newest-stop reveal intent awaiting a matching
  layout), the `listW`/`textW`/`fileIdx` caches and `listEntry` seam
  keeping `View()` off the whole file list, the `layoutDoneMsg`
  install-only-on-match case, and layout requests on the resize, `w`,
  load-completion, and search-completion paths;
  Issue #24 adds `listShow` (the requested-visible preference —
  `left`/`tab` hide, `right`/`shift+tab` show, both browse-phase keys
  whose presses route through `syncLayout` like every other
  text-width change), `listTop` (the list window's first visible
  entry), and `statusNote` (the filename-row buffer-status provider,
  returning "" until Issues #26, #29, and #30 supply real notes);
  Issue #8 adds `binarySkipped`, the distinct excluded-file count shown
  on the no-results screen; Issue #9 adds `overlay`, the open
  diagnostics box; Issue #15 adds the pop-up state (`popupID`,
  `popupSeq`, `popupPath`, the `popupTimer` test seam), the
  `popupExpiredMsg` case with instance-keyed dismissal, the any-key
  dismissal ahead of normal key routing, the current-file
  `loadDoneMsg` failure route into `openOverlay`, and pop-up
  compositing under the overlay in `View`; Issue #11 adds `diags` (the
  session diagnostic
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
  under a fatal-only overlay). Issue #15 adds `openOverlay` (the single
  overlay-opening route — appends lines when an overlay is already up,
  and cancels any live pop-up so it cannot return) and `composite`
  (the shared centred cell-exact splice `renderOverlay` and
  `renderPopup` both consume). See
  [error-overlay-and-fatal-outcomes.md](error-overlay-and-fatal-outcomes.md)
  and [file-change-popup.md](file-change-popup.md).
- `internal/app/noresults.go` — `renderNoResults` (Issue #8): the
  centred "No results found" message on the frame's middle row, with
  "(N binary files skipped)" appended when exclusion emptied the list,
  clipped to the frame width, every row padded so `Base` covers the
  screen. See
  [no-results-and-binary-exclusion.md](no-results-and-binary-exclusion.md).
- `internal/app/browse.go` — the Issue #5 browse composition:
  `loadDoneMsg` (the worker's prepared-buffer completion keyed by raw
  path and — Issue #25 — the minted request identity),
  `ensureLoad`/`loadCmd` (one async load per file; read, split,
  escape, and map all off the update path, behind `loadGate` and —
  since Issue #25 — the post-read `mapGate` in tests),
  `renderBrowse` (raw-path-ordered file list — `FileList` entries with
  `CurrentFile` underline on the current one — painted from the
  `listTop` scrolled window,
  `FilenameRule`, `Loading…`/`(unreadable)` placeholders — path sinks
  via `present.Path`; each row padded to the frame edge so `Base`'s
  background covers it), `filenameRule` (Issue #24: the path joined by
  a buffer-status note slot that wins cells over the path, which
  left-truncates to nothing so the note paints whole; a note too wide
  alone is dropped), `contentRow` (`Gutter`-styled
  right-justified number + two spaces over the viewport's visible-row
  slice), and `renderCells` (inverse-video
  spans over escaped `present.Cell`s via `Match`/`CurrentMatch`,
  marker spans, clip-edge wide clusters — Issue #21: `Blank`-marked
  wrap/clip filler cells never take the match style). Issue #12 adds `scroll`
  (the six scroll keys routed to `Viewport` units, then saving the
  anchor per path — a no-op on placeholders and outside browse) — the
  Issue #12 `bufferRows` adapter is gone since Issue #16, which builds
  real `viewport.Model` values via `viewport.Prepare`. Issue #16
  also adds `reservedW` (the right-indicator column: zero in wrap
  mode, one in run-off-edge, populated by Issue #20) and gives
  `contentRow` the blank continuation gutter (`Row.Cont`). Issue #17
  adds `layoutDoneMsg`/`installed` (the keyed prepared-layout
  completion and its cached form) and moves preparation off the
  update path: `syncLayout` recomputes the geometry after any
  parameter change (list width, text width, viewport resize),
  `layoutKey`/`currentRows` express the (path, revision, text width,
  wrap mode) install contract, `ensureLayout`/`layoutCmd` issue the
  worker command, and `renderBrowse` consumes the cached widths and
  paints the file list through `listEntry` only for its visible
  window. Issue #24 gives `listWidth` the real three-term formula and
  its `listShow` gate (see
  [file-list-layout.md](file-list-layout.md)), adds `scrollList` (the
  minimal-movement active-entry scroll, run inside `syncLayout`), and
  rewrites `truncateLeft` as a grapheme-boundary cut that never
  exceeds its budget. Issue #13 adds `navigate` (the `n`/`p` cursor step:
  strict no-op on zero/one stops, restyle within a file, and on a
  file crossing the departing anchor is saved, the destination's
  matching-key rows and saved anchor installed, and `ensureLoad`
  requests the load when uncached) and moves `currentStop`/
  `currentPath` onto `Index.Current()`. Issue #14 adds `reveal` (the
  target = the destination line's smallest span start cell, reported to
  `Viewport.Reveal`; the `saved` entry is replaced only when the
  viewport moved) and calls it at the end of every moved `navigate`;
  since Issue #17 it carries the pending intent when no matching
  layout is installed, committed by `commitReveal` on install for the
  newest stop. Issue #15 opens the file-change pop-up inside the
  `FileChanged`
  branch — selection time, before the destination loads — and batches
  its instance-keyed expiry command with the load. Issue #18 adds
  `pan` (the six pan keys routed to `Viewport` units under the same
  loaded-buffer gate as `scroll`) and the file-entry offset reset:
  `navigate`'s `FileChanged` branch and the current-path
  `loadDoneMsg` both `SetOffset(0)` ahead of the reveal. Issue #20
  populates the indicators: `contentRow` composes the gutter as
  number + indicator cell + separator space — `Indicator`-styled `_`
  for text hidden left, `*` for a match or marker entirely hidden
  left, blank otherwise — and appends the reserved rightmost column,
  blank except an `Indicator`-styled `*` on the current matched
  line's row when `Row.MatchHiddenRight` is set. See
  [browse-tracer.md](browse-tracer.md),
  [hidden-content-indicators.md](hidden-content-indicators.md),
  [match-navigation.md](match-navigation.md),
  [destination-reveal.md](destination-reveal.md),
  [file-change-popup.md](file-change-popup.md),
  [async-load-isolation.md](async-load-isolation.md),
  [viewport-scrolling.md](viewport-scrolling.md),
  [wrap-mode.md](wrap-mode.md),
  [horizontal-panning.md](horizontal-panning.md),
  [logical-anchor-and-layout.md](logical-anchor-and-layout.md),
  [safe-presentation.md](safe-presentation.md), and
  [theme-and-colour-toggle.md](theme-and-colour-toggle.md).
- `internal/app/popup.go` — the Issue #15 file-change pop-up:
  `popupExpiredMsg` (the expiry carrying its instance ID), `popupTick`
  (the one-second `tea.Tick` behind the `popupTimer` test seam),
  `openPopup` (mints the next instance, stores the destination's raw
  path bytes, returns the expiry command), and `renderPopup` (the
  centred `theme.Overlay` box — `present.Path`-escaped path
  left-truncated with a leading `…` — recomputed from the current
  frame dimensions on every render, so resize recentres and
  re-truncates without touching the timer). See
  [file-change-popup.md](file-change-popup.md).

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
  displayed, standalone CR → `^M`; since Issue #16 a tab expands with
  space cells to the next multiple of eight source-display columns as
  one cluster, replacing the provisional `→`) with per-byte `lo`/`hi`
  byte→cell maps and `Span` range→cell mapping including zero-width
  markers; — Issue #22 — `LineOfBOM` is the same escaper over a
  hidden-prefix variant (`lineOf(raw, hidden)`): a file's first line
  keeps its leading UTF-8 BOM in `Raw` while the three bytes paint
  nothing and map to the line-start position; `Cell` carries `Lead`
  (a cluster's first cell, the only
  legal wrap boundary), `Cont` (trailing cells of a multi-cell
  unit), and — Issue #21 — `Blank` (a substituted filler cell the row
  and clip layers mark, never produced by `LineOf` and never a match
  cell); a standalone zero-width cluster takes a provisional `Lead`
  cell on a `◌` (U+25CC) dotted-circle base so it is always a visible
  cell — the shared segmentation/width policy Viewport consumes.
  Grapheme-aware via `x/ansi`. See [safe-presentation.md](safe-presentation.md),
  [grapheme-highlight-expansion.md](grapheme-highlight-expansion.md),
  [line-terminators-and-bom.md](line-terminators-and-bom.md),
  and [wrap-mode.md](wrap-mode.md).
- `internal/present/doc.go` — the shared all-sink utility contract.

## internal/filebuffer

- `internal/filebuffer/filebuffer.go` — `Buffer`, one file's prepared
  display-ready content: `Load` reads, splits, escapes, and maps the
  file (all inside the worker command, never on `Update`) through
  `internal/present` — Issue #25 split it into `ReadFile` (the read
  phase) and `Prepare` (the decode/map phase on raw bytes) so tests
  can gate the expensive phase alone — validates each stop's
  submatches against the
  line's raw bytes, and exposes `LineCount`,
  `GutterWidth`/`GutterDigits` (largest line number's digit width +
  two spaces, minimum one slot), `Text`, `Cells`, and `Spans`. Since
  Issue #16 the buffer is the grapheme-policy source for layout:
  `Cells` carries the `Lead`/`Cont` cluster marks, so `*Buffer`
  satisfies `viewport.Source` and row models consume boundaries
  without re-segmenting. Issue #21 adds `clusterSpan`: each validated
  submatch's mapped span expands outward to `Lead` cluster boundaries,
  so `Spans` hands down the cluster-expanded spans highlighting,
  reveal, and the hidden-match indicators all consume. Issue #22 adds
  the structural line rules and the coordinate split: LF/CRLF
  terminate lines with their bytes retained in each line's raw view,
  and a leading UTF-8 BOM sends line one through `LineOfBOM` and
  shifts that line's rg submatch offsets by three into the raw view
  before validating. Issue #23's zero-width markers ride the same
  `Spans` channel as `Start == End` positions — empty submatches
  validate against the raw bytes like any other, and `clusterSpan`
  passes their cell-precise positions through. See
  [wrap-mode.md](wrap-mode.md),
  [grapheme-highlight-expansion.md](grapheme-highlight-expansion.md),
  [line-terminators-and-bom.md](line-terminators-and-bom.md), and
  [zero-width-match-markers.md](zero-width-match-markers.md).

## internal/viewport

- `internal/viewport/viewport.go` — Issue #12's reading position:
  `Row` (source line + cells + spans of one rendered row — Issue #16
  adds `Cont` marking a continuation row; Issue #17 adds `Start`, the
  display-column offset where the row begins, giving every rendered
  row its logical location), the `Rows`
  prepared-row provider interface built at load or layout time —
  Issue #14 adds `RowOf(Target)` so the provider resolves a display
  target to its rendered row — and `Viewport` — content dimensions,
  the clamped top rendered row
  (`[0, max(0, count − height)]`), and — Issue #17 — the `anchor`, a
  `(source line, display-column offset)` logical position the
  effective top resolves from after every resize or row-model swap:
  `resolve` lands the top on the row containing the anchor, and the
  deliberately lossy EOF clamp rewrites the anchor to the clamped row
  when it pulls the top upward. A scroll that moves replaces the
  anchor with the new top row's location (a clamped no-move keeps it);
  `Reveal` does likewise only on a move (visible target → no scroll;
  hidden target → `row − floor(height/3)` top, clamped; reports
  whether the top moved); `Anchor`/`SetAnchor` are the logical entry
  points, `SetTop` the ordinal one; `Visible` queries the provider
  only for the shown range. Issue #18 adds `off`, the horizontal pan
  offset: `Offset`/`SetOffset` and the six pan units
  (`Left`/`Right` one column, `TenLeft`/`TenRight` ten,
  `HalfLeft`/`HalfRight` `max(1, floor(width/2))`) clamp to
  `[0, max(0, S)]` where `maxOffset` recomputes the widest *visible*
  line's paintable boundary on every pan; `clampOff` re-clamps inside
  `clamp` and `resolve` so every visible-set change re-evaluates it;
  `Rows.Wrap` gates panning to run-off-edge models (dormant, retained
  under wrap); and `Visible` runs each run-off-edge row through
  `clipRow`, which drops out-of-window cells, translates spans into
  window cells, and blanks a cluster split by either clip edge.
  Issue #19 extends `Reveal` with `revealCell`: after vertical
  placement it resolves the target's unit on its rendered row — the
  grapheme cluster holding the start cell via `Lead`/`Cont`, or a
  one-cell marker for a zero-width span (`markedAt`) — and, when that
  unit is not painted in the clipped window, moves `off` minimally
  (`s` hidden left, `s + cw − width` hidden right or clipped, `s` for
  a cluster wider than the text area); the unpaintable case counts as
  geometrically revealed so repeats cannot loop, and the write
  deliberately bypasses the paintable-boundary clamp.
  Issue #20 adds `Row`'s indicator flags —
  `HiddenLeft`/`MatchHiddenLeft`/`MatchHiddenRight` — computed by
  `hiddenMarks` inside `clipRow` from the unclipped line and the same
  split-cluster blanked regions the clip produces: painted-cell
  visibility, markers painting wherever they sit, partially painted
  matches counting visible, and an entirely hidden match attributed to
  the side its hidden cells stand on. Issue #21 marks the substituted
  clip-edge cells `Blank` so a covering span never styles the filler.
  Issue #23 makes `lineExtent` count marker spans as one-cell units:
  an end-of-line marker extends the content extent by one cell (a
  marker-only line has extent 1) and every marker is a paintable
  boundary at its position, so the maximum can land on it even past an
  unfittable final cluster.
  See [viewport-scrolling.md](viewport-scrolling.md),
  [grapheme-highlight-expansion.md](grapheme-highlight-expansion.md),
  [hidden-content-indicators.md](hidden-content-indicators.md),
  [horizontal-panning.md](horizontal-panning.md),
  [destination-reveal.md](destination-reveal.md),
  [minimal-horizontal-reveal.md](minimal-horizontal-reveal.md),
  [zero-width-match-markers.md](zero-width-match-markers.md),
  [logical-anchor-and-layout.md](logical-anchor-and-layout.md), and
  [wrap-mode.md](wrap-mode.md).
- `internal/viewport/rows.go` — Issue #16's prepared row model:
  `Source` (the per-line cell/span interface `*filebuffer.Buffer`
  satisfies), `Key` (path, content revision, text width, wrap mode —
  the staleness contract Issue #17's install guard consumes),
  `Prepare` (one-time layout: run-off-edge maps row i to line i; wrap
  mode packs clusters greedily into Width-cell rows, moving an unfit
  cluster whole and splitting an over-wide one as a last resort, with
  an extra row for an end-of-line marker past a full final row — runs
  in a worker command since Issue #17), and
  `Model` (`Len`/`Row`/`RowOf` — `Row` materializes cells and
  row-local spans per query, blanking a split cluster's clipped lead —
  Issue #21 marks that substituted cell `Blank`; Issue #18 adds
  `Wrap`, reporting the key's mode so the viewport can gate panning
  and clipping).

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

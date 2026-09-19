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
  `VRG_TEST_LOAD_GATE`, `VRG_TEST_DIAG_ACK`) into `app.Option`s — see
  [search-collection.md](search-collection.md),
  [cancellation-cleanup.md](cancellation-cleanup.md),
  [browse-tracer.md](browse-tracer.md), and
  [stderr-replay.md](stderr-replay.md).

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
  `FeedTail` / `Prepare` / `Build`, same-path/same-line `Stop` merging,
  submatch `(start, end)` ordering, union `Highlights`, unsigned
  raw-path ordering, and working-directory path resolution without
  canonicalization. Issue #8 added binary exclusion — an `end` with
  non-null `binary_offset` drops the file's stops and counts it once in
  `Index.BinaryExcluded` — and `UsableResults()`, the retained-stop
  count the outcome logic consumes. Issue #9 added lifecycle
  validation: per-path open tracking on decoded raw bytes, the
  transition matrix (duplicate/orphaned begin/end, retained incomplete
  matches, post-summary records, the unterminated tail), binary-
  exclusion precedence over orphan retention, `File.Incomplete` for
  partially-lifecycled files, and `Integrity().Complete` — stream
  integrity assessed separately from process success. Issue #10 added
  the skip counters `Malformed`/`Unknown` — classified per record,
  unqualified by position — and `Build`'s bounded scan that discards
  records over `MaxRecordBytes` through their next newline. See
  [no-results-screen.md](no-results-screen.md),
  [error-overlay-and-outcomes.md](error-overlay-and-outcomes.md), and
  [record-robustness.md](record-robustness.md).
- `internal/searchindex/cursor.go` — Issue #13's circular matched-line
  cursor: `Cursor{File, Stop}` positions, `Index.Cursor()` reporting
  the position (absent on an empty index), `Next`/`Prev` stepping one
  stop in path-then-line order with wrap at both ends, and the
  `Move{Wrapped, FileChanged}` report — file change detected by
  comparing the destination to the departing file so a one-file wrap
  reports `Wrapped` alone. Zero or one total stops makes both keys
  strict no-ops. See [match-navigation.md](match-navigation.md).
- `internal/searchindex/oversized.go` — Issue #10's `MaxRecordBytes`
  (64 MiB payload limit) and the oversized-record accounting:
  `feedOversized` counts the record, recovers its path best-effort for
  diagnostics via `recoverOversizedPath` — a bounded `json.Decoder`
  token pass over the first `MaxRecordBytes` for `type` and
  `data.path` — and flags an after-`summary` arrival as an integrity
  failure. See [record-robustness.md](record-robustness.md).
- `internal/searchindex/doc.go` — package comment.

## internal/app

- `internal/app/rg.go` — the child-process seam: `spawn` runs `rg` from
  `PATH` with `cmd.Dir` set to the invocation working directory and
  drains both stdout and stderr concurrently for the child's whole
  lifetime; `Child`/`Result`/`StartFunc` are the boundary types, with
  `Child.Terminate` and an idempotent `Wait` backing the cleanup path.
  Issue #11 added `Child.Diags`: `drainStderr` line-splits stderr into
  a mutex-guarded pending queue (drainage never blocks on a consumer)
  and `feedDiags` forwards it onto the channel in order, closed at EOF.
- `internal/app/app.go` — the Bubble Tea model and `app.Run`: the
  "Searching…" state covering collection and post-exit index
  preparation, the `WithGate`/`WithCollectAck`/`WithDiagAck` test
  seams, the `searchDoneMsg` outcome decision — `stateBrowse`, the
  Issue #8 `stateNoResults` alternative with its "(N binary files
  skipped)" suffix, or Issue #9's `stateOverlayOnly` fatal
  presentation — the modal-overlay key routing, the sanitized
  start-failure diagnostic with exit 2 before the TUI, and the Issue #4
  surface: `ctrl+c`/`q` cancellation to 130, the `quitCmd`/`reapChild`
  cleanup boundary, `WithFailFunc`/`WithReapReport`, the `quitting`
  discard of late completions, alt-screen views, `ErrInterrupted` →
  130, and the Issue #7 `c` key toggling `m.theme` between the dark
  and light schemes. The search-derived `status` is fixed at
  completion; only `ctrl+c` overrides it to 130. Issue #10's
  `recordWarnings` composes the "N unrecognised record types skipped"
  warning from the index's `Unknown` count. Issue #12 adds the browse
  scroll-key route (`stateBrowse` + `isScrollKey` → `scrollBy`), the
  per-file `vps`/`rows` maps, saved-viewport re-clamping on resize and
  on load completion. Issue #14's `fileLoadedMsg` path also runs
  `model.reveal` when the completed path is the current file — the
  startup-after-load reveal trigger. Issue #13 adds the `n`/`p` browse
  route to
  `navigate` and drops `model.cur`: the current file and current
  matched line derive from the index's matched-line cursor. Issue #15
  adds the `popupID`/`popupSeq` pop-up state, the `popupExpireMsg`
  stale-instance-rejecting expiry case, any-key dismissal before the
  key switch, `View` compositing the pop-up over the base frame and the
  overlay over both, and the `options.popupTimer` test seam. Issue
  #16 adds `m.wrap` (on initially; the `w` browse key toggles it) and
  `m.revs` (the per-path content revision bumped on every successful
  load), and `WindowSizeMsg` now rebuilds the keyed row models through
  `rebuildRows` rather than only re-clamping. Issue #17 replaces that
  synchronous rebuild with the prepared-layout pipeline:
  `layoutReadyMsg` installs a row model only while its key still
  matches `layoutKey` (obsolete completions are discarded untouched),
  `layoutReqs`/`pendingReveals` hold the in-flight keys and deferred
  reveal intents, `w` and `WindowSizeMsg` just record parameters and
  return a `requestLayout` command, and installs `Restore` the saved
  viewport's anchor into the fresh model. `options.layoutGate` and
  `options.escapePath` (`WithLayoutGate`/`WithEscapePath`) are the new
  test seams. Issue #24 adds the `listWBase`/`listVisible`/`notes`
  fields — the longest sanitized path width prepared once at
  `searchDoneMsg`, the visibility preference (initially shown), and
  the synthetic per-file status-note map — and the
  `left`/`tab`/`right`/`shift+tab` browse routes that flip the
  preference and re-key the current file's layout. Issue #11 adds the session
  diagnostic collection `model.diags`: `collectDiags` appends sanitized
  lines as `Update` processes `stderrLineMsg` (forwarded from
  `Child.Diags()` by a `prog.Send` goroutine in `Run`),
  `searchDoneMsg`, `fileLoadedMsg` failures, and `failMsg`; after the
  program returns and the child is reaped, `replayDiags` writes the
  collection to stderr exactly once, in order — the common
  post-restoration writer that also replaced the controlled-failure
  direct write. Issue #25 turns `loading` into a path→request-identity
  map fed by `loadSeq`: `fileLoadedMsg` gains a `req` field its
  issuing command echoes back, and `Update` applies a completion only
  when the `(path, req)` pair matches a live request — unrequested,
  stale, forged, and already-settled completions are discarded
  untouched — while `options.decodeGate` (`WithDecodeGate`) joins
  `loadGate` as the seam holding the decode/map phase alone. Issue #26
  adds `failDiag` — each failed path's latest load diagnostic — and
  the notification split in `fileLoadedMsg`'s error path: a failure
  for the current file calls `openOverlay` (opening fresh or
  appending one occurrence scroll-preserved), a non-current failure
  collects the diagnostic only. See
  [cancellation-cleanup.md](cancellation-cleanup.md),
  [theme.md](theme.md),
  [no-results-screen.md](no-results-screen.md),
  [error-overlay-and-outcomes.md](error-overlay-and-outcomes.md),
  [record-robustness.md](record-robustness.md),
  [stderr-replay.md](stderr-replay.md),
  [async-load-isolation.md](async-load-isolation.md), and
  [read-failures.md](read-failures.md).
- `internal/app/outcome.go` — Issue #9's pure outcome decision:
  `DecideOutcome` maps `OutcomeInput` (process `Result`, stream
  `Integrity`, usable-results count, `RecordLoss`, caller `Warnings`)
  to the presentation, the escaped overlay lines, the dismiss-exits
  flag, and the fixed status — the whole PRD outcome table. Issue #10
  filled `RecordLoss` (`Malformed`, `Oversized`, the recoverable
  `Paths`) and added its row: a complete stream with skipped records
  and zero usable results — assessed after all filtering — is the
  fatal overlay-only outcome. `outcomeDiagnostics` composes the overlay
  lines in the universal order — captured stderr or a generated
  code-or-signal line for a silent failed process, then the integrity
  note, then `recordLossLines` (the malformed/oversized counts plus
  "oversized record skipped for \<path\>" lines), then warnings — all
  through `safepresentation.EscapeDiagnostic`/`EscapePath`. Issue #11
  split the composition into `processDiags` (the process component) and
  `tailDiags` (integrity + record loss + warnings) so the session
  collection shares it without re-collecting incrementally delivered
  stderr.
- `internal/app/overlay.go` — Issue #9's modal error overlay:
  grapheme-boundary `wrapCells` to the interior width (unbroken strings
  split mid-run), the complete wrapped row set scrolled by
  `up`/`down` clamped to `[0, rows − visible]`, and `compositeOverlay`
  centering the single-line bordered box over the base frame. Issue #15
  adds `openOverlay` — the shared open-or-append entry point that also
  clears any live file-change pop-up, which never returns after the
  overlay closes.
- `internal/app/popup.go` — Issue #15's file-change pop-up:
  `popupExpireMsg{id}` and `startPopup` minting a fresh instance ID per
  file crossing with a one-second `tea.Tick` expiry (or the
  `options.popupTimer` test seam), and `compositePopup` — the
  `theme.Overlay`-bordered box centred over the base frame carrying the
  current file's `EscapePath`-sanitized path, left-truncated with a
  leading `…` by `leftTruncate`/`tailCells` on whole grapheme clusters,
  all recomputed from the live terminal size at every render. See
  [file-change-popup.md](file-change-popup.md).
- `internal/app/browse.go` — the Issue #5 browse view: async
  `filebuffer` load commands gated by `WithLoadGate`, the
  `loading`/`bufs`/`failed` caches keyed by raw path, `fileLoadedMsg`,
  the full-width filename rule, the fixed-width file list with the
  current entry underlined, the right-justified gutter, inverse-video
  match runs, and the "Loading…"/"(unreadable)" placeholders — all
  styled through `internal/theme` since Issue #7 (base-wrapped frame,
  true-inverse matches, underlined current match and current file).
  Issue #11's `loadDiag` composes the single-line `cannot read
  \<EscapePath(path)\>: \<reason\>` diagnostic a failed load collects
  (`*fs.PathError` contributes only its cause). Issue #12 adds the
  `rowSource` seam the frame render queries per visible row,
  `curKey`/`contentRows`, and `isScrollKey`/`scrollBy` — the
  `up`/`down`/`u`/`d`/`pgup`/`pgdown` handling that scrolls the
  current file's saved per-file viewport clamped to the prepared row
  count and content height, a no-op on placeholders. Issue #13 adds
  `curFile` (the cursor-derived current file read by `curKey`,
  `startLoad`, the list underline, and the filename rule), the
  cursor-derived `curLine` driving `CurrentMatch`, and `navigate` —
  `n`/`p` step the index cursor and issue `startLoad` for the
  destination only on `Move.FileChanged`, with the departing file's
  viewport already saved by scroll write-through. Issue #14 extends
  `rowSource` with `TargetRow`, adds `model.reveal` — the
  starting-viewport sequence (saved `vps` top or first-visit top of
  file, then `Viewport.Reveal`, written back only on a move) — and wires
  it into `navigate`'s actual transitions and the
  current-file `fileLoadedMsg` path in app.go. Issue #15 makes
  `navigate` return `tea.Batch(m.startLoad(), m.startPopup())` on a
  file change — the pop-up starts at selection while the destination
  may still be loading, and load completion never restarts it. Issue
  #16 makes the file panel wrap-aware: `listWidth`/`textWidth` compute
  the text band (panel − list − gutter − `ReservedIndicator`),
  `prepareRows`/`rebuildRows` build and swap the keyed
  `viewport.Rows` models, `rowSource.At` now returns a `viewport.Row`,
  `contentCell` blanks continuation gutters and pads the reserved
  indicator column, and `contentText` renders a row's `[Start, End)`
  cell span. Issue #17 moves preparation off `Update`: `layoutKey`
  is the live demand, `requestLayout` issues gated `viewport.Prepare`
  commands (deduplicated by `layoutReqs`, nil on the matching-layout
  fast path), `currentRows` hides stale installed models so rendering
  and scrolling see a placeholder, `reveal` pends intents on
  `pendingReveals` that commit when a matching layout installs,
  `navigate` requests the destination's layout so cached-stale files
  re-prepare, `rowSource` embeds `viewport.Model` for the anchor
  translations, `listWBase` fixes the list's longest-path width at
  search-done, and `listCell` escapes only the visible window's
  entries through the `escapePath` seam. Issue #18 adds
  `isPanKey`/`panBy` (`,`/`.`/`<`/`>`/`[`/`]` — one cell, ten cells,
  `HalfText` of the layout's text width — no-ops in wrap mode and on
  placeholders), `navigate`'s `ResetOff` before the reveal on every
  file change, `rowSource` now embedding `viewport.Extent`, and
  `contentText`'s `off` window with blank cells for a cluster split
  by the left clip edge. Issue #19 extends `rowSource` with
  `StopTarget` and makes `reveal` run `Viewport.RevealOff` after the
  vertical `Reveal` — the minimal horizontal reveal on startup and
  every `n`/`p` transition, writing the viewport back when either
  half moved. Issue #20 makes `contentCell` populate the
  hidden-content indicators in run-off-edge mode: the gutter's first
  trailing space takes `LeftMark`'s `_`/`*` (composed
  `Gutter`/`Indicator`/`Gutter`), and the reserved rightmost cell
  takes `RightMark`'s `*` on the current matched line's row when a
  match is entirely hidden right — wrap mode draws neither. Issue
  #21 rewrites `contentText`'s clip detection cluster-aware: a cluster
  straddling either edge (`cl.Start < lo || cl.End - lo > textW`)
  contributes one unstyled blank per in-window cell — clip blanks are
  never match cells — replacing the byte-range `cont`/`clipped`
  inference that styled a clipped cluster's blank when the cluster
  was highlighted. Issue #23 makes `contentText` marker-aware: an
  empty highlight span inside text marks the existing cell it lands
  on (`MarkerAt` ORs into the per-cell highlight test — no text
  shifts), and a cell past `len(cells)` — the end-of-line marker —
  paints one inverse-video space clipped by the same
  `col + 1 > textW` bound, underlined by `CurrentMatch` on the
  current matched line. Issue #24 makes the file list responsive:
  `fileListWidth` computes the nonnegative three-term minimum
  (`longest + 2`, `floor(0.40 × width)`, `width − gutter − 10 −
  reserved indicator`), `listWidthFor`/`listWidth`/`gutterWidth`/
  `textWidth` apply it under the current file's gutter — returning
  zero while `listVisible` is false without touching the preference
  — `listCell` left-truncates its escaped path with a leading `…`
  for the requested visible entry only, `browseView` scrolls the
  list window to keep the active entry visible, and `filenameRule`
  gains the `─ path note ────` status-note slot whose synthetic
  `m.notes` string the path truncates around (Issues #26/#29/#30
  supply the real notes). Issue #25 makes `startLoad` mint a
  `loadSeq` request identity recorded in `loading` per raw path —
  at most one in flight, re-entry dropped not queued — which
  `fileLoadedMsg{path, req, buf, err}` echoes back; the command now
  runs `filebuffer.Read` under `loadGate`, then `decodeGate`, then
  `filebuffer.Decode`, so the read and the decode/map phase gate
  separately and the completion still carries a fully prepared
  buffer. Issue #26 makes a failed path retryable: `startLoad` clears
  `failed[key]` when it mints so the panel reads "Loading…" until
  settlement, `navigate` opens the prior-failure overlay from
  `failDiag` on a file-changed entry into a failed path while a
  same-file step still issues nothing, and `options.loader`
  (`WithLoader`) substitutes `filebuffer.Read` as the injected-loader
  test seam. See
  [browse-tracer.md](browse-tracer.md), [theme.md](theme.md),
  [stderr-replay.md](stderr-replay.md),
  [read-failures.md](read-failures.md),
  [file-change-popup.md](file-change-popup.md),
  [viewport-scrolling.md](viewport-scrolling.md),
  [match-navigation.md](match-navigation.md),
  [destination-reveal.md](destination-reveal.md),
  [wrap-mode.md](wrap-mode.md),
  [logical-anchor.md](logical-anchor.md),
  [horizontal-panning.md](horizontal-panning.md),
  [horizontal-reveal.md](horizontal-reveal.md),
  [hidden-content-indicators.md](hidden-content-indicators.md),
  [grapheme-highlight-expansion.md](grapheme-highlight-expansion.md),
  [zero-width-markers.md](zero-width-markers.md),
  [file-list-layout.md](file-list-layout.md), and
  [async-load-isolation.md](async-load-isolation.md).
- `internal/app/doc.go` — package comment.

## internal/filebuffer, internal/viewport, internal/theme, internal/safepresentation

- `internal/filebuffer/filebuffer.go` — `Read` reads the file by raw
  path bytes, `Decode` splits LF/CRLF and maps each line through
  `safepresentation.MapContent` (Issue #25 split the phases so the
  load command gates the read and the decode/map separately), and
  `Load` composes them; together they produce `Buffer`/`Line` records
  with `GutterWidth`, source `Raw` bytes, `Highlights` mapped to
  display cells, and (Issue #16) `Clusters` — the shared
  grapheme-cluster segmentation Viewport wraps from. Issue #18 adds
  `Line.MaxStart(width)` — the paintable boundary: the largest cell
  index where a cluster begins and fits within `width`, skipping an
  over-wide trailing cluster and reporting 0 when none fits. Issue
  #21 adds `Line.expandToClusters`, applied in `makeLine` after
  `CellsCovering`: every nonempty highlight range snaps outward to the
  whole clusters it touches, so `Highlights` is the
  cluster-aligned single span source painting, reveal, and the
  hidden-content indicators all consume. Issue #22 separates the
  coordinate views: `Line.SearchBytes()` is the rg-line view (`Raw`
  minus a leading UTF-8 BOM's three bytes on line 1), cell byte
  offsets stay raw-file coordinates, and the shadowing
  `Line.CellsCovering` maps rg-line ranges onto cells — removed
  terminator bytes, positions past the content end, and zero-width
  positions in them landing on the display end-of-line position —
  while a standalone `\r` stays content as `^M`. Issue #23 adds the
  marker surface: `Line.MarkerAt(cell)` reports an empty highlight
  span at a cell, `Line.Extent()` is the effective display width —
  the cell count plus one for an end-of-line marker — and `MaxStart`
  counts the EOL marker as a one-cell paintable candidate, so a
  marker-only line reports 0. See
  [browse-tracer.md](browse-tracer.md),
  [wrap-mode.md](wrap-mode.md),
  [horizontal-panning.md](horizontal-panning.md),
  [grapheme-highlight-expansion.md](grapheme-highlight-expansion.md),
  [line-structure.md](line-structure.md), and
  [zero-width-markers.md](zero-width-markers.md).
- `internal/viewport/viewport.go` — `Viewport`, the file panel's
  vertical window (`Top`, `Scroll`, `Clamp`), the scroll-unit helpers
  `HalfPage` and `MaxTop` (the BOF/EOF clamp bound), and `Rows`, the
  prepared rendered-row model built by `Prepare(buf, Key)` that the
  frame render slices. Issue #16 adds `Key` (path, content revision,
  text width, wrap mode), `Row`/`span` cell-range rows, `Row.Continuation`,
  `ReservedIndicator` (0 wrapping, 1 run-off-edge), and `wrapLine`'s
  cluster-boundary partitioning with the never-split blank-cell rule.
  Issue #17 adds `Anchor{Line, Cell}` — the width-independent logical
  reading position — the `Model` interface (`Len`/`AnchorAt`/`RowOf`)
  every positioning operation takes, `Viewport.anchor` plus
  `Viewport.Anchor`/`Restore`, and the `Rows.AnchorAt`/`Rows.RowOf`
  translations; `Scroll`/`Reveal` replace the anchor only when the
  effective top moves and any clamp that pulls the top up rewrites it
  (the lossy EOF rule). Issue #18 adds `Viewport.off` (the horizontal
  pan offset, per-file saved state), the `Extent` interface (`Model`
  plus `Key`/`At`) that `Scroll`/`Restore`/`Clamp`/`Reveal` now take,
  and `clampOff` — the wrap-aware re-clamp every visible-set change
  runs against `MaxOff`. Issue #23 makes the row model marker-aware:
  flat spans run to `Line.Extent()` so `Row.End` can reach
  `len(Line.Cells) + 1`, and `wrapLine` appends the end-of-line
  marker as an unbreakable one-cell unit — joining a partially full
  row, starting its own row after a full one, and giving an empty
  matched line one `[0,1)` row. See
  [viewport-scrolling.md](viewport-scrolling.md),
  [wrap-mode.md](wrap-mode.md),
  [logical-anchor.md](logical-anchor.md),
  [horizontal-panning.md](horizontal-panning.md),
  [zero-width-markers.md](zero-width-markers.md), and
  [hidden-content-indicators.md](hidden-content-indicators.md) (what
  `ReservedIndicator`'s column holds).
- `internal/viewport/pan.go` — Issue #18's horizontal panning:
  `Viewport.Off`/`ResetOff`/`Pan` (the offset reader, the
  file-change reset, and the per-keypress move-and-clamp that is a
  strict no-op under a wrap model), `HalfText` (the `[`/`]` unit,
  `max(1, floor(width / 2))`), and `MaxOff` — the largest valid
  offset as the paintable boundary of the widest *currently
  rendered* source line, derived from `Line.MaxStart` over only the
  visible row range. See
  [horizontal-panning.md](horizontal-panning.md).
- `internal/viewport/reveal.go` — Issue #14's vertical destination
  reveal: `Target{Line, Cell}` (the display target — the first
  submatch's start cell), `Rows.StopTarget` (stop → target through the
  byte→cell map, with the zero-width marker-cell fallback),
  `Rows.TargetRow` (target → rendered row; since Issue #16 the wrapped
  row whose span holds the target cell, falling back to the line's
  last row), and `Viewport.Reveal`
  (visible-target no-scroll, else top = `row − floor(h/3)` clamped to
  `[0, MaxTop]`, reporting whether the viewport moved). Issue #17's
  `Reveal` is model-aware and replaces the logical anchor only when
  the viewport moves — a no-scroll reveal retains the logical column.
  Issue #18 widens the parameter to `Extent` and re-clamps the
  horizontal offset against the newly visible rows on a move.
  Issue #19 adds `Viewport.RevealOff` — the minimal horizontal
  reveal (left rule `off = start`, right rule
  `off = start + cluster width − text width`, unpaintable-cluster
  geometric fallback to the start column) — plus `CellVisible`, the
  painted-cell visibility predicate the reveal and Issue #20's
  indicators share, and `targetCluster`, the `Clusters`-backed
  cell→(start, width) lookup. See
  [destination-reveal.md](destination-reveal.md),
  [logical-anchor.md](logical-anchor.md),
  [horizontal-panning.md](horizontal-panning.md),
  [horizontal-reveal.md](horizontal-reveal.md), and
  [hidden-content-indicators.md](hidden-content-indicators.md).
- `internal/viewport/indicators.go` — Issue #20's hidden-content
  indicators over the prepared row model: `LeftMark` (the row's
  gutter mark — `*` when a match or marker span is entirely hidden
  left, `_` when text is hidden left but no match is entirely hidden
  there, blank otherwise) and `RightMark` (`*` when a match or marker
  span is entirely hidden right — the reserved-column mark the
  renderer scopes to the current matched line's row), both built on
  the same `targetCluster`-backed painted-cell visibility as
  `CellVisible`: a clipped cluster's blanked in-window cells count as
  hidden, an empty span judges its one-cell marker position, and a
  partially painted span is not entirely hidden. See
  [hidden-content-indicators.md](hidden-content-indicators.md).
- `internal/theme/theme.go` — the active scheme's style set: `Dark()`
  (white on black, initially active), `Light()` (black on white), the
  pure `Toggled` flip behind the `c` key, `Plain()` (the no-style
  composition path for sink-safety tests), and the decorators `Base`,
  `Gutter`, `Match` (true-inverse colours), `CurrentMatch` (inverse +
  underline), `Indicator`, `FilenameRule`, `FileList`, `CurrentFile`
  (base + underline), and `Overlay` (base colours inside a plain
  single-line border). See [theme.md](theme.md).
- `internal/safepresentation/safepresentation.go` — `EscapePath`,
  `MapContent`, `Mapped`/`Cell`/`CellsCovering` byte→cell maps, and
  (Issue #16) `Cluster`/`Mapped.Clusters` — the shared grapheme
  segmentation — plus the structural eight-column tab expansion.
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

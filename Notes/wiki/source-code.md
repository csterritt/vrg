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
  evidence/gate/failure injection via `VRG_TEST_*` env vars; Issue #11:
  replays every collected diagnostic from `m.Diagnostics()` to stderr
  exactly once each in collection order after terminal restoration, and
  wires the `VRG_TEST_COLLECT_ACK` acknowledgement side channel and the
  `VRG_TEST_DIAGNOSTIC_TRIGGER`/`VRG_TEST_DIAGNOSTIC_TEXT` diagnostic
  emission trigger), usage error → sanitized diagnostic on stderr,
  exit 2. See [search-collection-path](search-collection-path.md).

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
  files by unsigned byte ordering of raw path bytes. Issue #8 added
  binary exclusion: a non-null `binary_offset` in an `end` event drops
  the file and all its previously collected matches from the builder,
  counts the file as a distinct excluded file, and drops later matches
  for the same file. `Index.ExcludedFiles()` exposes the distinct
  excluded-file count. Issue #9 added stream-integrity accounting and
  lifecycle validation: `Builder` tracks per-path open/closed state,
  summary-seen state, after-summary state, trailing-malformed state,
  and an integrity-failed flag. `Index.Integrity()` exposes the
  `Integrity{Complete}` assessment, kept separate from process success
  so the app can assess them independently. `Stop.Incomplete` marks
  retained matches whose lifecycle metadata is incomplete (orphaned
  match, file still open at stream end). `Builder.MarkTrailingMalformed`
  signals a trailing unterminated record. Issue #10 added robust
  record handling: `Index.MalformedCount()`, `Index.OversizedCount()`,
  `Index.UnknownCount()`, and `Index.OversizedDiagnostics()` expose
  separate counters and sanitized path diagnostics. `Builder.ReadFrom`
  is the bounded 64 MiB record reader that discards oversized records
  through the next newline and resynchronizes on the following record.
  Unknown string event types are counted separately from malformed and
  never independently alter exit status. Issue #13 added the circular
  matched-line cursor: `Cursor` tracks the current stop position over
  `Index.stops` (already ordered by unsigned raw path bytes then
  ascending line number and deduplicated by `(raw path, line number)`,
  so multiple submatches on one line are one stop). `NewCursor(idx)`
  starts at the first stop (position 0) or -1 when empty/nil.
  `Stop()`/`Position()`/`Len()` report state. `Next()`/`Prev()` move
  circularly with wrap at both ends and return the new stop, whether
  the cursor moved, and whether the file changed (raw path differs via
  `bytes.Equal`). With zero or one stop both are strict no-ops: no
  move, no file change. See
  [search-collection-path](search-collection-path.md),
  [record-robustness](record-robustness.md), and
  [browse-tracer](browse-tracer.md).

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
  complete control escaping. Issue #8 added `StateNoResults`: a
  centred no-results screen shown after a complete successful search
  (rg exit 0 or 1) with no usable results, with the optional
  `(N binary files skipped)` suffix when every matched file was
  excluded. The outcome logic consumes the retained stop count after
  binary filtering as the single usable-results value. `q` from
  no-results exits 1 through the Issue #4 cleanup path; `Esc` is a
  no-op; `ctrl+c` exits 130. Issue #9 added the fatal/warning outcome
  matrix and modal error overlay: `ProcessResult` (exit code + signal
  death), `OutcomeInput`/`Outcome`, the pure `DecideOutcome` function,
  `OverlayKind` (none/error/warning), `OverlayOpen()`/`OverlayKind()`
  accessors, `SearchCompleteMsg.Process`/`Stderr` fields, and
  `processResult`/`processResultFromSys` to derive the process result
  from the wait error. The fixed exit status is decided once at
  completion and never recomputed except by `ctrl+c` (which overrides
  to 130). The overlay is modal: up/down scroll, q/Esc dismiss
  (non-fatal) or exit 2 (fatal no-results), other keys ignored. Large
  diagnostics show both head and tail. Issue #10 extended the outcome
  matrix with record-loss inputs: `RecordLoss{Malformed, Unknown,
  Oversized}` and `RecordLossDiagnostics` on `OutcomeInput`, the
  `recordLossDiagnostics` helper, and new matrix rows for malformed,
  oversized, and unknown record loss. `collectResults` now uses
  `Builder.ReadFrom` (the bounded 64 MiB reader) instead of
  `bufio.Scanner`. Issue #11 added the session diagnostic collection
  independent of display: `diagnostics []string` field,
  `collectDiagnostic` helper (sanitizes through `EscapeDiagnostic`,
  appends to collection, fires `onCollect` callback),
  `Diagnostics()` accessor (returns a copy in collection order),
  `DiagnosticMsg` type (collects without display), `WithDiagnosticSignal`
  option (test seam for emitting a `DiagnosticMsg`),
  `WithOnCollect` option (test seam for the application-side
  acknowledgement side channel), and `watchDiagnostic` command. The
  `SearchCompleteMsg`, `SearchFailedMsg`, `ControlledFailureMsg`, and
  `DiagnosticMsg` handlers now collect into the session collection.
  The controlled-failure diagnostic is routed through the collection
  instead of a separate direct write, so exactly-once holds across
  both the former direct-write path and the replay mechanism. The
  shutdown boundary is defined at message-processing time: a
  diagnostic is collected once the model has processed the message
  carrying it; in-flight diagnostics are not awaited or replayed.
  Issue #12 added manual vertical scrolling with per-file saved
  viewport state: a `viewport *viewport.Viewport` field (nil while
  loading), `perFileOffset map[string]int` for per-file saved offsets
  keyed by raw path, `currentPath []byte`, a `RowProviderFactory` type
  and `WithRowProviderFactory` option (test seam for the render-cost
  guard), `ViewportOffset()`/`SavedOffset(path)` accessors,
  `handleScrollKey` (routes `up`/`down`/`u`/`d`/`pgup`/`pgdn` to the
  viewport), and `saveOffset` (records the current offset as per-file
  state after every scroll). `FileLoadCompleteMsg` now builds the
  viewport from prepared row data and restores the saved per-file
  offset (0 for a first visit). `WindowSizeMsg` calls
  `viewport.SetPanelHeight` to recompute layout and clamp the offset.
  `renderContentPanel` queries `viewport.Visible()` for the visible
  range only instead of scanning the full buffer per frame. Scroll keys
  are no-ops while the viewport is nil (loading placeholder). Issue #13
  added the circular matched-line cursor and App wiring: a
  `cursor *searchindex.Cursor` field (replacing the Issue #5 `browseIdx`
  int), created on `SearchCompleteMsg` via `searchindex.NewCursor`,
  selecting the first stop at startup; `handleNavigate(delta)` called
  for `n` (delta 1) and `p` (delta -1), moving the cursor circularly
  with strict no-op behavior for zero or one stop; on a same-file move
  only the current matched line styling changes (Issue #14 now applies
  a destination reveal - see below); on a cross-file move the
  departing file's viewport offset is saved, the content panel
  switches immediately, a cached destination is shown with its saved
  viewport restored (first visit starts at the top), and an uncached
  destination requests a load via `loadFileFor(path)`; a `fileCache
  map[string]*filebuffer.Buffer` session cache keyed by raw path (no
  eviction) so a revisited file can be shown immediately without a
  reload; `loadFile` now delegates to `loadFileFor` with the cursor's
  current stop's raw path; `CursorPosition()`/`CurrentPath()`
  accessors; `renderBrowse` derives the current file and current
  matched line from the cursor so the file list underline and
  current-match styling follow cursor selection; manual scrolling does
  not move the cursor, so `n`/`p` continue from the last selected
  stop; the file list remains passive with no direct selection route.
  Issue #14 added vertical destination reveal: a `needsReveal bool`
  field gates the startup-after-load and
  uncached-cross-file-navigation reveal triggers (set on
  `SearchCompleteMsg` before the startup load and on cross-file
  navigation to an uncached destination; cleared once applied; reload
  does not set it, so a reload preserves the saved viewport anchor
  without revealing a match); `revealTarget()` reads the cursor's
  current stop, computes the rendered target row via
  `targetRow(stop)`, records the offset before the reveal, calls
  `viewport.Reveal(targetRow)`, and saves the new offset as per-file
  state only if the reveal moved the viewport (a no-scroll reveal
  preserves the retained state); `targetRow(stop)` returns the
  0-based rendered row containing the display target - the start cell
  of the first submatch on the destination line (submatches are
  ordered by byte start, so the first identifies the target); without
  wrapping (Issue #16 pending) the rendered row is the 0-based source
  line index (`stop.LineNumber - 1`); the reveal is applied in
  `FileLoadCompleteMsg` (when `needsReveal` is set), in same-file
  `handleNavigate`, and in cross-file `handleNavigate` for a cached
  destination. Issue #15 added the file-change pop-up: `popupOpen`
  /`popupPath`/`popupInstance`/`popupDuration` fields; a
  process-wide `popupInstanceCounter` for fresh instance IDs;
  `FileChangePopupExpiryMsg{Instance}` instance-keyed expiry
  message; `startPopup(path)`/`dismissPopup()`/`cancelPopup()`
  helpers; `WithPopupDuration` option (test seam; production
  default 1 second, 0 for instant test timer);
  `PopupOpen()`/`PopupPath()`/`PopupInstance()` accessors;
  `handleNavigate` calls `startPopup(stop.RawPath)` on cross-file
  navigation before the cached/uncached branch (uncached batches the
  timer and load via `tea.Batch`); `Update` dismisses the pop-up on
  matching-instance expiry (stale expiry ignored); any keypress
  dismisses the pop-up before normal key routing (q still quits,
  scroll keys still scroll, Esc is a no-op besides dismissal);
  `SearchCompleteMsg` that opens an error overlay calls
  `cancelPopup()` so the pop-up does not return after overlay
  dismissal; `View` renders the pop-up via `renderPopup` (escaped
  path, left-truncated with `…`, centred horizontally and
  vertically, base padded to terminal height) below the error
  overlay; `truncateLeftCells` helper for safe left-truncation. Issue
  #16 added wrap mode and the swappable row model: a `wrapMode
  viewport.WrapMode` field (initial `WrapOn`), a `rowModel
  *viewport.RowModel` field, `buildViewport()` (constructs the row
  provider from the buffer, wrap mode, and panel dimensions; uses the
  `RowProviderFactory` test seam when set, otherwise builds a
  `RowModel`), `ViewportRowCount()` accessor, the `w` key toggle
  (toggles wrap mode in browse mode and rebuilds the viewport; no-op
  outside browse mode), `WindowSizeMsg` rebuilds the row model when
  the text width changes, `targetRow(stop)` now uses
  `RowModel.RowFromByte` with the first submatch's byte start to find
  the wrapped row, and `renderContentPanel` shows a blank gutter for
  continuation rows. Issue #17 added the logical anchor and off-UI
  layout preparation: a `prevBuildPath []byte` field (distinguishes
  same-file rebuilds from fresh loads), `buildViewport()` now returns
  a `tea.Cmd` (the layout preparation command) and has three paths
  (factory test seam, cache hit, cache miss/stale); `LayoutReadyMsg`
  with keyed installation guard (installs only when the key matches
  current parameters, discards out-of-order/stale completions);
  `pendingLayout`/`pendingLayoutKey`/`pendingReveal` fields;
  `layoutCache map[string]*viewport.RowModel`; `LayoutKey()`,
  `WrapMode()`, `HasPendingLayout()`, `PendingLayoutKey()`,
  `HasPendingReveal()` accessors; `WithLayoutGate`,
  `WithRowModelFactory`, `WithFileListProvider` options (test seams);
  `RowModelFactory` and `FileListProvider` types; cross-file
  navigation sets `viewport = nil` so installation restores the saved
  per-file anchor; `FileLoadCompleteMsg` and `handleNavigate` carry
  the reveal intent as `pendingReveal` when the viewport is nil; the
  `LayoutReadyMsg` handler commits the pending reveal; `renderBrowse`
  limits the file-list iteration to `min(fileCount, terminalHeight)`
  for the render-cost guard. Issue #18 added horizontal panning:
  `handlePanKey` routes `,`/`.`/`<`/`>`/`[`/`]` to `Viewport.Pan`;
  `buildViewport` (all three paths) and the `LayoutReadyMsg` handler
  carry over `Viewport.HOffset()` on same-file rebuilds and call
  `ResetHorizontal` on file change; `SetLayout` is called on every
  viewport installation; `WindowSizeMsg` updates the text width via
  `SetLayout` when the factory test seam is in use; `renderContentPanel`
  calls `Viewport.ClipLine` on each visible row before
  `renderLineWithHighlights`; `ViewportHOffset()` exposes the offset.
  See
  [search-collection-path](search-collection-path.md),
  [browse-tracer](browse-tracer.md),
  [outcome-contract](outcome-contract.md),
  [record-robustness](record-robustness.md),
  [manual-vertical-scrolling](manual-vertical-scrolling.md),
  [destination-reveal](destination-reveal.md),
  [wrap-mode-and-grapheme-policy](wrap-mode-and-grapheme-policy.md),
  [logical-anchor-and-layout-preparation](logical-anchor-and-layout-preparation.md),
  and [horizontal-panning](horizontal-panning.md).

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
  `^M` for standalone CR, tabs expanded to eight-column stops per
  Issue #16). `EscapeDiagnostic(raw []byte) string` escapes raw
  diagnostic bytes for safe display while preserving real line
  boundaries (LF preserved, CRLF normalized to LF, tabs expanded to
  eight-column stops, other controls escaped). `PathDisplay`/
  `ContentDisplay` expose `ByteCells` byte→cell mappings so highlight
  rendering can cover all cells of an escaped form. Issue #16 added
  the shared grapheme policy: `Cluster{StartByte, EndByte, Width}` and
  `GraphemeClusters(display string) []Cluster` segment an escaped
  display string into grapheme clusters using `github.com/rivo/uniseg`
  and compute each cluster's terminal cell width via
  `uniseg.StringWidth`. See [safe-presentation](safe-presentation.md),
  [browse-tracer](browse-tracer.md), and
  [wrap-mode-and-grapheme-policy](wrap-mode-and-grapheme-policy.md).

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
  carries `Number`, `Display`, `ByteCells`, `Highlights`, and (Issue
  #16) `Clusters` (grapheme clusters from the shared policy),
  `StartByte` (byte offset where a wrapped row begins), and
  `Continuation` (true for wrapped rows that are not the first row of
  their source line). Issue #16: `Load` now populates `Clusters` via
  `safepresentation.GraphemeClusters`. `Cluster` is an alias for
  `safepresentation.Cluster`. The completion message carries the fully
  prepared buffer so `Update` does no full-file work. See
  [browse-tracer](browse-tracer.md) and
  [wrap-mode-and-grapheme-policy](wrap-mode-and-grapheme-policy.md).

## internal/viewport

- `internal/viewport/viewport.go` — Issue #5 landed a minimal
  scrollable content view seam. Issue #12 expanded it into the full
  manual vertical scrolling and prepared-row rendering module. The
  `RowProvider` interface (`RowCount() int`, `Rows(start, end int)
  []filebuffer.Line`) supplies rendered rows on demand so the Viewport
  queries only the visible range. `Viewport` holds a `RowProvider`,
  `panelHeight`, and `offset`. Content height is `panelHeight - 1`
  (the filename row occupies one row). The offset is clamped to
  `[0, maxOffset]` where `maxOffset = max(0, rowCount - contentHeight)`,
  ensuring no avoidable blank rows below EOF; files shorter than the
  viewport naturally leave unused rows. `New(rows, panelHeight)` creates
  a viewport at offset 0. `Visible()` queries the row provider for the
  visible `[offset, offset+contentHeight)` range only. Scroll methods:
  `ScrollDown`/`ScrollUp` (one row), `ScrollHalfDown`/`ScrollHalfUp`
  (`max(1, floor(contentHeight/2))`), `ScrollPageDown`/`ScrollPageUp`
  (full `contentHeight`). `SetOffset`, `SetPanelHeight`, and `SetRows`
  re-clamp the offset. `RowCount()` returns the total rendered row
  count. Issue #14 added `Reveal(targetRow int)`: if the target row is
  already within the visible range, the offset is unchanged (visible-
  target no-scroll); otherwise the viewport is moved so the target
  lands at zero-based row `floor(contentHeight / 3)`, clamped to
  `[0, maxOffset]` so BOF and EOF available content takes precedence
  over one-third placement. `BufferRows(buf)` adapts a
  `filebuffer.Buffer` to `RowProvider`. Issue #16 added the wrap row
  model: `WrapMode` (`WrapOn` default, `WrapOff` run-off-edge) with
  `Toggle()`; `ReservedWidth(mode)` (0 in wrap, 1 in run-off-edge for
  the Issue #20 indicator); `TextWidth(panelWidth, gutterWidth, mode)`;
  `RowModelKey{Path, Revision, TextWidth, WrapMode}`; `RowModel`
  (prepared rows + source mappings + key, implements `RowProvider`);
  `BuildRowModel(buf, textWidth, mode, key)` (wraps at grapheme-cluster
  boundaries, two-cell clusters that don't fit move to the next row,
  continuation rows carry `Continuation=true`); and
  `RowFromByte(lineIndex, byteOffset)` mapping a source line and byte
  offset to the wrapped row containing that byte. Issue #17 added the
  logical anchor: `Anchor{LineIndex, Column int}` (width-independent
  source location); `RowModel.RowFromCell(lineIndex, column int) int`
  and `RowModel.RowAnchor(row int) Anchor`; `Viewport` anchor field
  with `Anchor()`, `SetAnchor`, and anchor-aware `SetRows`/
  `SetPanelHeight`/`SetOffset`; `syncAnchorToOffset` and
  `recomputeOffsetFromAnchor`; and lossy EOF-clamp anchor replacement.
  Issue #18 added horizontal panning: `hOffset`, `textWidth`, and
  `wrapMode` fields; `HOffset()`, `TextWidth()`, `SetLayout(textWidth,
  wrapMode)` (re-clamps on text-width-only changes, defers the clamp
  on wrap-mode changes to the next `SetRows`), `HalfPanWidth()`
  (`max(1, floor(textWidth/2))`), `Pan(columns)` (no-op in wrap mode,
  clamps to the paintable-boundary maximum), `ResetHorizontal()`,
  `SetHOffset(n)` (carries the offset across rebuilds, clamps in
  run-off-edge mode), `MaxHOffset()` (paintable-boundary maximum from
  the widest visible line), `ClipLine(line)` (grapheme-safe clipping
  with blank cells for split clusters); `clampHOffset` (no-op in wrap
  mode), `computeMaxHOffset`, `widestLine`, `paintableMaxOffset`,
  `clipLineToWindow`, and `clipHighlights` helpers; and
  `clampHOffset()` calls in `SetAnchor`/`SetOffset`/`SetPanelHeight`/
  `SetRows`/`Reveal`/all scroll methods for visible-set re-clamping.
  See
  [manual-vertical-scrolling](manual-vertical-scrolling.md),
  [browse-tracer](browse-tracer.md),
  [destination-reveal](destination-reveal.md),
  [wrap-mode-and-grapheme-policy](wrap-mode-and-grapheme-policy.md),
  [logical-anchor-and-layout-preparation](logical-anchor-and-layout-preparation.md),
  and [horizontal-panning](horizontal-panning.md).

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

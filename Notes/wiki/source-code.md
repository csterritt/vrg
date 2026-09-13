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
  Issue #19 added horizontal reveal triggers: `revealTarget` now calls
  `Viewport.RevealHorizontal` after the vertical reveal in run-off-edge
  mode, deriving the target cell from `line.ByteCells[sm.Start][0]` and
  the cluster width from `ClusterWidthAtCell`; `WithWrapMode` option
  and `wrapMode` config field for starting in run-off-edge mode in
  tests. Issue #20 added hidden-content indicators in run-off-edge
  mode: `renderContentPanel` emits a left `_`/`*` in the first trailing
  gutter space (text hidden left vs. match entirely hidden left) and a
  right `*` in the reserved column on the current matched line's
  visible row when a match is entirely hidden right; `leftIndicator`,
  `hasHiddenMatchRight`, `highlightHasVisibleCells`, `lineHasContent`,
  and `clusterCellWidth` derive visibility from the actually rendered
  cells after grapheme clipping (split clusters do not count), exclude
  the reserved column, and use `Theme.Indicator`; wrap mode draws
  neither. Issue #24 replaced the placeholder `fileListWidth`
  function with the responsive file-list layout: `listVisible bool`
  (initially true), `listOffset int`, `longestPathWidth int`, and
  `statusNote func() string` fields; `ListVisible()`/`ListWidth()`/
  `ListOffset()`/`ViewportAnchor()` accessors;
  `ComputeListWidth(termWidth, longestPathWidth, gutterWidth,
  reservedIndicator, visible)` (nonnegative minimum of
  `longestPathWidth+2`, `floor(0.40×termWidth)`, and
  `termWidth−(gutterWidth+10+reservedIndicator)`; 0 when not visible);
  `TruncateLeftGrapheme(s, maxCells)` (grapheme-safe left truncation
  with leading `…` via `safepresentation.GraphemeClusters`);
  `computeLongestPathWidth` (measures sanitized index paths through
  the shared grapheme policy); `toggleListVisible(visible)` (updates
  preference and rebuilds via `buildViewport`); `updateListOffset`
  and `currentFileIndex` (active-entry auto-scroll on navigation
  and render); `renderFilenameRow` (filename row with a buffer-
  status note slot, path truncated to fit the panel width);
  `graphemeCellWidthString` helper; `LayoutKey()` uses
  `m.ListWidth()` for the panel width; `handleNavigate` calls
  `updateListOffset` on every actual navigation; `renderBrowse`
  uses `m.ListWidth()`, auto-scrolls, queries only visible provider
  entries, applies `TruncateLeftGrapheme`, and skips list rendering
  for zero width; `renderContentPanel` delegates its first line to
  `renderFilenameRow`; `WithStatusNote(func() string)` test seam for
  the synthetic status note. Issue #25 added keyed asynchronous load
  isolation: a `RequestID uint64` field on `FileLoadCompleteMsg` so
  the completion handler can reject stale completions; a
  `loadRequestID uint64` model field as the monotonically increasing
  request identity counter; a `loadingPaths map[string]uint64` model
  field tracking in-flight loads by raw path (mapping to the active
  request ID); `startLoad(path) (Model, tea.Cmd)` as the single entry
  point for starting a load (enforces one-load-per-path by checking
  `loadingPaths`, assigns a fresh request ID, records the in-flight
  request, and delegates to `loadFileFor`); `loadFileFor(path,
  requestID)` now carries the request identity in the completion
  message; the `FileLoadCompleteMsg` handler validates the request
  ID against `loadingPaths` before mutating state (stale completions
  from cancelled or superseded requests are discarded), removes the
  path from `loadingPaths`, caches the buffer regardless of whether
  the path is current, and updates the visible panel only when the
  completion's path is still the current path; the
  `SearchCompleteMsg` handler sets `m.currentPath` to the startup
  file's raw path before starting the load so the keyed completion
  handler can identify it as current; `handleNavigate` calls
  `startLoad` instead of `loadFileFor` directly so the one-load-per-path
  rule is enforced on re-entry of a loading path. See
  [search-collection-path](search-collection-path.md),
  [browse-tracer](browse-tracer.md),
  [outcome-contract](outcome-contract.md),
  [record-robustness](record-robustness.md),
  [manual-vertical-scrolling](manual-vertical-scrolling.md),
  [destination-reveal](destination-reveal.md),
  [wrap-mode-and-grapheme-policy](wrap-mode-and-grapheme-policy.md),
  [logical-anchor-and-layout-preparation](logical-anchor-and-layout-preparation.md),
  [horizontal-panning](horizontal-panning.md),
  [horizontal-reveal](horizontal-reveal.md),
  [hidden-content-indicators](hidden-content-indicators.md),
  [file-list-layout](file-list-layout.md),
  and [async-load-isolation](async-load-isolation.md).
  Issue #26 added read-failure handling: a `readFailed bool` model
  field marking the current file's last load attempt as failed
  (rendering shows `(unreadable)` instead of `Loading…`); a
  `failedPaths map[string]string` model field tracking failed raw
  paths to sanitized diagnostics (a successful load removes the
  path); an `overlayReadFailure bool` model field marking the open
  overlay as a read-failure overlay; `openReadFailureOverlay`
  opening a fresh non-fatal `OverlayError` for a current-file load
  failure (or appending to an existing read-failure overlay on a
  re-entry retry failure without resetting `overlayScroll`), with
  search-complete overlays taking precedence and the file-change
  pop-up cancelled on open; the `FileLoadCompleteMsg` handler
  distinguishing errors from successful buffers while retaining the
  Issue #25 request-ID and current-path isolation (current-file
  errors mark the file unreadable and collect the diagnostic;
  non-current errors record the diagnostic in `failedPaths` and
  collect it without disturbing the visible panel; the fixed
  `ExitCode` is not recomputed); overlay dismissal clearing
  `overlayReadFailure`; the filename row using `(unreadable)` as
  the default status note when the current file has failed and no
  `WithStatusNote` callback overrides it; `truncateRightCells`
  truncating the status note when it would overflow the panel
  (reserving at least one cell for the path); `handleNavigate`
  reopening the prior-failure overlay immediately on re-entry into
  a previously failed file (clearing `readFailed`, setting
  `loading`, and starting exactly one retry load while the overlay
  is open, reusing the Issue #25 one-load-per-path rule); and public
  accessors `OverlayFatal()`, `OverlayText()`, `OverlayScroll()`,
  `IsLoading()`, and `ReadFailed()`. See
  [read-failures-and-retry](read-failures-and-retry.md).
  Issue #27 added explicit reload: `handleReload` rereads the current
  file on `r` (from browse or through a read-failure overlay) without
  rerunning ripgrep or changing cursor stops, recording the path in
  `reloadingPaths map[string]bool` so the `FileLoadCompleteMsg`
  handler increments the per-path content revision in
  `revisions map[string]int` (default `1`, incremented only for
  reloads); `LayoutKey()` now uses `contentRevision(path)` instead of
  the hardcoded `1` so stale cached layouts from a prior revision are
  discarded by the Issue #17 installation guard; `handleOverlayKey`
  routes `r` through a read-failure overlay so the user can retry
  without dismissing it; `HasPendingReloadAnchor()` exposes the
  intent for tests. See [explicit-reload](explicit-reload.md).
  Issue #28 generalized the Issue #14 `needsReveal`, Issue #17
  `pendingReveal`, and Issue #27 `pendingReloadAnchor` flags into a
  single `loadIntent LoadIntent` field with `IntentNone`,
  `IntentReveal`, and `IntentReloadAnchor` values. Stage one
  (`FileLoadCompleteMsg`) validates, caches, updates the revision,
  installs the buffer, and starts layout preparation without making
  row-based reveal decisions. Stage two (`LayoutReadyMsg` or
  synchronous cache-hit installation) commits the intent via
  `commitLoadIntent()`. Navigation during a pending reload replaces
  `IntentReloadAnchor` with `IntentReveal`. Stale layouts are
  discarded without consuming or mutating the intent. `LoadIntent()`
  exposes the intent for tests. Issue #29 integrated stale-match
  validation: `revealTarget` calls `Buffer.RevealTarget(stop)` for the
  validated `(lineIdx, byteStart, cell)` instead of reading
  `stop.Submatches[0]` directly; `targetRow` accepts the validated
  line/byte and falls back to `stop.LineNumber - 1` only when the
  buffer provides no target; `renderFilenameRow` shows
  `file changed since search` in the Issue #24 status slot when
  `m.buffer.Stale`. Issue #30 integrated unsupported-encoding
  detection: an `unsupportedEncoding bool` model flag mirrors
  `readFailed`; a current-file unsupported buffer opens the Issue #9
  overlay with the encoding diagnostic, shows
  `(unsupported encoding)` in the panel and the Issue #24 status
  slot, collects the diagnostic for replay, and builds no viewport; a
  non-current unsupported buffer is diagnostic-only; `handleReload`
  clears the flag (showing `Loading…` then re-detecting); cached
  unsupported destinations open the overlay through the
  cached-destination path in `handleNavigate`. See
  [load-completion-two-stage](load-completion-two-stage.md),
  [stale-match-validation](stale-match-validation.md), and
  [unsupported-encodings](unsupported-encodings.md).
  Issue #31 added the modal help overlay: `OverlayHelp` kind, `KeyBinding`
  struct, `KeyBindings()` (single binding-table data source covering
  navigation, scrolling, panning, wrap, colour, list toggle, reload,
  help, and quit/cancel), `HelpFooter()` (footer slot reserved for
  Issue #34), `helpText()` (builds overlay text from the binding table
  plus footer), and `openHelp()` (opens the overlay, cancels any active
  Issue #15 pop-up with no return on close). `h`/`?` open help from
  browse and no-results; while open, `up`/`down` scroll, `q`/`Esc`/`h`/`?`
  close, `ctrl+c` exits 130, and every other key (including `n`/`p`/`w`/`c`/`r`)
  is ignored with the underlying state unchanged. The help overlay
  shares the `renderOverlay` component with the error overlay (base
  colours, plain single-line border, wrapped text including unbroken
  strings, vertical scrolling reaching every row) but skips the
  head/tail compression so all rows are reachable by scrolling. At tiny
  sizes the overlay is clipped to the terminal without a borderless
  mode and restored on growth; rendering does not panic at 25×8. See
  [help-overlay](help-overlay.md).
  Issue #32 established the full overlay precedence stack (`ctrl+c`
  over modal error over help over pop-up over base keys), error-suspends-help
  with scroll restoration (`suspendedHelp`/`suspendedHelpScroll` fields,
  `HelpSuspended()` accessor), generalized append-preserving-scroll to
  all appended errors (a read failure while any error/warning overlay is
  open appends without resetting scroll and marks the overlay as a
  read-failure overlay so `r` can retry), pop-up cancellation by help and
  error with no return, `Esc` no-op with no overlay, the
  `dismissOverlay()` helper consolidating `q`/`Esc` dismissal, and the
  dismissal-outcome table for both `q` and `Esc` (fatal no-results exits
  2; error-over-help restores help at its saved scroll position; a second
  `q` from a still-running base state exits the fixed status; a second
  `Esc` leaves the state running). See
  [overlay-precedence](overlay-precedence.md).

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
  `uniseg.StringWidth`. Issue #21 added `ContentDisplay.ByteOffsets`:
  `ByteOffsets[i]` is the display byte offset where original byte `i`
  starts in `Text`, so `filebuffer` can map raw bytes to grapheme
  clusters by display byte range (a combining mark's cell is outside
  its cluster's cell range, so byte offsets are needed). See
  [safe-presentation](safe-presentation.md), [browse-tracer](browse-tracer.md),
  [wrap-mode-and-grapheme-policy](wrap-mode-and-grapheme-policy.md),
  and [grapheme-cluster-highlight-expansion](grapheme-cluster-highlight-expansion.md).

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
  map. `Buffer` carries `Lines`, `LineCount`, `GutterWidth`, and (Issue
  #22) `BOMOffset` (number of leading UTF-8 BOM bytes stripped, 0 or 3,
  for raw-file ↔ rg-line coordinate conversion). `Line` carries
  `Number`, `Display`, `ByteCells`, `Highlights`, and (Issue #16)
  `Clusters` (grapheme clusters from the shared policy), `StartByte`
  (byte offset where a wrapped row begins), and `Continuation` (true
  for wrapped rows that are not the first row of their source line).
  Issue #16: `Load` now populates `Clusters` via
  `safepresentation.GraphemeClusters`. `Cluster` is an alias for
  `safepresentation.Cluster`. The completion message carries the fully
  prepared buffer so `Update` does no full-file work. Issue #21 added
  grapheme-cluster highlight expansion: `expandedByteCells` remaps
  `ByteCells` so every byte in a cluster (including combining marks and
  ZWJ joiners) maps to the cluster's full cell range; `expandedHighlights`
  expands each submatch's byte range outward to the enclosing clusters'
  cell boundaries (combining-only matches highlight the base cluster,
  wide glyphs are never split, standalone zero-width clusters receive a
  visible fallback cell, multi-cell escaped forms like ESC → `^[` are
  preserved). The expanded `Highlights` and `ByteCells` are the single
  source for Viewport, App, Issue #19 reveal, and Issue #20 indicators.
  Issue #22 added structural line handling: `Load` detects and strips a
  leading UTF-8 BOM (`EF BB BF`) before `splitLines` so rg-line and
  raw-file coordinates are kept separate (`Buffer.BOMOffset` records the
  strip length); `splitLines` retains original line bytes including
  terminators for byte-coordinate mapping; terminator bytes and
  zero-width positions map to the display end-of-line column; a span
  covering visible text plus terminator highlights only the visible text
  (`expandedHighlights` uses the end byte's original cell end when it
  has no cluster); a standalone CR is escaped as `^M`; an empty file
  produces zero lines with a three-cell gutter; non-leading U+FEFF is
  ordinary content. Issue #23 added zero-width match markers:
  `markerCellsForStops(stops, byteCells, clusters)` returns the display
  cell positions of zero-width submatches (`Start == End`), mapping each
  through the expanded `ByteCells` (cluster-start mapping) and
  deduplicating; `clusterContentWidth(clusters)` returns the sum of
  cluster widths (the content extent before any EOL marker extension);
  for each marker cell at the content width (EOL), `Load` appends a
  space to `Display` and a 1-cell cluster to `Clusters` so the marker
  has a paintable cell (an empty matched line therefore has width one);
  each marker cell is appended to `Highlights` as `[cell, cell+1)` and
  `Highlights` are sorted by start cell so rendering processes them in
  cell order. The terminator-only `$` marker is an ordinary marker with
  no special cases. Issue #29 added best-effort stale-match
  validation: `Load` validates every submatch against the original
  line bytes (including terminators, BOM-adjusted) via `submatchValid`
  (line existence, range validity, byte equality), drops invalid
  submatches individually, marks `Buffer.Stale` on any failure, and
  computes highlights/markers from the validated stops only;
  `Buffer.RevealTarget(stop)` returns the validated reveal target
  (first surviving submatch, clamped recorded start with end-of-line
  cell clamping, or last source line); `Line` carries `RawBytes`
  (original line bytes), `ContentWidth`, `HasMarker`, `ValidStarts`,
  and `FirstRecordedStart` for the fallback computation. Issue #30
  added unsupported-encoding BOM detection: `detectUnsupportedBOM(data)`
  checks the leading bytes for UTF-32 LE (`FF FE 00 00`), UTF-32 BE
  (`00 00 FE FF`), UTF-16 LE (`FF FE`), and UTF-16 BE (`FE FF`), with
  longer BOMs checked before overlapping shorter ones so `FF FE 00 00`
  classifies as UTF-32 LE rather than UTF-16 LE; a detected file
  returns a placeholder `Buffer` with `UnsupportedEncoding = true`,
  `EncodingDiagnostic` set, no lines, no highlights, and `Stale =
  false` (the stale-match guard never runs against raw encoded
  bytes); a leading UTF-8 BOM (`EF BB BF`) is not unsupported and
  falls through to the existing Issue #22 strip path. See
  [browse-tracer](browse-tracer.md),
  [wrap-mode-and-grapheme-policy](wrap-mode-and-grapheme-policy.md),
  [grapheme-cluster-highlight-expansion](grapheme-cluster-highlight-expansion.md),
  [structural-line-handling](structural-line-handling.md),
  [zero-width-match-markers](zero-width-match-markers.md),
  [stale-match-validation](stale-match-validation.md), and
  [unsupported-encodings](unsupported-encodings.md).

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
  `clipLineToWindow`, and `clipHighlightsToPaintable` helpers (Issue
  #21 replaced `clipHighlights` with `clipHighlightsToPaintable`, which
  intersects highlights with fully-visible non-split cluster cell ranges
  so split-blank filler cells are never painted as match cells); and
  `clampHOffset()` calls in `SetAnchor`/`SetOffset`/`SetPanelHeight`/
  `SetRows`/`Reveal`/all scroll methods for visible-set re-clamping.
  Issue #19 added minimal horizontal reveal:
  `ClusterWidthAtCell(clusters, cell)` returns the terminal cell width
  of the grapheme cluster at a display cell (1 fallback);
  `RevealHorizontal(line, targetCell)` adjusts the offset so the
  target cluster is painted — no-op in wrap mode, no-op when already
  painted (fully within the window, not split), right-edge arithmetic
  for right-side reveal (`offset = target + clusterWidth - textWidth`),
  left-edge for left-side (`offset = target`), geometric fallback for
  unpaintable clusters (offset = target, no clamp, idempotent). See
  [manual-vertical-scrolling](manual-vertical-scrolling.md),
  [browse-tracer](browse-tracer.md),
  [destination-reveal](destination-reveal.md),
  [wrap-mode-and-grapheme-policy](wrap-mode-and-grapheme-policy.md),
  [logical-anchor-and-layout-preparation](logical-anchor-and-layout-preparation.md),
  [horizontal-panning](horizontal-panning.md),
  and [horizontal-reveal](horizontal-reveal.md).

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

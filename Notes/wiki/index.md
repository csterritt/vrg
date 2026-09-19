# Wiki index

Catalog of all wiki pages for the vrg project.

## Overview

- [project-overview.md](project-overview.md) — what vrg is, stack, layout

## Concepts and architecture

- [cli-foundation.md](cli-foundation.md) — Issue #1 CLI: mow.cli adapter,
  emission-prevention output strategy, shared declarations, preflight,
  help/positional/root/sanitization contracts
- [cli-flags-child-argv.md](cli-flags-child-argv.md) — Issue #2 CLI:
  search-flag allow-list, ordered scan records, combined shorts,
  cumulative `-u`, assignment rejection, exact child argv
- [search-collection.md](search-collection.md) — Issue #3 app/search
  path: `rg` spawn with the protected argv from the invocation working
  directory, concurrent dual-pipe drainage, the "Searching…" state
  spanning collection and index preparation, gate/collect-ack test
  seams, interim summary with `q` → exit 0, start-failure exit 2, and
  the SearchIndex record/merge/order/resolution contracts
- [cancellation-cleanup.md](cancellation-cleanup.md) — Issue #4
  cancellation and cleanup: `q`-while-searching and `ctrl+c` to exit
  130 (including the gate-held post-exit window), the `quitCmd`/
  `reapChild` terminate-and-reap boundary, `Terminate` + idempotent
  `Wait`, `quitting` discard of late completions, alt-screen display
  and PTY termios restoration, the `WithFailFunc` hook and the single
  post-restoration `writeFailureDiag` → exit 2, and the
  `VRG_TEST_REAP`/`VRG_TEST_FAIL` seams with the termios PTY harness
- [browse-tracer.md](browse-tracer.md) — Issue #5 browse view: the
  two-pane file list + file panel with the filename rule, gutter, and
  inverse-video matches; async `filebuffer` loading with prepared
  buffers and "Loading…"; the safe-presentation escape rules and
  byte→cell maps; the minimal viewport/theme seams; the
  `VRG_TEST_LOAD_GATE` seam; and the hostile-fixture raw-output
  sink-safety method
- [safe-presentation.md](safe-presentation.md) — Issue #6 shared
  safe-presentation utility: the canonical path/content/diagnostic
  escaping contracts, the replaced Issue #1 `cli.Escape`, every sink
  routed through `internal/safepresentation`, and the extensible
  `sinktest` sink-safety table (no-style raw-output assertions plus the
  styled payload-after-ESC check)
- [theme.md](theme.md) — Issue #7 theme module: the initially-dark and
  light schemes, the session-only `c` toggle, true-inverse match
  colours, current-match and current-file underlines, the indicator,
  overlay, gutter, filename-rule, and file-list styles, and how the
  base-wrapped frame consumes them
- [no-results-screen.md](no-results-screen.md) — Issue #8: binary
  exclusion on non-null `binary_offset` with the distinct-file count,
  `UsableResults` as retained stops after filtering, and the centred
  "No results found" screen with its "(N binary files skipped)" suffix,
  `q` → exit 1, `Esc` no-op, `ctrl+c` → 130
- [error-overlay-and-outcomes.md](error-overlay-and-outcomes.md) —
  Issue #9: the stream-lifecycle transition matrix and
  `Index.Integrity()` assessed separately from process success, the
  pure `DecideOutcome` function and every outcome-table row, the modal
  error overlay's keys/wrapping/scrolling/dismissal, stderr
  classification with generated code-or-signal diagnostics, and the
  fixed-status rule with the `ctrl+c` → 130 override
- [record-robustness.md](record-robustness.md) — Issue #10: the
  deterministic malformed/integrity/unknown disposition matrices with
  their two composite rows, the 64 MiB `MaxRecordBytes` limit with
  discard-and-resynchronize and best-effort oversized-path recovery,
  the triple-disposition unterminated oversized tail, missing-`end`
  retention, and the record-loss outcome rows with usable results
  assessed after all filtering
- [stderr-replay.md](stderr-replay.md) — Issue #11: the session
  diagnostic collection independent of display (`model.diags` fed by
  `stderrLineMsg` incremental child-stderr delivery, completion,
  load-failure, and `failMsg` messages), the message-processing
  shutdown boundary, `replayDiags`' exactly-once in-order replay after
  terminal restoration on every controlled exit, the unified
  controlled-failure path, `EscapePath`-single-lined embedded
  filenames, and the `VRG_TEST_DIAG_ACK` application-side
  acknowledgement
- [viewport-scrolling.md](viewport-scrolling.md) — Issue #12: the
  rendered-row scroll units (`up`/`down` one row, `u`/`d` half page,
  `pgup`/`pgdn` a page of the content height), the BOF/EOF clamps with
  no avoidable blank rows, placeholder no-ops, the per-file saved
  viewport map, and prepared-row rendering with the visible-range-only
  render-cost guard
- [match-navigation.md](match-navigation.md) — Issue #13: the single
  global matched-line cursor (`Cursor`/`Move`, `Next`/`Prev` circular
  steps, the zero- and one-stop strict no-ops, one stop per matched
  line), the cursor-derived current file with its load request and
  saved-viewport handoff, the list underline following selection,
  scroll-independent navigation, and the passive file list
- [destination-reveal.md](destination-reveal.md) — Issue #14: the
  display target as the first submatch's start cell, rendered-row
  reveal, visible-target no-scroll, one-third placement with BOF/EOF
  precedence, saved-viewport versus first-visit starting points, and
  the moving/no-scroll saved-state rules at navigation and
  startup-after-load
- [file-change-popup.md](file-change-popup.md) — Issue #15: the
  selection-time centred path pop-up on `n`/`p` file crossings, the
  instance-keyed one-second expiry with stale-instance rejection,
  dismiss-plus-normal-action key routing, render-time centring and
  left-truncation on resize without timer restart, error-overlay
  cancellation with no return, and the sanitized single-line path sink
- [wrap-mode.md](wrap-mode.md) — Issue #16: wrapping on by default at
  grapheme-cluster boundaries, the `w` toggle between wrapped rows and
  run-off-edge clipping, structural eight-column tab stops, the
  FileBuffer-owned `Clusters` segmentation shared by Viewport, the
  blank-cell rule for an unfit cluster, blank continuation gutters,
  the reserved 0/1 indicator width, the swappable row model keyed by
  (path, content revision, text width, wrap mode), and wrapped
  `TargetRow` reveal
- [logical-anchor.md](logical-anchor.md) — Issue #17: the
  width-independent `(source line, display column)` anchor retained
  through rewrap/toggle/resize, the scroll and moving-reveal
  replacement rules with the no-scroll reveal's retained column, the
  intentionally lossy EOF clamp updating the anchor, `Restore`'s
  containing-row mapping, `viewport.Prepare` moved off `Update` behind
  keyed `layoutReadyMsg` completions with `layoutReqs` dedup,
  `layoutKey` install-only-on-match guards, `currentRows` stale-model
  hiding, `pendingReveals` intents committing on install, the
  cached-file stale-layout re-request and fast path, and the
  visible-window-only render-cost bounds
- [horizontal-panning.md](horizontal-panning.md) — Issue #18: the
  run-off-edge horizontal window, the `,`/`.`/`<`/`>`/`[`/`]` pan
  units (one cell, ten cells, `HalfText`), the visible-lines extent
  policy and the three distinct extent definitions, the
  paintable-boundary `MaxOff`/`Line.MaxStart` clamp, re-clamping on
  every visible-set change with no restoration, wrap-mode retention
  and re-entry clamping, the file-change `ResetOff` before reveal,
  split-cluster blank cells at the left edge, the `Extent` interface,
  and the visible-rows-only extent-evaluation guard
- [horizontal-reveal.md](horizontal-reveal.md) — Issue #19: the
  minimal horizontal reveal at startup and on every `n`/`p` —
  painted-cell visibility (`CellVisible`), the left/right offset
  arithmetic with cluster widths, the oversized-match start-cell
  rule, and the unpaintable-cluster geometric fallback with its
  no-loop guarantee and indicator interplay
- [hidden-content-indicators.md](hidden-content-indicators.md) —
  Issue #20: the per-line gutter `_`/`*` for text and matches hidden
  left, the current-matched-line-only reserved right `*` for matches
  hidden right (absent when that line is off-screen, never
  overwriting text), painted-cell visibility after grapheme clipping
  excluding the reserved column, split-glyph blanks counted hidden,
  partial visibility counting visible, and wrap mode drawing neither
  indicators nor the column
- [grapheme-highlight-expansion.md](grapheme-highlight-expansion.md) —
  Issue #21: nonempty match spans snapped outward to whole grapheme
  clusters in `makeLine` (`CellsCovering` + `expandToClusters`), the
  expanded `Line.Highlights` the single source for painting, reveal,
  and indicators, combining-only matches covering their base cluster,
  the Issue #43 `◌` fallback cell for standalone combining clusters,
  wide glyphs and ZWJ sequences never split, and wrap/clip filler
  blanks never styled as match cells
- [line-structure.md](line-structure.md) — Issue #22: LF/CRLF as
  undisplayed terminators retained in `Raw`, the standalone-CR `^M`
  escape, the unterminated final line and no-phantom-line rules, the
  empty file's zero lines and three-cell gutter, the raw-file /
  rg-line / display-cell coordinate separation (`SearchBytes`,
  `searchOff`), terminator-to-EOL mapping for zero-width positions
  and removed bytes, visible-text-only highlights across
  terminators, and the leading UTF-8 BOM's three-byte adjustment
  with non-leading `U+FEFF` as content
- [zero-width-markers.md](zero-width-markers.md) — Issue #23: the
  one-cell inverse-video marker for zero-width matches, in-text
  marking without shifting text, the end-of-line marker's effective
  width extension (`Line.Extent()`, an empty matched line's extent
  1), cluster-start mapping, the marker's own wrap row after a full
  one, marker cells as reveal targets feeding pan clamping
  (`MaxStart` candidate, marker-only `MaxOff` 0) and the
  `_`/`*` indicators, and the terminator-only `$` on `hit\r\n` as an
  ordinary marker with no special cases
- [file-list-layout.md](file-list-layout.md) — Issue #24: the
  three-term list-width formula (`longest + 2`, `floor(0.40 × w)`,
  terminal minus gutter + 10 + reserved indicator) with the
  nonnegative clamp, `floor` rounding, recompute through the keyed
  layout path on load/gutter/mode/resize/hide-show, the zero-width
  allocation that retains the visibility preference, `left`/`tab`
  and `right`/`shift+tab` toggles with the list initially shown,
  grapheme-safe `…` left-truncation, active-entry auto-scroll, the
  filename-rule status-note slot (synthetic until Issues #26/#29/#30
  supply notes), anchor preservation through every relayout, and the
  visible-window-only render-cost guard
- [async-load-isolation.md](async-load-isolation.md) — Issue #25:
  navigation during held loads, `(raw path, request identity)`-keyed
  `fileLoadedMsg` completions with per-path cache/status updates and
  panel isolation, the dropped-not-queued one-load-per-path rule,
  session-long buffer retention, post-cancellation rejection, the
  filebuffer `Read`/`Decode` split, and the separately gated
  decode/map phase with its actionable-input list
- [read-failures.md](read-failures.md) — Issue #26: the
  "(unreadable)" placeholder with the current-file error overlay and
  retained stops, the non-current diagnostic-only policy and its
  mid-session visibility limits, the same-file-step versus
  cross-file-entry retry distinction, the five-step re-entry sequence
  (prior-failure overlay, "Loading…", exactly one retry, settlement
  independent of dismissal, the append-preserving-scroll primitive),
  composed-view robustness, the `WithLoader` injected-loader seam,
  and load failures never changing the fixed exit status
- [explicit-reload.md](explicit-reload.md) — Issue #27: the `r`
  reread route that never reruns `rg` or touches cursor stops, the
  dropped-not-queued duplicate under the one-load-per-path rule and
  the placeholder as the only completion signal, content revisions
  re-keying layouts so pre-reload preparations die on arrival,
  `currentRows` hiding the in-flight model, failure replacement
  through the Issue #26 overlay, the `pendingAnchor` intent recorded
  at load completion and committed on the matching install, the
  lossy shrink clamp, the one-stop retry route, and cache stability
  until `r`
- [load-completion-reveal.md](load-completion-reveal.md) — Issue #28:
  the two-stage load-completion contract — stage one files the
  buffer/revision and requests the keyed layout while making no
  row-based decision, the model-carried reveal intent always
  resolving the latest selected target (cluster-expanded start cell
  or marker cell) at install, obsolete layouts discarded without
  consuming intents, the installation-guarded commit's
  visible/no-scroll then one-third placement plus horizontal reveal,
  startup visible/hidden cases, and reload-intent arbitration where
  navigation during a load — even away-and-back ending on the same
  stop — replaces the anchor intent
- [stale-match-validation.md](stale-match-validation.md) — Issue #29:
  per-submatch validation on every load (line existence, range
  validity against the search-byte view, recorded-bytes equality),
  dropped submatches marking the buffer stale while survivors keep
  their highlights and markers, the three fallback landings (first
  survivor's cell, clamped recorded start, last source line) with no
  invented highlights or markers, the persistent "file changed since
  search" filename-row note recomputed per reload, UTF-16/32
  exclusion, and the all-stale outcome-matrix row proving the fixed
  status unchanged
- [unsupported-encodings.md](unsupported-encodings.md) — Issue #30:
  `detectEncoding` classifying the four UTF-16/32 BOMs with the
  longer-before-shorter overlap ordering (UTF-32 LE `FF FE 00 00`
  before UTF-16 LE `FF FE`) and the UTF-8 BOM never misclassified,
  the line-free buffer skipping stale validation entirely, the
  "(unsupported encoding)" placeholder via the `placeholder(path)`
  selector, the `cannot display …: unsupported encoding <name>`
  diagnostic under the current/non-current notification split,
  retained cursor stops and the unchanged `r` reload route, no
  forced encoding flag in the child argv, and the all-unsupported
  outcome-matrix row proving the fixed status unchanged
- [help-overlay.md](help-overlay.md) — Issue #31: the `h`/`?` modal
  help dialog over browse and no-results, the shared `scrollBox`/
  `compositeBox` component refactored out of the error overlay,
  wrapped-and-scrollable rows including split unbroken strings,
  tiny-size clipping with no borderless fallback, the modal key
  routing and precedence stack, pop-up cancellation on open, the
  `helpBindings` single binding-table data source, and the escaped
  `helpFooter` slot Issue #34 fills with the scale-and-limits note
- [overlay-precedence.md](overlay-precedence.md) — Issue #32: the
  composed key-precedence stack (`ctrl+c` over modal error over help
  over pop-up over base keys), error-suspends-help with retained
  scroll, append-preserving error scroll generalized, permanent
  pop-up cancellation by help and error, the full `q`/`Esc`
  dismissal-outcome table including the fatal-overlay exit-2 route,
  and `Esc` never exiting a base state
- [terminal-too-small.md](terminal-too-small.md) — Issue #33: the
  20-column/3-row gate — centred clipped "Terminal too small", the
  `q`/`ctrl+c`-only key map with `q` exiting the state-applicable
  status rather than dismissing a hidden overlay, `Esc` and all other
  keys strict no-ops, state-preserving layout/key/completion gates,
  full cursor/anchor/viewport/modal-stack recovery at the final
  dimensions after in-gate resizes, and the pop-up timer running on
  with the pop-up never composited
- [documentation-limits.md](documentation-limits.md) — Issue #34: the
  root `README.md` as the single user-facing documentation artifact,
  the `internal/docs` shared structured source rendering both the
  README limits section and the help-overlay footer, the
  synchronization tests asserting bindings/flags/exit-statuses/
  scale/memory against `helpBindings`, `optionDecls`, and
  `DecideOutcome`, the three independent scale examples with the
  64 MiB/base64/session-retention limits, and the "help footer note"
  sink-safety row
- [final-verification.md](final-verification.md) — Issue #35: the
  closing verification pass over the composed implementation — the
  clean-checkout `go build`/`go vet`/`go test ./...` gates, the
  uncached `-count=1` rerun of the Issue #4/#9/#11 PTY/subprocess
  suites, and the final binary's five smoke outcomes (browse 0,
  no-results 1, fatal 2 via `q` and `Esc`, cancellation 130 with the
  child gone and reaped, help-only 0 with the sentinel rg never
  invoked) — all green with no regressions
- [integrity-diagnostics.md](integrity-diagnostics.md) — Issue #36:
  the structured `Integrity.Causes` model (one `Cause{Kind, Path}` per
  offending physical record) with its stable per-kind diagnostic text,
  the one-cause precedence (extra-summary over after-summary,
  post-summary lifecycle suppression including the removed `context`
  exemption, the tail resolved at `Integrity()` time), detection-order
  plus end-of-stream ordering by unsigned raw-path bytes, uncapped
  deterministic multiplicity, `EscapePath` escaping, dual
  representation of post-summary record loss, and the universal
  process → integrity → record-loss → warning composition shared by
  overlay and replay with no status line for 0/1 exits
- [oversized-diagnostics.md](oversized-diagnostics.md) — Issue #37:
  the always-emitted pluralized oversized aggregate (`1 oversized
  record skipped` / `N oversized records skipped`) regardless of path
  recovery, per-path details deduplicated by raw path in
  first-occurrence order beneath the aggregate, the anonymous-record
  guarantees (fatal overlay never empty at exit 2; visible overlay
  plus stderr replay at exit 0), mixed-recoverability totals, and the
  component's slot in the universal order
- [panel-text-width.md](panel-text-width.md) — Issue #38: the
  terminal → panel → text width chain (panel =
  `width − listWidth − 1`, text = panel − gutter − reserved
  indicator), the one-cell separator column rendered even while the
  list is hidden, the layout key's `TextWidth` as the single width
  every wrap/clip/pan/reveal/indicator measures against, and the
  install paths (load, resize, wrap toggle, list hide/show,
  navigation) that re-key it
- [unified-rendering.md](unified-rendering.md) — Issue #39: the
  single ANSI-aware `safepresentation.CellWidth` every
  display-geometry consumer routes through (line rendering,
  highlight styling, centering, list-entry padding, indicator
  sizing, filename-row fitting, pop-up truncation/width/centering,
  theme overlay sizing), the cluster-driven file-panel renderer
  drawing straight from `Line.Clusters`, the `truncateLeftCells`
  replacement by shared cluster/cell primitives, and the mechanical
  `utf8.DecodeRuneInString` allow-list scanning `internal/` and
  `cmd/`

## Catalogs

- [source-code.md](source-code.md) — all files under `cmd/` and
  `internal/` with purposes
- [unit-tests.md](unit-tests.md) — unit and subprocess-boundary test
  catalog

## Bookkeeping

- [log.md](log.md) — chronological ingest/query/lint record

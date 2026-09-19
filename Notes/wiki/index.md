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

## Catalogs

- [source-code.md](source-code.md) — all files under `cmd/` and
  `internal/` with purposes
- [unit-tests.md](unit-tests.md) — unit and subprocess-boundary test
  catalog

## Bookkeeping

- [log.md](log.md) — chronological ingest/query/lint record

# Wiki index

Catalog of all wiki pages for the vrg project.

## Overview

- [project-overview.md](project-overview.md) — what vrg is, stack, layout

## Concepts and architecture

- [cli-foundation.md](cli-foundation.md) — Issue #1 CLI: mow.cli adapter,
  emission-prevention output strategy, shared declarations, preflight,
  help/positional/root/sanitization contracts
- [cli-flag-forwarding.md](cli-flag-forwarding.md) — Issue #2 CLI: search
  flag allow-list, ordered exact-spelling scan records, combined shorts,
  cumulative `-u`, assignment rejection, exact child argv
- [search-collection-path.md](search-collection-path.md) — Issue #3
  (extended by Issues #4, #8, #9, #10, and #11): ripgrep subprocess
  execution, dual-pipe drainage, SearchIndex parsing with binary-file
  exclusion and stream-integrity accounting, the searching/summary/no-results TUI
  lifecycle, cancellation, child termination/reaping, terminal
  restoration, controlled-failure cleanup, the fatal/warning
  outcome matrix with modal error overlay, and the session diagnostic
  collection with post-restoration stderr replay
- [browse-tracer.md](browse-tracer.md) — Issue #5 (extended by Issue #7,
  Issue #12, Issue #13, and Issue #15): two-pane browse view with file list,
  filename rule, gutter, true-inverse match highlights, current-match
  underline, async file loading with "Loading…" placeholder, safe-presentation core for
  paths and content, theme toggle (`c`), hostile-fixture sink-safety
  method, manual vertical scrolling with per-file saved viewport state,
  prepared-row rendering, the circular matched-line cursor with
  cursor-derived current file, cross-file load requests, viewport
  handoff, passive file list, and the transient centred file-change
  pop-up with instance-keyed one-second expiry, keypress dismissal,
  error-overlay cancellation, and safe left-truncated centred rendering
- [safe-presentation.md](safe-presentation.md) — Issue #6: shared
  safe-presentation utility for every output sink (path, content,
  diagnostic escaping), unified cli.Escape, shared sink-safety table
  and hostile fixtures, no-style composition method, and future-issue
  sink ownership rule
- [theme-module.md](theme-module.md) — Issue #7: active colour scheme
  (dark/light), `c` toggle with no persistence, true-inverse match
  styling, current-match underline, current-file underline, indicator,
  overlay, filename-rule, file-list, and base/gutter styles
- [manual-vertical-scrolling.md](manual-vertical-scrolling.md) —
  Issue #12: manual vertical scrolling with one-row, half-page, and
  full-page scroll units; BOF/EOF clamping with no avoidable blank
  rows; natural unused rows for short files; loading placeholder
  no-op; per-file saved vertical viewport state; prepared-row
  rendering via RowProvider; and the visible-range-only render-cost
  guard
- [destination-reveal.md](destination-reveal.md) — Issue #14: vertical
  destination reveal; display target as the first submatch's start
  cell; rendered-row reveal; visible-target no-scroll; one-third
  placement with BOF/EOF precedence; saved viewport versus
  top-of-file starting sequence; saved-state replacement for moving
  and no-scroll reveals; startup-after-load and navigation triggers
- [outcome-contract.md](outcome-contract.md) — Issue #9 (extended by
  Issues #10 and #11): stream-integrity accounting, separate
  process-success and stream-integrity assessment, the fatal/warning
  outcome matrix, the modal error overlay, exit statuses, stderr
  capture and fallback diagnostics, record-loss inputs and outcome
  rows, and the post-restoration stderr replay with session
  diagnostic collection
- [record-robustness.md](record-robustness.md) — Issue #10: robust
  handling of malformed, oversized, and unknown-type records with
  separate counters, the 64 MiB record limit with
  discard-and-resynchronize behavior, sanitized oversized-record
  diagnostics, unknown-type counting rules, missing-`end` retention,
  and record-loss outcome rows with after-filtering usable-results
  assessment
- [wrap-mode-and-grapheme-policy.md](wrap-mode-and-grapheme-policy.md) —
  Issue #16: wrapping on by default, `w` toggle, shared grapheme
  segmentation and cell-width policy, eight-column tab stops, the
  prepared swappable row model keyed by (path, content revision, text
  width, wrap mode), reserved indicator width, wrapped destination
  reveal, and continuation-row blank gutters
- [logical-anchor-and-layout-preparation.md](logical-anchor-and-layout-preparation.md) —
  Issue #17: width-independent logical viewport anchor with retention
  through rewrap/wrap-toggle/resize and replacement by scrolling/moving
  reveals, intentionally lossy EOF clamp, off-UI layout preparation
  keyed by (path, content revision, text width, wrap mode),
  installation-only-on-match guards with out-of-order discard,
  model-carried pending reveal intent, cached-file stale-layout
  navigation with matching-layout fast path, AC6 responsiveness during
  gated preparation, and render-cost guards for visible rows and
  file-list entries
- [horizontal-panning.md](horizontal-panning.md) — Issue #18:
  horizontal panning in run-off-edge mode (`,`.`<>` `[]` pan units),
  paintable-boundary maximum from the widest visible line, three
  distinct width definitions (content extent, extent policy, maximum
  valid offset), visible-set re-clamping with no restoration after
  destructive clamping, retention through wrap toggles with re-entry
  clamping, reset on file change, grapheme-safe blank-cell clipping,
  and the visible-row render-cost guard
- [horizontal-reveal.md](horizontal-reveal.md) — Issue #19: minimal
  horizontal reveal of the first-submatch start cell in run-off-edge
  mode; painted-cell visibility (split clusters are not painted);
  minimum movement (right-edge arithmetic for right-side, left-edge
  for left-side); unpaintable cluster geometric fallback with no
  panning loop; oversized matches reveal only the start cell; startup
  and navigation triggers through `revealTarget`; file-change reset
  ordering before reveal; `WithWrapMode` test seam
- [hidden-content-indicators.md](hidden-content-indicators.md) —
  Issue #20: run-off-edge hidden-content indicators — left `_`/`*` in
  the first trailing gutter space (text hidden left vs. match entirely
  hidden left), right `*` in the reserved column on the current
  matched line's visible row when a match is entirely hidden right,
  off-screen absence of the right indicator, no-overwrite guarantee,
  partial visibility counts as visible, visibility over rendered
  cells after grapheme clipping with the reserved column excluded,
  split-glyph blanks do not count as visible, and wrap mode has no
  indicators or reserved column
- [grapheme-cluster-highlight-expansion.md](grapheme-cluster-highlight-expansion.md)
  — Issue #21: match highlights expand to grapheme-cluster boundaries
  so they never split a cluster; combining-only matches highlight the
  whole base cluster; standalone zero-width clusters receive a visible
  fallback cell; wide glyphs and ZWJ sequences are never split;
  multi-cell escaped forms (ESC → `^[`) are preserved; wrap and clip
  blank filler cells are never painted as match cells; expanded
  `Highlights` and `ByteCells` are the single source for Viewport, App,
  Issue #19 reveal, and Issue #20 indicators; `ContentDisplay.ByteOffsets`
  added for raw-byte-to-cluster mapping
- [structural-line-handling.md](structural-line-handling.md) — Issue #22:
  LF/CRLF undisplayed terminators with retained original bytes;
  standalone CR escaped as `^M`; unterminated final line counted; no
  phantom trailing line; empty file zero lines with three-cell gutter;
  terminator bytes and zero-width positions map to display end-of-line
  column; span covering text plus terminator highlights visible text
  only; leading UTF-8 BOM invisible with rg-line ↔ raw-file coordinate
  separation (`Buffer.BOMOffset`); non-leading U+FEFF as ordinary
  content
- [zero-width-match-markers.md](zero-width-match-markers.md) —
  Issue #23: zero-width submatches render as one inverse-video cell at
  their mapped location (cluster-start mapping, EOL extension by one
  cell, empty matched line width one); markers participate in wrap,
  clip, extent, pan clamping, reveal, and Issue #20 indicators like
  any other cell; terminator-only `$` marker is an ordinary marker
  with no special cases
- [file-list-layout.md](file-list-layout.md) — Issue #24: responsive
  file-list width formula (longest path + 2, 40% cap, leave 10 cells
  plus reserved indicator), hide/show toggles (`left`/`tab` hide,
  `right`/`shift+tab` show, initially shown), grapheme-safe left
  truncation with leading `…`, active-entry auto-scroll, visible-window
  rendering only, filename-row buffer-status note slot with synthetic
  test seam, pathological-dimension clamping, and Issue #17 prepared-
  layout routing with anchor preservation
- [async-load-isolation.md](async-load-isolation.md) — Issue #25:
  keyed asynchronous load isolation — navigation during loads, raw-path
  and request-identity keyed completions, panel isolation for late
  completions, one-load-in-flight-per-path with dropped re-entry
  requests, session-long buffer retention with no eviction,
  post-cancellation rejection, and separately gated decode/map phase
  with `ctrl+c`/`n`/`p`/`w`/`c`/resize all actionable
- [read-failures-and-retry.md](read-failures-and-retry.md) — Issue #26:
  read-failure state and `(unreadable)` placeholder, non-fatal
  read-failure overlay with sanitized diagnostics, non-current
  diagnostic-only collection, same-file-step no-retry versus
  cross-file exactly-one retry, the five-step re-entry sequence with
  prior-failure overlay reopen, exactly-one in-flight retry,
  Esc-without-disturbance, settlement presentation, and
  append-preserving-scroll on second failure, composed-view
  robustness through Issue #24's slot, and load failures never
  changing the fixed search-derived exit status (all-fail fixed-0,
  current-file fixed-2, composed all-fail-with-fixed-2)

## Catalogs

- [source-code.md](source-code.md) — all files under `cmd/` and
  `internal/` with purposes
- [unit-tests.md](unit-tests.md) — unit and subprocess-boundary test
  catalog

## Bookkeeping

- [log.md](log.md) — chronological ingest/query/lint record

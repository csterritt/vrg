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
  Issue #12, and Issue #13): two-pane browse view with file list,
  filename rule, gutter, true-inverse match highlights, current-match
  underline, async file loading with "Loading…" placeholder, safe-presentation core for
  paths and content, theme toggle (`c`), hostile-fixture sink-safety
  method, manual vertical scrolling with per-file saved viewport state,
  prepared-row rendering, and the circular matched-line cursor with
  cursor-derived current file, cross-file load requests, viewport
  handoff, and passive file list
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

## Catalogs

- [source-code.md](source-code.md) — all files under `cmd/` and
  `internal/` with purposes
- [unit-tests.md](unit-tests.md) — unit and subprocess-boundary test
  catalog

## Bookkeeping

- [log.md](log.md) — chronological ingest/query/lint record

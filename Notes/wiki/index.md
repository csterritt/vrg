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

## Catalogs

- [source-code.md](source-code.md) — all files under `cmd/` and
  `internal/` with purposes
- [unit-tests.md](unit-tests.md) — unit and subprocess-boundary test
  catalog

## Bookkeeping

- [log.md](log.md) — chronological ingest/query/lint record

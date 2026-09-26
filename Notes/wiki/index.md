# Wiki index

Catalog of all wiki pages for the vrg project.

## Overview

- [project-overview.md](project-overview.md) — what vrg is, stack, layout

## Concepts and architecture

- [cli-foundation.md](cli-foundation.md) — Issue #1 CLI: mow.cli adapter,
  emission-prevention output strategy, shared declarations, preflight,
  help/positional/root/sanitization contracts
- [cli-flags-and-child-argv.md](cli-flags-and-child-argv.md) — Issue #2:
  flag allow-list, ordered exact-spelling forwarding, combined shorts,
  cumulative `-u` cap, `=` rejection, `--` protection, exact child argv
- [searchindex-records-and-stops.md](searchindex-records-and-stops.md) —
  Issue #3: rg JSON record decoding (text/bytes encodings, schema
  ranges, unknown types), stop merging and ordering, highlight union,
  working-directory path resolution
- [search-spawn-and-searching-screen.md](search-spawn-and-searching-screen.md) —
  Issue #3: rg spawn from the protected argv in the invocation working
  directory, concurrent dual-pipe drainage, `Searching…` across
  collection and post-exit preparation, interim summary, `q` → exit 0,
  start failure → sanitized diagnostic + exit 2
- [cancellation-and-cleanup.md](cancellation-and-cleanup.md) — Issue #4:
  `ctrl+c`/`q` cancellation through the gate-held preparation window,
  exit 130, terminate-and-reap on every controlled exit, reap-evidence
  side channel, display + termios restoration, the single
  post-restoration stderr diagnostic, the `VRG_TEST_*` seams, and the
  fake-rg/PTY harness
- [browse-tracer.md](browse-tracer.md) — Issue #5: two-pane browse view
  (raw-path file list, underlined current entry, filename rule,
  right-justified gutter, inverse-video matches), async prepared-buffer
  loading with `Loading…`, the safe-presentation core's path/content
  rules and byte→cell maps, the provisional tab form, the no-style
  sink-safety method, and the provisional list width pending Issue #24
- [safe-presentation.md](safe-presentation.md) — Issue #6: the shared
  `internal/present` utility (`Path`/`LineOf`/`Diagnostic`), the
  canonical per-sink-class escaping contracts, the `cli.Escape`
  replacement, the shared sink-safety table with its fixtures and
  no-style composition method, and later-sink row ownership
- [no-results-and-binary-exclusion.md](no-results-and-binary-exclusion.md) —
  Issue #8: `binary_offset` end records dropping a file's collected
  stops with a distinct excluded-file tally, usable results as retained
  stops (`LineCount`), and the centred "No results found" screen with
  its "(N binary files skipped)" suffix — `q` → 1, `Esc` no-op,
  `ctrl+c` → 130
- [error-overlay-and-fatal-outcomes.md](error-overlay-and-fatal-outcomes.md) —
  Issue #9: the stream lifecycle transition matrix (`Feed`,
  `IntegrityFailures`, `Stop.Incomplete`), the pure `decideOutcome`
  function and the full outcome table, the modal diagnostics overlay
  (scroll, `q`/`Esc` dismissal, fatal-only `Esc` exit, `ctrl+c`
  override), stderr classification with generated code-or-signal
  diagnostics, and the fixed-status rule
- [record-robustness.md](record-robustness.md) — Issue #10: the
  deterministic malformed/oversized/unknown disposition matrix, the
  64 MiB record limit with discard-and-resynchronize and path-recovery
  diagnostics, the triple-disposition unterminated oversized final
  record, unknown-type counting and its after-`summary` interaction,
  and the record-loss outcome rows
- [theme-and-colour-toggle.md](theme-and-colour-toggle.md) — Issue #7:
  the dark/light schemes (white-on-black / black-on-white), the
  in-memory `c` toggle, the named style set (base, gutter, true-inverse
  match, underlined current match, inverse indicator, single-line-
  bordered overlay, filename rule, file list, current-file underline),
  and how browse rendering consumes it
- [stderr-replay.md](stderr-replay.md) — Issue #11: the session
  diagnostic collection independent of display, incremental child-stderr
  collection through `diagMsg` on the unbuffered event channel, the
  processed-versus-in-flight shutdown boundary, `ReplayTo` on the common
  post-restoration writer for every controlled exit, controlled-failure
  unification, and the `VRG_TEST_DIAG_ACK_FILE` acknowledgement seam
- [viewport-scrolling.md](viewport-scrolling.md) — Issue #12: the
  rendered-row scroll units (one row, `max(1, floor(h/2))` half page,
  full content-height page), BOF/EOF clamping with no avoidable blank
  rows, the placeholder no-op, per-file saved top row restored on load,
  and prepared `viewport.Rows` rendering queried only for the visible
  range

## Catalogs

- [source-code.md](source-code.md) — all files under `cmd/` and
  `internal/` with purposes
- [unit-tests.md](unit-tests.md) — unit and subprocess-boundary test
  catalog

## Bookkeeping

- [log.md](log.md) — chronological ingest/query/lint record

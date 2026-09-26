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
- [match-navigation.md](match-navigation.md) — Issue #13: the single
  global matched-line cursor in `Index` (`Current`/`Next`/`Prev`
  returning `Step` with `Moved`/`FileChanged`/`Wrapped`), path-then-line
  stop order with startup on the first stop, circular `n`/`p` with
  zero- and one-stop strict no-ops, the cursor-derived current file
  with load requests and the saved-viewport handoff, manual scrolling
  leaving the cursor unchanged, and the passive file list
- [destination-reveal.md](destination-reveal.md) — Issue #14: the
  display target (first submatch's start cell, marker cell for
  zero-width), `Rows.RowOf` resolving it to a rendered row, the
  visible-target no-scroll and `floor(height/3)` placement with
  BOF/EOF precedence, the saved-or-top starting sequence on entry
  including the startup-after-load trigger, and the move-driven
  saved-state replacement rule
- [file-change-popup.md](file-change-popup.md) — Issue #15: the
  selection-time file-change pop-up (one-second instance-keyed timer
  with stale-expiry rejection, any-key dismissal that still performs
  the key's action, render-time centring and left-truncation surviving
  resize, error-overlay cancellation with no return, safe `present.Path`
  display, and the current-file load-failure overlay trigger)
- [wrap-mode.md](wrap-mode.md) — Issue #16: wrap on by default with the
  `w` toggle to run-off-edge, grapheme-boundary wrapping with the
  blank-cell rule for unfit clusters, the single shared
  segmentation/cell-width policy (`present.Cell` `Lead`/`Cont` with
  FileBuffer as `viewport.Source`), eight-column tab stops replacing
  the provisional `→`, blank continuation gutters, the reserved
  indicator width of zero or one, and the swappable row model keyed by
  path, content revision, text width, and wrap mode
- [logical-anchor-and-layout.md](logical-anchor-and-layout.md) —
  Issue #17: the width-independent logical anchor (`Row.Start`, anchor
  resolution, scroll/moving-reveal replacement, no-scroll column
  retention, deliberately lossy EOF clamp), off-UI layout preparation
  with (path, revision, width, wrap) keys and install-only-on-match
  guards discarding out-of-order completions, the pending reveal
  intent committing for the newest stop, cached-file stale-layout
  navigation with its fast path, and the visible-range render-cost
  guarantees for rows and list entries
- [horizontal-panning.md](horizontal-panning.md) — Issue #18: the six
  pan units (1, 10, `max(1, floor(text width / 2))` columns), the
  visible-lines extent policy with its paintable-boundary maximum and
  unpaintable-cluster exception, re-clamping on every visible-set
  change with no restoration, wrap-mode dormancy and re-entry
  clamping, the file-change offset reset, and grapheme-safe clipping
  with split-cluster blanking and span translation

## Catalogs

- [source-code.md](source-code.md) — all files under `cmd/` and
  `internal/` with purposes
- [unit-tests.md](unit-tests.md) — unit and subprocess-boundary test
  catalog

## Bookkeeping

- [log.md](log.md) — chronological ingest/query/lint record

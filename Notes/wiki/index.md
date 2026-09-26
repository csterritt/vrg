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
  sink-safety method, and the Issue #24 file-list layout
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
  and the record-loss outcome rows; Issue #37's always-emitted
  pluralized oversized aggregate, per-distinct-raw-path detail dedup,
  and never-silent anonymous records
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
- [minimal-horizontal-reveal.md](minimal-horizontal-reveal.md) —
  Issue #19: painted-cell visibility as the reveal criterion, minimum
  offset arithmetic using cluster widths (left hidden → start; right
  hidden/clipped → `start + cluster width − text width`), the
  oversized-match start-cell rule, the unpaintable-cluster geometric
  fallback with its no-loop guarantee, and the startup and
  per-navigation triggers after the file-change offset reset
- [hidden-content-indicators.md](hidden-content-indicators.md) —
  Issue #20: the per-line gutter `_`/`*` signposts for hidden-left
  text and entirely hidden matches, the current-matched-line-only
  right `*` in the reserved column with its off-screen absence and
  never-overwrite guarantee, painted-cell visibility after grapheme
  clipping shared with the reveal, partial visibility counting as
  visible, split-cluster blanks counting as hidden, and wrap mode's
  freedom from both indicators and the reserved column
- [grapheme-highlight-expansion.md](grapheme-highlight-expansion.md) —
  Issue #21: `clusterSpan`'s outward expansion of partial spans to
  `Lead` cluster boundaries (start/end/interior), combining-only
  matches highlighting the whole base cluster, the standalone-cluster
  provisional fallback cell, wide pairs and ZWJ sequences never split,
  `Cell.Blank`-marked wrap/clip fillers never painted as match cells,
  and the expanded span as the single source for highlight, reveal,
  and indicator visibility
- [line-terminators-and-bom.md](line-terminators-and-bom.md) —
  Issue #22: LF/CRLF as undisplayed terminators with retained raw
  bytes, standalone CR → `^M`, the unterminated final line and no
  phantom trailing line, the empty file's zero lines behind a
  three-cell gutter, terminator bytes and zero-width positions mapping
  to display end-of-line, text-plus-terminator spans highlighting the
  visible text only, and the leading UTF-8 BOM invisible with its
  three-byte raw-file/rg-line coordinate shift — non-leading U+FEFF
  staying ordinary content
- [zero-width-match-markers.md](zero-width-match-markers.md) —
  Issue #23: the one-cell inverse marker (`Start == End` span) painted
  without shifting text and underlined on the current matched line,
  cluster-start position mapping, the effective-width extension for
  end-of-line and empty lines, the marker overflow row after a full
  wrap row, marker extents in the paintable-boundary maximum, markers
  as reveal targets and indicator inputs, and the terminator-only `$`
  marker's ordinary-rule treatment
- [file-list-layout.md](file-list-layout.md) — Issue #24: the
  three-term list width (longest sanitized path plus two,
  `floor(0.40 × terminal width)`, terminal minus gutter-plus-ten-plus-
  reserved-indicator), the `left`/`tab` hide and `right`/`shift+tab`
  show preference surviving zero-width allocations, grapheme-safe
  leading-`…` truncation, the filename-row buffer-status note slot
  (real notes owned by Issues #26, #29, #30), minimal-movement list
  scrolling, and every width change routed through the Issue #17
  prepared-layout path preserving the logical anchor; plus Issue
  #38's terminal→panel→text width chain (`textW = panelW − gutter −
  reserved`, the single computation the layout key and every viewport
  install site share)
- [async-load-isolation.md](async-load-isolation.md) — Issue #25:
  navigation live while a file loads, load completions keyed by raw
  path plus minted request identity installing only on match, panel
  isolation for non-current completions, one in-flight load per path
  with re-entry dropped not queued, session-long buffer retention,
  post-cancellation rejection, the separately gated decode/map phase,
  and the actionable input list while it is held
- [read-failures.md](read-failures.md) — Issue #26: the
  current-versus-non-current failure split (overlay + `(unreadable)`
  panel and status note versus diagnostic-only), same-file steps never
  retrying while a cross-file entry runs the five-step re-entry
  sequence (prior overlay immediately, exactly one retry, settlement
  without dismissal, second-failure append preserving scroll, away-
  settled retries), and load failures never moving the fixed exit
  status — all behind the injected `readFile` seam
- [explicit-reload.md](explicit-reload.md) — Issue #27: `r` rereads the
  current file without rerunning rg or touching the stops, duplicate
  presses and re-entry dropped not queued with placeholder settlement
  as the completion signal, the cached buffer dropped immediately and
  failures landing as `(unreadable)` plus overlay, per-path content
  revisions discarding superseded prepared layouts, and the
  `pendingIntent` seam committing a reveal for the newest stop or an
  anchor-preserving no-reveal — generalized by Issue #28
- [load-completion-reveal.md](load-completion-reveal.md) — Issue #28:
  the two-stage load-completion contract — stage 1 installs the buffer,
  bumps the revision, and requests the layout keyed to the post-load
  (path, revision, text width, wrap) with no row-based decision; the
  carried intent — newest-selection reveal or undisturbed-reload
  anchor — survives obsolete layouts and commits only on a matching
  install; navigation during the load wins over cursor equality;
  marker and cluster-expanded targets, saved-viewport revisits, and
  non-current/pop-up isolation
- [stale-match-validation.md](stale-match-validation.md) — Issue #29:
  per-submatch validation on every load (line existence, range, and
  byte equality against raw bytes with the UTF-8 BOM shift), invalid
  submatches dropped individually while survivors keep their
  highlights, the `stale` buffer mark, the persistent `file changed
  since search` status note that only a clean reload clears, the
  three clamped fallback landing rules inventing no highlights or
  markers, the fallback riding the Issue #28 commit, and the fixed
  exit status
- [unsupported-encodings.md](unsupported-encodings.md) — Issue #30:
  the four UTF-16/UTF-32 BOM marks detected longest-first (UTF-32 LE
  winning the `FF FE` overlap) with the UTF-8 BOM never
  misclassified, the `(unsupported encoding)` placeholder and
  filename-row note, the collected `cannot display …: unsupported
  encoding <name>` diagnostic under the current/non-current overlay
  split with re-entry re-open, retained stops and `r` reloadability,
  the exclusion of encoded bytes from stale validation, and the fixed
  exit status
- [help-overlay.md](help-overlay.md) — Issue #31: `h`/`?` modal help
  over browse and no-results (close keys `q`/`Esc`/`h`/`?`, `up`/`down`
  scrolling, `ctrl+c`, all other keys ignored), the shared
  wrapped-scrollable overlay component it now shares with the error
  overlay, the single `helpBindings` binding table with the Issue #34
  footer slot, pop-up cancellation on open, error-suspension retention,
  wrapped unbroken text, tiny-size clipping, and the `help overlay`
  sink-safety row
- [overlay-precedence.md](overlay-precedence.md) — Issue #32: the
  `ctrl+c` → modal error → help → pop-up → base key-precedence stack,
  error-suspends-help with scroll restoration for both dismissal keys,
  appended errors preserving the reader's scroll, pop-up cancellation
  without return, the `Esc`/`q` dismissal-outcome table (fatal
  no-results overlay exiting 2 under either key), and `Esc` never
  exiting from a base state
- [terminal-too-small.md](terminal-too-small.md) — Issue #33: the
  20×3 minimum-size gate — centred "Terminal too small" as space
  permits, only `q`/`ctrl+c` active with `q` exiting the
  state-applicable status even past a logically open modal, `Esc` and
  all other keys no-ops, freeze-not-snapshot state preservation
  (cursor, per-file anchors, list/wrap/colour, horizontal offset, the
  modal stack and scroll positions), interior resizes deferring
  recovery to the final dimensions, and the pop-up timer continuing
  without display
- [documentation.md](documentation.md) — Issue #34: the root
  `README.md` as the single user-facing artifact (invocation, embedded
  generated help, binding table, four-status exit table, independent
  scale examples, 64 MiB record limit with the base64 caveat, memory
  limits, ripgrep 15.x/`--no-config`), the `limitNotes` footer note
  shared verbatim between README and help overlay, the README
  synchronization tests asserting against `helpBindings`,
  `optionDecls`, and `decideOutcome`, and the `help footer note`
  sink-safety row
- [final-verification.md](final-verification.md) — Issue #35: the
  clean-checkout verification pass (`go build`/`vet`/`test`, the
  `-count=1` PTY/subprocess re-runs, the five smoke outcomes through
  `scripts/smoke.py`), and the regression it caught — the process-group
  cancellation kill plus the seeded harness's marker alignment
- [stream-integrity-fatal-diagnostics.md](stream-integrity-fatal-diagnostics.md) —
  Issue #36: the structured `Cause`/`CauseKind` integrity model
  (`IntegrityCauses`, `Cause.Line()` through `present.Path`), the
  one-cause-per-physical-record overlap precedence (extra summary,
  post-summary lifecycle suppression, the removed context exemption,
  unterminated-fragment split, dual tallies), deterministic
  detection-order then end-of-stream ordering with unsigned raw-path
  missing ends, uncapped one-line-per-record multiplicity, the
  `outcomeInput`-driven `composeDiagnostics` universal order
  (process → integrity → record-loss aggregates and per-path details →
  unknown warnings) shared by overlay and replay, stderr suppressing
  the generated line, and no process-status line for exit 0/1

## Catalogs

- [source-code.md](source-code.md) — all files under `cmd/` and
  `internal/` with purposes
- [unit-tests.md](unit-tests.md) — unit and subprocess-boundary test
  catalog

## Bookkeeping

- [log.md](log.md) — chronological ingest/query/lint record

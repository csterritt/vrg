# No-results screen and binary exclusion (Issue #8)

The empty-outcome contract delivered by
[Issue #8](../issues/008-no-results-screen-and-binary-exclusion.md),
implemented in `internal/searchindex` (`index.go`) and `internal/app`
(`app.go`, `noresults.go`). Relevant PRD sections in `Notes/PRD-vrg.md`:
*Result index, records, and stream integrity* (the `binary_offset`
bullet) and *Outcome and exit-status contract* (the last table row and
the empty-screen bullet).

## Binary exclusion

A valid `end` record carrying a non-null `binary_offset` drops its file
from the index: every stop collected from that file's earlier `match`
records is removed, and the raw path joins `Index.excluded`, a set — so
the `BinaryExcluded()` tally counts each file once no matter how many
excluding ends arrive. Once a path is excluded, later match records for
it are dropped too — binary exclusion takes precedence over the Issue #9
orphan-retention rule, and applies even to an orphaned `end` (see
[error-overlay-and-fatal-outcomes.md](error-overlay-and-fatal-outcomes.md)
for the full lifecycle matrix). An `end` with `binary_offset:
null` — the ordinary case — excludes nothing, and an excluding `end`
for a file with no collected matches still counts the file.

## Usable results

"Usable results" is the count of *retained stops after filtering* —
binary exclusion now, record skipping (Issue #10) later — never the
number of `match` events received. `Index.LineCount()` is that value and
is the single number the outcome logic consumes: `Update`'s
`searchDoneMsg` case branches on `len(m.stops)` alone. Four match events
for a binary-excluded file plus one for a retained file yield usable
results of 1, and the browse list shows only the retained file.

## The no-results screen

When a completed search has zero usable results the model enters
`phaseNoResults` instead of `phaseBrowse` — unless the outcome is fatal
(fatal process result or integrity failure), where the diagnostics
overlay stands alone as `phaseFatal`. The screen branch, overlay
decision, and exit status all come from `decideOutcome`, Issue #9's
pure outcome function documented in
[error-overlay-and-fatal-outcomes.md](error-overlay-and-fatal-outcomes.md):
the `searchDoneMsg` branch no longer decides on `len(m.stops)` alone.
No load command runs on this screen and the ordinary exit status fixes
at 1 at completion. `renderNoResults`
composes the message centred horizontally on the frame's middle row,
clipped to the frame width, every row padded so `theme.Base` covers the
screen — the same full-width padding as
[the browse view](browse-tracer.md).

- Plain `No results found` for an empty stream.
- `No results found (N binary files skipped)` when `binarySkipped > 0`
  — the suffix applies identically to an all-filtered rg-0 stream and
  an rg-1 empty stream.

Keys on this screen: `q` quits through the ordinary path with the fixed
status 1 (the process boundary's terminate-and-reap cleanup from
[cancellation-and-cleanup.md](cancellation-and-cleanup.md) still runs),
`Esc` is a no-op, and `ctrl+c` overrides the fixed status with 130 via
`cancelled()`. Since Issue #9 the screen can sit beneath a warning
overlay — captured stderr under a benign exit opens the modal overlay
first, and `q`/`Esc` dismiss it to reveal this screen — and a fatal
outcome with zero usable results replaces it entirely with the
fatal-only overlay, where either dismissal key quits at status 2. See
[error-overlay-and-fatal-outcomes.md](error-overlay-and-fatal-outcomes.md).

## Real-rg nuance

rg only reports `binary_offset` when it has already emitted match
events before detecting the NUL: files explicitly named as the search
root always take that path, while files found by walking a directory
are suppressed outright when the NUL lands inside rg's initial
detection window (no `begin`/`match`/`end` at all — just the summary).
To exercise exclusion on a walked tree the first NUL must lie beyond
that window (~64 KiB observed with rg 15.2.0), e.g. thousands of
matching lines followed by a NUL. A small `foo\0bar` file found by
`vrg foo .` produces plain `No results found` — correct, but with no
skip count, because rg emitted nothing to exclude.

## Tests

`internal/searchindex/searchindex_test.go` pins exclusion after earlier
matches, the distinct-file tally (duplicate exclusion, excluded-without-
matches, post-exclusion matches dropped), and usable results as
retained stops. `internal/app/noresults_test.go` pins the rg-1 empty
stream (centred screen, no suffix, no load command), the rg-0
all-binary stream (suffix with the distinct count), the mixed
binary-excluded/retained stream browsing with usable results 1, `q` →
exit 1 without touching cancellation, `Esc` as a no-op, and `ctrl+c` →
130. See [unit-tests.md](unit-tests.md).

## Files

- `internal/searchindex/index.go` — `excluded` set, `exclude`, and
  `BinaryExcluded`; `Add` dispatches on end/match records.
- `internal/app/app.go` — `phaseNoResults`, the `binarySkipped` field,
  and the `searchDoneMsg` outcome branch in `Update` (driven by
  `decideOutcome` since Issue #9).
- `internal/app/noresults.go` — `renderNoResults`, the centred
  composition.
- `internal/app/noresults_test.go` — the outcome tests above.

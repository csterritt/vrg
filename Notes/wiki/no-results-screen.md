# No-results screen and binary exclusion (Issue #8)

[Issue #8](../issues/008-no-results-screen-and-binary-exclusion.md)
lands the last row of the PRD's *Outcome and exit-status contract* and
the binary bullet of *Result index, records, and stream integrity* in
[`Notes/PRD-vrg.md`](../PRD-vrg.md): confirmed binary files are excluded
entirely, and a complete successful search with nothing usable left
presents a centred "No results found" screen whose `q` dismissal exits
1.

## Binary exclusion

`internal/searchindex`'s `Index.Feed` now acts on `KindEnd` records: a
valid `end` event whose `binary_offset` is non-null
(`ParseRecord` already flags it as `Record.Binary`) drops that file and
**all its previously collected matches** — the path's accumulated stops
are deleted outright, so nothing of it survives `Prepare`. Exclusion
matches on decoded path bytes, so a `text`-encoded `end` drops matches
fed as `bytes` and vice versa.

`Index.BinaryExcluded` counts **distinct** excluded files: an internal
`excluded` set keyed by raw path bytes keeps a repeated binary `end` for
an already-counted file from double-counting — while a second binary
`end` still re-drops any matches collected for that path in between.

Exclusion is keyed on completed `end` events only: a file lacking its
`end` is not inferred nonbinary (Issue #9 marks it `Incomplete` —
retained with partial lifecycle metadata — and fails the stream's
integrity; Issue #10's record skipping will join binary exclusion in
the filtering chain).

## Usable results

`Index.UsableResults()` is the count of **retained stops after
filtering** — summed over the prepared `Files` — and is the single value
the outcome logic consumes. It is never the count of `match` events
received: excluded stops do not count, and merged same-line stops count
once. This is the value the remaining outcome-table rows (Issue #9's
fatal/record-loss branches) will consult as they land.

## The no-results screen

When `searchDoneMsg` arrives and the prepared index's usable-results
value is zero, the model enters `stateNoResults` instead of
`stateBrowse` — no load command issues. `View` renders the single line
centred horizontally and vertically (the same `center` helper the
"Searching…" screen uses), wrapped in the theme's base colours:

- `No results found` — a complete rg-1 stream or any search that
  retained nothing.
- `No results found (N binary files skipped)` — appended whenever
  `BinaryExcluded > 0`, i.e. when every matched file was excluded as
  binary. It applies to all-filtered rg-0 streams and rg-1 exits alike;
  N is the distinct-file count.

On this screen `q` exits the fixed status-1 outcome through the Issue #4
`quitCmd`/`reapChild` cleanup path (see
[cancellation-cleanup.md](cancellation-cleanup.md)); `Esc` is a no-op
(no overlay exists to dismiss and `Esc` never quits a base state); and
`ctrl+c` keeps its global 130 override. Issue #9's outcome decision can
open a warning overlay on top of this screen when diagnostics exist —
dismissal reveals the same no-results state; see
[error-overlay-and-outcomes.md](error-overlay-and-outcomes.md). The
record-loss outcome rows stay Issue #10 scope.

> ripgrep 15.x note: `binary_offset` end records appear only for files
> passed as explicit search operands. During directory traversal rg
> detects binary content and emits no records for the file at all, so a
> traversed binary-only search lands on the plain "No results found"
> screen. The suffix is exercised when the root itself is a binary file
> (for example `vrg foo b.bin`).

## Tests

See [unit-tests.md](unit-tests.md):
`internal/searchindex/index_test.go` covers drop-after-earlier-matches,
the distinct-file count, and usable-results accounting;
`internal/app/noresults_test.go` covers the rg-1 empty stream, the
rg-0/rg-1 all-binary stream with its count text, mixed retention
browsing with usable results 1, `Esc` no-op, `q` → 1 through the
cleanup path, and `ctrl+c` → 130.

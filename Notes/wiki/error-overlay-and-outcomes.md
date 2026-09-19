# Error overlay and fatal outcomes (Issue #9)

[Issue #9](../issues/009-error-overlay-and-fatal-outcomes.md) lands the
stream-lifecycle matrix from *Result index, records, and stream
integrity*, the whole *Outcome and exit-status contract*, and the modal
error overlay from *Colours, overlays, and key precedence* in
[`Notes/PRD-vrg.md`](../PRD-vrg.md): the ripgrep JSON stream's lifecycle
is validated while it is consumed, stream integrity is assessed
independently of process success, and a pure outcome function maps the
completed search to a presentation, dismissal behavior, and a fixed
exit status.

## Lifecycle validation

`internal/searchindex`'s `Index.Feed` now runs the full transition
matrix on decoded raw path bytes (a `text`-encoded `end` pairs with a
`bytes`-encoded `begin` of the same path), tracked per path so
interleaved files pair independently:

| Event | Valid | Integrity failure |
| --- | --- | --- |
| `begin(P)` | P not open → P opens | P already open, or P is binary-excluded (exclusion is terminal) |
| `match(P)` | P open → indexes under P | P not open → **retained** under P with `File.Incomplete` set; P excluded → dropped, file stays excluded |
| `end(P)` | P open → P closes; non-null `binary_offset` excludes P | P not open → orphaned/duplicate end |
| `context(P)` | ignored in every position | — (Issue #36 removes the post-summary exemption; Issue #44 makes it fail) |
| `summary` | exactly one, as the final record | a second `summary`, or any other record after it, fails the stream |

Plus the stream-shape rules: a file still open at stream end fails
integrity and is retained with `Incomplete` set; a missing summary
fails the stream; and a trailing unterminated fragment goes through
`Index.FeedTail` — a double disposition of `KindMalformed` plus a
broken-stream mark (Issue #10 counts it in `Index.Malformed`; see
[record-robustness.md](record-robustness.md)). Malformed and unknown
records carry no lifecycle meaning by themselves.

Two retention subtleties: an orphaned match does not "open" the file —
a later `end` for it is still orphaned — and binary exclusion takes
precedence over orphan retention, so a match after a binary-excluding
`end` is dropped and the file stays excluded. A well-formed `binary_offset`
is exclusion evidence even on an orphaned `end`: the retained matches
drop with it. Post-summary records are never dispatched — a match after
`summary` does not index.

`Index.Integrity().Complete` reports the result once feeding finishes:
the summary closed the stream, no violation occurred, and no begun file
was left open. It is assessed **separately from the child's process
result** — rg exiting 0 over a broken stream is still a failure, and a
complete stream under a fatal exit still validates.

## The outcome decision

`internal/app`'s `DecideOutcome` is a pure function of `OutcomeInput` —
the child's `Result` (stdout already consumed into the index), the
`searchindex.Integrity`, the usable-results count, the `RecordLoss`
counts, and caller-composed `Warnings`. It returns the initial
`presentation`, the escaped overlay lines, whether dismissal exits, and
the fixed `Status`. The matrix:

| Process | Stream | Usable results | Diagnostics | Presentation | Status |
| --- | --- | --- | --- | --- | --- |
| 0/1 | complete | > 0 | none | browse | 0 |
| 0/1 | complete | > 0 | any | browse + warning overlay | 0 |
| 0/1 | complete | 0 | none | no-results | 1 |
| 0/1 | complete | 0 | warnings only | no-results + warning overlay | 1 |
| 0/1 | complete | 0 | record loss (Issue #10) | overlay only | 2 |
| fatal (code ∉ {0,1} or signal) | any | > 0 | — | browse + error overlay | 2 |
| fatal | any | 0 | — | overlay only | 2 |
| any | integrity failure | > 0 | — | browse + error overlay | 2 |
| any | integrity failure | 0 | — | overlay only | 2 |

Issue #10 adds the record-loss inputs and rows: on a complete rg-0/1
stream, skipped (malformed/oversized) records with zero usable results
are **fatal** — the record-loss overlay-only row, exiting 2 on `q` or
`Esc` — while the same loss with usable results browses under the
overlay at exit 0. Usable results are assessed after all filtering, so
a skipped record plus a binary exclusion leaving zero retained stops
takes the record-loss fatal row, not the no-results row. Unknown-type
counts remain warnings that never independently change the status —
with zero results the warning overlay still precedes the no-results
screen at exit 1. See
[record-robustness.md](record-robustness.md).

The anomalous rg-1-with-retained-results case browses and exits 0.
`status` is decided once at `searchDoneMsg` — later keys can never
change it; only `ctrl+c` overrides to 130. Issue #26 extends the
matrix with three load-failure rows proving post-search failures
never reopen the fixed outcome: every retained file failing under
status 0 still exits 0, a current-file failure under status 2 still
exits 2, and the composed row — usable results at fixed status 2 with
every retained file subsequently failing — keeps 2 with the failures
confined to file presentation and diagnostics. See
[read-failures.md](read-failures.md).

## Diagnostics and stderr classification

Captured stderr is diagnostic **regardless of exit code**: on a 0/1
exit it opens a warning overlay, on a fatal outcome it is the process
component of the error overlay. When a failed process left no stderr, a
generated line names the exit code (`ripgrep exited with code 3`) or
the wait error's signal (`ripgrep died: signal: killed`). Component
order is process, then a stream-integrity note when the stream was not
whole, then caller warnings. Every line is escaped through
`safepresentation.EscapeDiagnostic` before it is stored, so hostile
bytes can never execute on the terminal — the sink-safety table drives
the hostile fixture set through the overlay's real composition path.
Whether or not an overlay shows them, these diagnostics also enter the
Issue #11 session collection and replay to stderr after terminal
restoration — see [stderr-replay.md](stderr-replay.md).

## The modal error overlay

The overlay is a single-line bordered box (`theme.Overlay`, base
colours) centred on the frame over whatever presentation is beneath —
including a blank frame for `stateOverlayOnly`. Interior text wraps on
grapheme boundaries to the interior width, splitting unbroken strings
mid-run so no row exceeds it. The complete wrapped row set stays in the
model: `up`/`down` scroll one row at a time, clamped to
`[0, rows − visible]`, and the set re-wraps on resize — Issue #41's
contract that a single frame needn't show head and tail at once.

Key precedence while open: `ctrl+c` exits 130 globally; `up`/`down`
scroll; `q`/`Esc` dismiss; every other key is ignored — including `c`,
which cannot toggle the theme through the modal. Dismissal reveals the
base state, except on `stateOverlayOnly` where `q` and `Esc` both exit
with the fixed status — the one place `Esc` terminates. From base
states `Esc` never exits. Since Issue #15, `openOverlay` is the single
open-or-append entry point: it also clears any live file-change
pop-up, which never returns after the overlay closes — see
[file-change-popup.md](file-change-popup.md).

## Tests

See [unit-tests.md](unit-tests.md):
`internal/searchindex/lifecycle_test.go` is the full transition matrix
plus the `FeedTail` double disposition; `internal/app/outcome_test.go`
is the single table-driven outcome matrix asserting initial
presentation, dismissal key, post-dismissal state, and final status;
`internal/app/overlay_test.go` covers key routing, scroll clamping,
unbroken-string wrapping within the border, the complete-diagnostic
contract, and generated diagnostics; `sinksafety_test.go` gains the
error-overlay row; `cmd/vrg/outcome_test.go` drives the real binary on
a pty through the fatal/warning/signal and ≥1 MiB stderr-content
fixture cases.

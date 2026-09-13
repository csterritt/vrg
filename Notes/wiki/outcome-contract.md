# Outcome contract (Issue #9, extended by Issue #10)

The fatal/warning outcome matrix and modal error overlay delivered by
[Issue #9](../issues/009-error-overlay-and-fatal-outcomes.md), adding
stream-integrity accounting, separate process-success and
stream-integrity assessment, a modal error overlay, and exit status 2
for fatal process or stream-integrity outcomes. [Issue #10](../issues/010-record-robustness-malformed-oversized-unknown.md)
extended the matrix with record-loss inputs (malformed, oversized,
unknown) and after-filtering usable-results assessment. Relevant PRD
sections: *Module Design → SearchIndex / App*, *Outcome and exit-status
contract*, *Colours, overlays, and key precedence*. See also
[record-robustness](record-robustness.md).

## Stream integrity

`internal/searchindex` assesses stream integrity separately from process
success. `Index.Integrity()` returns `Integrity{Complete}`, where
`Complete` is true only when every lifecycle rule passed:

- Exactly one `summary`, as the final record (a summary alone is a
  complete zero-result stream).
- No record after the summary (context records are ignored for
  lifecycle purposes).
- Every `begin` opens a file that was not already open (duplicate
  `begin` is an integrity failure).
- Every `match` arrives while its file is open (orphaned match is an
  integrity failure; the match is retained with incomplete metadata).
- Every `end` arrives while its file is open (orphaned/duplicate `end`
  is an integrity failure).
- No file is still open at stream end (missing `end` is an integrity
  failure; existing matches are retained with incomplete metadata).
- No trailing unterminated record (`Builder.MarkTrailingMalformed`
  signals the scanner saw one).

`Stop.Incomplete` marks retained matches whose lifecycle metadata is
incomplete (orphaned match, file still open at stream end). Binary
exclusion takes precedence over orphan retention: a `match` after a
binary-excluding `end` is dropped, not retained. Path identity uses
decoded raw path bytes, so `text` and `bytes` representations of the
same path are identical for lifecycle purposes.

## Process result

`ProcessResult` captures the ripgrep process exit outcome, kept separate
from stream integrity:

- `ExitCode` — the process exit code (signal number when `SignalDeath`
  is true).
- `SignalDeath` — true when the process died from a signal.

`processResult` derives the `ProcessResult` from the child's wait
error. A nil error is exit code 0. An `exec.ExitError` carries the exit
code or, for signal death, the signal number with `SignalDeath` set.

## Outcome matrix

`DecideOutcome(OutcomeInput) Outcome` is the pure outcome decision. The
stream is fatal when the process exits with a code other than 0 or 1,
dies by signal, or the stream integrity fails (incomplete). Issue #10
added record-loss fatal rows: when malformed or oversized records leave
zero usable results, the outcome is a record-loss fatal overlay (exit
2). Unknown-only loss never independently changes exit status; it
produces a warning overlay.

| Process | Integrity | Usable results | Stderr | Record loss | Initial state | Overlay | Fatal | Exit |
|---------|-----------|----------------|--------|-------------|---------------|---------|-------|------|
| 0       | complete  | >0             | none   | none        | browse        | none    | no    | 0    |
| 0       | complete  | 0              | none   | none        | no-results    | none    | no    | 1    |
| 0       | complete  | 0              | any    | none        | no-results    | warning | no    | 1    |
| 0       | complete  | >0             | any    | none        | browse        | warning | no    | 0    |
| 1       | complete  | >0             | none   | none        | browse        | none    | no    | 0    |
| 1       | complete  | >0             | any    | none        | browse        | warning | no    | 0    |
| 1       | complete  | 0              | none   | none        | no-results    | none    | no    | 1    |
| 1       | complete  | 0              | any    | none        | no-results    | warning | no    | 1    |
| fatal   | complete  | >0             | any    | none        | browse        | error   | no    | 2    |
| fatal   | complete  | 0              | any    | none        | no-results    | error   | yes   | 2    |
| any     | incomplete| >0             | any    | none        | browse        | error   | no    | 2    |
| any     | incomplete| 0              | any    | none        | no-results    | error   | yes   | 2    |
| 0       | complete  | >0             | any    | malformed   | browse        | warning | no    | 0    |
| 0       | complete  | 0              | any    | malformed   | no-results    | error   | yes   | 2    |
| 0       | complete  | 0              | any    | oversized   | no-results    | error   | yes   | 2    |
| 0       | complete  | 0              | any    | unknown     | no-results    | warning | no    | 1    |
| 0       | complete  | 0              | any    | malformed + binary exclusion | no-results | error | yes | 2 |

The fixed exit status is decided once at completion and never recomputed
except by `ctrl+c` (which overrides to 130).

### Record-loss inputs (Issue #10)

`OutcomeInput.RecordLoss` carries `Malformed`, `Unknown`, and
`Oversized` counts. `OutcomeInput.RecordLossDiagnostics` carries the
formatted diagnostic text for record-loss counts, combined with stderr
diagnostics for the overlay text. The `recordLossDiagnostics` helper
formats the malformed count ("N malformed record(s) skipped"), the
unknown count ("N unrecognised record types skipped"), and the
per-record oversized path diagnostics.

Usable-results assessment happens after all filtering (binary exclusion
and record loss), so a stream whose sole retained file was
binary-excluded after a skipped record follows the record-loss fatal
row rather than the no-results row.

## Overlay behavior

The modal overlay sits over the browse or no-results state. When
`OverlayFatal` is true, dismissal (q or Esc) exits 2 because there is no
underlying state. When the overlay is non-fatal, dismissal returns to
the base state.

Key routing when the overlay is open:

- `up`/`down` — scroll the overlay content.
- `q` — dismiss (non-fatal) or exit 2 (fatal no-results).
- `Esc` — dismiss (non-fatal) or exit 2 (fatal no-results). This is the
  one case where Esc terminates, because there is no underlying state.
- `ctrl+c` — exit 130 (overrides the fixed status).
- other keys — ignored.

When the overlay is closed, the base state handles keys: `q` quits with
the fixed exit status; `Esc` is a no-op; `ctrl+c` exits 130; `c`
toggles the theme in browse.

## Diagnostics

The overlay text is the captured stderr. When a failed process supplies
no stderr, `generatedDiagnostic` produces a fallback naming the exit
code or signal:

- `ripgrep exited with code N` (for a non-signal fatal exit).
- `ripgrep killed by signal N` (for signal death).

The overlay text is sanitized through `safepresentation.EscapeDiagnostic`
before rendering. Large diagnostics show both the head and tail so the
user sees the beginning and end of the captured stderr.

## Exit statuses

- `0` — clean browse quit (rg 0/1 with results, complete stream).
- `1` — no-results quit (rg 0/1 with no usable results, complete stream).
- `2` — fatal outcome (fatal exit code, signal death, or incomplete
  stream).
- `130` — cancellation (ctrl+c in any state, or q while searching).

Once a search-derived status is fixed, later `ctrl+c` still overrides
it to 130.

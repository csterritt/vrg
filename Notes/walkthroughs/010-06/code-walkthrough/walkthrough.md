# Issue #10: Record robustness

*2026-09-23T20:39:47Z by Showboat 0.6.1*
<!-- showboat-id: 9f745ec9-32c0-4e64-b9c2-4bf02981753b -->

Walkthrough for [Issue #10](../../../issues/010-record-robustness-malformed-oversized-unknown.md), implementing the record-robustness contract per `Notes/PRD-vrg.md` (*Result index, records, and stream integrity* — the malformed/oversized/unknown bullets; *Outcome and exit-status contract* — the record-loss and warning rows; *Resources and responsiveness* — the 64 MiB bullets). Every record position resolves to a deterministic disposition — skipped-and-counted malformed, stream-integrity failure, oversized, or unknown type — and the two composite cases (a trailing unterminated record, a malformed record after `summary`) carry both dispositions. A 64 MiB per-record limit is enforced explicitly with discard-through-newline resynchronization and best-effort path recovery for the diagnostic. Malformed plus oversized skips with zero usable results are a fatal record-loss outcome; with usable results they are warning diagnostics over browse. All generated artifacts live in this directory: the built `vrg` binary, the `demo-damaged-stream.sh` harness, and its `manual-damaged-stream/` tmux session capture. Test durations are stripped so the document verifies cleanly.

```bash
cd /home/chris/vrg && go vet ./... && go build ./... && go test -count=1 ./... | sed "s/[[:space:]][0-9.]*s$//" && echo GATES-OK
```

```output
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/present
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
?   	vrg/internal/viewport	[no test files]
GATES-OK
```

## Disposition matrix tests

`internal/searchindex/disposition_test.go` pins the deterministic categories. `TestMalformedDispositions` drives every Issue #3 schema-violation row — invalid JSON, bad base64, missing or non-string `type`, missing/mistyped required fields, `line_number` violations, bad submatch ranges, negative `binary_offset`, empty `submatches` — embedded mid-lifecycle in an intact stream: each is counted malformed, no integrity failure is raised, and the following records resynchronize and index. `TestIntegrityDispositions` drives the Issue #9 lifecycle rows through `Feed` asserting `Malformed()` stays zero — a skipped record never becomes a lifecycle failure. `TestCompositeDispositions` covers the two both-disposition rows: the trailing unterminated record (malformed + `unterminated trailing record`) and malformed bytes after `summary` (malformed + `record after summary`).

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestMalformedDispositions|TestIntegrityDispositions|TestCompositeDispositions' ./internal/searchindex 2>&1 | grep -vE '^(=== RUN|    --- (PASS|FAIL))' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestMalformedDispositions
--- PASS: TestIntegrityDispositions
--- PASS: TestCompositeDispositions
PASS
ok  	vrg/internal/searchindex
```

## 64 MiB limit and unknown types

`internal/searchindex/oversized_test.go` pins the resource limit. `TestOversizedBoundary` feeds a record exactly at 64 MiB — accepted and indexed — then one byte over — skipped and counted oversized. `TestOversizedResynchronizes` proves the record after an oversized one indexes normally: the newline-split itself discards the oversized payload. `TestOversizedDiagnosticNamesRecoveredPath` and `TestOversizedDiagnosticAnonymousWhenPathLost` cover the diagnostic forms — `oversized record skipped for <path>` when `type`/`data.path` decoded before the limit, the bare tally when the cut fell first. `TestOversizedOnlyFileAbsentFromList` shows a file whose only match records were oversized leaves no stops while its path still names the diagnostic, and `TestOversizedUnterminatedFinalRecord` pins the triple disposition — oversized + malformed + `unterminated trailing record`. `TestUnknownTypeDispositions` covers unknown-type counting: a separate `Unknown()` tally reported as `N unrecognised record types skipped`, no lifecycle effect, no substitution for `summary`, and unknown-after-`summary` counted plus independently flagged.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestOversized|TestUnknownTypeDispositions' ./internal/searchindex 2>&1 | grep -vE '^(=== RUN|    --- (PASS|FAIL))' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestOversizedBoundary
--- PASS: TestOversizedResynchronizes
--- PASS: TestOversizedDiagnosticNamesRecoveredPath
--- PASS: TestOversizedDiagnosticAnonymousWhenPathLost
--- PASS: TestOversizedOnlyFileAbsentFromList
--- PASS: TestOversizedUnterminatedFinalRecord
--- PASS: TestUnknownTypeDispositions
PASS
ok  	vrg/internal/searchindex
```

## Extended outcome matrix

`internal/app/outcome_test.go` extends the single `decideOutcome` table with the Issue #10 rows: malformed/oversized skips with usable results browse under a warning overlay at status 0; the same record loss with zero usable results — including a skipped record whose file was then binary-excluded, emptying the index after filtering — is the record-loss fatal row: the overlay stands alone and `q` or `Esc` exits 2. Unknown-type-only warnings over an empty stream take the warning overlay then no-results at 1 — the tally never changes the status on its own. Missing-`end` keeps retained matches browsable under an overlay at 2, or goes fatal at 2 when nothing was retained. Rows carrying undecodable bytes feed raw streams through `fixtureStream` into `Index.Feed`.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestOutcomeMatrix' ./internal/app 2>&1 | grep -vE '^(=== RUN|    --- (PASS|FAIL))' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestOutcomeMatrix
PASS
ok  	vrg/internal/app
```

## Manual check — damaged stream with usable results

`demo-damaged-stream.sh` (checked into this directory) runs the freshly built `vrg` on a real tmux PTY in a fixture directory whose `fakebin/rg` emits a valid `begin`, a valid `match`, a garbage line, an unknown-type record, a valid `end`, and a `summary`, then exits 0. The malformed record is skipped and counted, the unknown type tallied, the match retained — the warning overlay opens over the browse view listing `1 malformed record skipped` and `1 unrecognised record types skipped`; `Esc` dismisses to browse and `q` quits with status 0. The harness captures the composed screen at each step.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/010-06/code-walkthrough/vrg ./cmd/vrg && cd Notes/walkthroughs/010-06/code-walkthrough && ./demo-damaged-stream.sh
```

```output
exit=0
--- screen with overlay ---
f1.txt  ── f1.txt ──────────────────────────────────────────────────────────────
        1  hit one
                     ┌───────────────────────────────────┐
                     │1 malformed record skipped         │
                     │1 unrecognised record types skipped│
                     └───────────────────────────────────┘
--- screen after dismissal ---
f1.txt  ── f1.txt ──────────────────────────────────────────────────────────────
        1  hit one
```

## Result

Issue #10 is verified. `internal/searchindex` resolves every record position to a deterministic disposition: schema violations are skipped and counted malformed without touching lifecycle state, the 64 MiB payload limit is enforced explicitly with discard-through-newline resynchronization and best-effort path recovery (`oversized record skipped for <path>` when the path decoded before the limit, the anonymous tally otherwise), the unterminated oversized final record carries all three dispositions, and unknown types tally separately — counted in any position, never a substitute for `summary`, independently flagged after `summary`. `internal/app` feeds the malformed + oversized loss count and the record-skip diagnostics into `decideOutcome`: zero usable results after all filtering (binary exclusion included) makes record loss fatal at 2, usable results keep it a warning over browse at 0, and unknown-only warnings still land on no-results at 1. Missing-`end` retains its matches marked incomplete. The manual PTY run proves the composed behavior: browse, the overlay listing both skip counts, `Esc` dismissal, and `q` exiting 0.

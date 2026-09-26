# Issue #36: Stream-integrity fatal diagnostics

*2026-09-24T23:56:39Z by Showboat 0.6.1*
<!-- showboat-id: 61322e27-b158-41cf-9fcc-6a43c020b7b6 -->

Walkthrough for [Issue #36](../../../tasks/036-stream-integrity-fatal-diagnostics.md), implementing stream-integrity fatal diagnostics per `Notes/PRD-vrg.md` (*Result index, records, and stream integrity* — the lifecycle matrix; *Outcome and exit-status contract*). `internal/searchindex` now carries an ordered list of structured `Cause` values — a stable `CauseKind` plus the offending record's raw path bytes — with one cause per physical record, post-summary lifecycle suppression, and deterministic detection-order then end-of-stream ordering. `internal/app` composes one universal diagnostic component list — process, integrity causes, record-loss aggregates and per-path oversized details, unknown warnings — feeding both the overlay and the stderr replay; a fatal process result without stderr generates the exit-code/signal line while exit 0/1 never produce a process-status line. All generated artifacts live in this directory: the built `vrg` binary, the three `demo-*.sh` harnesses, and their `manual-*/` tmux session captures. Test durations are stripped so the document verifies cleanly.

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
ok  	vrg/internal/viewport
GATES-OK
```

## Structured integrity-cause tests

`internal/searchindex/causes_test.go` pins the structured contract. `TestIntegrityCauses` drives one exact-state table asserting the complete ordered `IntegrityCauses()` list — stable kind plus raw path — the one-line-per-cause `IntegrityFailures()` derivation, and the independent malformed/oversized/unknown tallies per row: every Issue #9 lifecycle-failure row, the second-summary-only-`extra summary record` precedence, post-summary records never lifecycle-processed (a `begin(Q)` cannot open Q, `context` lost its exemption), the dual representation of post-summary malformed/oversized/unknown records, the unterminated-fragment split, unsigned raw-path ordering of missing ends, and uncapped one-cause-per-record repetition. `TestOversizedAfterSummaryKeepsRecoveredPath` proves the recovered oversized path detail survives under the sole post-summary cause. The corrected lifecycle, disposition, and oversized rows run in the same suite.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestIntegrityCauses|TestOversizedAfterSummaryKeepsRecoveredPath|TestLifecycleMatrix|TestMalformedDispositions|TestIntegrityDispositions|TestCompositeDispositions|TestOversized|TestUnknownTypeDispositions" ./internal/searchindex 2>&1 | grep -vE "^(=== RUN|    --- (PASS|FAIL))" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestIntegrityCauses
--- PASS: TestOversizedAfterSummaryKeepsRecoveredPath
--- PASS: TestMalformedDispositions
--- PASS: TestIntegrityDispositions
--- PASS: TestCompositeDispositions
--- PASS: TestLifecycleMatrix
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

## Composed-diagnostic tests

`internal/app/diagnostics_test.go` pins the universal composition. `TestComposedDiagnostics` asserts the complete exact slices for both `composeDiagnostics` and `completionDiagnostics` — stderr as the process component, the generated `rg failed:` line only for fatal-without-stderr, no process line for exit 0/1, the full component order, the post-summary dual representation, uncapped repetition, and the escaped newline path — plus `decideOutcome`'s overlay list matching. `TestOutcomeCodesFromInput` pins the fixed statuses from the structured `outcomeInput`; `TestComposedDiagnosticsFromIndexOrdering` builds a real index with two still-open files twenty times proving the unsigned raw-path missing-end order is stable; `TestOverlayAndReplayShareComposedDiagnostics` and `TestFatalOverlayKeepsStderrCausesAndRecordLoss` drive `searchDoneMsg` proving overlay and replay share one composition and fatal results suppress nothing.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestComposedDiagnostics|TestOutcomeCodesFromInput|TestOverlayAndReplayShareComposedDiagnostics|TestFatalOverlayKeepsStderrCausesAndRecordLoss|TestOutcomeMatrix" ./internal/app 2>&1 | grep -vE "^(=== RUN|    --- (PASS|FAIL))" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestComposedDiagnostics
--- PASS: TestOutcomeCodesFromInput
--- PASS: TestComposedDiagnosticsFromIndexOrdering
--- PASS: TestOverlayAndReplayShareComposedDiagnostics
--- PASS: TestFatalOverlayKeepsStderrCausesAndRecordLoss
--- PASS: TestOutcomeMatrix
PASS
ok  	vrg/internal/app
```

## Manual check — clean exit, missing summary

`demo-missing-summary.sh` (checked into this directory) runs the built `vrg` on a real tmux PTY in a fixture directory whose `fakebin/rg` emits a valid begin/match/end stream with **no** summary and exits 0. Per Issue #36 a clean exit produces no process-status line: the fatal overlay names `missing summary` — never `ripgrep exited with code 0` — the first `q` dismisses to browse, the second quits with the fixed status 2, and the same diagnostic appears in the stderr replay (the overlay and the replay are one composition). The harness captures the composed screen at each step and the replay through a stderr redirect.

```bash
cd /home/chris/vrg/Notes/walkthroughs/036-06/code-walkthrough && ./demo-missing-summary.sh
```

```output
exit=2
--- screen with overlay ---
f.txt  ── f.txt ────────────────────────────────────────────────────────────────
       1  hit
                               ┌───────────────┐
                               │missing summary│
                               └───────────────┘
--- screen after dismissal ---
f.txt  ── f.txt ────────────────────────────────────────────────────────────────
       1  hit
--- stderr replay ---
missing summary
```

## Manual check — missing end for the only file

`demo-missing-end.sh` runs `vrg` against an `rg` emitting begin/match for `f.txt` then the final summary but no `end`. The retained match stays browsable (marked incomplete) and the sole end-of-stream cause names the file: `missing end for f.txt`. Dismissal reveals browse and the fixed status is 2, with the same line replayed on stderr.

```bash
cd /home/chris/vrg/Notes/walkthroughs/036-06/code-walkthrough && ./demo-missing-end.sh
```

```output
exit=2
--- screen with overlay ---
f.txt  ── f.txt ────────────────────────────────────────────────────────────────
       1  hit
                            ┌─────────────────────┐
                            │missing end for f.txt│
                            └─────────────────────┘
--- screen after dismissal ---
f.txt  ── f.txt ────────────────────────────────────────────────────────────────
       1  hit
--- stderr replay ---
missing end for f.txt
```

## Manual check — damaged stream with real child stderr

`demo-damaged.sh` exercises the universal order end to end: `rg` writes `rg: something failed` to stderr, emits an orphaned `end` for `g.txt`, a valid summary, then three post-summary records — a match, a malformed line, and an unknown type — and exits 3. The overlay shows every component together in the mandated order: the real stderr (suppressing the generated `rg failed:` line), `orphaned end for g.txt`, three `record after summary` causes one-per-offending-record, the malformed aggregate, and the unknown-type warning. Dismissal reveals browse, the fixed status is 2, and the identical text lands in the stderr replay.

```bash
cd /home/chris/vrg/Notes/walkthroughs/036-06/code-walkthrough && ./demo-damaged.sh
```

```output
exit=2
--- screen with overlay ---
f.txt  ── f.txt ────────────────────────────────────────────────────────────────
       1  hit
                     ┌───────────────────────────────────┐
                     │rg: something failed               │
                     │orphaned end for g.txt             │
                     │record after summary               │
                     │record after summary               │
                     │record after summary               │
                     │1 malformed record skipped         │
                     │1 unrecognised record types skipped│
                     └───────────────────────────────────┘
--- screen after dismissal ---
f.txt  ── f.txt ────────────────────────────────────────────────────────────────
       1  hit
--- stderr replay ---
rg: something failed
orphaned end for g.txt
record after summary
record after summary
record after summary
1 malformed record skipped
1 unrecognised record types skipped
```

## Result

Issue #36 is verified. `internal/searchindex` records structured `Cause` values — one per offending physical record, uncapped — in detection order followed by the end-of-stream causes (missing ends in unsigned raw-path order, missing summary, unterminated final record); post-summary records contribute only `record after summary` (or `extra summary record` for a second summary) and are never lifecycle-processed, while malformed/oversized/unknown tallies keep their dual representation. `internal/app` composes the single universal component list — process, integrity causes, record-loss aggregates and per-path oversized details, unknown warnings — that feeds both the overlay and the stderr replay verbatim; real child stderr suppresses the generated `rg failed:` line, exit 0/1 never produce a process-status line, and fatal outcomes no longer obscure integrity causes or record-loss details behind process status.

# Issue #37: Oversized-record aggregate and anonymous diagnostics

*2026-09-25T00:09:02Z by Showboat 0.6.1*
<!-- showboat-id: db71e89e-def0-4606-9026-d22d3272fb4a -->

Walkthrough for [Issue #37](../../../tasks/037-oversized-record-aggregate-anonymous-diagnostics.md), completing the oversized-record diagnostic contract per `Notes/PRD-vrg.md` (*Result index, records, and stream integrity* — the oversized bullets; *Resources and responsiveness* — the 64 MiB limit). Issue #36 landed the universal composition; Issue #37 completes it: the oversized component always leads with the pluralized per-record aggregate — exactly `1 oversized record skipped` or `N oversized records skipped`, emitted whenever the count is positive even when no path was recovered — followed by one `oversized record skipped for <escaped path>` detail per distinct raw path, deduplicated through `oversizedSeen` in first-occurrence stream order (the `text` and `bytes` encodings of one path agree). An anonymous oversized record — the limit cut before `type`/`data.path` were parsed — is therefore never silent: the fatal overlay with zero usable results carries exactly the aggregate and exits 2, and with usable results the aggregate opens the warning overlay over browse and lands in the replay. All generated artifacts live in this directory: the built `vrg` binary, the three `demo-*.sh` harnesses, and their `manual-*/` tmux session captures. Test durations are stripped so the document verifies cleanly.

```bash
cd /home/chris/vrg && go vet ./... && go build ./... && go test -count=1 ./... | sed 's/[[:space:]][0-9.]*s$//' && echo GATES-OK
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

## Raw-path deduplication tests

`internal/searchindex/oversized_test.go` pins the index-level contract. `TestOversizedDiagnosticsDeduplicateByPath` feeds two oversized records naming `big.txt` and asserts `Oversized()` still counts both records while `OversizedDiagnostics()` emits the path line once; `TestOversizedDiagnosticsDeduplicateByRawPath` mixes the `text` and `bytes` encodings of the same decoded path proving dedup keys on raw bytes, not the rendered form; `TestOversizedDiagnosticsMixedRecoverability` interleaves a recoverable path, an anonymous record, a second path, and a repeat — the detail lines come out one-per-distinct-path in first-occurrence order. The existing boundary, resync, anonymous-tally, and post-summary rows run alongside.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestOversized' ./internal/searchindex 2>&1 | grep -vE '^(=== RUN|    --- (PASS|FAIL))' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestOversizedAfterSummaryKeepsRecoveredPath
--- PASS: TestOversizedBoundary
--- PASS: TestOversizedResynchronizes
--- PASS: TestOversizedDiagnosticNamesRecoveredPath
--- PASS: TestOversizedDiagnosticAnonymousWhenPathLost
--- PASS: TestOversizedOnlyFileAbsentFromList
--- PASS: TestOversizedUnterminatedFinalRecord
--- PASS: TestOversizedDiagnosticsDeduplicateByPath
--- PASS: TestOversizedDiagnosticsDeduplicateByRawPath
--- PASS: TestOversizedDiagnosticsMixedRecoverability
PASS
ok  	vrg/internal/searchindex
```

## Composition and outcome tests

`internal/app` pins the component through the universal order and end-to-end through the real index. `TestComposedDiagnostics` carries the new exact-slice rows: the post-summary oversized record composes `record after summary`, `1 oversized record skipped`, then the recovered `big.txt` detail; the anonymous case emits the aggregate alone; the plural case with no recoverable paths emits `3 oversized records skipped`; the mixed case counts all four records while only the distinct paths appear. The model-level tests drive real 64 MiB streams through `fixtureStream`: `TestAnonymousOversizedFatalOverlayIsNeverEmpty` (aggregate-only fatal overlay, `q` exits 2, replay carries the aggregate), `TestAnonymousOversizedWithResultsOverlayAndReplay` (browse warning plus replay), `TestOversizedDetailsDeduplicateByPath` (`2 oversized records skipped` + one detail), and `TestOversizedMixedRecoverabilityComposition` (`4 oversized records skipped` + two details in first-occurrence order). `TestOutcomeMatrix` covers the three new oversized rows — anonymous-fatal, anonymous-warning, and named-warning.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestComposedDiagnostics|TestAnonymousOversized|TestOversizedDetailsDeduplicateByPath|TestOversizedMixedRecoverabilityComposition|TestOutcomeMatrix' ./internal/app 2>&1 | grep -vE '^(=== RUN|    --- (PASS|FAIL))' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestComposedDiagnostics
--- PASS: TestComposedDiagnosticsFromIndexOrdering
--- PASS: TestAnonymousOversizedFatalOverlayIsNeverEmpty
--- PASS: TestAnonymousOversizedWithResultsOverlayAndReplay
--- PASS: TestOversizedDetailsDeduplicateByPath
--- PASS: TestOversizedMixedRecoverabilityComposition
--- PASS: TestOutcomeMatrix
PASS
ok  	vrg/internal/app
```

## Manual check — oversized record with a recoverable path

`demo-oversized-named.sh` (checked into this directory) runs the built `vrg` on a real tmux PTY in a fixture whose `fakebin/rg` emits a valid begin/match/end stream for `f.txt`, then one oversized match record for `big.txt` — its `path` member precedes the giant `lines` value, so the 64 MiB cut leaves a recoverable path — then the summary, exiting 0. Usable results exist, so the warning overlay opens over browse carrying BOTH lines in order: the aggregate `1 oversized record skipped` then the recovered-path detail `oversized record skipped for big.txt`. The first `q` dismisses to browse, the second quits with status 0, and both lines reach the stderr replay — the overlay and the replay are one composition.

```bash
./demo-oversized-named.sh
```

```output
exit=0
--- screen with overlay ---
f.txt  ── f.txt ────────────────────────────────────────────────────────────────
       1  hit
                     ┌────────────────────────────────────┐
                     │1 oversized record skipped          │
                     │oversized record skipped for big.txt│
                     └────────────────────────────────────┘
--- screen after dismissal ---
f.txt  ── f.txt ────────────────────────────────────────────────────────────────
       1  hit
--- stderr replay ---
1 oversized record skipped
oversized record skipped for big.txt
```

## Manual check — anonymous oversized record, zero usable results

`demo-oversized-anonymous.sh` emits one oversized match record whose 64 MiB cut falls inside the giant `lines` value BEFORE `data.path` is ever parsed — the record is anonymous — followed by the summary and a clean exit 0. With zero usable results the outcome is record-loss fatal, and the overlay is never empty: it carries exactly `1 oversized record skipped`, no path detail exists to show, `q` exits with the fixed status 2, and the aggregate lands in the stderr replay. The harness also asserts no `skipped for` line can appear.

```bash
./demo-oversized-anonymous.sh
```

```output
exit=2
--- screen with overlay ---
                          ┌──────────────────────────┐
                          │1 oversized record skipped│
                          └──────────────────────────┘
--- stderr replay ---
1 oversized record skipped
```

## Manual check — duplicate oversized paths deduplicate by raw path

`demo-oversized-dedup.sh` emits TWO oversized match records naming the same file — once through the `text` encoding and once through `bytes` (`YmlnLnR4dA==` is base64 for `big.txt`, so the decoded raw paths agree) — plus valid `f.txt` records and the summary. The overlay leads with the per-record aggregate `2 oversized records skipped` but shows only ONE `oversized record skipped for big.txt` detail line: the harness greps the captured screen proving the detail is not duplicated while the count stays per-record. Dismissal reveals browse, the status is 0, and the same two lines reach the replay.

```bash
./demo-oversized-dedup.sh
```

```output
exit=0
--- screen with overlay ---
f.txt  ── f.txt ────────────────────────────────────────────────────────────────
       1  hit
                     ┌────────────────────────────────────┐
                     │2 oversized records skipped         │
                     │oversized record skipped for big.txt│
                     └────────────────────────────────────┘
--- screen after dismissal ---
f.txt  ── f.txt ────────────────────────────────────────────────────────────────
       1  hit
--- stderr replay ---
2 oversized records skipped
oversized record skipped for big.txt
```

## Result

Issue #37 is verified. `internal/searchindex` keeps two distinct semantics: `Oversized()` counts every oversized record while `oversizedSeen` admits each recovered raw path once, so `OversizedDiagnostics()` emits one detail line per distinct path in first-occurrence order — the `text` and `bytes` encodings of one path deduplicate together. `internal/app`'s universal composition (Issue #36) places the oversized component after the malformed aggregate and before the unknown-type warnings, leading with the always-emitted pluralized aggregate — `1 oversized record skipped` / `N oversized records skipped` — whenever the count is positive. Anonymous oversized records can therefore never pass silently: with zero usable results the fatal overlay is exactly the aggregate and exits 2; with usable results the aggregate opens the warning overlay over browse and reaches the stderr replay. Both sinks share the one composition, so overlay and replay stay identical.

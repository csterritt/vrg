# Issue #44: Post-summary context is a stream-integrity failure

*2026-09-25T12:21:46Z by Showboat 0.6.1*
<!-- showboat-id: 15e9233d-f959-4ce6-a82c-b93faabe5fd8 -->

Walkthrough for [Issue #44](../../../tasks/044-post-summary-context-integrity-failure.md) per `Notes/PRD-vrg.md` (*Result index, records, and stream integrity* — the lifecycle matrix and the summary-is-final rule; *Outcome and exit-status contract*). Issue #36 owns the parser change: it removed the `context` exemption in `Index.Add` and corrected the contradictory lifecycle row so a post-`summary` `context` yields the `record after summary` cause. Issue #44 supplies only regression coverage — no production-code change: a dedicated `internal/searchindex` exact-cause row plus neighbouring pre-`summary` ignored-rows, and an `internal/app` outcome assertion that the same stream composes exactly the after-`summary` cause and retains it for stderr replay. All generated artifacts live in this directory: the built `vrg` binary, the `demo-context-after-summary.sh` harness, and its `manual-context-after-summary/` tmux session captures. Test durations are stripped so the document verifies cleanly.

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

## Focused structured-cause and lifecycle coverage

`internal/searchindex/causes_test.go`'s `TestIntegrityCauses` gained Issue #44's dedicated row — an intact begin/match/end/`summary` stream followed by a `context` record asserting exactly the single `{Kind: CauseRecordAfterSummary}` cause with the retained stop unaffected — plus the neighbouring rows proving `context` before `begin` and before `summary` remains ignored (no cause, no opened file, no retained stop). `lifecycle_test.go`'s `TestLifecycleMatrix` carries Issue #36's corrected post-`summary` `context` row — expecting `record after summary`, not silence — and Issue #44's renamed pre-`summary`-only row "context before the summary has no lifecycle effect". The `-v` run below filters to the named subtests so each new row is visible.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestIntegrityCauses|TestLifecycleMatrix" ./internal/searchindex 2>&1 | grep -E "(context|summary)" | grep -E "PASS|FAIL" | sed -E "s/ \([0-9.]+s\)//" ; go test -count=1 -run "TestIntegrityCauses|TestLifecycleMatrix" ./internal/searchindex | sed -E "s/[[:space:]][0-9.]+s$//"
```

```output
    --- PASS: TestIntegrityCauses/missing_summary
    --- PASS: TestIntegrityCauses/second_summary_contributes_only_extra_summary
    --- PASS: TestIntegrityCauses/third_summary_repeats_extra_summary
    --- PASS: TestIntegrityCauses/begin_after_summary_cannot_open_the_file
    --- PASS: TestIntegrityCauses/match_after_summary_is_not_lifecycle-processed
    --- PASS: TestIntegrityCauses/context_before_begin_is_ignored
    --- PASS: TestIntegrityCauses/context_before_summary_is_ignored
    --- PASS: TestIntegrityCauses/context_after_summary
    --- PASS: TestIntegrityCauses/unterminated_fragment_after_summary
    --- PASS: TestIntegrityCauses/oversized_record_after_summary
    --- PASS: TestIntegrityCauses/unknown_type_after_summary
    --- PASS: TestIntegrityCauses/post-summary_records_never_touch_lifecycle_state
    --- PASS: TestLifecycleMatrix/context_before_the_summary_has_no_lifecycle_effect
    --- PASS: TestLifecycleMatrix/file_open_and_no_summary
    --- PASS: TestLifecycleMatrix/summary_alone_is_a_complete_zero-result_stream
    --- PASS: TestLifecycleMatrix/missing_summary
    --- PASS: TestLifecycleMatrix/second_summary
    --- PASS: TestLifecycleMatrix/match_record_after_summary
    --- PASS: TestLifecycleMatrix/context_record_after_summary
    --- PASS: TestLifecycleMatrix/begin_after_summary
    --- PASS: TestLifecycleMatrix/malformed_record_after_summary
ok  	vrg/internal/searchindex
```

## Outcome assertion

`internal/app/diagnostics_test.go`'s `TestPostSummaryContextIsFatalIntegrity` drives the same stream — valid begin/match/end, `summary`, then `context` — through `searchDoneMsg` under a clean exit 0. It asserts the fatal integrity outcome: the overlay opens over browse carrying exactly `record after summary` as the complete composed diagnostic, dismissal then `q` exits 2, and the identical line is retained in the session collection for the stderr replay. Neighbouring Issue #36 composition and outcome-matrix tests stay green.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestPostSummaryContextIsFatalIntegrity|TestComposedDiagnostics|TestOutcomeCodesFromInput|TestOverlayAndReplayShareComposedDiagnostics|TestOutcomeMatrix" ./internal/app 2>&1 | grep -vE "^(=== RUN|    --- (PASS|FAIL))" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestComposedDiagnostics
--- PASS: TestOutcomeCodesFromInput
--- PASS: TestComposedDiagnosticsFromIndexOrdering
--- PASS: TestOverlayAndReplayShareComposedDiagnostics
--- PASS: TestPostSummaryContextIsFatalIntegrity
--- PASS: TestOutcomeMatrix
PASS
ok  	vrg/internal/app
```

## Manual check — context record after a valid summary, clean exit

`demo-context-after-summary.sh` (checked into this directory) runs the built `vrg` on a real tmux PTY in a fixture directory whose `fakebin/rg` emits a valid begin/match/end stream, the `summary`, then a `context` record, and exits 0. Per the summary-is-final contract the outcome is a stream-integrity failure on the fatal path of the outcome matrix: the overlay names `record after summary` — the post-`summary` `context` is not silently accepted — the first `q` dismisses to browse, the second quits with the fixed status 2, and the same diagnostic appears in the stderr replay. The harness captures the composed screen at each step and the replay through a stderr redirect; the captures live under `manual-context-after-summary/`.

```bash
cd /home/chris/vrg/Notes/walkthroughs/044-03/code-walkthrough && ./demo-context-after-summary.sh
```

```output
exit=2
--- screen with overlay ---
f.txt  ── f.txt ────────────────────────────────────────────────────────────────
       1  hit
                             ┌────────────────────┐
                             │record after summary│
                             └────────────────────┘
--- screen after dismissal ---
f.txt  ── f.txt ────────────────────────────────────────────────────────────────
       1  hit
--- stderr replay ---
record after summary
```

## Result

Issue #44 is verified. The summary-is-final contract holds: any record after `summary` — `context` included — is a stream-integrity failure contributing exactly the `record after summary` cause, while pre-`summary` `context` records remain ignored for match and lifecycle semantics. The dedicated `TestIntegrityCauses` rows pin the exact cause list and untouched retained state, Issue #36's corrected lifecycle row (`context_record_after_summary`) stays green beside the renamed pre-`summary`-only row, and `TestPostSummaryContextIsFatalIntegrity` plus the tmux manual scenario prove the same stream takes the fatal path — overlay naming the after-`summary` cause, fixed exit status 2, identical text in the stderr replay.

# Issue #44: Post-summary context records are integrity failures

*2026-09-15T20:57:21Z by Showboat 0.6.1*
<!-- showboat-id: f66621e9-18b7-4dae-8a6e-dbd88a015e21 -->

Walkthrough for Issue #44 (Notes/tasks/044-post-summary-context-integrity-failure.md), which adds dedicated regression coverage proving that a `context` record after `summary` is a stream-integrity failure — not an exemption — per the summary-is-final contract. Issue #36 (Notes/issues/036-stream-integrity-fatal-diagnostics.md) owned the parser change: it removed the `context` exemption in `Builder.Add` and corrected the contradictory `"context after summary has no lifecycle effect"` row in `internal/searchindex/lifecycle_test.go`. Issue #44 (Notes/issues/044-post-summary-context-integrity-failure.md) begins from that green boundary and supplies focused coverage only — no second parser change. References: Notes/PRD-vrg.md (*Result index, records, and stream integrity* — the complete-stream bullets; *Outcome and exit-status contract* — the fatal rows).

Contracts verified:
- Issue #36's corrected lifecycle matrix row: `summary` then `context` is an integrity failure.
- Issue #44's dedicated structured-cause row: `summary` → `context` yields exactly the single `record after summary` cause.
- Neighbouring pre-`summary` `context` rows (before `begin`, while open, after `end` before `summary`) remain ignored for match/lifecycle semantics — no causes, complete stream.
- The same stream produces the fatal outcome path (exit 2) whose complete composed diagnostic names exactly `record after summary`, retained for post-restoration stderr replay.
- Manual scenario: a fake rg emitting valid records, then `summary`, then a `context` record, exiting 0 → treated as a stream-integrity failure, never silently accepted.

All generated artifacts live in this directory.

```bash
cd /home/chris/vrg && go build ./... && go vet ./... && echo GATES-OK
```

```output
GATES-OK
```

```bash
cd /home/chris/vrg && go test -count=1 ./... -timeout 120s | sed 's/[[:space:]][0-9.]*s$//'
```

```output
?   	vrg/Notes/walkthroughs/023-04/code-walkthrough/demo_artifacts	[no test files]
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/docs
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
ok  	vrg/internal/searchindex
?   	vrg/internal/sinkfixtures	[no test files]
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
```

## Focused lifecycle and structured-cause coverage (internal/searchindex)

Issue #36 owns the corrected lifecycle matrix row `context after summary is integrity failure` in `TestLifecycleMatrix` (`internal/searchindex/lifecycle_test.go`): `summary` followed by `context` makes the stream incomplete. Issue #44 strengthened the safety net around it: a neighbouring `context while open` lifecycle row, and — in `TestIntegrityCauseMatrix` (`internal/searchindex/integrity_test.go`) — the dedicated `summary` → `context` row asserting exactly the single structured `record after summary` cause, plus neighbouring rows proving `context` before `begin`, while open, and after `end` before `summary` still records no causes and stays complete. The `second summary then post-summary context` ordering row also exercises a post-`summary` `context` after an `extra summary` cause.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/searchindex/ -run 'TestLifecycleMatrix|TestIntegrityCauseMatrix/context' -timeout 60s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestIntegrityCauseMatrix
=== RUN   TestIntegrityCauseMatrix/context_after_summary_records_only_record_after_summary
=== RUN   TestIntegrityCauseMatrix/context_before_begin_records_no_causes_and_is_ignored
=== RUN   TestIntegrityCauseMatrix/context_while_open_records_no_causes_and_is_ignored
=== RUN   TestIntegrityCauseMatrix/context_after_end_before_summary_records_no_causes_and_is_ignored
=== RUN   TestIntegrityCauseMatrix/second_summary_then_post-summary_context_keeps_detection_order
--- PASS: TestIntegrityCauseMatrix (0.00s)
    --- PASS: TestIntegrityCauseMatrix/context_after_summary_records_only_record_after_summary (0.00s)
    --- PASS: TestIntegrityCauseMatrix/context_before_begin_records_no_causes_and_is_ignored (0.00s)
    --- PASS: TestIntegrityCauseMatrix/context_while_open_records_no_causes_and_is_ignored (0.00s)
    --- PASS: TestIntegrityCauseMatrix/context_after_end_before_summary_records_no_causes_and_is_ignored (0.00s)
    --- PASS: TestIntegrityCauseMatrix/second_summary_then_post-summary_context_keeps_detection_order (0.00s)
=== RUN   TestLifecycleMatrix
=== RUN   TestLifecycleMatrix/begin_while_not_open_opens_the_file
=== RUN   TestLifecycleMatrix/begin_while_already_open_is_integrity_failure_duplicate_begin
=== RUN   TestLifecycleMatrix/match_while_open_indexes_under_the_file
=== RUN   TestLifecycleMatrix/match_never_opened_is_orphaned_retained_incomplete
=== RUN   TestLifecycleMatrix/match_after_non-binary_end_is_orphaned_retained_incomplete
=== RUN   TestLifecycleMatrix/end_while_open_closes_the_file
=== RUN   TestLifecycleMatrix/end_while_not_open_is_orphaned_integrity_failure
=== RUN   TestLifecycleMatrix/context_before_begin_has_no_lifecycle_effect
=== RUN   TestLifecycleMatrix/context_after_end_has_no_lifecycle_effect
=== RUN   TestLifecycleMatrix/context_while_open_has_no_lifecycle_effect
=== RUN   TestLifecycleMatrix/context_after_summary_is_integrity_failure
=== RUN   TestLifecycleMatrix/file_still_open_at_stream_end_is_integrity_failure_retained_incomplete
=== RUN   TestLifecycleMatrix/summary_alone_is_complete_zero-result_stream
=== RUN   TestLifecycleMatrix/summary_missing_is_integrity_failure
=== RUN   TestLifecycleMatrix/second_summary_is_integrity_failure
=== RUN   TestLifecycleMatrix/any_record_after_summary_is_integrity_failure
=== RUN   TestLifecycleMatrix/trailing_unterminated_record_is_malformed_and_stream_incomplete
=== RUN   TestLifecycleMatrix/text_and_bytes_path_identity_agreement
=== RUN   TestLifecycleMatrix/interleaved_open_files_with_valid_pairing
=== RUN   TestLifecycleMatrix/binary_exclusion_precedence_over_orphan_retention
=== RUN   TestLifecycleMatrix/orphaned_match_after_non-binary_end_retained_with_incomplete_metadata
=== RUN   TestLifecycleMatrix/one_file_orphaned_one_file_valid_in_same_stream
--- PASS: TestLifecycleMatrix (0.00s)
    --- PASS: TestLifecycleMatrix/begin_while_not_open_opens_the_file (0.00s)
    --- PASS: TestLifecycleMatrix/begin_while_already_open_is_integrity_failure_duplicate_begin (0.00s)
    --- PASS: TestLifecycleMatrix/match_while_open_indexes_under_the_file (0.00s)
    --- PASS: TestLifecycleMatrix/match_never_opened_is_orphaned_retained_incomplete (0.00s)
    --- PASS: TestLifecycleMatrix/match_after_non-binary_end_is_orphaned_retained_incomplete (0.00s)
    --- PASS: TestLifecycleMatrix/end_while_open_closes_the_file (0.00s)
    --- PASS: TestLifecycleMatrix/end_while_not_open_is_orphaned_integrity_failure (0.00s)
    --- PASS: TestLifecycleMatrix/context_before_begin_has_no_lifecycle_effect (0.00s)
    --- PASS: TestLifecycleMatrix/context_after_end_has_no_lifecycle_effect (0.00s)
    --- PASS: TestLifecycleMatrix/context_while_open_has_no_lifecycle_effect (0.00s)
    --- PASS: TestLifecycleMatrix/context_after_summary_is_integrity_failure (0.00s)
    --- PASS: TestLifecycleMatrix/file_still_open_at_stream_end_is_integrity_failure_retained_incomplete (0.00s)
    --- PASS: TestLifecycleMatrix/summary_alone_is_complete_zero-result_stream (0.00s)
    --- PASS: TestLifecycleMatrix/summary_missing_is_integrity_failure (0.00s)
    --- PASS: TestLifecycleMatrix/second_summary_is_integrity_failure (0.00s)
    --- PASS: TestLifecycleMatrix/any_record_after_summary_is_integrity_failure (0.00s)
    --- PASS: TestLifecycleMatrix/trailing_unterminated_record_is_malformed_and_stream_incomplete (0.00s)
    --- PASS: TestLifecycleMatrix/text_and_bytes_path_identity_agreement (0.00s)
    --- PASS: TestLifecycleMatrix/interleaved_open_files_with_valid_pairing (0.00s)
    --- PASS: TestLifecycleMatrix/binary_exclusion_precedence_over_orphan_retention (0.00s)
    --- PASS: TestLifecycleMatrix/orphaned_match_after_non-binary_end_retained_with_incomplete_metadata (0.00s)
    --- PASS: TestLifecycleMatrix/one_file_orphaned_one_file_valid_in_same_stream (0.00s)
PASS
ok  	vrg/internal/searchindex
```

## Dedicated outcome assertion (internal/app)

`TestContextAfterSummaryOutcome` (`internal/app/outcome_test.go`, Issue #44) drives the same `summary` → `context` stream through the full `Update` flow and asserts the fatal path of the Issue #9 outcome matrix: exit 2 in both dispositions — zero usable results (fatal error overlay; `q` exits 2) and retained results (browse with a non-fatal error overlay; `q` dismisses to browse, `q` again exits 2). The complete composed diagnostic is exactly `record after summary` — the after-`summary` cause, never `ripgrep exited with code 0` — and the identical line is collected for post-restoration stderr replay (Issue #11/Issue #36 composition contract).

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run '^TestContextAfterSummaryOutcome$' -timeout 60s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestContextAfterSummaryOutcome
=== RUN   TestContextAfterSummaryOutcome/zero_results_fatal_overlay_exits_2
=== RUN   TestContextAfterSummaryOutcome/retained_results_browse_overlay_dismiss_q_exits_2
--- PASS: TestContextAfterSummaryOutcome (0.00s)
    --- PASS: TestContextAfterSummaryOutcome/zero_results_fatal_overlay_exits_2 (0.00s)
    --- PASS: TestContextAfterSummaryOutcome/retained_results_browse_overlay_dismiss_q_exits_2 (0.00s)
PASS
ok  	vrg/internal/app
```

## Manual scenario: valid records, then summary, then a context record (exit 0)

This is the issue's manual verification. The fake rg (`fakebin-context-after-summary/rg`) emits a valid `begin`/`match`/`end` stream for `demo/test.txt`, then a valid `summary`, then a `context` record, and exits 0. Before Issue #36 removed the exemption, this stream was silently accepted. Now the post-`summary` `context` is a stream-integrity failure: the error overlay names the after-`summary` cause `record after summary` — the retained match still browses underneath (usable results), so `q` dismisses to the browse view and `q` again exits 2 per the outcome matrix. The same composed diagnostic is replayed to stderr after terminal restoration. The PTY helper (`runpty.py`) sends `q,q` and captures the stripped terminal output; `exit=2` is the vrg exit status.

```bash
cd /home/chris/vrg/Notes/walkthroughs/044-03/code-walkthrough && PATH=$(pwd)/fakebin-context-after-summary:/usr/bin:/bin VRG_KEYS='q,q' VRG_DELAY=1.0 timeout 15 python3 runpty.py ./vrg hello demo 2>&1; echo exit=$?
```

```output
┌──────────────────────┐
│ record after summary │
└──────────────────────┘demo/test.txt   ── demo/test.txt ── 1  hello world record after summary
exit=2
```

## Zero-results variant: summary then context only (exit 0)

The same violation with no retained matches (`fakebin-context-only/rg` emits only `summary` then `context`, exit 0) exercises the outcome matrix's fatal no-results row: a fatal error overlay naming `record after summary` whose dismissal has no underlying state to return to, so `q` exits 2 immediately. The identical diagnostic is replayed to stderr after terminal restoration.

```bash
cd /home/chris/vrg/Notes/walkthroughs/044-03/code-walkthrough && PATH=$(pwd)/fakebin-context-only:/usr/bin:/bin VRG_KEYS='q' VRG_DELAY=1.0 timeout 15 python3 runpty.py ./vrg hello demo 2>&1; echo exit=$?
```

```output
┌──────────────────────┐
│ record after summary │
└──────────────────────┘record after summary
exit=2
```

## Summary

The walkthrough demonstrates:
- Full gates: `go build`, `go vet`, and the complete test suite all pass.
- Issue #36's corrected lifecycle row (`context after summary is integrity failure`) remains green alongside Issue #44's neighbouring pre-`summary` rows (`context while open`, plus the existing before-`begin` and after-`end` rows).
- Issue #44's dedicated structured-cause coverage: `summary` → `context` yields exactly the single `record after summary` cause; pre-`summary` `context` in every position yields no causes and a complete stream.
- Issue #44's dedicated outcome test `TestContextAfterSummaryOutcome`: the fatal path (exit 2) in both the zero-results and retained-results dispositions, with the complete composed diagnostic `record after summary` in the overlay and the stderr replay.
- The manual scenario end to end: a fake rg emitting valid records, then `summary`, then `context`, exiting 0 produces an error overlay naming `record after summary` — a stream-integrity failure, not a silent exemption — with exit 2 and the same line replayed to stderr after terminal restoration; the zero-results variant shows the fatal-overlay dismissal path.

References: Notes/issues/036-stream-integrity-fatal-diagnostics.md, Notes/issues/044-post-summary-context-integrity-failure.md, and Notes/PRD-vrg.md (*Result index, records, and stream integrity*; *Outcome and exit-status contract*).

# Issue #37: Oversized-record aggregate and anonymous diagnostics

*2026-09-15T15:53:16Z by Showboat 0.6.1*
<!-- showboat-id: 74e8256e-5f1a-4039-993a-6a1df55088cf -->

Walkthrough for Issue #37 (Notes/tasks/037-oversized-record-aggregate-anonymous-diagnostics.md), which extends the Issue #36 record-loss composition so the oversized component always leads with the pluralized aggregate count and covers anonymous oversized records that recover no path. References: Notes/issues/037-oversized-record-aggregate-anonymous-diagnostics.md and Notes/PRD-vrg.md (Result index, records, and stream integrity — oversized bullets; Resources and responsiveness — 64 MiB record limit).

Contracts verified:
- The oversized component always emits the pluralized aggregate built from Index.OversizedCount(): exactly `1 oversized record skipped` for one record and `N oversized records skipped` for every other count, regardless of path recovery.
- Each distinct recoverable raw path produces one `oversized record skipped for <sanitized path>` detail line appended after the aggregate, deduplicated in deterministic first-occurrence order; the aggregate's per-record count is never deduplicated.
- An anonymous oversized record with zero usable results produces a fatal overlay containing exactly the aggregate line — never an empty overlay — exiting 2.
- The same anonymous record with usable results produces a visible overlay and the aggregate in the stderr replay rather than passing silently.
- The oversized component sits inside Issue #36's universal component order, after the malformed aggregate and before the per-path details and unknown-type warnings.

All generated artifacts live in this directory.

```bash
cd /home/chris/vrg && go build ./... && go vet ./... && echo GATES-OK
```

```output
GATES-OK
```

```bash
cd /home/chris/vrg && go test -count=1 ./... -timeout 180s | sed 's/[[:space:]][0-9.]*s$//'
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

## App oversized-diagnostics tests

The Issue #37 tests (internal/app/outcome_test.go) drive real oversized streams through Builder.ReadFrom and the full Update flow. TestOutcomeOversizedAggregateDiagnostics asserts the always-emitted pluralized aggregate, per-path details deduplicated to one line per distinct raw path in first-occurrence order, the anonymous-oversized fatal case (overlay containing exactly the aggregate, exit 2 — never empty), the anonymous-oversized non-fatal case (visible warning overlay plus stderr replay), the mixed-recoverability multi-record case, and the aggregate's position after the malformed aggregate and before the per-path details. The Issue #36 post-summary oversized fixtures in TestDecideOutcomeComposedOrder and TestOutcomeIntegrityDiagnosticsFlow were updated to expect the aggregate between the record after summary cause and the per-path detail.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run '^(TestOutcomeOversizedAggregateDiagnostics|TestOutcomeIntegrityDiagnosticsFlow|TestDecideOutcomeComposedOrder|TestDecideOutcomeMatrix)$' -timeout 120s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestDecideOutcomeMatrix
=== RUN   TestDecideOutcomeMatrix/rg0_clean_browse
=== RUN   TestDecideOutcomeMatrix/rg1_retained_results_complete_stream_browse_0
=== RUN   TestDecideOutcomeMatrix/rg1_empty_no-results_1
=== RUN   TestDecideOutcomeMatrix/rg0_empty_no-results_1
=== RUN   TestDecideOutcomeMatrix/fatal_code_with_results_browse_overlay
=== RUN   TestDecideOutcomeMatrix/fatal_code_no_results_fatal_overlay
=== RUN   TestDecideOutcomeMatrix/signal_death_with_results_browse_overlay
=== RUN   TestDecideOutcomeMatrix/signal_death_no_results_fatal_overlay
=== RUN   TestDecideOutcomeMatrix/missing_summary_with_matches_fatal
=== RUN   TestDecideOutcomeMatrix/orphaned_end_incomplete_fatal
=== RUN   TestDecideOutcomeMatrix/incomplete_stream_rg0_fatal
=== RUN   TestDecideOutcomeMatrix/incomplete_stream_rg1_with_results_fatal
=== RUN   TestDecideOutcomeMatrix/stderr_warning_with_results_browse_warning_0
=== RUN   TestDecideOutcomeMatrix/stderr_warning_zero_results_warning_no-results_1
=== RUN   TestDecideOutcomeMatrix/stderr_warning_rg1_zero_results_warning_no-results_1
=== RUN   TestDecideOutcomeMatrix/fatal_code_with_stderr_results_error_overlay
=== RUN   TestDecideOutcomeMatrix/unknown-only_zero_results_warning_no-results_1
=== RUN   TestDecideOutcomeMatrix/malformed_with_usable_results_browse_warning_0
=== RUN   TestDecideOutcomeMatrix/malformed_zero_usable_results_record-loss_fatal_2
=== RUN   TestDecideOutcomeMatrix/oversized_zero_usable_results_record-loss_fatal_2
--- PASS: TestDecideOutcomeMatrix (0.00s)
    --- PASS: TestDecideOutcomeMatrix/rg0_clean_browse (0.00s)
    --- PASS: TestDecideOutcomeMatrix/rg1_retained_results_complete_stream_browse_0 (0.00s)
    --- PASS: TestDecideOutcomeMatrix/rg1_empty_no-results_1 (0.00s)
    --- PASS: TestDecideOutcomeMatrix/rg0_empty_no-results_1 (0.00s)
    --- PASS: TestDecideOutcomeMatrix/fatal_code_with_results_browse_overlay (0.00s)
    --- PASS: TestDecideOutcomeMatrix/fatal_code_no_results_fatal_overlay (0.00s)
    --- PASS: TestDecideOutcomeMatrix/signal_death_with_results_browse_overlay (0.00s)
    --- PASS: TestDecideOutcomeMatrix/signal_death_no_results_fatal_overlay (0.00s)
    --- PASS: TestDecideOutcomeMatrix/missing_summary_with_matches_fatal (0.00s)
    --- PASS: TestDecideOutcomeMatrix/orphaned_end_incomplete_fatal (0.00s)
    --- PASS: TestDecideOutcomeMatrix/incomplete_stream_rg0_fatal (0.00s)
    --- PASS: TestDecideOutcomeMatrix/incomplete_stream_rg1_with_results_fatal (0.00s)
    --- PASS: TestDecideOutcomeMatrix/stderr_warning_with_results_browse_warning_0 (0.00s)
    --- PASS: TestDecideOutcomeMatrix/stderr_warning_zero_results_warning_no-results_1 (0.00s)
    --- PASS: TestDecideOutcomeMatrix/stderr_warning_rg1_zero_results_warning_no-results_1 (0.00s)
    --- PASS: TestDecideOutcomeMatrix/fatal_code_with_stderr_results_error_overlay (0.00s)
    --- PASS: TestDecideOutcomeMatrix/unknown-only_zero_results_warning_no-results_1 (0.00s)
    --- PASS: TestDecideOutcomeMatrix/malformed_with_usable_results_browse_warning_0 (0.00s)
    --- PASS: TestDecideOutcomeMatrix/malformed_zero_usable_results_record-loss_fatal_2 (0.00s)
    --- PASS: TestDecideOutcomeMatrix/oversized_zero_usable_results_record-loss_fatal_2 (0.00s)
=== RUN   TestDecideOutcomeComposedOrder
=== RUN   TestDecideOutcomeComposedOrder/fatal_integrity_with_stderr_composes_both_in_order
=== RUN   TestDecideOutcomeComposedOrder/fatal_exit_no_stderr_generated_line_then_causes_then_record_loss
=== RUN   TestDecideOutcomeComposedOrder/signal_death_no_stderr_names_signal_then_causes
=== RUN   TestDecideOutcomeComposedOrder/fatal_exit_with_stderr_uses_stderr_then_causes
=== RUN   TestDecideOutcomeComposedOrder/second_summary_sole_line_is_extra_summary_record
=== RUN   TestDecideOutcomeComposedOrder/post-summary_begin_sole_line_is_record_after_summary
=== RUN   TestDecideOutcomeComposedOrder/post-summary_unterminated_fragment_is_record_after_summary_plus_malformed
=== RUN   TestDecideOutcomeComposedOrder/post-summary_oversized_is_record_after_summary_plus_oversized_detail
=== RUN   TestDecideOutcomeComposedOrder/post-summary_unknown_type_is_record_after_summary_plus_unknown_warning
=== RUN   TestDecideOutcomeComposedOrder/unterminated_record_dual_representation_with_malformed_aggregate
=== RUN   TestDecideOutcomeComposedOrder/repeated_orphaned_matches_emit_one_line_each_uncapped
=== RUN   TestDecideOutcomeComposedOrder/two_missing_ends_ordered_by_unsigned_raw-path_bytes
=== RUN   TestDecideOutcomeComposedOrder/non-fatal_warning_composes_stderr_then_record_loss_then_unknown
=== RUN   TestDecideOutcomeComposedOrder/incomplete_stream_rg0_no_stderr_names_cause_not_process_status
--- PASS: TestDecideOutcomeComposedOrder (0.00s)
    --- PASS: TestDecideOutcomeComposedOrder/fatal_integrity_with_stderr_composes_both_in_order (0.00s)
    --- PASS: TestDecideOutcomeComposedOrder/fatal_exit_no_stderr_generated_line_then_causes_then_record_loss (0.00s)
    --- PASS: TestDecideOutcomeComposedOrder/signal_death_no_stderr_names_signal_then_causes (0.00s)
    --- PASS: TestDecideOutcomeComposedOrder/fatal_exit_with_stderr_uses_stderr_then_causes (0.00s)
    --- PASS: TestDecideOutcomeComposedOrder/second_summary_sole_line_is_extra_summary_record (0.00s)
    --- PASS: TestDecideOutcomeComposedOrder/post-summary_begin_sole_line_is_record_after_summary (0.00s)
    --- PASS: TestDecideOutcomeComposedOrder/post-summary_unterminated_fragment_is_record_after_summary_plus_malformed (0.00s)
    --- PASS: TestDecideOutcomeComposedOrder/post-summary_oversized_is_record_after_summary_plus_oversized_detail (0.00s)
    --- PASS: TestDecideOutcomeComposedOrder/post-summary_unknown_type_is_record_after_summary_plus_unknown_warning (0.00s)
    --- PASS: TestDecideOutcomeComposedOrder/unterminated_record_dual_representation_with_malformed_aggregate (0.00s)
    --- PASS: TestDecideOutcomeComposedOrder/repeated_orphaned_matches_emit_one_line_each_uncapped (0.00s)
    --- PASS: TestDecideOutcomeComposedOrder/two_missing_ends_ordered_by_unsigned_raw-path_bytes (0.00s)
    --- PASS: TestDecideOutcomeComposedOrder/non-fatal_warning_composes_stderr_then_record_loss_then_unknown (0.00s)
    --- PASS: TestDecideOutcomeComposedOrder/incomplete_stream_rg0_no_stderr_names_cause_not_process_status (0.00s)
=== RUN   TestOutcomeIntegrityDiagnosticsFlow
=== RUN   TestOutcomeIntegrityDiagnosticsFlow/missing_summary_names_the_cause_not_the_exit_code
=== RUN   TestOutcomeIntegrityDiagnosticsFlow/missing_end_names_the_escaped_path
=== RUN   TestOutcomeIntegrityDiagnosticsFlow/context_after_summary_is_record_after_summary
=== RUN   TestOutcomeIntegrityDiagnosticsFlow/damaged_stream_with_real_stderr_composes_all_causes_together
=== RUN   TestOutcomeIntegrityDiagnosticsFlow/post-summary_unterminated_fragment_is_record_after_summary_plus_malformed
=== RUN   TestOutcomeIntegrityDiagnosticsFlow/post-summary_unknown_type_is_record_after_summary_plus_unknown_warning
=== RUN   TestOutcomeIntegrityDiagnosticsFlow/post-summary_oversized_is_record_after_summary_plus_oversized_detail
=== RUN   TestOutcomeIntegrityDiagnosticsFlow/path_with_newline_cannot_forge_paragraph_breaks
--- PASS: TestOutcomeIntegrityDiagnosticsFlow (0.00s)
    --- PASS: TestOutcomeIntegrityDiagnosticsFlow/missing_summary_names_the_cause_not_the_exit_code (0.00s)
    --- PASS: TestOutcomeIntegrityDiagnosticsFlow/missing_end_names_the_escaped_path (0.00s)
    --- PASS: TestOutcomeIntegrityDiagnosticsFlow/context_after_summary_is_record_after_summary (0.00s)
    --- PASS: TestOutcomeIntegrityDiagnosticsFlow/damaged_stream_with_real_stderr_composes_all_causes_together (0.00s)
    --- PASS: TestOutcomeIntegrityDiagnosticsFlow/post-summary_unterminated_fragment_is_record_after_summary_plus_malformed (0.00s)
    --- PASS: TestOutcomeIntegrityDiagnosticsFlow/post-summary_unknown_type_is_record_after_summary_plus_unknown_warning (0.00s)
    --- PASS: TestOutcomeIntegrityDiagnosticsFlow/post-summary_oversized_is_record_after_summary_plus_oversized_detail (0.00s)
    --- PASS: TestOutcomeIntegrityDiagnosticsFlow/path_with_newline_cannot_forge_paragraph_breaks (0.00s)
=== RUN   TestOutcomeOversizedAggregateDiagnostics
=== RUN   TestOutcomeOversizedAggregateDiagnostics/one_oversized_record_emits_singular_aggregate_then_detail
=== RUN   TestOutcomeOversizedAggregateDiagnostics/two_oversized_records_for_same_path_emit_plural_aggregate_and_one_detail
=== RUN   TestOutcomeOversizedAggregateDiagnostics/anonymous_oversized_zero_usable_results_fatal_aggregate_only
=== RUN   TestOutcomeOversizedAggregateDiagnostics/anonymous_oversized_with_usable_results_surfaces_aggregate_in_overlay_and_replay
=== RUN   TestOutcomeOversizedAggregateDiagnostics/mixed_recoverability_aggregate_totals_every_record_and_names_distinct_paths_once
=== RUN   TestOutcomeOversizedAggregateDiagnostics/aggregate_sits_after_malformed_aggregate_and_before_per-path_details
--- PASS: TestOutcomeOversizedAggregateDiagnostics (0.00s)
    --- PASS: TestOutcomeOversizedAggregateDiagnostics/one_oversized_record_emits_singular_aggregate_then_detail (0.00s)
    --- PASS: TestOutcomeOversizedAggregateDiagnostics/two_oversized_records_for_same_path_emit_plural_aggregate_and_one_detail (0.00s)
    --- PASS: TestOutcomeOversizedAggregateDiagnostics/anonymous_oversized_zero_usable_results_fatal_aggregate_only (0.00s)
    --- PASS: TestOutcomeOversizedAggregateDiagnostics/anonymous_oversized_with_usable_results_surfaces_aggregate_in_overlay_and_replay (0.00s)
    --- PASS: TestOutcomeOversizedAggregateDiagnostics/mixed_recoverability_aggregate_totals_every_record_and_names_distinct_paths_once (0.00s)
    --- PASS: TestOutcomeOversizedAggregateDiagnostics/aggregate_sits_after_malformed_aggregate_and_before_per-path_details (0.00s)
PASS
ok  	vrg/internal/app
```

## Binary demo: oversized record with recoverable path, then valid records and summary

Using the built vrg binary with a fake rg (fakebin-recoverable/rg) that emits one oversized match record (over the 64 MiB payload limit) naming demo/huge.txt, then a valid begin/match/end stream for demo/test.txt, then a summary, and exits 0. The PTY helper sends q to dismiss the warning overlay to browse, then q again to quit. The stripped output shows the warning overlay containing both the aggregate count line and the named-path detail line, then the browse view, then the same two-line diagnostic replayed to stderr after terminal restoration. The exit code is 0.

```bash
cd /home/chris/vrg/Notes/walkthroughs/037-04/code-walkthrough && PATH=$(pwd)/fakebin-recoverable:/usr/bin:/bin VRG_KEYS='q,q' VRG_DELAY=1.0 timeout 20 python3 runpty.py ./vrg hello demo 2>&1; echo exit=$?
```

```output
Searching…┌────────────────────────────────────────────┐
│ 1 oversized record skipped│
│ oversized record skipped for demo/huge.txt │
└────────────────────────────────────────────┘demo/test.txt   ── demo/test.txt ── 1  hello world1 oversized record skipped
oversized record skipped for demo/huge.txt
exit=0
```

## Binary demo: anonymous oversized record — aggregate only, never empty

Using a fake rg (fakebin-anonymous/rg) that emits one oversized match record whose data carries no path field — the limit is hit before any usable path is parsed, so per-path recovery produces nothing — followed by a valid summary, then exits 0. With zero usable results this is a record-loss fatal outcome: the overlay contains exactly the aggregate "1 oversized record skipped" with no path line — never an empty or absent diagnostic. The PTY helper sends q, which exits 2, and the same aggregate line is replayed to stderr after terminal restoration.

```bash
cd /home/chris/vrg/Notes/walkthroughs/037-04/code-walkthrough && PATH=$(pwd)/fakebin-anonymous:/usr/bin:/bin VRG_KEYS='q' VRG_DELAY=1.0 timeout 20 python3 runpty.py ./vrg hello demo 2>&1; echo exit=$?
```

```output
Searching…┌────────────────────────────┐
│ 1 oversized record skipped │
└────────────────────────────┘1 oversized record skipped
exit=2
```

## Summary

The walkthrough demonstrates:
- Full gates: go build, go vet, and the complete test suite all pass.
- App oversized-diagnostics tests: the always-emitted pluralized aggregate ("1 oversized record skipped" / "N oversized records skipped"), per-path details deduplicated to one line per distinct raw path in first-occurrence order, both anonymous-record outcomes (fatal aggregate-only overlay exiting 2; non-fatal warning overlay plus stderr replay), mixed recoverability, and the aggregate's position inside Issue #36's universal order.
- The binary with a fake rg emitting one recoverable-path oversized record, then valid records and a summary (exit 0): the warning overlay contains both the aggregate count and the named-path line, dismissal returns to browse, the same two lines replay to stderr, and q exits 0.
- The binary with a fake rg emitting one anonymous oversized record (no recoverable path) and a summary (exit 0): the fatal overlay contains exactly the aggregate line with no path line — never empty or absent — the same line replays to stderr, and q exits 2.

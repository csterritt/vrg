# Issue #37: oversized-record diagnostics — always-emitted aggregate, deduplicated per-path details, anonymous-record coverage

*2026-09-18T04:48:10Z by Showboat 0.6.1*
<!-- showboat-id: dd688a70-b335-43ed-8703-307666bf5a0f -->

Issue #37 completes the oversized-record half of the record-loss diagnostic in internal/app/outcome.go's recordLossLines: the pluralized per-record aggregate — exactly '1 oversized record skipped' or 'N oversized records skipped' — is emitted whenever the oversized count is positive regardless of path recovery, and the 'oversized record skipped for <sanitized path>' details are deduplicated by raw path in first-occurrence order beneath the aggregate. An anonymous oversized record (the 64 MiB limit hit before type/data.path parsed) therefore surfaces in the overlay and the stderr replay instead of producing an empty or absent diagnostic. See Notes/issues/037-oversized-record-aggregate-anonymous-diagnostics.md, Notes/tasks/037-oversized-record-aggregate-anonymous-diagnostics.md, and the 'Result index, records, and stream integrity' (oversized bullets) and 'Resources and responsiveness' (64 MiB limit) sections of Notes/PRD-vrg.md. This walkthrough demonstrates the outcome tests, then runs the issue's manual scenarios through the Issue #4-style fake-rg PTY harness. Artifacts (the built vrg binary and smoke.py) live in this directory.

## Outcome-test suite — internal/app

TestOversizedAggregateDiagnostics pins the oversized record-loss composition at the DecideOutcome level: singular and plural aggregates, anonymous counts, per-raw-path deduplication, and first-occurrence ordering. TestOversizedDiagnostics feeds real oversized streams — built by the oversizedRecordNamed/oversizedRecordAnonymous fixture helpers — through the real collection command and asserts the complete ordered overlay line list and the identical collected-diagnostics list for the named, anonymous, plural, duplicate-path, mixed-recoverability, and aggregates-before-details cases. TestOutcomeMatrix gains the anonymous fatal row (overlay-only, exactly the aggregate, exit 2) and the anonymous and named non-fatal rows (browse under the overlay, exit 0). TestIntegrityDiagnostics keeps the post-summary oversized fixture at [record after summary, 1 oversized record skipped, oversized record skipped for q.txt].

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestOversized|TestOutcomeMatrix|TestIntegrityDiagnostics|TestRecordLossDiagnostics' ./internal/app 2>&1 | grep -E '^( *--- (PASS|FAIL|SKIP)|ok|FAIL)' | sed -E 's/\([0-9.]+s\)//g; s/\t[0-9.]+s$//; s/^ +//' ; echo "app exit=${PIPESTATUS[0]}"
```

```output
--- PASS: TestOutcomeMatrix 
--- PASS: TestOutcomeMatrix/rg_0_clean_stream_browses 
--- PASS: TestOutcomeMatrix/anomalous_rg_1_with_retained_results_browses 
--- PASS: TestOutcomeMatrix/rg_1_empty_complete_stream_shows_no_results 
--- PASS: TestOutcomeMatrix/Esc_on_the_no-results_base_state_never_exits 
--- PASS: TestOutcomeMatrix/Esc_on_the_browse_base_state_never_exits 
--- PASS: TestOutcomeMatrix/fatal_exit_code_with_results_browses_under_the_overlay 
--- PASS: TestOutcomeMatrix/fatal_exit_code_with_results_dismissed_by_q 
--- PASS: TestOutcomeMatrix/fatal_exit_code_without_results_exits_on_q_dismissal 
--- PASS: TestOutcomeMatrix/fatal_exit_code_without_results_exits_on_Esc_dismissal 
--- PASS: TestOutcomeMatrix/failed_process_without_stderr_names_the_exit_code 
--- PASS: TestOutcomeMatrix/signal_death_with_results_names_the_signal 
--- PASS: TestOutcomeMatrix/signal_death_without_results_names_the_signal 
--- PASS: TestOutcomeMatrix/missing_summary_with_valid_matches_is_fatal 
--- PASS: TestOutcomeMatrix/orphaned_end_with_valid_matches_is_fatal 
--- PASS: TestOutcomeMatrix/integrity_failure_without_usable_results 
--- PASS: TestOutcomeMatrix/stderr_on_rg_0_warns_over_browse 
--- PASS: TestOutcomeMatrix/stderr_on_rg_1_empty_warns_to_no_results 
--- PASS: TestOutcomeMatrix/all_binary_after_a_warning_keeps_the_skip_count 
--- PASS: TestOutcomeMatrix/unknown_types_with_zero_results_warn_to_no_results 
--- PASS: TestOutcomeMatrix/malformed_skipped_with_usable_results_browses_under_overlay 
--- PASS: TestOutcomeMatrix/malformed_skipped_with_zero_usable_results_exits_on_q 
--- PASS: TestOutcomeMatrix/malformed_skipped_with_zero_usable_results_exits_on_Esc 
--- PASS: TestOutcomeMatrix/skipped_record_plus_binary_exclusion_is_fatal 
--- PASS: TestOutcomeMatrix/anonymous_oversized_with_no_usable_results_is_fatal 
--- PASS: TestOutcomeMatrix/anonymous_oversized_with_usable_results_browses 
--- PASS: TestOutcomeMatrix/named_oversized_with_usable_results_browses 
--- PASS: TestOutcomeMatrix/missing_end_with_retained_matches_browses_under_overlay 
--- PASS: TestOutcomeMatrix/missing_end_with_no_matches_is_fatal 
--- PASS: TestOutcomeMatrix/ctrl+c_after_completion_in_browse_exits_130 
--- PASS: TestOutcomeMatrix/ctrl+c_after_completion_in_no_results_exits_130 
--- PASS: TestOutcomeMatrix/ctrl+c_in_an_open_overlay_exits_130 
--- PASS: TestOutcomeMatrix/all_loads_fail_with_fixed_status_0_still_exits_0 
--- PASS: TestOutcomeMatrix/current-file_failure_with_fixed_status_2_still_exits_2 
--- PASS: TestOutcomeMatrix/all_loads_fail_with_fixed_status_2_still_exits_2 
--- PASS: TestOutcomeMatrix/all_stops_stale_with_fixed_status_0_still_exits_0 
--- PASS: TestOutcomeMatrix/all_files_unsupported_with_fixed_status_0_still_exits_0 
--- PASS: TestRecordLossDiagnostics 
--- PASS: TestOversizedAggregateDiagnostics 
--- PASS: TestOversizedAggregateDiagnostics/one_anonymous_record 
--- PASS: TestOversizedAggregateDiagnostics/one_named_record 
--- PASS: TestOversizedAggregateDiagnostics/two_anonymous_records 
--- PASS: TestOversizedAggregateDiagnostics/two_records_two_paths 
--- PASS: TestOversizedAggregateDiagnostics/two_records_one_path 
--- PASS: TestOversizedAggregateDiagnostics/repeats_keep_first-occurrence_order 
--- PASS: TestOversizedAggregateDiagnostics/escapable_path_deduplicates_by_raw_bytes 
--- PASS: TestOversizedDiagnostics 
--- PASS: TestOversizedDiagnostics/one_named_oversized_record 
--- PASS: TestOversizedDiagnostics/anonymous_oversized_emits_the_aggregate_alone 
--- PASS: TestOversizedDiagnostics/anonymous_oversized_alone_is_exactly_the_aggregate 
--- PASS: TestOversizedDiagnostics/two_named_oversized_records 
--- PASS: TestOversizedDiagnostics/same_path_deduplicates_the_detail 
--- PASS: TestOversizedDiagnostics/mixed_recoverability_counts_every_record 
--- PASS: TestOversizedDiagnostics/aggregates_precede_the_per-path_details 
--- PASS: TestIntegrityDiagnostics 
--- PASS: TestIntegrityDiagnostics/duplicate_begin_names_the_open_path 
--- PASS: TestIntegrityDiagnostics/orphaned_match_names_the_never-opened_path 
--- PASS: TestIntegrityDiagnostics/match_after_end_is_the_orphaned_match 
--- PASS: TestIntegrityDiagnostics/match_after_binary_end_is_the_orphaned_match 
--- PASS: TestIntegrityDiagnostics/orphaned_end_names_the_never-opened_path 
--- PASS: TestIntegrityDiagnostics/duplicate_end_is_the_orphaned_end 
--- PASS: TestIntegrityDiagnostics/missing_end_names_the_still-open_path 
--- PASS: TestIntegrityDiagnostics/missing_summary_names_itself,_not_an_exit_code 
--- PASS: TestIntegrityDiagnostics/exit_1_emits_no_process-status_line 
--- PASS: TestIntegrityDiagnostics/second_summary_reports_only_the_extra_summary 
--- PASS: TestIntegrityDiagnostics/match_after_summary_reports_only_its_position 
--- PASS: TestIntegrityDiagnostics/context_after_summary_reports_only_its_position 
--- PASS: TestIntegrityDiagnostics/post-summary_begin_cannot_open_the_file 
--- PASS: TestIntegrityDiagnostics/post-summary_tail_is_after-summary_plus_malformed 
--- PASS: TestIntegrityDiagnostics/unterminated_tail_follows_the_missing_summary 
--- PASS: TestIntegrityDiagnostics/post-summary_malformed_keeps_both_representations 
--- PASS: TestIntegrityDiagnostics/post-summary_oversized_keeps_both_representations 
--- PASS: TestIntegrityDiagnostics/post-summary_unknown_keeps_both_representations 
--- PASS: TestIntegrityDiagnostics/stderr_precedes_integrity_causes 
--- PASS: TestIntegrityDiagnostics/fatal_exit_with_stderr_reports_both 
--- PASS: TestIntegrityDiagnostics/fatal_exit_without_stderr_generates_the_code_line 
--- PASS: TestIntegrityDiagnostics/signal_death_names_the_signal_then_the_causes 
--- PASS: TestIntegrityDiagnostics/all_components_compose_in_universal_order 
--- PASS: TestIntegrityDiagnostics/non-fatal_components_keep_the_universal_order 
--- PASS: TestIntegrityDiagnostics/repeated_violations_emit_one_line_each 
--- PASS: TestIntegrityDiagnostics/missing_ends_order_by_unsigned_raw-path_bytes 
--- PASS: TestIntegrityDiagnostics/embedded_path_escapes_through_EscapePath 
--- PASS: TestIntegrityDiagnosticsDeterministic 
ok  	vrg/internal/app
app exit=0
```

## Manual scenarios — fake-rg PTY harness

smoke.py drives the built vrg binary on a real PTY (separated stderr pipe, handshake side file, bounded condition polls only - no fixed delays) through the issue's manual scenarios plus the fatal anonymous case. The fake rg is a small Python script whose PAD constant pads a match record past the 64 MiB limit: emit_oversized_named puts type and data.path before the giant lines value so the path recovers, while emit_oversized_anon puts the giant lines value first so the limit hits before the path field is ever parsed.

Scenario 1: one oversized record with a recoverable path, then valid records and summary -> browse with an overlay containing both the aggregate count and the named-path line, exit 0, both lines replayed to stderr.

Scenario 2: an oversized record hitting the limit before its path field, beside valid records -> the aggregate present with no path line, never an empty or absent diagnostic, exit 0.

Scenario 3: the same anonymous record as the stream's only content -> zero usable results makes the record loss fatal: the overlay alone carries exactly the aggregate and dismissal exits 2.

```bash
python3 smoke.py; echo "smoke exit=$?"
```

```output
scenario 1: recoverable-path oversized record
  [PASS] named oversized exit 0
  [PASS] named oversized overlay shows the aggregate
  [PASS] named oversized overlay names the path
  [PASS] named oversized dismissal reveals browse
  [PASS] named oversized replay shows the aggregate
  [PASS] named oversized replay names the path
  [PASS] named-oversized cursor restored
  [PASS] named-oversized alt screen exited
  [PASS] named-oversized termios restored
scenario 2: anonymous oversized record, usable results
  [PASS] anonymous oversized exit 0
  [PASS] anonymous oversized overlay shows the aggregate
  [PASS] anonymous oversized emits no path line
  [PASS] anonymous oversized replay shows the aggregate
  [PASS] anonymous oversized replay has no path line
  [PASS] anonymous-oversized cursor restored
  [PASS] anonymous-oversized alt screen exited
  [PASS] anonymous-oversized termios restored
scenario 3: anonymous oversized record only, fatal
  [PASS] anonymous-only exit 2
  [PASS] anonymous-only fatal overlay shows the aggregate
  [PASS] anonymous-only overlay has no path line
  [PASS] anonymous-only replay shows the aggregate
  [PASS] anonymous-only cursor restored
  [PASS] anonymous-only alt screen exited
  [PASS] anonymous-only termios restored
all checks passed
smoke exit=0
```

All three PTY scenarios pass: the recoverable-path oversized record surfaces the aggregate plus the named-path line in the overlay and the stderr replay at exit 0; the anonymous oversized record surfaces the aggregate alone - in the overlay and the replay at exit 0 with usable results, and as the fatal overlay's exact whole content at exit 2 with none. No path line is ever emitted for an unrecovered path, and the fatal overlay is never empty. Together with the outcome tests this covers every Issue #37 acceptance criterion.

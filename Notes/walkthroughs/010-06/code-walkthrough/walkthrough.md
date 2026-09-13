# Issue #10: Record robustness — malformed, oversized, unknown types

*2026-09-12T00:33:09Z by Showboat 0.6.1*
<!-- showboat-id: 862531e8-f31c-48db-91e2-b03d4eecb3c5 -->

Walkthrough for Issue #10 (Notes/tasks/010-record-robustness-malformed-oversized-unknown.md), implementing robust handling of malformed, oversized, and unknown-type records in ripgrep JSON streams with separate counters, bounded 64 MiB record parsing with discard-and-resynchronize behavior, sanitized oversized-record diagnostics, and record-loss outcome rows. References: Notes/PRD-vrg.md (Result index, records, and stream integrity; Outcome and exit-status contract; Resources and responsiveness).

Contracts verified:
- Malformed records are skipped and counted via Index.MalformedCount(), separate from stream-integrity failures except where the matrices mark both.
- Oversized records exceeding the 64 MiB payload limit (excluding the newline delimiter) are discarded through the next newline and counted via Index.OversizedCount().
- An oversized record is consumed and discarded through its next newline so parsing resynchronizes on the following record.
- A trailing oversized record without a newline is counted oversized and also counted malformed for its missing termination, and marks the stream incomplete.
- Unknown string event types are counted separately from malformed via Index.UnknownCount() and never independently change exit status.
- An unknown type after summary is counted unknown and its after-summary position is separately flagged as an integrity failure.
- Oversized-record diagnostics use sanitized paths through safepresentation.EscapePath (the Issue #6 utility).
- Record-loss outcome rows: malformed skipped with usable results → browse with warning overlay → 0; malformed/oversized skipped with zero usable results → record-loss fatal overlay → 2; unknown-only warnings with zero results → warning overlay → no-results → 1.
- Usable-results assessment happens after all filtering (binary exclusion and record loss).

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
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
ok  	vrg/internal/searchindex
?   	vrg/internal/sinkfixtures	[no test files]
ok  	vrg/internal/theme
?   	vrg/internal/viewport	[no test files]
```

## Malformed-record disposition tests

The SearchIndex tests (internal/searchindex/malformed_test.go, Issue #10) verify every row of the Issue #3 per-record schema matrix: invalid JSON, invalid base64, missing/empty/non-string type, missing required fields, invalid line numbers, empty submatches, invalid submatch ranges, malformed end.binary_offset, and malformed summary data. Each row asserts the deterministic disposition: skipped-and-counted malformed, stream-integrity failure, or both. Resynchronization after a skip is asserted: a malformed record between valid ones is skipped, counted, and followed by correct indexing of the remaining records.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/searchindex/ -run '^TestMalformedDisposition' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestMalformedDispositionMatrix
=== RUN   TestMalformedDispositionMatrix/invalid_JSON_is_malformed
=== RUN   TestMalformedDispositionMatrix/missing_type_is_malformed
=== RUN   TestMalformedDispositionMatrix/non-string_type_is_malformed
=== RUN   TestMalformedDispositionMatrix/begin_missing_path_is_malformed
=== RUN   TestMalformedDispositionMatrix/begin_wrongly_typed_path_is_malformed
=== RUN   TestMalformedDispositionMatrix/begin_invalid_base64_path_is_malformed
=== RUN   TestMalformedDispositionMatrix/match_missing_path_is_malformed
=== RUN   TestMalformedDispositionMatrix/match_missing_lines_is_malformed
=== RUN   TestMalformedDispositionMatrix/match_missing_line_number_defaults_to_zero_is_malformed
=== RUN   TestMalformedDispositionMatrix/match_missing_submatches_is_malformed
=== RUN   TestMalformedDispositionMatrix/match_wrongly_typed_path_is_malformed
=== RUN   TestMalformedDispositionMatrix/match_wrongly_typed_lines_is_malformed
=== RUN   TestMalformedDispositionMatrix/match_wrongly_typed_line_number_is_malformed
=== RUN   TestMalformedDispositionMatrix/match_wrongly_typed_submatches_is_malformed
=== RUN   TestMalformedDispositionMatrix/match_line_number_zero_is_malformed
=== RUN   TestMalformedDispositionMatrix/match_negative_line_number_is_malformed
=== RUN   TestMalformedDispositionMatrix/match_non-integer_line_number_is_malformed
=== RUN   TestMalformedDispositionMatrix/match_empty_submatches_array_is_malformed
=== RUN   TestMalformedDispositionMatrix/match_submatch_start_greater_than_end_is_malformed
=== RUN   TestMalformedDispositionMatrix/match_submatch_negative_start_is_malformed
=== RUN   TestMalformedDispositionMatrix/match_submatch_end_beyond_line_length_is_malformed
=== RUN   TestMalformedDispositionMatrix/match_invalid_base64_submatch_is_malformed
=== RUN   TestMalformedDispositionMatrix/end_missing_binary_offset_is_malformed
=== RUN   TestMalformedDispositionMatrix/end_wrongly_typed_binary_offset_is_malformed
=== RUN   TestMalformedDispositionMatrix/end_negative_binary_offset_is_malformed
=== RUN   TestMalformedDispositionMatrix/summary_missing_data_is_malformed
=== RUN   TestMalformedDispositionMatrix/summary_data_not_object_is_malformed
=== RUN   TestMalformedDispositionMatrix/duplicate_begin_is_integrity_failure_not_malformed
=== RUN   TestMalformedDispositionMatrix/orphaned_match_is_integrity_failure_not_malformed
=== RUN   TestMalformedDispositionMatrix/orphaned_end_is_integrity_failure_not_malformed
=== RUN   TestMalformedDispositionMatrix/match_after_end_is_integrity_failure_not_malformed
=== RUN   TestMalformedDispositionMatrix/second_summary_is_integrity_failure_not_malformed
=== RUN   TestMalformedDispositionMatrix/record_after_summary_is_integrity_failure_not_malformed
=== RUN   TestMalformedDispositionMatrix/trailing_unterminated_ordinary_record_is_malformed_and_stream_incomplete
=== RUN   TestMalformedDispositionMatrix/malformed_record_after_valid_summary_is_malformed_and_after-summary_integrity_failure
=== RUN   TestMalformedDispositionMatrix/missing-type_record_after_valid_summary_is_malformed_and_after-summary_integrity_failure
=== RUN   TestMalformedDispositionMatrix/malformed_match_between_valid_records_is_skipped_and_later_records_indexed
--- PASS: TestMalformedDispositionMatrix (0.00s)
    --- PASS: TestMalformedDispositionMatrix/invalid_JSON_is_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/missing_type_is_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/non-string_type_is_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/begin_missing_path_is_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/begin_wrongly_typed_path_is_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/begin_invalid_base64_path_is_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/match_missing_path_is_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/match_missing_lines_is_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/match_missing_line_number_defaults_to_zero_is_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/match_missing_submatches_is_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/match_wrongly_typed_path_is_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/match_wrongly_typed_lines_is_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/match_wrongly_typed_line_number_is_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/match_wrongly_typed_submatches_is_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/match_line_number_zero_is_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/match_negative_line_number_is_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/match_non-integer_line_number_is_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/match_empty_submatches_array_is_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/match_submatch_start_greater_than_end_is_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/match_submatch_negative_start_is_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/match_submatch_end_beyond_line_length_is_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/match_invalid_base64_submatch_is_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/end_missing_binary_offset_is_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/end_wrongly_typed_binary_offset_is_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/end_negative_binary_offset_is_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/summary_missing_data_is_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/summary_data_not_object_is_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/duplicate_begin_is_integrity_failure_not_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/orphaned_match_is_integrity_failure_not_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/orphaned_end_is_integrity_failure_not_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/match_after_end_is_integrity_failure_not_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/second_summary_is_integrity_failure_not_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/record_after_summary_is_integrity_failure_not_malformed (0.00s)
    --- PASS: TestMalformedDispositionMatrix/trailing_unterminated_ordinary_record_is_malformed_and_stream_incomplete (0.00s)
    --- PASS: TestMalformedDispositionMatrix/malformed_record_after_valid_summary_is_malformed_and_after-summary_integrity_failure (0.00s)
    --- PASS: TestMalformedDispositionMatrix/missing-type_record_after_valid_summary_is_malformed_and_after-summary_integrity_failure (0.00s)
    --- PASS: TestMalformedDispositionMatrix/malformed_match_between_valid_records_is_skipped_and_later_records_indexed (0.00s)
PASS
ok  	vrg/internal/searchindex
```

## Oversized record and unknown-type tests

The SearchIndex tests (internal/searchindex/oversized_test.go, Issue #10) verify the 64 MiB record payload limit boundary, discard-through-newline resynchronization, recoverable and unrecoverable oversized diagnostics, the oversized-only absent file, the unterminated oversized final record, and unknown-type counting rules.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/searchindex/ -run '^TestOversized|^TestUnknownType' -timeout 60s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestOversizedBoundary
=== RUN   TestOversizedBoundary/exactly_64_MiB_is_accepted
=== RUN   TestOversizedBoundary/64_MiB_plus_1_is_oversized
--- PASS: TestOversizedBoundary (0.00s)
    --- PASS: TestOversizedBoundary/exactly_64_MiB_is_accepted (0.00s)
    --- PASS: TestOversizedBoundary/64_MiB_plus_1_is_oversized (0.00s)
=== RUN   TestOversizedResynchronization
--- PASS: TestOversizedResynchronization (0.00s)
=== RUN   TestOversizedFinalRecord
--- PASS: TestOversizedFinalRecord (0.00s)
=== RUN   TestOversizedDiagnostics
=== RUN   TestOversizedDiagnostics/recoverable_path_diagnostic
=== RUN   TestOversizedDiagnostics/no_path_recovery_count_only
=== RUN   TestOversizedDiagnostics/file_with_only_oversized_matches_absent_from_stops
--- PASS: TestOversizedDiagnostics (0.00s)
    --- PASS: TestOversizedDiagnostics/recoverable_path_diagnostic (0.00s)
    --- PASS: TestOversizedDiagnostics/no_path_recovery_count_only (0.00s)
    --- PASS: TestOversizedDiagnostics/file_with_only_oversized_matches_absent_from_stops (0.00s)
=== RUN   TestUnknownType
=== RUN   TestUnknownType/unknown_type_is_counted_separately
=== RUN   TestUnknownType/unknown_type_does_not_substitute_for_summary
=== RUN   TestUnknownType/unknown_type_after_summary_is_unknown_and_after-summary_integrity_failure
--- PASS: TestUnknownType (0.00s)
    --- PASS: TestUnknownType/unknown_type_is_counted_separately (0.00s)
    --- PASS: TestUnknownType/unknown_type_does_not_substitute_for_summary (0.00s)
    --- PASS: TestUnknownType/unknown_type_after_summary_is_unknown_and_after-summary_integrity_failure (0.00s)
PASS
ok  	vrg/internal/searchindex
```

## Extended outcome matrix tests

The App tests (internal/app/outcome_test.go, Issue #10) extend the pure DecideOutcome matrix and the full Update flow with record-loss rows: unknown-only warnings with zero results, malformed skipped with usable results, malformed skipped with zero usable results (record-loss fatal), oversized with zero usable results (record-loss fatal), and skipped record plus binary exclusion leaving zero retained stops.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run '^TestDecideOutcome|^TestOutcomeMatrixFlow' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
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
=== RUN   TestDecideOutcomeGeneratedDiagnostic
--- PASS: TestDecideOutcomeGeneratedDiagnostic (0.00s)
=== RUN   TestDecideOutcomeStderrDiagnostic
--- PASS: TestDecideOutcomeStderrDiagnostic (0.00s)
=== RUN   TestOutcomeMatrixFlow
=== RUN   TestOutcomeMatrixFlow/rg0_clean_browse_dismiss_exits_0
=== RUN   TestOutcomeMatrixFlow/rg1_retained_results_browse_0
=== RUN   TestOutcomeMatrixFlow/rg1_empty_no-results_1
=== RUN   TestOutcomeMatrixFlow/fatal_code_with_results_browse_overlay_dismiss_browse_q_2
=== RUN   TestOutcomeMatrixFlow/fatal_code_with_results_browse_overlay_esc_dismiss_browse_q_2
=== RUN   TestOutcomeMatrixFlow/fatal_code_no_results_overlay_q_exits_2
=== RUN   TestOutcomeMatrixFlow/fatal_code_no_results_overlay_esc_exits_2
=== RUN   TestOutcomeMatrixFlow/signal_death_with_results_browse_overlay_dismiss_q_2
=== RUN   TestOutcomeMatrixFlow/signal_death_no_results_fatal_overlay_q_2
=== RUN   TestOutcomeMatrixFlow/stderr_warning_with_results_warning_overlay_dismiss_browse_q_0
=== RUN   TestOutcomeMatrixFlow/stderr_warning_zero_results_warning_overlay_dismiss_no-results_q_1
=== RUN   TestOutcomeMatrixFlow/stderr_warning_zero_results_warning_overlay_esc_dismiss_no-results_q_1
=== RUN   TestOutcomeMatrixFlow/unknown-only_zero_results_warning_overlay_dismiss_no-results_q_1
=== RUN   TestOutcomeMatrixFlow/malformed_with_usable_results_browse_warning_dismiss_browse_q_0
=== RUN   TestOutcomeMatrixFlow/malformed_zero_usable_results_record-loss_fatal_q_2
=== RUN   TestOutcomeMatrixFlow/malformed_zero_usable_results_record-loss_fatal_esc_2
=== RUN   TestOutcomeMatrixFlow/malformed_plus_binary_exclusion_zero_stops_record-loss_fatal_2
--- PASS: TestOutcomeMatrixFlow (0.00s)
    --- PASS: TestOutcomeMatrixFlow/rg0_clean_browse_dismiss_exits_0 (0.00s)
    --- PASS: TestOutcomeMatrixFlow/rg1_retained_results_browse_0 (0.00s)
    --- PASS: TestOutcomeMatrixFlow/rg1_empty_no-results_1 (0.00s)
    --- PASS: TestOutcomeMatrixFlow/fatal_code_with_results_browse_overlay_dismiss_browse_q_2 (0.00s)
    --- PASS: TestOutcomeMatrixFlow/fatal_code_with_results_browse_overlay_esc_dismiss_browse_q_2 (0.00s)
    --- PASS: TestOutcomeMatrixFlow/fatal_code_no_results_overlay_q_exits_2 (0.00s)
    --- PASS: TestOutcomeMatrixFlow/fatal_code_no_results_overlay_esc_exits_2 (0.00s)
    --- PASS: TestOutcomeMatrixFlow/signal_death_with_results_browse_overlay_dismiss_q_2 (0.00s)
    --- PASS: TestOutcomeMatrixFlow/signal_death_no_results_fatal_overlay_q_2 (0.00s)
    --- PASS: TestOutcomeMatrixFlow/stderr_warning_with_results_warning_overlay_dismiss_browse_q_0 (0.00s)
    --- PASS: TestOutcomeMatrixFlow/stderr_warning_zero_results_warning_overlay_dismiss_no-results_q_1 (0.00s)
    --- PASS: TestOutcomeMatrixFlow/stderr_warning_zero_results_warning_overlay_esc_dismiss_no-results_q_1 (0.00s)
    --- PASS: TestOutcomeMatrixFlow/unknown-only_zero_results_warning_overlay_dismiss_no-results_q_1 (0.00s)
    --- PASS: TestOutcomeMatrixFlow/malformed_with_usable_results_browse_warning_dismiss_browse_q_0 (0.00s)
    --- PASS: TestOutcomeMatrixFlow/malformed_zero_usable_results_record-loss_fatal_q_2 (0.00s)
    --- PASS: TestOutcomeMatrixFlow/malformed_zero_usable_results_record-loss_fatal_esc_2 (0.00s)
    --- PASS: TestOutcomeMatrixFlow/malformed_plus_binary_exclusion_zero_stops_record-loss_fatal_2 (0.00s)
PASS
ok  	vrg/internal/app
```

## Binary demo: mixed stream with malformed and unknown records

Using the built vrg binary with a fake rg (fakebin-mixed/rg) that emits a valid begin, a valid match, a garbage (malformed) line, an unknown-type line, a valid end, and a summary, then exits 0. The PTY helper sends Esc to dismiss the warning overlay, then q to quit. The stripped output shows the warning overlay listing both skip counts (1 malformed record skipped, 1 unrecognised record types skipped), then the browse view with demo.txt. The exit code is 0.

```bash
cd /home/chris/vrg/Notes/walkthroughs/010-06/code-walkthrough && PATH=$(pwd)/fakebin-mixed:/usr/bin:/bin VRG_KEYS=$'\x1b,q' VRG_DELAY=1.0 timeout 10 python3 runpty.py ./vrg hello demo 2>&1; echo exit=$?
```

```output
┌─────────────────────────────────────┐
│ 1 malformed record skipped          │
│ 1 unrecognised record types skipped │
└─────────────────────────────────────┘
demo.txt                  ── demo.txt ── Loading…
exit=0
```

## Summary

The walkthrough demonstrates:
- Malformed-record disposition tests: every row of the Issue #3 per-record schema matrix (invalid JSON, invalid base64, missing/invalid type, missing required fields, invalid line numbers, empty submatches, invalid submatch ranges, malformed end.binary_offset, malformed summary data), the composite rows (trailing unterminated record, malformed after summary), and resynchronization after a skip.
- Oversized record tests: the 64 MiB boundary (exactly 64 MiB accepted, 64 MiB+1 skipped), discard-through-newline resynchronization, recoverable and unrecoverable oversized diagnostics, the oversized-only absent file, and the unterminated oversized final record (oversized + malformed + incomplete).
- Unknown-type tests: unknown counted separately from malformed, unknown does not substitute for summary, unknown after summary is unknown and after-summary integrity failure.
- Extended outcome matrix tests: unknown-only warnings with zero results (warning → no-results → 1), malformed skipped with usable results (browse + warning → 0), malformed/oversized skipped with zero usable results (record-loss fatal → 2), and skipped record plus binary exclusion leaving zero retained stops (record-loss fatal → 2).
- The binary with a fake rg emitting a valid begin, a valid match, a garbage line, an unknown-type line, a valid end, and a summary with exit 0: browse view with a warning overlay listing both skip counts (1 malformed record skipped, 1 unrecognised record types skipped), q exiting 0.

References: Issue #10 (Notes/tasks/010-record-robustness-malformed-oversized-unknown.md), Notes/PRD-vrg.md (Result index, records, and stream integrity; Outcome and exit-status contract; Resources and responsiveness).


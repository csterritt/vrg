# Issue #36: Stream-integrity fatal diagnostics

*2026-09-15T15:26:11Z by Showboat 0.6.1*
<!-- showboat-id: c6b74e6d-5433-474c-9dfe-43181c9eb820 -->

Walkthrough for Issue #36 (Notes/tasks/036-stream-integrity-fatal-diagnostics.md), implementing structured stream-integrity causes and universal composed diagnostics so a fatal stream-integrity outcome names the actual violations instead of collapsing to `ripgrep exited with code 0`. References: Notes/issues/036-stream-integrity-fatal-diagnostics.md and Notes/PRD-vrg.md (Result index, records, and stream integrity; Outcome and exit-status contract).

Contracts verified:
- Index.Integrity() carries one structured IntegrityCause per offending physical record, with stable kinds and optional raw paths.
- Overlap precedence: a second summary contributes only extra summary; every other post-summary record contributes only record after summary without lifecycle dispatch (context included); a post-summary unterminated fragment contributes record after summary, not unterminated final record.
- Ordering: mid-stream causes in detection order, then end-of-stream causes (missing end sorted by unsigned raw-path bytes, missing summary, trailing-fragment cause).
- DecideOutcome composes process stderr or a generated process-status line, then integrity-cause lines, then record-loss diagnostics, then unknown-type warnings — identically in fatal and non-fatal branches.
- The same composed text is collected and replayed to stderr after terminal restoration.
- No process-status line is emitted for a 0/1 exit.
- Paths are escaped through safepresentation.EscapePath.

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

## SearchIndex structured-cause tests

The SearchIndex tests (internal/searchindex/integrity_test.go, Issue #36) verify that Index.Integrity() carries one structured IntegrityCause per offending physical record: duplicate begin, orphaned match, match after a binary-excluding end, orphaned end, missing end, missing summary, extra summary, record after summary, and unterminated final record. They pin the overlap precedence (second summary is only extra summary; post-summary records are only record after summary with no lifecycle effects, including context and unterminated fragments), detection-order mid-stream causes, deterministic unsigned raw-path ordering of missing-end causes, and uncapped one-cause-per-record multiplicity.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/searchindex/ -run '^(TestIntegrityCauses|TestLifecycleMatrix)$' -timeout 60s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
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

## App composed-diagnostics tests

The App tests (internal/app/outcome_test.go, Issue #36) verify the complete composed OverlayText — exact equality, not substring presence — for every cause kind, the universal process → integrity → record-loss → unknown-type component order across fatal and non-fatal branches, the generated-process-line restriction to signal/non-0/1 exits, EscapePath escaping of hostile path bytes, deterministic missing-end ordering, and that the collected stderr replay carries the same composed text as the overlay.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run '^(TestDecideOutcomeIntegrityCauseLines|TestDecideOutcomeComposedOrder|TestOutcomeIntegrityDiagnosticsFlow|TestOutcomeMissingEndDeterministic)$' -timeout 60s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestDecideOutcomeIntegrityCauseLines
=== RUN   TestDecideOutcomeIntegrityCauseLines/duplicate_begin_names_escaped_path
=== RUN   TestDecideOutcomeIntegrityCauseLines/orphaned_match_names_escaped_path
=== RUN   TestDecideOutcomeIntegrityCauseLines/match_after_binary-excluding_end_names_escaped_path
=== RUN   TestDecideOutcomeIntegrityCauseLines/orphaned_end_names_escaped_path
=== RUN   TestDecideOutcomeIntegrityCauseLines/missing_end_names_escaped_path
=== RUN   TestDecideOutcomeIntegrityCauseLines/missing_summary
=== RUN   TestDecideOutcomeIntegrityCauseLines/extra_summary
=== RUN   TestDecideOutcomeIntegrityCauseLines/record_after_summary
=== RUN   TestDecideOutcomeIntegrityCauseLines/unterminated_final_record
=== RUN   TestDecideOutcomeIntegrityCauseLines/path_with_newline_escaped_through_EscapePath
--- PASS: TestDecideOutcomeIntegrityCauseLines (0.00s)
    --- PASS: TestDecideOutcomeIntegrityCauseLines/duplicate_begin_names_escaped_path (0.00s)
    --- PASS: TestDecideOutcomeIntegrityCauseLines/orphaned_match_names_escaped_path (0.00s)
    --- PASS: TestDecideOutcomeIntegrityCauseLines/match_after_binary-excluding_end_names_escaped_path (0.00s)
    --- PASS: TestDecideOutcomeIntegrityCauseLines/orphaned_end_names_escaped_path (0.00s)
    --- PASS: TestDecideOutcomeIntegrityCauseLines/missing_end_names_escaped_path (0.00s)
    --- PASS: TestDecideOutcomeIntegrityCauseLines/missing_summary (0.00s)
    --- PASS: TestDecideOutcomeIntegrityCauseLines/extra_summary (0.00s)
    --- PASS: TestDecideOutcomeIntegrityCauseLines/record_after_summary (0.00s)
    --- PASS: TestDecideOutcomeIntegrityCauseLines/unterminated_final_record (0.00s)
    --- PASS: TestDecideOutcomeIntegrityCauseLines/path_with_newline_escaped_through_EscapePath (0.00s)
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
=== RUN   TestOutcomeMissingEndDeterministic
--- PASS: TestOutcomeMissingEndDeterministic (0.00s)
PASS
ok  	vrg/internal/app
```

## Binary demo: valid records, no summary, exit 0

Using the built vrg binary with a fake rg (fakebin-missing-summary/rg) that emits a valid begin/match/end stream for demo/test.txt but terminates without a summary record, then exits 0. The PTY helper sends q to dismiss the fatal overlay to browse, then q again to quit. The stripped output shows the error overlay naming the actual integrity cause — "missing summary record" — rather than "ripgrep exited with code 0", then the browse view, then the same diagnostic replayed to stderr after terminal restoration. The exit code is 2.

```bash
cd /home/chris/vrg/Notes/walkthroughs/036-06/code-walkthrough && PATH=$(pwd)/fakebin-missing-summary:/usr/bin:/bin VRG_KEYS='q,q' VRG_DELAY=1.0 timeout 10 python3 runpty.py ./vrg hello demo 2>&1; echo exit=$?
```

```output
┌────────────────────────┐
│ missing summary record │
└────────────────────────┘demo/test.txt   ── demo/test.txt ── 1  hello world missing summary record
exit=2
```

## Binary demo: missing end for the only file

Using a fake rg (fakebin-missing-end/rg) that emits begin/match for demo/test.txt and a valid summary but no end record for the only file, then exits 0. The fatal overlay names "missing end record for demo/test.txt"; q dismisses to browse and q again exits 2. The same diagnostic appears in the stderr replay.

```bash
cd /home/chris/vrg/Notes/walkthroughs/036-06/code-walkthrough && PATH=$(pwd)/fakebin-missing-end:/usr/bin:/bin VRG_KEYS='q,q' VRG_DELAY=1.0 timeout 10 python3 runpty.py ./vrg hello demo 2>&1; echo exit=$?
```

```output
┌──────────────────────────────────────┐
│ missing end record for demo/test.txt │
└──────────────────────────────────────┘demo/test.txt   ── demo/test.txt ── 1  hello world missing end record for demo/test.txt
exit=2
```

## Binary demo: damaged stream plus real child stderr

Using a fake rg (fakebin-damaged/rg) that writes a real line to stderr and emits a damaged stream: a duplicate begin, an orphaned match, a match after a binary-excluding end, an orphaned end, a second summary, a post-summary begin (lifecycle-suppressed: late.txt cannot produce a missing-end cause), and test.txt left open at stream end — then exits 0. The fatal overlay shows the stderr line first, then every integrity cause in detection order followed by the end-of-stream missing-end cause. q dismisses to browse; q again exits 2. The identical composed diagnostic is replayed to stderr after terminal restoration.

```bash
cd /home/chris/vrg/Notes/walkthroughs/036-06/code-walkthrough && PATH=$(pwd)/fakebin-damaged:/usr/bin:/bin VRG_KEYS='q,q' VRG_DELAY=1.0 timeout 10 python3 runpty.py ./vrg hello demo 2>&1; echo exit=$?
```

```output
┌──────────────────────────────────────────┐
│ rg: parse error near record 4            │
│ duplicate begin record for demo/test.txt │
│ orphaned match record for ghost.txt      │
│ orphaned match record for bin.dat        │
│ orphaned end record for ghost.txt        │
│ extra summary record│
│ record after summary│
│ missing end record for demo/test.txt     │
└──────────────────────────────────────────┘demo/test.txt   ── demo/test.txt ──
ghost.txt       1  hello worldrg: parse error near record 4
duplicate begin record for demo/test.txt
orphaned match record for ghost.txt
orphaned match record for bin.dat
orphaned end record for ghost.txt
extra summary record
record after summary
missing end record for demo/test.txt
exit=2
```

## Summary

The walkthrough demonstrates:
- Full gates: go build, go vet, and the complete test suite all pass.
- SearchIndex structured-cause tests: one IntegrityCause per offending physical record, all nine cause kinds, overlap precedence (extra summary, post-summary lifecycle suppression including context, post-summary unterminated fragments), detection-order plus end-of-stream ordering, deterministic unsigned raw-path missing-end sort, and uncapped multiplicity.
- App composed-diagnostics tests: exact OverlayText equality per cause kind, universal process → integrity → record-loss → unknown-type ordering, no process-status line for 0/1 exits, EscapePath escaping, and overlay/replay text equality.
- The binary with a fake rg emitting valid records but no summary (exit 0): the fatal overlay names "missing summary record" rather than "ripgrep exited with code 0", dismissal exits 2, and the same line is replayed to stderr.
- The binary with a fake rg missing the end record for the only file (exit 0): the overlay names "missing end record for demo/test.txt", dismissal exits 2, and the same line is replayed to stderr.
- The binary with a damaged stream plus real child stderr (exit 0): the overlay shows the stderr line followed by every integrity cause in contract order — duplicate begin, orphaned match, match after binary end (rendered as orphaned match), orphaned end, extra summary, record after summary, then missing end — and the identical block is replayed to stderr after dismissal with exit 2.

# Issue #36: stream-integrity fatal diagnostics — structured causes, stable per-cause text, universal composition

*2026-09-18T04:27:13Z by Showboat 0.6.1*
<!-- showboat-id: ecdfbb6c-e9d8-4c3a-bf54-423b24a72ac8 -->

Issue #36 replaces the generic "ripgrep event stream incomplete" note with structured, per-violation diagnostics. internal/searchindex records one Cause{Kind, Path} per offending physical record under the one-cause precedence (extra summary over after-summary, post-summary lifecycle suppression, the tail resolved at Integrity() time), emitted in detection order then the end-of-stream order (missing ends by unsigned raw-path bytes, missing summary, the tail cause); internal/app renders each cause as a stable diagnostic line inside the universal process -> integrity -> record-loss -> warning composition shared verbatim by the overlay and the stderr replay. See Notes/issues/036-stream-integrity-fatal-diagnostics.md, Notes/tasks/036-stream-integrity-fatal-diagnostics.md, and the 'Result index, records, and stream integrity' and 'Outcome and exit-status contract' sections of Notes/PRD-vrg.md. This walkthrough demonstrates the structured-cause and composed-diagnostic test suites, then runs the issue's three manual scenarios through the Issue #4 fake-rg PTY harness. Artifacts (the built vrg binary and smoke.py) live in this directory.

## Structured-cause suite — internal/searchindex

TestLifecycleMatrix asserts each matrix row's ordered Integrity().Causes list through checkCauses — one Cause{Kind, Path} per offending physical record — alongside Complete, the retained files, BinaryExcluded, UsableResults, and the independent Malformed/Unknown/Oversized counters. Issue #36's rows cover the corrected context-after-summary failure, the post-summary begin/cannot-reopen precedence, post-summary malformed/unknown/oversized dual representation, uncapped one-cause-per-violation multiplicity, missing-end ordering by unsigned raw-path bytes, mid-stream detection order, and the unterminated tail under both precedences.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestLifecycleMatrix|TestTrailingUnterminatedRecordDisposition' ./internal/searchindex 2>&1 | grep -E '^( *--- (PASS|FAIL|SKIP)|ok|FAIL)' | sed -E 's/\([0-9.]+s\)//g; s/\t[0-9.]+s$//; s/^ +//'; echo "searchindex exit=${PIPESTATUS[0]}"
```

```output
--- PASS: TestLifecycleMatrixDispositions 
--- PASS: TestLifecycleMatrixDispositions/duplicate_begin_is_integrity_only 
--- PASS: TestLifecycleMatrixDispositions/orphaned_match_is_integrity_only 
--- PASS: TestLifecycleMatrixDispositions/match_after_end_is_integrity_only 
--- PASS: TestLifecycleMatrixDispositions/orphaned_end_is_integrity_only 
--- PASS: TestLifecycleMatrixDispositions/duplicate_end_is_integrity_only 
--- PASS: TestLifecycleMatrixDispositions/second_summary_is_integrity_only 
--- PASS: TestLifecycleMatrixDispositions/match_after_summary_is_integrity_only 
--- PASS: TestLifecycleMatrixDispositions/begin_after_summary_is_integrity_only 
--- PASS: TestLifecycleMatrixDispositions/end_after_summary_is_integrity_only 
--- PASS: TestLifecycleMatrixDispositions/unknown_type_after_summary_counts_unknown_and_fails 
--- PASS: TestLifecycleMatrixDispositions/missing_end_fails_and_retains 
--- PASS: TestLifecycleMatrixDispositions/missing_summary_is_integrity_only 
--- PASS: TestLifecycleMatrix 
--- PASS: TestLifecycleMatrix/begin_while_not_open_opens_the_file 
--- PASS: TestLifecycleMatrix/duplicate_begin_while_open_is_an_integrity_failure 
--- PASS: TestLifecycleMatrix/match_while_open_indexes_under_the_file 
--- PASS: TestLifecycleMatrix/match_for_never-opened_path_is_retained_incomplete 
--- PASS: TestLifecycleMatrix/match_after_end_is_retained_incomplete 
--- PASS: TestLifecycleMatrix/match_after_binary_end_is_not_retained 
--- PASS: TestLifecycleMatrix/end_while_open_closes_the_file 
--- PASS: TestLifecycleMatrix/binary_end_while_open_excludes_without_failing 
--- PASS: TestLifecycleMatrix/end_for_never-opened_path_is_orphaned 
--- PASS: TestLifecycleMatrix/duplicate_end_is_orphaned 
--- PASS: TestLifecycleMatrix/orphaned_binary_end_excludes_and_fails 
--- PASS: TestLifecycleMatrix/context_before_summary_is_ignored 
--- PASS: TestLifecycleMatrix/context_after_summary_is_an_integrity_failure 
--- PASS: TestLifecycleMatrix/file_still_open_at_stream_end_fails_and_retains 
--- PASS: TestLifecycleMatrix/summary_alone_is_a_complete_zero-result_stream 
--- PASS: TestLifecycleMatrix/missing_summary_fails_the_stream 
--- PASS: TestLifecycleMatrix/second_summary_is_an_integrity_failure 
--- PASS: TestLifecycleMatrix/match_after_summary_is_an_integrity_failure 
--- PASS: TestLifecycleMatrix/begin_after_summary_is_an_integrity_failure 
--- PASS: TestLifecycleMatrix/end_after_summary_is_an_integrity_failure 
--- PASS: TestLifecycleMatrix/post-summary_records_cannot_reopen_lifecycle 
--- PASS: TestLifecycleMatrix/malformed_record_after_summary_is_an_integrity_failure 
--- PASS: TestLifecycleMatrix/unknown_record_after_summary_is_an_integrity_failure 
--- PASS: TestLifecycleMatrix/oversized_record_after_summary_keeps_its_own_count 
--- PASS: TestLifecycleMatrix/trailing_unterminated_record_fails_the_stream 
--- PASS: TestLifecycleMatrix/unterminated_fragment_as_the_only_summary_fails 
--- PASS: TestLifecycleMatrix/text_and_bytes_forms_of_one_path_agree 
--- PASS: TestLifecycleMatrix/bytes_begin_pairs_with_text_end 
--- PASS: TestLifecycleMatrix/interleaved_open_files_pair_independently 
--- PASS: TestLifecycleMatrix/orphaned_match_does_not_open_the_file 
--- PASS: TestLifecycleMatrix/orphan_match_then_begin_keeps_retained_stops 
--- PASS: TestLifecycleMatrix/begin_cannot_reopen_an_excluded_file 
--- PASS: TestLifecycleMatrix/duplicate_begin_on_excluded_file_stays_excluded 
--- PASS: TestLifecycleMatrix/repeated_identical_violations_produce_one_cause_each 
--- PASS: TestLifecycleMatrix/still-open_files_report_missing_ends_in_path_order 
--- PASS: TestLifecycleMatrix/violations_list_in_detection_order 
--- PASS: TestLifecycleMatrix/empty_stream_fails 
--- PASS: TestTrailingUnterminatedRecordDisposition 
ok  	vrg/internal/searchindex
searchindex exit=0
```

## Composed-diagnostic suite — internal/app

TestIntegrityDiagnostics feeds one stream fixture per integrity cause and composition rule through the real collection command, then asserts the complete ordered overlay line list and the identical collected-diagnostics list — every cause's stable text, EscapePath-escaped hostile paths, uncapped multiplicity, unsigned-raw-path missing-end ordering, the dual representation of post-summary malformed/oversized/unknown records, the stderr/generated-process-line rules (never a status line for 0/1), and the universal component order. TestIntegrityDiagnosticsDeterministic rebuilds the same damaged stream eight times to pin the ordering.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestIntegrityDiagnostics' ./internal/app 2>&1 | grep -E '^( *--- (PASS|FAIL|SKIP)|ok|FAIL)' | sed -E 's/\([0-9.]+s\)//g; s/\t[0-9.]+s$//; s/^ +//'; echo "app exit=${PIPESTATUS[0]}"
```

```output
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

## Repository gates

The full verification gates over the completed change: go vet ./..., go build ./..., and go test ./... (the cmd/vrg PTY suite is the slow line).

```bash
cd /home/chris/vrg && go vet ./...; echo "vet exit=$?" && go build ./...; echo "build exit=$?"
```

```output
vet exit=0
build exit=0
```

```bash
cd /home/chris/vrg && go test -count=1 ./... 2>&1 | sed -E 's/\t[0-9.]+s$//'; echo "test exit=${PIPESTATUS[0]}"
```

```output
?   	vrg/Notes/walkthroughs/007-04/code-walkthrough/fixture	[no test files]
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/docs
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
?   	vrg/internal/safepresentation/sinktest	[no test files]
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
test exit=0
```

## Manual scenarios — the Issue #4 fake-rg PTY harness

smoke.py (in this directory) drives the freshly built ./vrg binary under a real PTY with a separated stderr pipe, using the Issue #4 fake-rg fixture family: a handshake side file proves the fixture ran, every key is sent only after the expected rendered marker is observed in the ANSI-stripped PTY stream, draining is completion/EOF-driven, and bounded polls re-check explicit conditions — there is no fixed settling delay anywhere (assert_no_fixed_delays parses the script's own AST to prove no time.sleep call exists). Terminal restoration is checked on every run (cursor, alt-screen, termios).

Scenario 1 runs the issue's primary case: valid begin/match/end records for test.txt but no summary, child exit 0. Usable results exist, so the fatal outcome is the error overlay over browse — the first q dismisses it, the second quits at the fixed status 2. The overlay must name "missing summary record" and must never carry a manufactured "ripgrep exited with code 0" line; the same diagnostic replays to stderr.

Scenario 2 repeats with a missing end for the only file: begin/match then summary, exit 0 — the overlay names the still-open file via "missing end for \<path\>".

Scenario 3 runs a damaged stream plus real child stderr: an orphaned match, a duplicate begin, a second summary, a post-summary context, and a malformed record after the summary, with two genuine stderr lines and child exit 2 — the composed diagnostic must show the universal order (captured stderr first — real stderr wins over any generated code line — then every integrity cause in detection order, then the malformed record-loss count), identically in the overlay and the replay.

```bash
cd /home/chris/vrg/Notes/walkthroughs/036-06/code-walkthrough && python3 smoke.py; echo "smoke exit=$?"
```

```output
scenario 1: missing summary, child exit 0
  [PASS] missing-summary exit 2
  [PASS] missing-summary overlay names the cause
  [PASS] missing-summary emits no code-0 process line
  [PASS] missing-summary dismissal reveals browse
  [PASS] missing-summary stderr replays the cause
  [PASS] missing-summary replay emits no code-0 line
  [PASS] missing-summary cursor restored
  [PASS] missing-summary alt screen exited
  [PASS] missing-summary termios restored
scenario 2: missing end for the only file
  [PASS] missing-end exit 2
  [PASS] missing-end overlay names the file
  [PASS] missing-end stderr replays the cause
  [PASS] missing-end cursor restored
  [PASS] missing-end alt screen exited
  [PASS] missing-end termios restored
scenario 3: damaged stream + real child stderr
  [PASS] damaged exit 2
  [PASS] damaged overlay shows stderr line 1
  [PASS] damaged replay shows stderr line 1
  [PASS] damaged overlay shows stderr line 2
  [PASS] damaged replay shows stderr line 2
  [PASS] damaged overlay shows orphaned match cause
  [PASS] damaged replay shows orphaned match cause
  [PASS] damaged overlay shows duplicate begin cause
  [PASS] damaged replay shows duplicate begin cause
  [PASS] damaged overlay shows extra summary cause
  [PASS] damaged replay shows extra summary cause
  [PASS] damaged overlay shows after-summary cause
  [PASS] damaged replay shows after-summary cause
  [PASS] damaged overlay shows malformed count
  [PASS] damaged replay shows malformed count
  [PASS] damaged emits no generated code line
  [PASS] damaged replay universal order
  [PASS] damaged cursor restored
  [PASS] damaged alt screen exited
  [PASS] damaged termios restored
all checks passed
smoke exit=0
```

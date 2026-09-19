# Issue #44: post-summary context is an integrity failure — dedicated summary-is-final coverage

*2026-09-18T10:23:30Z by Showboat 0.6.1*
<!-- showboat-id: 9f92c04b-f9f3-4f20-8046-8a909bca10a3 -->

Issue #44 resolves the contradiction between Issue 9's former "context(P) in any position" exemption and the summary-is-final row in favor of the PRD's rule that *any* record after `summary` is a stream-integrity failure. Ownership is split at the parser boundary: Issue #36 removed the `context` exemption in `Index.Feed`'s post-summary branch and corrected the contradictory "context after summary has no lifecycle effect" lifecycle-matrix row; Issue #44 begins from that green boundary and adds the dedicated `context` regression coverage - no second parser change. See Notes/issues/036-stream-integrity-fatal-diagnostics.md, Notes/issues/044-post-summary-context-integrity-failure.md, Notes/tasks/044-post-summary-context-integrity-failure.md, and the 'Result index, records, and stream integrity' section of Notes/PRD-vrg.md. This walkthrough demonstrates the corrected lifecycle-matrix row and Issue #44's focused structured-cause and outcome assertions, then runs the issue's manual scenario through the Issue #4 fake-rg PTY harness. Artifacts (the built vrg binary and smoke.py) live in this directory.

## The lifecycle matrix — internal/searchindex

Issue #36's corrected row, "context after summary is an integrity failure", already asserts the post-summary context yields `CauseAfterSummary` beside retained results. Issue #44's dedicated row, "summary then context yields only the after-summary cause", asserts the minimal stream - `summary` then `context` - produces exactly the single structured `CauseAfterSummary` and nothing else. The neighbouring rows prove the exemption's surviving half: "context before begin is ignored" (a context whose path never opens contributes no cause and no file) and "context between end and summary is ignored" join the pre-existing "context before summary is ignored" - pre-summary context stays ignored for match/lifecycle semantics in every position.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestLifecycleMatrix' ./internal/searchindex 2>&1 | grep -E '^( *--- (PASS|FAIL|SKIP)|ok|FAIL)' | sed -E 's/\([0-9.]+s\)//g; s/\t[0-9.]+s$//; s/^ +//'; echo "searchindex exit=${PIPESTATUS[0]}"
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
--- PASS: TestLifecycleMatrix/context_before_begin_is_ignored 
--- PASS: TestLifecycleMatrix/context_between_end_and_summary_is_ignored 
--- PASS: TestLifecycleMatrix/context_after_summary_is_an_integrity_failure 
--- PASS: TestLifecycleMatrix/summary_then_context_yields_only_the_after-summary_cause 
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
ok  	vrg/internal/searchindex
searchindex exit=0
```

## Focused outcome assertions — internal/app

Issue #44's outcome coverage asserts the whole chain for the post-summary context stream: the `TestIntegrityDiagnostics` row "context after summary reports only its position" pins the complete composed diagnostic as exactly `record after summary` in both the overlay and the collected replay list; the new `TestOutcomeMatrix` row "context after summary is a fatal integrity failure" proves the fatal path - retained results browse under the error overlay and `q` exits 2; and the dedicated `TestContextAfterSummaryOutcome` asserts the fatal presentation, the exact diagnostic, and the stderr-replay retention in one place.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestIntegrityDiagnostics/context|TestOutcomeMatrix/context|^TestContextAfterSummaryOutcome$' ./internal/app 2>&1 | grep -E '^( *--- (PASS|FAIL|SKIP)|ok|FAIL)' | sed -E 's/\([0-9.]+s\)//g; s/\t[0-9.]+s$//; s/^ +//'; echo "app exit=${PIPESTATUS[0]}"
```

```output
--- PASS: TestOutcomeMatrix 
--- PASS: TestOutcomeMatrix/context_after_summary_is_a_fatal_integrity_failure 
--- PASS: TestIntegrityDiagnostics 
--- PASS: TestIntegrityDiagnostics/context_after_summary_reports_only_its_position 
--- PASS: TestIntegrityDiagnosticsDeterministic 
--- PASS: TestContextAfterSummaryOutcome 
ok  	vrg/internal/app
app exit=0
```

## Repository gates

The full verification gates over the completed coverage: go vet ./..., go build ./..., and go test ./... (the cmd/vrg PTY suite is the slow line).

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

## Manual scenario — the Issue #4 fake-rg PTY harness

smoke.py (in this directory) drives the freshly built ./vrg binary under a real PTY with a separated stderr pipe, using the Issue #4 fake-rg fixture family: a handshake side file proves the fixture ran, every key is sent only after the expected rendered marker is observed in the ANSI-stripped PTY stream, draining is completion/EOF-driven, and bounded polls re-check explicit conditions - there is no fixed settling delay anywhere (assert_no_fixed_delays parses the script's own AST to prove no time.sleep call exists). Terminal restoration is checked on every run (cursor, alt-screen, termios).

Scenario 1 runs the issue's manual check verbatim: a fake rg emits valid begin/match/end records for test.txt, then the summary, then a context record, exiting 0. Under the summary-is-final contract that context is a stream-integrity failure, so the fatal path applies: usable results exist, hence the error overlay over browse - the first q dismisses it, the second quits at the fixed status 2. The overlay must name the after-summary cause ("record after summary") - never a manufactured "ripgrep exited with code 0" line - and the same diagnostic replays to stderr. The record is not silently accepted.

Scenario 2 proves the surviving half of the amended exemption: context records in every pre-summary position - before begin, inside the file block, and between end and summary - keep the stream complete: no overlay, clean browse, exit 0, silent stderr.

```bash
cd /home/chris/vrg/Notes/walkthroughs/044-03/code-walkthrough && python3 smoke.py; echo "smoke exit=$?"
```

```output
scenario 1: valid records, summary, then context, exit 0
  [PASS] post-summary-context exit 2
  [PASS] post-summary-context overlay names the cause
  [PASS] post-summary-context emits no code-0 process line
  [PASS] post-summary-context dismissal reveals browse
  [PASS] post-summary-context stderr replays the cause
  [PASS] post-summary-context cursor restored
  [PASS] post-summary-context alt screen exited
  [PASS] post-summary-context termios restored
scenario 2: context records before summary, exit 0
  [PASS] pre-summary-context exit 0
  [PASS] pre-summary-context opens no overlay
  [PASS] pre-summary-context browses the file
  [PASS] pre-summary-context stderr stays silent
  [PASS] pre-summary-context cursor restored
  [PASS] pre-summary-context alt screen exited
  [PASS] pre-summary-context termios restored
all checks passed
smoke exit=0
```

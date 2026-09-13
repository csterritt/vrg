# Issue #9: Error overlay and fatal outcomes

*2026-09-13T00:00:00Z by Showboat 0.6.1*
<!-- showboat-id: a1b2c3d4-e5f6-4789-abcd-ef0123456789 -->

Walkthrough for Issue #9 (Notes/tasks/009-error-overlay-and-fatal-outcomes.md), implementing stream-integrity accounting, separate process-success and stream-integrity assessment, the fatal/warning outcome matrix, the modal error overlay, and exit status 2 for fatal process or stream-integrity outcomes. References: Notes/PRD-vrg.md (Result index contract, Outcome and exit-status contract, Colours, overlays, and key precedence).

Contracts verified:
- Stream integrity is assessed separately from process success.
- A complete stream requires a valid summary and valid paired begin/end metadata for encountered files.
- Orphaned or inconsistent lifecycle records, missing end events, missing summary, a second summary, any record after summary, or a trailing unterminated record make integrity fail.
- Context records do not participate in lifecycle validation.
- Binary exclusion takes precedence over generic orphaned-match retention.
- Fatal conditions include ripgrep exit code other than 0 or 1, signal termination, and stream-integrity failure.
- The fixed exit status is decided once at completion and never recomputed except by ctrl+c (which overrides to 130).
- The overlay is modal: up/down scroll, q/Esc dismiss (non-fatal) or exit 2 (fatal no-results), other keys ignored.
- Large diagnostics show both the head and tail of the captured stderr.
- When a failed process supplies no stderr, the overlay names the exit code or signal.

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

## SearchIndex lifecycle matrix tests

The SearchIndex tests (internal/searchindex/lifecycle_test.go, Issue #9) verify every row of the lifecycle transition matrix: duplicate begin, orphaned match, orphaned end, match after end, records after summary, second summary, interleaved open files with valid pairing, text/bytes path identity agreement, missing end, missing summary, trailing malformed/unterminated records, context records in arbitrary positions, and binary exclusion precedence.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/searchindex/ -run '^TestLifecycleMatrix' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
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
=== RUN   TestLifecycleMatrix/context_after_summary_has_no_lifecycle_effect
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
    --- PASS: TestLifecycleMatrix/context_after_summary_has_no_lifecycle_effect (0.00s)
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

## App outcome matrix and overlay tests

The App tests (internal/app/outcome_test.go and overlay_test.go, Issue #9) verify the pure outcome decision, the full Update flow with dismissal and exit assertions, the fixed-status ctrl+c override, the Esc-never-exits-base-state rule, the overlay key routing, scrolling, dismissal, long-line wrapping, sink safety, and diagnostic sanitization.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run '^TestDecideOutcome|^TestOutcomeMatrixFlow|^TestFixedStatus|^TestEscNever|^TestOverlay' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
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
=== RUN   TestFixedStatusCtrlCOverride
=== RUN   TestFixedStatusCtrlCOverride/ctrl+c_after_browse_completion
=== RUN   TestFixedStatusCtrlCOverride/ctrl+c_after_no-results_completion
=== RUN   TestFixedStatusCtrlCOverride/ctrl+c_with_open_overlay
--- PASS: TestFixedStatusCtrlCOverride (0.00s)
    --- PASS: TestFixedStatusCtrlCOverride/ctrl+c_after_browse_completion (0.00s)
    --- PASS: TestFixedStatusCtrlCOverride/ctrl+c_after_no-results_completion (0.00s)
    --- PASS: TestFixedStatusCtrlCOverride/ctrl+c_with_open_overlay (0.00s)
=== RUN   TestEscNeverExitsBaseState
--- PASS: TestEscNeverExitsBaseState (0.00s)
=== RUN   TestFixedStatusNotRecomputed
--- PASS: TestFixedStatusNotRecomputed (0.00s)
=== RUN   TestOverlayKeyDownScrolls
--- PASS: TestOverlayKeyDownScrolls (0.00s)
=== RUN   TestOverlayKeyUpScrolls
--- PASS: TestOverlayKeyUpScrolls (0.00s)
=== RUN   TestOverlayKeyQDismisses
--- PASS: TestOverlayKeyQDismisses (0.00s)
=== RUN   TestOverlayKeyEscDismisses
--- PASS: TestOverlayKeyEscDismisses (0.00s)
=== RUN   TestOverlayKeyCtrlCExits130
--- PASS: TestOverlayKeyCtrlCExits130 (0.00s)
=== RUN   TestOverlayKeyOtherIgnored
--- PASS: TestOverlayKeyOtherIgnored (0.00s)
=== RUN   TestOverlayFatalNoResultsQExits2
--- PASS: TestOverlayFatalNoResultsQExits2 (0.00s)
=== RUN   TestOverlayFatalNoResultsEscExits2
--- PASS: TestOverlayFatalNoResultsEscExits2 (0.00s)
=== RUN   TestOverlayFatalNoResultsCtrlCExits130
--- PASS: TestOverlayFatalNoResultsCtrlCExits130 (0.00s)
=== RUN   TestOverlayRendersDiagnostic
--- PASS: TestOverlayRendersDiagnostic (0.00s)
=== RUN   TestOverlayRendersBorder
--- PASS: TestOverlayRendersBorder (0.00s)
=== RUN   TestOverlayBaseColors
--- PASS: TestOverlayBaseColors (0.00s)
=== RUN   TestOverlayLongUnbrokenWraps
--- PASS: TestOverlayLongUnbrokenWraps (0.00s)
=== RUN   TestOverlaySinkSafetyNoStyle
=== RUN   TestOverlaySinkSafetyNoStyle/OSC
=== RUN   TestOverlaySinkSafetyNoStyle/CSI
=== RUN   TestOverlaySinkSafetyNoStyle/C0
=== RUN   TestOverlaySinkSafetyNoStyle/C1
=== RUN   TestOverlaySinkSafetyNoStyle/DEL
=== RUN   TestOverlaySinkSafetyNoStyle/StandaloneCR
=== RUN   TestOverlaySinkSafetyNoStyle/InvalidUTF8
=== RUN   TestOverlaySinkSafetyNoStyle/EmbeddedNewline
--- PASS: TestOverlaySinkSafetyNoStyle (0.00s)
    --- PASS: TestOverlaySinkSafetyNoStyle/OSC (0.00s)
    --- PASS: TestOverlaySinkSafetyNoStyle/CSI (0.00s)
    --- PASS: TestOverlaySinkSafetyNoStyle/C0 (0.00s)
    --- PASS: TestOverlaySinkSafetyNoStyle/C1 (0.00s)
    --- PASS: TestOverlaySinkSafetyNoStyle/DEL (0.00s)
    --- PASS: TestOverlaySinkSafetyNoStyle/StandaloneCR (0.00s)
    --- PASS: TestOverlaySinkSafetyNoStyle/InvalidUTF8 (0.00s)
    --- PASS: TestOverlaySinkSafetyNoStyle/EmbeddedNewline (0.00s)
=== RUN   TestOverlaySinkSafetyStyled
=== RUN   TestOverlaySinkSafetyStyled/OSC
=== RUN   TestOverlaySinkSafetyStyled/CSI
=== RUN   TestOverlaySinkSafetyStyled/C0
=== RUN   TestOverlaySinkSafetyStyled/C1
=== RUN   TestOverlaySinkSafetyStyled/DEL
=== RUN   TestOverlaySinkSafetyStyled/StandaloneCR
=== RUN   TestOverlaySinkSafetyStyled/InvalidUTF8
=== RUN   TestOverlaySinkSafetyStyled/EmbeddedNewline
--- PASS: TestOverlaySinkSafetyStyled (0.00s)
    --- PASS: TestOverlaySinkSafetyStyled/OSC (0.00s)
    --- PASS: TestOverlaySinkSafetyStyled/CSI (0.00s)
    --- PASS: TestOverlaySinkSafetyStyled/C0 (0.00s)
    --- PASS: TestOverlaySinkSafetyStyled/C1 (0.00s)
    --- PASS: TestOverlaySinkSafetyStyled/DEL (0.00s)
    --- PASS: TestOverlaySinkSafetyStyled/StandaloneCR (0.00s)
    --- PASS: TestOverlaySinkSafetyStyled/InvalidUTF8 (0.00s)
    --- PASS: TestOverlaySinkSafetyStyled/EmbeddedNewline (0.00s)
=== RUN   TestOverlayDiagnosticSanitized
--- PASS: TestOverlayDiagnosticSanitized (0.00s)
=== RUN   TestOverlayPreservesSafeWrapping
--- PASS: TestOverlayPreservesSafeWrapping (0.00s)
=== RUN   TestEscNeverQuitsWhenNoOverlay
--- PASS: TestEscNeverQuitsWhenNoOverlay (0.00s)
PASS
ok  	vrg/internal/app
```

## Process-boundary PTY tests

The cmd/vrg tests (cmd/vrg/outcome_test.go, Issue #9) extend the fake-rg/PTY harness with fatal exit, signal death, and large-stderr fixtures.

```bash
cd /home/chris/vrg && go test -count=1 -v ./cmd/vrg/ -run '^TestFatalExit|^TestSignalDeath|^TestStderrWarning|^TestStderrContent' -timeout 120s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestFatalExitWithResultsShowsOverlay
--- PASS: TestFatalExitWithResultsShowsOverlay (0.00s)
=== RUN   TestFatalExitNoOutputNamesExitCode
--- PASS: TestFatalExitNoOutputNamesExitCode (0.00s)
=== RUN   TestFatalExitNoOutputEscExits2
--- PASS: TestFatalExitNoOutputEscExits2 (0.00s)
=== RUN   TestSignalDeathNamesSignal
--- PASS: TestSignalDeathNamesSignal (0.00s)
=== RUN   TestStderrWarningWithSummaryShowsWarningOverlay
--- PASS: TestStderrWarningWithSummaryShowsWarningOverlay (0.00s)
=== RUN   TestStderrContentFixture
--- PASS: TestStderrContentFixture (0.00s)
PASS
ok  	vrg/cmd/vrg
```

## Binary demo: fatal exit with results (browse + error overlay)

Using the built vrg binary with a fake rg (fakebin-fatal-results/rg) that emits a valid stream with one match, writes "boom" to stderr, then exits 3. The PTY helper sends Esc to dismiss the overlay, then q to quit. The stripped output shows the error overlay containing "boom", then the browse view with test.txt. The exit code is 2.

```bash
cd /home/chris/vrg/Notes/walkthroughs/009-06/code-walkthrough && PATH=$(pwd)/fakebin-fatal-results:/usr/bin:/bin VRG_KEYS=$'\x1b,q' VRG_DELAY=1.0 timeout 10 python3 runpty.py ./vrg hello demo 2>&1; echo exit=$?
```

```output
┌──────┐
│ boom │
│      │
└──────┘
test.txt                  ── test.txt ── Loading…
exit=2
```

## Binary demo: fatal exit with no output (fatal overlay names exit code)

Using the built vrg binary with a fake rg (fakebin-fatal-empty/rg) that exits 2 with no output. The PTY helper sends q. The stripped output shows the fatal error overlay naming exit code 2. The exit code is 2.

```bash
cd /home/chris/vrg/Notes/walkthroughs/009-06/code-walkthrough && PATH=$(pwd)/fakebin-fatal-empty:/usr/bin:/bin VRG_KEYS=q VRG_DELAY=1.0 timeout 10 python3 runpty.py ./vrg hello demo 2>&1; echo exit=$?
```

```output
┌────────────────────────────┐
│ ripgrep exited with code 2 │
└────────────────────────────┘
exit=2
```

## Binary demo: fatal exit with no output (Esc also exits 2)

The same fatal no-results overlay, dismissed with Esc instead of q. Esc exits 2 because there is no underlying state.

```bash
cd /home/chris/vrg/Notes/walkthroughs/009-06/code-walkthrough && PATH=$(pwd)/fakebin-fatal-empty:/usr/bin:/bin VRG_KEYS=$'\x1b' VRG_DELAY=1.0 timeout 10 python3 runpty.py ./vrg hello demo 2>&1; echo exit=$?
```

```output
┌────────────────────────────┐
│ ripgrep exited with code 2 │
└────────────────────────────┘
exit=2
```

## Binary demo: stderr warning with zero results (warning overlay → no-results)

Using the built vrg binary with a fake rg (fakebin-warn-empty/rg) that writes "warn" to stderr and a summary-only stream, then exits 1. The PTY helper sends Esc to dismiss the warning overlay, then q to quit. The stripped output shows the warning overlay containing "warn", then the no-results screen. The exit code is 1.

```bash
cd /home/chris/vrg/Notes/walkthroughs/009-06/code-walkthrough && PATH=$(pwd)/fakebin-warn-empty:/usr/bin:/bin VRG_KEYS=$'\x1b,q' VRG_DELAY=1.0 timeout 10 python3 runpty.py ./vrg hello demo 2>&1; echo exit=$?
```

```output
┌──────┐
│ warn │
│      │
└──────┘


No results found
exit=1
```

## Summary

The walkthrough demonstrates:
- SearchIndex lifecycle matrix tests: every lifecycle transition row, including duplicate begin, orphaned match/end, match after end, records after summary, second summary, interleaved open files, text/bytes path identity, missing end/summary, trailing malformed records, context records, and binary exclusion precedence.
- App outcome matrix tests: the pure DecideOutcome function covering every row of the outcome table, the full Update flow with dismissal and exit assertions, the fixed-status ctrl+c override, the Esc-never-exits-base-state rule, and the fixed-status-not-recomputed rule.
- App overlay tests: key routing (up/down scroll, q/Esc dismiss, ctrl+c exit 130, other keys ignored), fatal no-results overlay (q/Esc exit 2, ctrl+c exit 130), diagnostic rendering, border, base colours, long-line wrapping, sink safety (no-style and styled), diagnostic sanitization, and safe wrapping after resize.
- Process-boundary PTY tests: fatal exit with results (browse + error overlay), fatal exit with no output (overlay names exit code), Esc on fatal no-results overlay (exits 2), signal death (overlay names signal), stderr warning with summary (warning overlay → no-results), and large stderr content (head and tail shown).
- The binary with a fake rg exiting 3 with stderr (fatal with results): error overlay containing "boom", Esc dismisses to browse, q exits 2.
- The binary with a fake rg exiting 2 with no output (fatal no results): fatal overlay naming exit code 2, q exits 2.
- The binary with a fake rg exiting 2 with no output, dismissed with Esc: Esc exits 2 (no underlying state).
- The binary with a fake rg writing "warn" to stderr and a summary-only stream (warning with zero results): warning overlay, Esc dismisses to no-results, q exits 1.

References: Issue #9 (Notes/tasks/009-error-overlay-and-fatal-outcomes.md), Notes/PRD-vrg.md (Result index contract, Outcome and exit-status contract, Colours, overlays, and key precedence).

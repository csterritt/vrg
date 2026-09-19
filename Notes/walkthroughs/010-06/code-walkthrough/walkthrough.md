# Issue #10: record robustness — malformed, oversized, unknown types

*2026-09-16T23:18:17Z by Showboat 0.6.1*
<!-- showboat-id: dd3c613a-75eb-4d96-91b8-7934b5d2be16 -->

Issue #10 makes SearchIndex resilient to damaged ripgrep streams and completes the outcome table: every record lands in a deterministic disposition — skipped-and-counted malformed, stream-integrity failure, the separate unknown-type count, or one of the two composite rows — records over the 64 MiB payload limit are consumed and discarded through their next newline with best-effort path recovery for the diagnostic, and record loss on an otherwise-complete stream with zero usable results is fatal. See `Notes/issues/010-record-robustness-malformed-oversized-unknown.md`, `Notes/tasks/010-record-robustness-malformed-oversized-unknown.md`, and the PRD sections "Result index, records, and stream integrity", "Outcome and exit-status contract", and "Resources and responsiveness" in `Notes/PRD-vrg.md`. All artifacts live in this directory.

## Gates — module integrity, build, vet

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

## SearchIndex — the disposition matrices

`internal/searchindex/disposition_test.go` is the deterministic disposition matrix: `TestSchemaMatrixDispositions` embeds one candidate per Issue #3 schema-matrix row between two valid matches — every missing/wrongly-typed required field, every invalid range, invalid JSON/base64, and missing or non-string `type` is counted malformed while the surrounding stops still index (resynchronization), the `data`-less context row stays valid, and unrecognized string types count unknown rather than malformed. `TestLifecycleMatrixDispositions` covers the Issue #9 integrity rows — none inflates the malformed count, and the unknown-type-after-`summary` row counts unknown while its position fails integrity. The two dedicated tests pin the composite rows: the non-oversized unterminated tail and the malformed-after-`summary` record are each counted malformed AND marked incomplete.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'Dispositions|MalformedAndIncomplete|UnterminatedTail' ./internal/searchindex 2>&1 | grep -E '^(--- |ok|FAIL)' | grep -vE '^--- .*/' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestSchemaMatrixDispositions
--- PASS: TestLifecycleMatrixDispositions
--- PASS: TestUnterminatedTailIsMalformedAndIncomplete
--- PASS: TestMalformedAfterSummaryIsMalformedAndIncomplete
--- PASS: TestOversizedUnterminatedTailTripleDisposition
ok  	vrg/internal/searchindex
```

## SearchIndex — the 64 MiB boundary, resynchronization, and diagnostics

`internal/searchindex/oversized_test.go` pins the payload limit: a record of exactly `MaxRecordBytes` (64 MiB, excluding its newline) is still parsed, and one byte over is consumed and discarded through its next newline — the orphaned `end` that follows proves parsing resynchronized. `TestOversizedRecordPathDiagnostics` shows both diagnostic forms: a `match` whose `type` and `data.path` parsed before the limit lands in `OversizedPaths` (reported as "oversized record skipped for \<path\>") while the oversized-only file is absent from the file list, and a record hitting the limit before `data.path` is counted anonymously. The unterminated oversized tail takes all three dispositions — oversized count, malformed count, incomplete stream — and `TestUnknownTypeCannotSubstituteForSummary` proves an unknown type cannot stand in for the required completion events.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'Oversized|UnknownType' ./internal/searchindex 2>&1 | grep -E '^(--- |ok|FAIL)' | grep -vE '^--- .*/' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestOversizedBoundary
--- PASS: TestOversizedRecordPathDiagnostics
--- PASS: TestOversizedUnterminatedTailTripleDisposition
--- PASS: TestUnknownTypeCannotSubstituteForSummary
ok  	vrg/internal/searchindex
```

## App — the extended outcome matrix

`internal/app/outcome_test.go`'s single table-driven `TestOutcomeMatrix` gains the Issue #10 rows: unknown-type warnings with zero results → warning overlay → no-results → 1; malformed skipped with usable results → browse + overlay → 0; malformed skipped with zero usable results → record-loss overlay → `q` 2 and `Esc` 2; a skipped record plus binary exclusion leaving zero retained stops → the record-loss fatal row 2 (usable results assessed after all filtering); and missing `end` with retained matches → browse + overlay → 2 versus no matches → overlay-only → 2.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'OutcomeMatrix' ./internal/app 2>&1 | grep -E '^(--- |    --- (PASS|FAIL)|ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
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
    --- PASS: TestOutcomeMatrix/missing_end_with_retained_matches_browses_under_overlay
    --- PASS: TestOutcomeMatrix/missing_end_with_no_matches_is_fatal
    --- PASS: TestOutcomeMatrix/ctrl+c_after_completion_in_browse_exits_130
    --- PASS: TestOutcomeMatrix/ctrl+c_after_completion_in_no_results_exits_130
    --- PASS: TestOutcomeMatrix/ctrl+c_in_an_open_overlay_exits_130
ok  	vrg/internal/app
```

## Manual PTY — the issue's fake-rg stream end to end

`pty_robustness.py` runs the real binary on a pty against a scripted fake rg emitting exactly the issue's manual stream — a valid `begin` for `./a.txt`, a valid `match`, a garbage line, a `{"type":"weird"}` line, a valid `end` (null `binary_offset`), a valid `summary`, exit 0 — and asserts on the raw byte stream: the browse view opens under the overlay listing "1 malformed record skipped" and "1 unrecognised record types skipped", `Esc` dismisses to the repainted browse content, and `q` exits 0. A second session repeats the stream without the `begin`/`end` pairing: the orphaned match is retained but integrity fails, so `q` exits 2.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/010-06/code-walkthrough/vrg ./cmd/vrg && cd Notes/walkthroughs/010-06/code-walkthrough && python3 pty_robustness.py
```

```output
robust       : garbage + weird-type lines between valid records -> browse + overlay listing both skip counts; Esc dismisses; q exits 0
no-pair      : same stream without begin/end -> orphaned match retained, integrity fails; q exits 2
OK
```

## Full suite — `go test ./...` and the race detector

```bash
cd /home/chris/vrg && go test -count=1 ./... 2>&1 | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//' && CGO_ENABLED=1 go test -count=1 -race ./internal/... 2>&1 | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
?   	vrg/Notes/walkthroughs/007-04/code-walkthrough/fixture	[no test files]
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
?   	vrg/internal/safepresentation/sinktest	[no test files]
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
?   	vrg/internal/viewport	[no test files]
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
?   	vrg/internal/safepresentation/sinktest	[no test files]
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
?   	vrg/internal/viewport	[no test files]
```

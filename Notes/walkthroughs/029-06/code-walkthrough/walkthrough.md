# Issue #29: stale-match validation and the file-changed note

*2026-09-18T00:42:50Z by Showboat 0.6.1*
<!-- showboat-id: 0a581107-b193-42aa-935c-6a6c62dc567a -->

Issue #29 makes every retained navigation stop survive a file that changed since the search ran — best-effort, byte-exact validation. At load or reload time each recorded submatch's search-coordinate range is checked against the loaded line's search bytes: a submatch whose bounds fall outside the line, or whose bytes no longer equal the captured ones, is dropped — its highlight goes with it — and the buffer marks itself stale. Validation is deliberately undetectable when content shifts are range-preserving byte-identical (the bytes match, so nothing drops); that is the contract, not a bug. A surviving submatch keeps its highlight and supplies the reveal cell. When every submatch on an existing line drops, the stop's first recorded start clamps into the line and maps to a valid cell — a start past the end lands on the line's last cell — with no invented highlight. A recorded line that no longer exists lands at the last source line's start, cell 0; an empty file keeps the recorded line with cell 0. Stale state is recomputed on every load, and the persistent filename-row note 'file changed since search' rides the Issue #24 status slot — no timer, no expiry — clearing only when a reload validates fully. Reveal decisions for stale stops commit through Issue #28's two-stage load/layout path against the matching installed layout, and the search-derived exit status never changes for a stale buffer. See Notes/issues/029-stale-match-validation-and-file-changed-note.md, Notes/tasks/029-stale-match-validation-and-file-changed-note.md, and the PRD section 'Encodings and stale-content validation' in Notes/PRD-vrg.md. All artifacts live in this directory.


## Gates — module integrity, build, vet

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```


## FileBuffer validation and fallback targets

internal/filebuffer/stale_test.go pins the coordinate contract: validation compares each recorded submatch's search-coordinate range against the line's SearchBytes — the ripgrep view that starts after a leading UTF-8 BOM — never the escaped display text. TestCleanContentNotStale / TestStaleRecomputedPerLoad: identical content validates clean, and the stale flag is recomputed per load through mismatched → clean → mismatched-again decodes. TestStaleOutOfBoundsRangeDrops / TestStaleSameLengthReplacementDrops: out-of-bounds ranges and byte-mismatching replacements drop. TestStalePartialSurvivalKeepsValidHighlight: a dropped "alpha" loses its highlight while the surviving one keeps it and supplies the reveal cell. TestStaleAllDroppedClampedStart / TestStaleClampedStartEOLFallbackLastCell: an all-dropped line clamps the first recorded start into the line — a start past the end lands on the last cell — with no invented highlight. TestStaleMissingLineLandsOnLastLine / TestStaleEmptyFileZeroLines: a deleted line lands at the last source line's start, and an empty file keeps the recorded line at cell 0. TestStaleValidationComparesSearchBytesNotDisplay / TestStaleCRLFTerminatorMatchValidates / TestStaleBOMAdjustedValidation / TestStaleZeroWidthSurvives: escaped text, CRLF terminators, the BOM adjustment, and zero-width submatches all validate against the search view.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestCleanContentNotStale|TestStaleOutOfBoundsRangeDrops|TestStaleSameLengthReplacementDrops|TestStalePartialSurvivalKeepsValidHighlight|TestStaleAllDroppedClampedStart|TestStaleClampedStartEOLFallbackLastCell|TestStaleMissingLineLandsOnLastLine|TestStaleEmptyFileZeroLines|TestStaleRecomputedPerLoad|TestStaleValidationComparesSearchBytesNotDisplay|TestStaleCRLFTerminatorMatchValidates|TestStaleBOMAdjustedValidation|TestStaleZeroWidthSurvives' ./internal/filebuffer 2>&1 | grep -E '^(--- |ok|FAIL|    ---)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestCleanContentNotStale
--- PASS: TestStaleOutOfBoundsRangeDrops
--- PASS: TestStaleSameLengthReplacementDrops
--- PASS: TestStalePartialSurvivalKeepsValidHighlight
--- PASS: TestStaleAllDroppedClampedStart
--- PASS: TestStaleClampedStartEOLFallbackLastCell
--- PASS: TestStaleMissingLineLandsOnLastLine
--- PASS: TestStaleEmptyFileZeroLines
--- PASS: TestStaleRecomputedPerLoad
--- PASS: TestStaleValidationComparesSearchBytesNotDisplay
--- PASS: TestStaleCRLFTerminatorMatchValidates
--- PASS: TestStaleBOMAdjustedValidation
--- PASS: TestStaleZeroWidthSurvives
ok  	vrg/internal/filebuffer
```


## App — the note, two-stage stale reveals, and the fixed status

internal/app/stale_test.go pins the integration. TestStaleNoteInFilenameRow: a same-length replacement loaded on entry carries 'file changed since search' in the filename row's status slot beside the still-named path — within the frame's width — and the note survives scrolling and a 44-column resize with no timer. TestStaleNoteClearedByCleanReload: reverting the file and reloading clears the note; a still-mismatched reload restores it — stale state is recomputed per load. TestStaleGatedReloadCommitRevealsSurvivor: navigation during a held reload selects a stop whose first recorded submatch dropped; the matching prepared layout commits the reveal at the surviving submatch's cell, never the dropped one's. TestStaleGatedReloadCommitRevealsClampedFallback: an all-dropped line commits the clamped-start fallback with no invented highlight. TestStaleMissingLineLandsOnLastLine: a deleted recorded line commits at the last source line's start. The outcome-matrix row 'all stops stale with fixed status 0 still exits 0' proves every retained stop validating stale leaves the search-derived exit status untouched.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestStaleNoteInFilenameRow|TestStaleNoteClearedByCleanReload|TestStaleGatedReloadCommitRevealsSurvivor|TestStaleGatedReloadCommitRevealsClampedFallback|TestStaleMissingLineLandsOnLastLine|TestOutcomeMatrix' ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL|    ---)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
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
    --- PASS: TestOutcomeMatrix/all_loads_fail_with_fixed_status_0_still_exits_0
    --- PASS: TestOutcomeMatrix/current-file_failure_with_fixed_status_2_still_exits_2
    --- PASS: TestOutcomeMatrix/all_loads_fail_with_fixed_status_2_still_exits_2
    --- PASS: TestOutcomeMatrix/all_stops_stale_with_fixed_status_0_still_exits_0
--- PASS: TestStaleNoteInFilenameRow
--- PASS: TestStaleNoteClearedByCleanReload
--- PASS: TestStaleGatedReloadCommitRevealsSurvivor
--- PASS: TestStaleGatedReloadCommitRevealsClampedFallback
--- PASS: TestStaleMissingLineLandsOnLastLine
ok  	vrg/internal/app
```


## Full module regression

The change touches Decode's per-line validation, the Line target state, Buffer.StopTarget, viewport.Rows delegating to the loaded buffer, and fileLoadedMsg's note wiring — the whole suite runs, plus the race detector over the gated worker tests.

```bash
cd /home/chris/vrg && go test -count=1 ./internal/filebuffer ./internal/viewport ./internal/app ./internal/safepresentation ./internal/searchindex ./internal/theme ./internal/cli ./cmd/vrg 2>&1 | sed -E 's/\t[0-9.]+s$//' && CGO_ENABLED=1 go test -race -count=1 ./internal/app 2>&1 | sed -E 's/\t[0-9.]+s$//'
```

```output
ok  	vrg/internal/filebuffer
ok  	vrg/internal/viewport
ok  	vrg/internal/app
ok  	vrg/internal/safepresentation
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/cli
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
```


## Manual check — the real binary on real ptys

manual_route.sh runs the issue's manual checks as the unprivileged user against disposable mktemp fixtures — never a repository file: it generates three single-file case directories under a trap removing the tree on exit or interruption, and pty_stale.py drives the sessions on real 80x24 ptys with stderr redirected to a file. The driver rewrites the fixture on disk between keypresses and sends the explicit Issue #27 reload key r — the file changing underneath a running browse is exactly the stale scenario. c1: 'needle one 10' highlighted on entry; the file's 'needle' becomes 'sizzle' on disk — same length — and r shows 'file changed since search' in the filename row with the changed word carrying no invented highlight; scrolling leaves the note in place, no timer. c2: 'needle two 05' and 'needle two 60' match a 60-line file; lines 41–60 are deleted and r shows the note — then n selects the stop whose line is gone and lands at the last source line 'pad two 40', the one-third placement EOF-clamped to the bottom row, with no invented highlight. c3: 'needle' → 'sizzle' + r shows the note; restoring the original bytes + r clears it and the highlight returns — stale state is recomputed per load. Every session exits 0 — the search-derived status — and the gated model tests above remain the authoritative deterministic verification.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/029-06/code-walkthrough/vrg ./cmd/vrg && bash Notes/walkthroughs/029-06/code-walkthrough/manual_route.sh
```

```output
fixture: <tmp> (c1/c2/c3 — one file each)
c1 startup  : 'needle one 10' highlighted in place
c1 edit r   : same-length 'sizzle' — 'file changed since search', no highlight
c1 d        : scrolled — the note persists, no timer
c1 q        : exit 0
c2 edit r   : trailing matched lines deleted + r — the note shows
c2 n        : n — lands at 'pad two 40', no invented highlight
c2 q        : exit 0
c3 edit r   : 'sizzle' on disk + r — the note shows
c3 revert r : reverted + r — the note clears, the highlight returns
c3 q        : exit 0
OK
```

# Issue #30: unsupported encodings — the UTF-16/UTF-32 BOM placeholder

*2026-09-18T01:04:02Z by Showboat 0.6.1*
<!-- showboat-id: 57f10fe2-bfe7-44b9-94dd-613f9ad4d11d -->

Issue #30 makes FileBuffer the unsupported-encoding oracle: a file opening with a UTF-16 or UTF-32 byte-order mark decodes to a line-free buffer reporting its encoding, so the panel shows '(unsupported encoding)' — no file text, no highlights — instead of misrepresenting ripgrep's transcoded match offsets as raw-file highlights. detectEncoding checks the longer four-byte UTF-32 BOMs before the overlapping two-byte UTF-16 ones, so FF FE 00 00 classifies as UTF-32 LE and is never swallowed by UTF-16 LE's FF FE; a leading UTF-8 BOM stays a supported signature and is never misclassified. Detection happens before line splitting and before Issue #29's per-submatch validation, so the raw encoded bytes never enter the stale-match guard. The app presents the placeholder through a placeholder(path) selector — 'Loading…' in flight, '(unreadable)' on failure, '(unsupported encoding)' on detection — collects 'cannot display <path>: unsupported encoding <name>' once per detection, and opens the overlay only when the detected file is current (Issue #26's current/non-current distinction). Unsupported files keep their indexed cursor stops and reload through r like any other file, and the search-derived exit status never changes — rg's default BOM detection also stays enabled, so the child argv carries no forced encoding flag and these files still yield matches. See Notes/issues/030-unsupported-encodings-utf16-utf32.md, Notes/tasks/030-unsupported-encodings-utf16-utf32.md, and the PRD sections 'Encodings and stale-content validation' and 'Invocation and child arguments' in Notes/PRD-vrg.md. All artifacts live in this directory.

## Gates — module integrity, build, vet

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

## FileBuffer BOM detection — four signatures, the overlap ordering, and the exclusions

internal/filebuffer/encoding_test.go pins the detection contract. TestUnsupportedBOMsClassified: each of the four byte-order marks reports its Unsupported() name — UTF-16 LE, UTF-16 BE, UTF-32 LE, UTF-32 BE — with no lines, no highlights, and no stale state; a file that is nothing but the two-byte signature still classifies, and a NUL third byte does not promote UTF-16 LE to UTF-32 LE. TestUTF32LECheckedBeforeUTF16LE is the overlap case: FF FE 00 00 opens with UTF-16 LE's own two-byte mark, so the longer BOM is checked first and wins. TestSupportedSignaturesNotUnsupported: the leading UTF-8 BOM decodes normally with its mark invisible, and plain text, the empty file, lone FF/FE bytes, a truncated UTF-32 mark, and signature bytes away from the start are never misclassified. TestUnsupportedSkipsStaleValidation: a recorded transcoded submatch that could never byte-equal a raw range leaves the buffer clean — Issue #29's validator never ran.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestUnsupportedBOMsClassified|TestUTF32LECheckedBeforeUTF16LE|TestSupportedSignaturesNotUnsupported|TestUnsupportedSkipsStaleValidation' ./internal/filebuffer 2>&1 | grep -E '^(--- |ok|FAIL|    ---)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestUnsupportedBOMsClassified
    --- PASS: TestUnsupportedBOMsClassified/UTF-16_LE
    --- PASS: TestUnsupportedBOMsClassified/UTF-16_BE
    --- PASS: TestUnsupportedBOMsClassified/UTF-32_LE
    --- PASS: TestUnsupportedBOMsClassified/UTF-32_BE
    --- PASS: TestUnsupportedBOMsClassified/UTF-16_LE_signature_only
    --- PASS: TestUnsupportedBOMsClassified/UTF-16_LE_with_a_NUL_third_byte
--- PASS: TestUTF32LECheckedBeforeUTF16LE
--- PASS: TestSupportedSignaturesNotUnsupported
    --- PASS: TestSupportedSignaturesNotUnsupported/UTF-8_BOM
    --- PASS: TestSupportedSignaturesNotUnsupported/plain_text
    --- PASS: TestSupportedSignaturesNotUnsupported/empty_file
    --- PASS: TestSupportedSignaturesNotUnsupported/lone_FF
    --- PASS: TestSupportedSignaturesNotUnsupported/lone_FE
    --- PASS: TestSupportedSignaturesNotUnsupported/truncated_UTF-32_BE
    --- PASS: TestSupportedSignaturesNotUnsupported/FF_FE_away_from_the_start
    --- PASS: TestSupportedSignaturesNotUnsupported/UTF-8_BOM_on_a_later_line
--- PASS: TestUnsupportedSkipsStaleValidation
ok  	vrg/internal/filebuffer
```

## App — notification, placeholder, reload, no stale note, and the fixed status

internal/app/encoding_test.go pins the integration over a fixture pairing each file's raw BOM-marked bytes with the transcoded rg line a real search records. TestUnsupportedCurrentShowsOverlayAndPlaceholder: crossing into the UTF-16 LE file opens the encoding diagnostic's overlay, shows '(unsupported encoding)' with no file text and no highlights while the filename row keeps naming the path, and the file stays a cursor stop n wraps out of. TestUnsupportedNonCurrentDiagnosticOnly: a detection completing for a non-current file collects the diagnostic with no overlay and no frame change; visiting it later shows the cached placeholder and collects nothing new. TestUnsupportedReloadPreservesPlaceholder: r mints a reload that reads 'Loading…' in flight, then an unchanged file settles back to the placeholder with a fresh overlay and a second collected diagnostic — detection reports once per load. TestUnsupportedRunsNoStaleValidation: the transcoded recorded submatch never marks the buffer stale and the filename row carries no 'file changed since search' note. TestUnsupportedComposedViewAtConstrainedWidths: a long escaped path's unsupported state at 80/30/20 columns keeps the truncated path identified with nothing overflowing and nonnegative dimensions. The outcome-matrix row 'all files unsupported with fixed status 0 still exits 0' proves the search-derived exit status survives every retained file decoding unsupported.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestUnsupportedCurrentShowsOverlayAndPlaceholder|TestUnsupportedNonCurrentDiagnosticOnly|TestUnsupportedReloadPreservesPlaceholder|TestUnsupportedRunsNoStaleValidation|TestUnsupportedComposedViewAtConstrainedWidths|TestOutcomeMatrix' ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL|    ---)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestUnsupportedCurrentShowsOverlayAndPlaceholder
--- PASS: TestUnsupportedNonCurrentDiagnosticOnly
--- PASS: TestUnsupportedReloadPreservesPlaceholder
--- PASS: TestUnsupportedRunsNoStaleValidation
--- PASS: TestUnsupportedComposedViewAtConstrainedWidths
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
    --- PASS: TestOutcomeMatrix/all_files_unsupported_with_fixed_status_0_still_exits_0
ok  	vrg/internal/app
```

## Full module regression

The change touches Decode's early encoding return, the Buffer.Unsupported() surface, fileLoadedMsg's detection branch, the placeholder selector, and the requestLayout/currentRows guards — the whole suite runs, plus the race detector over the packages that own the gated worker tests.

```bash
cd /home/chris/vrg && go test -count=1 ./internal/filebuffer ./internal/viewport ./internal/app ./internal/safepresentation ./internal/searchindex ./internal/theme ./internal/cli ./cmd/vrg 2>&1 | sed -E 's/\t[0-9.]+s$//' && CGO_ENABLED=1 go test -race -count=1 ./internal/app ./internal/filebuffer 2>&1 | sed -E 's/\t[0-9.]+s$//'
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
ok  	vrg/internal/filebuffer
```

## Manual check — the real binary on a real pty

manual_route.sh runs the issue's manual route as the unprivileged user against a disposable mktemp fixture — never a repository file: it writes u16.txt with the issue's own recipe, printf '\xff\xfeh\0i\0\n\0' (UTF-16 LE "hi\n"), under a trap removing the tree on exit or interruption, and pty_unsupported.py drives the session on a real 80x24 pty with stderr redirected to a file. rg's default BOM detection transcodes the file, so 'hi' still matches and u16.txt is a retained cursor stop. Startup opens on that file: the panel shows '(unsupported encoding)' — no file text, no highlights — and the current-file detection opens the overlay with 'cannot display <path>: unsupported encoding UTF-16 LE'. Esc dismisses it; the placeholder stays. r under the held load gate reads 'Loading…' mid-flight; releasing it lets the unchanged file settle back to the placeholder with a fresh overlay — detection is reported once per load. Esc dismisses again and q exits 0 — the fixed search-derived status — while stderr replays exactly one encoding diagnostic per load: the startup detection and the reload each collected one.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/030-04/code-walkthrough/vrg ./cmd/vrg && bash Notes/walkthroughs/030-04/code-walkthrough/manual_route.sh
```

```output
fixture: <tmp> (u16.txt — UTF-16 LE "hi\n")
startup     : '(unsupported encoding)' + the UTF-16 LE overlay
Esc         : overlay dismissed — the placeholder stays
r (gated)   : held reload reads 'Loading…'
release     : unchanged — placeholder + a new overlay
Esc         : second overlay dismissed
q           : exit 0
stderr      : stderr replays both encoding diagnostics
OK
```

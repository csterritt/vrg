# Issue #26: Read failures — "(unreadable)", notification split, and the gated re-entry retry

*2026-09-24T16:18:36Z by Showboat 0.6.1*
<!-- showboat-id: e055e4d4-fe82-4b2e-a9d2-f7093d33feeb -->

Walkthrough for [Issue #26](../../../issues/026-read-failures-unreadable-retry-rules.md), implementing the read-failure contract per `Notes/PRD-vrg.md` (*File loading, cache, reload, and selection consistency*; *Outcome and exit-status contract*): a read failure on the current file opens the Issue #9 error overlay and shows `(unreadable)` while its cursor stops stay navigable; a non-current failure is diagnostic-only (no overlay, no indicator — discoverable by visiting the file or at the Issue #11 exit replay); same-file `n`/`p` steps never reload, while entering a previously failed file from a different file runs a deterministic sequence — the prior failure's overlay opens immediately, exactly one retry issues behind `Loading…`, settlement updates the panel without waiting for dismissal, a second failure appends one scroll-preserving occurrence, and an in-flight retry settling after navigation away updates only that path; and a load failure can never move the fixed search-derived exit status. All generated artifacts live in this directory: the built `vrg` binary, the `fixture-src/` pair, the `demo-failures.sh` tmux harness, and its `fail/` session captures. Test durations are stripped so the document verifies cleanly.

```bash
set -o pipefail; cd /home/chris/vrg && test -z "$(gofmt -l internal/ cmd/)" && echo GOFMT-CLEAN && go vet ./... && go build ./... && go test -count=1 ./... | sed "s/[[:space:]][0-9.]*s$//" && CGO_ENABLED=1 go test -race -count=1 ./internal/app | sed "s/[[:space:]][0-9.]*s$//" && go build -o Notes/walkthroughs/026-06/code-walkthrough/vrg ./cmd/vrg && echo GATES-OK
```

```output
GOFMT-CLEAN
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/present
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
ok  	vrg/internal/app
GATES-OK
```

## Injected-loader model tests — `internal/app/failures_test.go`

The contracts are pinned at the model seam: `Model.readFile` is the read-phase seam (nil selects `filebuffer.ReadFile`), so tests substitute deterministic failing loaders — `failLoader` fails every path, `failLoaderFor` fails one — never relying on filesystem permissions, and `gatedLoaderModel` pairs the loader with `loadGate` so a retry can be parked in flight. `TestCurrentFileReadFailureNotifies` covers the current-file overlay, the `(unreadable)` placeholder, the filename row still naming the path, and the retained stops (a same-file `n` moves the cursor, no reload). `TestNonCurrentReadFailureIsDiagnosticOnly` lands a completion after the cursor left — byte-identical frame, one collected occurrence — then proves visiting the file surfaces its overlay. `TestCrossFileReEntryRetriesOnce` pins the re-entry: prior overlay immediately, `Loading…`, exactly one retry. `TestUnreadableComposedViewStaysWellFormed` drives a long escaped path through 80×24 down to 20×3 — truncated safe path in the Issue #24 slot, clipped placeholder, no overflow. The gated sequence tests cover `Esc` mid-retry, second-failure append preserving the reader's scroll, a successful retry collecting nothing while the prior overlay stays up, navigation-away settlement, and the in-flight drop per Issue #25. `TestOutcomeMatrix` gained the `failAll`/`absent`/`viewHas`/`replayHas` row fields and three rows proving load failures never recompute the fixed status: all-fail under fixed 0, current-fail under fixed 2, and the composed all-fail-with-fixed-2 case.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestCurrentFileReadFailureNotifies|TestNonCurrentReadFailureIsDiagnosticOnly|TestCrossFileReEntryRetriesOnce|TestUnreadableComposedViewStaysWellFormed|TestReEntrySequenceGated|TestReEntrySecondFailureAppendsPreservingScroll|TestReEntryRetrySuccessKeepsPriorOverlay|TestReEntryRetrySettlesAfterNavigatingAway|TestReEntryDuringInflightRetryIsDropped|TestOutcomeMatrix' ./internal/app 2>&1 | grep -vE '^(=== RUN|=== CONT)' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestCurrentFileReadFailureNotifies
--- PASS: TestNonCurrentReadFailureIsDiagnosticOnly
--- PASS: TestCrossFileReEntryRetriesOnce
--- PASS: TestUnreadableComposedViewStaysWellFormed
    --- PASS: TestUnreadableComposedViewStaysWellFormed/80x24
    --- PASS: TestUnreadableComposedViewStaysWellFormed/40x10
    --- PASS: TestUnreadableComposedViewStaysWellFormed/26x6
    --- PASS: TestUnreadableComposedViewStaysWellFormed/20x3
--- PASS: TestReEntrySequenceGated
--- PASS: TestReEntrySecondFailureAppendsPreservingScroll
--- PASS: TestReEntryRetrySuccessKeepsPriorOverlay
--- PASS: TestReEntryRetrySettlesAfterNavigatingAway
--- PASS: TestReEntryDuringInflightRetryIsDropped
--- PASS: TestOutcomeMatrix
    --- PASS: TestOutcomeMatrix/rg_0_clean_complete_stream_browses_to_exit_0
    --- PASS: TestOutcomeMatrix/rg_0_summary-only_stream_is_no-results_exit_1
    --- PASS: TestOutcomeMatrix/anomalous_rg_1_with_retained_results_browses_to_exit_0
    --- PASS: TestOutcomeMatrix/rg_1_empty_complete_stream_is_no-results_exit_1
    --- PASS: TestOutcomeMatrix/fatal_code_with_usable_results_overlays_browse,_q_dismissal,_exit_2
    --- PASS: TestOutcomeMatrix/fatal_code_with_usable_results_overlays_browse,_Esc_dismissal,_exit_2
    --- PASS: TestOutcomeMatrix/fatal_code_without_usable_results_is_overlay-only,_q_exits_2
    --- PASS: TestOutcomeMatrix/fatal_code_without_usable_results_is_overlay-only,_Esc_exits_2
    --- PASS: TestOutcomeMatrix/signal_death_with_usable_results_overlays_browse,_exit_2
    --- PASS: TestOutcomeMatrix/signal_death_without_usable_results_is_overlay-only,_exit_2
    --- PASS: TestOutcomeMatrix/missing_summary_with_valid_matches_overlays_browse,_exit_2
    --- PASS: TestOutcomeMatrix/orphaned_end_with_valid_matches_overlays_browse,_exit_2
    --- PASS: TestOutcomeMatrix/file_left_open_at_stream_end_overlays_browse,_exit_2
    --- PASS: TestOutcomeMatrix/stderr_warning_with_results_under_rg_0_overlays_browse,_exit_0
    --- PASS: TestOutcomeMatrix/stderr_warning_with_results_under_rg_1_overlays_browse,_exit_0
    --- PASS: TestOutcomeMatrix/stderr_warning_with_zero_results_under_rg_0_overlays_no-results,_exit_1
    --- PASS: TestOutcomeMatrix/stderr_warning_with_zero_results_under_rg_1_overlays_no-results,_exit_1
    --- PASS: TestOutcomeMatrix/all-binary_after_a_warning_lands_on_no-results_with_the_count,_exit_1
    --- PASS: TestOutcomeMatrix/unknown-type-only_warnings_with_zero_results_warn_then_no-results,_exit_1
    --- PASS: TestOutcomeMatrix/malformed_skip_with_usable_results_overlays_browse,_exit_0
    --- PASS: TestOutcomeMatrix/malformed_skip_with_zero_usable_results_is_record-loss_fatal,_q_exits_2
    --- PASS: TestOutcomeMatrix/malformed_skip_with_zero_usable_results_is_record-loss_fatal,_Esc_exits_2
    --- PASS: TestOutcomeMatrix/skipped_record_plus_binary_exclusion_is_record-loss_fatal,_exit_2
    --- PASS: TestOutcomeMatrix/missing_end_with_no_matches_is_fatal_overlay,_exit_2
    --- PASS: TestOutcomeMatrix/ctrl+c_in_browse_overrides_the_fixed_status_with_130
    --- PASS: TestOutcomeMatrix/ctrl+c_on_no-results_overrides_the_fixed_status_with_130
    --- PASS: TestOutcomeMatrix/ctrl+c_with_the_overlay_open_overrides_the_fixed_status_with_130
    --- PASS: TestOutcomeMatrix/every_retained_file_failing_to_load_keeps_the_fixed_status_0
    --- PASS: TestOutcomeMatrix/current-file_load_failure_keeps_the_fixed_status_2
    --- PASS: TestOutcomeMatrix/fatal_search_with_usable_results_then_all_files_fail_keeps_2
PASS
ok  	vrg/internal/app
```

## Manual route — the real binary on a PTY

`demo-failures.sh` (checked into this directory) runs the freshly built `vrg` on a real tmux PTY with real `rg`, as an unprivileged user. It copies the two matched files from `fixture-src/` into a disposable `mktemp` directory — repository and user files are never touched — records `b-second.txt`'s original mode, and an EXIT trap restores it (and removes the fixture) on completion or interruption. Stops are f1s1 < f2s1 < f2s2. After file 1 renders, `chmod 000` makes file 2 unreadable: `n` into it opens the error overlay over the `(unreadable)` placeholder; `Esc` dismisses it; a same-file `n` opens nothing; `n` wraps back to file 1 from the session cache; `p` re-enters file 2 — the prior failure's overlay reopens and exactly one retry runs, failing fast and appending a second scroll-preserved occurrence; keys under the open overlay are swallowed; `Esc` + two `p` steps return to file 1. Because a chmod-000 read fails inside a single render tick, the transient `Loading…` paint is unobservable at a PTY — so the harness then swaps file 2's path for a writerless FIFO and re-enters: the retry's `open()` blocks, holding the reopened overlay and `Loading…` on screen deterministically; `Esc` dismisses the overlay while the in-flight retry stays undisturbed; a writer pulse lets it settle to content with no overlay involved; and `q` exits 0 with both failure occurrences replayed on stderr. The deterministic model tests remain the authority for settlement and append semantics.

```bash
cd /home/chris/vrg/Notes/walkthroughs/026-06/code-walkthrough && unset RIPGREP_CONFIG_PATH && ./demo-failures.sh
```

```output
ok: startup: file 1's content renders
ok: startup: file 1's filename rule
ok: file 2 mode after chmod -> 0
ok: n into file 2: error overlay opens
ok: n into file 2: overlay names the path
ok: n into file 2: the (unreadable) placeholder
ok: Esc: overlay dismissed
ok: Esc: (unreadable) stays
ok: same-file n: no new overlay
ok: same-file n: still (unreadable)
ok: n wrap to file 1: cached content
ok: re-entry p: the prior failure overlay reopened
ok: re-entry p: retry settled to (unreadable)
ok: second failure appended one occurrence -> 2
ok: p under the open overlay: still two occurrences -> 2
ok: same-file p after dismissal: no new overlay
ok: fifo re-entry: the prior failure overlay reopened
ok: fifo re-entry: the in-flight retry paints Loading…
ok: Esc mid-retry: overlay dismissed
ok: Esc mid-retry: the retry is undisturbed
ok: retry settled: content replaced Loading…
ok: retry settled: the placeholder is gone
ok: vrg exit status -> 0
ok: stderr lists both failure occurrences -> 2
ok: post-exit pane shows the replayed diagnostics
demo-failures: all checks passed
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/026-06/code-walkthrough && strip="s|/tmp/vrg26-fixture\.[A-Za-z0-9_]*/||g" && echo "== n into file 2: overlay over (unreadable) ==" && sed -n "1,12p" fail/screen-01-n-into-f2-overlay.txt | sed "$strip; s/ *$//" && echo "== Esc: dismissed, placeholder stays ==" && sed -n "1,3p" fail/screen-02-esc-dismissed.txt | sed "$strip; s/ *$//" && echo "== re-entry p: overlay reopened, second occurrence appended ==" && sed -n "1,14p" fail/screen-04-reentry-overlay-unreadable.txt | sed "$strip; s/ *$//" && echo "== fifo re-entry: overlay + in-flight Loading… ==" && sed -n "1,12p" fail/screen-05-reentry-overlay-loading.txt | sed "$strip; s/ *$//" && echo "== Esc mid-retry: overlay gone, Loading… undisturbed ==" && sed -n "1,3p" fail/screen-06-esc-retry-inflight.txt | sed "$strip; s/ *$//" && echo "== retry settled: content replaced the placeholder ==" && sed -n "1,5p" fail/screen-07-retry-settled.txt | sed "$strip; s/ *$//" && echo "== q: exit 0, failures replayed on stderr ==" && cat fail/stderr.txt | sed "$strip"
```

```output
== n into file 2: overlay over (unreadable) ==
./a-first.txt   ── ./b-second.txt (unreadable) ─────────────────────────────────
./b-second.txt     (unreadable)








  ┌──────────────────────────────────────────────────────────────────────────┐
  │cannot read ./b-second.txt: open ./b-second.txt:│
== Esc: dismissed, placeholder stays ==
./a-first.txt   ── ./b-second.txt (unreadable) ─────────────────────────────────
./b-second.txt     (unreadable)

== re-entry p: overlay reopened, second occurrence appended ==
./a-first.txt   ── ./b-second.txt (unreadable) ─────────────────────────────────
./b-second.txt     (unreadable)







  ┌──────────────────────────────────────────────────────────────────────────┐
  │cannot read ./b-second.txt: open ./b-second.txt:│
  │permission denied                                                         │
  │cannot read ./b-second.txt: open ./b-second.txt:│
  │permission denied                                                         │
== fifo re-entry: overlay + in-flight Loading… ==
./a-first.txt   ── ./b-second.txt (unreadable) ─────────────────────────────────
./b-second.txt     Loading…








  ┌──────────────────────────────────────────────────────────────────────────┐
  │cannot read ./b-second.txt: open ./b-second.txt:│
== Esc mid-retry: overlay gone, Loading… undisturbed ==
./a-first.txt   ── ./b-second.txt (unreadable) ─────────────────────────────────
./b-second.txt     Loading…

== retry settled: content replaced the placeholder ==
./a-first.txt   ── ./b-second.txt ──────────────────────────────────────────────
./b-second.txt  1  MARK beta one
                2  x
                3  MARK beta two
                4  x
== q: exit 0, failures replayed on stderr ==
cannot read ./b-second.txt: open ./b-second.txt: permission denied
cannot read ./b-second.txt: open ./b-second.txt: permission denied
```

## Verdict

Issue #26 is verified. On the real binary, `n` into the chmod-000 file opened the error overlay naming the path over the `(unreadable)` placeholder — the filename rule carrying the real `(unreadable)` status note in its Issue #24 slot — while `Esc` dismissed the overlay and a same-file `n` produced no new overlay and no reload. A cross-file `p` re-entry reopened the retained prior failure, ran exactly one retry, and appended one scroll-preserving occurrence to the open overlay; after dismissal and return to file 1, a writerless-FIFO swap held the second re-entry's retry in flight so the reopened overlay and `Loading…` painted together, `Esc` dismissed without disturbing the load, and a writer pulse settled it to content with no overlay involved. `q` exited 0 — the fixed status untouched — and stderr replayed both `cannot read` occurrences. The injected-loader model tests are the deterministic authority for the full contract: the current/non-current notification split, the composed view's well-formedness from 80×24 down to 20×3, the gated sequence's settlement independent of dismissal, navigation-away isolation, the in-flight drop, and the three outcome-matrix rows proving load failures never recompute the fixed status.

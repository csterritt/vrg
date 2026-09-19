# Issue #26: read failures — "(unreadable)", notification, and retry rules

*2026-09-17T22:58:31Z by Showboat 0.6.1*
<!-- showboat-id: 21aadba2-09ac-485f-9b52-579b4ca886bf -->

Issue #26 makes asynchronous file-read failures explicit and recoverable: a current-file failure opens the Issue #9 error overlay and shows the "(unreadable)" placeholder while its cursor stops and identifying filename row are retained; a non-current failure is collected as a diagnostic only — no overlay, no indicator — discovered later by visiting the file or through the Issue #11 stderr replay; a same-file n/p step never retries while entering a failed file from a different file starts exactly one retry under the five-step re-entry sequence (prior-failure overlay immediately, "Loading…", one retry, settlement independent of dismissal, one appended occurrence on a second failure with scroll preserved); and no load failure ever changes the fixed search-derived exit status. See Notes/issues/026-read-failures-unreadable-retry-rules.md, Notes/tasks/026-read-failures-unreadable-retry-rules.md, and the PRD section 'File loading, cache, reload, and selection consistency' in Notes/PRD-vrg.md. All artifacts live in this directory.

## Gates — module integrity, build, vet

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

## Notification and retry tests — the injected-loader contract

internal/app/readfail_test.go pins the Issue #26 notification split through the WithLoader injected-loader seam — no filesystem permissions involved. TestCurrentFileFailureShowsOverlay: the current file's read failure opens the error overlay with the 'cannot read …' diagnostic, shows '(unreadable)', keeps the filename row naming the path, and the retained stops still step without minting a load or a new overlay. TestNonCurrentFailureIsDiagnosticOnly and TestNonCurrentFailureAppearsInReplay: a failure settling for a non-current file changes no frame and opens no overlay yet collects the diagnostic that reaches the stderr replay. TestCrossFileEntryRetriesFailedFile: re-entering a failed file shows the prior-failure overlay with 'Loading…' and mints exactly one retry whose second failure appends once to the still-open overlay and the collection. TestUnreadableComposedViewAtConstrainedWidths: the unreadable state at 80/30/20 columns — the …-truncated path in the Issue #24 filename-rule slot, the placeholder up, nothing overflowing, nonnegative layout dimensions.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestCurrentFileFailureShowsOverlay|TestNonCurrentFailureIsDiagnosticOnly|TestNonCurrentFailureAppearsInReplay|TestCrossFileEntryRetriesFailedFile|TestUnreadableComposedViewAtConstrainedWidths' ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL|    ---)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestCurrentFileFailureShowsOverlay
--- PASS: TestNonCurrentFailureIsDiagnosticOnly
--- PASS: TestNonCurrentFailureAppearsInReplay
--- PASS: TestCrossFileEntryRetriesFailedFile
--- PASS: TestUnreadableComposedViewAtConstrainedWidths
ok  	vrg/internal/app
```

## Outcome rows — load failures never move the fixed status

The outcome matrix gains the failLoads field — index file positions whose loads fail once the search settles, the current file's through the transition's own load command under failAllLoader. Three rows prove the guarantee: every retained file failing under fixed status 0 still exits 0; a current-file failure under fixed status 2 still exits 2; and the composed row — usable results at fixed status 2 where every retained file then fails — keeps 2, the failures confined to file presentation and diagnostics while the already-fixed fatal-search outcome is never recomputed.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestOutcomeMatrix/(all_loads_fail|current-file_failure)' ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL|    ---)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestOutcomeMatrix
    --- PASS: TestOutcomeMatrix/all_loads_fail_with_fixed_status_0_still_exits_0
    --- PASS: TestOutcomeMatrix/current-file_failure_with_fixed_status_2_still_exits_2
    --- PASS: TestOutcomeMatrix/all_loads_fail_with_fixed_status_2_still_exits_2
ok  	vrg/internal/app
```

## Gated re-entry tests — the five-step sequence

The gated re-entry tests drive the deterministic sequence with heldNthLoad (the gate holding only the nth minted load — the re-entry retry is a deterministic ordinal) and gatedFailLoader (a mutex-guarded fail set a gated worker's read observes). TestReentryShowsPriorFailureAndStartsOneRetry: entering a failed file from a different file shows the prior-failure overlay with 'Loading…' immediately and mints exactly one retry while the overlay is up — a re-entry during the in-flight retry dropped per Issue #25's one-load-per-path rule. TestReentryRetryEscLeavesLoadUndisturbed: Esc dismisses the overlay while the request stays live and held; the settled second failure restores '(unreadable)' and re-opens the overlay. TestReentryRetrySuccessKeepsPriorOverlay: a successful retry shows content, collects nothing new, and leaves the prior-failure overlay up until dismissed. TestReentryRetrySecondFailureAppends: the second failure appends exactly one occurrence to the open overlay — the reader's overlayScroll untouched — mirrored once in the session collection. TestReentryRetryAwayAndBack: navigating away lets the retry settle as a non-current diagnostic-only failure, and a later re-entry re-runs the same sequence against the new prior state.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestReentryShowsPriorFailureAndStartsOneRetry|TestReentryRetryEscLeavesLoadUndisturbed|TestReentryRetrySuccessKeepsPriorOverlay|TestReentryRetrySecondFailureAppends|TestReentryRetryAwayAndBack' ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL|    ---)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestReentryShowsPriorFailureAndStartsOneRetry
--- PASS: TestReentryRetryEscLeavesLoadUndisturbed
--- PASS: TestReentryRetrySuccessKeepsPriorOverlay
--- PASS: TestReentryRetrySecondFailureAppends
--- PASS: TestReentryRetryAwayAndBack
ok  	vrg/internal/app
```

## Full module regression

The failure record, the failDiag prior-state map, the notification split, and the retry path touch navigation, overlay, replay, and layout — the whole suite runs.

The new current-file error overlay also reaches the cmd/vrg boundary tests whose fake-rg fixtures never wrote the reported files — a single q now dismisses the modal overlay instead of quitting. The fixtures are made honest: writeHappyFiles creates the stream's a.go/b.go so the quit-driving tests exercise ordinary browse→exit, and the hostile-filename replay test — where the failure is the point — dismisses the overlay before its quit keypress.

```bash
cd /home/chris/vrg && go test -count=1 ./internal/filebuffer ./internal/viewport ./internal/app ./internal/safepresentation ./internal/searchindex ./internal/theme ./internal/cli ./cmd/vrg 2>&1 | sed -E 's/\t[0-9.]+s$//'
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
```

## Manual check — the real binary on a pty

manual_route.sh runs the whole route as the unprivileged user against a disposable mktemp fixture — never a repository file: it copies the two matched files (fixture/aaa-readable.txt, one stop; fixture/bbb-blocked.txt, two stops), records the second file's original mode, and arms a trap restoring the mode and removing the directory on exit or interruption. pty_readfail.py then drives vrg on a real 80x24 pty with stderr redirected to a file: the search reads both files while readable — the chmod 000 lands only after the index is built, since rg could not read an already-blocked file and its stops would not exist — then n into file 2 fails the load: the error overlay opens over the '(unreadable)' placeholder, Esc dismisses, a same-file n mints nothing, p p returns to file 1's cached content, and the next p re-enters file 2 — the prior-failure overlay reopens with 'Loading…' while the one retry sits held at VRG_TEST_LOAD_GATE. Releasing the gate lets the retry fail: exactly one 'cannot read' appends to the still-open overlay. q exits 0 and both failures replay to real stderr. The injected-loader model tests above remain the authoritative deterministic verification.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/026-06/code-walkthrough/vrg ./cmd/vrg && bash Notes/walkthroughs/026-06/code-walkthrough/manual_route.sh
```

```output
fixture: <tmp> (bbb-blocked.txt mode 664 recorded)
startup   : file 1 up — the search read both files while readable — '─ <tmp>/aaa-readable.txt ─────────────'
n         : file 2's load failed — overlay 'cannot read …' over '(unreadable)'
Esc       : overlay dismissed — '(unreadable)' remains
n         : same-file step — no retry, no new overlay
p p       : back on file 1 — cached content, no loads
p         : re-entry — prior-failure overlay up, 'Loading…' under it, one retry held
release   : retry failed — one occurrence appended to the open overlay (2 shown)
q         : exit 0 — failures never touched the fixed status
stderr    : cannot read <tmp>/bbb-blocked.txt: permission denied
stderr    : cannot read <tmp>/bbb-blocked.txt: permission denied
OK
permissions restored — no file left altered
```

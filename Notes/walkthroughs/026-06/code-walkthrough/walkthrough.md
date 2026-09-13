# Issue #26: Read failures — unreadable, retry rules, re-entry

*2026-09-12T13:22:14Z by Showboat 0.6.1*
<!-- showboat-id: ddc129b2-46d4-492e-a122-772f42abdba5 -->

Walkthrough for Issue #26 (Notes/tasks/026-read-failures-unreadable-retry-rules.md), implementing read-failure notification and retry rules in vrg. Unreadable files no longer leave the UI stuck on Loading; the panel shows (unreadable), the cursor stops remain navigable, diagnostics are collected for replay, and the fixed search-derived exit status is preserved. Re-entering a previously failed file from a different file follows a deterministic five-step sequence: the prior-failure overlay reopens immediately with Loading, exactly one retry starts while the overlay is open, Esc dismisses without disturbing the in-flight load, settlement updates the placeholder without waiting for dismissal, and a second failure appends exactly one new diagnostic occurrence with the reader's scroll position preserved. References: Notes/PRD-vrg.md (File loading, cache, reload, and selection consistency), Notes/wiki/read-failures-and-retry.md.

Contracts verified:

- A current-file read failure opens a non-fatal error overlay with the sanitized diagnostic.
- The content panel shows (unreadable) instead of Loading after a current-file failure.
- The cursor stops remain navigable after a current-file failure.
- A non-current read failure collects the diagnostic without opening an overlay or disturbing the visible panel.
- The non-current failure diagnostic appears in the replay.
- A same-file n/p step requests no reload.
- Entry from a different file into an uncached, previously failed file requests exactly one retry.
- The composed view stays well-formed at constrained widths (Issue #24 slot rules, (unreadable) placeholder, no overflow, nonnegative dimensions).
- Load failures never change the fixed search-derived exit status (all-fail fixed-0, current-file fixed-2, composed all-fail-with-fixed-2).
- Re-entry reopens the prior-failure overlay immediately with Loading (not (unreadable)).
- Exactly one retry starts while the overlay is open.
- Esc dismisses the overlay without disturbing the in-flight load.
- Settlement updates the placeholder to content or (unreadable) without waiting for dismissal.
- A successful retry collects no new diagnostic; the overlay remains displayed until dismissed.
- A second failure appends exactly one new diagnostic occurrence with the reader's scroll position preserved.
- Navigating away during a retry updates only that file's state/cache (Issue #25 isolation).
- A second re-entry while a retry is in flight is dropped under the one-load-per-path rule.

The injected-loader model tests are the authoritative deterministic verification. The manual route at the end demonstrates the behavior against real files with chmod 000, using a disposable temporary fixture directory and a shell trap that restores the original mode on exit or interruption.

```bash
go build ./cmd/... ./internal/... && go vet ./cmd/... ./internal/... && echo GATES-OK
```

```output
GATES-OK
```

```bash
go test -count=1 ./cmd/... ./internal/... -timeout 120s | sed 's/[[:space:]][0-9.]*s$//'
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
ok  	vrg/internal/viewport
```

## Injected-loader notification and outcome-row tests

The injected-loader tests (internal/app/read_failure_test.go) verify the current-file overlay and placeholder, non-current diagnostic-only collection, same-file versus cross-file retry, composed-view robustness, and the outcome-matrix rows proving load failures never change the fixed exit status.

```bash
go test -count=1 -v ./internal/app/ -run '^TestReadFailure' -timeout 60s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestReadFailureCurrentFileOverlayPlaceholder
--- PASS: TestReadFailureCurrentFileOverlayPlaceholder (0.00s)
=== RUN   TestReadFailureCurrentFileDismissReturnsToBrowse
--- PASS: TestReadFailureCurrentFileDismissReturnsToBrowse (0.00s)
=== RUN   TestReadFailureNonCurrentDiagnosticOnly
--- PASS: TestReadFailureNonCurrentDiagnosticOnly (0.00s)
=== RUN   TestReadFailureNonCurrentDiagnosticInReplay
--- PASS: TestReadFailureNonCurrentDiagnosticInReplay (0.00s)
=== RUN   TestReadFailureSameFileStepNoRetry
--- PASS: TestReadFailureSameFileStepNoRetry (0.00s)
=== RUN   TestReadFailureCrossFileEntryRetries
--- PASS: TestReadFailureCrossFileEntryRetries (0.00s)
=== RUN   TestReadFailureComposedViewRobustness
=== RUN   TestReadFailureComposedViewRobustness/width-80
=== RUN   TestReadFailureComposedViewRobustness/width-40
=== RUN   TestReadFailureComposedViewRobustness/width-20
--- PASS: TestReadFailureComposedViewRobustness (0.00s)
    --- PASS: TestReadFailureComposedViewRobustness/width-80 (0.00s)
    --- PASS: TestReadFailureComposedViewRobustness/width-40 (0.00s)
    --- PASS: TestReadFailureComposedViewRobustness/width-20 (0.00s)
=== RUN   TestReadFailureOutcomeAllFailFixed0
--- PASS: TestReadFailureOutcomeAllFailFixed0 (0.00s)
=== RUN   TestReadFailureOutcomeCurrentFileFailureFixed2
--- PASS: TestReadFailureOutcomeCurrentFileFailureFixed2 (0.00s)
=== RUN   TestReadFailureOutcomeComposedAllFailFixed2
--- PASS: TestReadFailureOutcomeComposedAllFailFixed2 (0.00s)
PASS
ok  	vrg/internal/app
```

## Gated re-entry sequence tests

The gated re-entry tests (internal/app/reentry_test.go) verify the five-step re-entry sequence: immediate prior-failure overlay reopen with Loading, exactly one in-flight retry, Esc-without-disturbance, settlement presentation, append-preserving-scroll on second failure, navigation away during retry, and one-load-per-path drop on re-entry while in flight.

```bash
go test -count=1 -v ./internal/app/ -run '^TestReentry' -timeout 60s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestReentryOverlayReopensImmediately
--- PASS: TestReentryOverlayReopensImmediately (0.00s)
=== RUN   TestReentrySuccessfulSettlement
--- PASS: TestReentrySuccessfulSettlement (0.00s)
=== RUN   TestReentrySecondFailureAppend
--- PASS: TestReentrySecondFailureAppend (0.00s)
=== RUN   TestReentryEscDoesNotDisturbInFlightLoad
--- PASS: TestReentryEscDoesNotDisturbInFlightLoad (0.00s)
=== RUN   TestReentryNavigateAwayDuringRetry
--- PASS: TestReentryNavigateAwayDuringRetry (0.00s)
=== RUN   TestReentryOneRetryWhileInFlight
--- PASS: TestReentryOneRetryWhileInFlight (0.00s)
PASS
ok  	vrg/internal/app
```

## Manual demonstration: injected failing loader

The manual case demonstrates the read-failure behavior with an injected failing loader (the deterministic verification):

- File 1 (src/a.go) loads successfully.
- File 2 (src/b.go) fails to load (unreadable).
- Startup shows file 1.
- n into file 2 shows the overlay and (unreadable).
- Esc dismisses the overlay.
- A same-file n opens no new overlay and requests no retry.
- p p p back into file 2 (re-entry) shows the overlay with Loading.
- Dismissal, then q exits 0 with the failures listed in diagnostics.

The demonstration uses a test program that injects a gated failing loader. The program drives the model through the same Update flow the TUI uses, capturing the state at each step.

```bash
cp Notes/walkthroughs/026-06/code-walkthrough/demo_artifacts/manual_demo_test.go internal/app/manual_demo_test.go && go test -count=1 -v -tags manual_demo ./internal/app/ -run '^TestManualDemoReadFailure$' -timeout 30s; rm internal/app/manual_demo_test.go
```

```output
=== RUN   TestManualDemoReadFailure
Step 1 (startup): file 1 shows content (correct)
Step 2 (n into file 2): overlay opens, panel shows (unreadable) (correct)
Step 3 (same-file n): no new overlay, no retry (correct)
Step 4 (p p p back into file 2): overlay reopens with Loading (correct)
Step 5 (q): exit 0, failures listed in diagnostics (correct)

Manual demonstration passed: read-failure behavior matches the Issue #26 contracts.
--- PASS: TestManualDemoReadFailure (0.00s)
PASS
ok  	vrg/internal/app	0.024s
```

```bash
cp Notes/walkthroughs/026-06/code-walkthrough/demo_artifacts/manual_demo_test.go internal/app/manual_demo_test.go && go test -count=1 -v -tags manual_demo ./internal/app/ -run '^TestManualDemoReadFailure$' -timeout 30s 2>&1 | sed 's/[[:space:]]0\.[0-9]*s//g; s/([0-9.]*s)/(0.00s)/g'; rm internal/app/manual_demo_test.go
```

```output
=== RUN   TestManualDemoReadFailure
Step 1 (startup): file 1 shows content (correct)
Step 2 (n into file 2): overlay opens, panel shows (unreadable) (correct)
Step 3 (same-file n): no new overlay, no retry (correct)
Step 4 (p p p back into file 2): overlay reopens with Loading (correct)
Step 5 (q): exit 0, failures listed in diagnostics (correct)

Manual demonstration passed: read-failure behavior matches the Issue #26 contracts.
--- PASS: TestManualDemoReadFailure (0.00s)
PASS
ok  	vrg/internal/app
```

```bash
cp Notes/walkthroughs/026-06/code-walkthrough/demo_artifacts/manual_demo_test.go internal/app/manual_demo_test.go && go test -count=1 -v -tags manual_demo ./internal/app/ -run '^TestManualDemoReadFailure$' -timeout 30s 2>&1 | sed 's/[[:space:]]0\.[0-9]*s//g; s/([0-9.]*s)/(0.00s)/g'; rm internal/app/manual_demo_test.go
```

```output
=== RUN   TestManualDemoReadFailure
Step 1 (startup): file 1 shows content (correct)
Step 2 (n into file 2): overlay opens, panel shows (unreadable) (correct)
Step 3 (same-file n): no new overlay, no retry (correct)
Step 4 (p p p back into file 2): overlay reopens with Loading (correct)
Step 5 (q): exit 0, failures listed in diagnostics (correct)

Manual demonstration passed: read-failure behavior matches the Issue #26 contracts.
--- PASS: TestManualDemoReadFailure (0.00s)
PASS
ok  	vrg/internal/app
```

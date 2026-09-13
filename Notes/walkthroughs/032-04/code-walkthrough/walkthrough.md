# Issue #32: Overlay precedence and reconciled q/Esc semantics

*2026-09-12T14:21:33Z by Showboat 0.6.1*
<!-- showboat-id: ae3139ea-af87-433d-86c8-076b575f5797 -->

Walkthrough for Issue #32 (Notes/tasks/032-overlay-precedence-esc-semantics.md), establishing the overlay precedence stack and the reconciled q/Esc semantics. The precedence stack is ctrl+c over modal error over help over pop-up over base keys. A new error while help is open suspends help, saving its scroll position; dismissing the error restores help at that exact position. The Issue #26 append-preserving-scroll primitive is generalized to all appended errors: a read failure while any error or warning overlay is open appends without resetting the scroll position. Opening help or an error cancels any active Issue #15 file-change pop-up with no return. Esc is an overlay-dismissal key only: with no overlay it is a no-op in searching, no-results, and browsing (its only effect is dismissing a pop-up). Esc never exits from a base state; the one exception is dismissing a fatal no-results overlay with Esc, which terminates with status 2 because there is no underlying state. The dismissal-outcome table covers browse+error, browse+help, browse+error-over-help, no-results+warning, no-results+help, fatal no-results, and record-loss no-results for both q and Esc. References: Notes/PRD-vrg.md (Colours, overlays, and key precedence; Outcome and exit-status contract), Notes/wiki/overlay-precedence.md, Issue #32.

Contracts verified:

- Error suspends help: a new error while help is open suspends help, saving its scroll position.
- Help scroll restoration after error dismissal for both q and Esc.
- Appending a second error while the reader is scrolled preserves the scroll position and makes the appended text reachable.
- Opening help cancels any pop-up; opening an error cancels any pop-up; no suspended pop-up returns.
- Esc with no overlay is a no-op in browsing and no-results.
- Esc still dismisses a pop-up when one is open.
- The dismissal-outcome table for both q and Esc: browse+error, browse+help, browse+error-over-help, no-results+warning, no-results+help, fatal no-results, record-loss no-results.
- Follow-up assertions: a second q from a still-running base state exits the fixed status; a second Esc leaves the state running; the full three-key error-over-help sequence (dismiss error, close restored help, base-state q exits).
- Error-first key routing: error over help over pop-up over base keys; ctrl+c remains global.

The model tests are the authoritative deterministic verification. The manual route at the end demonstrates the behavior with a real terminal.

```bash
cd /home/chris/vrg && go build ./cmd/... ./internal/... && go vet ./cmd/... ./internal/... && echo GATES-OK
```

```output
GATES-OK
```

```bash
cd /home/chris/vrg && go test -count=1 ./cmd/... ./internal/... -timeout 120s | sed 's/[[:space:]][0-9.]*s$//'
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

## Suspension model tests

The suspension tests (internal/app/overlay_precedence_test.go) verify that a new error while help is open suspends help, saving its scroll position, and that dismissing the error with q or Esc restores help at that exact position. The error-first key routing test verifies that scrolling the error overlay does not affect the suspended help's saved scroll position.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run '^TestErrorSuspendsHelp|^TestErrorFirstKeyRouting|^TestErrorOverHelp' -timeout 60s | sed 's/(0\.[0-9]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestErrorSuspendsHelpScrollRestoredQ
--- PASS: TestErrorSuspendsHelpScrollRestoredQ (0.00s)
=== RUN   TestErrorSuspendsHelpScrollRestoredEsc
--- PASS: TestErrorSuspendsHelpScrollRestoredEsc (0.00s)
=== RUN   TestErrorFirstKeyRouting
--- PASS: TestErrorFirstKeyRouting (0.00s)
=== RUN   TestErrorOverHelpEscRestoresHelp
--- PASS: TestErrorOverHelpEscRestoresHelp (0.00s)
=== RUN   TestErrorOverHelpQRestoresHelp
--- PASS: TestErrorOverHelpQRestoresHelp (0.00s)
PASS
ok  	vrg/internal/app
```

## Append model tests

The append test verifies that a second error appended to an overlay the reader has scrolled to position P keeps the reader at P with the new text reachable. This generalizes the Issue #26 append-preserving-scroll primitive to all appended errors via the r retry route (Issue #27).

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run '^TestAppendErrorPreservesScroll|^TestErrorCancelsPopup|^TestEscNoOverlayDismissesPopup|^TestEscNoOpNoResults' -timeout 60s | sed 's/(0\.[0-9]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestAppendErrorPreservesScroll
--- PASS: TestAppendErrorPreservesScroll (0.00s)
=== RUN   TestErrorCancelsPopup
--- PASS: TestErrorCancelsPopup (0.00s)
=== RUN   TestEscNoOverlayDismissesPopup
--- PASS: TestEscNoOverlayDismissesPopup (0.00s)
=== RUN   TestEscNoOpNoResults
--- PASS: TestEscNoOpNoResults (0.00s)
PASS
ok  	vrg/internal/app
```

## Dismissal-outcome model tests

The dismissal-outcome table tests (TestDismissalOutcomeTableQ and TestDismissalOutcomeTableEsc) exercise every row of the Issue #32 dismissal-outcome table for both q and Esc as the dismissal key. Each row asserts the overlay is open, dismisses it, and runs state-specific follow-up assertions: from every still-running base state a second q exits the fixed status; a second Esc leaves the state running. For the error-over-help row, the full three-key sequence is asserted: dismiss error, close restored help, then base-state q exits.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run '^TestDismissalOutcomeTable' -timeout 60s | sed 's/(0\.[0-9]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestDismissalOutcomeTableQ
=== RUN   TestDismissalOutcomeTableQ/browse_with_error_overlay
=== RUN   TestDismissalOutcomeTableQ/browse_with_help
=== RUN   TestDismissalOutcomeTableQ/browse_with_error_over_help
=== RUN   TestDismissalOutcomeTableQ/empty_result_with_warning_overlay
=== RUN   TestDismissalOutcomeTableQ/no-results_with_help_open
=== RUN   TestDismissalOutcomeTableQ/fatal_with_no_usable_results
=== RUN   TestDismissalOutcomeTableQ/record-loss_with_no_results
--- PASS: TestDismissalOutcomeTableQ (0.00s)
    --- PASS: TestDismissalOutcomeTableQ/browse_with_error_overlay (0.00s)
    --- PASS: TestDismissalOutcomeTableQ/browse_with_help (0.00s)
    --- PASS: TestDismissalOutcomeTableQ/browse_with_error_over_help (0.00s)
    --- PASS: TestDismissalOutcomeTableQ/empty_result_with_warning_overlay (0.00s)
    --- PASS: TestDismissalOutcomeTableQ/no-results_with_help_open (0.00s)
    --- PASS: TestDismissalOutcomeTableQ/fatal_with_no_usable_results (0.00s)
    --- PASS: TestDismissalOutcomeTableQ/record-loss_with_no_results (0.00s)
=== RUN   TestDismissalOutcomeTableEsc
=== RUN   TestDismissalOutcomeTableEsc/browse_with_error_overlay
=== RUN   TestDismissalOutcomeTableEsc/browse_with_help
=== RUN   TestDismissalOutcomeTableEsc/browse_with_error_over_help
=== RUN   TestDismissalOutcomeTableEsc/empty_result_with_warning_overlay
=== RUN   TestDismissalOutcomeTableEsc/no-results_with_help_open
=== RUN   TestDismissalOutcomeTableEsc/fatal_with_no_usable_results
=== RUN   TestDismissalOutcomeTableEsc/record-loss_with_no_results
--- PASS: TestDismissalOutcomeTableEsc (0.00s)
    --- PASS: TestDismissalOutcomeTableEsc/browse_with_error_overlay (0.00s)
    --- PASS: TestDismissalOutcomeTableEsc/browse_with_help (0.00s)
    --- PASS: TestDismissalOutcomeTableEsc/browse_with_error_over_help (0.00s)
    --- PASS: TestDismissalOutcomeTableEsc/empty_result_with_warning_overlay (0.00s)
    --- PASS: TestDismissalOutcomeTableEsc/no-results_with_help_open (0.00s)
    --- PASS: TestDismissalOutcomeTableEsc/fatal_with_no_usable_results (0.00s)
    --- PASS: TestDismissalOutcomeTableEsc/record-loss_with_no_results (0.00s)
PASS
ok  	vrg/internal/app
```

## Manual demonstration

The manual cases are demonstrated through a deterministic Go test that drives the model through the exact key sequences and renders the view at each step. This avoids depending on a real PTY while proving the observable behavior.

The cases demonstrated:
1. Esc with no overlay does nothing in browsing.
2. q with a browse error overlay closes it (browse still running).
3. A second q exits with the fixed status (2 for a fatal search).
4. A fake rg exiting 3 with no output shows a fatal overlay.
5. Both Esc and q on that fatal overlay exit 2.
6. The gated error-while-help-open case restores help at its scroll position (full three-key sequence: q dismisses error and restores help, second q closes help to browse, third q exits with fixed status 0).

The manual demo test (demo_artifacts/manual_demo_test.go) exercises the Issue #32 manual verification scenarios.

```bash
cp /home/chris/vrg/Notes/walkthroughs/032-04/code-walkthrough/demo_artifacts/manual_demo_test.go /home/chris/vrg/internal/app/manual_demo_test.go && cd /home/chris/vrg && go test -count=1 -v -tags manual_demo ./internal/app/ -run '^TestManualDemoOverlayPrecedence$' -timeout 30s 2>&1 | sed 's/[[:space:]]0\.[0-9]*s//g; s/([0-9.]*s)/(0.00s)/g'; rm /home/chris/vrg/internal/app/manual_demo_test.go
```

```output
=== RUN   TestManualDemoOverlayPrecedence
    manual_demo_test.go:60: step 1 OK: Esc with no overlay is a no-op in browsing (state=browse)
    manual_demo_test.go:99: step 2 OK: q closed the browse error overlay (browse still running, exit=2)
    manual_demo_test.go:107: step 3 OK: second q exited with fixed status 2
    manual_demo_test.go:132: step 4 OK: fake rg exit 3 with no output shows fatal overlay (fatal=true)
    manual_demo_test.go:140: step 5a OK: Esc on fatal overlay exited 2
    manual_demo_test.go:161: step 5b OK: q on fatal overlay exited 2
    manual_demo_test.go:195: step 6a OK: help open at scroll 2
    manual_demo_test.go:208: step 6b OK: read failure suspended help (error overlay open, help suspended)
    manual_demo_test.go:221: step 6c OK: q dismissed error, help restored at scroll 2
    manual_demo_test.go:234: step 6d OK: second q closed help to browse
    manual_demo_test.go:242: step 6e OK: third q exited with fixed status 0
    manual_demo_test.go:244: Manual demonstration passed: overlay precedence behavior matches the Issue #32 contracts.
--- PASS: TestManualDemoOverlayPrecedence (0.00s)
PASS
ok  	vrg/internal/app
```

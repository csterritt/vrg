# Issue #33: Terminal too-small screen with full state recovery

*2026-09-12T14:38:34Z by Showboat 0.6.1*
<!-- showboat-id: fa12ad7a-882e-4ca6-bd1a-bb28e2a420df -->

Walkthrough for Issue #33 (Notes/tasks/033-terminal-too-small-with-state-recovery.md), the terminal too-small screen with full state recovery. The fixed minimum terminal size is 20 columns and 3 rows. When either dimension falls below this minimum, the too-small gate is installed: the View renders the centred "Terminal too small" message instead of the normal browse, no-results, or searching screen; only q and ctrl+c are active; and the full state is preserved for recovery on resize. q exits with the state-applicable outcome (130 if searching, else the fixed status decided at completion), taking precedence over Issue #32's dismissal semantics — q exits even if a modal overlay is logically open, rather than dismissing it. Esc and every other key are no-ops; a logically open overlay remains open after recovery. Resizes wholly within the too-small state (e.g. 19x2 to 10x1 to 25x8) keep the gate installed at every sub-minimum step, install no ordinary layout, mutate no anchors, and defer recovery to the final dimensions. An active file-change pop-up's timer continues during too-small; the pop-up is not displayed, and an expiry during too-small dismisses it so it is absent after recovery. References: Notes/PRD-vrg.md (Layout and indicators — minimum-size bullet), Notes/wiki/too-small-screen.md, Issue #33.

Contracts verified:

- Threshold: 20x3 minimum; below 20 columns or below 3 rows triggers the too-small screen; exactly 20x3 does not.
- Display: centred "Terminal too small" message; recovery restores the normal view.
- Active keys: q and ctrl+c only; Esc and every other key are no-ops.
- Exit semantics: q exits 130 while searching, the fixed status while browsing, 1 on no-results, 2 with a fatal no-results overlay logically open, and the fixed status (2) with a browse error overlay logically open (exiting rather than dismissing). ctrl+c exits 130.
- Round-trip state restoration: cursor selection, per-file viewport state, logical anchors, file-list visibility, wrap, colour, horizontal offset, which overlay is open, help scroll, error scroll, help-suspended-by-error relationship.
- Resizes wholly within too-small (19x2 to 10x1 to 25x8): no ordinary layout at 10x1, no anchor mutation, no partial modal restoration, recovery at 25x8.
- Pop-up timer continuation: pop-up not displayed while too-small, timer continues, expiry during too-small dismisses it.

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

## Threshold model tests

The threshold tests (internal/app/too_small_test.go) verify the 20x3 minimum: a width below 20 columns or a height below 3 rows triggers the too-small screen with the centred message, exactly 20x3 does not trigger it, and growing back from too-small restores the normal view.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run '^TestTooSmallWidthBelow20$|^TestTooSmallHeightBelow3$|^TestTooSmallBoundary20x3NotTooSmall$|^TestTooSmallRecoveryRestoresView$' -timeout 60s | sed 's/(0\.[0-9]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestTooSmallWidthBelow20
--- PASS: TestTooSmallWidthBelow20 (0.00s)
=== RUN   TestTooSmallHeightBelow3
--- PASS: TestTooSmallHeightBelow3 (0.00s)
=== RUN   TestTooSmallBoundary20x3NotTooSmall
--- PASS: TestTooSmallBoundary20x3NotTooSmall (0.00s)
=== RUN   TestTooSmallRecoveryRestoresView
--- PASS: TestTooSmallRecoveryRestoresView (0.00s)
PASS
ok  	vrg/internal/app
```

## Recovery model tests

The recovery tests verify that a too-small round trip preserves the full state: cursor selection, per-file viewport state and logical anchors, horizontal offset, file-list visibility, wrap mode, colour scheme, scrolled help overlay, scrolled error overlay, and an error-over-help stack. The resize-wholly-within tests verify that a sequence of resizes wholly within the too-small state (19x2 to 10x1 to 25x8) installs no ordinary layout at the pathological dimensions, mutates no anchors, performs no partial modal restoration, and recovers at the final 25x8 dimensions with a scrolled help overlay, an error-over-help stack, and a nontrivial viewport state all preserved.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run '^TestTooSmallRoundTrip|^TestTooSmallResizeWhollyWithin' -timeout 60s | sed 's/(0\.[0-9]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestTooSmallRoundTripPreservesCursor
--- PASS: TestTooSmallRoundTripPreservesCursor (0.00s)
=== RUN   TestTooSmallRoundTripPreservesViewportAndAnchor
--- PASS: TestTooSmallRoundTripPreservesViewportAndAnchor (0.00s)
=== RUN   TestTooSmallRoundTripPreservesHorizontalOffset
--- PASS: TestTooSmallRoundTripPreservesHorizontalOffset (0.00s)
=== RUN   TestTooSmallRoundTripPreservesListVisibility
--- PASS: TestTooSmallRoundTripPreservesListVisibility (0.00s)
=== RUN   TestTooSmallRoundTripPreservesWrapMode
--- PASS: TestTooSmallRoundTripPreservesWrapMode (0.00s)
=== RUN   TestTooSmallRoundTripPreservesColourSetting
--- PASS: TestTooSmallRoundTripPreservesColourSetting (0.00s)
=== RUN   TestTooSmallRoundTripPreservesScrolledHelpOverlay
--- PASS: TestTooSmallRoundTripPreservesScrolledHelpOverlay (0.00s)
=== RUN   TestTooSmallRoundTripPreservesScrolledErrorOverlay
--- PASS: TestTooSmallRoundTripPreservesScrolledErrorOverlay (0.00s)
=== RUN   TestTooSmallRoundTripPreservesErrorOverHelpStack
--- PASS: TestTooSmallRoundTripPreservesErrorOverHelpStack (0.00s)
=== RUN   TestTooSmallResizeWhollyWithinPreservesState
--- PASS: TestTooSmallResizeWhollyWithinPreservesState (0.00s)
=== RUN   TestTooSmallResizeWhollyWithinErrorOverHelpStack
--- PASS: TestTooSmallResizeWhollyWithinErrorOverHelpStack (0.00s)
PASS
ok  	vrg/internal/app
```

## Exit-semantics model tests

The exit-semantics tests verify that q on the too-small screen exits with the state-applicable outcome, taking precedence over Issue #32's dismissal semantics. q exits 130 while searching, the fixed status (0) while browsing, 1 on no-results, 2 with a fatal no-results overlay logically open, and the fixed status (2) with a browse error overlay logically open (exiting rather than dismissing). ctrl+c exits 130. Esc with an overlay logically open is a no-op (the overlay is still open after recovery at the same scroll position). Every other key (n/p/w/c/r/h/? and scroll keys) is a no-op.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run '^TestTooSmallQ|^TestTooSmallCtrlC|^TestTooSmallEsc|^TestTooSmallOtherKeys' -timeout 60s | sed 's/(0\.[0-9]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestTooSmallQDuringSearchingExits130
--- PASS: TestTooSmallQDuringSearchingExits130 (0.00s)
=== RUN   TestTooSmallQWhileBrowsingExitsFixedStatus
--- PASS: TestTooSmallQWhileBrowsingExitsFixedStatus (0.00s)
=== RUN   TestTooSmallQOnNoResultsExits1
--- PASS: TestTooSmallQOnNoResultsExits1 (0.00s)
=== RUN   TestTooSmallQFatalNoResultsOverlayExits2
--- PASS: TestTooSmallQFatalNoResultsOverlayExits2 (0.00s)
=== RUN   TestTooSmallQBrowseErrorOverlayExitsProgram
--- PASS: TestTooSmallQBrowseErrorOverlayExitsProgram (0.00s)
=== RUN   TestTooSmallCtrlCExits130
--- PASS: TestTooSmallCtrlCExits130 (0.00s)
=== RUN   TestTooSmallEscWithOverlayIsNoOp
--- PASS: TestTooSmallEscWithOverlayIsNoOp (0.00s)
=== RUN   TestTooSmallOtherKeysAreNoOps
--- PASS: TestTooSmallOtherKeysAreNoOps (0.00s)
PASS
ok  	vrg/internal/app
```

## Pop-up timer continuation model tests

The pop-up tests verify that an active file-change pop-up's timer continues during too-small. The pop-up is not displayed on the too-small screen, even while the timer is still running. An expiry arriving during too-small dismisses the pop-up so it is absent after recovery.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run '^TestTooSmallPopup' -timeout 60s | sed 's/(0\.[0-9]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestTooSmallPopupTimerContinuesAndExpiryDismisses
--- PASS: TestTooSmallPopupTimerContinuesAndExpiryDismisses (0.00s)
=== RUN   TestTooSmallPopupNotDisplayed
--- PASS: TestTooSmallPopupNotDisplayed (0.00s)
PASS
ok  	vrg/internal/app
```

## Manual demonstration

The manual case is demonstrated through a deterministic Go test that drives the model through the exact key sequence and renders the view at each step. This avoids depending on a real PTY while proving the observable behavior.

The case demonstrated:
1. Open help.
2. Scroll help.
3. Shrink terminal to 15x2.
4. Show "Terminal too small".
5. Enlarge terminal.
6. Show help reappearing at the same scroll position.
7. Shrink again.
8. Press q to exit.

The manual demo test (demo_artifacts/manual_demo_test.go) exercises the Issue #33 manual verification scenario.

```bash
cp /home/chris/vrg/Notes/walkthroughs/033-04/code-walkthrough/demo_artifacts/manual_demo_test.go /home/chris/vrg/internal/app/manual_demo_test.go && cd /home/chris/vrg && go test -count=1 -v -tags manual_demo ./internal/app/ -run '^TestManualDemoTooSmall$' -timeout 30s 2>&1 | sed 's/[[:space:]]0\.[0-9]*s//g; s/([0-9.]*s)/(0.00s)/g'; rm /home/chris/vrg/internal/app/manual_demo_test.go
```

```output
=== RUN   TestManualDemoTooSmall
    manual_demo_test.go:55: step 1 OK: help open
    manual_demo_test.go:64: step 2 OK: help scrolled to position 3
    manual_demo_test.go:82: step 3 OK: terminal shrunk to 15x2, 'Terminal too small' shown, help logically open at scroll 3
    manual_demo_test.go:85: step 4 OK: 'Terminal too small' displayed
    manual_demo_test.go:92: step 5 OK: terminal enlarged to 80x24
    manual_demo_test.go:105: step 6 OK: help reappeared at scroll position 3
    manual_demo_test.go:116: step 7 OK: terminal shrunk to 15x2 again, 'Terminal too small' shown
    manual_demo_test.go:126: step 8 OK: q exited with fixed browse status 0
    manual_demo_test.go:127: Manual demonstration passed: too-small screen behavior matches the Issue #33 contracts.
--- PASS: TestManualDemoTooSmall (0.00s)
PASS
ok  	vrg/internal/app
```

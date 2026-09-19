# Issue #41: error overlays keep every wrapped row scrollable — no head/tail compression

*2026-09-18T09:31:35Z by Showboat 0.6.1*
<!-- showboat-id: 983cfef9-3d71-460a-b039-d2461c2041cf -->

Issue #41 makes the complete-scrollable-row contract explicit: the error overlay's scrollable row set is the complete wrapped diagnostic - no head-plus-ellipsis-plus-tail compression ever removes rows from the model - and overlayScroll clamps to [0, max(0, rows-visible)] in both the key handler and the render path, so every row of an arbitrarily long diagnostic is reachable by up/down. The scrollBox component behind the overlay (shared with help since Issue #31) already kept the complete set, so the work lands as the missing proofs plus documentation; the modal key contract is unchanged (u, d, page up, and page down stay ignored while the overlay is open), appended errors still extend the set at the reader's position, and render-time clipping at tiny sizes is preserved. This contract supersedes Issue #9's original requirement that the >= 1 MiB stderr fixture show head and tail in one frame - TestStderrContentFixture keeps its drainage/completeness/captured-stderr assertions while tail reachability moves to model-level proofs. See Notes/issues/041-overlay-full-scroll-no-head-tail-compression.md, Notes/tasks/041-overlay-full-scroll-no-head-tail-compression.md, and the 'Colours, overlays, and key precedence' section of Notes/PRD-vrg.md. This walkthrough runs the Issue #41 model tests and the revised TestStderrContentFixture, then drives the issue's manual scenario on a real PTY through smoke.py. Artifacts (the built vrg binary, smoke.py, and the generated fixture) live in this directory.

## Automated tests - complete rows, clamp, and traversal

overlay_test.go pins the contract at the model level: TestOverlayKeepsCompleteDiagnostic asserts the >= 1 MiB stderr shape's scrollable set holds both the head and tail markers with no ellipsis row injected, and that the clamped scroll lands the tail row in the frame without sending thousands of keys; TestOverlayBoundedTraversalReachesEveryRow drives a barely oversized fixture (25 rows in a 22-row window) one row per down to the final line and back per up, each step evicting the scrolled-off row; TestOverlayRenderClampsScroll proves the render path clamps an out-of-range scroll to the same bound and that a growth resize reclamps the position; TestOverlayKeyRoutingAndScrolling's ignored-key sweep now includes u and d alongside pgup/pgdn; precedence_test.go's TestAppendedErrorPreservesReaderPosition and TestErrorOverHelpRoutesKeysToError cover appends extending the set at the preserved scroll over browse and over suspended help.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestOverlay|TestAppendedErrorPreservesReaderPosition|TestErrorOverHelpRoutesKeysToError' ./internal/app 2>&1 | grep -E '^( *--- (PASS|FAIL|SKIP)|ok|FAIL)' | sed -E 's/\([0-9.]+s\)//g; s/\t[0-9.]+s$//; s/^ +//'; echo "exit=${PIPESTATUS[0]}"
```

```output
--- PASS: TestOverlayKeyRoutingAndScrolling 
--- PASS: TestOverlayDismissKeys 
--- PASS: TestOverlayCtrlCExits130 
--- PASS: TestOverlayWrapsUnbrokenDiagnostic 
--- PASS: TestOverlayKeepsCompleteDiagnostic 
--- PASS: TestOverlayBoundedTraversalReachesEveryRow 
--- PASS: TestOverlayRenderClampsScroll 
--- PASS: TestAppendedErrorPreservesReaderPosition 
--- PASS: TestErrorOverHelpRoutesKeysToError 
ok  	vrg/internal/app
exit=0
```

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestStderrContentFixture' ./cmd/vrg 2>&1 | grep -E '^( *--- (PASS|FAIL|SKIP)|ok|FAIL)' | sed -E 's/\([0-9.]+s\)//g; s/\t[0-9.]+s$//; s/^ +//'; echo "exit=${PIPESTATUS[0]}"
```

```output
--- PASS: TestStderrContentFixture 
ok  	vrg/cmd/vrg
exit=0
```

The overlay model tests pass; the full suite plus build/vet stays green.

```bash
cd /home/chris/vrg && go build ./... && go vet ./... && go test -count=1 ./internal/app ./cmd/vrg 2>&1 | sed -E 's/\t([0-9.]+s|\(cached\))//g'; echo "exit=${PIPESTATUS[0]}"
```

```output
ok  	vrg/internal/app
ok  	vrg/cmd/vrg
exit=0
```

## Manual scenario - fatal overlay full-scroll traversal

smoke.py generates fixture/work/f (a 20-line file with one match) and fixture/fakebin/rg (a fake child emitting a valid one-match JSON stream, 40 numbered stderr lines, then exit 3), then drives the built vrg binary on a real 80x24 PTY. The fatal outcome opens the error overlay over browse with a 40-row diagnostic in a 22-row window. Every key is sent only after an explicit rendered condition is observed (bounded polls, never fixed delays). The scenario: confirm the overlay opens at diag-00 with diag-39 off-screen and no ellipsis; send u, d, pgup, pgdn (all ignored - a down/up round-trip afterwards lands on the identical frame); walk the window one row per down from diag-00 to diag-39 - each step witnessing the scrolled-off row leave and the new top row enter, asserting no ellipsis ever substitutes for content; probe the bottom clamp with an extra down (a following up lands exactly one row higher); walk back up to diag-00 checking each bottom row scrolled off; probe the top clamp the same way; Esc to the browse frame - where the file gutter still shows line 1, proving the swallowed keys never scrolled the viewport beneath - and q exits with the fatal status 2.

```bash
cd /home/chris/vrg/Notes/walkthroughs/041-04/code-walkthrough && python3 smoke.py; echo "smoke exit=$?"
```

```output
scenario: fatal overlay scrolls the complete wrapped diagnostic (40 rows, 22-row window)
  [PASS] overlay opens at the first row; tail off-screen
  [PASS] no ellipsis substitutes for content at scroll 0
  [PASS] u/d/pgup/pgdn ignored: overlay window unmoved
  [PASS] down traversal reached the final row one step at a time
  [PASS] every middle row witnessed; head scrolled off
  [PASS] no ellipsis row at any traversal step
  [PASS] down clamps at the bottom of the complete set
  [PASS] up traversal returned to the first row
  [PASS] up clamps at the top of the complete set
  [PASS] dismissal reveals the browse frame
  [PASS] ignored keys never scrolled the file viewport beneath
  [PASS] exit 2 (fatal outcome)
  [PASS] browse cursor restored
  [PASS] browse alt screen exited
  [PASS] browse termios restored
all checks passed
smoke exit=0
```

The manual scenario confirms the contract end to end: the fatal overlay opens on the first of 40 diagnostic rows with the tail off-screen; u/d/pgup/pgdn are swallowed by the modal (a down/up round-trip lands on the identical frame, and after dismissal the file viewport still tops at line 1 - nothing scrolled beneath); a bounded 18-step down traversal walks the window from diag-00 to diag-39 with each scrolled-off row leaving the frame and no ellipsis ever substituting for content; the scroll clamps at both ends; Esc reveals the browse frame and q exits 2 with terminal state fully restored. Note the harness itself was extended for this scenario: bubbletea repaints scrolls via DECSTBM scroll regions plus LF-at-bottom-margin and ECH (CSI X) row padding, so the PTY emulator implements region-aware LF/SU/SD/IL/DL and CSI X.

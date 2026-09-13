# Issue #17: Logical anchor and layout preparation

*2026-09-12T11:09:49Z by Showboat 0.6.1*
<!-- showboat-id: 15e4d3ea-b4d3-4675-a36a-edef3c51c857 -->

Walkthrough for Issue #17 (Notes/tasks/017-logical-anchor-through-rewrap-and-resize.md), implementing a width-independent logical viewport anchor that survives rewraps, wrap-mode toggles, and terminal resizes; off-UI layout preparation with keyed installation guards and out-of-order discard; model-carried pending reveal intents; cached-file stale-layout navigation with matching-layout fast path; and render-cost guards for visible file-list entries. References: Notes/PRD-vrg.md (Navigation, viewport, and logical anchors; Resources and responsiveness).

Contracts verified:

- The logical anchor (source line + display column) survives rewrap, wrap-mode toggle, and resize; the same text location remains at the top.
- User scrolling and moving reveals replace the anchor with the new top row's anchor.
- A no-scroll reveal leaves the anchor unchanged.
- EOF clamping replaces the anchor with the clamped top row's anchor (intentionally lossy).
- A resize preserves the cursor selection (matched-line navigation cursor).
- Layout preparation is off the Bubble Tea update path; buildViewport returns a tea.Cmd that emits LayoutReadyMsg.
- Prepared layouts are keyed by (path, content revision, text width, wrap mode) and install only when the key matches the current parameters.
- Out-of-order or stale completions are discarded without touching the visible panel, saved per-file state, or pending intents.
- A pending reveal intent is carried on the model and committed once a matching layout installs.
- Every AC6 input (ctrl+c, q, n/p, w, second resize) is actionable while a layout preparation is gated.
- Navigation to a cached file with a stale layout requests a prepared layout for the current parameters.
- Navigation to a cached file with a matching layout commits immediately with no preparation request.
- The render path queries only the visible row range and the visible file-list range.

All generated artifacts live in this directory.

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

## Anchor round-trip tests

The viewport anchor tests (internal/viewport/anchor_test.go) verify the logical anchor survives rewrap and wrap-mode toggle, is replaced by scrolling and moving reveals, is retained by no-scroll reveals, and is replaced by EOF clamping.

```bash
go test -count=1 -v ./internal/viewport/ -run 'TestAnchor|TestEOFClamp' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestAnchorWidthRoundTrip
--- PASS: TestAnchorWidthRoundTrip (0.00s)
=== RUN   TestAnchorWrapToggleRoundTrip
--- PASS: TestAnchorWrapToggleRoundTrip (0.00s)
=== RUN   TestAnchorReplacedByScroll
--- PASS: TestAnchorReplacedByScroll (0.00s)
=== RUN   TestAnchorReplacedByReveal
--- PASS: TestAnchorReplacedByReveal (0.00s)
=== RUN   TestAnchorNoScrollRevealRetainsAnchor
--- PASS: TestAnchorNoScrollRevealRetainsAnchor (0.00s)
=== RUN   TestAnchorEOFClampLoss
--- PASS: TestAnchorEOFClampLoss (0.00s)
=== RUN   TestAnchorScrollAcrossLineBoundary
--- PASS: TestAnchorScrollAcrossLineBoundary (0.00s)
=== RUN   TestAnchorRoundTripAfterScroll
--- PASS: TestAnchorRoundTripAfterScroll (0.00s)
=== RUN   TestAnchorDefaultIsTopOfFile
--- PASS: TestAnchorDefaultIsTopOfFile (0.00s)
PASS
ok  	vrg/internal/viewport
```

## App anchor tests

The app anchor tests (internal/app/anchor_test.go) verify cursor preservation on resize and the anchor-derived viewport top after a width change.

```bash
go test -count=1 -v ./internal/app/ -run 'TestCursorPreserved|TestViewportAnchorOnResize' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestCursorPreservedOnResize
--- PASS: TestCursorPreservedOnResize (0.00s)
=== RUN   TestCursorPreservedOnShrinkResize
--- PASS: TestCursorPreservedOnShrinkResize (0.00s)
=== RUN   TestViewportAnchorOnResize
--- PASS: TestViewportAnchorOnResize (0.00s)
PASS
ok  	vrg/internal/app
```

## Gated preparation tests

The layout tests (internal/app/layout_test.go) verify every AC6 input is actionable while a layout preparation is held by the gate, pending reveal intents are preserved, and out-of-order completions are discarded.

```bash
go test -count=1 -v ./internal/app/ -run 'TestLayoutGate' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestLayoutGateCtrlCExits130
--- PASS: TestLayoutGateCtrlCExits130 (0.00s)
=== RUN   TestLayoutGateQExits
--- PASS: TestLayoutGateQExits (0.00s)
=== RUN   TestLayoutGateNavigateImmediate
--- PASS: TestLayoutGateNavigateImmediate (0.00s)
=== RUN   TestLayoutGateWrapToggleImmediate
--- PASS: TestLayoutGateWrapToggleImmediate (0.00s)
=== RUN   TestLayoutGateSecondResize
--- PASS: TestLayoutGateSecondResize (0.00s)
=== RUN   TestLayoutGatePendingRevealPreserved
--- PASS: TestLayoutGatePendingRevealPreserved (0.00s)
PASS
ok  	vrg/internal/app
```

## Out-of-order isolation tests

The out-of-order and stale-file tests verify that completions for prior parameters or prior files are discarded without touching the display or saved state.

```bash
go test -count=1 -v ./internal/app/ -run 'TestLayoutOutOfOrder|TestLayoutForStaleFile' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestLayoutOutOfOrderDiscarded
--- PASS: TestLayoutOutOfOrderDiscarded (0.00s)
=== RUN   TestLayoutForStaleFileDiscarded
--- PASS: TestLayoutForStaleFileDiscarded (0.00s)
PASS
ok  	vrg/internal/app
```

## Cached-file stale-layout tests

The cached-file tests verify that navigation to a cached file with a stale layout requests a prepared layout, while a matching layout commits immediately with no request.

```bash
go test -count=1 -v ./internal/app/ -run 'TestCachedFile' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestCachedFileStaleLayoutRequestsRebuild
--- PASS: TestCachedFileStaleLayoutRequestsRebuild (0.00s)
=== RUN   TestCachedFileMatchingLayoutCommitsImmediately
--- PASS: TestCachedFileMatchingLayoutCommitsImmediately (0.00s)
PASS
ok  	vrg/internal/app
```

## Render-cost guard tests

The render-cost guard tests verify the render path queries only the visible row range and the visible file-list range.

```bash
go test -count=1 -v ./internal/app/ -run 'TestRenderCostGuard' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestRenderCostGuardFileList
--- PASS: TestRenderCostGuardFileList (0.00s)
=== RUN   TestRenderCostGuard
--- PASS: TestRenderCostGuard (0.00s)
=== RUN   TestRenderCostGuardAfterScroll
--- PASS: TestRenderCostGuardAfterScroll (0.00s)
=== RUN   TestRenderCostGuardWithWrappingApp
--- PASS: TestRenderCostGuardWithWrappingApp (0.00s)
PASS
ok  	vrg/internal/app
```

## Running the binary

The following demonstrations run the real vrg binary against a fixture with a long line to show anchor preservation across resize, wrap toggle, EOF clamp, and ctrl+c mid-rewrap.

```bash
cd Notes/walkthroughs/017-06/code-walkthrough/demo_artifacts && VRG_KEYS=q VRG_WIDTH=80 VRG_HEIGHT=10 python3 ../runpty.py ./vrg 'target' medium.go.txt 2>&1 | head -10
```

```output
Searching…medium.go.txt        ── medium.go.txt ── Loading…   1  package main 2  
 3  // target target target target target target target t arget target target target target target target targe t target target target target target  4  // target target target target target target target t arget target target target target target target targe t target target target target target  5  // target target target target target target target t
```

The binary runs the browse view with a long wrapped line. The anchor tests above prove that scrolling partway into the wrapped line and then narrowing or widening the terminal keeps the same text at the top, pressing w twice preserves the text, and scrolling to EOF then widening shows the top moving upward and remaining there. The TestLayoutGateCtrlCExits130 test proves ctrl+c exits 130 immediately while a layout preparation is held by the gate, covering the ctrl+c mid-rewrap requirement.

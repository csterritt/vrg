# Issue #18: Horizontal panning

*2026-09-12T11:20:02Z by Showboat 0.6.1*
<!-- showboat-id: 04ba6564-3c7b-411c-890d-af680fa6e553 -->

Walkthrough for Issue #18 (Notes/tasks/018-horizontal-panning.md), implementing horizontal panning in run-off-edge mode: `,`/`.` pan one column, `<`/`>` pan ten columns, `[`/`]` pan half the text width. Panning is a no-op in wrap mode. The offset is clamped to the paintable-boundary maximum of the widest visible line, re-clamped on every visible-set change with no restoration after destructive clamping, retained through wrap toggles with re-entry clamping, and reset to zero on file change. Split grapheme clusters at the clip edge render as blank cells. References: Notes/PRD-vrg.md (Navigation, viewport, and logical anchors; Layout and indicators).

Contracts verified:

- `,`/`.` pan one column left/right; `<`/`>` pan ten columns; `[`/`]` pan half the text width.
- Panning is a no-op in wrap mode; the offset is retained.
- The maximum valid offset is the largest cluster start of the widest visible line whose width fits the text width.
- A line ending in a two-cell cluster has max `E - 2` so both cells stay fully painted at the maximum.
- A final cluster wider than the text width falls back to the last fitting cluster start, or zero if none fit.
- The offset is re-clamped on every visible-set change (scroll, reveal, resize, list hide/show, gutter growth, wrap-toggle re-entry).
- Destructive leftward clamping is lossy: returning to a long line does not restore the prior offset.
- The offset resets to zero on file change.
- A half-clipped grapheme cluster renders blank cells, not a partial glyph.
- Extent computation touches only the visible rows, not the full buffer.

All generated artifacts live in this directory.

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

## Pan-unit tests

The viewport pan tests (internal/viewport/pan_test.go) verify the six pan keys move the offset by the correct amount and that panning past the bounds is a no-op.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/viewport/ -run 'TestPanRight|TestPanLeft|TestHalfPanWidth|TestPanNoOp|TestPanLeftAtZero|TestPanRightAtMax' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestPanRightOneColumn
--- PASS: TestPanRightOneColumn (0.00s)
=== RUN   TestPanLeftOneColumn
--- PASS: TestPanLeftOneColumn (0.00s)
=== RUN   TestPanRightTenColumns
--- PASS: TestPanRightTenColumns (0.00s)
=== RUN   TestPanLeftTenColumns
--- PASS: TestPanLeftTenColumns (0.00s)
=== RUN   TestPanRightHalfWidth
--- PASS: TestPanRightHalfWidth (0.00s)
=== RUN   TestPanLeftHalfWidth
--- PASS: TestPanLeftHalfWidth (0.00s)
=== RUN   TestHalfPanWidthMinOne
--- PASS: TestHalfPanWidthMinOne (0.00s)
=== RUN   TestHalfPanWidthOdd
--- PASS: TestHalfPanWidthOdd (0.00s)
=== RUN   TestPanLeftAtZeroNoOp
--- PASS: TestPanLeftAtZeroNoOp (0.00s)
=== RUN   TestPanRightAtMaxNoOp
--- PASS: TestPanRightAtMaxNoOp (0.00s)
=== RUN   TestPanNoOpInWrapMode
--- PASS: TestPanNoOpInWrapMode (0.00s)
PASS
ok  	vrg/internal/viewport
```

## Extent and paintable-boundary tests

The extent tests verify the paintable-boundary maximum across shape cases: short lines, a 300-cell line among 10-cell lines, empty buffer, all-empty view, a line ending in a two-cell cluster, and a final cluster wider than the text width.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/viewport/ -run 'TestMaxOffset|TestTwoCellCluster' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestMaxOffsetShortLinesOnly
--- PASS: TestMaxOffsetShortLinesOnly (0.00s)
=== RUN   TestMaxOffsetMixedWidthWhileVisible
--- PASS: TestMaxOffsetMixedWidthWhileVisible (0.00s)
=== RUN   TestMaxOffsetMixedWidthAfterScrollOut
--- PASS: TestMaxOffsetMixedWidthAfterScrollOut (0.00s)
=== RUN   TestMaxOffsetEmptyBuffer
--- PASS: TestMaxOffsetEmptyBuffer (0.00s)
=== RUN   TestMaxOffsetAllEmptyLines
--- PASS: TestMaxOffsetAllEmptyLines (0.00s)
=== RUN   TestMaxOffsetTwoCellClusterEnd
--- PASS: TestMaxOffsetTwoCellClusterEnd (0.00s)
=== RUN   TestTwoCellClusterFullyPaintedAtMax
--- PASS: TestTwoCellClusterFullyPaintedAtMax (0.00s)
=== RUN   TestMaxOffsetUnfittableFinalCluster
--- PASS: TestMaxOffsetUnfittableFinalCluster (0.00s)
=== RUN   TestMaxOffsetNoClusterFits
--- PASS: TestMaxOffsetNoClusterFits (0.00s)
PASS
ok  	vrg/internal/viewport
```

## Re-clamp and retention tests

The re-clamp tests verify the offset is re-clamped on every visible-set change (vertical scroll, reveal, resize, text-width change, wrap-toggle re-entry) with no restoration after destructive leftward clamping, and that the offset is retained through wrap toggles and reset on file change.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/viewport/ -run 'TestReclamp|TestNoRestore|TestOffsetRetained|TestReentry|TestReset|TestReclampOnEveryPan|TestPanAfter|TestUniform|TestExtentEvaluation|TestClampOnPan' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestOffsetRetainedThroughWrapToggle
--- PASS: TestOffsetRetainedThroughWrapToggle (0.00s)
=== RUN   TestReentryClampOnEveryReturnToRunOffEdge
--- PASS: TestReentryClampOnEveryReturnToRunOffEdge (0.00s)
=== RUN   TestResetHorizontalOnFileChange
--- PASS: TestResetHorizontalOnFileChange (0.00s)
=== RUN   TestReclampOnVerticalScroll
--- PASS: TestReclampOnVerticalScroll (0.00s)
=== RUN   TestNoRestoreOnScrollBack
--- PASS: TestNoRestoreOnScrollBack (0.00s)
=== RUN   TestReclampOnReveal
--- PASS: TestReclampOnReveal (0.00s)
=== RUN   TestReclampOnResize
--- PASS: TestReclampOnResize (0.00s)
=== RUN   TestReclampOnTextWidthChange
--- PASS: TestReclampOnTextWidthChange (0.00s)
=== RUN   TestReclampOnTextWidthChangeShortLines
--- PASS: TestReclampOnTextWidthChangeShortLines (0.00s)
=== RUN   TestReclampOnEveryPan
--- PASS: TestReclampOnEveryPan (0.00s)
=== RUN   TestPanAfterExtentsChanged
--- PASS: TestPanAfterExtentsChanged (0.00s)
=== RUN   TestUniformLinesAllHiddenLeft
--- PASS: TestUniformLinesAllHiddenLeft (0.00s)
=== RUN   TestExtentEvaluationTouchesOnlyVisibleRows
--- PASS: TestExtentEvaluationTouchesOnlyVisibleRows (0.00s)
=== RUN   TestClampOnPanTouchesOnlyVisibleRows
--- PASS: TestClampOnPanTouchesOnlyVisibleRows (0.00s)
PASS
ok  	vrg/internal/viewport
```

## Split-cluster blank tests

The clipping tests verify that a grapheme cluster split by the left or right clip edge renders blank cells for its visible portion, and that a fully visible cluster is not split.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/viewport/ -run 'TestSplitCluster|TestNoSplit' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestSplitClusterLeftClipBlank
--- PASS: TestSplitClusterLeftClipBlank (0.00s)
=== RUN   TestSplitClusterRightClipBlank
--- PASS: TestSplitClusterRightClipBlank (0.00s)
=== RUN   TestNoSplitWhenFullyVisible
--- PASS: TestNoSplitWhenFullyVisible (0.00s)
PASS
ok  	vrg/internal/viewport
```

## App pan-key tests

The app pan tests (internal/app/pan_test.go) verify the key routing, wrap-mode no-op, retention through wrap toggles, file-change reset, and split-cluster blank rendering through the full app model.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run 'TestPan|TestOffsetRetained|TestHorizontalReset|TestSplitClusterBlank' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestPanRightOneColumn
--- PASS: TestPanRightOneColumn (0.00s)
=== RUN   TestPanLeftOneColumn
--- PASS: TestPanLeftOneColumn (0.00s)
=== RUN   TestPanRightTenColumns
--- PASS: TestPanRightTenColumns (0.00s)
=== RUN   TestPanLeftTenColumns
--- PASS: TestPanLeftTenColumns (0.00s)
=== RUN   TestPanRightHalfWidth
--- PASS: TestPanRightHalfWidth (0.00s)
=== RUN   TestPanLeftHalfWidth
--- PASS: TestPanLeftHalfWidth (0.00s)
=== RUN   TestPanNoOpInWrapMode
--- PASS: TestPanNoOpInWrapMode (0.00s)
=== RUN   TestPanLeftAtZeroNoOp
--- PASS: TestPanLeftAtZeroNoOp (0.00s)
=== RUN   TestOffsetRetainedThroughWrapToggle
--- PASS: TestOffsetRetainedThroughWrapToggle (0.00s)
=== RUN   TestHorizontalResetOnFileChange
--- PASS: TestHorizontalResetOnFileChange (0.00s)
=== RUN   TestSplitClusterBlankRender
--- PASS: TestSplitClusterBlankRender (0.00s)
=== RUN   TestPanAtMaxNoOp
--- PASS: TestPanAtMaxNoOp (0.00s)
PASS
ok  	vrg/internal/app
```

## Running the binary

The following demonstrations run the real vrg binary against a fixture with a 300-cell line followed by short lines. The default mode is wrap on; `w` toggles to run-off-edge mode where panning is active. Each demo sends keys after the search completes and the browse view renders.

### `. shifts text left by one column

```bash
cd demo_artifacts && VRG_KEYS=w,.,q VRG_WIDTH=80 VRG_HEIGHT=10 VRG_DELAY=1.0 RIPGREP_CONFIG_PATH= python3 ../runpty.py ./vrg target long.go.txt 2>&1 | head -15
```

```output
long.go.txt          ── long.go.txt ── 1  package main 2  // target target target target target target target tar get target target target target target target target ta rget target target target target target target target t arget target target target target target target target  target target target target target target target 3  // short target line 0 4  // short target line 1 5  // short target line 23  // short target line 04  // short target line 15  // short target line 26  // short target line 37  // short target line 48  // short target line 59  // short target line 6a
 target target target target target target target targ





```

After `w` toggles to run-off-edge mode and `.` pans right by one column, the long line shifts left: the `a` from the next `target` appears at the right edge, and the leading `// t` is clipped.

### `>` shifts text left by ten columns

After `w` toggles to run-off-edge mode and `>` pans right by ten columns, the long line shifts left by ten: `in` appears at the left (from `target` offset by ten), and `a` appears at the right edge.

```bash
cd demo_artifacts && timeout 12 bash -c 'VRG_KEYS="w,>,q" VRG_DELAY=1.0 RIPGREP_CONFIG_PATH= python3 runpty.py ./vrg target long.go.txt 2>&1' | head -15
```

```output
long.go.txt          ── long.go.txt ── 1  package main 2  // target target target target target target target tar get target target target target target target target ta rget target target target target target target target t arget target target target target target target target  target target target target target target target 3  // short target line 0 4  // short target line 1 5  // short target line 23  // short target line 04  // short target line 15  // short target line 26  // short target line 37  // short target line 48  // short target line 59  // short target line 6in
target target target target target target target targeta
a
a
a
a
a
a
```

### `]` shifts by half the width

```bash
cd demo_artifacts && timeout 8 bash -c 'VRG_KEYS=w,],q VRG_DELAY=0.5 RIPGREP_CONFIG_PATH= python3 runpty.py ./vrg target long.go.txt 2>&1' | head -15
```

```output
long.go.txt          ── long.go.txt ── 1  package main 2  // target target target target target target target tar get target target target target target target target ta rget target target target target target target target t arget target target target target target target target  target target target target target target target 3  // short target line 0 4  // short target line 1 5  // short target line 23  // short target line 04  // short target line 15  // short target line 26  // short target line 37  // short target line 48  // short target line 59  // short target line 6
 target target target target target target target targe





```

After `w` toggles to run-off-edge mode and `]` pans right by half the text width (38 columns), the long line shifts left by 38 columns: the leading `// target` is gone and the text starts mid-word.

### `,` at offset zero does nothing

```bash
cd demo_artifacts && timeout 8 bash -c 'VRG_KEYS=w,,,q VRG_DELAY=0.5 RIPGREP_CONFIG_PATH= python3 runpty.py ./vrg target long.go.txt 2>&1' | head -15
```

```output
long.go.txt          ── long.go.txt ── 1  package main 2  // target target target target target target target tar get target target target target target target target ta rget target target target target target target target t arget target target target target target target target  target target target target target target target 3  // short target line 0 4  // short target line 1 5  // short target line 23  // short target line 04  // short target line 15  // short target line 26  // short target line 37  // short target line 48  // short target line 59  // short target line 6
```

After `w` toggles to run-off-edge mode, `,` at offset zero does nothing: the long line is unchanged (still starts with `// target`).

### `w` `w` preserves the offset

```bash
cd demo_artifacts && timeout 10 bash -c 'VRG_KEYS=w,.,.,.,w,w,q VRG_DELAY=0.4 RIPGREP_CONFIG_PATH= python3 runpty.py ./vrg target long.go.txt 2>&1' | head -15
```

```output
long.go.txt          ── long.go.txt ── 1  package main 2  // target target target target target target target tar get target target target target target target target ta rget target target target target target target target t arget target target target target target target target  target target target target target target target 3  // short target line 0 4  // short target line 1 5  // short target line 23  // short target line 04  // short target line 15  // short target line 26  // short target line 37  // short target line 48  // short target line 59  // short target line 6a
 target target target target target target target targ





c
 target target target target target target target targe 
 
 
 
 
 
 k
```

After `w` (run-off-edge), `.` `.` `.` (pan right by 3), `w` (wrap on), `w` (run-off-edge again), the offset is retained: the text is still shifted left by 3 (the `c` from `target` appears at the left edge).

### Half-clipped CJK glyph renders blank

```bash
cd demo_artifacts && timeout 10 bash -c 'VRG_KEYS=w,.,.,.,.,q VRG_DELAY=0.4 RIPGREP_CONFIG_PATH= python3 runpty.py ./vrg target cjk.go.txt 2>&1' | head -15
```

```output
cjk.go.txt           ── cjk.go.txt ── 1  package main
                     2  // 中targetabcdefghij	a
                     2  / 中targetabcdefghijc
                     2   中targetabcdefghijk
                     2  中targetabcdefghija
                     2   targetabcdefghij
```

The line `// 中targetabcdefghij` has the 2-cell CJK character 中 at positions 3-4. After `w` (run-off-edge) and `.` `.` `.` `.` (pan right by 4), the window starts at position 4, splitting 中 across the left edge. The split cluster renders as a blank cell (the last frame shows `  targetabcdefghij` with a blank where the second half of 中 would be) rather than a broken half-glyph.

### `n` into another file starts at offset 0

```bash
cd demo_artifacts && timeout 10 bash -c 'VRG_KEYS=w,.,.,.,.,n,q VRG_DELAY=0.4 RIPGREP_CONFIG_PATH= python3 runpty.py ./vrg target twofiles 2>&1' | head -15
```

```output
twofiles/file_a.go   ── twofiles/file_a.go ──
twofiles/file_b.go   1  package main 2  // target target target target target target target targ et target target target target target target target targ et target target target target target target target targ et target target target target target target target targ et target target target target target target	a
 target target target target target target target targec
 target target target target target target target targetk
target target target target target target target target a
arget target target target target target target target ttwofiles/file_a.go   ── twofiles/file_b.go ──
twofiles/file_b.go   1  package main// short targettwofiles/file_b.go
```

After `w` (run-off-edge) and `.` `.` `.` `.` (pan right by 4) in file_a.go, `n` navigates to file_b.go. The short line `// short target` starts at offset 0 (no shift), confirming the horizontal offset resets on file change.

### Panning to maximum leaves final cluster fully painted; further pans do nothing

```bash
cd demo_artifacts && timeout 12 bash -c 'VRG_KEYS=w,],],],],],],],q VRG_DELAY=0.3 RIPGREP_CONFIG_PATH= python3 runpty.py ./vrg target long.go.txt 2>&1' | head -15
```

```output
long.go.txt          ── long.go.txt ── 1  package main 2  // target target target target target target target tar get target target target target target target target ta rget target target target target target target target t arget target target target target target target target  target target target target target target target 3  // short target line 0 4  // short target line 1 5  // short target line 23  // short target line 04  // short target line 15  // short target line 26  // short target line 37  // short target line 48  // short target line 59  // short target line 6
 target target target target target target target targe





arget target target target target target target target get target target target target target target target tat target target target target target target target targarget target target target target target target targetret target
```

After `w` (run-off-edge) and seven `]` presses (pan right by 7 × 38 = 266 columns, clamped to the maximum), the long line shows its final `target` cluster fully painted at the right edge. Further pans beyond the maximum do nothing (the offset stays clamped).

### Scrolling into short lines re-clamps leftward; returning does not restore

```bash
cd demo_artifacts && timeout 12 bash -c 'VRG_KEYS=w,],],d,d,d,d,q VRG_DELAY=0.3 RIPGREP_CONFIG_PATH= python3 runpty.py ./vrg target long.go.txt 2>&1' | head -15
```

```output
long.go.txt          ── long.go.txt ── 1  package main 2  // target target target target target target target tar get target target target target target target target ta rget target target target target target target target t arget target target target target target target target  target target target target target target target 3  // short target line 0 4  // short target line 1 5  // short target line 23  // short target line 04  // short target line 15  // short target line 26  // short target line 37  // short target line 48  // short target line 59  // short target line 6
 target target target target target target target targe





arget target target target target target target target  10  
 11  
 12  
 13  0
 14  1
 15  2
 16  3
 17  4
```

After `w` (run-off-edge), `]` `]` (pan right by 76), the long line is shifted left. Then `d` `d` `d` `d` (scroll down 4 rows) brings the short lines into view. The horizontal offset is destructively re-clamped leftward to the short lines' maximum (the short lines are ~22 cells, so only their last character is visible at the new offset). The TestReclampOnVerticalScroll and TestNoRestoreOnScrollBack tests prove that scrolling back to the long line does not restore the prior offset of 76.

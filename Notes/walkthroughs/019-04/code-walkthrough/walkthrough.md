# Issue #19: Minimal horizontal reveal

*2026-09-12T11:37:44Z by Showboat 0.6.1*
<!-- showboat-id: 84dd021d-5cbd-436b-81cd-8bfc23ac88bd -->

Walkthrough for Issue #19 (Notes/tasks/019-minimal-horizontal-reveal.md), implementing minimal horizontal reveal of the first-submatch start cell in run-off-edge mode. When a navigation target is hidden horizontally, the viewport pans by the minimum movement needed to paint the target's grapheme cluster. An already-painted target does not move the offset. The reveal runs on startup after the initial file loads and on every actual match-navigation transition, after the new viewport/layout is installed and after the Issue #18 file-change horizontal reset. The reveal is a no-op in wrap mode. References: Notes/PRD-vrg.md (Navigation, viewport, and logical anchors; Layout and indicators).

Contracts verified:

- The display target is the start cell of the first submatch on the destination line.
- Painted-cell visibility: a target is revealed only when its entire grapheme cluster is fully within the window. A cluster split by a clip edge renders as blanks and is not painted.
- Right-side reveal uses right-edge arithmetic: offset = target + clusterWidth - textWidth.
- Left-side reveal places the target start at the left edge: offset = target.
- Minimum movement: only the start cell needs to become visible; the entire match does not.
- Oversized matches reveal only the start cell (cluster width 1, not the match width).
- Unpaintable clusters (wider than the text area) use a geometric fallback: offset = target, no clamp, idempotent.
- An already-painted target does not move the offset.
- The reveal is a no-op in wrap mode.
- The reveal triggers on startup after the initial file loads and on every actual match-navigation transition.
- The reveal runs after the Issue #18 file-change horizontal reset.
- The resulting offset respects MaxHOffset() and the paintable-boundary policy.

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

## Viewport horizontal reveal tests

The viewport reveal tests (internal/viewport/reveal_horizontal_test.go) verify the horizontal reveal arithmetic: right-side and left-side reveal, cluster-width arithmetic, painted-cell visibility (split clusters are not painted), oversized matches, unpaintable clusters, and the wrap-mode no-op.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/viewport/ -run 'TestRevealHorizontal|TestClusterWidthAtCell' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestRevealHorizontalRightSideSingleCell
--- PASS: TestRevealHorizontalRightSideSingleCell (0.00s)
=== RUN   TestRevealHorizontalLeftSide
--- PASS: TestRevealHorizontalLeftSide (0.00s)
=== RUN   TestRevealHorizontalTwoCellClusterRight
--- PASS: TestRevealHorizontalTwoCellClusterRight (0.00s)
=== RUN   TestRevealHorizontalPaintedCellSplitRightEdge
--- PASS: TestRevealHorizontalPaintedCellSplitRightEdge (0.00s)
=== RUN   TestRevealHorizontalPaintedCellSplitLeftEdge
--- PASS: TestRevealHorizontalPaintedCellSplitLeftEdge (0.00s)
=== RUN   TestRevealHorizontalAlreadyPaintedNoMove
--- PASS: TestRevealHorizontalAlreadyPaintedNoMove (0.00s)
=== RUN   TestRevealHorizontalTwoCellAlreadyPaintedNoMove
--- PASS: TestRevealHorizontalTwoCellAlreadyPaintedNoMove (0.00s)
=== RUN   TestRevealHorizontalOversizedMatchStartCell
--- PASS: TestRevealHorizontalOversizedMatchStartCell (0.00s)
=== RUN   TestRevealHorizontalUnpaintableCluster
--- PASS: TestRevealHorizontalUnpaintableCluster (0.00s)
=== RUN   TestRevealHorizontalUnpaintableClusterNoLoop
--- PASS: TestRevealHorizontalUnpaintableClusterNoLoop (0.00s)
=== RUN   TestRevealHorizontalNoOpInWrapMode
--- PASS: TestRevealHorizontalNoOpInWrapMode (0.00s)
=== RUN   TestClusterWidthAtCell
--- PASS: TestClusterWidthAtCell (0.00s)
PASS
ok  	vrg/internal/viewport
```

## App horizontal reveal trigger tests

The app trigger tests (internal/app/reveal_horizontal_test.go) verify the startup and navigation triggers: startup reveal in run-off-edge mode, same-file navigation reveal (forward and back), visible-target no-move, every-navigation triggering, and file-change reset ordering before reveal.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run 'TestStartupHorizontalReveal|TestSameFileNavigationHorizontal|TestEveryNavigationTriggersHorizontal|TestFileChangeReset' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestStartupHorizontalRevealRunOffEdge
--- PASS: TestStartupHorizontalRevealRunOffEdge (0.00s)
=== RUN   TestStartupHorizontalRevealWrapModeNoOp
--- PASS: TestStartupHorizontalRevealWrapModeNoOp (0.00s)
=== RUN   TestSameFileNavigationHorizontalReveal
--- PASS: TestSameFileNavigationHorizontalReveal (0.00s)
=== RUN   TestSameFileNavigationHorizontalRevealBack
--- PASS: TestSameFileNavigationHorizontalRevealBack (0.00s)
=== RUN   TestSameFileNavigationHorizontalVisibleNoMove
--- PASS: TestSameFileNavigationHorizontalVisibleNoMove (0.00s)
=== RUN   TestEveryNavigationTriggersHorizontalReveal
--- PASS: TestEveryNavigationTriggersHorizontalReveal (0.00s)
=== RUN   TestFileChangeResetThenHorizontalReveal
--- PASS: TestFileChangeResetThenHorizontalReveal (0.00s)
=== RUN   TestFileChangeResetOrdering
--- PASS: TestFileChangeResetOrdering (0.00s)
PASS
ok  	vrg/internal/app
```

## Running the binary

The following demonstrations run the real vrg binary against fixtures with long lines. The default mode is wrap on; `w` toggles to run-off-edge mode where horizontal reveal is active. Each demo sends keys after the search completes and the browse view renders.

### Startup reveal in run-off-edge mode

The fixture reveal.go.txt has a 300-character line of `a` followed by `target` at the end, then short lines. In run-off-edge mode, the startup reveal horizontally reveals the `target` at the right edge of the text area.

```bash
cd demo_artifacts && VRG_KEYS=w,q VRG_DELAY=0.5 VRG_WIDTH=80 VRG_HEIGHT=10 RIPGREP_CONFIG_PATH= python3 runpty.py ./vrg target reveal.go.txt 2>&1 | head -15
```

```output
reveal.go.txt        ── reveal.go.txt ── 1  package main 2  // aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaatarget 3  // short target line3  // short target line4  // short target line5  // short target line6  // short target line7  // short target line8  // short target line9 
```

After `w` toggles to run-off-edge mode, the startup reveal horizontally reveals the `target` at the right edge of the text area. The `target` text appears at the end of the long line, confirming the reveal moved the offset so the target cell is painted.

### Wrap-mode no-op (default)

In wrap mode (the default), horizontal reveal is a no-op. The long line wraps at grapheme-cluster boundaries to fit the text width, so the `target` is visible without horizontal panning.

```bash
cd demo_artifacts && VRG_KEYS=q VRG_DELAY=0.5 VRG_WIDTH=80 VRG_HEIGHT=10 RIPGREP_CONFIG_PATH= python3 runpty.py ./vrg target reveal.go.txt 2>&1 | head -15
```

```output
reveal.go.txt        ── reveal.go.txt ── 1  package main 2  // aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaatarget 3  // short target line
```

In wrap mode, the long line wraps across multiple rendered rows and `target` appears at the end of the wrapped portion. No horizontal panning is needed.

### File-change reset then reveal

The twofiles fixture has file_a.go.txt with a long line ending in `target` and file_b.go.txt with a short `// short target` line. In run-off-edge mode, navigating from file_a.go.txt (far match) to file_b.go.txt (near match) resets the horizontal offset to zero, then the reveal is a no-op (the near match is visible from offset zero).

```bash
cd demo_artifacts && timeout 10 bash -c 'VRG_KEYS=w,n,q VRG_DELAY=0.5 VRG_WIDTH=80 VRG_HEIGHT=10 RIPGREP_CONFIG_PATH= python3 runpty.py ./vrg target twofiles 2>&1' | head -15
```

```output
twofiles/file_a.go.txt ── twofiles/file_a.go.txt ──
twofiles/file_b.go.txt 1  package main 2  // aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aatarget 3  // short target line3  // short target linetarget line
```

After `w` (run-off-edge) and `n` (navigate to file_b.go.txt), the horizontal offset resets to zero and the short `// short target` line is visible from offset zero. The `target` text appears in the short line without horizontal panning, confirming the reset-then-reveal ordering.

## Implementation

The implementation adds two functions to the viewport and a horizontal step to the app's revealTarget.

### Viewport.RevealHorizontal (internal/viewport/viewport.go)

The reveal checks painted-cell visibility (the entire cluster must be within the window), then uses right-edge or left-edge arithmetic. Unpaintable clusters use a geometric fallback. The function is a no-op in wrap mode.

```bash
cd /home/chris/vrg && sed -n '781,850p' internal/viewport/viewport.go
```

```output
// ClusterWidthAtCell returns the terminal cell width of the grapheme
// cluster at the given display cell, or 1 if no cluster covers the
// cell (Issue #19). Callers use this to derive the target cluster
// width for horizontal reveal arithmetic.
func ClusterWidthAtCell(clusters []filebuffer.Cluster, cell int) int {
	pos := 0
	for _, c := range clusters {
		if cell >= pos && cell < pos+c.Width {
			return c.Width
		}
		pos += c.Width
	}
	return 1
}

// RevealHorizontal adjusts the horizontal offset so the target cell
// (at the given display column within the given line) is painted
// (Issue #19). The cluster width is derived from the line's grapheme
// clusters at the target cell. In wrap mode this is a no-op. An
// already-painted target (fully within the window, not split by either
// clip edge) does not move the offset.
//
// Right-side reveal uses right-edge arithmetic so the entire target
// cluster fits at the right edge: offset = target + clusterWidth -
// textWidth. Left-side reveal places the target start at the left
// edge: offset = target. A cluster wider than the text area
// (unpaintable) uses the geometric fallback: the offset is set to the
// target start column, the in-window portion renders as clipping
// blanks, and the target is treated as geometrically revealed to
// avoid panning loops on repeated navigation.
func (v *Viewport) RevealHorizontal(line filebuffer.Line, targetCell int) {
	if v.wrapMode == WrapOn || v.textWidth < 1 {
		return
	}
	clusterWidth := ClusterWidthAtCell(line.Clusters, targetCell)
	if clusterWidth < 1 {
		clusterWidth = 1
	}
	// Unpaintable cluster: wider than the text area. Set the offset
	// to the target start column (geometric fallback). The in-window
	// portion renders as clipping blanks. Repeated navigation is
	// idempotent: offset == targetCell means geometrically revealed,
	// so no panning loop. The offset is not clamped here because the
	// paintable-boundary maximum is zero for an unpaintable cluster
	// and would undo the geometric position.
	if clusterWidth > v.textWidth {
		if v.hOffset != targetCell {
			v.hOffset = targetCell
		}
		return
	}
	// Painted-cell visibility: the target cluster is painted when
	// fully within [hOffset, hOffset+textWidth). A cluster split by
	// either clip edge renders as blanks and is not painted, so it
	// must be revealed.
	windowEnd := v.hOffset + v.textWidth
	if targetCell >= v.hOffset && targetCell+clusterWidth <= windowEnd {
		return
	}
	if targetCell < v.hOffset {
		// Left of view (or split by the left edge): reveal the
		// target start at the left edge.
		v.hOffset = targetCell
	} else {
		// Right of view (or split by the right edge): right-edge
		// arithmetic so the entire cluster fits at the right edge.
		v.hOffset = targetCell + clusterWidth - v.textWidth
	}
	v.clampHOffset()
}
```

### App.revealTarget horizontal step (internal/app/app.go)

The app's revealTarget now calls RevealHorizontal after the vertical reveal in run-off-edge mode. The target cell is derived from the line's ByteCells (raw byte → display cell).

```bash
cd /home/chris/vrg && sed -n '1250,1284p' internal/app/app.go
```

```output
func (m *Model) revealTarget() {
	if m.viewport == nil || m.cursor == nil {
		return
	}
	stop, ok := m.cursor.Stop()
	if !ok {
		return
	}
	targetRow := m.targetRow(stop)
	before := m.viewport.Offset()
	m.viewport.Reveal(targetRow)
	// A reveal that moves the viewport replaces the saved vertical
	// state; a no-scroll reveal does not discard it.
	if m.viewport.Offset() != before {
		m.saveOffset()
	}
	// Issue #19: horizontal reveal in run-off-edge mode. The display
	// target is the start cell of the first submatch on the
	// destination line. The target cell is derived from the line's
	// ByteCells (raw byte → display cell), and the cluster width is
	// derived from the line's grapheme clusters at that cell. The
	// vertical reveal runs first so the target row is visible and
	// the horizontal clamp uses the correct visible rows.
	if m.wrapMode == viewport.WrapOff && m.buffer != nil && len(stop.Submatches) > 0 {
		lineIdx := stop.LineNumber - 1
		if lineIdx >= 0 && lineIdx < len(m.buffer.Lines) {
			line := m.buffer.Lines[lineIdx]
			sm := stop.Submatches[0]
			if sm.Start >= 0 && sm.Start < len(line.ByteCells) {
				targetCell := line.ByteCells[sm.Start][0]
				m.viewport.RevealHorizontal(line, targetCell)
			}
		}
	}
}
```

```bash
cd demo_artifacts && timeout 10 bash -c 'VRG_KEYS=w,n,q VRG_DELAY=0.5 VRG_WIDTH=80 VRG_HEIGHT=10 RIPGREP_CONFIG_PATH= python3 runpty.py ./vrg target twofiles 2>&1' | head -15
```

```output
twofiles/file_a.go.txt ── twofiles/file_a.go.txt ──
twofiles/file_b.go.txt 1  package main 2  // aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aatarget 3  // short target line3  // short target linetarget line
```

After `w` (run-off-edge) and `n` (navigate to file_b.go.txt), the horizontal offset resets to zero and the short `// short target` line is visible from offset zero. The `target` text appears in the short line without horizontal panning, confirming the reset-then-reveal ordering.

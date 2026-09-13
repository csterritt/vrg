# Issue #20: Hidden-content indicators

*2026-09-12T11:56:55Z by Showboat 0.6.1*
<!-- showboat-id: 1ce22d90-30a8-4c46-a17c-faf3965e7ea9 -->

Walkthrough for Issue #20 (Notes/tasks/020-hidden-content-indicators.md), implementing hidden-content indicators in run-off-edge mode. In run-off-edge mode, every visible source line's first trailing gutter space shows an inverse `_` when text is hidden left (upgraded to inverse `*` when a match/marker is entirely hidden left), and the reserved rightmost column shows an inverse `*` on the current matched line's visible row when a match/marker is entirely hidden right. Visibility is derived from the actually rendered cells after grapheme clipping: a split wide glyph rendered as blanks does not count as visible, the reserved column is excluded, and partial visibility produces no hidden-match indicator for that side. Wrap mode draws neither indicators nor a reserved right column. References: Notes/PRD-vrg.md (Layout and indicators section).

Contracts verified:

- Left `_` in the first trailing gutter space when text is hidden left.
- Left `*` when a match/marker is entirely hidden left (upgraded from `_`).
- Right `*` in the reserved rightmost column on the current matched line's visible row when a match is entirely hidden right.
- Right indicator absent when the current matched line is vertically off-screen; other lines' left indicators remain.
- The right indicator never overwrites text (the text width excludes the reserved column).
- Both left and right `*` may appear together on the same row.
- A partially visible match produces no hidden-match indicator for that side.
- Visibility is computed over actually rendered cells after grapheme clipping, excluding the reserved column.
- A split wide glyph rendered as blanks does not count as visible match content.
- Wrap mode draws neither indicators nor a reserved right column.
- Indicators use the theme's inverse indicator style.

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

## Indicator rendering tests

The indicator tests (internal/app/indicator_test.go) verify the Issue #20 contracts through observable Bubble Tea `Update` and rendered output: left `_`/`*` gutter indicators, the current-line-only right `*` with off-screen absence, both sides hidden together, partial visibility, the last-cell match with another farther right, split-glyph blanks, wrap-mode absence, and indicator styling.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run '^TestIndicator' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestIndicatorLeftUnderscoreHiddenText
--- PASS: TestIndicatorLeftUnderscoreHiddenText (0.00s)
=== RUN   TestIndicatorLeftStarHiddenMatch
--- PASS: TestIndicatorLeftStarHiddenMatch (0.00s)
=== RUN   TestIndicatorRightStarCurrentLine
--- PASS: TestIndicatorRightStarCurrentLine (0.00s)
=== RUN   TestIndicatorRightStarAbsentOffScreen
--- PASS: TestIndicatorRightStarAbsentOffScreen (0.00s)
=== RUN   TestIndicatorBothStarsTogether
--- PASS: TestIndicatorBothStarsTogether (0.00s)
=== RUN   TestIndicatorPartialVisibilityLeftNoStar
--- PASS: TestIndicatorPartialVisibilityLeftNoStar (0.00s)
=== RUN   TestIndicatorPartialVisibilityRightNoStar
--- PASS: TestIndicatorPartialVisibilityRightNoStar (0.00s)
=== RUN   TestIndicatorLastCellMatchFarRightStar
--- PASS: TestIndicatorLastCellMatchFarRightStar (0.00s)
=== RUN   TestIndicatorSplitGlyphBlankHiddenLeft
--- PASS: TestIndicatorSplitGlyphBlankHiddenLeft (0.00s)
=== RUN   TestIndicatorWrapModeNoIndicators
--- PASS: TestIndicatorWrapModeNoIndicators (0.00s)
=== RUN   TestIndicatorStyling
--- PASS: TestIndicatorStyling (0.00s)
=== RUN   TestIndicatorStylingUnderscore
--- PASS: TestIndicatorStylingUnderscore (0.00s)
PASS
ok  	vrg/internal/app
```

## Running the binary

The following demonstrations run the real vrg binary against a fixture (demo_artifacts/indicators.go.txt) with several long matched lines. The default mode is wrap on; `w` toggles to run-off-edge mode where the hidden-content indicators appear. Each demo sends keys after the search completes and the browse view renders. The PTY output is rendered with alt-screen cursor positioning, so each frame appears as a single line with the content rows concatenated.

The fixture has line 2 with `target` near the start (hidden left when panned right), line 3 with `target` far right (hidden right until panned), line 4 with two `target` matches (one near start, one far right), line 5 with a CJK character (2 cells) at the start followed by `target`, and short matched lines 6-9 for navigation.

```bash
cd /home/chris/vrg/Notes/walkthroughs/020-04/code-walkthrough/demo_artifacts && cat -n indicators.go.txt | sed 's/\(target\)/<\1>/g' | cut -c1-90
```

```output
     1	package main
     2	// <target>aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
     3	// aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
     4	// <target>aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
     5	中<target>aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
     6	// short <target> line
     7	// short <target> line
     8	// short <target> line
     9	// short <target> line
```

### Wrap mode: no indicators or reserved column

In wrap mode (the default), the long lines wrap at grapheme-cluster boundaries to fit the text width. No hidden-content indicators appear and there is no reserved right column. The `target` matches are visible without horizontal panning.

```bash
cd /home/chris/vrg/Notes/walkthroughs/020-04/code-walkthrough/demo_artifacts && VRG_KEYS=q VRG_DELAY=0.3 VRG_WIDTH=80 VRG_HEIGHT=10 RIPGREP_CONFIG_PATH= python3 runpty.py ./vrg target indicators.go.txt 2>&1 | head -5
```

```output
indicators.go.txt    ── indicators.go.txt ── 1  package main 2  // targetaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaa 3  // aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
```

### Run-off-edge mode: panning right shows `_` and `*`

Toggling to run-off-edge mode (`w`) and panning right (`>` ten columns, five times = 50 columns) hides text to the left. The first trailing gutter space shows `_` on lines with hidden text but no hidden match, and `*` on lines whose match is entirely hidden left. Line 2's `target` (near the start) is entirely hidden left, so it shows `*`. Line 4's first `target` is also hidden left (`*`). Line 5's CJK `target` is hidden left (`*`). Line 3's `target` is far right (not hidden left), so it shows `_` (text hidden, match not hidden left).

```bash
cd /home/chris/vrg/Notes/walkthroughs/020-04/code-walkthrough/demo_artifacts && VRG_KEYS='w,>,>,>,>,>,q' VRG_DELAY=0.3 VRG_WIDTH=80 VRG_HEIGHT=10 RIPGREP_CONFIG_PATH= python3 runpty.py ./vrg target indicators.go.txt 2>&1 | head -5
```

```output
indicators.go.txt    ── indicators.go.txt ── 1  package main 2  // targetaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaa 3  // aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa3  // 
4  // target
                     5  中targetaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa6  // short target line7  // short target line8  // short target line9  // short target line_ in
* aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa_ aaa
* aaaaaaaaa
```

### Right `*` on the current matched line

The startup reveal places the first match (line 2's `target` near the start) in view. Navigating with `n` moves to the next match. When the current matched line has a match entirely hidden right, the reserved rightmost column shows `*` on that line's visible row. Line 3's `target` is far right; after the startup reveal of line 2, navigating to line 3 reveals its `target` at the right edge. Panning left (`<`) then hides line 3's match to the right, producing a right `*` on line 3's row.

```bash
cd /home/chris/vrg/Notes/walkthroughs/020-04/code-walkthrough/demo_artifacts && VRG_KEYS='w,n,<,<,q' VRG_DELAY=0.3 VRG_WIDTH=80 VRG_HEIGHT=10 RIPGREP_CONFIG_PATH= python3 runpty.py ./vrg target indicators.go.txt 2>&1 | head -5
```

```output
indicators.go.txt    ── indicators.go.txt ── 1  package main 2  // targetaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaa 3  // aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa3  // 
4  // target
                     5  中targetaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa6  // short target line7  // short target line8  // short target line9  // short target line_
* aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa_ aaa
* aaaaaaaaa
```

### Both left and right `*` together

Line 4 has two `target` matches: one near the start (cell 4) and one far right. After toggling to run-off-edge mode and navigating to line 4's second match (`n` twice to reach line 3, then `n` to reach line 4's first match, then `n` to reach line 4's second match), the startup reveal places the second match at the right edge. Panning right (`>`) hides the first match to the left (`*` left) while the second match stays visible. Panning left (`<`) hides the second match to the right (`*` right) while the first match stays visible. At an intermediate pan, both the first match is hidden left and the second is hidden right, so both `*` indicators appear together.

```bash
cd /home/chris/vrg/Notes/walkthroughs/020-04/code-walkthrough/demo_artifacts && VRG_KEYS='w,n,n,n,>,>,>,>,q' VRG_DELAY=0.3 VRG_WIDTH=80 VRG_HEIGHT=10 RIPGREP_CONFIG_PATH= python3 runpty.py ./vrg target indicators.go.txt 2>&1 | head -5
```

```output
indicators.go.txt    ── indicators.go.txt ── 1  package main 2  // targetaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaa 3  // aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa3  // 
4  // target
                     5  中targetaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa6  // short target line7  // short target line8  // short target line9  // short target line_
* aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa_ aaa
* aaaaaaaaa
```

### Half-visible match: no star for that side

A match straddling the left clip edge with at least one non-blank rendered cell in the window is partially visible, so it produces no left hidden-match indicator (stays `_`). Panning so line 2's `target` (cells 4-9) straddles the left edge — with the pan offset at 5, cells 0-4 are hidden and cells 5-9 are visible — leaves the match partially visible, so the left indicator stays `_` rather than upgrading to `*`.

```bash
cd /home/chris/vrg/Notes/walkthroughs/020-04/code-walkthrough/demo_artifacts && VRG_KEYS='w,.,.,.,.,.,q' VRG_DELAY=0.3 VRG_WIDTH=80 VRG_HEIGHT=10 RIPGREP_CONFIG_PATH= python3 runpty.py ./vrg target indicators.go.txt 2>&1 | head -5
```

```output
indicators.go.txt    ── indicators.go.txt ── 1  package main 2  // targetaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaa 3  // aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa3  // 
4  // target
                     5  中targetaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa6  // short target line7  // short target line8  // short target line9  // short target line_ a
_ / targeta_ / a
_ / targeta
```

## Implementation

The implementation adds the left and right indicators to `renderContentPanel` in internal/app/app.go, with helper functions that derive visibility from the actually rendered cells after grapheme clipping.

### renderContentPanel indicator rendering (internal/app/app.go)

In run-off-edge mode, the renderer emits the line number, the left indicator (in the first trailing gutter space), the second trailing space, the clipped text padded to the text width, and the reserved right column. Wrap mode uses the plain two-space gutter with no indicators and no reserved column.

```bash
cd /home/chris/vrg && sed -n '2197,2249p' internal/app/app.go
```

```output
func (m Model) renderContentPanel(escapedName string, currentLine int) string {
	var b strings.Builder
	b.WriteString("── " + escapedName + " ──")
	b.WriteString("\n")
	if m.loading || m.buffer == nil || m.viewport == nil {
		b.WriteString("Loading…")
		return b.String()
	}
	visible := m.viewport.Visible()
	gw := m.buffer.GutterWidth - 2
	if gw < 1 {
		gw = 1
	}
	runOff := m.wrapMode == viewport.WrapOff
	hOffset := m.viewport.HOffset()
	textWidth := m.viewport.TextWidth()
	for _, line := range visible {
		// Issue #18: clip the line to the horizontal pan window with
		// grapheme-safe blank cells before rendering.
		clipped := m.viewport.ClipLine(line)
		if clipped.Continuation {
			// Issue #16: continuation rows have a blank gutter
			// aligned with the first row's text.
			b.WriteString(strings.Repeat(" ", gw+2))
		} else if runOff {
			// Issue #20: the first trailing gutter space carries
			// the left hidden-content indicator.
			b.WriteString(fmt.Sprintf("%*d", gw, clipped.Number))
			b.WriteString(leftIndicator(line, m.theme, hOffset, textWidth))
			b.WriteString(" ")
		} else {
			b.WriteString(fmt.Sprintf("%*d  ", gw, clipped.Number))
		}
		b.WriteString(renderLineWithHighlights(clipped, m.theme, currentLine))
		if runOff && !clipped.Continuation {
			// Issue #20: pad to the text width and render the
			// reserved rightmost column. The text width already
			// excludes the reserved column, so the indicator
			// never overwrites text.
			renderedCells := clusterCellWidth(clipped.Clusters)
			if renderedCells < textWidth {
				b.WriteString(strings.Repeat(" ", textWidth-renderedCells))
			}
			if clipped.Number == currentLine && hasHiddenMatchRight(line, hOffset, textWidth) {
				b.WriteString(m.theme.Indicator("*"))
			} else {
				b.WriteString(" ")
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}
```

### Visibility helpers (internal/app/app.go)

`leftIndicator` returns the styled left gutter indicator: `*` if any highlight is entirely hidden left, `_` if text is hidden left, or a blank. `hasHiddenMatchRight` reports whether any highlight is entirely hidden right. `highlightHasVisibleCells` walks the clusters and reports whether any fully visible (non-split) cluster overlaps the highlight's in-window cell range — split clusters rendered as blanks do not count.

```bash
cd /home/chris/vrg && sed -n '2251,2340p' internal/app/app.go
```

```output
// leftIndicator returns the styled left gutter indicator for a line in
// run-off-edge mode (Issue #20): inverse "*" if a match/marker is
// entirely hidden left, inverse "_" if text is hidden left, or a blank
// space otherwise. Visibility is derived from the actually rendered
// cells after grapheme clipping.
func leftIndicator(line filebuffer.Line, t theme.Theme, hOffset, textWidth int) string {
	if hOffset <= 0 || !lineHasContent(line) {
		return " "
	}
	windowEnd := hOffset + textWidth
	for _, hl := range line.Highlights {
		if !highlightHasVisibleCells(hl, line.Clusters, hOffset, windowEnd) && hl[0] < hOffset {
			return t.Indicator("*")
		}
	}
	return t.Indicator("_")
}

// hasHiddenMatchRight reports whether any match/marker on the line is
// entirely hidden right of the visible window (Issue #20). A match is
// entirely hidden right when it has no non-blank visible cells in the
// window and extends past the right edge.
func hasHiddenMatchRight(line filebuffer.Line, hOffset, textWidth int) bool {
	windowEnd := hOffset + textWidth
	for _, hl := range line.Highlights {
		if !highlightHasVisibleCells(hl, line.Clusters, hOffset, windowEnd) && hl[1] > windowEnd {
			return true
		}
	}
	return false
}

// highlightHasVisibleCells reports whether any cell in the highlight
// range [hl[0], hl[1]) is a non-blank rendered cell within the window
// [hOffset, windowEnd) (Issue #20). A cell is non-blank only if it
// belongs to a fully visible (non-split) cluster; a split wide glyph
// rendered as blanks does not count.
func highlightHasVisibleCells(hl [2]int, clusters []filebuffer.Cluster, hOffset, windowEnd int) bool {
	inStart := hl[0]
	if inStart < hOffset {
		inStart = hOffset
	}
	inEnd := hl[1]
	if inEnd > windowEnd {
		inEnd = windowEnd
	}
	if inStart >= inEnd {
		return false
	}
	cellPos := 0
	for _, c := range clusters {
		cs := cellPos
		ce := cellPos + c.Width
		cellPos = ce
		if c.Width == 0 {
			continue
		}
		// Fully visible (not split by either clip edge)?
		if cs >= hOffset && ce <= windowEnd {
			// Overlaps [inStart, inEnd)?
			if cs < inEnd && ce > inStart {
				return true
			}
		}
	}
	return false
}

// lineHasContent reports whether the line has any non-zero-width
// grapheme clusters (Issue #20). Zero-width clusters (e.g. combining
// marks) alone do not count as visible text.
func lineHasContent(line filebuffer.Line) bool {
	for _, c := range line.Clusters {
		if c.Width > 0 {
			return true
		}
	}
	return false
}

// clusterCellWidth returns the total terminal cell width of the given
// clusters (Issue #20). Used to pad the text area before the reserved
// right-indicator column.
func clusterCellWidth(clusters []filebuffer.Cluster) int {
	w := 0
	for _, c := range clusters {
		w += c.Width
	}
	return w
}
```

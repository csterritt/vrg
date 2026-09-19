# Issue #43: a standalone combining cluster paints a real one-cell ◌ fallback

*2026-09-18T10:12:57Z by Showboat 0.6.1*
<!-- showboat-id: e547db5b-0c72-46ff-b62d-05debd404f2e -->

Issue #43 materializes a real fallback cell for standalone combining clusters - grapheme clusters with no base and no independent visible cell (width 0 under the shared rivo/uniseg policy). The recorded Candidate-A decision (Notes/decisions/043-combining-cluster-fallback-cell.md) displays '◌' (U+25CC, UTF-8 E2 97 8C) followed by the cluster's original combining-mark bytes, composed onto the dotted-circle 'no base' carrier; safepresentation.MapContent emits it as unit('◌'+cl, 1, s, e) - one Cell, one Cluster, width pinned to 1 (constructed, never re-measured), the cell's byte range still the cluster's original source bytes. Every layer - clusters, byte-to-cell maps, wrapping, clipping, panning, highlight expansion, and the Issue #39 cluster-driven renderer - agrees the cluster occupies exactly one visible cell, so no following cluster can overlap it and a match on it is never a zero-cell highlight. See Notes/issues/043-combining-cluster-fallback-cell.md, Notes/tasks/043-combining-cluster-fallback-cell.md, and the 'Text, graphemes, and safe presentation' section of Notes/PRD-vrg.md. This walkthrough runs the Issue #43 fallback tests, then drives the issue's manual scenario on a real PTY through smoke.py. Artifacts (the built vrg binary, smoke.py, and the generated fixture) live in this directory.

## Automated tests - the fallback cell contract

internal/filebuffer/filebuffer_test.go pins the display geometry: TestHighlightStandaloneCombiningGetsFallbackCell asserts the one-cell ◌́ form and its [0,1) highlight; TestStandaloneCombiningFallbackCellGeometry asserts the recorded display bytes (E2 97 8C + mark bytes), the one-cell cluster record, the byte-to-cell mapping resolving the fallback cell to the original source bytes, and the next cluster's ownership of the next cell - for a line-opening mark and a two-mark cluster; TestStandaloneCombiningFallbackHighlightNeverAdjacent proves a mid-line standalone mark's match highlights exactly the fallback cell, never the adjacent escape or text cells.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'Test(HighlightStandaloneCombiningGetsFallbackCell|StandaloneCombiningFallbackCellGeometry|StandaloneCombiningFallbackHighlightNeverAdjacent)' ./internal/filebuffer 2>&1 | grep -E '^( *--- (PASS|FAIL|SKIP)|ok|FAIL)' | sed -E 's/\([0-9.]+s\)//g; s/\t[0-9.]+s$//; s/^ +//'; echo "exit=${PIPESTATUS[0]}"
```

```output
--- PASS: TestHighlightStandaloneCombiningGetsFallbackCell 
--- PASS: TestStandaloneCombiningFallbackCellGeometry 
--- PASS: TestStandaloneCombiningFallbackCellGeometry/mark_opens_the_line 
--- PASS: TestStandaloneCombiningFallbackCellGeometry/two_marks_still_one_fallback_cell 
--- PASS: TestStandaloneCombiningFallbackHighlightNeverAdjacent 
ok  	vrg/internal/filebuffer
exit=0
```

The propagation layers agree the fallback is an ordinary cell: internal/viewport/wrap_test.go's TestWrapCountsStandaloneCombiningFallbackCell proves the line's extent and both row models count it, and internal/app/grapheme_test.go's composed-frame tests assert the styled ◌́ run closes before the next cluster's unstyled cell (TestStandaloneCombiningFallbackCellHighlighted) and that one pan step hides the fallback's single cell whole while the hidden match stars the gutter (TestStandaloneCombiningFallbackPansAsOneCell).

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestWrapCountsStandaloneCombiningFallbackCell' ./internal/viewport 2>&1 | grep -E '^( *--- (PASS|FAIL|SKIP)|ok|FAIL)' | sed -E 's/\([0-9.]+s\)//g; s/\t[0-9.]+s$//; s/^ +//'; go test -count=1 -v -run 'TestStandaloneCombining' ./internal/app 2>&1 | grep -E '^( *--- (PASS|FAIL|SKIP)|ok|FAIL)' | sed -E 's/\([0-9.]+s\)//g; s/\t[0-9.]+s$//; s/^ +//'; echo "exit=$?"
```

```output
--- PASS: TestWrapCountsStandaloneCombiningFallbackCell 
ok  	vrg/internal/viewport
--- PASS: TestStandaloneCombiningFallbackCellHighlighted 
--- PASS: TestStandaloneCombiningFallbackPansAsOneCell 
ok  	vrg/internal/app
exit=0
```

Full tree verification:

```bash
cd /home/chris/vrg && go vet ./... && go build ./... && go test -count=1 ./... 2>&1 | sed -E 's/\t([0-9.]+s|\(cached\))//g'; echo "exit=${PIPESTATUS[0]}"
```

```output
?   	vrg/Notes/walkthroughs/007-04/code-walkthrough/fixture	[no test files]
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/docs
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
?   	vrg/internal/safepresentation/sinktest	[no test files]
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
exit=0
```

## Manual scenario - smoke.py on a real PTY

smoke.py drives the issue's manual scenario end-to-end. It writes a fixture: work/a.txt holds a line that begins with UTF-8 combining acute (CC 81) followed by an x-run past the window edge, and a second line with a mid-line standalone mark; a fake rg emits match results covering the standalone mark bytes. It then launches vrg on a real PTY, emulates the terminal reply, and checks the rendered grid cell-by-cell.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/043-05/code-walkthrough/vrg ./cmd/vrg
```

```output
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/043-05/code-walkthrough && python3 smoke.py
```

```output
scenario: standalone combining marks paint the one-cell ◌ fallback; the match styles exactly that cell; one pan step hides it whole
  [PASS] line 1 opens with the ◌ fallback cell
  [PASS] the fallback is one cell holding ◌+marks
  [PASS] the next cluster paints the next cell - no overlap
  [PASS] the match highlights exactly the fallback cell
  [PASS] nothing adjacent is highlighted
  [PASS] line 2 shows the ◌ fallback mid-line
  [PASS] mid-line fallback is one cell holding ◌+marks
  [PASS] the following 'c' paints the next cell
  [PASS] the mid-line match highlights the fallback cell
  [PASS] adjacent cells never highlighted
  [PASS] line 1's mark keeps its plain-match style after n
  [PASS] one pan step hides the fallback's single cell
  [PASS] the x run shifted exactly one cell left
  [PASS] the hidden-left match stars the gutter
  [PASS] line 2 keeps the text-hidden '_' mark (match visible)
  [PASS] exit 0 (clean results, q)
  [PASS] browse cursor restored
  [PASS] browse alt screen exited
  [PASS] browse termios restored
all checks passed
```

## Result

On a real PTY, a standalone combining cluster renders as the recorded ◌+marks fallback in exactly one cell: the line-opening mark's ◌́ sits in the first content cell with 'x' in the next, the mid-line mark's fallback sits beside its preceding 'A' with 'c' after it - no overlap, no shared cell. The match styles exactly the fallback cell (inverse for a plain match, underline as the current stop) and never an adjacent cell. After w + . the single fallback cell pans off whole, the x-run shifts exactly one cell, and the gutter carries the hidden-match star while line 2 keeps the text-hidden indicator. All 19 checks passed; the session exited cleanly with the terminal restored.

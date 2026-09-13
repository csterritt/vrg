# Issue #12: Manual vertical scrolling and per-file viewport

*2026-09-12T00:57:22Z by Showboat 0.6.1*
<!-- showboat-id: 1ffd0413-026f-41fa-9f02-4329e9490173 -->

Walkthrough for Issue #12 (Notes/tasks/012-manual-vertical-scrolling-and-per-file-viewport.md), implementing manual vertical scrolling with one-row, half-page, and full-page scroll units; BOF/EOF clamping with no avoidable blank rows; natural unused rows for files shorter than the viewport; scroll-key no-ops while a file is loading; per-file saved vertical viewport state for later revisits; and prepared-row rendering where View() only accesses visible rows. References: Notes/PRD-vrg.md (Navigation, viewport, and logical anchors; Module Design).

Contracts verified:

- One-row scroll unit: up/down move the top row by exactly one rendered row.
- Half-page scroll unit: u/d move by max(1, floor(contentHeight / 2)) where contentHeight = panelHeight - 1 (filename row).
- Full-page scroll unit: pgup/pgdn move by the full content height.
- BOF clamp: scrolling up at the top of the file is a no-op (top row stays at 0).
- EOF clamp: scrolling down past EOF stops with the last row at the bottom, leaving no avoidable blank rows below EOF.
- Files shorter than the viewport naturally leave unused rows; scrolling is a no-op.
- Scroll keys are no-ops while the content panel shows the Loading placeholder (viewport is nil).
- Per-file vertical viewport state is saved after every scroll and restored on a later revisit; a first visit starts at the top.
- Prepared-row rendering: the Viewport queries the row provider only for the visible range; a counting fake proves the render path never scans the full buffer per frame.

All generated artifacts live in this directory.

```bash
cd /home/chris/vrg && go build ./... && go vet ./... && echo GATES-OK
```

```output
GATES-OK
```

```bash
cd /home/chris/vrg && go test -count=1 ./... -timeout 120s | sed "s/[[:space:]][0-9.]*s$//"
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

## Viewport unit tests

The viewport tests (internal/viewport/viewport_test.go, Issue #12) verify every scroll unit, both clamps, odd-height half-page behavior, file sizes (shorter/equal/longer than viewport), the empty file, and the render-cost guard at the Viewport level.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/viewport/ -run "^TestScroll" -timeout 30s | sed "s/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//"
```

```output
=== RUN   TestScrollDownOneRow
--- PASS: TestScrollDownOneRow (0.00s)
=== RUN   TestScrollUpOneRow
--- PASS: TestScrollUpOneRow (0.00s)
=== RUN   TestScrollHalfDown
--- PASS: TestScrollHalfDown (0.00s)
=== RUN   TestScrollHalfUp
--- PASS: TestScrollHalfUp (0.00s)
=== RUN   TestScrollPageDown
--- PASS: TestScrollPageDown (0.00s)
=== RUN   TestScrollPageUp
--- PASS: TestScrollPageUp (0.00s)
PASS
ok  	vrg/internal/viewport
```

One-row (up/down), half-page (u/d as max(1, floor(contentHeight/2))), and full-page (pgup/pgdn as contentHeight) scroll units all pass.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/viewport/ -run "^TestClamp" -timeout 30s | sed "s/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//"
```

```output
=== RUN   TestClampBOF
--- PASS: TestClampBOF (0.00s)
=== RUN   TestClampEOF
--- PASS: TestClampEOF (0.00s)
=== RUN   TestClampEOFLastRowAtBottom
--- PASS: TestClampEOFLastRowAtBottom (0.00s)
PASS
ok  	vrg/internal/viewport
```

BOF clamp: scrolling up at the top is a no-op. EOF clamp: scrolling down past EOF stops at maxOffset with the last row at the bottom (no avoidable blank rows).

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/viewport/ -run "^TestHalfPage|^TestFile|^TestEmpty|^TestVisible|^TestSetOffset|^TestContentHeight|^TestSetPanelHeight" -timeout 30s | sed "s/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//"
```

```output
=== RUN   TestHalfPageOddHeight
=== RUN   TestHalfPageOddHeight/panelHeight=6
=== RUN   TestHalfPageOddHeight/panelHeight=8
=== RUN   TestHalfPageOddHeight/panelHeight=3
=== RUN   TestHalfPageOddHeight/panelHeight=4
=== RUN   TestHalfPageOddHeight/panelHeight=10
--- PASS: TestHalfPageOddHeight (0.00s)
    --- PASS: TestHalfPageOddHeight/panelHeight=6 (0.00s)
    --- PASS: TestHalfPageOddHeight/panelHeight=8 (0.00s)
    --- PASS: TestHalfPageOddHeight/panelHeight=3 (0.00s)
    --- PASS: TestHalfPageOddHeight/panelHeight=4 (0.00s)
    --- PASS: TestHalfPageOddHeight/panelHeight=10 (0.00s)
=== RUN   TestHalfPageOddHeightUp
=== RUN   TestHalfPageOddHeightUp/panelHeight=6
=== RUN   TestHalfPageOddHeightUp/panelHeight=8
=== RUN   TestHalfPageOddHeightUp/panelHeight=3
--- PASS: TestHalfPageOddHeightUp (0.00s)
    --- PASS: TestHalfPageOddHeightUp/panelHeight=6 (0.00s)
    --- PASS: TestHalfPageOddHeightUp/panelHeight=8 (0.00s)
    --- PASS: TestHalfPageOddHeightUp/panelHeight=3 (0.00s)
=== RUN   TestFileShorterThanViewport
--- PASS: TestFileShorterThanViewport (0.00s)
=== RUN   TestFileEqualToViewport
--- PASS: TestFileEqualToViewport (0.00s)
=== RUN   TestFileLongerThanViewport
--- PASS: TestFileLongerThanViewport (0.00s)
=== RUN   TestFileLongerThanViewportMaxOffset
--- PASS: TestFileLongerThanViewportMaxOffset (0.00s)
=== RUN   TestEmptyFile
--- PASS: TestEmptyFile (0.00s)
=== RUN   TestVisibleQueriesOnlyVisibleRange
--- PASS: TestVisibleQueriesOnlyVisibleRange (0.00s)
=== RUN   TestVisibleQueriesOnlyVisibleRangeAtEOF
--- PASS: TestVisibleQueriesOnlyVisibleRangeAtEOF (0.00s)
=== RUN   TestSetOffsetClamp
--- PASS: TestSetOffsetClamp (0.00s)
=== RUN   TestContentHeight
--- PASS: TestContentHeight (0.00s)
=== RUN   TestSetPanelHeight
--- PASS: TestSetPanelHeight (0.00s)
=== RUN   TestSetPanelHeightClampsOffset
--- PASS: TestSetPanelHeightClampsOffset (0.00s)
PASS
ok  	vrg/internal/viewport
```

Odd-height half-page formula, file sizes (shorter/equal/longer/empty), render-cost guard (visible range only), SetOffset/SetPanelHeight clamping, and ContentHeight all pass.

## App scroll tests

The app tests (internal/app/scroll_test.go, Issue #12) verify the loading-placeholder no-op, per-file viewport state (saved/restored/first-visit), scroll key behavior through Update, BOF/EOF clamps, and the render-cost guard at the App level (counting fake row provider).

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run "^TestScroll|^TestPerFile|^TestRender" -timeout 30s | sed "s/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//"
```

```output
=== RUN   TestScrollKeysOnLoadingPlaceholder
--- PASS: TestScrollKeysOnLoadingPlaceholder (0.00s)
=== RUN   TestPerFileViewportStateSaved
--- PASS: TestPerFileViewportStateSaved (0.00s)
=== RUN   TestPerFileViewportStateRestoredOnReload
--- PASS: TestPerFileViewportStateRestoredOnReload (0.00s)
=== RUN   TestPerFileViewportStateFirstVisitStartsAtTop
--- PASS: TestPerFileViewportStateFirstVisitStartsAtTop (0.00s)
=== RUN   TestScrollDownOneRow
--- PASS: TestScrollDownOneRow (0.00s)
=== RUN   TestScrollUpOneRow
--- PASS: TestScrollUpOneRow (0.00s)
=== RUN   TestScrollHalfDown
--- PASS: TestScrollHalfDown (0.00s)
=== RUN   TestScrollHalfUp
--- PASS: TestScrollHalfUp (0.00s)
=== RUN   TestScrollPageDown
--- PASS: TestScrollPageDown (0.00s)
=== RUN   TestScrollPageUp
--- PASS: TestScrollPageUp (0.00s)
=== RUN   TestScrollClampBOF
--- PASS: TestScrollClampBOF (0.00s)
=== RUN   TestScrollClampEOF
--- PASS: TestScrollClampEOF (0.00s)
=== RUN   TestRenderCostGuard
--- PASS: TestRenderCostGuard (0.00s)
=== RUN   TestRenderCostGuardAfterScroll
--- PASS: TestRenderCostGuardAfterScroll (0.00s)
=== RUN   TestRenderShowsOnlyVisibleRows
--- PASS: TestRenderShowsOnlyVisibleRows (0.00s)
PASS
ok  	vrg/internal/app
```

## Manual demonstration on a long file

The binary is run on a long file through a PTY to demonstrate the scroll keys. A 60-line fixture file with a match on line 1 is used so the browse panel loads and scrolling is exercised. The PTY runner sends keys and captures the rendered frames.

## Manual demonstration on a long file

The binary is run on a 60-line fixture file through a PTY (80x24 terminal, contentHeight = 23). A fake rg emits one match so vrg enters the browse state and loads the file. The showlines.py helper strips ANSI sequences and extracts the visible line numbers from the last rendered frame.

```bash
cd /home/chris/vrg/Notes/walkthroughs/012-04/code-walkthrough && echo "=== Initial view (offset 0, contentHeight = 23) ===" && python3 showlines.py "q"
```

```output
=== Initial view (offset 0, contentHeight = 23) ===
line 01
line 02
line 03
line 04
line 05
line 06
line 07
line 08
line 09
line 10
line 11
line 12
line 13
line 14
line 15
line 16
line 17
line 18
line 19
line 20
line 21
line 22
line 23
```

Initial view at offset 0 shows lines 1-23 (contentHeight = panelHeight - 1 = 24 - 1 = 23).

```bash
cd /home/chris/vrg/Notes/walkthroughs/012-04/code-walkthrough && ./demo.sh
```

```output
=== Initial view (offset 0, contentHeight = 23) ===
line 01
line 02
line 03
line 04
line 05
line 06
line 07
line 08
line 09
line 10
line 11
line 12
line 13
line 14
line 15
line 16
line 17
line 18
line 19
line 20
line 21
line 22
line 23

=== After down (one row, offset 1) ===
line 02
line 03
line 04
line 05
line 06
line 07
line 08
line 09
line 10
line 11
line 12
line 13
line 14
line 15
line 16
line 17
line 18
line 19
line 20
line 21
line 22
line 23
line 24

=== After d (half page = 11, offset 11) ===
line 12
line 13
line 14
line 15
line 16
line 17
line 18
line 19
line 20
line 21
line 22
line 23
line 24
line 25
line 26
line 27
line 28
line 29
line 30
line 31
line 32
line 33
line 34

=== After pgdn (full page = 23, offset 23) ===
line 24
line 25
line 26
line 27
line 28
line 29
line 30
line 31
line 32
line 33
line 34
line 35
line 36
line 37
line 38
line 39
line 40
line 41
line 42
line 43
line 44
line 45
line 46

=== EOF clamp: 3x pgdn (offset 69 -> clamped to 37) ===
line 38
line 39
line 40
line 41
line 42
line 43
line 44
line 45
line 46
line 47
line 48
line 49
line 50
line 51
line 52
line 53
line 54
line 55
line 56
line 57
line 58
line 59
line 60

=== down at EOF (no-op, stays at lines 38-60) ===
line 38
line 39
line 40
line 41
line 42
line 43
line 44
line 45
line 46
line 47
line 48
line 49
line 50
line 51
line 52
line 53
line 54
line 55
line 56
line 57
line 58
line 59
line 60

=== up at BOF (no-op, stays at lines 1-23) ===
line 01
line 02
line 03
line 04
line 05
line 06
line 07
line 08
line 09
line 10
line 11
line 12
line 13
line 14
line 15
line 16
line 17
line 18
line 19
line 20
line 21
line 22
line 23

=== u (half page up from offset 23 -> offset 12) ===
line 13
line 14
line 15
line 16
line 17
line 18
line 19
line 20
line 21
line 22
line 23
line 24
line 25
line 26
line 27
line 28
line 29
line 30
line 31
line 32
line 33
line 34
line 35

=== pgup (full page up from offset 23 -> offset 0) ===
line 01
line 02
line 03
line 04
line 05
line 06
line 07
line 08
line 09
line 10
line 11
line 12
line 13
line 14
line 15
line 16
line 17
line 18
line 19
line 20
line 21
line 22
line 23
```

Summary of observed behavior:

- Initial view at offset 0 shows lines 1-23 (contentHeight = panelHeight - 1 = 24 - 1 = 23).
- down moves one row: lines 2-24 (offset 1).
- d moves half a page: lines 12-34 (offset 11 = max(1, floor(23/2))).
- pgdn moves a full page: lines 24-46 (offset 23 = contentHeight).
- 3x pgdn (offset 69) clamps to maxOffset = 37: lines 38-60, last row at the bottom, no avoidable blank rows.
- down at EOF is a no-op: stays at lines 38-60.
- up at BOF is a no-op: stays at lines 1-23.
- u (half page up from offset 23) moves to offset 12: lines 13-35.
- pgup (full page up from offset 23) moves to offset 0: lines 1-23.

References: Issue #12 (Notes/issues/012-manual-vertical-scrolling-and-per-file-viewport.md), Notes/PRD-vrg.md (Navigation, viewport, and logical anchors; Module Design).

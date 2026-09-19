# Issue #16: wrap mode and the w toggle

*2026-09-17T16:52:42Z by Showboat 0.6.1*
<!-- showboat-id: 21c580da-20ad-4e09-87b2-8be18fa6e71b -->

Issue #16 lands wrap mode: the file panel wraps long lines by default at grapheme-cluster boundaries, `w` toggles the session between wrapped rows and run-off-edge clipping, tabs expand structurally to eight-column source-display stops, and the rendered-row model is a keyed, swappable value — `Key{Path, Revision, TextWidth, Wrap}` — prepared once per layout instead of recomputed by `View()`. `internal/filebuffer` owns the shared segmentation (`Line.Clusters`) and cell-width policy that `internal/viewport` wraps from; wrapped continuation rows carry a blank gutter; the reserved right-edge indicator column is 0 cells while wrapping and 1 in run-off-edge mode (populated by Issue #20); and the Issue #14 reveal now maps a match deep inside a wrapped line to its own rendered row. See `Notes/issues/016-wrap-mode-and-toggle.md`, `Notes/tasks/016-wrap-mode-and-toggle.md`, and the PRD sections "Text, graphemes, and safe presentation" and "Layout and indicators" in `Notes/PRD-vrg.md`. All artifacts live in this directory.

## Gates — module integrity, build, vet

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

## Row model — wrap counts, cluster boundaries, run-off-edge, the key

`internal/viewport/wrap_test.go` pins the row-model contracts: a long ASCII line wraps into `ceil(cells / textWidth)` rows partitioning its cells in order with `Continuation()` true after the first row (empty lines still yield one row); a two-cell cluster that does not fit moves whole to the next row leaving blank cells behind — a cluster is never split; a regional-indicator flag pair and a tab expansion each wrap as single unbreakable clusters; every wrapped row boundary coincides with a `Line.Clusters` boundary at every width; run-off-edge mode keeps one row per source line at any width; the model carries its `Key` verbatim so a layout change makes it stale; and `TargetRow` resolves a match's start cell to the wrapped row containing it.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/viewport 2>&1 | grep -E '^(--- |    --- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestStopTargetIsFirstSubmatchStartCell
--- PASS: TestStopTargetZeroWidthLandsPastLastCell
--- PASS: TestTargetRow
--- PASS: TestRevealVisibleTargetNoScroll
--- PASS: TestRevealHiddenTargetOneThird
--- PASS: TestRevealBOFEOFClamps
--- PASS: TestRevealStartsFromCurrentTop
--- PASS: TestScrollUnitsMoveRenderedRows
--- PASS: TestHalfPageUnit
--- PASS: TestScrollClampByFileLength
--- PASS: TestScrollClampBOF
--- PASS: TestClampPullsTopUp
--- PASS: TestPrepareRows
--- PASS: TestWrapRowCountsASCII
--- PASS: TestWrapMovesWideClusterToNextRow
--- PASS: TestWrapKeepsClustersTogether
--- PASS: TestWrapTabIsOneCluster
--- PASS: TestWrapRowsAlignToClusterBoundaries
--- PASS: TestRunOffEdgeOneRowPerLine
--- PASS: TestRowsCarryTheirKey
--- PASS: TestWrappedTargetRow
--- PASS: TestRevealDeepInWrappedLine
ok  	vrg/internal/viewport
```

## Shared segmentation and structural tabs — safepresentation + filebuffer

`TestMapContentTabStops` and `TestTabExpandsToEightColumnStops` pin the structural rule: a tab expands with blank cells to the next multiple of eight source-display columns — a tab already on a stop takes a full eight — every expansion cell maps back to the tab's byte range, and the expansion is one unbreakable `Cluster`, so stops never shift with gutter width or horizontal pan. `TestLineClustersExposeBoundaries` proves FileBuffer exposes the shared grapheme segmentation, and `TestHighlightCoversTabExpansion` proves a stop over a tab highlights its whole expansion.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TabStops|Tab|Clusters|EscapedContent' ./internal/safepresentation ./internal/filebuffer 2>&1 | grep -E '^(--- |    --- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestMapContentTabStops
    --- PASS: TestMapContentTabStops/lone_tab
    --- PASS: TestMapContentTabStops/tab_after_one_column
    --- PASS: TestMapContentTabStops/tab_on_a_stop
ok  	vrg/internal/safepresentation
--- PASS: TestLoadEscapedContent
--- PASS: TestTabExpandsToEightColumnStops
    --- PASS: TestTabExpandsToEightColumnStops/leading_tab
    --- PASS: TestTabExpandsToEightColumnStops/tab_after_one_column
    --- PASS: TestTabExpandsToEightColumnStops/tab_after_seven_columns
    --- PASS: TestTabExpandsToEightColumnStops/tab_on_a_stop_takes_a_full_eight
    --- PASS: TestTabExpandsToEightColumnStops/wide_glyph_counts_cells_not_bytes
--- PASS: TestLineClustersExposeBoundaries
--- PASS: TestHighlightCoversTabExpansion
ok  	vrg/internal/filebuffer
```

## App — default wrap, the w toggle, wrapped reveal, resize rebuild, render cost

`internal/app/wrap_test.go` covers the feature end to end: wrap is on at startup with numbered first rows and blank continuation gutters; `w` swaps the keyed row model for run-off-edge — the long line becomes one clipped row with the rightmost panel cell reserved blank — and back; a match near the end of a many-screen wrapped line is revealed on its own continuation row at `floor(content height / 3)` (the needle's styled run may straddle the row boundary — the match start cell is the revealed row's guarantee); and a resize rebuilds the model at the new text width and re-clamps the saved viewport. The scroll suite's `TestRenderQueriesOnlyVisibleRows` counting fake still proves a frame queries `rowSource` only for the visible range — `View()` never wraps the buffer.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'Wrap|RenderQueriesOnlyVisibleRows|Resize' ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestResizeDuringSearching
--- PASS: TestResizeRecomposesBrowse
--- PASS: TestNavigationWrapsBothEnds
--- PASS: TestOverlayWrapsUnbrokenDiagnostic
--- PASS: TestPopupRecentresOnResizeWithoutRestart
--- PASS: TestRenderQueriesOnlyVisibleRows
--- PASS: TestResizeReclampsViewport
--- PASS: TestWrapOnByDefaultBlankContinuationGutters
--- PASS: TestWTogglesWrapMode
--- PASS: TestRevealMatchDeepInWrappedLine
--- PASS: TestResizeRebuildsRowModel
ok  	vrg/internal/app
```

## Full suite — `go test ./...`

```bash
cd /home/chris/vrg && go test -count=1 ./... 2>&1 | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
?   	vrg/Notes/walkthroughs/007-04/code-walkthrough/fixture	[no test files]
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
?   	vrg/internal/safepresentation/sinktest	[no test files]
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
```

## Manual check — `vrg needle .` on a pty against a 500-cell line

Build the real binary into this directory, then `pty_wrap.py` runs it on a 100x24 pty against `fixture/long.txt` — a 500-cell line with a `needle` match near its end, a tabbed line, and padding — replays the raw byte stream through a small terminal emulator, and asserts on the screen grid:

- **init** — wrap is on by default: line 1 is numbered and its match underlined; the tabbed line shows `bb` on stop column 8 and `cc` on stop 16 — the line's own cell positions, independent of the gutter; the 500-cell line fills the screen in wrapped rows whose continuation gutters are blank.
- **n** — the deep match's wrapped row was hidden; the reveal lands it at content row `floor(23/3) = 7` (screen row 8) behind a blank continuation gutter, underlined as the current match.
- **w** — run-off-edge mode: every source line is exactly one numbered row (sequential gutters), the 500-cell line clips to one row, and the rightmost panel cell stays blank — the reserved indicator column Issue #20 populates.
- **w** again — back to wrapped rows with blank continuation gutters; **q** exits 0 after restoring the screen.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/016-04/code-walkthrough/vrg ./cmd/vrg && python3 Notes/walkthroughs/016-04/code-walkthrough/pty_wrap.py
```

```output
init    : wrap on — ' 1  needle first' / ' 2  aa      bb      cc' / continuation '    xxxxxx'
n       : match revealed on continuation row 8 — '    needlexxxxxx'
w       : one row per line — line 3 clipped ' 3  xxxx'…'xxx ' (last cell blank)
w       : wrapped again — '    xxxxxxxx'
q       : exit 0
OK
```

All Issue #16 contracts verified: wrapping is on by default and breaks only at grapheme-cluster boundaries with the never-split blank-cell rule; `w` toggles to run-off-edge clipping and back; tabs expand structurally to eight-column source-display stops independent of gutter and pan; continuation rows carry blank gutters aligned under the first row's text; the reserved indicator column is 0 cells wrapping / 1 run-off-edge; the row model is keyed by (path, content revision, text width, wrap mode) and swapped whole on load, toggle, and resize; `View()` queries only visible prepared rows; and the Issue #14 reveal maps a match deep inside a wrapped line to its own rendered row at `floor(h / 3)`. Issue #17 owns preparing the model off the update path; Issue #20 owns the indicator column's content.

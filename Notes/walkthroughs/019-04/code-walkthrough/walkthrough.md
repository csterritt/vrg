# Issue #19: Minimal horizontal reveal of the first-submatch start

*2026-09-24T11:49:37Z by Showboat 0.6.1*
<!-- showboat-id: 406badde-ff16-4bc1-a458-0e2dac6cd260 -->

Walkthrough for [Issue #19](../../../issues/019-minimal-horizontal-reveal.md), implementing the minimal horizontal reveal of the first-submatch start cell in run-off-edge mode per `Notes/PRD-vrg.md` (*Navigation, viewport, and logical anchors* — the horizontal-reveal bullet and the file-change entry sequence; *Layout and indicators* — painted-cell visibility excluding the reserved column). `Viewport.Reveal` gained a horizontal half: after vertical placement it resolves the target's unit on its rendered row — the grapheme cluster holding the start cell, or a one-cell marker for a zero-width match — and when that unit is not painted in the clipped window the offset moves minimally: to the start column when hidden left, to `start + cluster width − text width` when hidden right or clipped, unchanged when already painted. A match wider than the text area reveals its start cell alone; a cluster wider than the whole text area takes the start column (past the paintable-boundary maximum), renders in-window clipping blanks, and counts as geometrically revealed so repeated navigation cannot loop — while still counting as not visible for Issue #20's indicators. The reveal runs at startup and on every navigation action, after the Issue #18 file-change offset reset. All generated artifacts live in this directory: the built `vrg` binary, the `demo-hreveal.sh` tmux harness, and its `hreveal/` session captures. Test durations are stripped so the document verifies cleanly.

```bash
set -o pipefail; cd /home/chris/vrg && test -z "$(gofmt -l internal/)" && echo "GOFMT-CLEAN" && go vet ./... && go build ./... && go test -count=1 ./... | sed "s/[[:space:]][0-9.]*s$//" && CGO_ENABLED=1 go test -race -count=1 ./internal/app ./internal/viewport | sed "s/[[:space:]][0-9.]*s$//" && go build -o Notes/walkthroughs/019-04/code-walkthrough/vrg ./cmd/vrg && echo GATES-OK
```

```output
GOFMT-CLEAN
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/present
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
ok  	vrg/internal/app
ok  	vrg/internal/viewport
GATES-OK
```

## Reveal arithmetic tests

`internal/viewport/hreveal_test.go` pins the minimal-movement arithmetic against real prepared row models at text width 10: a single-cell target right of view moves the offset to `T − (width − 1)` so the start cell lands on the last text column and no further; a two-cell cluster right of view moves to `T + cluster width − width` so both of its cells paint at the right edge — never a half glyph; a target hidden left (including a cluster split by the left clip edge) moves the offset to the target column exactly; a painted target — including a cluster flush with the right edge — leaves the offset alone; a target hidden on both axes reveals both in one call; and a wrap-mode reveal leaves the dormant offset untouched.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestHRevealRightOfView|TestHRevealLeftOfView|TestHRevealPaintedTargetKeepsOffset|TestHRevealMovesBothAxes|TestHRevealNoOpInWrapMode" ./internal/viewport 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestHRevealRightOfViewPaintsStartCell
--- PASS: TestHRevealRightOfViewWideClusterPaintsWhole
--- PASS: TestHRevealLeftOfViewLandsOnTargetColumn
--- PASS: TestHRevealPaintedTargetKeepsOffset
    --- PASS: TestHRevealPaintedTargetKeepsOffset/single_cell_mid-window
    --- PASS: TestHRevealPaintedTargetKeepsOffset/single_cell_on_the_last_column
    --- PASS: TestHRevealPaintedTargetKeepsOffset/wide_cluster_flush_with_the_right_edge
--- PASS: TestHRevealMovesBothAxes
--- PASS: TestHRevealNoOpInWrapMode
PASS
ok  	vrg/internal/viewport
```

## Painted-cell visibility and the edge cases

Visibility is *painted cells after clipping*, not window geometry: a start cell geometrically inside the window but blanked because its two-cell glyph is split by the right edge counts as hidden and is revealed. A match wider than the text area reveals its start cell alone — the clipped span sits at the window edge while the rest of the match stays hidden right. A cluster wider than the whole text area cannot paint at any offset, so the reveal sets the offset to its start column (beyond the paintable-boundary pan maximum), renders the in-window cells as clipping blanks, and treats the target as geometrically revealed — a repeat reveal does not move, so navigation cannot loop; Issue #20's indicators still count it as not visible. Marker cells are one painted unit: an end-of-line marker reveals to the last column, and a marker on a clipped cluster's lead already counts as painted.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestHRevealClippedBlankCountsAsHidden|TestHRevealOversizedSpanShowsStartCell|TestHRevealUnpaintableClusterUsesStartColumn|TestHRevealMarkerCells" ./internal/viewport 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestHRevealClippedBlankCountsAsHidden
--- PASS: TestHRevealOversizedSpanShowsStartCell
--- PASS: TestHRevealUnpaintableClusterUsesStartColumn
--- PASS: TestHRevealMarkerCells
PASS
ok  	vrg/internal/viewport
```

## App trigger tests

`internal/app/hreveal_test.go` drives the triggers through `Update` in run-off-edge mode: a same-file `n` to a match at cell 295 moves the offset to `295 + 1 − text width` with the start cell on the last text column; `p` back lands on the target column; an already-visible match moves nothing; the startup reveal applies horizontally when the first file's layout installs; a file crossing applies the Issue #18 `SetOffset(0)` reset before revealing; and a match starting on a two-cell CJK glyph reveals by the cluster-width rule so both cells paint.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestHReveal" ./internal/app 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestHRevealSameFileNScrollsRightMinimal
--- PASS: TestHRevealPBackScrollsLeftToTarget
--- PASS: TestHRevealVisibleMatchKeepsOffset
--- PASS: TestHRevealAppliesAtStartup
--- PASS: TestHRevealAfterFileChangeReset
--- PASS: TestHRevealWideClusterMatchPaintsBothCells
PASS
ok  	vrg/internal/app
```

## Manual check — reveal on a real PTY

`demo-hreveal.sh` (checked into this directory) runs the freshly built `vrg` on a real tmux PTY with a fake `rg` feeding a one-file fixture at 80x24 (7-cell list, 4-cell gutter, 68-cell text area, one reserved indicator column). `a.txt` has matches at columns 5 and 300: line 1 `xxxxxhit` (match at cell 5), line 2 of 300 x's + `hit` (cell 300), line 3 of 280 x's + `hit` (cell 280 — inside the line-2 reveal window), and line 4 of 300 x's + `世` (the CJK match at cells 300-301). The run demonstrates: `w` into run-off-edge at offset 0; `n` to the cell-300 match scrolling right the minimum — offset 233 = 300 + 1 − 68 — with the start cell on the last text column; `p` back scrolling left to exactly column 5; `n` to the cell-280 match moving nothing (the row is byte-identical while the current-line underline advances); and `n` to the 世 match moving to offset 234 = 300 + 2 − 68 so both cells of the first glyph paint at the right edge.

```bash
cd /home/chris/vrg/Notes/walkthroughs/019-04/code-walkthrough && ./demo-hreveal.sh
```

```output
ok: w: run-off-edge, offset 0 — line 2 leads from cell 0 -> xxxxxxxxxx
ok: n: far match reveals right — start cell on the last text column -> xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxh
ok: p: back to cell 5 — offset lands on the target column -> hit
ok: n: far match again — offset 233 -> xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxh
ok: n: visible match — row 2 of the window is unchanged -> xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxh
ok: underline moved to line 3 — the cursor advanced without scrolling
ok: n: CJK match — both cells of 世 painted at the right edge -> xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx世
ok: exit status -> 0
demo-hreveal: all checks passed
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/019-04/code-walkthrough && echo "== offset 0: lines 2-4 lead from cell 0 ==" && sed -n "3,5p" hreveal/screen-01-runoff.txt | sed "s/ *$//" && echo "== n -> line 2 (cell 300): offset 233 = 300 + 1 - 68 — start cell on the last text column; line 3's hit (cell 280) already paints mid-window ==" && sed -n "2,5p" hreveal/screen-02-n-far.txt | sed "s/ *$//" && echo "== p -> line 1 (cell 5): offset 5 — the window opens on the target column ==" && sed -n "2p" hreveal/screen-03-p-back.txt | sed "s/ *$//" && echo "== n -> line 3 (cell 280): already painted — the offset stays 233, frame byte-identical ==" && sed -n "2,5p" hreveal/screen-04-nomove.txt | sed "s/ *$//" && echo "== n -> line 4 (世 at cells 300-301): offset 234 = 300 + 2 - 68 — both cells of the glyph paint at the right edge ==" && sed -n "2,5p" hreveal/screen-05-cjk.txt | sed "s/ *$//"
```

```output
== offset 0: lines 2-4 lead from cell 0 ==
        2  xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
        3  xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
        4  xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
== n -> line 2 (cell 300): offset 233 = 300 + 1 - 68 — start cell on the last text column; line 3's hit (cell 280) already paints mid-window ==
        1
        2  xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxh
        3  xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxhit
        4  xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
== p -> line 1 (cell 5): offset 5 — the window opens on the target column ==
        1  hit
== n -> line 3 (cell 280): already painted — the offset stays 233, frame byte-identical ==
        1
        2  xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxh
        3  xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxhit
        4  xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
== n -> line 4 (世 at cells 300-301): offset 234 = 300 + 2 - 68 — both cells of the glyph paint at the right edge ==
        1
        2  xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxhi
        3  xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxhit
        4  xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx世
```

## Verdict

Issue #19 is verified. In run-off-edge mode the destination reveal now covers the horizontal axis: a hidden first-submatch start cell moves the offset by the minimum columns that paint it — the target column when hidden left, `start + cluster width − text width` when hidden right — with visibility judged on rendered cells after grapheme-safe clipping, so a geometrically inside cell blanked by a clip edge still triggers the reveal. Oversized matches reveal their start cell alone; an unpaintable cluster wider than the text area takes its start column, renders clipping blanks, and cannot loop. On the real binary the same-file `n` landed the cell-300 match's start on the last text column, `p` returned to exactly column 5, the already-painted cell-280 match moved nothing, and the 世 match painted both cells of its first glyph at the right edge.

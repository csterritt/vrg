# Issue #16: Wrap mode and toggle

*2026-09-23T23:19:24Z by Showboat 0.6.1*
<!-- showboat-id: 00964880-0ec8-4dc3-a5ff-3a02209e30d5 -->

Walkthrough for [Issue #16](../../../issues/016-wrap-mode-and-toggle.md), implementing wrap mode per `Notes/PRD-vrg.md` (*Text, graphemes, and safe presentation* — the shared grapheme policy, wrap-at-boundaries, and tab bullets; *Layout and indicators* — the reserved-indicator and continuation-gutter bullets; *Navigation, viewport, and logical anchors* — the rendered-row scroll unit and wrapped-target reveal; stories 64, 68, and 71 under *Wrapping, indicators, and text display*). Wrapping is on initially; `w` toggles run-off-edge mode, which reserves one rightmost indicator column (populated later by Issue #20), so text width is panel width minus gutter minus that reserved width. `present.LineOf` is the single segmentation and cell-width policy: each grapheme cluster's first cell carries `Lead` — the only legal wrap boundary — and `Cont` marks a multi-cell unit's trailing cells; `*filebuffer.Buffer` exposes them through the new `viewport.Source` interface, so the row model never re-derives graphemes. Tabs expand structurally with space cells to the next multiple of eight source-display columns as one cluster, replacing Issue #5's provisional `→`. `viewport.Prepare` builds the swappable `Model` keyed by `(path, content revision, text width, wrap mode)` — the staleness contract Issue #17's asynchronous preparation consumes — packing clusters greedily, moving an unfit cluster whole to the next row (blank remainder), splitting an over-wide cluster only as a last resort, and giving an end-of-line marker past a full final row its own continuation row. Continuation rows paint a blank gutter aligned with the lead row's text, and `Reveal` resolves a display target through `Model.RowOf`, so a match deep inside a screen-tall wrapped line lands its rendered row at `floor(height/3)`. All generated artifacts live in this directory: the built `vrg` binary, the `demo-wrap.sh` tmux harness, and its `wrap/` session captures. Test durations are stripped so the document verifies cleanly.

```bash
cd /home/chris/vrg && go vet ./... && go build ./... && go test -count=1 ./... | sed "s/[[:space:]][0-9.]*s$//" && echo GATES-OK
```

```output
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/present
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
GATES-OK
```

## Wrap row-model tests

`internal/viewport/wrap_test.go` pins the prepared row model against `lineSource`, a `Source` fake over real `present.Line` segmentation. `TestWrapRowModel` covers the row-count table: short and empty lines, exact-multiple packing with no invented empty row, a two-cell wide cluster (`世`) that cannot fit the row's remaining cells moving whole to the next row and leaving a blank, a combining cluster kept whole, an over-wide escape cluster (`\u0085`, six cells) splitting across rows as a last resort with its clipped lead cell blanked, and the tab expansion — one cluster — moving whole or splitting like any oversized unit. `TestWrapRowLineAndContinuation` proves every rendered row reports its source line plus `Cont`; `TestRunOffEdgeRowModel` pins the one-row-per-line mode whose rows carry full cells for the frame to clip; `TestRowModelKey` pins the `(path, revision, width, wrap)` key; `TestWrapTranslatesSpansPerRow` proves coverage spans clip to each row's local cells and a boundary marker belongs to the next row; and `TestEndOfLineMarkerRows` covers the marker past a completely full wrap row occupying its own continuation row.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestWrap|TestRunOffEdge|TestRowModelKey|TestEndOfLineMarker' ./internal/viewport 2>&1 | grep -vE '^(=== RUN|=== CONT)' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestWrapRowModel
    --- PASS: TestWrapRowModel/short_line_is_one_row
    --- PASS: TestWrapRowModel/ascii_packs_full_rows
    --- PASS: TestWrapRowModel/exact_multiple_adds_no_empty_row
    --- PASS: TestWrapRowModel/empty_line_still_occupies_a_row
    --- PASS: TestWrapRowModel/wide_cluster_at_the_boundary_leaves_a_blank
    --- PASS: TestWrapRowModel/combining_cluster_stays_whole
    --- PASS: TestWrapRowModel/oversized_cluster_splits_as_a_last_resort
    --- PASS: TestWrapRowModel/tab_expansion_moves_whole_then_splits
--- PASS: TestWrapRowLineAndContinuation
--- PASS: TestRunOffEdgeRowModel
--- PASS: TestRowModelKey
--- PASS: TestWrapTranslatesSpansPerRow
--- PASS: TestEndOfLineMarkerRows
PASS
ok  	vrg/internal/viewport
```

## Shared grapheme policy and tab stops

`internal/filebuffer/cluster_test.go` pins the buffer as the single source of the segmentation policy: `TestCellClusterBoundaries` asserts `Lead` on every cluster's first cell and `Cont` on a multi-cell unit's trailing cells across ASCII, wide, combining, and caret-escape units; `TestLeadingCombiningCluster` covers a baseless combining mark at line start; `TestTabStopCells` pins the eight-column expansion as one cluster whose first cell leads, with the tab byte's span covering the whole expansion so a recorded submatch highlights all of it. In `internal/present/line_test.go`, `TestLineTabStops` asserts the cell positions Issue #5 deferred — expansion to the next multiple of eight source-display columns from column zero, mid-line, at a stop boundary, and doubled — plus the raw tab never surviving presentation.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestCellClusterBoundaries|TestLeadingCombiningCluster|TestTabStopCells' ./internal/filebuffer 2>&1 | grep -vE '^(=== RUN|=== CONT)' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//' && go test -count=1 -v -run 'TestLineTabStops|TestLineText|TestLineWidth' ./internal/present 2>&1 | grep -vE '^(=== RUN|=== CONT)' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestCellClusterBoundaries
--- PASS: TestLeadingCombiningCluster
--- PASS: TestTabStopCells
PASS
ok  	vrg/internal/filebuffer
--- PASS: TestLineText
    --- PASS: TestLineText/plain
    --- PASS: TestLineText/no_final_newline
    --- PASS: TestLineText/empty_line
    --- PASS: TestLineText/escape
    --- PASS: TestLineText/bell
    --- PASS: TestLineText/backspace
    --- PASS: TestLineText/nul
    --- PASS: TestLineText/delete
    --- PASS: TestLineText/vertical_tab
    --- PASS: TestLineText/osc_sequence
    --- PASS: TestLineText/csi_sequence
    --- PASS: TestLineText/c1_nel
    --- PASS: TestLineText/c1_csi
    --- PASS: TestLineText/invalid_utf-8
    --- PASS: TestLineText/invalid_utf-8_run
    --- PASS: TestLineText/truncated_sequence
    --- PASS: TestLineText/standalone_cr
    --- PASS: TestLineText/crlf_only_line
    --- PASS: TestLineText/crlf_removed
    --- PASS: TestLineText/lf_removed
    --- PASS: TestLineText/tab_expansion
    --- PASS: TestLineText/combining_cluster
    --- PASS: TestLineText/printable_unicode
--- PASS: TestLineWidth
    --- PASS: TestLineWidth/ascii
    --- PASS: TestLineWidth/caret_escape
    --- PASS: TestLineWidth/c1_escape
    --- PASS: TestLineWidth/invalid_byte
    --- PASS: TestLineWidth/standalone_cr
    --- PASS: TestLineWidth/wide_rune
    --- PASS: TestLineWidth/combining_cluster
    --- PASS: TestLineWidth/tab_expands_to_the_next_stop
    --- PASS: TestLineWidth/tab_at_a_stop_takes_eight
--- PASS: TestLineTabStops
    --- PASS: TestLineTabStops/tab_at_column_zero
    --- PASS: TestLineTabStops/tab_at_column_one
    --- PASS: TestLineTabStops/tab_ending_at_a_stop
    --- PASS: TestLineTabStops/tab_starting_at_a_stop
    --- PASS: TestLineTabStops/double_tab
PASS
ok  	vrg/internal/present
```

## Toggle wiring and wrapped reveal tests

`internal/app/wrap_test.go` drives the mode through `Update`. `TestWrapOnByDefaultBlankContinuationGutter` proves a freshly loaded 100-cell line wraps at the text width (panel minus list minus gutter, zero reserved) with a blank-gutter continuation row. `TestWTogglesRunOffEdge` sends `w`: the reserved indicator column widens by one, shrinking text width to 69, the line renders as one clipped row, and a second `w` restores the wrapped rows. For the reveal contract, `internal/viewport`'s `TestRevealFindsRowOfWrappedLine` resolves a target at cell 150 of a 200-cell line to rendered row 15 and lands it at `floor(6/3)` = 2, and `internal/app`'s `TestNRevealInsideWrappedLine` runs a 2000-cell line with the match at cell 1900 — rendered row 27 — through startup reveal, `n` away, and `p` back, landing at `floor(23/3)` = 7 (top 20) each time.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestWrapOnByDefault|TestWToggles|TestNRevealInsideWrappedLine' ./internal/app 2>&1 | grep -vE '^(=== RUN|=== CONT)' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//' && go test -count=1 -v -run 'TestRevealFindsRowOfWrappedLine' ./internal/viewport 2>&1 | grep -vE '^(=== RUN|=== CONT)' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestWrapOnByDefaultBlankContinuationGutter
--- PASS: TestWTogglesRunOffEdge
--- PASS: TestNRevealInsideWrappedLine
PASS
ok  	vrg/internal/app
--- PASS: TestRevealFindsRowOfWrappedLine
PASS
ok  	vrg/internal/viewport
```

## Render-cost guard

`View()` must never invoke the wrapper for lines outside the visible rows: layout happens once in `Prepare`, and `Row` materializes cells and spans only for a queried row. `TestVisibleRowsNeverWrapsOffscreenLines` (viewport) prepares a 1000-line source, then clears the fake's query log and proves `Visible()` queries `Cells`/`Spans` for exactly the four lines behind the shown rows. `TestRenderQueriesOnlyVisibleRows` (app) installs a counting `Rows` fake through `m.rows` and proves a frame queries exactly the visible row indices — before and after a scroll.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestVisibleRowsNeverWrapsOffscreenLines' ./internal/viewport 2>&1 | grep -vE '^(=== RUN|=== CONT)' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//' && go test -count=1 -v -run 'TestRenderQueriesOnlyVisibleRows|TestBufferPreparesAsRowSource' ./internal/app 2>&1 | grep -vE '^(=== RUN|=== CONT)' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestVisibleRowsNeverWrapsOffscreenLines
PASS
ok  	vrg/internal/viewport
--- PASS: TestRenderQueriesOnlyVisibleRows
--- PASS: TestBufferPreparesAsRowSource
PASS
ok  	vrg/internal/app
```

## Manual check — wrap mode and w on a real PTY

`demo-wrap.sh` (checked into this directory) runs the freshly built `vrg` on a real tmux PTY at 80x24 — content height 23, so the one-third reveal row is 7. Its `fakebin/rg` reports two stops in one 40-line `long.txt`: line 1 (`hit00001`, the startup stop) and line 19, a 500-cell line whose `hit` match starts at display cell 450. With the 10-cell list and 4-cell gutter the text width is 66 in wrap mode and 65 in run-off-edge, so the long line occupies rendered rows 18..25 and the match sits in row 24 — hidden below the startup window. The startup frame already shows the lead row and four continuation rows behind blank gutters; line 2 proves the eight-column tab stops. `n` reveals the match row at floor(23/3) = 7 (top 17); `w` collapses the line to one 65-cell clipped row; `w` again restores the wrapped rows. Plain-text captures are stored under `wrap/`.

```bash
cd /home/chris/vrg/Notes/walkthroughs/016-04/code-walkthrough && ./demo-wrap.sh
```

```output
ok: startup: file panel -> long.txt
ok: startup: current match -> hit00001
ok: startup: tab stops -> a       b       c
ok: startup: lead-row gutter -> 19  
ok: startup: lead-row fill -> xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
ok: startup: continuation gutter blank ->     
ok: startup: continuation aligned -> xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
ok: startup: deeper continuation -> xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
ok: n: first content row -> x000018
ok: n: lead row of line 19 -> 19  
ok: n: match on its own row -> hit
ok: n: match row gutter blank ->     
ok: n: match underlined (current match)
ok: w: run-off-edge gutter -> 19  
ok: w: clipped to reserved edge -> xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
ok: w: next row is line 20 -> x000020
ok: w back: lead-row fill -> xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
ok: w back: continuation gutter blank ->     
ok: w back: continuation aligned -> xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
ok: exit status -> 0
demo-wrap: all checks passed
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/016-04/code-walkthrough && echo '--- startup, bottom rows: line 19 lead row (gutter 19) then blank-gutter continuations ---' && sed -n '19,24p' wrap/screen-01-startup.txt | sed 's/ *$//' && echo '--- after n: top row is rendered row 17 (line 18); match row lands a third down ---' && sed -n '2,3p' wrap/screen-02-n-reveal.txt | sed 's/ *$//' && echo '[... wrapped continuation rows ...]' && sed -n '9p' wrap/screen-02-n-reveal.txt | cut -c1-14 | sed 's/$/<blank gutter>/' && sed -n '9p' wrap/screen-02-n-reveal.txt | cut -c60-80 | sed 's/ *$//' | sed 's/^/cells 45..65 of the match row: /' && echo '--- after w: line 19 is one clipped run-off-edge row; line 20 follows ---' && sed -n '2,5p' wrap/screen-03-runoff.txt | sed 's/ *$//'
```

```output
--- startup, bottom rows: line 19 lead row (gutter 19) then blank-gutter continuations ---
          18  x000018
          19  xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
              xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
              xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
              xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
              xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
--- after n: top row is rendered row 17 (line 18); match row lands a third down ---
          18  x000018
          19  xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
[... wrapped continuation rows ...]
              <blank gutter>
cells 45..65 of the match row: xxxxxxxxxhitxxxxxxxxx
--- after w: line 19 is one clipped run-off-edge row; line 20 follows ---
          18  x000018
          19  xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
          20  x000020
          21  x000021
```

## Verdict

Issue #16 is verified. Wrapping is on by default; `w` toggles to run-off-edge and back, adding or removing the reserved right-indicator column (zero in wrap, one in run-off-edge) so text width is panel minus gutter minus reserved. Wrapping breaks only at grapheme-cluster boundaries — the shared policy `present.LineOf` emits through `Lead`/`Cont` cell marks and `*filebuffer.Buffer` exposes via `viewport.Source` — with an unfit cluster moving whole and leaving a blank, an over-wide cluster splitting only as a last resort, and an end-of-line marker past a full row taking its own continuation row. Tabs expand structurally to eight-column source-display stops independent of gutter and pan. Continuation rows paint blank gutters aligned with the lead row's text. The prepared `viewport.Model` is a swappable value keyed by (path, content revision, text width, wrap mode), built synchronously at load, toggle, and resize, and queried only for visible rows; `Reveal` resolves a wrapped display target through `RowOf` to land its rendered row at `floor(height/3)`. The tmux run shows the whole contract on a real PTY: the 500-cell line wraps with blank continuation gutters, `w` clips it to one row at the reserved edge, tabs sit on eight-column stops, and `n` reveals the deep match on its own rendered row a third down.

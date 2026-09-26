# Issue #18: Horizontal panning in run-off-edge mode

*2026-09-24T01:01:50Z by Showboat 0.6.1*
<!-- showboat-id: e142cb29-95a4-4dcf-ad24-06e0a90b22a5 -->

Walkthrough for [Issue #18](../../../issues/018-horizontal-panning.md), implementing horizontal panning in run-off-edge mode per `Notes/PRD-vrg.md` (*Navigation, viewport, and logical anchors* — the pan-unit bullet, the visible-lines extent policy, and the wrap-toggle offset-retention bullet; *Layout and indicators* — the reserved-indicator width and the hidden-left signposting this geometry enables). The viewport now holds `off`, a horizontal pan offset in display cells, driven by six pan keys — `,`/`.` one column, `<`/`>` ten, `[`/`]` `max(1, floor(text width / 2))` — clamped to `[0, max(0, S)]` where S is the paintable boundary of the widest *currently rendered* source line: the largest cell index whose grapheme cluster also fits the text width, so the maximum always leaves one whole cluster painted. `clampOff` runs inside `clamp` and `resolve`, so every visible-set change — scroll, reveal, resize (which is also how a list hide/show or gutter growth arrives), row-model swap, wrap-toggle re-entry — re-clamps, permanently; and every pan recomputes the maximum from the visible rows. The offset is dormant under wrap and retained through `w w` with re-entry clamping, and resets to zero on file change ahead of the Issue #19 horizontal reveal. `Visible()` clips run-off-edge rows to `[off, off+width)`: a cluster split by either edge paints blank, never half a glyph. All generated artifacts live in this directory: the built `vrg` binary, the `demo-pan.sh` tmux harness, and its `pan/` session captures. Test durations are stripped so the document verifies cleanly.

```bash
set -o pipefail; cd /home/chris/vrg && test -z "$(gofmt -l internal/)" && echo "GOFMT-CLEAN" && go vet ./... && go build ./... && go test -count=1 ./... | sed "s/[[:space:]][0-9.]*s$//" && CGO_ENABLED=1 go test -race -count=1 ./internal/app ./internal/viewport | sed "s/[[:space:]][0-9.]*s$//" && go build -o Notes/walkthroughs/018-04/code-walkthrough/vrg ./cmd/vrg && echo GATES-OK
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

## Pan-unit and wrap-dormancy tests

`internal/viewport/pan_test.go` pins the six pan units against a 300-cell fixture: `Left`/`Right` one column, `TenLeft`/`TenRight` ten, `HalfLeft`/`HalfRight` `max(1, floor(text width / 2))` — including odd widths and the width-1 floor — all clamped to `[0, 299]`. `SetOffset` is the stored-offset entry point the file-change reset drives with 0. Under a wrap model every pan is a no-op and the dormant offset neither moves nor leaks into the wrapped rows; a `w w` round trip retains it when the visible set is unchanged and re-clamps it on re-entry when the set changed while wrapped. `internal/app/pan_test.go` proves the key wiring through `Update` and the file-change reset.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestPanUnits|TestSetOffsetClamps|TestPanNoOpInWrapMode|TestPanWithoutContentIsNoOp|TestOffsetRetainedThroughWrapToggle|TestWrapReentry" ./internal/viewport 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//" && go test -count=1 -v -run "TestPanKeysMoveOffset|TestPanKeysNoOpInWrapMode|TestPanOffsetSurvivesWrapToggle|TestPanOffsetResetsOnFileChange" ./internal/app 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestPanUnits
    --- PASS: TestPanUnits/right_pans_one_column
    --- PASS: TestPanUnits/left_pans_one_column
    --- PASS: TestPanUnits/ten_columns_right
    --- PASS: TestPanUnits/ten_columns_left
    --- PASS: TestPanUnits/half_the_text_width_right
    --- PASS: TestPanUnits/half_of_odd_width_floors
    --- PASS: TestPanUnits/half_the_text_width_left
    --- PASS: TestPanUnits/half_of_width_one_is_one_column
    --- PASS: TestPanUnits/right_clamps_at_the_extent_maximum
    --- PASS: TestPanUnits/ten_right_clamps_at_the_extent_maximum
    --- PASS: TestPanUnits/half_right_clamps_at_the_extent_maximum
    --- PASS: TestPanUnits/left_clamps_at_zero
    --- PASS: TestPanUnits/ten_left_clamps_at_zero
    --- PASS: TestPanUnits/half_left_clamps_at_zero
--- PASS: TestSetOffsetClamps
--- PASS: TestPanNoOpInWrapMode
--- PASS: TestPanWithoutContentIsNoOp
--- PASS: TestOffsetRetainedThroughWrapToggle
--- PASS: TestWrapReentryClampsToNewVisibleSet
--- PASS: TestWrapReentryClampsAfterWidthChange
PASS
ok  	vrg/internal/viewport
--- PASS: TestPanKeysMoveOffset
--- PASS: TestPanKeysNoOpInWrapMode
--- PASS: TestPanOffsetSurvivesWrapToggle
--- PASS: TestPanOffsetResetsOnFileChange
PASS
ok  	vrg/internal/app
```

## Extent policy, paintable boundary, and clipping tests

The visible-lines extent tests cover the full policy: the mixed-width fixture's 299 maximum while the 300-cell line is visible collapsing to 9 once it scrolls out (with no restoration on return); empty buffer, placeholder, and all-empty views clamping to 0; the paintable boundary stopping at a two-cell final cluster's start — with a render-level assertion that both cells paint there — and falling back to the last fitting cluster when the final cluster cannot fit, or 0 when none can (the documented unpaintable-cluster exception, whose in-window cells render blank by design). Re-clamping fires on every visible-set change — scroll, moving reveal, width resize (the same route a list hide/show or gutter growth takes), and a row-model swap — and every pan recomputes the maximum from the now-visible rows. Clipping blanks a cluster split by either window edge and translates spans into window cells; the uniform-lines test pins the hidden-left geometry Issue #20's `_` indicators need; and the counting-fake guard proves extent evaluation queries only the visible rows of the prepared layout.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestExtent|TestPaintableBoundary|TestReclampOnVisibleSetChanges|TestSplitClusterClipsToBlankCells|TestClipTranslatesSpans|TestUniformLinesAllHiddenLeft" ./internal/viewport 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//" && go test -count=1 -v -run "TestHalfClippedGlyphPaintsBlank|TestPanToMaximumPaintsFinalCluster" ./internal/app 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestExtentFollowsWidestVisibleLine
--- PASS: TestExtentEmptyViewsClampToZero
    --- PASS: TestExtentEmptyViewsClampToZero/placeholder:_no_prepared_rows
    --- PASS: TestExtentEmptyViewsClampToZero/empty_buffer
    --- PASS: TestExtentEmptyViewsClampToZero/all-empty_lines
--- PASS: TestPaintableBoundaryStopsAtFinalCluster
--- PASS: TestPaintableBoundaryUnfittableFinalCluster
--- PASS: TestReclampOnVisibleSetChanges
    --- PASS: TestReclampOnVisibleSetChanges/vertical_scroll
    --- PASS: TestReclampOnVisibleSetChanges/moving_reveal
    --- PASS: TestReclampOnVisibleSetChanges/width_resize_re-fits_clusters
    --- PASS: TestReclampOnVisibleSetChanges/row-model_swap
--- PASS: TestSplitClusterClipsToBlankCells
    --- PASS: TestSplitClusterClipsToBlankCells/no_clip_paints_whole
    --- PASS: TestSplitClusterClipsToBlankCells/right_edge_splits_世
    --- PASS: TestSplitClusterClipsToBlankCells/世_whole_inside_the_window
    --- PASS: TestSplitClusterClipsToBlankCells/世_leads_the_window
    --- PASS: TestSplitClusterClipsToBlankCells/left_edge_splits_世
    --- PASS: TestSplitClusterClipsToBlankCells/window_on_世's_lead_cell_alone
--- PASS: TestClipTranslatesSpans
--- PASS: TestUniformLinesAllHiddenLeft
--- PASS: TestExtentQueriesOnlyVisibleRows
PASS
ok  	vrg/internal/viewport
--- PASS: TestHalfClippedGlyphPaintsBlank
--- PASS: TestPanToMaximumPaintsFinalCluster
PASS
ok  	vrg/internal/app
```

## Manual check — panning on a real PTY

`demo-pan.sh` (checked into this directory) runs the freshly built `vrg` on a real tmux PTY with a fake `rg` feeding a two-file fixture at 80x24 (7-cell list, 4-cell gutter, 68-cell text area). `a.txt` line 2 is 300 cells (`0123456789abcdefghij` + x's) and line 3 is `ab世` + 291 c's — 世 occupying cells 2-3; lines 4-46 are ten-cell s-lines. `b.txt` line 2 is `ab世` + 76 x's + `世` — 82 cells, the final cluster at cells 80-81, so its paintable boundary is 80. The run demonstrates: `,` at offset 0 inert; `.` shifting text left one column; `>` ten; `]` half the text width (34); the half-clipped 世 painting blank; pans clamping at the 299 maximum and staying inert past it; `w w` retaining the offset through a real wrap-layout round trip; scrolling into the short lines re-clamping the offset left to 9 with no restoration on return; `n` into b.txt starting at offset 0; and the paintable-boundary maximum landing on 世's start so the final cluster paints whole.

```bash
cd /home/chris/vrg/Notes/walkthroughs/018-04/code-walkthrough && ./demo-pan.sh
```

```output
ok: startup: file panel -> a.txt
ok: w: run-off-edge line 2 leads from cell 0 -> 0123456789
ok: , at offset 0 does nothing -> 0123456789abcdefghijxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
ok: . pans one column -> 1
ok: offset 3: line 2 from cell 3 -> 3
ok: half-clipped 世 paints blank ->  c
ok: > pans ten columns -> d
ok: ] pans half the text width (34) -> x
ok: maximum offset paints only the last cell -> x
ok: pans past the maximum do nothing -> x
ok: w: wrapped continuation shows a blank gutter ->     
ok: w w: offset retained at the maximum -> x
ok: scroll into short lines re-clamps to 9 -> 4
ok: scroll back does not restore: offset stays 9 -> 9abcdefghij
ok: n: file change starts at offset 0 -> ab
ok: maximum leaves the final 世 whole -> 世
ok: pans past the boundary do nothing -> 世
ok: exit status -> 0
demo-pan: all checks passed
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/018-04/code-walkthrough && echo "== offset 0: line 2 leads from cell 0 ==" && sed -n "3p" pan/screen-01-runoff.txt | sed "s/ *$//" && echo "== offset 3: line 3's 世 (cells 2-3) is split — its trailing cell paints blank ==" && sed -n "3,4p" pan/screen-02-halfclip.txt | sed "s/ *$//" && echo "== clamped at the 299 maximum: only the widest line's last cell paints ==" && sed -n "3,4p" pan/screen-03-max.txt | sed "s/ *$//" && echo "== w: wrap mode — line 2 is five rows with blank-gutter continuations ==" && sed -n "3,5p" pan/screen-04-wrap.txt | sed "s/ *$//" && echo "== w back: the offset was dormant — still at 299 ==" && sed -n "3p" pan/screen-05-retained.txt | sed "s/ *$//" && echo "== scrolled into the ten-cell lines: re-clamped to offset 9 (each line's last digit) ==" && sed -n "2,4p" pan/screen-06-reclamp.txt | sed "s/ *$//" && echo "== scrolled back: the offset stays 9 — no restoration ==" && sed -n "3p" pan/screen-07-norestore.txt | sed "s/ *$//" && echo "== n into b.txt: offset reset to 0 — line 2 leads with ab世 ==" && sed -n "3p" pan/screen-08-filechange.txt | sed "s/ *$//" && echo "== b.txt maximum = 80: the final 世 (cells 80-81) paints whole ==" && sed -n "3p" pan/screen-09-cluster-max.txt | sed "s/ *$//"
```

```output
== offset 0: line 2 leads from cell 0 ==
        2  0123456789abcdefghijxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
== offset 3: line 3's 世 (cells 2-3) is split — its trailing cell paints blank ==
        2  3456789abcdefghijxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
        3   ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
== clamped at the 299 maximum: only the widest line's last cell paints ==
        2  x
        3
== w: wrap mode — line 2 is five rows with blank-gutter continuations ==
        2  0123456789abcdefghijxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
           xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
           xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
== w back: the offset was dormant — still at 299 ==
        2  x
== scrolled into the ten-cell lines: re-clamped to offset 9 (each line's last digit) ==
b.txt   4  4
        5  5
        6  6
== scrolled back: the offset stays 9 — no restoration ==
        2  9abcdefghijxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
== n into b.txt: offset reset to 0 — line 2 leads with ab世 ==
        2  ab世xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
== b.txt maximum = 80: the final 世 (cells 80-81) paints whole ==
        2  世
```

## Verdict

Issue #18 is verified. The six pan keys move the offset by their contracted units — one column, ten columns, `max(1, floor(text width / 2))` — clamped to the visible-lines extent: the maximum is the widest *currently rendered* line's paintable boundary, so it always leaves one whole cluster painted and is recomputed on every pan and re-clamped on every visible-set change, with permanent loss rather than restoration. The offset is dormant under wrap, retained through `w w` with re-entry clamping, and reset to zero on file change and current-file load completion ahead of Issue #19's horizontal reveal. Clipping is grapheme-safe: a cluster split by either window edge paints blank cells — demonstrated live by the half-clipped 世 — and at the maximum the final cluster paints whole, as the `世`-terminated b.txt row shows.

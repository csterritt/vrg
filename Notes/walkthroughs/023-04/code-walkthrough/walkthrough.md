# Issue #23: Zero-width match markers

*2026-09-24T14:18:24Z by Showboat 0.6.1*
<!-- showboat-id: 6c1cab05-6fe7-470c-987e-1e3d590d8956 -->

Walkthrough for [Issue #23](../../../issues/023-zero-width-match-markers.md), implementing zero-width match markers per `Notes/PRD-vrg.md` (*Text, graphemes, and safe presentation*): a zero-width submatch — `^`, `$`, an empty look-around — becomes a one-cell inverse-video marker at its mapped display position, underlined on the current matched line, marking an existing cell without shifting following text. A marker inside a grapheme cluster lands on the cluster's first cell so a wide glyph is never split; a position on removed terminator bytes or at end of line is an end-of-line marker that extends the line's effective width by one cell — a marker-only line has extent 1 — and a marker after a completely full wrap row occupies another row. Markers count as one-cell paintable units in the Issue #18 extent and paintable-boundary maximum, are navigable reveal targets, and feed the Issue #20 hidden-content indicators as entirely hidden one-cell units; a terminator-only `$` on `hit\r\n` maps to display column 3 and follows every ordinary marker rule. All generated artifacts live in this directory: the built `vrg` binary, the `demo-markers.sh` tmux harness, and its `markers/` session captures. Test durations are stripped so the document verifies cleanly.

```bash
set -o pipefail; cd /home/chris/vrg && test -z "$(gofmt -l internal/)" && echo "GOFMT-CLEAN" && go vet ./... && go build ./... && go test -count=1 ./... | sed "s/[[:space:]][0-9.]*s$//" && CGO_ENABLED=1 go test -race -count=1 ./internal/app ./internal/viewport ./internal/filebuffer ./internal/present | sed "s/[[:space:]][0-9.]*s$//" && go build -o Notes/walkthroughs/023-04/code-walkthrough/vrg ./cmd/vrg && echo GATES-OK
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
ok  	vrg/internal/filebuffer
ok  	vrg/internal/present
GATES-OK
```

## Marker position tests

`internal/filebuffer/marker_test.go` pins the zero-width contracts at `Load`: `TestZeroWidthMarkerPositions` maps a `{start,start}` submatch to a marker span at beginning of line, mid-line, and end of line — a position on any byte of `a世b`'s wide cluster or the emoji ZWJ cluster lands on the cluster's first cell (the glyph is never split), and a position on the LF of `hit\r\n` or a range covering the CRLF/LF lands on display column 3; `TestMarkersOnEveryLine` accumulates one BOL marker per line across a stop set, empty lines included.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestZeroWidthMarkerPositions|TestMarkersOnEveryLine" ./internal/filebuffer 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestZeroWidthMarkerPositions
    --- PASS: TestZeroWidthMarkerPositions/bol_on_a_text_line
    --- PASS: TestZeroWidthMarkerPositions/bol_on_an_empty_line
    --- PASS: TestZeroWidthMarkerPositions/eol_on_a_text_line
    --- PASS: TestZeroWidthMarkerPositions/mid-line_position
    --- PASS: TestZeroWidthMarkerPositions/inside_a_wide_cluster's_first_byte
    --- PASS: TestZeroWidthMarkerPositions/inside_a_wide_cluster's_middle_byte
    --- PASS: TestZeroWidthMarkerPositions/inside_a_wide_cluster's_last_byte
    --- PASS: TestZeroWidthMarkerPositions/inside_a_zwj_cluster
    --- PASS: TestZeroWidthMarkerPositions/at_the_lf_of_a_crlf
    --- PASS: TestZeroWidthMarkerPositions/covering_a_crlf
    --- PASS: TestZeroWidthMarkerPositions/covering_an_lf
--- PASS: TestMarkersOnEveryLine
PASS
ok  	vrg/internal/filebuffer
```

## Viewport marker tests

`internal/viewport/marker_test.go` pins the viewport contracts: `TestMarkerExtentAndPanClamp` — a marker is a one-cell paintable unit that extends its line's extent (an end-of-line marker makes `"ab"` extent 3 and is itself the boundary at cell 2) and remains a paintable boundary past an unfittable final cluster; `TestMarkerOnlyLineExtent` — a marker-only line has extent 1, so its pan maximum is 0; `TestMarkerHiddenDrivesIndicators` — an entirely hidden marker upgrades the gutter signpost to `*` even with no text to hide; `TestTerminatorMarkerIsOrdinary` — the `hit\r\n` `$` marker at display column 3 follows the ordinary extent, reveal, clip, indicator, and wrap rules (a width-3 row fills completely, so the marker occupies a continuation row); `TestInteriorMarkers` — BOL and mid-line markers ride the same clip, indicator, and pan rules. The pre-existing `TestEndOfLineMarkerRows` (wrap rows), `TestHRevealMarkerCells` (reveal targeting), and the app's `TestRenderCellsBlankFillersNeverMatchStyled` (a marker still paints its own cell on a clip-blanked wide glyph) cover the wrap-row, navigation, and rendered-cell sides.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestMarkerExtentAndPanClamp|TestMarkerOnlyLineExtent|TestMarkerHiddenDrivesIndicators|TestTerminatorMarkerIsOrdinary|TestInteriorMarkers|TestEndOfLineMarkerRows|TestHRevealMarkerCells" ./internal/viewport 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//" && go test -count=1 -v -run "TestRenderCellsBlankFillersNeverMatchStyled" ./internal/app 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestHRevealMarkerCells
--- PASS: TestMarkerExtentAndPanClamp
--- PASS: TestMarkerOnlyLineExtent
--- PASS: TestMarkerHiddenDrivesIndicators
--- PASS: TestTerminatorMarkerIsOrdinary
--- PASS: TestInteriorMarkers
--- PASS: TestEndOfLineMarkerRows
PASS
ok  	vrg/internal/viewport
--- PASS: TestRenderCellsBlankFillersNeverMatchStyled
    --- PASS: TestRenderCellsBlankFillersNeverMatchStyled/covered_blank_stays_plain
    --- PASS: TestRenderCellsBlankFillersNeverMatchStyled/clip-edge_split_blank_stays_plain
    --- PASS: TestRenderCellsBlankFillersNeverMatchStyled/marker_on_a_blank_still_paints
PASS
ok  	vrg/internal/app
```

## Manual check — markers on a real PTY

`demo-markers.sh` (checked into this directory) runs the freshly built `vrg` on a real tmux PTY with real `rg` at 80x24 (7-cell list, 3-cell gutter with the indicator cell at column 9, 69-cell text area, reserved indicator column at column 80). The fixture `a.txt` is `"alpha"`, an empty line, `"hit\r\n"`, and `"omega"`. Run A — `vrg '^' a.txt` — shows an inverse cell at column 0 of every line including the empty one (the marker marks the cell; `"lpha"` follows unshifted), `n` navigating between the marker stops, and `w` + `>` panning to offset 4 where every line's marker is entirely hidden left and every gutter shows `*`. Run B — `vrg '$' a.txt` — shows an inverse cell one past each line's last character, the `hit\r\n` terminator-only marker at display column 3, the marker-only empty line, `n` navigation, and `w` + `>` panning to offset 5 where lines whose markers stay visible keep `_` while lines 2 and 3 — markers entirely hidden left — show `*`.

```bash
cd /home/chris/vrg/Notes/walkthroughs/023-04/code-walkthrough && ./demo-markers.sh
```

```output
ok: wrap '^': line 1's first cell is the marker, 'lpha' unshifted ->  lpha
ok: wrap '^': line 1's marker is the current match — inverse + underline
ok: wrap '^': the empty line 2's marker cell paints alone -> 
ok: wrap '^': the empty line 2 carries a marker cell
ok: wrap '^': line 3 'it' follows the cell-0 marker unshifted ->  it
ok: wrap '^': line 3 paints its marker
ok: wrap '^': line 4 paints its marker
ok: n -> line 2's marker is the current match
ok: n -> line 3's marker is the current match
ok: n: line 1's marker is no longer the current match
ok: offset 4: line 1's marker hidden left -> '*' -> *
ok: offset 4: the marker-only line 2's marker hidden left -> '*' -> *
ok: offset 4: line 3's marker hidden left -> '*' -> *
ok: offset 4: line 4's marker hidden left -> '*' -> *
ok: offset 4: line 1 shows only its last cell -> a
ok: vrg '^' exit status -> 0
ok: wrap '$': line 1's text unchanged -> alpha
ok: wrap '$': line 1's marker paints one cell past 'alpha'
ok: wrap '$': line 1's marker is the current match — inverse + underline
ok: wrap '$': the empty line 2's marker is its only cell
ok: wrap '$': the empty line 2 shows nothing else -> 
ok: wrap '$': line 3 displays 'hit' — the CRLF is undisplayed -> hit
ok: wrap '$': the terminator-only marker lands on column 3, right after 'hit'
ok: wrap '$': line 4's marker paints one cell past 'omega'
ok: n -> line 2's marker is the current match
ok: offset 5: line 1's marker stays visible at the window's edge -> '_' -> _
ok: offset 5: line 1's marker still paints its cell
ok: offset 5: the marker-only line 2's marker hidden left -> '*' -> *
ok: offset 5: line 3's terminator marker hidden left -> '*' -> *
ok: offset 5: line 4's marker stays visible -> '_' -> _
ok: vrg '$' exit status -> 0
demo-markers: all checks passed
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/023-04/code-walkthrough && echo "== wrap: vrg ^ a.txt — inverse cell at column 0 of every line (the marker cell renders blank in plain capture) ==" && sed -n "2,5p" markers/screen-01-caret-wrap.txt | sed "s/ *$//" && echo "== run-off-edge offset 4: every marker hidden left -> all gutters * ==" && sed -n "2,5p" markers/screen-03-caret-panned.txt | sed "s/ *$//" && echo "== wrap: vrg \$ a.txt — marker one cell past each last char; the empty line and hit CRLF included ==" && sed -n "2,5p" markers/screen-04-dollar-wrap.txt | sed "s/ *$//" && echo "== run-off-edge offset 5: markers visible -> _; markers hidden left -> * ==" && sed -n "2,5p" markers/screen-05-dollar-panned.txt | sed "s/ *$//"
```

```output
== wrap: vrg ^ a.txt — inverse cell at column 0 of every line (the marker cell renders blank in plain capture) ==
       1   lpha
       2
       3   it
       4   mega
== run-off-edge offset 4: every marker hidden left -> all gutters * ==
       1* a
       2*
       3*
       4* a
== wrap: vrg $ a.txt — marker one cell past each last char; the empty line and hit CRLF included ==
       1  alpha
       2
       3  hit
       4  omega
== run-off-edge offset 5: markers visible -> _; markers hidden left -> * ==
       1_
       2*
       3*
       4_
```

## Verdict

Issue #23 is verified. A zero-width submatch renders as one inverse-video cell at its mapped position — underlined on the current matched line — marking an existing cell without shifting following text. Positions inside wide or ZWJ clusters land on the cluster's first cell; positions on removed terminator bytes or at end of line mark the display end-of-line position — `vrg '$'` on `hit\r\n` paints its marker at column 3 — and end-of-line markers extend the effective line width by one cell, so a marker-only line has extent 1 and pan maximum 0, and a marker after a completely full wrap row occupies a continuation row. Markers are navigable reveal targets (`n` walks them), count as one-cell units in the Issue #18 paintable-boundary maximum, and feed the Issue #20 indicators: on the real binary, panning past `^` markers set every gutter to `*` — the empty line's marker included — while panning `$` markers to the window edge left `_` on lines whose marker still painted and `*` where the marker was entirely hidden.


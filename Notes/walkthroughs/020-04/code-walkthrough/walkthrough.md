# Issue #20: Hidden-content indicators

*2026-09-24T12:39:17Z by Showboat 0.6.1*
<!-- showboat-id: f2ecc9ae-d5bd-4714-b4b7-5bd22b7e6eb4 -->

Walkthrough for [Issue #20](../../../issues/020-hidden-content-indicators.md), implementing the hidden-content indicators per `Notes/PRD-vrg.md` (*Layout and indicators*): in run-off-edge mode, every visible source line's first trailing gutter space shows an inverse `_` when any of its text is hidden left of the window — upgraded to an inverse `*` when a match or marker on that line is entirely hidden left — and the reserved rightmost column shows an inverse `*` on the current matched line's row when a match or marker there is entirely hidden right. Visibility is computed from the actually rendered cells after grapheme-safe clipping (`hiddenMarks` inside `clipRow`): a partially painted match counts as visible, a match on a clip-edge-split cluster's blanked cells counts as entirely hidden, a marker paints wherever it sits, and the reserved column is excluded from the window. Wrap mode draws neither indicators nor the reserved column. All generated artifacts live in this directory: the built `vrg` binary, the `demo-indicators.sh` tmux harness, and its `indicators/` session captures. Test durations are stripped so the document verifies cleanly.

```bash
set -o pipefail; cd /home/chris/vrg && test -z "$(gofmt -l internal/)" && echo "GOFMT-CLEAN" && go vet ./... && go build ./... && go test -count=1 ./... | sed "s/[[:space:]][0-9.]*s$//" && CGO_ENABLED=1 go test -race -count=1 ./internal/app ./internal/viewport | sed "s/[[:space:]][0-9.]*s$//" && go build -o Notes/walkthroughs/020-04/code-walkthrough/vrg ./cmd/vrg && echo GATES-OK
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

## Visibility-flag tests

`internal/viewport/indicators_test.go` pins the `Row` indicator flags against real prepared row models: `HiddenLeft` reports any of the line's text hidden left (a fully hidden line signposts; an empty line stays blank); `MatchHiddenLeft`/`MatchHiddenRight` report a match or marker *entirely* hidden on that side — half-painted matches and in-window markers count as visible, a match on a two-cell cluster split by a clip edge counts as entirely hidden on that side, and the Issue #19 unpaintable cluster reports hidden right. Wrap-mode rows carry no flags at all.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestHiddenLeft|TestMatchHidden|TestHiddenBothSides|TestSplitClusterBlank|TestUnpaintableCluster|TestWrapModeNoIndicatorFlags" ./internal/viewport 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestHiddenLeftTextFlag
    --- PASS: TestHiddenLeftTextFlag/offset_zero_hides_nothing
    --- PASS: TestHiddenLeftTextFlag/offset_hides_the_head
    --- PASS: TestHiddenLeftTextFlag/offset_at_the_last_cell_still_hides
--- PASS: TestMatchHiddenLeftFlag
    --- PASS: TestMatchHiddenLeftFlag/match_fully_painted_at_offset_zero
    --- PASS: TestMatchHiddenLeftFlag/match_half_hidden_is_visible
    --- PASS: TestMatchHiddenLeftFlag/match_flush_with_the_window_edge_is_visible
    --- PASS: TestMatchHiddenLeftFlag/match_entirely_hidden_left
    --- PASS: TestMatchHiddenLeftFlag/marker_hidden_left
    --- PASS: TestMatchHiddenLeftFlag/marker_at_the_window's_first_cell_is_visible
--- PASS: TestMatchHiddenRightFlag
    --- PASS: TestMatchHiddenRightFlag/match_entirely_hidden_right
    --- PASS: TestMatchHiddenRightFlag/match_half_hidden_right_is_visible
    --- PASS: TestMatchHiddenRightFlag/match_ending_on_the_last_text_cell_is_visible
    --- PASS: TestMatchHiddenRightFlag/marker_hidden_right
    --- PASS: TestMatchHiddenRightFlag/marker_on_the_first_column_past_the_window
    --- PASS: TestMatchHiddenRightFlag/marker_inside_the_window_is_visible
--- PASS: TestHiddenBothSidesFlags
--- PASS: TestSplitClusterBlankCountsAsHidden
--- PASS: TestUnpaintableClusterCountsNotVisible
--- PASS: TestWrapModeNoIndicatorFlags
PASS
ok  	vrg/internal/viewport
```

## Rendered-frame tests

`internal/app/indicators_test.go` drives the indicators through `Update` and the composed frame at 80x24 (7-cell list, digit-width gutter, text area, one reserved column): per-line `_`/`*`/blank gutters including the empty line and the Issue #18 uniform-lines geometry; the theme's inverse pair for both marks; the reserved column's `*` on the current matched line only — blank for non-current matched lines, unmatched lines, and when the current line scrolls off-screen; both stars together on one row; partial visibility drawing no star on either side; a match ending on the last text cell painted beside a farther match's `*` (the indicator never overwrites text); split-glyph blanks earning stars; and wrap mode drawing no indicators and no reserved column.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestGutter|TestIndicators|TestRightStar|TestBothSides|TestPartial|TestLastCell|TestSplitGlyph|TestWrapModeDraws|TestUniformLinesEvery" ./internal/app 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestGutterUnderscoreStarAndBlank
--- PASS: TestIndicatorsPaintInverse
--- PASS: TestRightStarCurrentMatchedLineOnly
--- PASS: TestRightStarAbsentWhenCurrentLineOffScreen
--- PASS: TestBothSidesHiddenStarsTogether
--- PASS: TestPartialMatchVisibilityDrawsNoStar
--- PASS: TestLastCellMatchAndFarMatchRightStar
--- PASS: TestSplitGlyphBlanksDrawStars
--- PASS: TestWrapModeDrawsNoIndicatorsOrReservedColumn
--- PASS: TestUniformLinesEveryGutterUnderscore
PASS
ok  	vrg/internal/app
```

## Manual check — indicators on a real PTY

`demo-indicators.sh` (checked into this directory) runs the freshly built `vrg` on a real tmux PTY with a fake `rg` feeding a one-file fixture at 80x24 (7-cell list, 4-cell gutter with the indicator cell at column 10, 68-cell text area, reserved indicator column at column 80). `a.txt` has several long matched lines: line 1 `"hit"` + 150 x's + `"far"` (matches at cells 0-2 and 153-155), line 2 the same over y's, line 3 of 150 z's + `"far"`, line 4 of 5 x's + `"mid"` + 62 x's + `"far"` (cells 5-7 and 70-72), and eight nine-cell filler lines. The run demonstrates: startup in wrap mode with no indicators and no reserved column; `w` into run-off-edge showing the current line's reserved `*` for the hidden-right `"far"`; `>` panning ten columns so `"hit"`/`"mid"` matches hidden left upgrade their gutters to `*` while plain text hides behind `_`; `n` to line 4 revealing `"mid"`; `.` once leaving `"mid"` partially painted — the gutter stays `_`; `.` twice more hiding it entirely — `*`; `<` back to offset 0 earning the right `*`; `.` three times poking one cell of `"far"` into the window — partially visible, so the reserved cell clears; and `w` back to wrap clearing every indicator.

```bash
cd /home/chris/vrg/Notes/walkthroughs/020-04/code-walkthrough && ./demo-indicators.sh
```

```output
ok: wrap: line 1's gutter indicator cell stays blank ->  
ok: wrap: no reserved column — text reaches the last cell -> x
ok: offset 0: current line 1 — 'far' hidden right -> reserved '*' -> *
ok: offset 0: line 3's 'far' hidden right but it is not current ->  
ok: offset 0: nothing hidden left — the gutters stay blank ->  
ok: offset 10: line 1's 'hit' entirely hidden left -> '*' -> *
ok: offset 10: line 1 still hides 'far' right -> both stars -> *
ok: offset 10: line 2's 'hit' hidden left -> '*' -> *
ok: offset 10: line 3 hides only text left -> '_' -> _
ok: offset 10: line 4's 'mid' (cells 5-7) hidden left -> '*' -> *
ok: offset 10: line 5's nine cells all hidden left -> '_' -> _
ok: offset 10: line 3's hidden-right 'far' draws nothing — not current ->  
ok: n x3 -> line 4 is the current matched line
ok: offset 5: 'mid' and 'far' both fully painted — no stars ->  
ok: offset 5: cells 0-4 hidden left -> '_' -> _
ok: offset 6: 'mid' one cell hidden — partially visible -> '_' -> _
ok: offset 8: 'mid' entirely hidden left -> '*' -> *
ok: offset 0: 'far' entirely hidden right on the current line -> '*' -> *
ok: offset 3: 'far' one cell painted — partially visible -> blank ->  
ok: offset 3: 'mid' still fully painted — gutter '_' -> _
ok: wrap again: no gutter indicator on any row ->    
ok: wrap again: no reserved column — text at the last cell -> x
ok: exit status -> 0
demo-indicators: all checks passed
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/020-04/code-walkthrough && echo "== wrap mode at startup: no indicators; wrapped text fills the last cell ==" && sed -n "2,4p" indicators/screen-01-wrap.txt | sed "s/ *$//" && echo "== run-off-edge offset 0: current line 1 hides far right -> col-80 *; other lines blank ==" && sed -n "2,5p" indicators/screen-02-offset0.txt | sed "s/ *$//" && echo "== offset 10: hit/mid hidden left upgrade gutters to *; text-only lines show _; the current line keeps both stars ==" && sed -n "2,6p" indicators/screen-03-panned.txt | sed "s/ *$//" && echo "== offset 8 on line 4: mid entirely hidden left -> gutter * ==" && sed -n "5p" indicators/screen-04-mid-hidden.txt | sed "s/ *$//" && echo "== offset 3 on line 4: one cell of far pokes into the window — partially visible -> reserved cell blank ==" && sed -n "5p" indicators/screen-05-far-partial.txt | sed "s/ *$//" && echo "== w back to wrap: indicators gone, no reserved column ==" && sed -n "2,4p" indicators/screen-06-wrap.txt | sed "s/ *$//"
```

```output
== wrap mode at startup: no indicators; wrapped text fills the last cell ==
        1  hitxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
           xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
           xxxxxxxxxxxxxxxfar
== run-off-edge offset 0: current line 1 hides far right -> col-80 *; other lines blank ==
        1  hitxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx*
        2  hityyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy
        3  zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz
        4  xxxxxmidxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
== offset 10: hit/mid hidden left upgrade gutters to *; text-only lines show _; the current line keeps both stars ==
        1* xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx*
        2* yyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy
        3_ zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz
        4* xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxfar
        5_
== offset 8 on line 4: mid entirely hidden left -> gutter * ==
        4* xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxfar
== offset 3 on line 4: one cell of far pokes into the window — partially visible -> reserved cell blank ==
        4_ xxmidxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxf
== w back to wrap: indicators gone, no reserved column ==
        1  hitxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
           xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
           xxxxxxxxxxxxxxxfar
```

## Verdict

Issue #20 is verified. In run-off-edge mode every visible source line's first trailing gutter space signposts hidden-left content — `_` for hidden text, `*` when a match or marker on that line is entirely hidden left, blank otherwise — and the reserved rightmost column paints `*` only on the current matched line's row when a match or marker is entirely hidden right, disappearing when that line scrolls off-screen. Visibility is painted-cell visibility after grapheme clipping: a partially painted match counts as visible (no star for that side), a match on a clip-edge-split cluster's blanked cells counts as hidden, and the indicator column is excluded from the window. On the real binary the panned frame showed `*` gutters on lines whose matches hid left, `_` on lines hiding only text, both stars together on the current line, the right `*` clearing the moment one cell of `"far"` poked into the window, and wrap mode drawing neither indicators nor the reserved column.


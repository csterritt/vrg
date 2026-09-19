# Issue #20: hidden-content indicators in run-off-edge mode

*2026-09-17T20:41:50Z by Showboat 0.6.1*
<!-- showboat-id: 48d46a8b-1904-460c-99bf-a4e3bff6ba7f -->

Issue #20 lands the hidden-content indicators: in run-off-edge mode every visible source line's gutter marks what its row hides on the left — '_' when ordinary text is hidden, '*' when a match or end-of-line marker is entirely hidden — and the reserved rightmost column shows '*' on the current matched line alone when a match or marker is entirely hidden to the right. Visibility is the same painted-cell visibility Issue #19 defined: viewport.CellVisible requires the whole grapheme cluster holding a cell to fit inside the text window, so a position geometrically inside but clipped to a blank still counts as hidden, and a partial match at either edge earns no indicator. internal/viewport/indicators.go derives the marks from that oracle — LeftMark upgrades '_' to '*' when a highlight span's cells are entirely hidden left, RightMark reports '*' when a span is entirely hidden right — and model.contentCell renders them: the gutter's first trailing space becomes the left slot (only that cell is replaced, so the line number never moves), and the already-reserved rightmost column gains the themed star for the current row only. Both marks render through Theme.Indicator, the same inverse styling as match highlighting. Wrap mode draws nothing: no gutter marks, no reserved column, the text area claims the cell the flat layout reserves. See Notes/issues/020-hidden-content-indicators.md, Notes/tasks/020-hidden-content-indicators.md, and the PRD sections "Layout and indicators" and "Navigation, viewport, and logical anchors" in Notes/PRD-vrg.md. All artifacts live in this directory.

## Gates — module integrity, build, vet

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

## Indicator arithmetic — the marks over real prepared buffers

internal/viewport/indicators_test.go pins the contract: LeftMark is blank when nothing hides left, '_' when text hides left, and '*' when a highlight span's cells are entirely hidden left — a span straddling the left edge stays '_' because it is only partially hidden; RightMark is '*' only when a span is entirely hidden right, blank for a straddling span or for ordinary text; a zero-width span judges at its one-cell marker position; and every flat row's left mark reflects its own line, so a line with no left-hidden content contributes a blank while its neighbours mark.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'LeftMark|RightMark' ./internal/viewport 2>&1 | grep -E '^(--- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestLeftMark
--- PASS: TestRightMark
--- PASS: TestLeftMarkOnEveryFlatRow
ok  	vrg/internal/viewport
```

## App wiring — gutters, the reserved column, styling, wrap

internal/app/indicators_test.go drives the same contracts through Update/View: every visible row's gutter marks independently; the right '*' follows the current matched line and blanks when that line scrolls off-screen; left and right stars appear together on the current line; a partially visible match suppresses both indicators; a match painted in the last text cell stays visible while a farther hidden match still stars the right column; a wide glyph clipped to blanks at the window edge counts its hidden half as hidden; a uniform buffer marks every visible row; wrap mode draws no marks and no reserved column; and the marks render with the dark theme's inverse styling.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'GutterMark|RightStar|BothStars|PartiallyVisibleMatch|LastCellMatch|SplitGlyphBlanks|UniformLinesMark|WrapModeDrawsNoIndicators|IndicatorsRenderInverse' ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestGutterMarkPerVisibleLine
--- PASS: TestRightStarFollowsCurrentLine
--- PASS: TestRightStarAbsentWhenCurrentLineOffScreen
--- PASS: TestBothStarsAppearTogether
--- PASS: TestPartiallyVisibleMatchShowsNoStar
--- PASS: TestLastCellMatchWithFartherMatchHidden
--- PASS: TestSplitGlyphBlanksCountHidden
--- PASS: TestUniformLinesMarkEveryVisibleRow
--- PASS: TestWrapModeDrawsNoIndicators
--- PASS: TestIndicatorsRenderInverseStyled
ok  	vrg/internal/app
```

## Manual check — the real binary on a pty

pty_indicators.py runs the built vrg on a real 100x24 pty and replays the byte stream through a small terminal emulator, over one generated fixture file whose five lines all exceed the flat text width tw (derived from the runtime file-list width): line 1 holds a visible cell-0 match plus a far match at tw+36; line 2 a cell-10 match plus a far match at tw+5 that straddles the right edge at offset 10; line 3 a cell-4 match entirely hidden left once panned; line 4 a cell-20 match that stays painted; and line 5 no match at all. The session toggles w into run-off-edge — the current line's far match stars the reserved right column alone — then > pans to offset 10 where every gutter marks ('*' where a match is entirely hidden left, '_' elsewhere) while both stars sit together on the current line and line 2's half-visible far match earns none; n moves to line 2, whose straddling far match shows no star while the departed line 1's right column blanks; five commas pan back to offset 5 where the far match is entirely hidden right and the star appears; and a final w restores wrap mode — no marks, no reserved column, the text claiming the last cell.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/020-04/code-walkthrough/vrg ./cmd/vrg && python3 Notes/walkthroughs/020-04/code-walkthrough/pty_indicators.py
```

```output
setup    : 29-col file list, 67-cell flat text width
w        : run-off-edge — current line's far match hidden right — '1  needlex'…'xx*'
>        : row 1 gutter '*' right '*'
>        : row 2 gutter '_' right ' '
>        : row 3 gutter '*' right ' '
>        : row 4 gutter '_' right ' '
>        : row 5 gutter '_' right ' '
>        : both stars on the current line — '1* xxxx'…'xx*'
n        : current line 2 — far match half-visible, no star — '2_ need'…'dl '
,,,,,    : far match now entirely hidden right — '2_ xxxx'…'xx*'
w        : wrap mode — no marks, no reserved column — '1  need'…'xxx'
q        : exit 0
OK
```

## Full suite

```bash
cd /home/chris/vrg && go test -count=1 ./... 2>&1 | grep -vE '^(=== RUN|--- PASS|=== PAUSE|=== CONT|    )' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
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

All Issue #20 contracts verified: in run-off-edge mode every visible source line's gutter reports its left-hidden content — '_' for hidden text, '*' when a match or end-of-line marker is entirely hidden — and the reserved rightmost column stars the current matched line alone when a match or marker is entirely hidden right. Visibility is painted-cell visibility, so a cluster clipped to blanks at either edge counts its hidden cells as hidden, a partial match earns no indicator on that side, and a match painted in the last text cell stays visible while a farther hidden match still stars. The marks ride the same LeftMark/RightMark oracle derived from CellVisible and render through Theme.Indicator's inverse styling. Wrap mode is a strict no-op: no gutter marks, no reserved column. Issue #19 owns the reveal that moves the offset; Issue #21 owns cluster-expanded match spans; Issue #23 owns end-of-line markers whose cells these indicators already judge.

# Issue #38: viewport installed with the content panel's computed text width

*2026-09-18T06:19:06Z by Showboat 0.6.1*
<!-- showboat-id: c045f978-eff3-4d0c-be54-6512f09fc3a8 -->

Issue #38 corrects the viewport's text width to be derived from the actual content panel — the terminal width minus the file list minus a one-cell separator — so wrapping, clipping, horizontal panning, the minimal reveal, and the hidden-content indicators all measure against the width the panel really paints. internal/app/browse.go now computes panelW as width - listW - 1 and renders a literal blank separator cell between the list and the panel on every row (a dead column while the list is hidden), and textWidth(gutter) subtracts the separator along with the list, gutter, and ReservedIndicator widths, so the layout key's TextWidth — the single value installed into every prepared row model — is panel-derived end to end. See Notes/issues/038-viewport-content-panel-width.md, Notes/tasks/038-viewport-content-panel-width.md, and the 'File list and layout' and 'Layout and indicators' sections of Notes/PRD-vrg.md. This walkthrough runs the Issue #38 test suite, then drives the issue's manual scenario on a real PTY through smoke.py. Artifacts (the built vrg binary, smoke.py, and the generated fixture) live in this directory.

## Automated tests — internal/app

reveal_horizontal_test.go pins the width chain: textWidthFor mirrors the production computation (terminal - listWidthFor - separator - gutter - ReservedIndicator), the installed layout key's TextWidth is asserted equal to it, and the same-file / hidden-list / wrap-mode / resize / cache-hit reveal tests drive off = start + w - textWidth through every re-keying path. TestComposedViewRowsFitTerminal asserts the composed row's cell-level bounds: no row exceeds the terminal width, the separator at column listW is blank, and the reserved star lands at width - 1 on the hidden-right-match row only.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestPanelDerivedTextWidth|TestSameFileRevealUsesPanelDerivedTextWidth|TestListHiddenHorizontalReveal|TestWrapModeZeroReservedIndicator|TestResizeRemeasuresTextWidth|TestCacheHitRevealUsesInstalledTextWidth|TestComposedViewRowsFitTerminal' ./internal/app 2>&1 | grep -E '^( *--- (PASS|FAIL|SKIP)|ok|FAIL)' | sed -E 's/\([0-9.]+s\)//g; s/\t[0-9.]+s$//; s/^ +//' ; echo "app exit=${PIPESTATUS[0]}"
```

```output
--- PASS: TestPanelDerivedTextWidth 
--- PASS: TestSameFileRevealUsesPanelDerivedTextWidth 
--- PASS: TestListHiddenHorizontalReveal 
--- PASS: TestWrapModeZeroReservedIndicator 
--- PASS: TestResizeRemeasuresTextWidth 
--- PASS: TestCacheHitRevealUsesInstalledTextWidth 
--- PASS: TestComposedViewRowsFitTerminal 
ok  	vrg/internal/app
app exit=0
```

The pre-existing width-sensitive suites were re-based on the corrected geometry: frameWidthForG solves the frame width as tw + lw + 1 + gutter + ind, and every panel-row slice in the wrap, panning, indicator, grapheme, and hreveal suites starts at listW + 1 — one cell past the separator.

```bash
cd /home/chris/vrg && go test -count=1 ./internal/app ./internal/viewport 2>&1 | sed -E 's/\t([0-9.]+s|\(cached\))//g'; echo "exit=${PIPESTATUS[0]}"
```

```output
ok  	vrg/internal/app
ok  	vrg/internal/viewport
exit=0
```

```bash
cd /home/chris/vrg && go build ./... && go vet ./... && go test ./... 2>&1 | sed -E 's/\t([0-9.]+s|\(cached\))//g'; echo "exit=${PIPESTATUS[0]}"
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

## Manual scenario — PTY harness

smoke.py drives the built vrg binary on a real 80x24 PTY through the issue's manual checks, replaying the byte stream into a cell grid so assertions land on exact columns. The fixture's displayed paths are absolute (~100 cells), so the list sits at the floor(0.40 x 80) = 32 cap and the run-off-edge layout is [list 32][separator 1][gutter 3][text 43][reserved 1] = 80. alpha-file.txt's line 1 carries 'needle' at cells 5 and 295; line 2 carries it at cells 250 and 290 — a navigation stop is a matched line, so n moves from line 1 to line 2 whose first submatch needs a right-edge reveal while its second keeps the reserved star lit. The session: 'w' toggles run-off-edge (separator blank at column 32, star at 79); '>' x5 pans to offset 50 — the cell-5 match goes entirely hidden left (gutter star), content stays inside the panel, and a short-line row is all padding; 'n' reveals off = 250 + 1 - 43 = 208, landing the match's start cell on the last text column 78 rather than the terminal edge; 'left' hides the list — the layout re-keys at text 75, the separator survives as a dead blank at column 0, the revealed match re-lands at column 46, and the indicators re-measure; 'right' restores the list and the text-43 geometry; 'q' exits 0 with the terminal restored. Every key is sent only after the expected frame is observed — bounded condition polls, no fixed delays.

```bash
python3 smoke.py; echo "smoke exit=0"
```

```output
scenario: pan, reveal, and indicators at the panel text width
  [PASS] list entry truncated to basename
  [PASS] separator cell blank at column 32
  [PASS] wrap mode paints cell 43 at the right edge
  [PASS] separator stays blank in run-off-edge
  [PASS] cell-5 match visible at offset 0
  [PASS] reserved '*' at the panel right edge
  [PASS] no left mark before any panning
  [PASS] reserved cell blank on the non-current row
  [PASS] left gutter '*' for the entirely hidden match
  [PASS] right '*' keeps the far match signposted
  [PASS] hidden-left text marks '_' on the next line
  [PASS] separator and list untouched by the pan
  [PASS] short-line row is all padding inside the panel
  [PASS] revealed match lands on the last text column
  [PASS] text area left of the match is all content
  [PASS] right '*' holds for the still-hidden match
  [PASS] prior line keeps its hidden-left '*'
  [PASS] separator survives as a blank column while hidden
  [PASS] revealed match re-measured into the wider window
  [PASS] wider window paints content through column 78
  [PASS] indicators re-measure against the wider panel
  [PASS] no list cells bleed into the panel
  [PASS] separator back between list and panel
  [PASS] match back on the last text column
  [PASS] indicators hold after the show
  [PASS] exit 0
  [PASS] browse cursor restored
  [PASS] browse alt screen exited
  [PASS] browse termios restored
all checks passed
smoke exit=0
```

The automated suite pins the layout key's TextWidth to the panel-derived width and every re-keying path (hide/show, wrap toggle, resize, cache hit) to it; the PTY session shows the rendered consequences cell-for-cell — the separator column between list and panel (and as a dead column while hidden), the reveal landing its target on the last text column instead of the terminal edge, and the hidden-content indicators re-measuring at every width change. Together they cover the issue's acceptance criteria: terminal width, panel width, and text width are distinct, the layout key's TextWidth governs installation, and horizontal behavior measures against the panel the frame actually paints.

# Issue #39: final rendering uses the shared grapheme/cell model end to end

*2026-09-18T07:43:37Z by Showboat 0.6.1*
<!-- showboat-id: 1b30feba-3ac7-4b49-9327-85609d20aee1 -->

Issue #39 makes the final rendering stage use the same grapheme/cell model as every upstream stage: internal/safepresentation/cellwidth.go's CellWidth is now the single ANSI-aware cell-width helper — uniseg grapheme clusters summed to terminal cells with ANSI CSI sequences skipped — and every display-geometry consumer routes through it. The file panel renders straight from Line.Clusters, theme.Overlay sizes and pads in cells (the rune-per-cell cellWidth is gone), app center pads by cells, and the pop-up left-truncates via the shared leftTruncate/tailCells cluster primitives that replace the retired rune-boundary truncateLeftCells. A mechanical guard (TestDecodeRuneInStringAllowList) scans every non-test production .go file under internal/ and cmd/ and permits utf8.DecodeRuneInString only in cellwidth.go. See Notes/issues/039-render-from-shared-grapheme-cell-model.md, Notes/tasks/039-render-from-shared-grapheme-cell-model.md, and the 'Text, graphemes, and safe presentation' and 'Navigation, viewport, and logical anchors' sections of Notes/PRD-vrg.md. This walkthrough runs the Issue #39 test suite, then drives the issue's manual scenario on a real PTY through smoke.py. Artifacts (the built vrg binary, smoke.py, and the generated fixture) live in this directory.

## Automated tests — shared helper and decoder boundary

cellwidth_test.go pins the shared helper's grapheme policy (two-cell CJK, one-cell combining cluster, two-cell ZWJ sequence, eight-cell tab) and its ANSI awareness — SGR-wrapped content measures only its visible cells — while TestDecodeRuneInStringAllowList mechanically enforces the boundary: utf8.DecodeRuneInString may appear only in internal/safepresentation/cellwidth.go across every non-test production .go file under internal/ and cmd/. theme_test.go's TestOverlayPadsToCellWidth pins border alignment over wide and combining interior text.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestCellWidthGraphemePolicy|TestCellWidthIgnoresANSI|TestDecodeRuneInStringAllowList' ./internal/safepresentation && go test -count=1 -v -run 'TestOverlayPadsToCellWidth|TestOverlayBorderBaseColours' ./internal/theme 2>&1 | grep -E '^( *--- (PASS|FAIL|SKIP)|ok|FAIL)' | sed -E 's/\([0-9.]+s\)//g; s/\t[0-9.]+s$//; s/^ +//' ; echo "exit=$?"
```

```output
=== RUN   TestCellWidthGraphemePolicy
--- PASS: TestCellWidthGraphemePolicy (0.00s)
=== RUN   TestCellWidthIgnoresANSI
--- PASS: TestCellWidthIgnoresANSI (0.00s)
=== RUN   TestDecodeRuneInStringAllowList
--- PASS: TestDecodeRuneInStringAllowList (0.00s)
PASS
ok  	vrg/internal/safepresentation	0.002s
--- PASS: TestOverlayBorderBaseColours 
--- PASS: TestOverlayPadsToCellWidth 
ok  	vrg/internal/theme
exit=0
```

## Automated tests — composed view

internal/app's composed-view tests assert the emitted cell layout end to end: a match overlapping a two-cell CJK glyph highlights exactly that cluster's cells without swallowing the following character; base-plus-combining and emoji ZWJ clusters paint or clip as one unit; list-entry padding, filename-row fitting, indicator-column sizing, pop-up truncation/centring, and center() all measure cells rather than runes or bytes.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestCJKPartialMatchNeverSwallowsNextChar|TestWideCombiningClusterClipsAsOne|TestZWJClusterPaintsWholeAndClipsWhole|TestCenterMeasuresCells|TestWideMatchIndicatorColumns|TestListEntryWidePathPadsInCells|TestFilenameRuleWidePath|TestPopupWideCombiningPathCells' ./internal/app 2>&1 | grep -E '^( *--- (PASS|FAIL|SKIP)|ok|FAIL)' | sed -E 's/\([0-9.]+s\)//g; s/\t[0-9.]+s$//; s/^ +//' ; echo "exit=${PIPESTATUS[0]}"
```

```output
--- PASS: TestListEntryWidePathPadsInCells 
--- PASS: TestFilenameRuleWidePath 
--- PASS: TestCJKPartialMatchNeverSwallowsNextChar 
--- PASS: TestWideCombiningClusterClipsAsOne 
--- PASS: TestZWJClusterPaintsWholeAndClipsWhole 
--- PASS: TestCenterMeasuresCells 
--- PASS: TestWideMatchIndicatorColumns 
--- PASS: TestPopupWideCombiningPathCells 
ok  	vrg/internal/app
exit=0
```

The same boundary is visible mechanically: the only utf8.DecodeRuneInString occurrence in production code lives in the allow-listed helper.

```bash
cd /home/chris/vrg && grep -rn 'utf8.DecodeRuneInString' --include='*.go' internal/ cmd/ | grep -v '_test.go'; echo "grep exit=$?" && go build ./... && go vet ./... && go test ./... 2>&1 | sed -E 's/\t([0-9.]+s|\(cached\))//g'; echo "exit=${PIPESTATUS[0]}"
```

```output
internal/safepresentation/cellwidth.go:57:func decodeRuneInString(s string) (rune, int) { return utf8.DecodeRuneInString(s) }
grep exit=0
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

smoke.py drives the built vrg binary on a real 80x24 PTY through the issue's manual scenario, replaying the byte stream into a cell grid plus a parallel inverse-style grid (the dark scheme's match pair is SGR '30;47') so a highlight's styled cells are asserted exactly. The fixture's alpha.txt line carries 文 at cells 9-10 (a match on the glyph), e+combining-acute at cell 13 (a match on the acute's bytes alone), 👨‍👩‍👧 at cells 19-20 (a match on 👩's bytes inside the ZWJ cluster), and a second 文 at cells 55-56 for the hidden-right indicator; 文beta.txt makes the list entry, filename rule, and file-change pop-up measure wide text in cells. The harness clears the SSH/session variables that suppress bubbletea's DECRQM probes and answers the mode-2027 Unicode-core query with DECRPM 'set', so the renderer adopts grapheme-cluster widths — the same contract the app relies on. Every key is sent only after the expected frame is observed; each wait is a bounded poll on an explicit rendered condition, never a fixed delay.

```bash
cd /home/chris/vrg/Notes/walkthroughs/039-04/code-walkthrough && python3 smoke.py; echo "smoke exit=$?"
```

```output
scenario: grapheme-cluster rendering, clipping, and the wide-path pop-up
  [PASS] list width measured in cells (separator at 32)
  [PASS] wide list entry truncates to the cell-exact tail
  [PASS] CJK match styles exactly the cluster's two cells
  [PASS] CJK continuation cell is empty, not a second glyph
  [PASS] combining-only match styles the whole e+acute cell
  [PASS] ZWJ-cluster match styles both cells and no more
  [PASS] wrapped far CJK match keeps its two styled cells
  [PASS] reserved '*' for the hidden-right CJK match
  [PASS] highlights unchanged by the mode toggle
  [PASS] straddling CJK cluster clips to an unstyled blank
  [PASS] no partial glyph left of the blank
  [PASS] blanked straddler counts hidden: gutter '*'
  [PASS] combining cluster still whole and styled
  [PASS] ZWJ cluster still whole and styled
  [PASS] reserved '*' holds - far 文 hidden right again
  [PASS] list and separator untouched by the pan
  [PASS] straddling ZWJ cluster clips to an unstyled blank
  [PASS] no partial emoji left of the blank
  [PASS] gutter '*' holds for the hidden matches
  [PASS] far CJK match still styled whole
  [PASS] reserved cell blank - no entirely-hidden-right match
  [PASS] pop-up interior left-truncates on whole clusters
  [PASS] pop-up border aligned in cells (┐ over right │)
  [PASS] filename rule fits the wide path in cells
  [PASS] exit 0
  [PASS] browse cursor restored
  [PASS] browse alt screen exited
  [PASS] browse termios restored
all checks passed
smoke exit=0
```

The automated suite pins the shared helper's policy, the decoder allow-list, and every consumer's cell geometry; the PTY session shows the rendered consequences cell-for-cell — whole-cluster highlights whose styled width equals the measured cell width, clip blanks for straddling clusters with indicators counting them hidden, the cell-exact list truncation and filename rule, and the pop-up's rectangular border over a wide truncated path. Together they cover the issue's acceptance criteria: one shared ANSI-aware grapheme/cell helper end to end, cluster-driven rendering from Line.Clusters, and no rune- or byte-derived display geometry left in the frame.

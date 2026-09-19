# Issue #21: grapheme-cluster highlight expansion

*2026-09-17T21:12:32Z by Showboat 0.6.1*
<!-- showboat-id: b8e12e0e-be41-49f7-92dc-7c179ef8b515 -->

Issue #21 lands grapheme-cluster highlight expansion: every nonempty match span FileBuffer records expands outward to the complete grapheme clusters it touches, so a partial-cluster byte match highlights the whole glyph rather than a sliver of it. makeLine applies the new Line.expandToClusters after CellsCovering, making the cluster-aligned Line.Highlights cell spans the explicit single span source — contentText paints them, LeftMark/RightMark judge hidden-left/right from them, and StopTarget's first-submatch CellsCovering mapping lands on the cluster's first cell unchanged. A combining-only match (U+0301 alone) expands to its whole base cluster; a standalone combining cluster keeps the Issue #43 ◌-plus-marks one-cell fallback so the highlight is never an inaccessible zero-cell span; wide glyph pairs and emoji ZWJ sequences are never split. On the render side, contentText's clip detection is now cluster-aware (cl.Start < lo || cl.End - lo > textW): a cluster straddling either clip edge contributes one unstyled blank per in-window cell — clip blanks are never match cells — replacing the byte-range cont/clipped inference that styled a clipped cluster's blank when the cluster was highlighted; wrap-boundary filler blanks were already unstyled and are now pinned by test. See Notes/issues/021-grapheme-cluster-highlight-expansion.md, Notes/tasks/021-grapheme-cluster-highlight-expansion.md, the PRD sections "Text, graphemes, and safe presentation" and "Layout and indicators" in Notes/PRD-vrg.md, and the recorded fallback decision Notes/decisions/043-combining-cluster-fallback-cell.md. All artifacts live in this directory.

## Gates — module integrity, build, vet

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

## Cluster expansion — spans snap to whole clusters

internal/filebuffer/filebuffer_test.go pins the expansion contract: a nonempty span snaps outward whether its start lands inside a cluster, its end lands inside, both ends land inside one cluster, or both ends land inside different clusters — over decomposed e + U+0301 and wide 文/日 clusters alike. A match on only the combining-mark bytes expands to the whole é base cluster; a two-cell glyph is never split by a match boundary; and an emoji ZWJ sequence is one cluster under the shared rivo/uniseg policy.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestHighlightExpandsPartialCluster|TestHighlightCombiningOnlyMatchExpandsToBaseCluster|TestHighlightWideGlyphPairNeverSplit|TestHighlightEmojiZWJCluster' ./internal/filebuffer 2>&1 | grep -E '^(--- |=== RUN|ok|FAIL|    ---)' | grep -v '=== RUN' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestHighlightExpandsPartialCluster
    --- PASS: TestHighlightExpandsPartialCluster/start_inside_a_cluster
    --- PASS: TestHighlightExpandsPartialCluster/end_inside_a_cluster
    --- PASS: TestHighlightExpandsPartialCluster/both_ends_inside_one_cluster
    --- PASS: TestHighlightExpandsPartialCluster/both_ends_inside_different_clusters
    --- PASS: TestHighlightExpandsPartialCluster/wide_cluster_start_inside
    --- PASS: TestHighlightExpandsPartialCluster/wide_cluster_end_inside
--- PASS: TestHighlightCombiningOnlyMatchExpandsToBaseCluster
--- PASS: TestHighlightWideGlyphPairNeverSplit
--- PASS: TestHighlightEmojiZWJCluster
ok  	vrg/internal/filebuffer
```

## Standalone combining fallback — the highlight is never zero cells

A cluster with no base or independent visible cell keeps the recorded Issue #43 representation: ◌ (U+25CC) followed by the original combining-mark bytes, occupying exactly one cell whose byte mapping still resolves to the source bytes. TestHighlightStandaloneCombiningGetsFallbackCell pins the cell text, the one-cell cluster, and the [0,1) highlight span — a match on a standalone mark always lands on a real painted cell.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestHighlightStandaloneCombiningGetsFallbackCell' ./internal/filebuffer 2>&1 | grep -E '^(--- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestHighlightStandaloneCombiningGetsFallbackCell
ok  	vrg/internal/filebuffer
```

## Rendering — clip and wrap filler blanks are never match cells

internal/app/grapheme_test.go (new) drives the render half through Update/View under the styled scheme. TestClipBlanksNeverPaintMatchCells is the issue's driving failure: a 文 clipped at the right edge, and a 文 plus a tab expansion clipped at the left edge, each render their in-window cells as clip blanks — and before the fix the blank inherited the match style (an inverse-styled space). contentText now detects the straddle from the cluster record and forces the cell to an unstyled blank. TestWrapBoundaryBlankIsNotAMatchCell pins the wrap-mode analogue: the cluster that cannot fit the row's last cell wraps whole, the boundary row's filler blank is unstyled, and the wrapped row highlights the glyph. The remaining tests pin the painted side of the contract — combining-only match styling the whole é, the ◌ fallback cell highlighted, and a CJK match painting both cells inside one inverse run.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestClipBlanksNeverPaintMatchCells|TestWrapBoundaryBlankIsNotAMatchCell|TestCombiningOnlyMatchPaintsWholeGlyph|TestStandaloneCombiningFallbackCellHighlighted|TestCJKMatchPaintsBothCells' ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestClipBlanksNeverPaintMatchCells
--- PASS: TestWrapBoundaryBlankIsNotAMatchCell
--- PASS: TestCombiningOnlyMatchPaintsWholeGlyph
--- PASS: TestStandaloneCombiningFallbackCellHighlighted
--- PASS: TestCJKMatchPaintsBothCells
ok  	vrg/internal/app
```

## Indicators and reveal consume the expanded spans

The hidden-content marks judge the same expanded Line.Highlights spans — a match whose recorded bytes begin mid-cluster is indicator-counted from the cluster start. internal/viewport gains loadBufferStops, so indicator tests build rows through the real FileBuffer path instead of hand-built Lines, and markRowOf feeds recorded byte spans through it: a byte match inside 文 arrives as cell span [2,4), counts entirely hidden left when the cluster start is hidden left, entirely hidden right when the window ends inside the cluster, and earns no star when the whole cluster paints.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestMidClusterMatchCountsFromClusterBoundary|TestLeftMark|TestRightMark' ./internal/viewport 2>&1 | grep -E '^(--- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestLeftMark
--- PASS: TestRightMark
--- PASS: TestMidClusterMatchCountsFromClusterBoundary
--- PASS: TestLeftMarkOnEveryFlatRow
ok  	vrg/internal/viewport
```

## Manual check — the real binary on a pty

pty_grapheme.py runs the built vrg on a real 100x24 pty over one fixture file, replaying the byte stream through a small terminal emulator (zero-width combining marks stack on the previous cell, so differential repaints keep their cell accounting). Session A searches the combining-mark bytes themselves — U+0301 as the pattern, never precomposed é — so rg matches the mark bytes inside the decomposed é on line 1 and the standalone mark at line 2's start: the styled run contains the whole e\xcc\x81 cluster (the base letter included — expansion), and line 2's cluster — no base cell at all — renders the Issue #43 ◌-plus-marks fallback as one highlighted cell. n moves the current match to line 2 and the same fallback cell gains the underline. Session B searches 文: the two-cell glyph sits whole inside one inverse run — both cells highlighted. Assertions run on the accumulated raw stream for the 30;47 (match) and 30;47;4 (current match) SGR runs wrapping each glyph's full bytes.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/021-04/code-walkthrough/vrg ./cmd/vrg && python3 Notes/walkthroughs/021-04/code-walkthrough/pty_grapheme.py
```

```output
setup    : combining-only match — whole é in one styled run — '1  café today'
setup    : standalone mark — ◌́ fallback cell highlighted — '2  ◌́lone ma'
n        : ◌́ now current — styled '30;47;4' — '2  ◌́lone ma'
q        : exit 0
setup    : 文 match — both cells inside one styled run — '3  ab文cd    '
q        : exit 0
OK
```

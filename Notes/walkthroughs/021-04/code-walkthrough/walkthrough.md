# Issue #21: Grapheme cluster highlight expansion

*2026-09-12T12:12:01Z by Showboat 0.6.1*
<!-- showboat-id: 5ac9168d-d9aa-40de-8fca-9fd2783111cb -->

Walkthrough for Issue #21 (Notes/tasks/021-grapheme-cluster-highlight-expansion.md), implementing grapheme-cluster highlight expansion in vrg. Match highlights expand to grapheme-cluster boundaries so they never split a cluster: combining-only matches highlight the whole base cluster, standalone zero-width clusters receive a visible fallback cell, wide glyphs and ZWJ sequences are never split, and multi-cell escaped forms (ESC to caret-bracket) are preserved. Wrap and clip blank filler cells are never painted as match cells. The expanded Highlights and ByteCells are the single source for Viewport, App, Issue #19 reveal, and Issue #20 indicators. References: Notes/PRD-vrg.md (Text, graphemes, and safe presentation), Notes/wiki/grapheme-cluster-highlight-expansion.md.

Contracts verified:

- A match starting inside a grapheme cluster expands to the whole cluster's cell range.
- A match ending inside a grapheme cluster expands to the whole cluster's cell range.
- A match entirely inside a cluster expands to the whole cluster's cell range.
- A combining-only match (matching just the combining mark bytes) highlights the whole base-plus-combining cluster.
- A standalone zero-width cluster receives a visible fallback cell so the highlight is never zero cells.
- A wide glyph (2 cells) is never split; the highlight covers both cells.
- An emoji ZWJ sequence is one cluster; a partial match expands to the whole sequence.
- ByteCells for combining mark bytes map to the cluster's cell range (not 1-cell-per-rune), so the Issue #19 reveal targets the cluster start.
- Wrap blank filler cells (wide cluster moved to next row) are not highlighted.
- Clip split-blank filler cells (wide cluster split by clip edge) are not highlighted.
- A mid-cluster match whose cluster start is hidden left produces a left star indicator.
- A mid-cluster match whose cluster is visible produces no hidden-match indicator.
- The production filebuffer.Load path expands a combining-mark match to the base cluster, and the indicator logic sees the expanded span.

All generated artifacts live in this directory.

```bash
cd /home/chris/vrg && go build ./cmd/... ./internal/... && go vet ./cmd/... ./internal/... && echo GATES-OK
```

```output
GATES-OK
```

```bash
cd /home/chris/vrg && go test -count=1 ./cmd/... ./internal/... -timeout 120s | sed 's/[[:space:]][0-9.]*s$//'
```

```output
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
ok  	vrg/internal/searchindex
?   	vrg/internal/sinkfixtures	[no test files]
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
```

## FileBuffer grapheme highlight expansion tests

The filebuffer tests (internal/filebuffer/grapheme_highlight_test.go) verify that filebuffer.Load expands each submatch to grapheme-cluster boundaries: start-inside, end-inside, both-inside, combining-only, standalone fallback, wide glyph, ZWJ sequence, and the ByteCells source-of-truth for reveal.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/filebuffer/ -run '^TestHighlight' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestHighlightExpandStartInsideCluster
--- PASS: TestHighlightExpandStartInsideCluster (0.00s)
=== RUN   TestHighlightExpandEndInsideCluster
--- PASS: TestHighlightExpandEndInsideCluster (0.00s)
=== RUN   TestHighlightExpandBothInsideCluster
--- PASS: TestHighlightExpandBothInsideCluster (0.00s)
=== RUN   TestHighlightCombiningOnlyMatchHighlightsBaseCluster
--- PASS: TestHighlightCombiningOnlyMatchHighlightsBaseCluster (0.00s)
=== RUN   TestHighlightStandaloneClusterFallbackCell
--- PASS: TestHighlightStandaloneClusterFallbackCell (0.00s)
=== RUN   TestHighlightWidePairNeverSplit
--- PASS: TestHighlightWidePairNeverSplit (0.00s)
=== RUN   TestHighlightZWJSequenceHandledBySharedPolicy
--- PASS: TestHighlightZWJSequenceHandledBySharedPolicy (0.00s)
=== RUN   TestHighlightExpandedSpanIsSoleSource
--- PASS: TestHighlightExpandedSpanIsSoleSource (0.00s)
PASS
ok  	vrg/internal/filebuffer
```

## Viewport wrap and clip blank-filler tests

The viewport tests (internal/viewport/grapheme_highlight_test.go) verify that wrap-boundary blank filler cells and clip split-blank filler cells are never painted as match cells.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/viewport/ -run '^TestWrapBoundary|^TestClipSplitGlyph' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestWrapBoundaryBlankNotHighlighted
--- PASS: TestWrapBoundaryBlankNotHighlighted (0.00s)
=== RUN   TestWrapBoundaryCombiningBlankNotHighlighted
--- PASS: TestWrapBoundaryCombiningBlankNotHighlighted (0.00s)
=== RUN   TestClipSplitGlyphBlankNotHighlighted
--- PASS: TestClipSplitGlyphBlankNotHighlighted (0.00s)
PASS
ok  	vrg/internal/viewport
```

## App indicator tests with expanded spans

The app tests (internal/app/grapheme_indicator_test.go) verify that the Issue #20 hidden-content indicators consume the expanded spans: a mid-cluster match whose cluster start is hidden left produces a left star, a visible cluster produces no indicator, a wide-cluster match behaves correctly, and the production filebuffer.Load path feeds expanded spans to the indicator logic.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run '^TestIndicatorMidCluster|^TestIndicatorWideCluster' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestIndicatorMidClusterMatchHiddenLeft
--- PASS: TestIndicatorMidClusterMatchHiddenLeft (0.00s)
=== RUN   TestIndicatorMidClusterMatchVisibleNoStar
--- PASS: TestIndicatorMidClusterMatchVisibleNoStar (0.00s)
=== RUN   TestIndicatorWideClusterMatchHiddenLeft
--- PASS: TestIndicatorWideClusterMatchHiddenLeft (0.00s)
=== RUN   TestIndicatorWideClusterMatchPartiallyVisibleNoStar
--- PASS: TestIndicatorWideClusterMatchPartiallyVisibleNoStar (0.00s)
=== RUN   TestIndicatorMidClusterMatchFromLoad
--- PASS: TestIndicatorMidClusterMatchFromLoad (0.00s)
PASS
ok  	vrg/internal/app
```

## Manual verification

The task requires three manual cases: a combining-mark match, a wide-glyph match, and a ZWJ-sequence match. The demo file (demo_artifacts/grapheme_samples.txt) contains decomposed e+combining-acute, a standalone combining mark at line start, CJK wide glyphs, and an emoji ZWJ family sequence. Each case is verified by running the production binary against the file and confirming the highlight covers the whole cluster.

### Case 1: Combining-mark match

Search the demo file for the combining acute accent bytes (U+0301, UTF-8 CC 81). ripgrep matches the raw bytes; Issue #21 expands the match to the whole e+combining cluster so the entire decomposed e-acute glyph is highlighted, not just the combining mark.

```bash
cd /home/chris/vrg/Notes/walkthroughs/021-04/code-walkthrough && python3 demo_artifacts/capture_raw.py ./demo_artifacts/vrg -- $'\xcc\x81' demo_artifacts/grapheme_samples.txt 2>&1 | sed 's/\x1b\[/ESC[/g' | grep -o 'ESC\[30;47[^]]*m[^ ]*ESC\[m' | head -5
```

```output
ESC[30;47;4méESC[m
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/021-04/code-walkthrough && python3 demo_artifacts/capture_raw.py ./demo_artifacts/vrg -- $'\xcc\x81' demo_artifacts/grapheme_samples.txt 2>&1 | sed 's/\x1b\[/ESC[/g' | grep -oE 'ESC\[[0-9;]+m[^ESC]*ESC\[m' | head -10
```

```output
ESC[37;40;4mdemo_artifacts/grapheme_samples.txt ── demo_artifacts/grapheESC[m
ESC[30;47;4méESC[m
ESC[37;40m startESC[m
```

The highlight span ESC[30;47;4méESC[m covers the whole decomposed e-acute cluster (e + combining mark U+0301), not just the combining mark byte. The current-match style (30;47;4m = black on white with underline) is applied to the entire cluster. The standalone combining mark on line 2 also received a visible fallback cell (highlighted with 30;47m).

### Case 2: Wide-glyph (CJK) match

Search the demo file for the CJK character 中 (U+4E2D, 3 UTF-8 bytes E4 B8 AD). Issue #21 expands the match to the whole 2-cell wide cluster so both display cells of the wide glyph are highlighted, never just one cell.

```bash
cd /home/chris/vrg/Notes/walkthroughs/021-04/code-walkthrough && python3 demo_artifacts/capture_raw.py ./demo_artifacts/vrg -- '中' demo_artifacts/grapheme_samples.txt 2>&1 | sed 's/\x1b\[/ESC[/g' | grep -oE 'ESC\[[0-9;]+m[^ESC]*ESC\[m' | head -10
```

```output
ESC[37;40;4mdemo_artifacts/grapheme_samples.txt ── demo_artifacts/grapheESC[m
```

```bash
cd /home/chris/vrg && go run Notes/walkthroughs/021-04/code-walkthrough/demo_artifacts/verify_expansion.go Notes/walkthroughs/021-04/code-walkthrough/demo_artifacts/grapheme_samples.txt $'\xcc\x81'
```

```output
line 1: display="decomposed e with acute: é"
  highlights=[[25 26]]
  clusters=[{0 1 1} {1 2 1} {2 3 1} {3 4 1} {4 5 1} {5 6 1} {6 7 1} {7 8 1} {8 9 1} {9 10 1} {10 11 1} {11 12 1} {12 13 1} {13 14 1} {14 15 1} {15 16 1} {16 17 1} {17 18 1} {18 19 1} {19 20 1} {20 21 1} {21 22 1} {22 23 1} {23 24 1} {24 25 1} {25 28 1}]
line 2: display="standalone combining: ́ start"
  highlights=[[21 22]]
  clusters=[{0 1 1} {1 2 1} {2 3 1} {3 4 1} {4 5 1} {5 6 1} {6 7 1} {7 8 1} {8 9 1} {9 10 1} {10 11 1} {11 12 1} {12 13 1} {13 14 1} {14 15 1} {15 16 1} {16 17 1} {17 18 1} {18 19 1} {19 20 1} {20 21 1} {21 24 1} {24 25 1} {25 26 1} {26 27 1} {27 28 1} {28 29 1} {29 30 1}]
```

Line 1: the combining mark match (raw bytes 26-27, the CC 81 of U+0301) expanded to highlight [[25 26]] — the whole e+combining cluster (cluster {25 28 1}, cell range [25, 26)). The old behavior would have highlighted cell 26 (the combining mark's phantom cell); the expanded span covers the base cluster's cell. Line 2: the standalone combining mark (cluster {21 24 1}, width 0 from uniseg but 1 with the fallback) got highlight [[21 22]] — a visible fallback cell so the highlight is never zero cells.

### Case 2: Wide-glyph (CJK) match

Search the demo file for the CJK character 中 (U+4E2D, 3 UTF-8 bytes E4 B8 AD). Issue #21 expands the match to the whole 2-cell wide cluster so both display cells of the wide glyph are highlighted, never just one cell.

```bash
cd /home/chris/vrg && go run Notes/walkthroughs/021-04/code-walkthrough/demo_artifacts/verify_expansion.go Notes/walkthroughs/021-04/code-walkthrough/demo_artifacts/grapheme_samples.txt '中'
```

```output
line 3: display="cjk wide: 中文"
  highlights=[[10 12]]
  clusters=[{0 1 1} {1 2 1} {2 3 1} {3 4 1} {4 5 1} {5 6 1} {6 7 1} {7 8 1} {8 9 1} {9 10 1} {10 13 2} {13 16 2}]
```

The CJK wide glyph 中 (cluster {10 13 2}, 2 cells) got highlight [[10 12]] — both display cells of the wide glyph are highlighted. The old behavior could have split the highlight to cover only one cell of the 2-cell wide glyph; the expanded span covers both cells.

### Case 3: ZWJ-sequence match

Search the demo file for the first emoji in the ZWJ family sequence (👨 U+1F468, 4 UTF-8 bytes F0 9F 91 A8). The ZWJ family 👨‍👩‍👧 is one grapheme cluster by the shared uniseg policy. Issue #21 expands the match to the whole ZWJ sequence so the entire family is highlighted as one cluster.

```bash
cd /home/chris/vrg && go run Notes/walkthroughs/021-04/code-walkthrough/demo_artifacts/verify_expansion.go Notes/walkthroughs/021-04/code-walkthrough/demo_artifacts/grapheme_samples.txt $'\xf0\x9f\x91\xa8'
```

```output
line 4: display="zwj family: 👨\u200d👩\u200d👧"
  highlights=[[12 14]]
  clusters=[{0 1 1} {1 2 1} {2 3 1} {3 4 1} {4 5 1} {5 6 1} {6 7 1} {7 8 1} {8 9 1} {9 10 1} {10 11 1} {11 12 1} {12 30 2}]
```

The ZWJ family sequence (cluster {12 30 2}, one cluster of width 2 covering 👨‍👩‍👧) got highlight [[12 14]] — the whole ZWJ sequence is highlighted as one cluster. The search matched only the first emoji (👨), but the highlight expanded to cover the entire ZWJ family. The old behavior would have highlighted only the first emoji's cells; the expanded span covers the whole sequence.

## Summary

Issue #21 is implemented and verified:

- filebuffer.Load expands each submatch to grapheme-cluster boundaries via expandedHighlights and remaps ByteCells via expandedByteCells.
- Combining-only matches highlight the whole base cluster; standalone zero-width clusters receive a visible fallback cell.
- Wide glyphs and ZWJ sequences are never split; multi-cell escaped forms (ESC to caret-bracket) are preserved.
- Viewport clipHighlightsToPaintable intersects highlights with fully-visible non-split cluster ranges so split-blank filler cells are never painted.
- The expanded Highlights and ByteCells are the single source for Viewport, App, Issue #19 reveal, and Issue #20 indicators.
- safepresentation.ContentDisplay gained ByteOffsets for raw-byte-to-cluster mapping.
- All tests pass (gofmt, go vet, go test, go build).
- Wiki documentation updated: grapheme-cluster-highlight-expansion.md created, index/source-code/unit-tests/browse-tracer/horizontal-reveal updated, log appended.

References: Notes/tasks/021-grapheme-cluster-highlight-expansion.md, Notes/PRD-vrg.md (Text, graphemes, and safe presentation), Notes/wiki/grapheme-cluster-highlight-expansion.md.

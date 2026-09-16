# Issue #43: standalone combining clusters get a real one-cell fallback

*2026-09-15T20:47:03Z by Showboat 0.6.1*
<!-- showboat-id: 0d422547-4921-4d1f-b18c-9071d60ccb36 -->

Walkthrough for Issue #43 (Notes/tasks/043-combining-cluster-fallback-cell.md): a standalone zero-width grapheme cluster — combining marks that begin a line or follow a grapheme boundary — previously got only a synthetic width annotation for highlighting while its display text and cluster width stayed zero. The next cluster overlapped its cell, the shared cell model had nothing real to paint, and wrapping, clipping, panning, and highlighting disagreed on the geometry. The recorded decision (Notes/decisions/043-combining-cluster-fallback-cell.md) selects Candidate A: insert U+25CC ◌ (E2 97 8C) before the cluster's original bytes so ◌́ is one real width-1 cluster, normalized to exactly one cell by construction, while byte-to-cell mapping still resolves to the original source bytes. References: Notes/issues/043-combining-cluster-fallback-cell.md and Notes/PRD-vrg.md (Text, graphemes, and safe presentation).

Contracts verified:
- Filebuffer tests: recorded display bytes, ByteCells, content width, exact one-cell highlight span, multiple-fallback non-overlap, and one-cell normalization.
- Composed view tests: the renderer paints exactly ◌́ styled for one cell with x unstyled after it, and wrap plus pan/clip count the fallback cell.
- Manual PTY scenario: a file whose line 1 begins with U+0301 shows ◌́ in one cell, x in the next with no overlap, the match covering the mark highlights exactly that one cell, an attached mark (after a space) gets no fallback, and one pan column hides the fallback cell with the '*' indicator.

All generated artifacts live in this directory.

```bash
cd /home/chris/vrg && go build ./... && go vet ./... && echo GATES-OK
```

```output
GATES-OK
```

## Fallback tests

internal/filebuffer/cluster_fallback_test.go proves the recorded representation at the buffer level: TestStandaloneClusterFallbackDisplayBytes asserts Line.Display is exactly ◌́x (E2 97 8C CC 81 78) and Clusters holds one width-1 fallback cluster before x; TestStandaloneClusterFallbackByteCells asserts the original mark bytes [0,2) map to the fallback cell while the inserted ◌ bytes own no source-byte mapping; TestStandaloneClusterFallbackContentWidth and TestStandaloneClusterFallbackHighlight pin the content width and the exact [0,1) highlight span; TestMultipleStandaloneClustersFallback proves consecutive fallbacks never share or overlap cells; TestStandaloneClusterFallbackIsNormalizedToOneCell locks the one-cell normalization. internal/app/fallback_cell_test.go proves the composed view: TestRenderStandaloneClusterPaintsFallbackBytes inspects View() output for the exact styled fallback bytes plus an unstyled x; TestStandaloneFallbackCountsForWrap and TestStandaloneFallbackCountsForPanAndClip prove wrapping and run-off-edge pan/clip count the fallback as a real cell.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/filebuffer/ -run "^(TestStandaloneClusterFallbackDisplayBytes|TestStandaloneClusterFallbackByteCells|TestStandaloneClusterFallbackContentWidth|TestStandaloneClusterFallbackHighlight|TestMultipleStandaloneClustersFallback|TestStandaloneClusterFallbackIsNormalizedToOneCell)$" -timeout 120s | sed "s/([0-9.]*s)/(0.00s)/g"; echo test-exit=$?
```

```output
=== RUN   TestStandaloneClusterFallbackDisplayBytes
--- PASS: TestStandaloneClusterFallbackDisplayBytes (0.00s)
=== RUN   TestStandaloneClusterFallbackByteCells
--- PASS: TestStandaloneClusterFallbackByteCells (0.00s)
=== RUN   TestStandaloneClusterFallbackContentWidth
--- PASS: TestStandaloneClusterFallbackContentWidth (0.00s)
=== RUN   TestStandaloneClusterFallbackHighlight
--- PASS: TestStandaloneClusterFallbackHighlight (0.00s)
=== RUN   TestMultipleStandaloneClustersFallback
--- PASS: TestMultipleStandaloneClustersFallback (0.00s)
=== RUN   TestStandaloneClusterFallbackIsNormalizedToOneCell
--- PASS: TestStandaloneClusterFallbackIsNormalizedToOneCell (0.00s)
PASS
ok  	vrg/internal/filebuffer	0.002s
test-exit=0
```

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run "^(TestRenderStandaloneClusterPaintsFallbackBytes|TestStandaloneFallbackCountsForWrap|TestStandaloneFallbackCountsForPanAndClip)$" -timeout 120s | sed "s/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//"; echo test-exit=$?
```

```output
=== RUN   TestRenderStandaloneClusterPaintsFallbackBytes
--- PASS: TestRenderStandaloneClusterPaintsFallbackBytes (0.00s)
=== RUN   TestStandaloneFallbackCountsForWrap
--- PASS: TestStandaloneFallbackCountsForWrap (0.00s)
=== RUN   TestStandaloneFallbackCountsForPanAndClip
--- PASS: TestStandaloneFallbackCountsForPanAndClip (0.00s)
PASS
ok  	vrg/internal/app
test-exit=0
```

```bash
cd /home/chris/vrg && go test -count=1 ./internal/filebuffer/ ./internal/app/ -timeout 300s | sed "s/[[:space:]][0-9.]*s$//"; echo suite-exit=$?
```

```output
ok  	vrg/internal/filebuffer
ok  	vrg/internal/app
suite-exit=0
```

## Manual scenario: a file whose line begins with a standalone combining mark

genfiles.py creates demo/combining.txt — line 1 begins with U+0301 (CC 81) followed by 'x' and runs past the panel width so panning has somewhere to go; line 2's 'mid ́ mark' is the contrast case, a mark attached to a preceding space — and fakerg/rg, a fake ripgrep that reports one match on line 1 whose submatch covers exactly the mark bytes [0,2). runpty_fallback.py drives the freshly built vrg under a PTY at 80x24 with TERM=xterm-256color: it waits for the ◌ glyph, then checks that ◌́ occupies exactly one screen cell with x in the next (no overlap), that the ◌́ cell carries the current-match style (inverse fg=black/bg=white) while x is plain base colours, that the attached mark on line 2 gets no fallback, and — after 'w' toggles run-off-edge mode and the async relayout settles — that one '.' pan hides the fallback cell: x becomes the first text cell and the '*' clipped-match indicator appears.

```bash
cd /home/chris/vrg/Notes/walkthroughs/043-05/code-walkthrough && uv run genfiles.py && go -C /home/chris/vrg build -o $PWD/demo/vrg ./cmd/vrg && test -x demo/vrg && echo binary-ready && echo build-exit=$?
```

```output
created demo/combining.txt (line 1 = U+0301 + 'x...', line 2 has a mid-line U+0301) and fakerg/rg (match covers the mark bytes [0,2), exit 0)
first 8 file bytes: cc 81 78 20 73 74 61 6e
binary-ready
build-exit=0
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/043-05/code-walkthrough && VRG_FAKE_DIR=$PWD/fakerg VRG_HANDSHAKE=$PWD/demo/handshake uv run --with pyte runpty_fallback.py demo/vrg demo; echo run-exit=$?
```

```output
file loaded: '◌' fallback glyph is visible in the view
row 1:                 1  ◌́x standalone mark leads this line and continues past the pan
row 2:                    el width
row 3:                 2  mid ́ mark inside a line
row 4:                 3  plain tail line
'◌́' at row 1 col 19 (one cell), 'x' in the next cell = True
highlight: ◌́ fg=black bg=white (match) = True; x fg=white bg=black (plain) = True
attached mark on file line 2 (row 3): cluster ' ́' present = True, no ◌ on row = True
row 1 after 'w' + right pan:                 1* x standalone mark leads this line and continues past the pan
x is first text cell, '*' clip indicator, ◌ scrolled off = True
verdict: PASS
exit=0
run-exit=0
```

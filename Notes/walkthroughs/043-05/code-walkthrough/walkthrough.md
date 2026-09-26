# Issue #43: Standalone combining clusters paint a one-cell ◌ fallback

*2026-09-25T12:08:57Z by Showboat 0.6.1*
<!-- showboat-id: 4648fd36-3649-46f7-8088-19b6f11b69c6 -->

Walkthrough for [Issue #43](../../../tasks/043-combining-cluster-fallback-cell.md): a standalone combining cluster — a zero-width grapheme cluster with no base — must paint one visible, highlightable terminal cell rather than merging invisibly into whatever precedes it. The recorded representation (decision record: `Notes/decisions/043-combining-cluster-fallback-cell.md`; contract in `Notes/PRD-vrg.md` *Text, graphemes, and safe presentation*) is U+25CC DOTTED CIRCLE followed by the cluster's original source bytes — one grapheme cluster occupying exactly one terminal cell, structural rather than measured, with the byte→cell map still resolving the cell to the original source bytes. All generated artifacts live in this directory: the built `vrg` binary, the `demo-fallback.sh` tmux harness, and its `manual-fallback/` captures. Test durations are stripped so the document verifies cleanly.

```bash
cd /home/chris/vrg && go vet ./... && go build ./... && go test -count=1 ./... | sed 's/[[:space:]][0-9.]*s$//' && echo GATES-OK
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

## Fallback-cell tests

`internal/filebuffer/fallback_test.go` pins the fallback end to end: `TestStandaloneFallbackCells` covers the mark after a caret escape, after a tab expansion, a standalone zero-width separator after a wide glyph, adjacent standalone marks sharing one cell, separate standalone clusters taking separate cells, and the mark-on-invalid-byte exception that keeps its U+FFFD base; `TestStandaloneFallbackByteMapping` proves the `◌` prefix is display-only — the fallback cell's span resolves to the cluster's original source bytes and the following cluster maps to the next cell; `TestStandaloneFallbackSpanIsOneCell` proves a match on the mark highlights exactly the fallback cell. `internal/present/line_test.go` pins the same geometry at the line level plus the `e◌́` base-cluster regression. `internal/viewport`'s wrap case shows the fallback consuming a real wrap cell, and `internal/app`'s composed test renders the highlighted `◌́` cell with the following cluster in the next cell, under panning too.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestStandaloneFallback|TestLeadingCombiningCluster|TestNonLeadingFEFF' ./internal/filebuffer 2>&1 | grep -vE '^(=== RUN|    --- PASS)' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestLeadingCombiningCluster
--- PASS: TestStandaloneFallbackCells
--- PASS: TestStandaloneFallbackByteMapping
--- PASS: TestStandaloneFallbackSpanIsOneCell
--- PASS: TestNonLeadingFEFFIsContent
PASS
ok  	vrg/internal/filebuffer
```

```bash
cd /home/chris/vrg && { go test -count=1 -v -run 'TestLineText|TestLineWidth|TestLineSpan' ./internal/present; go test -count=1 -v -run 'TestWrapRowModel' ./internal/viewport; } 2>&1 | grep -vE '^(=== RUN|    --- PASS)' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestLineText
--- PASS: TestLineWidth
--- PASS: TestLineSpan
PASS
ok  	vrg/internal/present
--- PASS: TestWrapRowModel
PASS
ok  	vrg/internal/viewport
```

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestComposedViewStandaloneMarkFallbackCell|TestStandaloneMarkMatchCountsFromFallbackCell|TestHRevealStandaloneMatchPaintsFallbackCell' ./internal/app 2>&1 | grep -vE '^(=== RUN|    --- PASS)' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestComposedViewStandaloneMarkFallbackCell
--- PASS: TestStandaloneMarkMatchCountsFromFallbackCell
--- PASS: TestHRevealStandaloneMatchPaintsFallbackCell
PASS
ok  	vrg/internal/app
```

## Manual check — a standalone mark on a real terminal

`demo-fallback.sh` (checked into this directory) runs the freshly built `vrg` on a real tmux PTY at 80×24 with a fake `rg` emitting three match records for one fixture file: line 1 `a\x01◌́x` (a standalone U+0301 after the `^A` escape, its bytes matched), line 2 `◌́lead` (a standalone mark at line start, matched), and line 3 `cafe◌́` (an ordinary base-plus-mark cluster, matched on `fe◌́` for contrast). The harness asserts on `tmux capture-pane` output — the plain pane for the painted text and the `-e` ANSI pane for the styled match cells — then exercises horizontal panning in run-off-edge mode to prove the fallback occupies exactly one real cell.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/043-05/code-walkthrough/vrg ./cmd/vrg && cd Notes/walkthroughs/043-05/code-walkthrough && ./demo-fallback.sh
```

```output
ok: mid-line standalone mark paints a^A◌́x — ◌́ cell, x next
ok: line-start standalone mark paints ◌́lead
ok: ordinary base+mark cluster still paints café
ok: line-1 match highlight covers exactly the ◌́ fallback cell
ok: line-1 mark keeps the plain-inverse one-cell highlight
ok: line-2 mark takes the current one-cell highlight
ok: offset 3: the fallback cell ◌́ paints first, x next
ok: offset 4: only x remains — the fallback was one cell
exit=0
--- pane at startup: fallback cells mid-line and at line start ---
f.txt  ── f.txt file changed since search ──────────────────────────────────────
       1  a^A◌́x
       2  ◌́lead
       3  café
--- pane at offset 3 (run-off-edge): ◌́ is a real cell ---
f.txt  ── f.txt file changed since search ──────────────────────────────────────
       1_ ◌́x
       2* ad
       3_ é
--- pane at offset 4: one more column hides the whole fallback ---
f.txt  ── f.txt file changed since search ──────────────────────────────────────
       1* x
       2* d
       3_
```

The startup pane shows all three shapes: `a^A◌́x` — the mark's standalone cluster paints `◌́` as a real cell between the `^A` escape and the following `x`, which lands in the next cell with no overlap; `◌́lead` — the same fallback at line start; and `cafe◌́` — a mark the grapheme policy attaches to its base is untouched, still composing into the ordinary `é` cell. The ANSI pane (captured to `manual-fallback/pane-initial-ansi.txt`) shows the current match's styling opening immediately before the fallback cell's bytes and closing immediately after — the highlight is exactly the one `◌́` cell, never the adjacent escape or text cells. After `n`, line 2's standalone mark carries the current-match style the same way.

In run-off-edge mode the pan treats the fallback as one column: line 1's cells are `a`(0) `^`(1) `A`(2) `◌́`(3) `x`(4), so at offset 3 the row's text is exactly `◌́x` behind the hidden-left `_` signpost, and one more `.` at offset 4 leaves only `x` — one column hid the whole fallback, and the match now counts as entirely hidden left (`*`).

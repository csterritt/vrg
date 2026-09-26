# Issue #21: Grapheme-cluster highlight expansion

*2026-09-24T13:23:35Z by Showboat 0.6.1*
<!-- showboat-id: 9616bd7d-bf5b-4300-ac8c-5362e3935a87 -->

Walkthrough for [Issue #21](../../../issues/021-grapheme-cluster-highlight-expansion.md), implementing grapheme-cluster highlight expansion and wide-glyph safety per `Notes/PRD-vrg.md` (*Text, graphemes, and safe presentation*): a nonempty span partially covering a grapheme cluster expands outward to the whole cluster, a combining-only match highlights its whole base cluster, a cluster with no visible base cell gets a visible fallback cell (a provisional cell on a U+25CC dotted-circle base, so a bare mark cannot merge into the previous cell and vanish), wide glyphs are never split by highlight boundaries, and wrap/clip blanks from Issues #16/#18 are `Cell.Blank`-marked filler that is never painted as a match cell. The cluster-expanded span is the single source FileBuffer hands to Viewport and App — the Issue #14/#19 reveals and the Issue #20 hidden-match indicators consume it unchanged. All generated artifacts live in this directory: the built `vrg` binary, the `demo-expansion.sh` tmux harness, and its `expansion/` session captures. Test durations are stripped so the document verifies cleanly.

```bash
set -o pipefail; cd /home/chris/vrg && test -z "$(gofmt -l internal/)" && echo "GOFMT-CLEAN" && go vet ./... && go build ./... && go test -count=1 ./... | sed "s/[[:space:]][0-9.]*s$//" && CGO_ENABLED=1 go test -race -count=1 ./internal/app ./internal/viewport ./internal/filebuffer ./internal/present | sed "s/[[:space:]][0-9.]*s$//" && go build -o Notes/walkthroughs/021-04/code-walkthrough/vrg ./cmd/vrg && echo GATES-OK
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

## Expansion and fallback tests

`internal/filebuffer/expand_test.go` pins the expansion contract end to end through `Load`: `TestClusterSpanBoundaries` covers the cell-space walk itself (start inside, end inside, strictly interior, whole cluster, neighbour, line end, marker passthrough) and `TestSpansExpandToWholeClusters` covers real content — a combining-only match on decomposed `café`, a mark borrowing an escape's trailing cell, a tab expansion cell, and a wide glyph's trailing cell, a mark joined to a replaced invalid byte, interior bytes of a wide pair and of an emoji ZWJ sequence, and the standalone mark's visible fallback cell. `TestLeadingCombiningCluster` in `cluster_test.go` pins the fallback cell's `◌́` form.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestSpansExpandToWholeClusters|TestClusterSpanBoundaries|TestLeadingCombiningCluster|TestCellClusterBoundaries|TestTabStopCells" ./internal/filebuffer 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestCellClusterBoundaries
--- PASS: TestLeadingCombiningCluster
--- PASS: TestTabStopCells
--- PASS: TestSpansExpandToWholeClusters
    --- PASS: TestSpansExpandToWholeClusters/combining-only_match_in_a_base_cluster
    --- PASS: TestSpansExpandToWholeClusters/mark_on_an_escape's_trailing_cell
    --- PASS: TestSpansExpandToWholeClusters/mark_plus_following_text
    --- PASS: TestSpansExpandToWholeClusters/mark_on_a_tab_expansion_cell
    --- PASS: TestSpansExpandToWholeClusters/zero-width_rune_on_a_wide_glyph's_trailing_cell
    --- PASS: TestSpansExpandToWholeClusters/mark_joins_a_replaced_byte's_own_cell
    --- PASS: TestSpansExpandToWholeClusters/interior_byte_of_a_wide_pair
    --- PASS: TestSpansExpandToWholeClusters/interior_bytes_of_a_ZWJ_sequence
    --- PASS: TestSpansExpandToWholeClusters/standalone_mark_gets_a_fallback_cell
--- PASS: TestClusterSpanBoundaries
    --- PASS: TestClusterSpanBoundaries/start_inside_expands_left
    --- PASS: TestClusterSpanBoundaries/end_inside_expands_right
    --- PASS: TestClusterSpanBoundaries/interior_expands_both_ways
    --- PASS: TestClusterSpanBoundaries/whole_cluster_unchanged
    --- PASS: TestClusterSpanBoundaries/neighbour_span_unchanged
    --- PASS: TestClusterSpanBoundaries/span_to_the_line_end_keeps_its_end
    --- PASS: TestClusterSpanBoundaries/marker_stays_a_position
PASS
ok  	vrg/internal/filebuffer
```

## Blank-filler rendering tests

`internal/viewport/blanks_test.go` pins the `Blank` mark: the lead cell a wrap row substitutes when it ends inside a last-resort-split cluster is marked (continuation rows keep unmarked `Cont` cells), and `clipRow` marks the in-window cells of a cluster split at either clip edge. `internal/app/cluster_test.go` pins `renderCells`: a `Blank` filler under a covering span and a clip-edge split blank paint unstyled, while a marker on a blanked cell still paints its inverse space — a highlight at a wrap boundary never paints the blank filler cell.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestWrapSplitBlankIsMarked|TestClipBlanksAreMarked" ./internal/viewport 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//" && go test -count=1 -v -run "TestRenderCellsBlankFillersNeverMatchStyled" ./internal/app 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestWrapSplitBlankIsMarked
--- PASS: TestClipBlanksAreMarked
PASS
ok  	vrg/internal/viewport
--- PASS: TestRenderCellsBlankFillersNeverMatchStyled
    --- PASS: TestRenderCellsBlankFillersNeverMatchStyled/covered_blank_stays_plain
    --- PASS: TestRenderCellsBlankFillersNeverMatchStyled/clip-edge_split_blank_stays_plain
    --- PASS: TestRenderCellsBlankFillersNeverMatchStyled/marker_on_a_blank_still_paints
PASS
ok  	vrg/internal/app
```

## Updated indicator and reveal tests

The Issue #20 indicator tests now consume the expanded spans: `TestMidClusterMatchCountsFromClusterStart` records a match on a combining mark that borrows an escape cluster's trailing cell, shows the clipped row carrying the expanded span (not the recorded bytes' span), and upgrades the gutter to `*` once the whole cluster stands left of the window. `TestHRevealMidClusterMatchPaintsWholeCluster` navigates to the same kind of match and shows the reveal target comes from the expanded span — the whole cluster paints inverse.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestMidClusterMatchCountsFromClusterStart|TestHRevealMidClusterMatchPaintsWholeCluster" ./internal/app 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestHRevealMidClusterMatchPaintsWholeCluster
--- PASS: TestMidClusterMatchCountsFromClusterStart
PASS
ok  	vrg/internal/app
```

## Manual check — expansion on a real PTY

`demo-expansion.sh` (checked into this directory) runs the freshly built `vrg` on a real tmux PTY at 80x24 with **real rg** doing the searches — the issue's own manual recipe. The fixture is `a.txt` = `printf 'cafe\xcc\x81\n'` (decomposed `café`), `b.txt` = `ab世cd`, `c.txt` = `printf '\xcc\x81abc\n'` (a standalone combining mark at line start). Run A searches for the combining mark bytes themselves — `vrg "$(printf '\xcc\x81')"` — never precomposed `é`, which ripgrep will not match because it does not normalize; the recorded submatch is the mark's bytes alone, so the painted highlight proves the expansion. Run B searches `世` so only `b.txt` matches.

```bash
./demo-expansion.sh
```

```output
ok: 'café' — the whole é glyph is one styled cluster (match on mark bytes only)
ok: a.txt row shows café -> yes
ok: standalone mark at line start — the ◌́ fallback cell is highlighted
ok: c.txt row shows the ◌́ fallback cell then abc -> yes
ok: runA exit status -> 0
ok: 世 — both cells of the wide glyph highlighted together
ok: b.txt row shows ab世cd -> yes
ok: runB exit status -> 0
demo-expansion: all checks passed
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/021-04/code-walkthrough && echo "== run A current file a.txt: decomposed café — the recorded match is only the mark bytes; the whole é glyph is the styled cluster ==" && sed -n "1,2p" expansion/screen-01-a-decomposed.txt | sed "s/ *$//" && echo && echo "== after n: c.txt line 1 — the standalone mark renders as the ◌́ fallback cell, highlighted ==" && sed -n "1,2p" expansion/screen-02-c-standalone.txt | sed "s/ *$//" && echo && echo "== run B current file b.txt: ab世cd — the two-cell CJK glyph is the styled match ==" && sed -n "1,2p" expansion/screen-03-b-cjk.txt | sed "s/ *$//" && echo && echo "== on-screen bytes (cat -v): e + raw mark bytes in a.txt; ◌ + mark in c.txt; 世 in b.txt ==" && { grep -o "cafe.*" expansion/screen-01-a-decomposed.txt; grep -o ".*abc" expansion/screen-02-c-standalone.txt; grep -o "ab.*cd" expansion/screen-03-b-cjk.txt; } | cat -v
```

```output
== run A current file a.txt: decomposed café — the recorded match is only the mark bytes; the whole é glyph is the styled cluster ==
./a.txt  ── ./a.txt ────────────────────────────────────────────────────────────
./c.txt  1  café

== after n: c.txt line 1 — the standalone mark renders as the ◌́ fallback cell, highlighted ==
./a.txt  ── ./c.txt ────────────────────────────────────────────────────────────
./c.txt  1  ◌́abc

== run B current file b.txt: ab世cd — the two-cell CJK glyph is the styled match ==
./b.txt  ── ./b.txt ────────────────────────────────────────────────────────────
         1  ab世cd

== on-screen bytes (cat -v): e + raw mark bytes in a.txt; ◌ + mark in c.txt; 世 in b.txt ==
cafeM-LM-^A
./c.txt  1  M-bM-^WM-^LM-LM-^Aabc
abM-dM-8M-^Vcd
```

## Verdict

Issue #21 is verified. FileBuffer expands every recorded submatch's mapped span outward to `Lead`-marked cluster boundaries (`clusterSpan`), so partial-cluster matches highlight whole glyphs: on the real binary a search for the bare combining-mark bytes `\xcc\x81` lit the entire `é` glyph of decomposed `café`, and a `世` search lit both cells of the wide glyph. A standalone combining cluster takes a provisional cell on a `◌` dotted-circle base — the visible fallback — which lit as a real highlighted cell at c.txt's line start (a bare mark would have merged into the previous cell and painted nothing). Wrap- and clip-edge filler cells are `Blank`-marked and never take match styling even under a covering span, while markers still paint their positions. Reveal and the hidden-match indicators consume the same expanded spans unchanged: a match recorded mid-cluster reveals and indicator-counts from its cluster start.

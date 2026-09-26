# Issue #39: Render from the shared grapheme/cell model

*2026-09-25T00:50:57Z by Showboat 0.6.1*
<!-- showboat-id: fb005ca4-f353-4901-bdf7-e097636046da -->

Walkthrough for [Issue #39](../../../tasks/039-render-from-shared-grapheme-cell-model.md): the renderer now draws every piece of display geometry from one shared ANSI-aware grapheme/cell model per `Notes/PRD-vrg.md` (*Text, graphemes, and safe presentation* and *Navigation, viewport, and logical anchors*). `internal/present/cellwidth.go` is the single authority — `CellWidth`, `TruncateLeft`, `Truncate`, `Cut`, and `Wrap` implement one grapheme policy (a base plus its combining marks is one cluster, an emoji ZWJ sequence is one two-cell cluster, escape/control sequences paint no cells). Every consumer routes through it: line rendering and match styling emit `present.Line` cells directly, file-list entry padding and `listWidth`, indicator-column sizing, `filenameRule` fitting, the file-change pop-up's truncation and centring, the overlay's wrap and splice, and `Theme.Overlay`'s border sizing and padding. A source-scanning guard permits `utf8.DecodeRuneInString` in that helper file alone. All generated artifacts live in this directory: the built `vrg` binary, the `demo-cell-model.sh` tmux harness, and its `manual-cell-model/` captures. Test durations are stripped so the document verifies cleanly.

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

## Composed-view cluster tests

`internal/app/cellmodel_test.go` pins the composed `View()` output against the shared cell model, with decomposed combining marks throughout: a match overlapping a two-cell CJK character styles exactly that cluster's cells and never swallows the following character; a combining-only match paints the whole é cell; an interior-bytes match inside an emoji ZWJ sequence covers the whole two-cell cluster; a clip edge splitting a wide cluster blanks the in-window cell; file-list entries and the filename rule measure wide and combining names by painted cells; the indicator columns report a wide-cluster match hidden until its whole cluster paints; and the file-change pop-up's interior width, leading-`…` truncation, and centring hold for wide and combining paths.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestComposedView|TestPopupWide|TestDecodeRune|TestOverlaySizes|TestCellWidth|TestTruncateLeft|TestTruncateCut' ./internal/app ./internal/present ./internal/theme 2>&1 | grep -vE '^(=== RUN|    --- PASS)' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestComposedViewWideClusterMatchCoversExactlyItsCells
--- PASS: TestComposedViewCombiningClusterStaysWhole
--- PASS: TestComposedViewZWJClusterMeasuredAndUnsplit
--- PASS: TestComposedViewWidePathListAndRuleGeometry
--- PASS: TestComposedViewIndicatorColumnWideText
--- PASS: TestPopupWideAndCombiningPathGeometry
--- PASS: TestTruncateLeftGraphemeSafe
--- PASS: TestDecodeRuneInStringOnlyInCellHelper
--- PASS: TestComposedViewContentStaysInsidePanel
PASS
ok  	vrg/internal/app
--- PASS: TestCellWidthSharedGraphemePolicy
--- PASS: TestTruncateLeftSharedGraphemePolicy
--- PASS: TestTruncateCutAndWrapShareThePolicy
PASS
ok  	vrg/internal/present
--- PASS: TestOverlaySizesByMeasuredCellWidth
PASS
ok  	vrg/internal/theme
```

## The mechanical guard

`internal/app/guard_test.go`'s `TestDecodeRuneInStringOnlyInCellHelper` parses every non-test production `.go` file under `internal/` and `cmd/` and permits `utf8.DecodeRuneInString` only in `internal/present/cellwidth.go` — an occurrence anywhere else fails, including one in a newly added file or package, so no display-geometry consumer can grow a private rune-decoding width or truncation loop that evades the shared policy. The demonstration below plants a probe calling the symbol inside `internal/app`, watches the guard reject it, then removes it.

```bash
cd /home/chris/vrg && cat > internal/app/tmp_decode_probe.go <<'EOF'
package app

import "unicode/utf8"

func tmpDecodeProbe(s string) rune { r, _ := utf8.DecodeRuneInString(s); return r }
EOF
go test -count=1 -run TestDecodeRuneInStringOnlyInCellHelper ./internal/app >/tmp/g39.txt 2>&1; st=$?; tail -3 /tmp/g39.txt | sed -E 's/[[:space:]][0-9.]*s$//'; rm internal/app/tmp_decode_probe.go; echo "guard-exit=$st (fails as designed)"; go test -count=1 -run TestDecodeRuneInStringOnlyInCellHelper ./internal/app | tail -2 | sed -E 's/[[:space:]][0-9.]*s$//'
```

```output
FAIL
FAIL	vrg/internal/app
FAIL
guard-exit=1 (fails as designed)
ok  	vrg/internal/app
```

## Manual check — CJK, combining, and ZWJ clusters on a live terminal

`demo-cell-model.sh` (checked in here) drives the built `vrg` on a real tmux PTY at 80x24. Its `fakebin/rg` emits: a combining-only match on the mark's bytes alone in `xe<U+0301>yz`, an interior-bytes match inside the emoji ZWJ sequence in `a👨‍👩‍👧c`, a `世界` match on a line 200+ cells long, and a second file named `世界名.txt`. Stage 1 reconstructs the match-styled runs from `capture-pane -e` and requires each to equal its whole cluster exactly. Stage 2 leaves wrap mode and pans once so the window edge splits `世`'s two cells — the in-window cell must paint blank, never half the glyph. Stage 3 presses `n` to cross files and checks the file-change pop-up: the interior is the 10-cell wide path and all three border rows measure the same width.

```bash
cd /home/chris/vrg/Notes/walkthroughs/039-04/code-walkthrough && ./demo-cell-model.sh
```

```output
styled run 'é': combining-only match covers the whole é cluster
styled run '👨\u200d👩\u200d👧': interior match covers the whole ZWJ cluster
styled run '世界': the CJK match covers both cells of both clusters
mixed.txt   ── mixed.txt ───────────────────────────────────────────────────────
世界名.txt  1  xéyz
            2  a👨‍👩‍👧c
            3  p12345678世界xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
               xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
clip edge blanked the split 世 cell; 界 paints whole: '3_  界'
mixed.txt   ── mixed.txt ───────────────────────────────────────────────────────
世界名.txt  1*
            2*
            3_  界xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx

pop-up rows: ['┌──────────┐', '│世界名.txt│', '└──────────┘']
measured widths: [12, 12, 12]
pop-up interior carries the wide path; all border rows align
mixed.txt   ── 世界名.txt ──────────────────────────────────────────────────────
世界名.txt  1  content of wide file








                                  ┌──────────┐
                                  │世界名.txt│
exit=0
```

## Result

Issue #39 is verified. The composed tests and the live captures agree: each highlight covers its whole grapheme cluster at exactly its cell width — the combining-only match paints `e◌́` as one cell, the interior-bytes match paints the entire `👨‍👩‍👧` cluster across both its cells, and the CJK match covers both two-cell clusters. The clip edge never splits a cluster: the split `世` cell renders blank while `界` paints whole, and the hidden-match `*` indicators on lines 1–2 report the obscured matches through the same cell geometry. The file-change pop-up sizes and centres by measured cells — the `世界名.txt` interior is 10 painted cells and every border row measures 12. The source-scanning guard passes against the tree and rejects a planted violation, so the shared grapheme/cell model cannot be silently bypassed. `go build`, `go vet`, and `go test ./...` all pass, and the session exits 0.

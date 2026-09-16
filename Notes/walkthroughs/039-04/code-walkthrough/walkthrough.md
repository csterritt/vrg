# Issue #39: Final rendering uses the shared grapheme/cell model end to end

*2026-09-15T18:04:41Z by Showboat 0.6.1*
<!-- showboat-id: 47dba80f-5b8a-4b44-a574-9c61b410d151 -->

Walkthrough for Issue #39 (Notes/tasks/039-render-from-shared-grapheme-cell-model.md): final rendering now consumes the shared grapheme/cell model end to end. FileBuffer and Viewport already produced grapheme-aware, cell-based ranges; the final renderer previously re-derived geometry from rune counts, so two-cell CJK highlights consumed the next character, combining sequences were styled/clipped independently of their base, emoji ZWJ sequences could split, and list/overlay/filename/pop-up widths were measured in runes. References: Notes/issues/039-render-from-shared-grapheme-cell-model.md and Notes/PRD-vrg.md (Text, graphemes, and safe presentation; Navigation, viewport, and logical anchors).

Contracts verified:
- One ANSI-aware grapheme/cell helper (internal/safepresentation/cellwidth.go: GraphemeClustersANSI, CellWidth, TruncateLeftCells) is the single display-geometry policy; ANSI CSI sequences measure zero cells.
- renderLineWithHighlights renders from Line.Clusters: a highlight overlapping a cluster styles exactly that cluster's cells — two cells for CJK, one for a base-plus-combining sequence, two for an emoji ZWJ sequence — and never swallows or splits neighbours.
- Wrapping, left/right clipping, left/right truncation, file-list padding, indicator sizing, filename-row fitting, pop-up truncation/centring, and Theme overlay sizing/padding all measure in cells through the helper.
- A static guard scans every non-test production .go file under internal/ and cmd/ and rejects utf8.DecodeRuneInString anywhere except the shared helper file.
- Wrapped rows retain source-line cluster byte offsets; the renderer subtracts Line.StartByte to index row-local Display (found and fixed via this walkthrough's narrow-terminal PTY run).

All generated artifacts live in this directory.

```bash
cd /home/chris/vrg && go build ./... && go vet ./... && echo GATES-OK
```

```output
GATES-OK
```

## Composed-view tests and the static guard

internal/app/cell_render_test.go asserts the composed View() layout cell-by-cell: a match overlapping a two-cell CJK character highlights exactly that cluster's cells and never swallows the following character; a base-plus-combining sequence is styled as one cluster; an emoji ZWJ sequence occupies its measured width and is never split by a highlight or clip boundary; file-list padding, filename-row fitting, pop-up truncation/centring, and overlay wrapping measure wide and combining text in cells. internal/theme/theme_test.go covers overlay border alignment for wide, combining, and mixed-width text. internal/safepresentation/cellwidth_test.go covers the shared helper (ANSI-aware segmentation, CellWidth, TruncateLeftCells).

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ ./internal/theme/ ./internal/safepresentation/ -run '^(TestRenderCJKHighlightCoversExactlyClusterCells|TestRenderCombiningClusterStyledAsOne|TestRenderEmojiZWJHighlightNeverSplit|TestRenderEmojiZWJClipBoundaryNeverSplits|TestFileListWidePathPaddingCells|TestFilenameRowWidePathFitsPanel|TestPopupWidePathTruncatedToCells|TestPopupWidePathCentredInCells|TestPopupCombiningPathNeverSplitsCluster|TestOverlayWideTextWrapsAndPadsToCells|TestDecodeRuneInStringOnlyInSharedCellHelper|TestOverlayWideTextBorderAlignment|TestOverlayCombiningTextBorderAlignment|TestOverlayMixedWidthRowsAlign|TestCellWidth|TestGraphemeClustersANSI|TestGraphemeClustersANSIPlain|TestTruncateLeftCells)$' -timeout 120s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestRenderCJKHighlightCoversExactlyClusterCells
--- PASS: TestRenderCJKHighlightCoversExactlyClusterCells (0.00s)
=== RUN   TestRenderCombiningClusterStyledAsOne
--- PASS: TestRenderCombiningClusterStyledAsOne (0.00s)
=== RUN   TestRenderEmojiZWJHighlightNeverSplit
--- PASS: TestRenderEmojiZWJHighlightNeverSplit (0.00s)
=== RUN   TestRenderEmojiZWJClipBoundaryNeverSplits
--- PASS: TestRenderEmojiZWJClipBoundaryNeverSplits (0.00s)
=== RUN   TestFileListWidePathPaddingCells
--- PASS: TestFileListWidePathPaddingCells (0.00s)
=== RUN   TestFilenameRowWidePathFitsPanel
--- PASS: TestFilenameRowWidePathFitsPanel (0.00s)
=== RUN   TestPopupWidePathTruncatedToCells
--- PASS: TestPopupWidePathTruncatedToCells (0.00s)
=== RUN   TestPopupWidePathCentredInCells
--- PASS: TestPopupWidePathCentredInCells (0.00s)
=== RUN   TestPopupCombiningPathNeverSplitsCluster
--- PASS: TestPopupCombiningPathNeverSplitsCluster (0.00s)
=== RUN   TestOverlayWideTextWrapsAndPadsToCells
--- PASS: TestOverlayWideTextWrapsAndPadsToCells (0.00s)
=== RUN   TestDecodeRuneInStringOnlyInSharedCellHelper
--- PASS: TestDecodeRuneInStringOnlyInSharedCellHelper (0.00s)
PASS
ok  	vrg/internal/app
=== RUN   TestOverlayWideTextBorderAlignment
--- PASS: TestOverlayWideTextBorderAlignment (0.00s)
=== RUN   TestOverlayCombiningTextBorderAlignment
--- PASS: TestOverlayCombiningTextBorderAlignment (0.00s)
=== RUN   TestOverlayMixedWidthRowsAlign
--- PASS: TestOverlayMixedWidthRowsAlign (0.00s)
PASS
ok  	vrg/internal/theme
=== RUN   TestCellWidth
=== RUN   TestCellWidth/empty
=== RUN   TestCellWidth/ascii
=== RUN   TestCellWidth/wide_cjk
=== RUN   TestCellWidth/wide_mixed
=== RUN   TestCellWidth/combining_cluster
=== RUN   TestCellWidth/combining_mid-string
=== RUN   TestCellWidth/emoji_zwj
=== RUN   TestCellWidth/ellipsis
=== RUN   TestCellWidth/box_drawing
=== RUN   TestCellWidth/ansi_around_text
=== RUN   TestCellWidth/ansi_around_wide
=== RUN   TestCellWidth/ansi_only
--- PASS: TestCellWidth (0.00s)
    --- PASS: TestCellWidth/empty (0.00s)
    --- PASS: TestCellWidth/ascii (0.00s)
    --- PASS: TestCellWidth/wide_cjk (0.00s)
    --- PASS: TestCellWidth/wide_mixed (0.00s)
    --- PASS: TestCellWidth/combining_cluster (0.00s)
    --- PASS: TestCellWidth/combining_mid-string (0.00s)
    --- PASS: TestCellWidth/emoji_zwj (0.00s)
    --- PASS: TestCellWidth/ellipsis (0.00s)
    --- PASS: TestCellWidth/box_drawing (0.00s)
    --- PASS: TestCellWidth/ansi_around_text (0.00s)
    --- PASS: TestCellWidth/ansi_around_wide (0.00s)
    --- PASS: TestCellWidth/ansi_only (0.00s)
=== RUN   TestGraphemeClustersANSI
--- PASS: TestGraphemeClustersANSI (0.00s)
=== RUN   TestGraphemeClustersANSIPlain
--- PASS: TestGraphemeClustersANSIPlain (0.00s)
=== RUN   TestTruncateLeftCells
=== RUN   TestTruncateLeftCells/ascii_fits
=== RUN   TestTruncateLeftCells/ascii_keep
=== RUN   TestTruncateLeftCells/keep_zero
=== RUN   TestTruncateLeftCells/keep_negative
=== RUN   TestTruncateLeftCells/wide_never_split
=== RUN   TestTruncateLeftCells/wide_boundary
=== RUN   TestTruncateLeftCells/combining_kept_whole
=== RUN   TestTruncateLeftCells/combining_dropped_whole
=== RUN   TestTruncateLeftCells/emoji_zwj_kept_whole
=== RUN   TestTruncateLeftCells/emoji_zwj_dropped_whole
=== RUN   TestTruncateLeftCells/ansi_kept_tail
=== RUN   TestTruncateLeftCells/ansi_before_kept_text
=== RUN   TestTruncateLeftCells/ansi_not_counted
--- PASS: TestTruncateLeftCells (0.00s)
    --- PASS: TestTruncateLeftCells/ascii_fits (0.00s)
    --- PASS: TestTruncateLeftCells/ascii_keep (0.00s)
    --- PASS: TestTruncateLeftCells/keep_zero (0.00s)
    --- PASS: TestTruncateLeftCells/keep_negative (0.00s)
    --- PASS: TestTruncateLeftCells/wide_never_split (0.00s)
    --- PASS: TestTruncateLeftCells/wide_boundary (0.00s)
    --- PASS: TestTruncateLeftCells/combining_kept_whole (0.00s)
    --- PASS: TestTruncateLeftCells/combining_dropped_whole (0.00s)
    --- PASS: TestTruncateLeftCells/emoji_zwj_kept_whole (0.00s)
    --- PASS: TestTruncateLeftCells/emoji_zwj_dropped_whole (0.00s)
    --- PASS: TestTruncateLeftCells/ansi_kept_tail (0.00s)
    --- PASS: TestTruncateLeftCells/ansi_before_kept_text (0.00s)
    --- PASS: TestTruncateLeftCells/ansi_not_counted (0.00s)
PASS
ok  	vrg/internal/safepresentation
```

The static guard above (TestDecodeRuneInStringOnlyInSharedCellHelper) passes on the clean tree. To prove it is not vacuous, drop a throwaway production file calling utf8.DecodeRuneInString into internal/app, re-run the guard, then remove the file: the scan must reject it. (The FAIL package line's trailing duration is stripped since it varies run to run.)

```bash
cd /home/chris/vrg && printf 'package app\n\nimport "unicode/utf8"\n\nvar _ = utf8.DecodeRuneInString\n' > internal/app/guard_probe_tmp.go && go test -count=1 ./internal/app/ -run 'TestDecodeRuneInStringOnlyInSharedCellHelper' 2>&1 | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'; s=${PIPESTATUS[0]}; rm -f internal/app/guard_probe_tmp.go; echo guard-caught-exit=$s
```

```output
--- FAIL: TestDecodeRuneInStringOnlyInSharedCellHelper (0.00s)
    decode_guard_test.go:74: utf8.DecodeRuneInString found outside internal/safepresentation/cellwidth.go (display geometry must use the shared grapheme/cell helper): [internal/app/guard_probe_tmp.go]
FAIL
FAIL	vrg/internal/app
FAIL
guard-caught-exit=1
```

## Binary demo: whole-cluster highlights, wide paths, and cell-measured overlay

The vrg binary is built into this directory and run under a PTY (runpty.py, pyte screen capture) at 60x12 against a fake rg (fakebin/rg, forced first on PATH inside the PTY child). The fake stream reports three paths under demo/: mixed.txt holds a two-cell CJK match, a base-plus-combining e + U+0301 match, an emoji ZWJ match, and a 25-character CJK run for wrap/clip edges; 中é名.txt is a real wide-character path (combining mark in the name) exercised by the file list, filename row, and file-change pop-up; 中zzmissing.txt is absent from disk so navigating to it opens the non-fatal read-failure overlay containing a wide path. The rg submatches deliberately cover only part of the combining and ZWJ clusters, so the highlight must expand to the whole cluster.

Each frame prints screen rows between | markers; beneath any row containing match-styled cells a marker row prints one ^ per styled cell, so highlight coverage can be checked cell-by-cell against the text row's grapheme clusters.

Key sequence: five n presses reach 中é名.txt (file-change pop-up centred in cells); the sixth n reaches the missing wide path (read-failure overlay with wide text, borders aligned); q dismisses the overlay; two p presses return to mixed.txt; w switches to run-off-edge; two > pans push the window right onto the CJK run so the left edge splits a two-cell cluster — the split cell renders blank, never half a glyph; q exits.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/039-04/code-walkthrough/vrg ./cmd/vrg && echo BUILD-OK
```

```output
BUILD-OK
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/039-04/code-walkthrough && VRG_KEYS='n,n,n,n,n,n,q,p,p,w,>,>,q' VRG_DELAY=0.9 VRG_WIDTH=60 VRG_HEIGHT=12 timeout 120 uv run --with pyte python3 runpty.py ./vrg 'needle|中|e|👩' demo/src 2>&1; echo exit=$?
```

```output
=== initial screen (60x12) ===
|demo/src/mixed.txt     ── demo/src/mixed.txt ──             |
|demo/src/中é名.txt     1  a中b needle c                     |
|                           ^^  ^^^^^^                       |
|demo/中zzmissing.txt   2  déf needle h                      |
|                           ^  ^^^^^^                        |
|                       3  i👩j needle k                     |
|                           ^^  ^^^^^^                       |
|                       4  needle 中中中中中中中中中中中中中 |
|                          ^^^^^^ ^^^^^^^^^^^^^^^^^^^^^^^^^^ |
|                          中中中中中中中中中中中中 tail need|
|                          ^^^^^^^^^^^^^^^^^^^^^^^^      ^^^^|
|                          le                                |
|                          ^^                                |
|                       5  needle tail line                  |
|                          ^^^^^^                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |

=== after 'n' (60x12) ===
|demo/src/mixed.txt     ── demo/src/mixed.txt ──             |
|demo/src/中é名.txt     1  a中b needle c                     |
|                           ^^  ^^^^^^                       |
|demo/中zzmissing.txt   2  déf needle h                      |
|                           ^  ^^^^^^                        |
|                       3  i👩j needle k                     |
|                           ^^  ^^^^^^                       |
|                       4  needle 中中中中中中中中中中中中中 |
|                          ^^^^^^ ^^^^^^^^^^^^^^^^^^^^^^^^^^ |
|                          中中中中中中中中中中中中 tail need|
|                          ^^^^^^^^^^^^^^^^^^^^^^^^      ^^^^|
|                          le                                |
|                          ^^                                |
|                       5  needle tail line                  |
|                          ^^^^^^                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |

=== after 'n' (60x12) ===
|demo/src/mixed.txt     ── demo/src/mixed.txt ──             |
|demo/src/中é名.txt     1  a中b needle c                     |
|                           ^^  ^^^^^^                       |
|demo/中zzmissing.txt   2  déf needle h                      |
|                           ^  ^^^^^^                        |
|                       3  i👩j needle k                     |
|                           ^^  ^^^^^^                       |
|                       4  needle 中中中中中中中中中中中中中 |
|                          ^^^^^^ ^^^^^^^^^^^^^^^^^^^^^^^^^^ |
|                          中中中中中中中中中中中中 tail need|
|                          ^^^^^^^^^^^^^^^^^^^^^^^^      ^^^^|
|                          le                                |
|                          ^^                                |
|                       5  needle tail line                  |
|                          ^^^^^^                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |

=== after 'n' (60x12) ===
|demo/src/mixed.txt     ── demo/src/mixed.txt ──             |
|demo/src/中é名.txt     1  a中b needle c                     |
|                           ^^  ^^^^^^                       |
|demo/中zzmissing.txt   2  déf needle h                      |
|                           ^  ^^^^^^                        |
|                       3  i👩j needle k                     |
|                           ^^  ^^^^^^                       |
|                       4  needle 中中中中中中中中中中中中中 |
|                          ^^^^^^ ^^^^^^^^^^^^^^^^^^^^^^^^^^ |
|                          中中中中中中中中中中中中 tail need|
|                          ^^^^^^^^^^^^^^^^^^^^^^^^      ^^^^|
|                          le                                |
|                          ^^                                |
|                       5  needle tail line                  |
|                          ^^^^^^                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |

=== after 'n' (60x12) ===
|demo/src/mixed.txt     ── demo/src/mixed.txt ──             |
|demo/src/中é名.txt     1  a中b needle c                     |
|                           ^^  ^^^^^^                       |
|demo/中zzmissing.txt   2  déf needle h                      |
|                           ^  ^^^^^^                        |
|                       3  i👩j needle k                     |
|                           ^^  ^^^^^^                       |
|                       4  needle 中中中中中中中中中中中中中 |
|                          ^^^^^^ ^^^^^^^^^^^^^^^^^^^^^^^^^^ |
|                          中中中中中中中中中中中中 tail need|
|                          ^^^^^^^^^^^^^^^^^^^^^^^^      ^^^^|
|                          le                                |
|                          ^^                                |
|                       5  needle tail line                  |
|                          ^^^^^^                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |

=== after 'n' (60x12) ===
|demo/src/mixed.txt     ── demo/src/中é名.txt ──             |
|demo/src/中é名.txt     1  needle in wide name               |
|                          ^^^^^^                            |
|demo/中zzmissing.txt                                        |
|                                                            |
|                                                            |
|                     demo/src/中é名.txt                     |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |

=== after 'n' (60x12) ===
|┌──────────────────────────────────────────────┐            |
|│ open demo/中zzmissing.txt: no such file or d │            |
|│ irectory                                     │            |
|└──────────────────────────────────────────────┘            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |

=== after 'q' (60x12) ===
|demo/src/mixed.txt     ── …o/中zzmissing.txt ── (unreadable)|
|demo/src/中é名.txt     (unreadable)                         |
|demo/中zzmissing.txt                                        |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |

=== after 'p' (60x12) ===
|demo/src/mixed.txt     ── demo/src/中é名.txt ──             |
|demo/src/中é名.txt     1  needle in wide name               |
|                          ^^^^^^                            |
|demo/中zzmissing.txt                                        |
|                                                            |
|                                                            |
|                     demo/src/中é名.txt                     |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |

=== after 'p' (60x12) ===
|demo/src/mixed.txt     ── demo/src/mixed.txt ──             |
|demo/src/中é名.txt     1  a中b needle c                     |
|                           ^^  ^^^^^^                       |
|demo/中zzmissing.txt   2  déf needle h                      |
|                           ^  ^^^^^^                        |
|                       3  i👩j needle k                     |
|                           ^^  ^^^^^^                       |
|                       4  needle 中中中中中中中中中中中中中 |
|                          ^^^^^^ ^^^^^^^^^^^^^^^^^^^^^^^^^^ |
|                     demo/src/mixed.txt                     |
|                          le                                |
|                          ^^                                |
|                       5  needle tail line                  |
|                          ^^^^^^                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |

=== after 'w' (60x12) ===
|demo/src/mixed.txt     ── demo/src/mixed.txt ──             |
|demo/src/中é名.txt     1  a中b needle c                     |
|                           ^^  ^^^^^^                       |
|demo/中zzmissing.txt   2  déf needle h                      |
|                           ^  ^^^^^^                        |
|                       3  i👩j needle k                     |
|                           ^^  ^^^^^^                       |
|                       4  needle 中中中中中中中中中中中中中 |
|                          ^^^^^^ ^^^^^^^^^^^^^^^^^^^^^^^^^^ |
|                       5  needle tail line                  |
|                          ^^^^^^                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |

=== after '>' (60x12) ===
|demo/src/mixed.txt     ── demo/src/mixed.txt ──             |
|demo/src/中é名.txt     1* e c                               |
|                        ^ ^                                 |
|demo/中zzmissing.txt   2*  h                                |
|                        ^                                   |
|                       3* e k                               |
|                        ^ ^                                 |
|                       4*  中中中中中中中中中中中中中中中中 |
|                        ^  ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^ |
|                       5* l line                            |
|                        ^                                   |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |

=== after '>' (60x12) ===
|demo/src/mixed.txt     ── demo/src/mixed.txt ──             |
|demo/src/中é名.txt     1*                                   |
|                        ^                                   |
|demo/中zzmissing.txt   2*                                   |
|                        ^                                   |
|                       3*                                   |
|                        ^                                   |
|                       4*  中中中中中中中中中中中中中中中中 |
|                        ^  ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^ |
|                       5*                                   |
|                        ^                                   |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |

=== after 'q' (60x12) ===
|demo/src/中é名.txt     1*                                   |
|                        ^                                   |
|demo/中zzmissing.txt   2*                                   |
|                        ^                                   |
|                       3*                                   |
|                        ^                                   |
|                       4*  中中中中中中中中中中中中中中中中 |
|                        ^  ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^ |
|1u                     5*                                   |
|                        ^                                   |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|open demo/中zzmissing.txt: no such file or directory        |
|                                                            |

exit=0
exit=0
```

Reading the frames: on the initial screen the two-cell 中 highlight covers exactly two cells (^^) and the following b stays unstyled; the e + U+0301 combining cluster covers exactly one cell even though the rg submatch only spans the base byte; the 👩‍💻 ZWJ sequence covers its full measured width; the wrapped 25-character CJK run breaks on cluster boundaries with every visible cell styled; the file list is padded to the cell width of the longest entry (demo/中zzmissing.txt at 20 cells + 2). After the fifth n the file-change pop-up shows demo/src/中é名.txt centred in cells. After the sixth n the read-failure overlay wraps the wide path open demo/中zzmissing.txt: no such file or directory inside borders that stay column-aligned. The q frame shows the filename row left-truncated by whole clusters (── …o/中zzmissing.txt ── (unreadable)). In run-off-edge mode each > pan shifts the window ten cells: rows show the inverse * indicator in the gutter for matches hidden left, and where the window edge splits a two-cell cluster the split cell renders as a blank rather than half a glyph — nothing is ever clipped mid-cluster. The final q exits; exit=0 is vrg's exit status.


# Issue #38: Install the viewport with the layout key's text width

*2026-09-15T16:33:23Z by Showboat 0.6.1*
<!-- showboat-id: de01320e-d38e-41e0-a083-1367c28f9f21 -->

Walkthrough for Issue #38 (Notes/tasks/038-viewport-content-panel-width.md): the viewport is now installed with the layout key's text width — terminal width minus file-list width minus the one-cell separator, minus gutter, minus the reserved right-indicator width — instead of a raw-terminal-derived width. References: Notes/issues/038-viewport-content-panel-width.md and Notes/PRD-vrg.md (File list and layout; Layout and indicators; Navigation, viewport, and logical anchors).

Contracts verified:
- The three widths stay distinct: terminal width, panel width (terminal minus list minus separator), text width (panel minus gutter minus reserved indicator).
- LayoutKey.TextWidth is installed at every viewport install site: LayoutReadyMsg, the synchronous factory seam, the cache-hit fast path, and the WindowSizeMsg resize path.
- Run-off-edge clipping, panning, minimal reveal, and the hidden-content indicators are measured against the text width; the reserved right-indicator column sits at the panel's right edge outside the text area and no composed row exceeds the terminal width.
- File-list hide/show and resize re-measure the text width from the new panel width.

All generated artifacts live in this directory.

```bash
cd /home/chris/vrg && go build ./... && go vet ./... && echo GATES-OK
```

```output
GATES-OK
```

## Corrected horizontal tests

The tests in internal/app/reveal_horizontal_test.go, pan_test.go, and indicator_test.go now compute every expected position from the text width — terminal width minus the actual ListWidth() minus the one-cell separator minus the gutter minus viewport.ReservedWidth(mode) — through the textWidthFor helper, covering list-shown, list-hidden, wrap-mode (zero reservation), resize re-measurement, the synchronous factory-seam install, and a composed-view assertion that no rendered row exceeds the terminal width and the reserved right-indicator column sits at the panel's right edge outside the text area.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run '^(TestStartupHorizontalRevealRunOffEdge|TestStartupHorizontalRevealWrapModeNoOp|TestSameFileNavigationHorizontalReveal|TestSameFileNavigationHorizontalRevealBack|TestSameFileNavigationHorizontalVisibleNoMove|TestEveryNavigationTriggersHorizontalReveal|TestFileChangeResetThenHorizontalReveal|TestFileChangeResetOrdering|TestListHiddenHorizontalReveal|TestWrapModeZeroReservedIndicator|TestResizeRemeasuresTextWidth|TestFactorySeamInstallsLayoutTextWidth|TestComposedViewRowsFitTerminal|TestPanRightHalfWidth|TestPanLeftHalfWidth)$' -timeout 120s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestPanRightHalfWidth
--- PASS: TestPanRightHalfWidth (0.00s)
=== RUN   TestPanLeftHalfWidth
--- PASS: TestPanLeftHalfWidth (0.00s)
=== RUN   TestStartupHorizontalRevealRunOffEdge
--- PASS: TestStartupHorizontalRevealRunOffEdge (0.00s)
=== RUN   TestStartupHorizontalRevealWrapModeNoOp
--- PASS: TestStartupHorizontalRevealWrapModeNoOp (0.00s)
=== RUN   TestSameFileNavigationHorizontalReveal
--- PASS: TestSameFileNavigationHorizontalReveal (0.00s)
=== RUN   TestSameFileNavigationHorizontalRevealBack
--- PASS: TestSameFileNavigationHorizontalRevealBack (0.00s)
=== RUN   TestSameFileNavigationHorizontalVisibleNoMove
--- PASS: TestSameFileNavigationHorizontalVisibleNoMove (0.00s)
=== RUN   TestEveryNavigationTriggersHorizontalReveal
--- PASS: TestEveryNavigationTriggersHorizontalReveal (0.00s)
=== RUN   TestFileChangeResetThenHorizontalReveal
--- PASS: TestFileChangeResetThenHorizontalReveal (0.00s)
=== RUN   TestFileChangeResetOrdering
--- PASS: TestFileChangeResetOrdering (0.00s)
=== RUN   TestListHiddenHorizontalReveal
--- PASS: TestListHiddenHorizontalReveal (0.00s)
=== RUN   TestWrapModeZeroReservedIndicator
--- PASS: TestWrapModeZeroReservedIndicator (0.00s)
=== RUN   TestResizeRemeasuresTextWidth
--- PASS: TestResizeRemeasuresTextWidth (0.00s)
=== RUN   TestFactorySeamInstallsLayoutTextWidth
--- PASS: TestFactorySeamInstallsLayoutTextWidth (0.00s)
=== RUN   TestComposedViewRowsFitTerminal
--- PASS: TestComposedViewRowsFitTerminal (0.00s)
PASS
ok  	vrg/internal/app
```

## Binary demo: pan and file-list toggle stay inside the panel boundary

The vrg binary is built into this directory and run under a PTY (runpty.py, pyte screen capture) at 60x12 against a fake rg (fakebin/rg) that reports a needle match on a 300-cell line in demo/src/alpha.txt plus a short-line match in demo/src/beta.txt. The file list is visible (width 20: longest path 18 + 2), so the panel is 60 - 20 - 1 = 39 cells and the run-off-edge text width is 39 - 3 (gutter) - 1 (reserved indicator) = 35.

Key sequence: w enters run-off-edge mode; > pans right ten cells three times; left hides the file list (the panel grows to 59 cells and the text width to 55); two more > pans re-measure against the wider panel; right shows the list again (text width returns to 35); q exits. Rows are printed between | markers: content, indicators, and padding stay inside the panel boundary — the right * sits at the last cell (the panel's right edge), the left _ stays in the gutter, and no text bleeds under or past the file list.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/038-04/code-walkthrough/vrg ./cmd/vrg && echo BUILD-OK
```

```output
BUILD-OK
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/038-04/code-walkthrough && PATH=$PWD/fakebin:$PATH VRG_KEYS='w,>,>,>,left,>,>,right,q' VRG_DELAY=0.4 VRG_WIDTH=60 VRG_HEIGHT=12 timeout 60 uv run --with pyte python3 runpty.py ./vrg needle demo/src 2>&1; echo exit=$?
```

```output
=== initial screen (60x12) ===
|demo/src/alpha.txt   ── demo/src/alpha.txt ──               |
|demo/src/beta.txt    1  pack my box with five dozen liquor j|
|                        ugs pack my box with five dozen liqu|
|                        or jugs pack my box with five dozen |
|                        liquor jugs pack my box with five do|
|                        zen liquor jugs pack my box with fiv|
|                        e dozen liquor jugs pack my box with|
|                         five dozen liquor jugs pack my bone|
|                        edlepack my box with five dozen liqu|
|                        or jugs pack                        |
|                     2  tail line of alpha                  |
|                                                            |

=== after 'w' (60x12) ===
|demo/src/alpha.txt   ── demo/src/alpha.txt ──               |
|demo/src/beta.txt    1  pack my box with five dozen liquor *|
|                     2  tail line of alpha                  |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |

=== after '>' (60x12) ===
|demo/src/alpha.txt   ── demo/src/alpha.txt ──               |
|demo/src/beta.txt    1_ x with five dozen liquor jugs pack *|
|                     2_ of alpha                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |

=== after '>' (60x12) ===
|demo/src/alpha.txt   ── demo/src/alpha.txt ──               |
|demo/src/beta.txt    1_ e dozen liquor jugs pack my box wit*|
|                     2_                                     |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |

=== after '>' (60x12) ===
|demo/src/alpha.txt   ── demo/src/alpha.txt ──               |
|demo/src/beta.txt    1_ quor jugs pack my box with five doz*|
|                     2_                                     |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |

=== after 'left' (60x12) ===
| ── demo/src/alpha.txt ──                                   |
| 1_ quor jugs pack my box with five dozen liquor jugs pack *|
| 2_                                                         |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |

=== after '>' (60x12) ===
| ── demo/src/alpha.txt ──                                   |
| 1_ pack my box with five dozen liquor jugs pack my box wit*|
| 2_                                                         |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |

=== after '>' (60x12) ===
| ── demo/src/alpha.txt ──                                   |
| 1_ x with five dozen liquor jugs pack my box with five doz*|
| 2_                                                         |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |

=== after 'right' (60x12) ===
|demo/src/alpha.txt   ── demo/src/alpha.txt ──               |
|demo/src/beta.txt    1_ x with five dozen liquor jugs pack *|
|                     2_                                     |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |

=== after 'q' (60x12) ===
|demo/src/alpha.txt   ── demo/src/alpha.txt ──               |
|demo/src/beta.txt    1_ x with five dozen liquor jugs pack *|
|                     2_                                     |
|                    1u                                      |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |
|                                                            |

exit=0
exit=0
```

Reading the frames: after 'w' the clipped line fills the 35-cell text area and the reserved right-indicator cell shows * at the last terminal column (the panel's right edge). Each '>' shifts the window ten cells — the phrase content visibly advances — while the left _ indicator appears in the gutter and * stays pinned at the panel edge; the list cells (columns 0-19) are never overwritten by content. After 'left' the list disappears, the panel starts at column 0, and the text area widens to 55 cells with * still at the last column; further > pans re-measure against the wider text width. After 'right' the list returns and the text area is back to 35 cells. The 'q' frame shows the alt-screen teardown residue; exit=0 is the exit status. Under the pre-fix code the viewport measured these boundaries from the raw terminal width, so the text area and indicator would have run 20 cells past the panel edge — under and past the file list.

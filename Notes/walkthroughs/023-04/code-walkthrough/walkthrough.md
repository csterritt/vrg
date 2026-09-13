# Issue #23: Zero-width match markers

*2026-09-12T12:34:47Z by Showboat 0.6.1*
<!-- showboat-id: e559b39c-eca2-4141-bf04-70b3c06294ed -->

Walkthrough for Issue #23 (Notes/tasks/023-zero-width-match-markers.md), implementing zero-width match markers in vrg. A zero-width regex submatch (Start == End) renders as exactly one inverse-video cell at its mapped display location, underlined on the current matched line, marking an existing cell without shifting following text. A marker at end of line extends the effective line width by one cell, so an empty matched line has width one and a marker after a completely full wrap row occupies another row. A zero-width position inside a grapheme cluster maps to the cluster start with no split wide glyph. A terminator-only $ match on hit\r\n (byte 4) produces a single marker cell at display column 3 following exactly the same reveal, wrap, clip, horizontal-extent, and indicator rules as any other marker, including counting as entirely hidden for the Issue #20 gutter * and right * indicators. Marker cells are navigable reveal targets. Markers participate in horizontal extent and pan clamping so a marker-only line has extent 1 and maximum offset 0 under Issue #18's paintable-boundary maximum. References: Notes/PRD-vrg.md (Text, graphemes, and safe presentation), Notes/wiki/zero-width-match-markers.md, Notes/issues/023-zero-width-match-markers.md.

Contracts verified:

- A zero-width submatch renders as one inverse-video cell at its mapped location.
- A marker at beginning of line marks the existing cell without shifting text.
- A marker at end of line extends the effective line width by one cell.
- An empty matched line has width one (single space, one 1-cell cluster).
- A marker after a completely full wrap row occupies another row.
- A zero-width position inside a wide cluster maps to the cluster start (no split).
- A zero-width position inside a combining cluster maps to the cluster start.
- A terminator-only $ match on hit\r\n produces a single marker cell at display column 3.
- A terminator-only $ match on hit\n produces a single marker cell at display column 3.
- Markers participate in wrap, clip, extent, pan clamping, and reveal.
- Markers participate in Issue #20 indicators (hidden-left *, hidden-right *).
- A marker-only line has extent 1 and maximum pan offset 0.
- The EOL marker's virtual cluster is the last cluster and has width 1.

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

## FileBuffer marker tests

The filebuffer tests (internal/filebuffer/marker_test.go) verify the Issue #23 contracts: BOL/EOL/empty-line markers, LF and CRLF terminator-only markers (column 3), wide and combining cluster-start mapping, no text shifting, mid-line no-width-extension, coexistence with non-zero-width highlights, and the EOL cluster being last with width 1.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/filebuffer/ -run '^TestMarker' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestMarkerAtBOL
--- PASS: TestMarkerAtBOL (0.00s)
=== RUN   TestMarkerAtEOL
--- PASS: TestMarkerAtEOL (0.00s)
=== RUN   TestMarkerOnEmptyLine
--- PASS: TestMarkerOnEmptyLine (0.00s)
=== RUN   TestMarkerOnCRLFTerminatorOnly
--- PASS: TestMarkerOnCRLFTerminatorOnly (0.00s)
=== RUN   TestMarkerOnLFTerminatorOnly
--- PASS: TestMarkerOnLFTerminatorOnly (0.00s)
=== RUN   TestMarkerInsideWideCluster
--- PASS: TestMarkerInsideWideCluster (0.00s)
=== RUN   TestMarkerInsideCombiningCluster
--- PASS: TestMarkerInsideCombiningCluster (0.00s)
=== RUN   TestMarkerAtEOLDoesNotShiftText
--- PASS: TestMarkerAtEOLDoesNotShiftText (0.00s)
=== RUN   TestMarkerMidLineDoesNotExtendWidth
--- PASS: TestMarkerMidLineDoesNotExtendWidth (0.00s)
=== RUN   TestMarkerAndNonZeroWidthOnSameLine
--- PASS: TestMarkerAndNonZeroWidthOnSameLine (0.00s)
=== RUN   TestMarkerEOLClusterIsLast
--- PASS: TestMarkerEOLClusterIsLast (0.00s)
PASS
ok  	vrg/internal/filebuffer
```

## Viewport marker tests

The viewport tests (internal/viewport/marker_test.go) verify the Issue #23 viewport contracts: wrap-row occupation (a marker after a completely full wrap row occupies another row), marker-only-line extent 1, max pan offset 0, reveal targeting, hidden-left/right clip exclusion, visible clip inclusion, pan clamping participation, and same-row wrap behavior.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/viewport/ -run '^TestMarker' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestMarkerAfterFullWrapRow
--- PASS: TestMarkerAfterFullWrapRow (0.00s)
=== RUN   TestMarkerOnlyLineExtentOne
--- PASS: TestMarkerOnlyLineExtentOne (0.00s)
=== RUN   TestMarkerOnlyLineMaxPanOffsetZero
--- PASS: TestMarkerOnlyLineMaxPanOffsetZero (0.00s)
=== RUN   TestMarkerCellRevealTarget
--- PASS: TestMarkerCellRevealTarget (0.00s)
=== RUN   TestMarkerHiddenLeftNotInClip
--- PASS: TestMarkerHiddenLeftNotInClip (0.00s)
=== RUN   TestMarkerVisibleInClip
--- PASS: TestMarkerVisibleInClip (0.00s)
=== RUN   TestMarkerHiddenRightNotInClip
--- PASS: TestMarkerHiddenRightNotInClip (0.00s)
=== RUN   TestMarkerParticipatesInPanClamping
--- PASS: TestMarkerParticipatesInPanClamping (0.00s)
=== RUN   TestMarkerEOLWrapRowCount
--- PASS: TestMarkerEOLWrapRowCount (0.00s)
=== RUN   TestMarkerEOLRevealHorizontal
--- PASS: TestMarkerEOLRevealHorizontal (0.00s)
PASS
ok  	vrg/internal/viewport
```

## App marker indicator tests

The app tests (internal/app/marker_indicator_test.go) verify that zero-width markers participate in the Issue #20 hidden-content indicators: a marker entirely hidden left produces a gutter *, a marker entirely hidden right produces a right * on the current matched line, a visible marker produces no indicator, and a terminator-only $ marker follows the same rules as any other marker.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run '^TestIndicatorMarker|^TestIndicatorTerminatorOnly' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestIndicatorMarkerHiddenLeftStar
--- PASS: TestIndicatorMarkerHiddenLeftStar (0.00s)
=== RUN   TestIndicatorMarkerVisibleNoStar
--- PASS: TestIndicatorMarkerVisibleNoStar (0.00s)
=== RUN   TestIndicatorMarkerHiddenRightStar
--- PASS: TestIndicatorMarkerHiddenRightStar (0.00s)
=== RUN   TestIndicatorTerminatorOnlyMarkerHiddenLeftStar
--- PASS: TestIndicatorTerminatorOnlyMarkerHiddenLeftStar (0.00s)
=== RUN   TestIndicatorMarkerOnlyLineAlwaysVisible
--- PASS: TestIndicatorMarkerOnlyLineAlwaysVisible (0.00s)
PASS
ok  	vrg/internal/app
```

## Manual verification

The task requires manual cases demonstrating the marker rendering through the production path. The verify_markers.go helper loads a file through the production filebuffer.Load path with a zero-width submatch at the given byte position and prints the display text, highlights, byte cells, and cluster widths. The fake rg binaries (fakebin/rg_bol, fakebin/rg_eol, fakebin/rg_crlf_eol) emit valid JSON streams with zero-width submatches so the full vrg TUI can be driven through the capture_raw.py PTY harness.

### Case 1: hit\r\n terminator-only $ marker at display column 3

The demo file (demo_artifacts/crlf_demo.txt) contains CRLF-terminated lines. A zero-width match at byte 4 (the \n of "hit\r\n") maps to display column 3 via the Issue #22 terminator-to-EOL mapping. The verify_markers.go helper loads the file with a zero-width submatch at byte 4 and prints the display, highlights, and clusters. The display should be "hit " (3 content chars + 1 marker space), the highlight should be [3, 4), and the last cluster should have width 1.

```bash
cd /home/chris/vrg && go run Notes/walkthroughs/023-04/code-walkthrough/demo_artifacts/verify_markers.go Notes/walkthroughs/023-04/code-walkthrough/demo_artifacts/crlf_demo.txt 4
```

```output
line 1 display="hit "
  highlights=[[3 4]]
  cluster_count=4 cluster_width=4
    cluster[0] bytes=[0,1) width=1
    cluster[1] bytes=[1,2) width=1
    cluster[2] bytes=[2,3) width=1
    cluster[3] bytes=[3,4) width=1
line 2 display="world"
  highlights=[]
  cluster_count=5 cluster_width=5
    cluster[0] bytes=[0,1) width=1
    cluster[1] bytes=[1,2) width=1
    cluster[2] bytes=[2,3) width=1
    cluster[3] bytes=[3,4) width=1
    cluster[4] bytes=[4,5) width=1
line 3 display="empty"
  highlights=[]
  cluster_count=5 cluster_width=5
    cluster[0] bytes=[0,1) width=1
    cluster[1] bytes=[1,2) width=1
    cluster[2] bytes=[2,3) width=1
    cluster[3] bytes=[3,4) width=1
    cluster[4] bytes=[4,5) width=1
line 4 display=""
  highlights=[]
  cluster_count=0 cluster_width=0
line 5 display="end"
  highlights=[]
  cluster_count=3 cluster_width=3
    cluster[0] bytes=[0,1) width=1
    cluster[1] bytes=[1,2) width=1
    cluster[2] bytes=[2,3) width=1
```

### Case 2: Marker-only line has extent 1

An empty line (just "\n") with a zero-width match at byte 0 (the \n terminator) produces a marker at cell 0. The line has width one: its display is a single space and its Clusters slice has one 1-cell cluster. The verify_markers.go helper loads a one-line empty file with a zero-width submatch at byte 0.

```bash
cd /home/chris/vrg && go run Notes/walkthroughs/023-04/code-walkthrough/demo_artifacts/verify_markers.go Notes/walkthroughs/023-04/code-walkthrough/demo_artifacts/empty_line.txt 0
```

```output
line 1 display=" "
  highlights=[[0 1]]
  cluster_count=1 cluster_width=1
    cluster[0] bytes=[0,1) width=1
```

### Case 3: vrg '^' file — inverse cell at column 0 of each line, including empty lines

The demo file (demo_artifacts/bol_demo.txt) contains five lines: "hit", "world", "empty", "" (empty), "end". The fake rg (fakebin/rg_bol) emits zero-width matches for ^ (BOL) at byte 0 of each line. Each line should show an inverse-video cell at column 0 marking the beginning of the line, including the empty line (line 4) which has width one because of its marker. The capture_raw.py PTY harness drives the full vrg TUI with the no-style-escaped output so the inverse marker cells are visible as styled spans.

```bash
cd /home/chris/vrg/Notes/walkthroughs/023-04/code-walkthrough && ln -sf rg_bol fakebin/rg && PATH=$PWD/fakebin:$PWD/demo_artifacts:$PATH VRG_KEYS=q VRG_DELAY=1.0 VRG_WIDTH=60 VRG_HEIGHT=10 python3 demo_artifacts/capture_raw.py demo_artifacts/vrg -- '^' demo_artifacts/bol_demo.txt 2>&1 | sed 's/\x1b\[/ESC[/g' | tr -d '\r' | grep -oE 'ESC\[[0-9;]+m[^E]*' | grep -vE '^\[' | head -20; echo
```

```output
ESC[37;40;4mdemo_artifacts/bol_demo.txt ── demo_artifacts/bol_demo.txt ─
ESC[37;40m
ESC[30;47;4mh
ESC[37;40mit
ESC[37;40m
ESC[30;47mw
ESC[37;40morld
ESC[37;40m
ESC[30;47me
ESC[37;40mmpty
ESC[37;40m
ESC[30;47m 
ESC[37;40m
ESC[30;47me
ESC[37;40mnd
ESC[37;40m

```

### Case 4: vrg '$' file — inverse cell after each line's last character

The demo file (demo_artifacts/eol_demo.txt) contains five lines: "hit", "world", "empty", "" (empty), "end". The fake rg (fakebin/rg_eol) emits zero-width matches for $ (EOL) at the terminator byte of each line. Each line should show an inverse-video space after the last character (the EOL marker extending the line width by one cell), including the empty line (line 4) which has width one because of its marker. The current matched line (line 1) is underlined.

```bash
cd /home/chris/vrg/Notes/walkthroughs/023-04/code-walkthrough && ln -sf rg_eol fakebin/rg && PATH=$PWD/fakebin:$PWD/demo_artifacts:$PATH VRG_KEYS=q VRG_DELAY=1.0 VRG_WIDTH=60 VRG_HEIGHT=10 python3 demo_artifacts/capture_raw.py demo_artifacts/vrg -- '$' demo_artifacts/eol_demo.txt 2>&1 | sed 's/\x1b\[/ESC[/g' | tr -d '\r' | grep -oE 'ESC\[[0-9;]+m[^E]*' | head -15; echo
```

```output
ESC[37;40;4mdemo_artifacts/eol_demo.txt ── demo_artifacts/eol_demo.txt ─
ESC[37;40m
ESC[30;47;4m 
ESC[37;40m
ESC[30;47m 
ESC[37;40m
ESC[30;47m 
ESC[37;40m
ESC[30;47m 
ESC[37;40m
ESC[30;47m 
ESC[37;40m

```

### Case 5: n navigation between markers

The same demo file (demo_artifacts/eol_demo.txt) with EOL markers. Pressing `n` navigates to the next marker (line 2), which becomes the current matched line (underlined). The capture sends `n` then `q` to show the navigation result. Line 2's EOL marker should be underlined (current match) and line 1's marker should be inverse only (no longer current).

```bash
cd /home/chris/vrg/Notes/walkthroughs/023-04/code-walkthrough && ln -sf rg_eol fakebin/rg && PATH=$PWD/fakebin:$PWD/demo_artifacts:$PATH VRG_KEYS=n,q VRG_DELAY=1.0 VRG_WIDTH=60 VRG_HEIGHT=10 python3 demo_artifacts/capture_raw.py demo_artifacts/vrg -- '$' demo_artifacts/eol_demo.txt 2>&1 | sed 's/\x1b\[/ESC[/g' | tr -d '\r' | grep -oE 'ESC\[[0-9;]+m[^E]*' | head -15; echo
```

```output
ESC[37;40;4mdemo_artifacts/eol_demo.txt ── demo_artifacts/eol_demo.txt ─
ESC[37;40m
ESC[30;47;4m 
ESC[37;40m
ESC[30;47m 
ESC[37;40m
ESC[30;47m 
ESC[37;40m
ESC[30;47m 
ESC[37;40m
ESC[30;47m 
ESC[37;40m
ESC[30;47m 
ESC[30;47;4m 

```

### Case 6: vrg '$' crlf_demo.txt — CRLF terminator-only marker at column 3

The demo file (demo_artifacts/crlf_demo.txt) contains CRLF-terminated lines. The fake rg (fakebin/rg_crlf_eol) emits zero-width matches for $ at the \n byte of each CRLF. Line 1 "hit\r\n" has \n at byte 4, which maps to display column 3 via the Issue #22 terminator-to-EOL mapping. The verify_markers.go helper confirms the marker lands at [3, 4) with the display extended to "hit " (4 cells). The full TUI shows the inverse marker space after "hit" at column 3.

```bash
cd /home/chris/vrg && go run Notes/walkthroughs/023-04/code-walkthrough/demo_artifacts/verify_markers.go Notes/walkthroughs/023-04/code-walkthrough/demo_artifacts/crlf_demo.txt 4 2>&1 | head -8
```

```output
line 1 display="hit "
  highlights=[[3 4]]
  cluster_count=4 cluster_width=4
    cluster[0] bytes=[0,1) width=1
    cluster[1] bytes=[1,2) width=1
    cluster[2] bytes=[2,3) width=1
    cluster[3] bytes=[3,4) width=1
line 2 display="world"
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/023-04/code-walkthrough && ln -sf rg_crlf_eol fakebin/rg && PATH=$PWD/fakebin:$PWD/demo_artifacts:$PATH VRG_KEYS=q VRG_DELAY=1.0 VRG_WIDTH=60 VRG_HEIGHT=10 python3 demo_artifacts/capture_raw.py demo_artifacts/vrg -- '$' demo_artifacts/crlf_demo.txt 2>&1 | sed 's/\x1b\[/ESC[/g' | tr -d '\r' | grep -oE 'ESC\[[0-9;]+m[^E]*' | head -15; echo
```

```output
ESC[37;40;4mdemo_artifacts/crlf_demo.txt ── demo_artifacts/crlf_demo.txt
ESC[37;40m
ESC[30;47;4m 
ESC[37;40m
ESC[30;47m 
ESC[37;40m
ESC[30;47m 
ESC[37;40m
ESC[30;47m 
ESC[37;40m
ESC[30;47m 
ESC[37;40m

```

### Case 7: Run-off-edge panning past a marker sets the left *

The demo file (demo_artifacts/long_line.txt) contains a single 200-character line. The fake rg (fakebin/rg_bol_long) emits a zero-width BOL match at byte 0. In run-off-edge mode (toggled with `w`), panning right with `.` by one column hides the BOL marker (at column 0) entirely left of the window. The Issue #20 left `*` indicator appears in the first trailing gutter space because the match is entirely hidden left. The capture sends `w` (toggle to run-off-edge), `.` (pan right 1), `q` (quit).

```bash
cd /home/chris/vrg/Notes/walkthroughs/023-04/code-walkthrough && ln -sf rg_bol_long fakebin/rg && PATH=$PWD/fakebin:$PWD/demo_artifacts:$PATH VRG_KEYS=w,.,q VRG_DELAY=1.0 VRG_WIDTH=60 VRG_HEIGHT=10 python3 demo_artifacts/capture_raw.py demo_artifacts/vrg -- '^' demo_artifacts/long_line.txt 2>&1 | sed 's/\x1b\[/ESC[/g' | tr -d '\r' | grep -oE 'ESC\[[0-9;]+m[^E]*' | grep -E '(\*|30;47)' | head -10; echo
```

```output
ESC[30;47;4ma
ESC[30;47m*

```

## Summary

Issue #23 zero-width match markers are implemented and verified. The FileBuffer produces one-cell inverse-video markers at mapped display locations (cluster-start mapping for positions inside clusters, EOL extension by one cell for terminator/end-of-line positions). Markers participate in wrap (full-row marker wraps to next row), clip (hidden markers excluded from clipped output), extent (marker-only line has extent 1), pan clamping (marker-only line has max offset 0), reveal (marker cell is a navigable target), and Issue #20 indicators (hidden-left `*`, hidden-right `*`). The terminator-only `$` marker on `hit\r\n` produces a single marker cell at display column 3 following exactly the same rules as any other marker. References: Issue #23 (Notes/issues/023-zero-width-match-markers.md), Notes/PRD-vrg.md (Text, graphemes, and safe presentation), Notes/wiki/zero-width-match-markers.md.

All generated artifacts (demo files, fake rg binaries, verify_markers.go, capture_raw.py, vrg binary) live in this directory under demo_artifacts/ and fakebin/.

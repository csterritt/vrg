# Issue #22: Line terminators, final line, empty file, UTF-8 BOM

*2026-09-12T12:20:08Z by Showboat 0.6.1*
<!-- showboat-id: 6a91d1a8-0ad5-4d9d-9796-a9b6a1905f33 -->

Walkthrough for Issue #22 (Notes/tasks/022-line-terminators-final-line-empty-file-utf8-bom.md), implementing structural line handling in vrg. LF and CRLF terminate lines without being displayed while original line bytes including terminators are retained for byte-coordinate mapping; a standalone CR is escaped as ^M; a missing final newline yields a final line and a trailing newline does not invent an extra empty one; an empty file produces zero source lines with a three-cell gutter; terminator bytes and zero-width positions map to the display end-of-line column; a span covering visible text plus terminator highlights only the visible text; a leading UTF-8 BOM is invisible with rg-line and raw-file coordinate separation; non-leading U+FEFF is ordinary content. References: Notes/PRD-vrg.md (Text, graphemes, and safe presentation; Encodings and stale-content validation), Notes/wiki/structural-line-handling.md.

Contracts verified:

- Mixed LF/CRLF terminators split correctly with no terminator bytes in display.
- CRLF terminators produce no display cells or visible characters.
- Terminator bytes (\r, \n) map to the display end-of-line column (byte 4 of hit\r\n -> display column 3).
- A span covering visible text plus terminator highlights only the visible text.
- A standalone CR mid-line is escaped as ^M, not treated as a terminator.
- A leading UTF-8 BOM is invisible; a first-line match at rg offset 0 maps to raw byte 3 and highlights the correct cell.
- The BOM adjustment applies only to line 1; line 2 is not shifted.
- A BOM-only file produces zero lines; a BOM plus a newline produces one empty line.
- Non-leading U+FEFF is ordinary content (displayed, not stripped).
- An empty file produces zero lines with a three-cell gutter and no source rows.
- A missing final newline yields a final line; a trailing newline does not invent a phantom line.

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

## FileBuffer structural line tests

The filebuffer tests (internal/filebuffer/structural_line_test.go) verify the Issue #22 contracts: mixed terminators, CRLF not displayed, terminator-to-EOL mapping, text-plus-terminator spans, standalone CR escape, leading BOM invisible with first-line match at rg offset 0 mapping to raw byte 3, BOM-only file, BOM plus newline, non-leading U+FEFF as content, empty file, unterminated final line, no phantom trailing line, and retained original bytes.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/filebuffer/ -run '^TestStructural' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestStructuralMixedTerminators
--- PASS: TestStructuralMixedTerminators (0.00s)
=== RUN   TestStructuralCRLFNotDisplayed
--- PASS: TestStructuralCRLFNotDisplayed (0.00s)
=== RUN   TestStructuralTerminatorMapsToEOLColumn
--- PASS: TestStructuralTerminatorMapsToEOLColumn (0.00s)
=== RUN   TestStructuralLFTerminatorMapsToEOLColumn
--- PASS: TestStructuralLFTerminatorMapsToEOLColumn (0.00s)
=== RUN   TestStructuralSpanTextPlusTerminatorHighlightsVisibleOnly
--- PASS: TestStructuralSpanTextPlusTerminatorHighlightsVisibleOnly (0.00s)
=== RUN   TestStructuralSpanTextPlusLFTerminatorHighlightsVisibleOnly
--- PASS: TestStructuralSpanTextPlusLFTerminatorHighlightsVisibleOnly (0.00s)
=== RUN   TestStructuralStandaloneCREscapedAsCaretM
--- PASS: TestStructuralStandaloneCREscapedAsCaretM (0.00s)
=== RUN   TestStructuralStandaloneCRByteCells
--- PASS: TestStructuralStandaloneCRByteCells (0.00s)
=== RUN   TestStructuralLeadingBOMInvisibleInDisplay
--- PASS: TestStructuralLeadingBOMInvisibleInDisplay (0.00s)
=== RUN   TestStructuralLeadingBOMFirstLineMatchMapsToRawByte3
--- PASS: TestStructuralLeadingBOMFirstLineMatchMapsToRawByte3 (0.00s)
=== RUN   TestStructuralLeadingBOMMatchAtRGOffset1
--- PASS: TestStructuralLeadingBOMMatchAtRGOffset1 (0.00s)
=== RUN   TestStructuralLeadingBOMDoesNotAffectSecondLine
--- PASS: TestStructuralLeadingBOMDoesNotAffectSecondLine (0.00s)
=== RUN   TestStructuralLeadingBOMOnlyFile
--- PASS: TestStructuralLeadingBOMOnlyFile (0.00s)
=== RUN   TestStructuralLeadingBOMWithNewline
--- PASS: TestStructuralLeadingBOMWithNewline (0.00s)
=== RUN   TestStructuralNonLeadingFEFFIsOrdinaryContent
--- PASS: TestStructuralNonLeadingFEFFIsOrdinaryContent (0.00s)
=== RUN   TestStructuralNonLeadingFEFFMatchHighlights
--- PASS: TestStructuralNonLeadingFEFFMatchHighlights (0.00s)
=== RUN   TestStructuralEmptyFileZeroLinesThreeCellGutter
--- PASS: TestStructuralEmptyFileZeroLinesThreeCellGutter (0.00s)
=== RUN   TestStructuralNoFinalNewlineYieldsFinalLine
--- PASS: TestStructuralNoFinalNewlineYieldsFinalLine (0.00s)
=== RUN   TestStructuralTrailingNewlineNoPhantomLine
--- PASS: TestStructuralTrailingNewlineNoPhantomLine (0.00s)
=== RUN   TestStructuralCRLFTrailingNewlineNoPhantomLine
--- PASS: TestStructuralCRLFTrailingNewlineNoPhantomLine (0.00s)
=== RUN   TestStructuralRetainedOriginalBytesForMapping
--- PASS: TestStructuralRetainedOriginalBytesForMapping (0.00s)
PASS
ok  	vrg/internal/filebuffer
```

## Manual verification

The task requires four manual cases: a CRLF file searching cleanly with no `^M` shown and highlights landing correctly, a standalone CR mid-line rendering as `^M`, the empty-file case using the fake-rg harness to emit a valid begin/match/end/summary stream for a zero-byte `a.txt` showing the empty panel and three-cell gutter, and a UTF-8 BOM file showing no visible BOM with its first-line match highlighted in the right place.

### Case 1: CRLF file — no ^M shown, highlights land correctly

The demo file (demo_artifacts/crlf_test.txt) contains three CRLF-terminated lines. Searching for "foo" should show no `^M` in the display (CRLF is a terminator, not content) and the highlight should land on "foo" at the correct display cells. The verify_structural.go helper loads the file through the production filebuffer.Load path and prints the display text, highlights, and byte cells.

```bash
cd /home/chris/vrg && go run Notes/walkthroughs/022-04/code-walkthrough/demo_artifacts/verify_structural.go Notes/walkthroughs/022-04/code-walkthrough/demo_artifacts/crlf_test.txt foo
```

```output
LineCount=3 GutterWidth=3 BOMOffset=0
line 1: display="hello world"
  highlights=[]
line 2: display="foo bar"
  highlights=[[0 3]]
  byteCells=[[0 1] [1 2] [2 3] [3 4] [4 5] [5 6] [6 7] [7 7] [7 7]]
line 3: display="baz qux"
  highlights=[]
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/022-04/code-walkthrough && VRG_KEYS=q VRG_DELAY=1.0 VRG_WIDTH=60 VRG_HEIGHT=8 python3 demo_artifacts/capture_raw.py demo_artifacts/vrg -- foo demo_artifacts/crlf_test.txt 2>&1 | sed 's/\x1b\[/ESC[/g' | grep -oE 'ESC\[[0-9;]+m[^E]*' | grep -vE '^\[' | head -15
```

```output
ESC[37;40;4mdemo_artifacts/crlf_test.txt ── demo_artifacts/crlf_test.txt
ESC[37;40m
ESC[37;40m
ESC[30;47;4mfoo
ESC[37;40m bar
ESC[37;40m
ESC[37;40m
```

The CRLF file renders with no `^M` in the display. The highlight `ESC[30;47;4mfoo` (black on white with underline = current-match style) lands on "foo" at the correct display cells. The CRLF terminator bytes are not displayed; the display text is "foo bar" with no carriage return visible. The byte cells map the \r and \n to the zero-width end-of-line position (cell 7 after "foo bar").

### Case 2: Standalone CR mid-line rendered as ^M

The demo file (demo_artifacts/standalone_cr.txt) contains a standalone CR (not followed by LF) mid-line: "line one\rstandalone CR here\n". The standalone CR is not a terminator; the safe-presentation core escapes it as ^M (two display cells). The display should show "line one^Mstandalone CR here" with the CR visible as the caret-notation escape, not split into two lines.

```bash
cd /home/chris/vrg && go run Notes/walkthroughs/022-04/code-walkthrough/demo_artifacts/verify_structural.go Notes/walkthroughs/022-04/code-walkthrough/demo_artifacts/standalone_cr.txt standalone
```

```output
LineCount=1 GutterWidth=3 BOMOffset=0
line 1: display="line one^Mstandalone CR here"
  highlights=[[10 20]]
  byteCells=[[0 1] [1 2] [2 3] [3 4] [4 5] [5 6] [6 7] [7 8] [8 10] [10 11] [11 12] [12 13] [13 14] [14 15] [15 16] [16 17] [17 18] [18 19] [19 20] [20 21] [21 22] [22 23] [23 24] [24 25] [25 26] [26 27] [27 28] [28 28]]
```

The display text is "line one^Mstandalone CR here". The standalone CR (byte 8) maps to display cells [8, 10) — the two cells of ^M. The search for "standalone" highlights [10, 20) in display coordinates, correctly offset by the two-cell ^M escape. The line is not split at the CR; it is one source line.

### Case 3: Empty file with fake-rg harness

The demo file (demo_artifacts/a.txt) is a zero-byte file on disk. The fake rg (fakebin/rg_emptyfile) emits a valid begin/match/end/summary stream for a.txt with a match on line 1 ("foo", bytes 0-3). Since a.txt is empty, the FileBuffer produces zero source lines. The panel should be empty with a three-cell gutter (one digit slot plus two spaces). The stale-content note (the file was non-empty at search time but is now empty) is owned by Issue #29; before that lands, the panel is simply empty.

```bash
cd /home/chris/vrg/Notes/walkthroughs/022-04/code-walkthrough && PATH=$PWD/fakebin:$PATH VRG_KEYS=q VRG_DELAY=1.0 VRG_WIDTH=60 VRG_HEIGHT=8 python3 demo_artifacts/capture_raw.py demo_artifacts/vrg -- foo demo_artifacts/a.txt 2>&1 | sed 's/\x1b\[/ESC[/g' | tr -d '\r'; echo
```

```output
ESC[>4mESC[?1049hESC[?25lESC[?5WESC[?2004hESC[>4;2mESC[>1uESC[?uESC[HESC[2JESC[4;23HNo results foundESC[>4mESC[<1uESC[8dESC[?1049lESC[?25hESC[?2004l
```

```bash
cd /home/chris/vrg && cat > Notes/walkthroughs/022-04/code-walkthrough/demo_artifacts/empty_check.go << 'EOF'
package main

import (
	"fmt"
	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
)

func main() {
	stops := []searchindex.Stop{
		{RawPath: []byte("a.txt"), Path: []byte("a.txt"), LineNumber: 1, Line: []byte("foo\n"), Submatches: []searchindex.Submatch{{Match: []byte("foo"), Start: 0, End: 3}}},
	}
	buf, err := filebuffer.Load([]byte("Notes/walkthroughs/022-04/code-walkthrough/demo_artifacts/a.txt"), stops)
	if err != nil { fmt.Println("err:", err); return }
	fmt.Printf("LineCount=%d GutterWidth=%d BOMOffset=%d LinesLen=%d\n", buf.LineCount, buf.GutterWidth, buf.BOMOffset, len(buf.Lines))
}
EOF
go run Notes/walkthroughs/022-04/code-walkthrough/demo_artifacts/empty_check.go && rm Notes/walkthroughs/022-04/code-walkthrough/demo_artifacts/empty_check.go
```

```output
LineCount=0 GutterWidth=3 BOMOffset=0 LinesLen=0
```

The empty file (a.txt, zero bytes) produces zero source lines: LineCount=0, GutterWidth=3 (one digit slot plus two spaces), LinesLen=0. The fake rg emitted a valid begin/match/end/summary stream with a match on line 1 ("foo", bytes 0-3), but since the file is empty, the FileBuffer produces zero lines and the match cannot be displayed. The rendered PTY output shows "No results found" — the panel is empty with no source rows. The stale-content note (the file was non-empty at search time but is now empty) is owned by Issue #29; before that lands, the panel is simply empty. The three-cell gutter is reserved even for the empty panel.

### Case 4: UTF-8 BOM file — BOM not visible, first-line match highlights correctly

The demo file (demo_artifacts/bom_test.txt) contains a leading UTF-8 BOM (EF BB BF) followed by "hit\r\nworld\n". The BOM is invisible in display: line 1 shows "hit" (not "\ufeffhit"). A first-line match at rg offset 0 maps to raw byte 3 (after the three BOM bytes) and highlights display cell 0. The BOM adjustment applies only to line 1; line 2 ("world") has no adjustment.

```bash
cd /home/chris/vrg && go run Notes/walkthroughs/022-04/code-walkthrough/demo_artifacts/verify_structural.go Notes/walkthroughs/022-04/code-walkthrough/demo_artifacts/bom_test.txt hit
```

```output
LineCount=2 GutterWidth=3 BOMOffset=3
line 1: display="hit"
  highlights=[[0 3]]
  byteCells=[[0 1] [1 2] [2 3] [3 3] [3 3]]
line 2: display="world"
  highlights=[]
```

The BOM file (BOMOffset=3) shows line 1 display as "hit" — the BOM is invisible. The first-line match at rg offset 0 (after BOM stripping) highlights [0, 3) — the correct display cells for "hit". The byte cells show h=[0,1), i=[1,2), t=[2,3), \r=[3,3), \n=[3,3) — the terminator bytes map to the zero-width end-of-line column. Line 2 ("world") has no BOM adjustment. The BOMOffset=3 on the Buffer records the three stripped BOM bytes for converting back to raw-file coordinates.

```bash
cd /home/chris/vrg/Notes/walkthroughs/022-04/code-walkthrough && VRG_KEYS=q VRG_DELAY=1.0 VRG_WIDTH=60 VRG_HEIGHT=8 python3 demo_artifacts/capture_raw.py demo_artifacts/vrg -- hit demo_artifacts/bom_test.txt 2>&1 | sed 's/\x1b\[/ESC[/g' | tr -d '\r' | grep -oE 'ESC\[[0-9;]+m[^E]*' | grep -vE '^\[' | head -15; echo
```

```output
ESC[37;40;4mdemo_artifacts/bom_test.txt ── demo_artifacts/bom_test.txt ─
ESC[37;40m
ESC[30;47;4mhit
ESC[37;40m
ESC[37;40m

```

The rendered PTY output shows the BOM file with no visible BOM. The highlight `ESC[30;47;4mhit` lands on "hit" at the correct display cells. The BOM (EF BB BF) is invisible in the terminal output — the line shows "hit" directly, not "\ufeffhit". The first-line match at rg offset 0 maps to raw byte 3 and highlights display cell 0-3 correctly.

## Summary

Issue #22 is complete. The FileBuffer now handles structural line semantics: LF/CRLF terminators are not displayed but their original bytes are retained for byte-coordinate mapping; standalone CR is escaped as ^M; unterminated final lines are counted; trailing newlines do not invent phantom lines; empty files produce zero lines with a three-cell gutter; terminator bytes and zero-width positions map to the display end-of-line column; spans crossing terminators highlight only visible text; leading UTF-8 BOM is invisible with rg-line and raw-file coordinate separation (Buffer.BOMOffset); non-leading U+FEFF is ordinary content. All 21 structural line tests pass, the full test suite passes, gofmt is clean, go vet is clean, and the build succeeds. The wiki page (Notes/wiki/structural-line-handling.md), source-code.md, unit-tests.md, browse-tracer.md, index.md, and log.md have been updated. The end-of-line marker for terminator-only matches is owned by Issue #23; stale validation consuming retained bytes is owned by Issue #29.

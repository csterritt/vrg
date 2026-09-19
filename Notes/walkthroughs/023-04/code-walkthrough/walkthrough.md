# Issue #23: zero-width match markers

*2026-09-17T21:40:49Z by Showboat 0.6.1*
<!-- showboat-id: 16bcd5fb-f368-4772-a1a7-6b4c82ceb225 -->

Issue #23 lands the zero-width match marker: a zero-width submatch — `^`, `$`, `a*` over non-a text — paints as exactly one inverse-video space at its mapped display cell, underlined on the current matched line, and follows every ordinary match rule for wrapping, clipping, horizontal reveal and extent, pan clamping, destination reveal, and the hidden-content indicators. filebuffer.Line gains MarkerAt(cell) — an empty Highlights span records a marker position — and Extent(), the effective display width: len(Cells) plus one when a marker sits at the end-of-line position, so an empty matched line has extent 1. A marker inside text marks the existing cell it lands on without shifting following text; a position inside a grapheme cluster maps to the cluster start so no wide glyph splits; and MaxStart counts the EOL marker as a one-cell paintable candidate — a marker-only line has extent 1 and MaxOff 0 under Issue #18's paintable-boundary rule. Viewport flat spans run to Extent() (Row.End can reach len(Cells)+1) and wrapLine appends the marker as an unbreakable one-cell unit — joining a partial row, occupying its own continuation row after a full one, and giving an empty matched line one [0,1) row. contentText ORs MarkerAt into the per-cell highlight test and paints a cell past len(Cells) as one inverse space clipped by the same col+1 > textW bound. The terminator-only $ on hit\r\n — rg reports (4,4), CellsCovering maps it to display column 3 — is the canonical ordinary marker with no special cases. See Notes/issues/023-zero-width-match-markers.md, Notes/tasks/023-zero-width-match-markers.md, and the PRD sections 'Text, graphemes, and safe presentation' and 'Layout and indicators' in Notes/PRD-vrg.md. All artifacts live in this directory.

## Gates — module integrity, build, vet

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

## Marker tests — filebuffer positions/extent and viewport row model

internal/filebuffer/filebuffer_test.go pins the marker contracts. TestZeroWidthMarkerPositions is the position table: beginning of line, inside text, inside a wide cluster's bytes and inside a combining cluster (both mapping to the cluster start — no split glyph), end of line, the LF terminator byte, an empty line and its terminator, the $ on hit\r\n plus the \r-only and whole-\r\n spans all landing on display column 3, and a position past the line — each asserting MarkerAt and the recorded empty [c,c) span. TestEOLMarkerExtendsEffectiveWidth proves Extent() adds one only for an end-of-line marker — an empty matched line's extent is 1, BOL and mid-text markers add nothing, ab文 plus its marker is 5. TestMarkerFeedsPaintableBoundary proves MaxStart counts the EOL marker as a one-cell candidate (3 even at width 1, so the marker can paint alone) and a marker-only line reports 0.

internal/viewport/marker_test.go covers the marker through the row model: TestEOLMarkerWrapRows — the marker joins a wrapped row with a cell to spare and occupies its own continuation row [3,4) after an exactly full row; TestEOLMarkerFlatRows — run-off-edge rows span the effective width, hit + $ is [0,4) and the empty matched line's marker is its single cell [0,1); TestMarkerExtentFeedsMaxOff — the marker's start feeds MaxOff (3 for hit + $, where the marker paints alone) and a marker-only line has extent 1 with MaxOff 0; TestMarkerHiddenDrivesIndicators — LF and CRLF markers alike: entirely hidden left upgrades the gutter to *, entirely hidden right earns the reserved *; TestMarkerIsRevealTarget and TestRevealLandsOnMarkerRow — the marker cell is a navigable reveal target, its own wrapped row after a full text row, landing visible by the one-third placement clamped to EOF.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestZeroWidthMarkerPositions|TestEOLMarkerExtendsEffectiveWidth|TestMarkerFeedsPaintableBoundary|TestEOLMarkerWrapRows|TestEOLMarkerFlatRows|TestMarkerExtentFeedsMaxOff|TestMarkerHiddenDrivesIndicators|TestMarkerIsRevealTarget|TestRevealLandsOnMarkerRow' ./internal/filebuffer ./internal/viewport 2>&1 | grep -E '^(--- |ok|FAIL|    ---)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestZeroWidthMarkerPositions
    --- PASS: TestZeroWidthMarkerPositions/beginning_of_line
    --- PASS: TestZeroWidthMarkerPositions/inside_text
    --- PASS: TestZeroWidthMarkerPositions/inside_a_wide_cluster
    --- PASS: TestZeroWidthMarkerPositions/wide_cluster's_last_byte
    --- PASS: TestZeroWidthMarkerPositions/inside_a_combining_cluster
    --- PASS: TestZeroWidthMarkerPositions/end_of_line
    --- PASS: TestZeroWidthMarkerPositions/the_LF_terminator_byte
    --- PASS: TestZeroWidthMarkerPositions/empty_line
    --- PASS: TestZeroWidthMarkerPositions/empty_line's_terminator
    --- PASS: TestZeroWidthMarkerPositions/$_on_CRLF
    --- PASS: TestZeroWidthMarkerPositions/CR_byte_only
    --- PASS: TestZeroWidthMarkerPositions/whole_CRLF
    --- PASS: TestZeroWidthMarkerPositions/a_position_past_the_line
--- PASS: TestEOLMarkerExtendsEffectiveWidth
    --- PASS: TestEOLMarkerExtendsEffectiveWidth/no_marker
    --- PASS: TestEOLMarkerExtendsEffectiveWidth/EOL_marker
    --- PASS: TestEOLMarkerExtendsEffectiveWidth/empty_line's_marker_is_width_one
    --- PASS: TestEOLMarkerExtendsEffectiveWidth/BOL_marker_adds_no_cell
    --- PASS: TestEOLMarkerExtendsEffectiveWidth/mid-text_marker_adds_no_cell
    --- PASS: TestEOLMarkerExtendsEffectiveWidth/wide_line_plus_marker
    --- PASS: TestEOLMarkerExtendsEffectiveWidth/BOL_and_EOL_markers_extend_once
--- PASS: TestMarkerFeedsPaintableBoundary
ok  	vrg/internal/filebuffer
--- PASS: TestEOLMarkerWrapRows
--- PASS: TestEOLMarkerFlatRows
--- PASS: TestMarkerExtentFeedsMaxOff
--- PASS: TestMarkerHiddenDrivesIndicators
--- PASS: TestMarkerIsRevealTarget
--- PASS: TestRevealLandsOnMarkerRow
ok  	vrg/internal/viewport
```

## Full module regression

The marker threads through every display system — painting, wrapping, panning, reveal, clipping, and the indicators all consume it — so the whole suite runs.

```bash
cd /home/chris/vrg && go test -count=1 ./internal/filebuffer ./internal/viewport ./internal/app ./internal/safepresentation ./internal/searchindex ./internal/theme ./internal/cli ./cmd/vrg 2>&1 | sed -E 's/\t[0-9.]+s$//'
```

```output
ok  	vrg/internal/filebuffer
ok  	vrg/internal/viewport
ok  	vrg/internal/app
ok  	vrg/internal/safepresentation
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/cli
ok  	vrg/cmd/vrg
```

## Manual check — the real binary on a pty

pty_markers.py runs the built vrg on a real 100x24 pty over per-case fixture directories, replaying the byte stream through a small terminal emulator. Session A searches `^` in a file holding 'hit', 'miss', and an empty line: the marker at column 0 marks the existing cell — 'h' styled inverse+underline on the current line, 'm' inverse on line 2, text unshifted — while the EMPTY line 3 paints a styled space, its extent-one marker cell; `n` then moves the underline to line 2's marker and on to the empty line's marker — every marker a navigable reveal target. Session B searches `$` in a CRLF file: the terminator-only match on hit\r\n (rg position (4,4)) paints an ordinary styled space at display column 3 after 'hit' — no ^M anywhere — and 'miss' earns its own after its last char; `n` moves the underline to the next marker. Session C searches `^`, toggles `w` for run-off-edge, and pans `>`: the offset clamps to the paintable boundary 3 with every column-0 marker now entirely hidden left, so all three lines show the '\*' gutter mark exactly like an entirely hidden match.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/023-04/code-walkthrough/vrg ./cmd/vrg && python3 Notes/walkthroughs/023-04/code-walkthrough/pty_markers.py
```

```output
setup    : '^' — line 1 'h' styled+underlined, line 2 'm' styled, empty line 3 paints a styled space — '1  hit    ' / '2  miss   '
n        : underline moved to line 2's marker — 'm' now current
n        : underline on the empty line's marker cell — navigable target
q        : exit 0
setup    : '$' on hit\r\n — styled space at column 3 after 'hit', another after 'miss'; no ^M — '1  hit   ' / '2  miss  '
n        : underline moved to the next end-of-line marker
q        : exit 0
w,>      : '>' past the markers — every gutter shows '*' for the entirely hidden marker — '1*      ' / '2* s    ' / '3*      '
q        : exit 0
OK
```

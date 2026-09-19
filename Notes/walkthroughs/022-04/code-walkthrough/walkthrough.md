# Issue #22: line terminators, final line, empty file, UTF-8 BOM

*2026-09-17T21:24:18Z by Showboat 0.6.1*
<!-- showboat-id: 7969e5c9-67cf-4693-9c38-abfdc4a6f552 -->

Issue #22 lands FileBuffer's structural line handling with the three coordinate views kept separate. makeLine now splits each raw line into leading-BOM, content, and terminator regions: LF and CRLF terminate lines without displaying while Line.Raw retains the original bytes; a \r with no following \n stays content and escapes ^M; a missing final newline still yields the last line, a trailing newline invents none, and an empty file has zero source lines with the one-digit-slot three-cell gutter. Line records searchOff — three for a leading UTF-8 BOM on line 1, zero elsewhere — and contentEnd: Line.SearchBytes() is the rg-line view ripgrep's reported lines and submatch offsets index, cell byte offsets stay raw-file coordinates, and the shadowing Line.CellsCovering translates rg ranges onto cells, landing removed terminator bytes, positions past the content end, and zero-width positions on the display end-of-line position (byte 4 of hit\r\n maps to display column 3) while a text-plus-terminator span highlights only the visible cells. A leading \xef\xbb\xbf is invisible with rg offset 0 mapping to raw byte 3; U+FEFF anywhere else is ordinary content escaped \ufeff. The end-of-line marker these positions locate is Issue #23's; the stale validation consuming the retained bytes is Issue #29's. See Notes/issues/022-line-terminators-final-line-empty-file-utf8-bom.md, Notes/tasks/022-line-terminators-final-line-empty-file-utf8-bom.md, and the PRD sections 'Text, graphemes, and safe presentation' (first two bullets) and 'Encodings and stale-content validation' (UTF-8 BOM bullet) in Notes/PRD-vrg.md. All artifacts live in this directory.

## Gates — module integrity, build, vet

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

## Structural line tests — terminators, EOL mapping, and the BOM

internal/filebuffer/filebuffer_test.go pins the Issue #22 contracts. TestTerminatorsRemovedFromDisplayRetainedInRaw proves LF, CRLF, and mixed terminators produce no display cells while Line.Raw retains the original bytes; TestStandaloneCREscapesNotTerminator covers a \r with no following \n — content escaped ^M mid-line and on an unterminated final line; TestLoadEmptyFile pins zero Lines() entries with the one-digit-slot three-cell gutter. TestTerminatorBytesMapToEndOfLine is the EOL table: the rg $ at byte 4 of hit\r\n, the \r byte alone, the whole \r\n, the LF byte, and an empty line's positions all land on the display end-of-line position as the empty cell span Issue #23's marker occupies. TestSpanCrossingTerminatorHighlightsVisibleTextOnly covers .* over hit\r highlighting only the visible cells. TestLeadingUTF8BOMInvisibleWithAdjustedCoordinates proves the BOM invisible, Raw retaining it, SearchBytes() exposing the rg-line view, cell offsets in raw-file coordinates (rg offset 0 → raw byte 3), and later lines unadjusted; TestBOMLineTerminatorMapsToEndOfLine and TestNonLeadingFEFFIsOrdinaryContent cover the BOM-line terminator and U+FEFF-as-content.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestTerminatorsRemovedFromDisplayRetainedInRaw|TestStandaloneCREscapesNotTerminator|TestLoadEmptyFile|TestTerminatorBytesMapToEndOfLine|TestSpanCrossingTerminatorHighlightsVisibleTextOnly|TestLeadingUTF8BOMInvisibleWithAdjustedCoordinates|TestBOMLineTerminatorMapsToEndOfLine|TestNonLeadingFEFFIsOrdinaryContent' ./internal/filebuffer 2>&1 | grep -E '^(--- |=== RUN|ok|FAIL|    ---)' | grep -v '=== RUN' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestLoadEmptyFile
--- PASS: TestTerminatorsRemovedFromDisplayRetainedInRaw
--- PASS: TestStandaloneCREscapesNotTerminator
--- PASS: TestTerminatorBytesMapToEndOfLine
    --- PASS: TestTerminatorBytesMapToEndOfLine/zero-width_inside_CRLF
    --- PASS: TestTerminatorBytesMapToEndOfLine/CR_byte_only
    --- PASS: TestTerminatorBytesMapToEndOfLine/whole_CRLF
    --- PASS: TestTerminatorBytesMapToEndOfLine/LF_terminator_byte
    --- PASS: TestTerminatorBytesMapToEndOfLine/zero-width_at_LF_end
    --- PASS: TestTerminatorBytesMapToEndOfLine/zero-width_on_an_empty_line
    --- PASS: TestTerminatorBytesMapToEndOfLine/empty_line's_terminator
--- PASS: TestSpanCrossingTerminatorHighlightsVisibleTextOnly
    --- PASS: TestSpanCrossingTerminatorHighlightsVisibleTextOnly/rg_dot-star_covers_text_and_CR
    --- PASS: TestSpanCrossingTerminatorHighlightsVisibleTextOnly/mid-line_start_through_CRLF
    --- PASS: TestSpanCrossingTerminatorHighlightsVisibleTextOnly/whole_line_including_CRLF
--- PASS: TestLeadingUTF8BOMInvisibleWithAdjustedCoordinates
--- PASS: TestBOMLineTerminatorMapsToEndOfLine
--- PASS: TestNonLeadingFEFFIsOrdinaryContent
ok  	vrg/internal/filebuffer
```

## Full filebuffer and whole-module regression

The Line.CellsCovering shadow also serves viewport's StopTarget — rg-coordinate reveal targets on BOM lines now translate too — so the whole suite runs.

```bash
cd /home/chris/vrg && go test -count=1 ./internal/filebuffer ./internal/viewport ./internal/app ./internal/safepresentation 2>&1 | sed -E 's/\t[0-9.]+s$//' | tail -6
```

```output
ok  	vrg/internal/filebuffer
ok  	vrg/internal/viewport
ok  	vrg/internal/app
ok  	vrg/internal/safepresentation
```

## Manual check — the real binary on a pty

pty_structure.py runs the built vrg on a real 100x24 pty over per-case fixture directories, replaying the byte stream through a small terminal emulator. Session A searches hit in a CRLF file: rows render '1  hit' / '3  hit x' with no ^M anywhere in the raw stream and the styled run holding 'hit'. Session B searches cd in a file with a standalone CR mid-line: the row renders '1  ab^Mcd' — the \r is content, escaped by the safe-presentation core — and the match after it stays styled. Session C searches hit in a UTF-8 BOM file: no BOM bytes or \ufeff anywhere in the output, and the first-line match styles 'hit' at the line's first cells — rg offset 0 mapped to raw byte 3. Session D puts a fake rg on PATH emitting a valid begin/match/end/summary stream claiming a foo match on line 1 of f/a.txt while a.txt is zero bytes on disk: the loaded buffer holds zero source lines, so every panel row is blank — the empty panel with its three-cell gutter (the stale note for this situation is Issue #29's; before it lands, just an empty panel).

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/022-04/code-walkthrough/vrg ./cmd/vrg && python3 Notes/walkthroughs/022-04/code-walkthrough/pty_structure.py
```

```output
setup    : CRLF — '1  hit', no ^M, hit styled — '1  hit    '
n        : line 3 current — '3  hit x    '
q        : exit 0
setup    : standalone CR renders ^M — '1  ab^Mcd   '
q        : exit 0
setup    : BOM invisible — '1  hit' styled at the line start — '1  hit    '
q        : exit 0
setup    : zero-byte a.txt — panel all blank, zero source rows — '            '
q        : exit 0
OK
```

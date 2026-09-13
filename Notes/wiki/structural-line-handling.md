# Structural line handling

Issue #22: line terminators, unterminated final line, empty file, and
leading UTF-8 BOM. FileBuffer splits file bytes into lines, strips
terminators from display while retaining original bytes for
byte-coordinate mapping, handles the empty file as zero lines with a
three-cell gutter, maps terminator bytes and zero-width positions to
the display end-of-line column, highlights only the visible text for
spans crossing terminators, and strips a leading UTF-8 BOM so rg-line
and raw-file coordinates are kept separate. References:
[PRD-vrg](../PRD-vrg.md) (Text, graphemes, and safe presentation;
Encodings and stale-content validation),
[grapheme-cluster-highlight-expansion](grapheme-cluster-highlight-expansion.md),
[safe-presentation](safe-presentation.md),
[wrap-mode-and-grapheme-policy](wrap-mode-and-grapheme-policy.md).

## Line terminators

LF (`\n`) and CRLF (`\r\n`) are line terminators: they end a line and
are not displayed. The original line bytes including terminators are
retained in `Line.ByteCells` (one entry per raw byte) for
byte-coordinate mapping and later stale validation (Issue #29). A
standalone CR (not followed by LF) is not a terminator; the
[safe-presentation](safe-presentation.md) core escapes it as `^M`
(two display cells). `splitLines` scans the raw file bytes and emits
each line including its terminator; a trailing terminator does not
produce an extra empty line, and a missing final newline still yields
a final line.

`splitLines` is a byte-level scan: it recognises `\n` and `\r\n` as
terminators and treats every other byte (including a standalone `\r`)
as content. Mixed terminator files (LF on some lines, CRLF on others)
split correctly because the scan checks each byte position
independently.

## Final-line and empty-file rules

- A file without a trailing newline still counts and displays its last
  line. `"a\nb\nc"` has three lines; the third (`"c"`) has no
  terminator but is a full source line.
- A trailing newline does not invent an extra empty line. `"a\nb\n"`
  has two lines, not three.
- An empty file (zero bytes) produces zero source lines. `LineCount`
  is 0, `Lines` is empty, and `GutterWidth` is 3 (one digit slot plus
  two spaces). The panel is empty with no source rows. The stale note
  for an empty file that was non-empty at search time is owned by
  Issue #29.

`gutterWidth` returns the digit count of the largest line number plus
two spaces, with a minimum of one digit. A zero-line view reserves one
digit slot, so its gutter is three cells wide.

## Terminator-to-EOL display mapping

Removed terminator bytes and zero-width positions at the end of a line
map to the display end-of-line position. For `hit\r\n` (bytes h, i, t,
`\r`, `\n`), the display is `"hit"` (three cells, columns 0–2). Both
the `\r` (byte 3) and `\n` (byte 4) map to `[3, 3)` — the zero-width
end-of-line column. `safepresentation.EscapeContent` produces these
zero-width cell ranges for terminator bytes; `expandedByteCells`
preserves them because no grapheme cluster covers the end-of-line
display byte offset.

A span covering visible text plus the terminator highlights only the
visible text. For a submatch `[0, 5)` on `hit\r\n`, the highlight is
`[0, 3)` — covering `"hit"` but not the terminator bytes. The
end-of-line *marker* for a terminator-only zero-width match is owned
by Issue #23; this issue covers the byte-to-cell mapping and the
visible-text-only highlight for spans that cross a terminator.

## Leading UTF-8 BOM

A leading UTF-8 BOM (`EF BB BF`, U+FEFF) is invisible in display.
Ripgrep 15.x removes a leading UTF-8 BOM from searched line data under
default detection, so rg's line 1 offsets omit the three BOM bytes.
`Load` detects and strips the BOM before `splitLines` so the raw-line
bytes are in rg-line coordinates and submatch offsets align directly.
`Buffer.BOMOffset` records the number of stripped BOM bytes (0 or 3)
for converting back to raw-file coordinates (e.g. Issue #29 stale
validation). A first-line match at rg offset 0 maps to raw byte 3 and
highlights the correct display cell (column 0 after BOM stripping).
The BOM adjustment applies only to line 1; line 2 and beyond have no
adjustment.

A BOM-only file (`EF BB BF` with no content) produces zero source
lines, just like an empty file. A BOM followed by a lone newline
produces one empty line (the newline terminates line 1, which has no
visible content after BOM stripping).

Non-leading U+FEFF is not a file BOM. It is ordinary content:
displayed, counted, and matchable like any other valid printable
Unicode character. Only a U+FEFF at the very start of the file is
treated as a BOM and stripped.

## Raw-file and rg-line coordinate views

Two coordinate views are maintained throughout:

- **Raw-file coordinates** — byte offsets in the actual file on disk.
  For a BOM file, raw byte 0 is the first BOM byte and raw byte 3 is
  the first content byte.
- **rg-line coordinates** — byte offsets in the line data as ripgrep
  reports it. For line 1 of a BOM file, rg offset 0 is the first
  content byte (BOM removed). For all other lines, rg-line and raw-line
  coordinates coincide.

`Load` strips the BOM so `Line.ByteCells` maps rg-line bytes to
display cells. `Buffer.BOMOffset` is the conversion offset for line 1:
`rawByte = rgOffset + BOMOffset` for line 1, `rawByte = rgOffset` for
all other lines. The display view (cell columns) is derived from the
rg-line view through `EscapeContent` and the grapheme policy.

## Implementation

The implementation lives in
[internal/filebuffer/filebuffer.go](../../internal/filebuffer/filebuffer.go):

- `Load` detects `EF BB BF` at the start of the file data, strips it,
  and records `Buffer.BOMOffset`.
- `splitLines` splits the (BOM-stripped) data into lines including
  terminators.
- `safepresentation.EscapeContent` produces the display text (no
  terminators), per-byte cell ranges, and per-byte display offsets.
- `expandedByteCells` remaps cell ranges to grapheme-cluster
  boundaries (Issue #21); terminator bytes keep their zero-width
  end-of-line cells because no cluster covers them.
- `expandedHighlights` converts submatch byte ranges to display-cell
  spans. When the end byte has no cluster (a terminator), the
  highlight uses the end byte's original cell end (the end-of-line
  position) so a span covering visible text plus terminator
  highlights only the visible text.

## Tests

Tests live in
[internal/filebuffer/structural_line_test.go](../../internal/filebuffer/structural_line_test.go)
(external package `filebuffer_test`). See
[unit-tests](unit-tests.md) for the full catalogue.

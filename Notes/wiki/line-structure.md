# Structural line handling — terminators, final lines, empty files, and the UTF-8 BOM (Issue #22)

Delivered by
[Issue #22](../issues/022-line-terminators-final-line-empty-file-utf8-bom.md)
([task](../tasks/022-line-terminators-final-line-empty-file-utf8-bom.md)):
FileBuffer's structural line handling now keeps the three coordinate
views separate — raw-file bytes, the rg-line view ripgrep's reported
lines and submatch offsets index, and display cells — so terminators,
final-line and empty-file rules, and the leading UTF-8 BOM all map
consistently. Relevant PRD sections: *Text, graphemes, and safe
presentation* (the first two bullets) and *Encodings and
stale-content validation* (the UTF-8 BOM bullet) in
[`Notes/PRD-vrg.md`](../PRD-vrg.md); user stories 48, 73, 74. Builds on
[safe-presentation.md](safe-presentation.md) (`Mapped` — the escaped
text plus byte→cell map — and the standalone-CR `^M` escape); the
end-of-line *marker* these mappings locate is
[Issue #23](zero-width-markers.md)'s, and the
stale validation that consumes the retained bytes is Issue #29's.

## Terminators and line counting

`Load` splits on LF; a `\r` immediately before the `\n` is half of the
CRLF terminator. Both forms terminate lines without producing display
cells, yet `Line.Raw` retains the original line bytes — terminator
included — for byte-coordinate mapping and Issue #29's validation.
A `\r` with no following `\n` is not a terminator: it stays content and
the safe-presentation core escapes it as `^M`, whether it sits mid-line
or ends an unterminated final line.

- A missing final newline still yields the last line; a trailing
  newline does not invent an extra empty one.
- An empty file has **zero** source lines — an empty panel — and the
  gutter still reserves its one digit slot: `GutterWidth` is the digit
  width of the largest line number plus two spaces, minimum one digit
  slot, so the zero-line gutter is three cells.

## The three coordinate views

`makeLine` splits each raw line into the leading-BOM, content, and
terminator regions, and `Line` records the boundaries:

- **Raw-file view** — `Line.Raw`, the line's bytes as read from disk
  (BOM and terminator included). `Cell.Start`/`End` byte offsets are
  raw-file coordinates, so the byte→cell map always describes file
  bytes.
- **Rg-line view** — `Line.SearchBytes()`, exactly the bytes ripgrep
  reported and indexed submatch offsets against: a leading UTF-8 BOM
  is stripped on line 1 (rg's default BOM detection removes it from
  searched data) while the terminator stays. It differs from `Raw`
  only by the `searchOff` adjustment — three on a BOM line, zero
  elsewhere.
- **Display cells** — `Mapped` produced from the content region, with
  cell byte offsets shifted into raw-file coordinates.

`Line.CellsCovering(start, end)` is the single mapper from rg-line
byte ranges to display-cell ranges, shadowing the embedded
`Mapped.CellsCovering` with the coordinate translation layered on:
rg offsets shift by `searchOff` into raw coordinates, then the
content region maps through the byte→cell map. `makeLine` records
`Highlights` and `Rows.StopTarget` derives reveal targets through it,
so BOM lines need no special cases downstream.

## Terminator-to-end-of-line mapping

Bytes the display removed — the line terminator — and positions past
the content end all land on the display end-of-line position, one past
the last cell:

- A zero-width position inside the terminator — the rg `$` at byte 4
  of `hit\r\n` — maps to display column 3; so does a span covering
  only terminator bytes (`\r` alone or the whole `\r\n`). The recorded
  highlight is the empty cell span `[3,3)`, the marker position
  [Issue #23](zero-width-markers.md) paints; the indicators treat an
  empty span as the one-cell position of a marker.
- A span covering visible text plus terminator — `.*` matching
  `hit\r` on a CRLF line — highlights only the visible text `[0,3)`;
  removed bytes add no cell.
- A zero-width position inside content lands on the cell holding its
  byte (a position inside a cluster maps to the cluster's start); the
  same end-of-line rule covers a BOM line's terminator in rg
  coordinates.

## The leading UTF-8 BOM

A leading `\xef\xbb\xbf` on line 1 is invisible: it never reaches
`MapContent`, so no `\ufeff` escape or stray cell appears. Because rg
omits the BOM's three bytes from first-line offsets, the line's cell
map keeps raw-file coordinates — the first content cell maps byte
range `[3,4)` — and `CellsCovering` shifts rg ranges by three, so an
rg match at offset 0 maps to raw byte 3 and highlights the correct
cells. A `U+FEFF` anywhere else — mid-line-1 or at the start of any
later line — is ordinary content: the safe-presentation core escapes
the non-printable rune as `\ufeff` with unadjusted offsets.

## Tests

See [unit-tests.md](unit-tests.md) — `internal/filebuffer`'s Issue #22
cases cover LF/CRLF/mixed splitting with retained `Raw`, the
standalone-CR `^M` escape, the zero-line empty file's gutter, the
terminator-to-EOL table, text-plus-terminator spans, the BOM
adjustment and `SearchBytes` view, and non-leading `U+FEFF` content.

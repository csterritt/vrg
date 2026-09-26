# Line terminators, final line, empty file, and the UTF-8 BOM

Issue #22
(`Notes/issues/022-line-terminators-final-line-empty-file-utf8-bom.md`,
tasks
`Notes/tasks/022-line-terminators-final-line-empty-file-utf8-bom.md`)
delivered FileBuffer's structural line handling: LF and CRLF terminate
lines without being displayed while the original line bytes are
retained for byte-coordinate mapping, an unterminated final line still
counts and a trailing newline invents no phantom line, an empty file
has zero source lines behind a three-cell gutter, removed terminator
bytes and zero-width positions map to the display end-of-line
position, a span crossing a terminator highlights the visible text
only, and a leading UTF-8 BOM is invisible while its three bytes split
the first line into separate raw-file and rg-line coordinate views.

PRD cross-references: "Text, graphemes, and safe presentation" (the
first two bullets — line endings, and terminator/zero-width EOL
mapping) and "Encodings and stale-content validation" (the UTF-8 BOM
bullet) in `Notes/PRD-vrg.md`, plus "Testing Decisions → FileBuffer".

## Structural lines

`filebuffer.Load` splits the file on LF and hands each line's bytes —
**terminator included** — to `present.LineOf`. Retaining the original
bytes is what makes the raw view a byte-coordinate map: a recorded
submatch can name terminator bytes and still validate
(`raw[3:5] == "\r\n"` on `hit\r\n`), which is also the view Issue
#29's stale-content revalidation will compare against. The splitting
rules are the ones rg's line data already follows:

- A missing final newline still yields the final line; a trailing
  newline does not invent an extra empty one.
- An empty file produces **zero** source lines — no rows at all. The
  gutter still reserves its minimum one-digit slot plus two spaces, so
  a zero-line panel sits behind a **three-cell** gutter (the same
  "digit slots + two spaces" rule as every other file, clamped to one
  slot).
- A standalone CR — one not followed by LF — is not a terminator. It
  stays inside its line and the safe-presentation core escapes it as
  `^M` (see [safe-presentation.md](safe-presentation.md)).

## Terminator bytes map to end of line

`LineOf` emits no cells for LF or CRLF bytes; they keep their slots in
the line's per-byte `lo`/`hi` map pointed at the display end-of-line
position — the same place zero-width positions at or past the line's
end land. So byte 4 of `hit\r\n` maps to display column 3, a recorded
match solely on removed terminator bytes becomes an end-of-line marker
position (`Start == End`), and a span covering visible text plus the
terminator — like rg's real `t\r?$` match `t\r` at bytes 2–4 — maps to
the visible text alone (`{2,3}` covers the `t` cell). The marker these
positions paint is Issue #23's
([zero-width-match-markers.md](zero-width-match-markers.md)); the
mapping itself is already cell-precise.

## Raw-file and rg-line coordinates

Ripgrep 15.x removes a leading UTF-8 BOM from searched first-line data
under its default encoding detection (kept on — vrg forces no raw
mode), so on that one line the coordinate spaces disagree: **rg-line
offsets** count from after the BOM while the **raw-file view** retains
all bytes. `Load` detects `EF BB BF` at file start and bridges the
views:

- The first line is escaped by `present.LineOfBOM`: the BOM's three
  bytes stay in `Raw` but produce no display — no `◌` fallback, no
  garbage — and map to the line-start position.
- Every submatch on line 1 is shifted by three into the raw view
  before bounds-checking and byte-equality validation, so rg offset 0
  names raw byte 3 and highlights the cells of the text rg actually
  saw. Positions and endpoints shift alike: rg's end-of-line offset 3
  on `hit` is raw byte 6 — the LF — and maps to the end-of-line
  marker position.
- Lines after the first are unshifted, and a U+FEFF anywhere but the
  file's leading bytes is ordinary content — mid-line it joins the
  previous cell as a zero-width cluster, at a later line's start it
  takes the usual `◌` fallback cell, and its bytes match and highlight
  normally.

The same `bytes`-vs-`text` JSON encodings feed the same path — the
recorded `Submatch.Bytes` are compared against the shifted raw range.
The end-of-line *marker* for terminator-only matches is Issue #23's
([zero-width-match-markers.md](zero-width-match-markers.md)), and the
stale-note consumption of the retained raw bytes is Issue #29's.

## Tests

- `internal/filebuffer/lines_test.go` — the structural table: LF,
  CRLF, and mixed terminators undisplayed with unterminated-final and
  blank-line cases; standalone CRs escaping as `^M` without breaking
  the line; the empty file's zero lines and three-cell gutter;
  terminator-only and zero-width submatches mapping to the
  end-of-line position; text-plus-terminator spans covering the
  visible text only; the BOM's invisible display with rg offset 0
  landing on raw byte 3 (including a second, content U+FEFF proving
  the shift is real) and later lines unshifted; BOM-only files as one
  zero-display line; and non-leading U+FEFF as ordinary content.

See [unit-tests.md](unit-tests.md) § `internal/filebuffer`.

## Files

- `internal/filebuffer/filebuffer.go` — `Load` detects the leading
  UTF-8 BOM, escapes line one through `LineOfBOM`, and shifts that
  line's submatch offsets by three before validating and mapping.
- `internal/present/line.go` — `LineOf` is now `lineOf(raw, hidden)`
  with a hidden-prefix parameter; `LineOfBOM` runs it with the BOM's
  length so the prefix stays in `Raw` without painting, its bytes
  mapping to the line-start position.

See also: [safe-presentation.md](safe-presentation.md) (the escaping
core `LineOf`/`LineOfBOM` share and the `^M` rule),
[grapheme-highlight-expansion.md](grapheme-highlight-expansion.md)
(the cluster expansion applied after coordinate mapping), and
[browse-tracer.md](browse-tracer.md) (the panel these buffers render
into).

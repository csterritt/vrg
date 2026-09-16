## Issue 39: Final rendering uses the shared grapheme/cell model end to end

**Type**: AFK
**Blocked by**: Issue 38 — the renderer must draw into the correct content-panel width before its cell-level output can be verified against it

### Parent PRD

`Notes/PRD-vrg.md` — audit source: `Notes/critiques/final-audit-vrg.md`, High finding 4

### What to build

Unify the last rendering stage with the grapheme/cell policy used everywhere upstream (`internal/app/app.go:3312-3395`, `internal/theme/theme.go:177-223`). `FileBuffer` and `Viewport` already produce cell-based, grapheme-aware ranges, but the final stage — `renderLineWithHighlights`, `cellToBytePos`, `visibleWidth`, and Theme overlay sizing — reverts to one rune equals one cell. Observable consequences: a two-cell CJK highlight can consume the following character, combining sequences can be styled or truncated separately from their base cluster, and padding/overlay widths are wrong for wide text.

- Render the file panel directly from `Line.Clusters` and their cell spans rather than re-deriving positions from runes or bytes.
- Introduce one shared ANSI-aware cell-width helper implementing the same grapheme policy, and route every width/sizing consumer through it. The scope explicitly includes line rendering and highlight styling; `visibleWidth`; file-list entry padding in the composed view; indicator-column sizing; filename-row fitting; Theme overlay sizing; and file-change pop-up truncation, width, and centering.
- Replace `truncateLeftCells`'s rune-boundary implementation and its false assumption that `safepresentation.EscapePath` produces no combining marks. Route the pop-up through `TruncateLeftGrapheme` or the same shared cluster/cell primitive so printable wide and combining path text is never split or miscentered.
- Highlight and truncation boundaries must fall only on grapheme-cluster boundaries measured in cells. Mechanically enforce this boundary with one decidable allow-list: across every non-test production `.go` file under `internal/` and `cmd/`, `utf8.DecodeRuneInString` may appear only in `internal/safepresentation/cellwidth.go`. This includes display-geometry packages such as `internal/viewport` and `internal/filebuffer`; unrelated production decoding must use `for range`, an existing standard-library primitive, or another implementation that does not call the allow-listed symbol.

See PRD *Text, graphemes, and safe presentation* (single consistent grapheme/cell policy) and *Navigation, viewport, and logical anchors*.

### How to verify

- **Manual**: browse a file containing CJK text, combining sequences (e.g. `e` + combining acute), and emoji ZWJ sequences with a match overlapping them → each highlight covers the whole cluster and exactly its cell width; overlay text containing wide characters is sized and padded correctly; nothing is clipped mid-cluster.
- **Automated**: composed-view assertions — render through the app `View()` and inspect the emitted cell layout — for CJK highlights not consuming the following character, combining sequences styled/truncated only with their base, emoji ZWJ sequences treated as one cluster, overlay and file-list padding widths for wide diagnostic/path text, filename-row fitting, and pop-up truncation/centering for wide and combining paths. Add a grep/static guard that rejects rune-decoding width or truncation loops outside the shared grapheme/cell helper.

### Acceptance criteria

- [ ] Given a match overlapping a two-cell CJK character, then the highlight spans exactly that cluster's cells and never swallows the following character.
- [ ] Given a line containing a base character plus combining marks, then the sequence is styled, clipped, and truncated only as one cluster.
- [ ] Given an emoji ZWJ sequence, then it occupies its measured cell width and is never split by a highlight or clip boundary.
- [ ] Given overlays containing wide or combining text, then Theme sizing and padding use the shared cell-width helper and borders align.
- [ ] Given a wide or combining path in the file list, filename row, or file-change pop-up, then padding, fitting, left truncation, and centering use grapheme boundaries and measured cells; `truncateLeftCells` no longer relies on rune counts or the false no-combining-marks assumption.
- [ ] Given any consumer of display width in the final render path, then it uses the single ANSI-aware grapheme/cell helper — including `renderLineWithHighlights`, `cellToBytePos`, `visibleWidth`, list-entry padding, indicator sizing, filename-row fitting, pop-up truncation/centering, and Theme overlay sizing — and a mechanical check finds no separate `utf8.DecodeRuneInString`-based width/truncation loop.

### User stories addressed

- User story 71: one consistent grapheme/cell policy for wrapping, clipping, and highlights
- User story 72: partial-grapheme matches highlight the whole grapheme
- User story 75: invalid content replaced and controls escaped in every output sink
- User story 82: help and diagnostics wrapped and vertically scrollable at usable sizes

---

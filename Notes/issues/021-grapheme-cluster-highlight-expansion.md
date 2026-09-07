## Issue 21: Grapheme-cluster highlight expansion and wide-glyph safety

**Type**: AFK
**Blocked by**: Issue 19, Issue 20

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

FileBuffer maps submatch byte ranges to display cells using the shared grapheme policy, and Viewport/App render them consistently.

- A nonempty span that partially covers a cluster is expanded outward to the whole cluster.
- A combining-only match inside a base cluster highlights that whole cluster.
- A cluster with no base/independent visible cell (e.g. a standalone combining mark) receives a visible fallback cell so the highlight is never zero cells.
- Wide glyphs are never split by highlight boundaries; wrap/clip blanks from Issues 16/18 are never painted as match cells.
- Reveal (Issues 14/19) and indicators (Issue 20) use the same expanded cell spans: after this issue, the span FileBuffer hands to Viewport/App is the cluster-expanded span, so a match whose recorded bytes start mid-cluster reveals and is indicator-counted from the cluster start. Update Issue 20's indicator tests to consume the expanded spans (a match starting mid-cluster whose cluster start is hidden left counts as hidden left).

See PRD *Text, graphemes, and safe presentation* (grapheme expansion bullets) and *Testing Decisions → FileBuffer*.

### How to verify

- **Manual**: in a file containing decomposed `e\u0301` (`printf 'cafe\xcc\x81\n'`), search for the combining mark itself, `vrg "$(printf '\xcc\x81')" .` → the whole `é` glyph is highlighted (partial-cluster expansion). Do **not** search for precomposed `é`: ripgrep does not normalise, so it will not match the decomposed bytes. Search a CJK character → both cells highlighted; a file containing a standalone `\xcc\x81` at line start, searched the same way → a visible highlighted fallback cell appears.
- **Automated**: FileBuffer tests for partial-grapheme span expansion (start inside, end inside, both), combining-only match, standalone combining cluster fallback cell, wide-cluster cell pair, emoji ZWJ sequence; rendering test that a highlight at a wrap boundary does not paint the blank filler cell.

### Acceptance criteria

- [ ] Given a submatch covering part of a grapheme cluster, then the highlight covers the entire cluster's cells.
- [ ] Given a match consisting only of combining marks attached to a base, then the base cluster is highlighted.
- [ ] Given a cluster with no visible base cell, then a fallback cell is rendered and highlighted.
- [ ] Given a two-cell glyph, then both of its cells are highlighted together and never split.

### User stories addressed

- User story 72: partial-grapheme matches highlight the whole grapheme

---

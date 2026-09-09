## Issue 20: Hidden-content indicators — gutter `_`/`*` and reserved right column `*`

**Type**: AFK
**Blocked by**: Issue 19

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

Run-off-edge mode only (wrap mode has no indicators and no reserved column).

- **Left, every visible source line**: the first trailing gutter space shows inverse `_` if any text is hidden left, upgraded to inverse `*` if a match/marker on that line is entirely hidden left. Blank otherwise.
- **Right, current matched line only**: the reserved rightmost column shows inverse `*` on that line's visible row when at least one match/marker is entirely hidden right. Blank elsewhere; never overwrites text. If the current matched line is vertically off-screen, no right indicator is drawn.
- Partially visible matches count as visible for indicator purposes. Visibility is computed over actually rendered cells after grapheme clipping, excluding the reserved column; a split wide glyph rendered as blanks is not visible.
- Both left and right stars may appear together.

See PRD *Layout and indicators* (indicator bullets) and *Further Notes* (asymmetric scope).

### How to verify

- **Manual**: `w` to run-off-edge in a file with several long matched lines; pan right → `_` appears on lines with hidden text, `*` on lines whose match is fully hidden; on the current matched line with a second match far right, `*` appears in the last column; pan so a match is half visible → no star for that side.
- **Automated**: rendering tests for: `_` vs `*` per line; right `*` only on the current line; current line off-screen → no right marker; both sides hidden; partial visibility → no marker; a match in the last text cell with another farther right → right `*`; split-wide-glyph blanks not counted as visible; a uniform-lines fixture where every visible line has hidden-left text at a nonzero offset shows `_` on every visible line — permitted, since the pan clamp's painted-cluster guarantee (Issue 18) constrains painted cells, not indicator counts; wrap mode draws no indicators and no reserved column.

### Acceptance criteria

- [ ] Given run-off-edge mode and a line with text hidden left, then its gutter shows inverse `_`; given a match entirely hidden left, then inverse `*`.
- [ ] Given the current matched line has a match entirely hidden right, then the reserved column shows inverse `*` on that line only.
- [ ] Given a match partially visible on a side, then no hidden-match indicator is shown for that side.
- [ ] Given a non-current line with a match hidden right, then no right indicator is drawn.
- [ ] Given wrap mode, then no indicators and no reserved column exist.

### User stories addressed

- User story 67: per-line gutter `_`/`*`
- User story 68: reserved right column `*` for the current matched line
- User story 69: partially visible matches count as visible

---

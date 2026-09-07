## Issue 23: Zero-width match markers

**Type**: AFK
**Blocked by**: Issue 20, Issue 22

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

A zero-width submatch is rendered as one inverse-video space at its mapped display location (underlined on the current matched line).

- It marks an existing cell without shifting text; at end of line it extends the effective line width by one cell. An empty matched line therefore has width one; a marker after a completely full wrap row occupies another row.
- A position inside a cluster maps to the cluster start; no split wide glyph.
- A match solely on removed terminator bytes (e.g. `$` on `hit\r\n`) becomes an ordinary end-of-line marker at column 3 — not a special case: same reveal (Issues 14/19), wrap (16), clip (18), horizontal-extent (18) and indicator (20) rules as any other marker, including counting as "entirely hidden" for gutter `*` and right `*`.
- Marker cells are navigable targets and participate in horizontal extent/pan clamping: a marker contributes to its line's content extent, and the Issue 18 paintable-boundary maximum treats it like any other cluster (a marker-only line has extent 1 and maximum offset 0).

See PRD *Text, graphemes, and safe presentation* (zero-width bullets) and *Testing Decisions → FileBuffer / Viewport*.

### How to verify

- **Manual**: `vrg '^' file` → each line shows an inverse cell at column 0 (empty lines show a single inverse cell); `vrg '$' file` → an inverse cell after each line's last character; `n` between them works; in run-off-edge mode panning past the marker sets the left `*`.
- **Automated**: FileBuffer/Viewport tests: zero-width at BOL, inside a wide cluster (maps to cluster start), at EOL (width +1), on an empty line (width 1), on LF and CRLF terminator-only matches (`hit\r\n` → column 3); marker after an exactly-full wrap row creates a new row; marker hidden left/right drives indicators; marker is the reveal target for `n`; a marker-only line has extent 1 and maximum pan offset 0 (Issue 18's paintable-boundary maximum).

### Acceptance criteria

- [ ] Given a zero-width match at a text position, then one inverse cell is drawn there and following text is not shifted.
- [ ] Given a zero-width match at end of line or on an empty line, then a marker cell is drawn and the effective line width includes it.
- [ ] Given a terminator-only match on `hit\r\n`, then a single marker cell appears at display column 3 with the same wrap/clip/indicator behaviour as any other marker.
- [ ] Given a zero-width position inside a wide cluster, then the marker is placed at the cluster start and the glyph is not split.
- [ ] Given a marker entirely hidden horizontally, then the corresponding `*` indicator is shown.

### User stories addressed

- User story 70: zero-width matches rendered as one inverse cell, visible and navigable

---

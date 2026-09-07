## Issue 29: Stale-match validation and "file changed since search" note

**Type**: AFK
**Blocked by**: Issue 27, Issue 22

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

FileBuffer validates each submatch against loaded content (on first load and every reload).

- Check line existence, range validity against original line bytes (including terminators, with UTF-8 BOM adjustment from Issue 22), and byte equality with the recorded submatch bytes (`text` or `bytes` form). On any failure drop that submatch and mark the buffer stale; keep other valid highlights.
- A stale buffer shows "file changed since search" in the filename row every time it is displayed; no timer. `r` recomputes it and the note disappears only if the new content validates fully.
- Stale entries remain navigation stops with clamped landing positions: reveal the first surviving submatch; if none survive but the line exists, use the first recorded start clamped to the line's bytes and mapped to a valid display cell (EOL fallback clamps to the last rendered cell when there is no marker); if the line is gone, land at the last source line's start; an empty file remains a zero-line panel. Fallbacks draw no highlights or markers.
- Best-effort only: same-text moves or changes outside matched spans may go undetected.
- Do not run validation against UTF-16/32 raw bytes (Issue 30).

See PRD *Encodings and stale-content validation* (validation and stale-entry bullets).

### How to verify

- **Manual**: search, then edit the matched word in file 2 to a same-length different word; `n` into file 2 (or `r` if current) → no highlight on that line and the filename row says "file changed since search"; delete trailing lines so a matched line is gone → `n` to it lands on the last line without highlight; revert the file and `r` → note clears.
- **Automated**: FileBuffer tests: out-of-bounds range; same-length text replacement; one of two submatches surviving; all dropped but line exists → clamped start; line gone → last line; empty file; reload clearing/preserving the note; validation uses original/search bytes not stripped display text; CRLF terminator match still validates.

### Acceptance criteria

- [ ] Given a submatch whose bytes no longer match the loaded line, then it is not highlighted and the buffer is marked stale.
- [ ] Given a stale buffer, then "file changed since search" appears in the filename row on every display until a reload validates cleanly.
- [ ] Given a stale stop with surviving submatches, when navigated to, then the first survivor is revealed.
- [ ] Given a stale stop whose line no longer exists, when navigated to, then the viewport lands at the last source line's start with no invented highlight.
- [ ] Given a stale stop whose line exists but has no survivors, then the landing position is the recorded start clamped to the line.

### User stories addressed

- User story 44: mismatched submatches dropped; persistent "file changed since search" note
- User story 45: stale entries retained with clamped landing positions

---

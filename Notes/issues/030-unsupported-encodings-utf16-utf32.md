## Issue 30: Unsupported encodings — UTF-16/UTF-32 BOM placeholder

**Type**: AFK
**Blocked by**: Issue 26, Issue 27

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

- FileBuffer detects UTF-16 LE/BE and UTF-32 LE/BE BOMs (check longer BOMs before overlapping shorter ones: UTF-32 LE `FF FE 00 00` before UTF-16 LE `FF FE`).
- Such files show "(unsupported encoding)", no file text, no highlights, and an explanatory diagnostic. They remain indexed cursor stops and reloadable with `r`.
- Notification follows the current/non-current distinction from Issue 26: overlay if current, diagnostic-only otherwise.
- No stale-match validation runs on their raw bytes.
- rg's default BOM detection remains enabled (no forced encoding flag), so these files can still yield matches.

See PRD *Encodings and stale-content validation* (first bullet) and *Invocation…* (no forced raw-encoding flag).

### How to verify

- **Manual**: `printf '\xff\xfeh\0i\0\n\0' > u16.txt`; `vrg hi .` → the file is listed; entering it shows "(unsupported encoding)" and an overlay; `r` keeps the placeholder; `q` exits 0 and stderr includes the encoding diagnostic.
- **Automated**: FileBuffer tests for all four BOMs including the UTF-32/UTF-16 LE overlap ordering; a UTF-8 BOM is not misclassified; App tests: current file → overlay + placeholder; non-current → diagnostic only; `r` on an unsupported file reissues a load and preserves the placeholder if unchanged; no stale validation invoked.

### Acceptance criteria

- [ ] Given a file starting with a UTF-16 or UTF-32 BOM, then the panel shows "(unsupported encoding)" with no text or highlights.
- [ ] Given `FF FE 00 00`, then the file is classified as UTF-32 LE, not UTF-16 LE.
- [ ] Given such a file is current, then an overlay explains it; given it is non-current, then only a diagnostic is collected.
- [ ] Given an unsupported file, then it remains a navigation stop and `r` reloads it.
- [ ] Given an unsupported file, then no "file changed since search" validation is applied to its bytes.

### User stories addressed

- User story 47: BOM-marked UTF-16/UTF-32 files show "(unsupported encoding)"

---

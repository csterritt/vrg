## Issue 27: `r` explicit reload — dropped duplicates, anchor preservation, failure replaces content

**Type**: AFK
**Blocked by**: Issue 26, Issue 17

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

- `r` rereads the current file from disk without rerunning rg. It never adds/removes cursor stops or discovers new matches. "Loading…" is shown during the reload.
- Reload preserves the cursor and the logical viewport anchor (Issue 17), clamped to new content; reload alone does not reveal a match.
- A failed reload replaces the old display with "(unreadable)" — stale content is never presented as refreshed.
- A duplicate `r` (or re-entry) while a load for that path is in flight is dropped, not queued; there is no cancellation. The placeholder changing from "Loading…" to content or "(unreadable)" is the completion signal, after which `r` starts a new load.
- `r` works even with a one-stop index (the only retry route there).
- Cached content is intentionally stable until `r`; disk edits are not observed otherwise.
- Filename row still identifies the path during reload.

Validation of original matches against new content ("file changed" note) is Issue 29.

See PRD *File loading, cache, reload, and selection consistency* (reload bullets).

### How to verify

- **Manual**: open a file, scroll, edit it externally (append lines) → display unchanged; `r` → new content, same top position; delete the file; `r` → "(unreadable)"; restore it; `r` → content again. With a single-match search, `r` reloads.
- **Automated**: model tests with gated loader: `r` shows "Loading…" and issues one load; second `r` while gated issues nothing; after completion `r` issues a new load; anchor preserved across reload (clamped when file shrinks); failed reload → "(unreadable)" and old content absent; one-stop index `r` works; no reload triggered by disk-change simulation without `r`.

### Acceptance criteria

- [ ] Given a loaded file, when `r` is pressed, then "Loading…" is shown and the file is reread; the cursor and logical anchor are preserved, clamped to new content.
- [ ] Given a load in flight for the current path, when `r` is pressed again, then no second load is started and none is queued.
- [ ] Given the placeholder has settled, when `r` is pressed, then a new load starts.
- [ ] Given the reload fails, then the panel shows "(unreadable)" and no old content.
- [ ] Given a one-stop index, when `r` is pressed, then the file is reloaded.
- [ ] Given the file changes on disk, when no `r` is pressed, then the display does not change.

### User stories addressed

- User story 40: `r` rereads without rerunning the search
- User story 42: duplicate load requests dropped; placeholder change is the completion signal
- User story 43: failed reload shows "(unreadable)"
- User story 46: cached content stable until `r`

---

## Issue 8: "No results found" screen (exit 1) and binary-file exclusion

**Type**: AFK
**Blocked by**: Issue 5

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

- After a complete, successful (rg exit 0 or 1) search with no usable results, show a centred "No results found" screen. `q` exits 1. `Esc` is a no-op there.
- SearchIndex: a valid `end` event with non-null `binary_offset` drops that file and all its previously collected matches. Count distinct excluded files.
- When all results were excluded as binary, the screen reads "No results found (N binary files skipped)" and still exits 1 — this applies to rg exit 0 with all-filtered results as well as rg exit 1.

See PRD *Result index…* (binary bullet) and *Outcome and exit-status contract* (last row and the empty-screen bullet).

### How to verify

- **Manual**:
  1. `vrg zzzznotfound .` → "No results found"; `q` → exit 1.
  2. In a directory containing only a binary file with a match (`printf 'foo\0bar' > b.bin`), `vrg foo .` → "No results found (1 binary files skipped)"; `q` → exit 1.
- **Automated**: SearchIndex fixture where a file emits two `match` events then an `end` with `binary_offset` → file absent, count 1; App model tests for the rg-1 empty stream (exit 1) and rg-0 all-binary stream (message includes count, exit 1); `Esc` no-op on this screen.

### Acceptance criteria

- [ ] Given rg exits 1 with a complete stream and no matches, then "No results found" is shown and `q` exits 1.
- [ ] Given a file with earlier matches whose `end` reports a non-null `binary_offset`, then none of its matches appear in the index.
- [ ] Given every matched file was excluded as binary, then the screen appends "(N binary files skipped)" with the distinct-file count and `q` exits 1.
- [ ] Given the no-results screen, when `Esc` is pressed, then nothing happens.

### User stories addressed

- User story 12: empty search shows "No results found", exit 1
- User story 13: binary files excluded entirely
- User story 14: binary-only results explain the empty list

---

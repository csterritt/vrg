## Issue 26: Read failures — "(unreadable)", current vs non-current notification, retry rules

**Type**: AFK
**Blocked by**: Issue 25, Issue 9, Issue 11

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

- A read failure for the **current** file shows an error overlay (Issue 9 component) and the "(unreadable)" placeholder in the panel; the file's cursor stops are retained and the filename row still identifies the path.
- A failure for a **non-current** file is recorded as a diagnostic only (collected for stderr replay, Issue 11) — no overlay, no in-UI indicator; the user discovers it by visiting that file (which then shows the overlay and placeholder) or at exit.
- Retry happens on re-entry from a **different** file, not on `n`/`p` steps between stops within the same failed file. (`r` retry is Issue 27.)
- Load failures never change the fixed search-derived exit status — including when **every** retained file fails to load, and including when the fixed status is already 2 (fatal search with usable results). Add these as rows to the Issue 9 outcome matrix.

See PRD *File loading, cache, reload, and selection consistency* (failure/retry bullets and the mid-session diagnostic note).

### How to verify

- **Manual** (run as an unprivileged user; `chmod 000` does not deny root, so on elevated shells use the automated injected loader instead): make the second matched file unreadable (`chmod 000`); startup shows file 1; `n` into file 2 → overlay + "(unreadable)"; `Esc`; `n` to another stop in file 2 → no new overlay; `p` `p` back into file 2 from file 1 → overlay again (retry attempted); `Esc` to dismiss the overlay, then `q` exits 0 (the first `q` with the overlay open would only dismiss it) and stderr lists the failures.
- **Automated**: model tests using an injected failing loader (not filesystem permissions): current-file failure → overlay + placeholder + stops retained; non-current failure → diagnostic collected, no overlay, no indicator; visiting the failed file later → overlay; same-file step → no reload request; entry from a different file → one new load request; outcome-matrix rows: all files fail to load with fixed status 0 → `q` still 0; current-file failure with fixed status 2 → still 2; a non-current failure diagnostic appears in the Issue 11 replay.

### Acceptance criteria

- [ ] Given the current file fails to load, then an error overlay is shown and the panel reads "(unreadable)" while its stops remain navigable.
- [ ] Given a non-current file fails to load, then a diagnostic is collected without any overlay or indicator.
- [ ] Given a failed file, when navigating between its own stops, then no retry is attempted.
- [ ] Given a failed file, when entered from a different file, then a retry load is issued.
- [ ] Given any load failure — including every retained file failing — then the ordinary exit status is unchanged, whether it was 0 or 2.
- [ ] Given a non-current load failure, then its diagnostic is present in the stderr replay at exit.

### User stories addressed

- User story 37: late non-current failure recorded without interruption
- User story 38: current-file failure shows overlay and "(unreadable)" retaining stops
- User story 39: retry on re-entry or `r`, not on same-file steps

---

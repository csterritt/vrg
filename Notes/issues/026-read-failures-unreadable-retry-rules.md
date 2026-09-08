## Issue 26: Read failures — "(unreadable)", current vs non-current notification, retry rules

**Type**: AFK
**Blocked by**: Issue 9, Issue 11, Issue 24, Issue 25

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

- A read failure for the **current** file shows an error overlay (Issue 9 component) and the "(unreadable)" placeholder in the panel; the file's cursor stops are retained and the filename row still identifies the path.
- The composed view stays well-formed at constrained widths: the filename row keeps identifying the (possibly truncated) path per Issue 24's slot/truncation rules while the panel shows the placeholder — nothing overflows the terminal and layout dimensions stay nonnegative.
- A failure for a **non-current** file is recorded as a diagnostic only (collected for stderr replay, Issue 11) — no overlay, no in-UI indicator; the user discovers it by visiting that file (which then shows the overlay and placeholder) or at exit.
- Retry happens on re-entry from a **different** file, not on `n`/`p` steps between stops within the same failed file. (`r` retry is Issue 27.)
- **Re-entry sequence** (deterministic order, applying when a previously failed file is entered from a different file):
  1. The file's prior failure is shown immediately: the error overlay opens (or re-opens) with the prior failure diagnostic, and the panel switches from "(unreadable)" to "Loading…".
  2. Exactly one retry load starts immediately — while the overlay is open, not after dismissal. (If a load for that path is somehow already in flight, the request is dropped per the one-load-per-path rule and the existing load's settlement drives step 3.)
  3. When the retry settles, the placeholder updates to content or "(unreadable)" without waiting for overlay dismissal. The overlay remains open and dismissible (`q`/`Esc`) throughout; a successful retry leaves the prior-failure overlay displayed until the user dismisses it.
  4. A second failure appends exactly one new diagnostic occurrence to the open overlay while preserving the reader's scroll position (the minimal append-preserving-scroll primitive owned by this issue; Issue 32 later generalizes it to all appended errors) and collects exactly one new occurrence for stderr replay (Issue 11). A success collects nothing new.
  5. If the user navigates away while the retry is in flight, the load completes per Issue 25 (updates that path's cache/status only); a later re-entry follows this same sequence again against the new prior state.
- Load failures never change the fixed search-derived exit status — including when **every** retained file fails to load, including when the fixed status is already 2 (fatal search with usable results), and including both at once: usable results with fixed status 2 where every retained file subsequently fails to load, where the ordinary status remains 2, the load failures affect only file presentation and diagnostics, and the already-fixed fatal-search outcome is not recomputed. Add these as rows to the Issue 9 outcome matrix.

See PRD *File loading, cache, reload, and selection consistency* (failure/retry bullets and the mid-session diagnostic note).

### How to verify

- **Manual** (run as an unprivileged user; `chmod 000` does not deny root, so on elevated shells use the automated injected loader instead): make the second matched file unreadable (`chmod 000`); startup shows file 1; `n` into file 2 → overlay + "(unreadable)"; `Esc`; `n` to another stop in file 2 → no new overlay; `p` `p` back into file 2 from file 1 → overlay again with the panel on "Loading…" (retry attempted immediately, per the re-entry sequence); `Esc` to dismiss the overlay while the retry is still running or after it fails again (the second failure appends one occurrence without moving the reader); then `q` exits 0 (the first `q` with the overlay open would only dismiss it) and stderr lists the failures. This manual route assumes an effectively immediate second failure; the slow-retry transition itself is verified by the gated model test below.
- **Automated**: model tests using an injected failing loader (not filesystem permissions): current-file failure → overlay + placeholder + stops retained; non-current failure → diagnostic collected, no overlay, no indicator; visiting the failed file later → overlay; same-file step → no reload request; entry from a different file → one new load request; **gated re-entry retry presentation**: entering the failed file from another file shows the prior-failure overlay and "Loading…" immediately and issues exactly one load; while the retry is gated, `Esc` dismisses the overlay without disturbing the load; releasing the gate with success → content replaces "Loading…", no new overlay or diagnostic; releasing with failure → "(unreadable)" returns, exactly one new occurrence is appended to the overlay (reader position preserved, via the append-preserving-scroll primitive owned by this issue) and to the replay collection; outcome-matrix rows: all files fail to load with fixed status 0 → `q` still 0; current-file failure with fixed status 2 → still 2; usable search results with fixed status 2 where every retained file subsequently fails to load → the ordinary status remains 2, the load failures affect only file presentation and diagnostics, and the already-fixed fatal-search outcome is not recomputed; a non-current failure diagnostic appears in the Issue 11 replay; composed-view test: the unreadable state with a long escaped path at ordinary and constrained widths — the row shows the truncated safe path, the panel shows "(unreadable)", nothing overflows, and layout dimensions remain nonnegative.

### Acceptance criteria

- [ ] Given the current file fails to load, then an error overlay is shown and the panel reads "(unreadable)" while its stops remain navigable.
- [ ] Given a non-current file fails to load, then a diagnostic is collected without any overlay or indicator.
- [ ] Given a failed file, when navigating between its own stops, then no retry is attempted.
- [ ] Given a failed file, when entered from a different file, then the prior-failure overlay and a "Loading…" placeholder are shown immediately and exactly one retry load is started.
- [ ] Given an in-flight entry retry, then the overlay remains dismissible and the placeholder updates on settlement without waiting for dismissal.
- [ ] Given an entry retry that fails again, then exactly one new diagnostic occurrence is appended (overlay and replay collection) with the reader's overlay position preserved; given a successful retry, then no new diagnostic is collected.
- [ ] Given any load failure — including every retained file failing — then the ordinary exit status is unchanged, whether it was 0 or 2.
- [ ] Given a non-current load failure, then its diagnostic is present in the stderr replay at exit.

### User stories addressed

- User story 37: late non-current failure recorded without interruption
- User story 38: current-file failure shows overlay and "(unreadable)" retaining stops
- User story 39: retry on re-entry or `r`, not on same-file steps

---

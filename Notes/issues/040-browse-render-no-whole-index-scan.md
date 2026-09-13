## Issue 40: Browse rendering drops the per-frame whole-index scan

**Type**: AFK
**Blocked by**: None — can start immediately

### Parent PRD

`Notes/PRD-vrg.md` — audit source: `Notes/critiques/final-audit-vrg.md`, High finding 5

### What to build

Remove whole-index work from the render path (`internal/app/app.go:2917-2936`, `2938-3013`). Every `View()` call currently invokes `m.index.Stops()` — copying every stop — and `groupByFile` — scanning and allocating groups for the entire index — before rendering only a small visible window of the file list. At the documented scale of roughly 100,000 matched lines this directly violates the requirement that frame rendering not scan the whole file list, and it puts constant-factor index-sized work on every keystroke.

- Prepare immutable per-file groups, current-file indexes, and any display metadata (truncated paths, widths) **once**, when the search completes and the index is finalized — not per frame.
- `View()` renders only the visible file-list range, reading precomputed groups rather than rescanning the index.
- Navigation updates the current-file pointer/index without regrouping; per-file data structures are shared, not re-derived.

See PRD *Resources and responsiveness* (frame rendering must not scan the whole file list; ~100,000 matched-line scale) and *File list and layout*.

### How to verify

- **Manual**: generate a large result set (tens of thousands of matched lines across many files) and hold `n`/`j` or resize repeatedly → navigation and repainting stay responsive with no per-keystroke stall growing with index size.
- **Automated**: strengthen the file-list cost guard so that accessing or copying all stops during a render fails the test — e.g. an index test double that panics or fails when `Stops()` is called, or a counter asserting bounded index access per `View()`; plus a render test asserting only the visible file range is materialized.

### Acceptance criteria

- [ ] Given a completed search, then `View()` performs no call that enumerates or copies the full stop list or regroups all files.
- [ ] Given a large index, then rendering cost per frame is bounded by the visible window, not by total stops or total files.
- [ ] Given navigation between files, then current-file tracking updates by index into precomputed groups without reallocation of the whole grouping.
- [ ] Given the strengthened cost guard, then any code path that reintroduces a whole-index scan or copy during rendering fails the test.
- [ ] Given the refactor, then file-list behaviour — deterministic order, current-file underline and scroll-into-view, left-truncation, width cap, hide/show toggle — is unchanged.

### User stories addressed

- User story 34: full-file loading and expensive processing leave input responsive
- User story 24: every retained matched file listed in deterministic path order
- User story 26: current file underlined and kept within the scrolled file list

---

## Issue 3: Spawn ripgrep, collect JSON into SearchIndex, "Searching…" screen

**Type**: AFK
**Blocked by**: Issue 2

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

The first real Bubble Tea application path: start `rg` with the protected argv from the invocation working directory, show a "Searching…" screen, collect the whole JSON event stream into a SearchIndex, then show an interim summary screen (`N files, M matched lines`) that `q` dismisses with exit 0. Later issues replace the interim screen with browsing.

- Start failure (rg not on PATH, exec error) → sanitized stderr diagnostic, exit 2, no TUI.
- SearchIndex recognises `begin`, `match`, `end`, `summary`, `context` (ignored). Accept both `text` and base64 `bytes` for paths, lines, and submatch text.
- Retain raw path bytes, line number, submatch byte ranges and recorded bytes. Merge same-path/same-line matches into one stop; submatches ordered by start then end. Index ordered by unsigned raw path bytes then line number.
- Resolve relative result paths against the invocation working directory; do not canonicalize.
- Collection and processing run off the UI update path; the UI stays responsive (the model handles resize while searching).
- "Searching" covers the whole of collection **and** post-exit processing: parsing, filtering, sorting and index preparation. The app stays in the searching state (and Issue 4's cancellation rules apply) until the index is ready, even after rg itself has exited. Structure the collector so index preparation can be held by a test gate independently of rg exit.

Happy-path only: rg exit 0, well-formed stream. Cancellation, errors, empty results, binaries and malformed records are separate issues.

See PRD *Implementation Decisions → Invocation and child arguments* (bullets 7–9), *Result index, records, and stream integrity* (bullets 1–3), and *Module Design → SearchIndex / App*.

### How to verify

- **Manual**:
  1. In a repo, `vrg func .` shows "Searching…" then the summary line; `q` exits 0.
  2. Invoke the built binary by explicit path with an rg-free `PATH` (e.g. `PATH=/nonexistent "$PWD/bin/vrg" foo`) → stderr diagnostic naming the start failure, exit 2. (Do not rely on `PATH` lookup for `vrg` itself, or the shell rather than rg startup is what fails.)
- **Automated**:
  - SearchIndex tests with fixture JSON: `text` and `bytes` paths/lines/submatches; duplicate same-line matches merge into one stop; raw-byte ordering (non-UTF-8 path bytes vs valid UTF-8); relative path resolution against cwd.
  - App model test: injected search-completion message transitions Searching → summary; `q` yields exit 0.
  - Subprocess test with a fake `rg` script that echoes its argv: assert exact child argv and cwd.

### Acceptance criteria

- [ ] Given a valid invocation, when the app starts, then "Searching…" is shown until the stream is fully collected and indexed.
- [ ] Given matches on the same path and line, then they form one navigation stop with submatches sorted by start then end.
- [ ] Given paths supplied as `bytes`, then their raw bytes are retained and ordered by unsigned byte comparison.
- [ ] Given rg cannot be started, then stderr receives a sanitized diagnostic and the process exits 2 without entering the TUI.
- [ ] Given a resize message during searching, then the model handles it without blocking on collection.
- [ ] Given rg has exited but index preparation is still held by a test gate, then the app is still in the searching state ("Searching…" shown; no browse view).

### User stories addressed

- User story 9: clear diagnostic and exit 2 if ripgrep cannot start
- User story 10: "Searching…" screen until collection completes

---

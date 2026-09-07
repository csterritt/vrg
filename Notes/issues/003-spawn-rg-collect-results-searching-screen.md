## Issue 3: Spawn ripgrep, collect JSON into SearchIndex, "Searching…" screen

**Type**: AFK
**Blocked by**: Issue 2

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

The first real Bubble Tea application path: start `rg` with the protected argv from the invocation working directory, show a "Searching…" screen, collect the whole JSON event stream into a SearchIndex, then show an interim summary screen (`N files, M matched lines`) that `q` dismisses with exit 0. Later issues replace the interim screen with browsing.

- Start failure (rg not on PATH, exec error) → sanitized stderr diagnostic, exit 2, no TUI.
- SearchIndex recognises `begin`, `match`, `end`, `summary`, `context` (ignored). Accept both `text` and base64 `bytes` for paths, lines, and submatch text.
- **Per-record schema validation** follows the known-event matrix below, based on the ripgrep 15.x JSON schema. Violations are skipped-and-counted malformed records; the skip/count accounting and disposition tests land with Issue 10, and cross-record lifecycle rules are Issue 9's. Issue 3's parser reads exactly the required fields, and its happy-path fixtures conform to the matrix.

| Event | Required fields (type-checked) | Range constraints | Intentionally ignored |
|---|---|---|---|
| `begin` | `type`; `data.path` (`text` string or `bytes` base64) | — | — |
| `match` | `type`; `data.path`; `data.lines` (`text`/`bytes`); `data.line_number` integer; `data.submatches` non-empty array, each element with `match` (`text`/`bytes`), `start`, `end` integers | `line_number ≥ 1`; `0 ≤ start ≤ end ≤` byte length of the decoded `data.lines`; values fit in int64 | `data.absolute_offset`; any other fields |
| `end` | `type`; `data.path`; `data.binary_offset` present, `null` or integer | `binary_offset ≥ 0` when an integer | — |
| `summary` | `type`; `data` object | — | `data.elapsed_total`, `data.stats`, and all nested fields |
| `context` | `type` | — | the entire `data` payload |

  Rules: invalid JSON, invalid base64, and missing/non-string `type` are malformed; a string `type` outside the five known events is an unknown type (Issue 10's separate count), not malformed. Normalizations rather than violations: submatches are sorted by `(start, end)`; overlapping submatches within one record are retained and highlighted as their union; `text` and `bytes` encodings of the same bytes are the same value. Whether `line_number` refers to an existing line is not a stream validation — it is checked against loaded content by Issue 29's stale guard.
- Retain raw path bytes, line number, submatch byte ranges and recorded bytes. Merge same-path/same-line matches into one stop; submatches ordered by start then end. Index ordered by unsigned raw path bytes then line number.
- Resolve relative result paths against the invocation working directory; do not canonicalize.
- **Dual-pipe drainage from the first spawn.** Both stdout and stderr are pipes, drained concurrently for the whole child lifetime, so neither pipe can fill and block rg. Stderr bytes are buffered (or safely handed off) without allowing backpressure; their classification, overlay presentation, and outcome effects are Issue 9's. Drainage is structured so that terminating the child (Issue 4's cancellation) ends it promptly rather than waiting for further output.
- Collection and processing run off the UI update path; the UI stays responsive (the model handles resize while searching).
- "Searching" covers the whole of collection **and** post-exit processing: parsing, filtering, sorting and index preparation. The app stays in the searching state (and Issue 4's cancellation rules apply) until the index is ready, even after rg itself has exited. Structure the collector so index preparation can be held by a test gate independently of rg exit.

Outcome scope is happy-path only: rg exit 0, well-formed stream, and captured stderr is not yet classified or shown. Cancellation, errors, empty results, binaries and malformed records are separate issues. The child-process foundation above is **not** happy-path-only: the concurrent dual-pipe drainage obligation holds from this issue onward.

See PRD *Implementation Decisions → Invocation and child arguments* (bullets 7–9), *Result index, records, and stream integrity* (bullets 1–3), and *Module Design → SearchIndex / App*.

### How to verify

- **Manual**:
  1. In a repo, `vrg func .` shows "Searching…" then the summary line; `q` exits 0.
  2. Invoke the built binary by explicit path with an rg-free `PATH` (e.g. `PATH=/nonexistent "$PWD/bin/vrg" foo`) → stderr diagnostic naming the start failure, exit 2. (Do not rely on `PATH` lookup for `vrg` itself, or the shell rather than rg startup is what fails.)
- **Automated**:
  - SearchIndex tests with fixture JSON: `text` and `bytes` paths/lines/submatches; duplicate same-line matches merge into one stop; raw-byte ordering (non-UTF-8 path bytes vs valid UTF-8); relative path resolution against cwd.
  - App model test: injected search-completion message transitions Searching → summary; `q` yields exit 0.
  - Subprocess test with a fake `rg` script that echoes its argv: assert exact child argv and cwd.
  - **Dual-pipe backpressure test** (basic handshake/backpressure assertion, owned here): a fake rg writes ≥ 1 MiB to stderr (well over pipe capacity) interleaved with a valid stdout stream, with a handshake confirming it finished writing both before exit. Assert vrg completes without deadlock and the stdout stream is collected completely. Issue 9 reuses this fixture shape for diagnostic-content assertions.

### Acceptance criteria

- [ ] Given a valid invocation, when the app starts, then "Searching…" is shown until the stream is fully collected and indexed.
- [ ] Given matches on the same path and line, then they form one navigation stop with submatches sorted by start then end.
- [ ] Given paths supplied as `bytes`, then their raw bytes are retained and ordered by unsigned byte comparison.
- [ ] Given rg cannot be started, then stderr receives a sanitized diagnostic and the process exits 2 without entering the TUI.
- [ ] Given a resize message during searching, then the model handles it without blocking on collection.
- [ ] Given rg has exited but index preparation is still held by a test gate, then the app is still in the searching state ("Searching…" shown; no browse view).
- [ ] Given rg writes diagnostics to stderr while still exiting 0 with a well-formed stdout stream, then the stderr bytes are captured without blocking the child.
- [ ] Given a fake rg that writes more than pipe capacity to stderr while streaming stdout, then vrg neither deadlocks nor loses the stream.

### User stories addressed

- User story 9: clear diagnostic and exit 2 if ripgrep cannot start
- User story 10: "Searching…" screen until collection completes

---

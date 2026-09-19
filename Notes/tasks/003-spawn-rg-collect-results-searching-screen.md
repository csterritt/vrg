# Tasks for #3: Spawn ripgrep, collect JSON into SearchIndex, "Searching…" screen

Parent issue: #3
Parent PRD: PRD-vrg.md
**Blocked by issues**: #2
**Acceptance criteria**: AC2–AC3 → Tasks 1–2; AC1, AC4–AC8 → Tasks 3–4

## Tasks

### 1. Specify SearchIndex record parsing and indexing

**Type**: RED  
**Output**: Failing tests cover `text`/`bytes` paths, lines, and submatches; same-line merging including mixed-encoding same-value records; submatch ordering and overlapping-submatch union coverage; unsigned raw-byte path ordering; relative-path resolution against the working directory; and happy-path fixtures conforming to the per-record schema matrix.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issue #2 is complete. Add table-driven tests in `internal/searchindex` for the Issue #3 record and index contracts and the Result index, records, and stream integrity section of `Notes/PRD-vrg.md`. Drive fixture JSON through the index for `begin`, `match`, `end`, `summary`, and ignored `context` events, accepting both `text` and base64 `bytes` encodings for paths, lines, and submatch text; require same-path/same-line matches to merge into one navigation stop with submatches sorted by start then end; require mixed-encoding records — a `text` record and a `bytes` record carrying the same bytes for the same path and line — to merge into that one stop as the same value, proving cross-encoding path identity rather than inferring it from separate encoding fixtures; and require overlapping submatches within one record to be retained, with the prepared highlight coverage asserted as their union without dropping either original range; require retained raw path bytes, line numbers, submatch byte ranges, and recorded submatch bytes; require index ordering by unsigned raw path bytes then ascending line number with non-UTF-8 path bytes ordered deterministically against valid UTF-8; require relative result paths resolved against the invocation working directory without canonicalization; and conform every happy-path fixture to the issue's per-record schema matrix including its range constraints. Keep this task test-only.

---

### 2. Implement SearchIndex parsing and index preparation

**Type**: GREEN  
**Output**: Record parsing, stop merging, ordering, and path-resolution tests pass.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement only enough of `internal/searchindex` to satisfy Task 1: typed record decoding for both JSON encodings, per-record validation of exactly the required fields from the issue's matrix, stop merging and submatch ordering, raw byte retention, and result-path resolution against the supplied working directory. Keep parsing separated from the cross-record lifecycle and integrity accounting owned by Issue #9 and from skip/count handling owned by Issue #10, per the issue's scope notes.

---

### 3. Specify the search lifecycle and dual-pipe drainage

**Type**: RED  
**Output**: Failing App model tests cover the searching screen, gate-held post-exit preparation, completion to the interim summary, `q` exit 0, resize during search, and start failure; failing subprocess tests assert the exact child argv and working directory and deadlock-free dual-pipe drainage.  
**Depends on**: 2

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Add failing model tests in `internal/app` and subprocess tests for the remaining Issue #3 contracts. Require "Searching…" to cover collection and post-exit processing, with a test gate able to hold index preparation independently of rg exit so the app stays in the searching state even after rg has exited; an injected search-completion message to transition to the interim summary screen (`N files, M matched lines`) with `q` exiting 0; resize messages handled without blocking on collection; and rg start failure to produce a sanitized stderr diagnostic and exit 2 without entering the TUI. Add subprocess tests with a fake `rg` that echoes its argv, asserting the exact child argv and the invocation working directory, and the dual-pipe backpressure fixture: a fake rg writing at least 1 MiB to stderr — well over pipe capacity — interleaved with a valid stdout stream, with a handshake confirming it finished writing both before exit, asserting vrg neither deadlocks nor loses the stdout stream. Keep this task test-only.

---

### 4. Implement the rg spawn, dual-pipe drainage, and searching screen

**Type**: GREEN  
**Output**: Lifecycle, start-failure, argv, and backpressure tests pass; "Searching…" shows until the index is ready.  
**Depends on**: 3

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the spawn and collection path in `internal/app` and its narrow process seams: start rg with the protected argv from the invocation working directory, pipe and concurrently drain both stdout and stderr for the whole child lifetime with stderr buffered safely so neither pipe can block rg, structure drainage so terminating the child ends it promptly, run collection and index preparation off the UI update path with the test-gate hold Task 3 requires, keep the model responsive to resize while searching, transition to the interim summary on completion with `q` exiting 0, and map start failure to the sanitized diagnostic and exit 2. Stderr classification and outcome effects are owned by Issue #9; cancellation and cleanup by Issue #4 — do not implement them here.

---

### 5. Create the search-collection finish marker

**Type**: FINISH MARKER  
**Output**: Finish marker exists at `Notes/finish-markers/003-06/finish-marker.md`.  
**Depends on**: 4

Write `Task 003-06 finished successfully at <time>` to `Notes/finish-markers/003-06/finish-marker.md`, replacing `<time>` with the current UTC timestamp (for example, `date -u +"%Y-%m-%dT%H:%M:%SZ"`). Create the `Notes/finish-markers/003-06/` directory if it does not already exist.

---

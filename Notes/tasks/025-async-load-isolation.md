# Tasks for #25: Asynchronous load isolation — navigate during load, late results update only their own file

Parent issue: #25
Parent PRD: PRD-vrg.md
**Blocked by issues**: #13, #16
**Acceptance criteria**: AC1–AC5 → Tasks 1–2
**Manual verification**: Task 4 owns the issue's manual checks.

## Tasks

### 1. Specify asynchronous load isolation

**Type**: RED  
**Output**: Failing gated model tests cover navigation during loads, keyed late completions for non-current files, the one-load-per-path rule, cached revisits, post-cancellation rejection, and input responsiveness during the decode/map phase.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issues #13 and #16 are complete. Add failing model tests in `internal/app` with worker gates for the Issue #25 contracts and the first three File loading bullets of `Notes/PRD-vrg.md`. Require navigation to remain fully active while a file loads so the user can move past a slow file, with placeholder scrolling a no-op and other keys keeping their normal meanings; load completions keyed by raw path and request identity to update only that path's cache and status, affecting the visible panel only when that path is current — A→B→C navigation with A's completion arriving while C is current leaves C's panel unchanged and A cached; successful buffers to be retained for the session with no eviction; at most one load in flight per raw path so re-entering a loading path starts nothing and queues nothing; late messages after cancellation to be ignored; and the decode/map phase of the current file's load to be separately gatable with `ctrl+c`, `n`/`p`, `w`, `c`, and resize all actionable while it is held. Keep this task test-only.

---

### 2. Implement keyed load isolation

**Type**: GREEN  
**Output**: Gated navigation, isolation, one-load-per-path, cache, and responsiveness tests pass.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the keyed load machinery in `internal/app` to satisfy Task 1: path and request identity on completion messages, per-path cache and status updates isolated from the current panel, the one-load-per-path rule with dropped re-entry requests, session-long retention of successful buffers, rejection of post-cancellation completions, and prepared buffers in completion messages so `Update` never decodes or maps a full file. Load-completion reveal semantics are owned by Issue #28 and failures by Issue #26.

---

### 3. Document asynchronous load isolation

**Type**: DOCUMENT  
**Output**: Wiki documentation records keyed completions, the one-load rule, session caching, and the responsiveness contract.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #25 implementation and tests into the appropriate pages under `Notes/wiki`. Document navigation during loads, path-and-request-identity keying with panel isolation, the one-load-in-flight-per-path rule with dropped re-entry, session-long buffer retention with no eviction, post-cancellation rejection, and the separately gated decode/map phase with its actionable-input list. Cross-reference Issue #25 and the File loading, cache, reload, and selection consistency section of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the load-isolation walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/025-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/025-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the gated model tests, then run the manual case: a very large first file A and small second file B, pressing `n` immediately at startup so B shows while A still loads, then `p` so A shows content only once loaded, never before. Reference Issue #25 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---

# Tasks for #11: Stderr replay of all collected diagnostics after terminal restore

Parent issue: #11
Parent PRD: PRD-vrg.md
**Blocked by issues**: #4, #6, #9
**Acceptance criteria**: AC1–AC2, AC4–AC5 → Tasks 1–2; AC3, AC6 → Tasks 3–4
**Manual verification**: Task 6 owns the issue's manual checks.

## Tasks

### 1. Specify the session diagnostic collection and replay

**Type**: RED  
**Output**: Failing model tests cover ordered exactly-once replay including never-displayed diagnostics, the shutdown boundary, exactly-once across the direct-write and replay mechanisms, and escaped single-lined embedded filenames.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issues #4, #6, and #9 are complete. Add failing model tests in `internal/app` for the Issue #11 contracts and the replay bullet of the Colours, overlays, and key precedence section of `Notes/PRD-vrg.md`. Maintain a session diagnostic collection independent of what was displayed; require three collected diagnostics — one displayed in an overlay and two never displayed — to reach the replay writer exactly once each, in collection order, on a normal exit; require a diagnostic delivered and processed before a `ctrl+c` in the next update to be replayed, while a gated, undelivered diagnostic is not waited for and not replayed; require the Issue #4 controlled-failure diagnostic to enter the collection before shutdown and be replayed by the common post-restoration writer with no separate direct write, so exactly-once holds across both mechanisms; and require a diagnostic embedding a filename with `\n` and ESC to be escaped and single-lined through the Issue #6 utility, adding the replay writer to the sink-safety table. Keep this task test-only.

---

### 2. Implement the collection and replay writer

**Type**: GREEN  
**Output**: Collection, shutdown-boundary, and exactly-once tests pass; replay runs after terminal restoration on every controlled exit.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the session collection, the replay writer as part of the Issue #4 cleanup sequence running after the PTY and display restoration step without waiting on unrelated in-flight work, and the shutdown boundary where a diagnostic is collected once the model has processed the message carrying it. Route the controlled-failure diagnostic through the collection instead of the direct write so the Issue #4 writer serves every controlled exit. Write no persistent log.

---

### 3. Specify PTY replay ordering and acknowledgement

**Type**: RED  
**Output**: Failing PTY tests cover the application-side acknowledgement before the exit keypress, replay after the display-restoration sequence with termios already restored, exactly-once alongside earlier diagnostics in collection order, and the escaped filename case.  
**Depends on**: 2

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Add failing PTY tests on the Issue #4 harness for the process-level Issue #11 contracts. Add a test-only application-side acknowledgement emitted once a diagnostic has been processed into the session collection, in the same mechanism family as the reap-evidence side channel, and require the test to wait for that acknowledgement — not a child-side write handshake, which proves only that bytes reached the pipe — before sending the exit key. Cover `ctrl+c` after a stderr diagnostic → exit 130 with termios restored and the line on vrg's stderr after the display-restoration sequence exactly once; normal `q` after a completed stream with a stderr warning → the same ordering; the injected controlled failure → its diagnostic exactly once after restoration alongside any earlier diagnostics in collection order, counted across both mechanisms with no duplicate; and a diagnostic embedding a filename with `\n` and ESC escaped and single-lined in the replayed text. Keep this task test-only.

---

### 4. Implement the acknowledgement and replay integration

**Type**: GREEN  
**Output**: PTY replay-ordering tests pass with the acknowledgement side channel in place.  
**Depends on**: 3

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the application-side acknowledgement side channel gated for tests only, and integrate the replay writer into the process boundary so replayed bytes appear on stderr strictly after the display-restoration sequence with input modes already restored, on normal completion, cancellation, and controlled failure alike.

---

### 5. Document the stderr replay contract

**Type**: DOCUMENT  
**Output**: Wiki documentation records the session collection, shutdown boundary, replay ordering, and exactly-once rules.  
**Depends on**: 4

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #11 implementation and tests into the appropriate pages under `Notes/wiki`. Document the session diagnostic collection independent of display, the shutdown boundary for processed versus in-flight diagnostics, replay after terminal restoration in collection order and exactly once per occurrence, the controlled-failure unification with the Issue #4 writer, filename escaping in replayed text, and the acknowledgement side channel. Cross-reference Issue #11 and the Colours, overlays, and key precedence and Outcome and exit-status contract sections of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 6. Create the stderr-replay walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/011-06/code-walkthrough`.  
**Depends on**: 5

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/011-06/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the model collection and boundary tests and the PTY acknowledgement and ordering tests, then run the manual case: a fake rg emitting stderr "warn one" with a valid stream, browsing, `q`, and the shell showing "warn one" exactly once after the TUI closes. Reference Issue #11 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---

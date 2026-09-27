# Tasks for #46: Bubble Tea runtime errors go through the common shutdown/diagnostic-replay path

Parent issue: #46
Parent PRD: PRD-vrg.md
**Blocked by issues**: #45 — shared-file/harness ordering with #41 and #48 in `cmd/vrg`: never implement overlapping PTY work concurrently; whichever issue lands second adapts to the already-landed `outcome_test.go` changes and handshake helpers. If #48 has landed, these tests must use its acknowledgement harness with no fixed settling/inter-key delay, and any new key-sending helper must be added to #48's matrix and every new acknowledgement hook it requires to #45's hook manifest; if #46 lands first, #48's reciprocal adaptation clause applies
**Acceptance criteria**: AC1–AC5 → Tasks 1–2
**Manual verification**: Task 4 owns the issue's manual checks.

## Tasks

### 1. Specify the unified shutdown contract for every `Run()` return shape

**Type**: RED  
**Output**: Failing subprocess/PTY tests inject the three return shapes through the Issue #45 tagged runner at the real `program.Run()` boundary, each asserting terminal restoration, the ordered replay (session diagnostics, then invalid-final-model diagnostic, then the runtime error exactly once), and exit 2.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issue #45 is complete — the return-shape injection seam is its dedicated `vrg_testhooks` program-runner boundary; app options alone cannot substitute `program.Run()` results. Respect the header's ordering rule with Issues #41/#48: if #48 has landed, use its per-occurrence acknowledgement harness and add no fixed settling/inter-key delay; register any new key-sending helper in #48's finite matrix and every new acknowledgement hook it requires in #45's explicit hook manifest, and adapt around #41's `outcome_test.go` changes if present. Add failing subprocess/PTY tests in `cmd/vrg` driving `VRG_TEST_RUN_FINAL_MODEL`/`VRG_TEST_RUN_ERROR` through a real PTY lifecycle in which diagnostics were collected before the injected return, covering the full matrix: (1) valid final model + `Run()` error → collected diagnostics replayed to stderr in collection order after terminal restoration, the runtime error appended exactly once, exit 2; (2) invalid/nil final model + `Run()` error → retained session diagnostics replayed, then a diagnostic naming the invalid-final-model condition, then the runtime error once, exit 2; (3) invalid/nil final model + nil `Run()` error → retained session diagnostics then the invalid-final-model diagnostic, exit 2 — never a silent or zero exit. Each test must prove the injected tuple reached the executable's actual post-`Run()` type/error branches rather than merely causing a controlled model quit, assert the child was terminated/reaped and the terminal restored exactly as on normal exits, and assert no diagnostic is emitted both by a direct write and the replay. Keep this task test-only.

---

### 2. Implement the diagnostic snapshot and unified shutdown sequence

**Type**: GREEN  
**Output**: The return-shape tests pass; `runSearch` recovers session diagnostics independently of the final-model assertion and routes every `Run()` return shape through the single ordered shutdown/replay sequence, exiting 2 on every failing shape.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Introduce a shutdown result / diagnostic snapshot in `cmd/vrg/main.go` and `internal/app` that survives independently of the final-model type assertion — e.g. a collector owned by `runSearch` that the model appends diagnostics to as they are collected, in the same option family as `WithOnCollect` — so session diagnostics reach stderr even when `finalModel` is nil or has the wrong type; the assertion must not be the only channel through which collected diagnostics reach stderr. Route every `Run()` return shape through one implementable shutdown sequence: `Run()` returns after Bubble Tea has restored the terminal; `runSearch` then terminates and reaps the child exactly as on normal exits; only after both restoration and cleanup complete does it replay — session diagnostics retained in the snapshot in collection order, then a diagnostic for an absent or wrong-type final model when applicable (never a silent exit), then the `program.Run()` runtime error appended exactly once with no duplicate emission from a direct write plus the replay. Cleanup may also complete inside the model before `Run()` returns, but replay must still wait until `Run()` has returned and both terminal restoration and cleanup are complete. Every failing `Run()` return shape is a controlled application failure exiting 2, matching the existing startup-failure convention. Run the focused subprocess tests plus `go build ./...`, `go vet ./...`, and `go test ./...` in both build variants.

---

### 3. Document the unified runtime-error path

**Type**: DOCUMENT  
**Output**: Wiki documentation records the diagnostic snapshot, the single shutdown sequence, and the three return-shape outcomes.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #46 implementation into the appropriate pages under `Notes/wiki`. Document the collector that decouples session diagnostics from the final-model assertion, the ordered shutdown sequence (terminal restoration → child termination/reap → replay of session diagnostics → invalid-final-model diagnostic → the runtime error exactly once), the exit-2 controlled-failure convention extended to runtime errors, and the tagged-runner injection used by the tests. Cross-reference Issue #46 and the *Outcome and exit-status contract* section of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the runtime-error walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/046-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/046-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the three return-shape subprocess tests, then run the issue's manual scenario: use the Issue #45 tagged program-runner control to return an error after diagnostics were collected in a real PTY lifecycle → the process exits 2, the terminal is restored, and stderr contains the session's collected diagnostics in order followed once by the application error. Capture commands, outputs, and exit statuses. Reference Issue #46 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---

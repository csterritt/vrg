## Issue 46: Bubble Tea runtime errors go through the common shutdown/diagnostic-replay path

**Type**: AFK
**Blocked by**: Issue 45 — the return-shape injection seam the automated verification needs is the dedicated program-runner boundary in that issue's `vrg_testhooks` topology; app options alone cannot substitute `program.Run()` results, and landing this first would either re-add production hooks or invent a competing harness. Shared-file/harness ordering with Issues 41 and 48: never implement overlapping `cmd/vrg` PTY work concurrently; whichever issue lands second adapts its tests to the already-landed `outcome_test.go` changes and handshake helpers. If Issue 48 has landed, this issue must use its acknowledgement harness and add no fixed settling/inter-key delay; any new key-sending helper must be added to Issue 48's matrix and every new acknowledgement hook it requires to Issue 45's hook manifest. If this issue lands first, Issue 48's reciprocal adaptation rule applies.

### Parent PRD

`Notes/PRD-vrg.md` — audit source: `Notes/critiques/final-audit-vrg.md`, Medium finding 11

### What to build

Unify the `program.Run()` error path with the common shutdown contract (`cmd/vrg/main.go:169-207`). When `Run()` returns an error, the code currently writes that error directly and returns before reading the final model or replaying diagnostics the session already collected — bypassing the post-restoration, in-order replay contract every other controlled failure follows. A failed final-model type assertion also exits silently.

- Introduce a **shutdown result / diagnostic snapshot** that survives independently of the final-model type assertion — e.g. a collector owned by `runSearch` that the model appends diagnostics to as they are collected (in the same option family as `WithOnCollect`), so session diagnostics are recoverable even when `finalModel` is nil or has the wrong type. The final-model assertion must not be the only channel through which collected diagnostics reach stderr.
- Consume Issue 45's dedicated build-constrained program-runner seam, not its app-option seam, to inject the test matrix. The tagged runner must return each requested final-model/error shape from the same call site that production uses; subprocess tests must prove they reached the executable's actual post-`Run()` type/error branches rather than merely causing a controlled model quit.
- Route every `Run()` return shape through a single implementable shutdown sequence: `Run()` returns after Bubble Tea has restored the terminal; `runSearch` then terminates/reaps the child as needed; only after both restoration and cleanup complete does it replay diagnostics to stderr in this exact order:
  1. Session diagnostics retained in the snapshot, in collection order.
  2. A diagnostic for an absent or wrong-type final model, when applicable — never a silent exit.
  3. The `program.Run()` runtime error itself, appended exactly once — no duplicate emission from a direct write *and* the replay.
  Cleanup may also complete inside the model before `Run()` returns, but replay must still wait until `Run()` has returned and both terminal restoration and cleanup are complete.
- Every failing `Run()` return shape is a controlled application failure: **exit 2**, matching the existing startup-failure convention.

See PRD *Outcome and exit-status contract* (diagnostic collection and replay, cleanup on application failures) and user stories 22–23.

### How to verify

- **Manual**: use Issue 45's tagged program-runner control to return an error after diagnostics were collected in a real PTY lifecycle → the process exits 2, the terminal is restored, and stderr contains the session's collected diagnostics in order followed once by the application error.
- **Automated**: subprocess/PTY tests covering the full return-shape matrix, each asserting terminal restoration, replay order, exactly-once emission, and exit status 2:
  1. valid final model + `Run()` error → collected diagnostics replayed in order, runtime error appended once;
  2. invalid/nil final model + `Run()` error → retained session diagnostics replayed, then the invalid-final-model diagnostic, then the runtime error once;
  3. invalid/nil final model + nil `Run()` error → retained session diagnostics replayed, then the invalid-final-model diagnostic, exit 2 rather than a silent or zero exit.

### Acceptance criteria

- [ ] Given `program.Run()` returns an error after diagnostics were collected, then those diagnostics are replayed to stderr in collection order after terminal restoration, with the application error appended exactly once, and the process exits 2.
- [ ] Given a runtime error, then the terminal is restored and the child is terminated/reaped exactly as on normal exits.
- [ ] Given an invalid or missing final model — with or without a `Run()` error — then session diagnostics are still replayed and a diagnostic names the invalid-final-model condition, instead of a silent exit; the process exits 2.
- [ ] Given the unified path, then no diagnostic is duplicated between direct writes and replay, and every return shape produces exactly one deterministic replay sequence.
- [ ] Given each of the three return-shape cases, then the tagged runner injects that tuple at the real executable `Run()` boundary, the corresponding post-`Run()` branch is proven to execute, and the exit status is 2 — non-zero and consistent with the existing controlled-failure contract.

### User stories addressed

- User story 22: all collected diagnostics safely replayed to stderr after terminal restoration
- User story 23: every exit restores the terminal and terminates/reaps a running ripgrep child
- User story 9: clear stderr diagnostic and exit 2 on startup failures (extended uniformly to runtime failures)

---

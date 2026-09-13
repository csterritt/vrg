## Issue 46: Bubble Tea runtime errors go through the common shutdown/diagnostic-replay path

**Type**: AFK
**Blocked by**: Issue 45 — the runtime-error injection seam the automated verification needs belongs to the test-only harness that issue installs (`vrg_testhooks` build); landing this first would either re-add production hooks or invent a competing harness

### Parent PRD

`Notes/PRD-vrg.md` — audit source: `Notes/critiques/final-audit-vrg.md`, Medium finding 11

### What to build

Unify the `program.Run()` error path with the common shutdown contract (`cmd/vrg/main.go:169-207`). When `Run()` returns an error, the code currently writes that error directly and returns before reading the final model or replaying diagnostics the session already collected — bypassing the post-restoration, in-order replay contract every other controlled failure follows. A failed final-model type assertion also exits silently.

- Introduce a **shutdown result / diagnostic snapshot** that survives independently of the final-model type assertion — e.g. a collector owned by `runSearch` that the model appends diagnostics to as they are collected (in the same option family as `WithOnCollect`), so session diagnostics are recoverable even when `finalModel` is nil or has the wrong type. The final-model assertion must not be the only channel through which collected diagnostics reach stderr.
- Route every `Run()` return shape through a single shutdown sequence: terminate/reap the child, let Bubble Tea restore the terminal, then replay diagnostics to stderr in this exact order:
  1. Session diagnostics retained in the snapshot, in collection order.
  2. A diagnostic for an absent or wrong-type final model, when applicable — never a silent exit.
  3. The `program.Run()` runtime error itself, appended exactly once — no duplicate emission from a direct write *and* the replay.
- Every `Run()` failure shape is a controlled application failure: **exit 2**, matching the existing startup-failure convention.

See PRD *Outcome and exit-status contract* (diagnostic collection and replay, cleanup on application failures) and user stories 22–23.

### How to verify

- **Manual**: force a Bubble Tea runtime error in a PTY run via the Issue 45 test-only harness (an injected program failure after diagnostics were collected) → the process exits 2, the terminal is restored, and stderr contains the session's collected diagnostics in order followed once by the application error.
- **Automated**: subprocess/PTY tests covering the full return-shape matrix, each asserting terminal restoration, replay order, exactly-once emission, and exit status 2:
  1. valid final model + `Run()` error → collected diagnostics replayed in order, runtime error appended once;
  2. invalid/nil final model + `Run()` error → retained session diagnostics replayed, then the invalid-final-model diagnostic, then the runtime error once;
  3. invalid/nil final model + nil `Run()` error → retained session diagnostics replayed, then the invalid-final-model diagnostic, exit 2 rather than a silent or zero exit.

### Acceptance criteria

- [ ] Given `program.Run()` returns an error after diagnostics were collected, then those diagnostics are replayed to stderr in collection order after terminal restoration, with the application error appended exactly once, and the process exits 2.
- [ ] Given a runtime error, then the terminal is restored and the child is terminated/reaped exactly as on normal exits.
- [ ] Given an invalid or missing final model — with or without a `Run()` error — then session diagnostics are still replayed and a diagnostic names the invalid-final-model condition, instead of a silent exit; the process exits 2.
- [ ] Given the unified path, then no diagnostic is duplicated between direct writes and replay, and every return shape produces exactly one deterministic replay sequence.
- [ ] Given each of the three return-shape cases, then the exit status is 2 — non-zero and consistent with the existing controlled-failure contract.

### User stories addressed

- User story 22: all collected diagnostics safely replayed to stderr after terminal restoration
- User story 23: every exit restores the terminal and terminates/reaps a running ripgrep child
- User story 9: clear stderr diagnostic and exit 2 on startup failures (extended uniformly to runtime failures)

---

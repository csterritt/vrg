## Issue 46: Bubble Tea runtime errors go through the common shutdown/diagnostic-replay path

**Type**: AFK
**Blocked by**: None — can start immediately

### Parent PRD

`Notes/PRD-vrg.md` — audit source: `Notes/critiques/final-audit-vrg.md`, Medium finding 11

### What to build

Unify the `program.Run()` error path with the common shutdown contract (`cmd/vrg/main.go:169-207`). When `Run()` returns an error, the code currently writes that error directly and returns before reading the final model or replaying diagnostics the session already collected — bypassing the post-restoration, in-order replay contract every other controlled failure follows. A failed final-model type assertion also exits silently.

- Route runtime failure through a single shutdown result that retains the latest model's collected diagnostics, appends the application failure exactly once, performs terminal restore/cleanup, and then runs the common in-order stderr replay.
- A failed or absent final-model type assertion produces its own diagnostic rather than exiting silently.
- The runtime error itself is reported once — no duplicate emission from both the direct write and the replay.

See PRD *Outcome and exit-status contract* (diagnostic collection and replay) and user stories 22–23.

### How to verify

- **Manual**: force a Bubble Tea runtime error in a PTY test run (via the test harness, e.g. an injected program failure after diagnostics were collected) → the process exits non-zero, the terminal is restored, and stderr contains the session's collected diagnostics in order followed once by the application error.
- **Automated**: a subprocess/PTY test asserting that a `Run()` error after collected diagnostics produces: terminal restored, diagnostics replayed in collection order, the runtime error appended exactly once, correct exit status; plus a case where the final model is unavailable/invalid asserting the invalid-final-model diagnostic is emitted rather than silence.

### Acceptance criteria

- [ ] Given `program.Run()` returns an error after diagnostics were collected, then those diagnostics are replayed to stderr in order after terminal restoration, with the application error appended once.
- [ ] Given a runtime error, then the terminal is restored and the child is terminated/reaped exactly as on normal exits.
- [ ] Given an invalid or missing final model, then a diagnostic is emitted for that condition instead of a silent exit.
- [ ] Given the unified path, then no diagnostic is duplicated between direct writes and replay.
- [ ] Given the runtime-error exit, then the exit status remains non-zero and distinct per the existing failure contract.

### User stories addressed

- User story 22: all collected diagnostics safely replayed to stderr after terminal restoration
- User story 23: every exit restores the terminal and terminates/reaps a running ripgrep child
- User story 9: clear stderr diagnostic and exit 2 on startup failures (extended uniformly to runtime failures)

---

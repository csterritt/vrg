## Issue 11: Stderr replay of all collected diagnostics after terminal restore

**Type**: AFK
**Blocked by**: Issue 4, Issue 6, Issue 9

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

Maintain a session diagnostic collection independent of what was displayed. On every controlled exit (normal completion, cancellation, controlled application failure from Issue 4), after terminal restoration, replay each collected occurrence exactly once, in collection order, to sanitized stderr.

- Includes diagnostics never shown in an overlay (unknown-type warnings, later non-current load failures from Issue 26, anything collected just before exit).
- Preserve diagnostic line boundaries; embedded filenames are single-line escaped (Issue 6 utility). Add the replay writer to the Issue 6 sink-safety table.
- Replay is part of the Issue 4 cleanup sequence and runs **after** the PTY/display restoration step; it does not delay exit waiting for unrelated in-flight work.
- Define the shutdown boundary: a diagnostic is "collected" once the model has processed the message carrying it. A diagnostic message that is processed before the exit decision is replayed; one still in flight at exit is not waited for.
- No persistent log is written.

See PRD *Colours, overlays, and key precedence* (last bullet) and *Outcome and exit-status contract* (cleanup bullet).

### How to verify

- **Manual**: fake rg emitting stderr "warn one" and a valid stream → browse; `q`; the shell shows "warn one" on stderr after the TUI closes, and only once.
- **Automated**:
  - Model test collecting three diagnostics (one displayed, two never displayed) and asserting the replay writer receives exactly those three, in order, once each.
  - **Shutdown-boundary model test**: a diagnostic message is delivered and processed, then `ctrl+c` in the *next* update → the diagnostic is replayed. Conversely, a gated diagnostic that has not been delivered when `ctrl+c` is processed is not waited for and not replayed.
  - **PTY subprocess tests** (Issue 4 harness): (a) fake rg writes a stderr line, then blocks; vrg emits a **test-only application-side acknowledgement** once the diagnostic has been processed into the session collection (same mechanism family as Issue 4's reap evidence, e.g. an environment-variable-gated side channel). The test waits for that acknowledgement — not merely for a child-side write handshake, which proves only that bytes reached the pipe — and only then sends `ctrl+c` → exit 130, PTY termios restored, and the stderr line appears in vrg's stderr *after* the display-restoration sequence, exactly once. (b) Normal `q` after a completed stream with a stderr warning → same ordering (acknowledgement before the keypress). (c) Injected controlled failure (Issue 4 hook) → replay still occurs. (d) A diagnostic embedding a filename with `\n` and ESC is escaped and single-lined in the replayed text.

### Acceptance criteria

- [ ] Given collected diagnostics, when the program exits normally, then each is written once to stderr, in collection order, after terminal restoration.
- [ ] Given a diagnostic that was never shown in an overlay, then it is still replayed.
- [ ] Given cancellation via `ctrl+c` or `q`-while-searching, then diagnostics collected before the cancel keypress was processed are replayed, and exit does not wait on undelivered work.
- [ ] Given a controlled application failure, then collected diagnostics are replayed after terminal restoration.
- [ ] Given a diagnostic embedding a filename with control bytes, then the replayed text is escaped and single-lined for the filename.
- [ ] Given the PTY harness, then the replay bytes appear after the display-restoration sequence and the PTY input modes are already restored when they appear.
- [ ] Given a diagnostic whose collection has been acknowledged application-side, when `ctrl+c` is processed afterwards, then that diagnostic is replayed exactly once.

### User stories addressed

- User story 22: all collected diagnostics safely replayed to stderr

---

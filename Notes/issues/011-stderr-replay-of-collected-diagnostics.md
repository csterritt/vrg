## Issue 11: Stderr replay of all collected diagnostics after terminal restore

**Type**: AFK
**Blocked by**: Issue 9

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

Maintain a session diagnostic collection independent of what was displayed. On every controlled exit (normal completion, cancellation, application failure under vrg's control), after terminal restoration, replay each collected occurrence exactly once, in collection order, to sanitized stderr.

- Includes diagnostics never shown in an overlay (unknown-type warnings, later non-current load failures from Issue 26, anything collected just before exit).
- Preserve diagnostic line boundaries; embedded filenames are single-line escaped (Issue 6 utility).
- Do not delay exit waiting for unrelated in-flight work.
- No persistent log is written.

See PRD *Colours, overlays, and key precedence* (last bullet) and *Outcome and exit-status contract* (cleanup bullet).

### How to verify

- **Manual**: fake rg emitting stderr "warn one" and a valid stream → browse; `q`; the shell shows "warn one" on stderr after the TUI closes, and only once.
- **Automated**: model test collecting three diagnostics (one displayed, two never displayed) and asserting the replay writer receives exactly those three, in order, once each; PTY subprocess test asserting the replay appears after the terminal-restore sequence and that a filename with `\n` is escaped; `ctrl+c` path also replays.

### Acceptance criteria

- [ ] Given collected diagnostics, when the program exits normally, then each is written once to stderr, in collection order, after terminal restoration.
- [ ] Given a diagnostic that was never shown in an overlay, then it is still replayed.
- [ ] Given cancellation via `ctrl+c`, then collected diagnostics are still replayed.
- [ ] Given a diagnostic embedding a filename with control bytes, then the replayed text is escaped and single-lined for the filename.

### User stories addressed

- User story 22: all collected diagnostics safely replayed to stderr

---

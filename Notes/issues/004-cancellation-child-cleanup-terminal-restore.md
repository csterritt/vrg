## Issue 4: Cancellation (`q` while searching, `ctrl+c` anywhere), child termination/reap, terminal restore, controlled-failure cleanup

**Type**: AFK
**Blocked by**: Issue 3

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

- `ctrl+c` in any state and `q` while searching/result preparation is incomplete: cancel outstanding work, terminate and reap the rg child, restore the terminal, exit 130 with no further screen. "Incomplete" includes the post-exit sorting/index-preparation phase from Issue 3: `q` pressed after rg has exited but before the index is ready is cancellation (130), not a browse quit.
- Every ordinary exit path (including the interim `q` from Issue 3) restores the terminal and terminates/reaps a still-running child.
- **Controlled application failure after the child has started** (terminal/TUI startup failure, a Bubble Tea program error, an internal panic caught by vrg) also performs cleanup: terminate/reap the child, restore the terminal, then write a sanitized diagnostic to stderr **exactly once, after terminal restoration, through a single post-restoration stderr writer** — at this issue the writer covers just the controlled-failure diagnostic; Issue 11 generalizes it into the session-collection replay. The failure path must never both write the diagnostic directly and also hand it to a collection/replay mechanism (exactly-once across mechanisms). Exit status 2. This is the App contract's "application failures under vrg's control"; OS-forced termination and OOM are out of scope. Provide an injectable failure hook so tests can trigger it deterministically.
- Terminal restoration means **both** the display (leave alternate screen, cursor visible) and the **PTY input modes** (raw/no-echo undone). Both must hold on normal quit, cancellation, and injected controlled failure.
- Late search-completion messages after cancellation must not revive the UI.
- `Esc` during searching is a no-op.
- Build the **subprocess boundary test harness**: a controllable fake `rg` with explicit readiness/completion handshakes (e.g. writes a ready file / waits on a fifo), run under a controlled terminal/PTY so tests can assert actual child termination and terminal restoration rather than just that a cancel command was emitted. The harness must expose (a) that the child is gone **and** (b) that vrg's wait/reap path ran — for example the fake rg's exit is observed via vrg's own reporting/logging hook or a test-only environment variable that makes vrg print the reaped wait status to a side channel — rather than treating "no such PID" as proof of reaping. Later issues (9, 11) reuse this harness.

See PRD *Outcome and exit-status contract* (rows 1–2 and the cleanup bullet), *Colours, overlays, and key precedence* (`ctrl+c` precedence), *Module Design → App* (failure modes), and *Testing Decisions → Subprocess boundary / Responsiveness boundaries*.

### How to verify

- **Manual**:
  1. Run `vrg . /` (long search); press `q` while "Searching…" → prompt returns, `echo $?` is 130, `pgrep rg` shows no orphan.
  2. Same with `ctrl+c`.
  3. Terminal is usable afterwards: no alt-screen residue, cursor visible, typed characters echo, `stty -a` shows cooked mode (`icanon`, `echo`), i.e. identical to `stty -a` before the run.
- **Automated** (PTY harness):
  - Fake rg signals ready and then blocks; send `q`; assert exit 130, the fake rg process is gone, vrg's wait/reap evidence is present, the display-restoration sequence was written, and the PTY's termios after exit equals the termios captured before launch.
  - Same for `ctrl+c`.
  - Fake rg emits a complete valid stream and exits, with index preparation held by a test gate; send `q` → exit 130 and cancellation cleanup, not the Issue 3 summary/browse quit.
  - Injected controlled failure (via the failure hook) after the fake rg has signalled ready → child gone and reaped, PTY termios restored, a sanitized diagnostic on stderr appearing exactly once and after the display-restoration sequence, exit 2.
  - Model test: `Esc` during searching produces no state change.
  - Model test: late search-completion messages arriving after cancellation do not revive the UI.

### Acceptance criteria

- [ ] Given searching is in progress, when `q` is pressed, then the child is terminated and reaped, the terminal restored, and the exit status is 130.
- [ ] Given rg has exited but sorting/index preparation has not completed, when `q` is pressed, then the exit status is 130 (cancellation), not a browse quit.
- [ ] Given any state, when `ctrl+c` is pressed, then the exit status is 130 after cleanup.
- [ ] Given a normal exit while rg still runs, then no rg process remains and vrg has waited on it.
- [ ] Given a controlled application failure after the child started, then the child is terminated and reaped, the terminal is restored, a sanitized diagnostic is written to stderr exactly once after terminal restoration, and the exit status is 2.
- [ ] Given any of the above exits under the PTY harness, then the PTY input modes after exit equal those before launch and the display-restoration sequence was emitted.
- [ ] Given cancellation, when a late completion message arrives, then the UI is not revived.
- [ ] Given the searching screen, when `Esc` is pressed, then nothing happens.

### User stories addressed

- User story 11: `q` during searching or `ctrl+c` anywhere cancels and exits 130
- User story 23: every normal exit restores the terminal and reaps the child

---

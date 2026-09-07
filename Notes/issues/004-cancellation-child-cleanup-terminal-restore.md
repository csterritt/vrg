## Issue 4: Cancellation (`q` while searching, `ctrl+c` anywhere), child termination/reap, terminal restore

**Type**: AFK
**Blocked by**: Issue 3

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

- `ctrl+c` in any state and `q` while searching/result preparation is incomplete: cancel outstanding work, terminate and reap the rg child, restore the terminal, exit 130 with no further screen.
- Every ordinary exit path (including the interim `q` from Issue 3) restores the terminal and terminates/reaps a still-running child.
- `Esc` during searching is a no-op.
- Build the **subprocess boundary test harness**: a controllable fake `rg` with explicit readiness/completion handshakes (e.g. writes a ready file / waits on a fifo), run under a controlled terminal/PTY so tests can assert actual child termination and terminal restoration rather than just that a cancel command was emitted.

See PRD *Outcome and exit-status contract* (rows 1–2 and the cleanup bullet), *Colours, overlays, and key precedence* (`ctrl+c` precedence), and *Testing Decisions → Subprocess boundary / Responsiveness boundaries*.

### How to verify

- **Manual**:
  1. Run `vrg . /` (long search); press `q` while "Searching…" → prompt returns, `echo $?` is 130, `pgrep rg` shows no orphan.
  2. Same with `ctrl+c`.
  3. Terminal is usable afterwards (no alt-screen residue, cursor visible).
- **Automated**:
  - Subprocess/PTY test: fake rg signals ready and then blocks; send `q`; assert exit 130, fake rg process is gone (waited/reaped), and terminal restoration sequence was written.
  - Same for `ctrl+c`.
  - Model test: `Esc` during searching produces no state change.
  - Test that late search-completion messages arriving after cancellation do not revive the UI.

### Acceptance criteria

- [ ] Given searching is in progress, when `q` is pressed, then the child is terminated and reaped, the terminal restored, and the exit status is 130.
- [ ] Given any state, when `ctrl+c` is pressed, then the exit status is 130 after cleanup.
- [ ] Given a normal exit while rg still runs, then no rg process remains.
- [ ] Given cancellation, when a late completion message arrives, then the UI is not revived.
- [ ] Given the searching screen, when `Esc` is pressed, then nothing happens.

### User stories addressed

- User story 11: `q` during searching or `ctrl+c` anywhere cancels and exits 130
- User story 23: every normal exit restores the terminal and reaps the child

---

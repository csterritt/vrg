## Issue 9: Error overlay and fatal search outcomes (exit 2)

**Type**: AFK
**Blocked by**: Issue 8

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

The modal error overlay and the fatal rows of the outcome table.

- Capture rg stderr fully (drain it concurrently). Include it in diagnostics regardless of exit code. If a failed process supplies no stderr, generate a diagnostic naming the exit code or signal.
- Fatal conditions: rg exits other than 0/1, dies by signal, or **stream integrity fails** (missing summary, missing/orphaned/inconsistent `begin`/`end` pairs, lost completion metadata). Integrity and process success are assessed separately; a summary alone is a complete zero-result stream.
- With usable results: browse with the error overlay open; `q`/`Esc` dismiss to browsing; eventual `q` exits 2.
- Without usable results: error overlay; dismissal exits 2.
- Non-fatal stderr on rg 0/1 with usable results: browse with a warning overlay, eventual exit 0.
- Overlay: base colours, single-line border, `up`/`down` scroll, `q`/`Esc` dismiss, `ctrl+c` exits 130, other keys ignored. Text wraps to interior width, including unbroken strings. Fixed exit status is decided once searching completes and never changes afterwards.
- Frame the outcome decision as a testable function of (process result, integrity, usable-result count, diagnostics) so later issues extend it.

See PRD *Outcome and exit-status contract* (table rows 4–5, 7 and bullets), *Result index…* (integrity bullets), *Colours, overlays, and key precedence* (error overlay bullet).

### How to verify

- **Manual**:
  1. Fake rg that emits two valid matches then exits 3 with stderr "boom" → browse view with overlay containing "boom"; `Esc` dismisses; `q` exits 2.
  2. Fake rg that exits 2 with no output → overlay naming exit code 2; `q` exits 2.
  3. Fake rg killed by SIGKILL mid-stream → overlay names the signal.
- **Automated**: App model tests for each fatal row with/without usable results; missing `summary` with valid matches → overlay + exit 2; orphaned `end` → integrity failure; stderr on rg 0 → warning overlay, exit 0; overlay key routing (scroll, dismiss, ignored keys, `ctrl+c`); long unbroken diagnostic wraps within the border. Subprocess test with the fake rg exiting non-zero after a handshake.

### Acceptance criteria

- [ ] Given rg exits with a code other than 0/1 or by signal and usable results exist, then browsing starts with an error overlay and the eventual `q` exits 2.
- [ ] Given the same failure with no usable results, then the overlay's dismissal exits 2.
- [ ] Given a stream lacking a valid summary or with unpaired lifecycle events, then integrity fails and the fatal rules apply.
- [ ] Given rg exits 0/1 with stderr output and usable results, then a warning overlay is shown and the eventual exit is 0.
- [ ] Given a failed process with empty stderr, then the overlay names the exit code or signal rather than being empty.
- [ ] Given an error overlay, when any key other than `up`/`down`/`q`/`Esc`/`ctrl+c` is pressed, then it is ignored.

### User stories addressed

- User story 15: partial results browsable alongside an error overlay; exit 2
- User story 16: fatal search with no results shows overlay and exits 2
- User story 19: missing completion metadata reported

---

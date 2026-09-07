## Issue 15: File-change pop-up with instance-keyed timer

**Type**: AFK
**Blocked by**: Issue 6, Issue 9, Issue 13

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

When navigation selects a stop in a different file, show a brief centred pop-up containing the single-line safe path, left-truncated with `…` to fit.

- Starts at selection, not at load completion; load completion does not restart it.
- Lifetime one second **or** until any key press; that key also performs its normal action in the same update.
- Each pop-up has a fresh instance ID; an expiry message from a stale instance cannot dismiss a newer pop-up.
- Centring and truncation are computed from the current terminal size at every render; resize neither dismisses nor restarts the timer.
- The pop-up path text is routed through the Issue 6 utility; add the pop-up to the sink-safety table.
- Opening an error overlay (Issue 9) cancels the pop-up and it does not return. Cancellation by the help overlay is owned by Issue 31, which depends on this issue; Issue 32 tests the combined precedence.

See PRD *Colours, overlays, and key precedence* (pop-up bullet) and *Testing Decisions → Overlays/output*.

### How to verify

- **Manual**: press `n` across a file boundary → pop-up appears centred, disappears after ~1 s; press `n` again quickly → pop-up shows the new file and `n` still advanced; resize while shown → it re-centres.
- **Automated**: model tests driven by injected expiry messages with explicit instance IDs: expiry for instance 1 after instance 2 was created leaves instance 2 visible; a key dismisses and its action is applied; resize keeps the pop-up and recentres/re-truncates; long path truncated with leading `…`; an error overlay arriving (injected current-file diagnostic message) cancels the pop-up and it does not return after dismissal; hostile path bytes in the pop-up pass the Issue 6 sink-safety check.

### Acceptance criteria

- [ ] Given navigation changes the current file, then a centred pop-up with the escaped, truncated path appears immediately.
- [ ] Given the pop-up is visible, when its own expiry message arrives, then it disappears; when a stale instance's expiry arrives, then it stays.
- [ ] Given the pop-up is visible, when any key is pressed, then the pop-up disappears and the key's normal action is performed.
- [ ] Given a resize while visible, then the pop-up is re-centred and re-truncated without restarting its timer.
- [ ] Given a pop-up is visible, when an error overlay opens, then the pop-up is cancelled and does not reappear when the overlay is dismissed.
- [ ] Given a path containing control bytes, then the pop-up shows the escaped single-line form.

### User stories addressed

- User story 58: brief centred filename pop-up on file change
- User story 59: one-second or key-press lifetime; key acts normally; stays centred on resize

---

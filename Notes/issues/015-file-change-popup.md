## Issue 15: File-change pop-up with instance-keyed timer

**Type**: AFK
**Blocked by**: Issue 13

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

When navigation selects a stop in a different file, show a brief centred pop-up containing the single-line safe path, left-truncated with `…` to fit.

- Starts at selection, not at load completion; load completion does not restart it.
- Lifetime one second **or** until any key press; that key also performs its normal action in the same update.
- Each pop-up has a fresh instance ID; an expiry message from a stale instance cannot dismiss a newer pop-up.
- Centring and truncation are computed from the current terminal size at every render; resize neither dismisses nor restarts the timer.
- Opening help or an error overlay (Issues 9/31) cancels the pop-up and it does not return.

See PRD *Colours, overlays, and key precedence* (pop-up bullet) and *Testing Decisions → Overlays/output*.

### How to verify

- **Manual**: press `n` across a file boundary → pop-up appears centred, disappears after ~1 s; press `n` again quickly → pop-up shows the new file and `n` still advanced; resize while shown → it re-centres.
- **Automated**: model tests driven by injected expiry messages with explicit instance IDs: expiry for instance 1 after instance 2 was created leaves instance 2 visible; a key dismisses and its action is applied; resize keeps the pop-up and recentres/re-truncates; long path truncated with leading `…`; help open cancels the pop-up.

### Acceptance criteria

- [ ] Given navigation changes the current file, then a centred pop-up with the escaped, truncated path appears immediately.
- [ ] Given the pop-up is visible, when its own expiry message arrives, then it disappears; when a stale instance's expiry arrives, then it stays.
- [ ] Given the pop-up is visible, when any key is pressed, then the pop-up disappears and the key's normal action is performed.
- [ ] Given a resize while visible, then the pop-up is re-centred and re-truncated without restarting its timer.

### User stories addressed

- User story 58: brief centred filename pop-up on file change
- User story 59: one-second or key-press lifetime; key acts normally; stays centred on resize

---

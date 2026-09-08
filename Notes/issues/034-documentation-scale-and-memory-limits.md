## Issue 34: Documentation — scale examples, content assumptions, memory limits

**Type**: AFK
**Blocked by**: Issue 10, Issue 31

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

A user-facing `README.md` at the repository root (the single selected documentation artifact — not a `docs/` page) and the help overlay footer note documenting:

- Independent scale examples (~10,000 matched files, ~100,000 matched lines, individual files ~50 MB) and that they are not simultaneous capacity guarantees.
- The ~50 MB example assumes UTF-8 with ordinary line lengths; base64 `bytes` expansion can push a single `match` record over the 64 MiB limit, which is skipped and reported (naming the path when recoverable).
- Buffers are retained for the session; no eviction, no aggregate memory bound, no reliable OOM recovery; forced termination cannot guarantee terminal cleanup.
- Invocation syntax, the flag allow-list, exit statuses, and key bindings (kept in sync with the help overlay by consuming Issue 31's binding table).
- A complete exit-status table covering all four statuses with their state-specific triggers: 0 (successful search and browse), 1 (no results), 2 (pre-TUI usage/root/start failures and fatal search outcomes), and 130 (cancellation — `q` while searching or result preparation is incomplete, and `ctrl+c` in any state).
- The ripgrep reference family: VRG targets ripgrep 15.x semantics, and VRG supplies `--no-config` so ripgrep configuration files are never honoured.
- Any generated README/help text that embeds runtime strings goes through the Issue 6 utility (static text needs no sanitization). Issue 34 also owns the sink-safety table row assigned to it by Issue 6: the rendered help footer — and any other generated text path that accepts runtime strings — joins the shared hostile-fixture table, substituting Issue 6's fixtures at every runtime-substitution point and asserting through the no-style composition path that no fixture control byte survives in raw output.

See PRD *Resources and responsiveness*, *Out of Scope*, and *Further Notes*.

### How to verify

- **Manual**: read the README; every flag listed matches Issue 2's allow-list; every key matches the help overlay; every exit status matches the outcome table; the ripgrep 15.x reference family and `--no-config` behaviour are stated.
- **Automated**: a test that iterates Issue 31's binding table and asserts each binding appears in the README (or generates the README section from the same table and asserts the committed README is up to date); a test that the README lists each allow-listed flag from Issue 2's table; a test that the README's exit-status documentation covers 0, 1, 2, and 130 — including the cancellation triggers (`q` during search/result preparation, `ctrl+c` anywhere) and the pre-TUI usage/root/start failures — and that its search-derived values agree with Issue 9's outcome function; a test that the README identifies ripgrep 15.x as the reference family and states that VRG supplies `--no-config`.

### Acceptance criteria

- [ ] Given the README, then it states the three scale examples and explicitly says they are independent, not aggregate, guarantees.
- [ ] Given the README, then it explains the 64 MiB record limit, the base64 expansion caveat, and the oversized-record diagnostic.
- [ ] Given the README, then it documents session-long buffer retention and the lack of OOM/forced-termination guarantees.
- [ ] Given the README, then its flag list, key bindings, and exit statuses match the implementation, including 130 for cancellation and 2 for pre-TUI usage/root/start failures.
- [ ] Given the README, then it identifies ripgrep 15.x as the reference family and explains that VRG supplies `--no-config` so ripgrep configuration files are ignored.

### User stories addressed

- User story 85: documented scale examples, assumptions, and memory limitations

---

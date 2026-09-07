## Issue 34: Documentation — scale examples, content assumptions, memory limits

**Type**: AFK
**Blocked by**: Issue 10

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

A user-facing README (or `docs/` page) and the help overlay footer note documenting:

- Independent scale examples (~10,000 matched files, ~100,000 matched lines, individual files ~50 MB) and that they are not simultaneous capacity guarantees.
- The ~50 MB example assumes UTF-8 with ordinary line lengths; base64 `bytes` expansion can push a single `match` record over the 64 MiB limit, which is skipped and reported (naming the path when recoverable).
- Buffers are retained for the session; no eviction, no aggregate memory bound, no reliable OOM recovery; forced termination cannot guarantee terminal cleanup.
- Invocation syntax, the flag allow-list, exit statuses, and key bindings (kept in sync with the help overlay).

Also confirm the documented ripgrep reference family (15.x) and `--no-config` behaviour.

See PRD *Resources and responsiveness*, *Out of Scope*, and *Further Notes*.

### How to verify

- **Manual**: read the README; every flag listed matches Issue 2's allow-list; every key matches the help overlay; every exit status matches the outcome table.
- **Automated**: a test that extracts the key-binding table from the help overlay source and asserts each binding appears in the README (or generates the README section from the same source); a test that the README lists each allow-listed flag.

### Acceptance criteria

- [ ] Given the README, then it states the three scale examples and explicitly says they are independent, not aggregate, guarantees.
- [ ] Given the README, then it explains the 64 MiB record limit, the base64 expansion caveat, and the oversized-record diagnostic.
- [ ] Given the README, then it documents session-long buffer retention and the lack of OOM/forced-termination guarantees.
- [ ] Given the README, then its flag list, key bindings, and exit statuses match the implementation.

### User stories addressed

- User story 85: documented scale examples, assumptions, and memory limitations

---

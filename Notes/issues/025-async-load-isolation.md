## Issue 25: Asynchronous load isolation — navigate during load, late results update only their own file

**Type**: AFK
**Blocked by**: Issue 13

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

- Navigation remains fully active while a file is loading; the user can move past a slow file. Scrolling a placeholder is a no-op; other keys keep their normal meaning.
- Load completions are keyed by raw path and request identity. A completion updates only that path's cache/status. It affects the visible panel only if that path is current at arrival time.
- Cached successful buffers are retained for the session (no eviction).
- At most one load in flight per raw path; re-entering a path already loading does not start another load (the request is dropped, not queued).
- Late messages after cancellation cannot revive the UI.
- A load completion carries a prepared buffer (decoded, mapped) so `Update` does no full-file decoding; while the decode/map phase is held by a gate, `ctrl+c`, `n`/`p`, `w`, `c`, and resize remain actionable. (The analogous contract for layout/rewrap is Issue 17.)

Load-completion reveal semantics for the current file are Issue 28; failures are Issue 26.

See PRD *File loading, cache, reload, and selection consistency* (first three bullets) and *Testing Decisions → Asynchronous App behavior*.

### How to verify

- **Manual**: with a very large file A first in the index and small file B second, press `n` immediately at startup → B shows while A still loads; press `p` → A shows content once loaded (not before).
- **Automated**: model tests with worker gates: A→B→C navigation with A's completion arriving while C is current → C's panel unchanged, A cached; re-entering A while its load is gated → no second load request; navigating back to A after completion shows cached content without a new load; completion after cancellation ignored; with the decode/map phase of the current file's load gated, `ctrl+c` → 130, `n` advances, `w`/`c` toggle, resize applies — none wait on the gate.

### Acceptance criteria

- [ ] Given a load is in flight, when `n`/`p` is pressed, then navigation proceeds immediately.
- [ ] Given a late load completion for a non-current file, then only that file's cache updates and the current panel is unchanged.
- [ ] Given a file already loading, when it is re-entered, then no additional load is started.
- [ ] Given a file loaded earlier in the session, when revisited, then its cached content is shown without a new load.
- [ ] Given the current file's decode/map phase is held by a gate, when `ctrl+c` or normal browse input arrives, then it is handled without waiting.

### User stories addressed

- User story 35: navigation continues during loading
- User story 36: a late result updates only its own file's cache

---

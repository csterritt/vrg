## Issue 45: Move test hooks and spin-watchers out of the production binary

**Type**: AFK
**Blocked by**: None — can start immediately

### Parent PRD

`Notes/PRD-vrg.md` — audit source: `Notes/critiques/final-audit-vrg.md`, Medium finding 10

### What to build

Remove the test-only seams compiled into the released binary (`cmd/vrg/main.go:76-167`). Environment variables currently make the production binary able to truncate or append arbitrary user-writable files, inject fake diagnostics/failures, hold result preparation indefinitely, and run tight file-watcher polling loops with no sleep or cancellation. These are test-harness capabilities shipping as normal production behaviour — a security and robustness problem, not just untidiness.

- Move every env-var-controlled test seam behind a test-only build constraint (e.g. a `vrg_testhooks` tag or `_test.go`-only injection), or inject the behaviour from test code through a dedicated harness API that does not exist in the production build.
- If any polling/watching genuinely remains in production code, make it cancellable and non-spinning (sleep/backoff or event-driven).
- Tests that relied on these env vars are updated to use the new harness; their coverage is preserved.

Consult `Notes/skills/code-writing/production-code-and-build-constraints` for the project's build-constraint conventions before choosing the mechanism.

### How to verify

- **Manual**: build the production binary (`go build ./cmd/vrg`), set each former test-hook env var, and run the binary → none of them alter behaviour; strings/symbols for the hooks are absent from the production binary.
- **Automated**: the existing tests that exercised these seams still pass via the test-only harness (build-tag-gated test file or injected hooks); a test or build check asserting the production binary does not read the test-hook env vars; a race/timeout run confirming no remaining watcher spins without cancellation.

### Acceptance criteria

- [ ] Given the production build, then no environment variable can trigger file truncation/appends, injected diagnostics or failures, held preparation, or test-only watcher loops.
- [ ] Given test builds, then all prior seam-controlled behaviours remain available to tests through the test-only mechanism.
- [ ] Given any remaining production polling, then it is cancellable and bounded (sleep/backoff or event-driven) — no tight spin.
- [ ] Given `go build ./...` and `go vet ./...` in the default configuration, then the test-hook code is excluded or unreachable.
- [ ] Given the updated harness, then every test previously using the env-var seams still passes.

### User stories addressed

- None directly — production-hygiene hardening from the audit; it protects the story 22–23 exit/replay and cleanup contracts by keeping test-only failure injection and unkillable spin loops out of the released binary.

---

## Issue 48: Critical PTY tests wait on application handshakes, not fixed sleeps

**Type**: AFK
**Blocked by**: Issue 45 — the handshakes must ride the same test-only harness mechanism (`vrg_testhooks`-tagged seams) that issue installs; implementing first would either add more production hooks that Issue 45 must then migrate or invent a competing harness

### Parent PRD

`Notes/PRD-vrg.md` — audit source: `Notes/critiques/final-audit-vrg.md`, Medium finding 13

### What to build

Replace the fixed 100–500 ms `time.Sleep` delays in the critical PTY/subprocess tests (`cmd/vrg/outcome_test.go:18-58`, `cmd/vrg/replay_test.go:176-184`, `221-234`, `315-326`) with application-side acknowledgements. Issue 35 explicitly requires these tests to run without relying on sleeps: the harnesses currently wait fixed durations to *assume* model transitions completed, which can hide ordering bugs and flakes under load.

- Add application-side acknowledgement for each state transition and key-processing boundary the tests wait on — e.g. a test-visible signal when a message has been processed, a load has completed, or a state has been entered — exposed through the test-only `vrg_testhooks`-tagged seam mechanism Issue 45 installs (never in the production binary).
- Each test waits on the handshake for the specific transition it cares about, with a bounded timeout for failure reporting — never on elapsed time as a proxy for progress.
- The handshakes must not change production behaviour or timing; they observe it.

See PRD *Testing Decisions* (critical PTY/subprocess tests without sleeps) and Issue 35's verification contract.

### How to verify

- **Manual**: run `go test ./cmd/vrg -count=10` (and under `-race`) → all PTY/subprocess tests pass deterministically with no `time.Sleep`-based synchronization remaining in the listed files.
- **Automated**: the tests themselves — each former sleep site now blocks on a handshake channel/condition with a failure timeout; a grep-level or review check confirms no fixed-delay synchronization remains in `outcome_test.go`/`replay_test.go`; repeated and race-mode runs demonstrate stability.

### Acceptance criteria

- [ ] Given the outcome and replay PTY harnesses, then no test step synchronizes on a fixed `time.Sleep` delay.
- [ ] Given each awaited model transition or key-processing boundary, then an application-side acknowledgement exists and the test waits on it.
- [ ] Given a handshake that never arrives, then the test fails on a bounded timeout with a useful message rather than hanging.
- [ ] Given `go test ./cmd/vrg -count=10` and `-race` runs, then the tests pass consistently, including under parallel load.
- [ ] Given the production binary, then handshake instrumentation is absent or inert (test-only, consistent with Issue 45's mechanism).

### User stories addressed

- Testing-infrastructure issue: it hardens the verification of the stories exercised by these PTY tests — principally stories 11, 15, 16, and 22 (cancellation, fatal outcomes, diagnostic replay) — per the PRD's Testing Decisions.

---

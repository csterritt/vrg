## Issue 48: Critical PTY tests wait on application handshakes, not fixed sleeps

**Type**: AFK
**Blocked by**: Issue 45 — the handshakes must ride the same test-only harness mechanism (`vrg_testhooks`-tagged seams) that issue installs; implementing first would either add more production hooks that Issue 45 must then migrate or invent a competing harness

### Parent PRD

`Notes/PRD-vrg.md` — audit source: `Notes/critiques/final-audit-vrg.md`, Medium finding 13

### What to build

Replace fixed-delay synchronization throughout the critical PTY/subprocess harnesses with application-side acknowledgements. Scope this work by helper and behavior, not by the old line ranges: it includes every key send or assumed application transition in `runVrgWithKeys` and `runVrgKillChild` (`outcome_test.go`), every `runVrgReplay` trigger callback (`replay_test.go`), and `runVrgWithQuit` (`search_test.go`), including settle and inter-key sleeps outside the originally cited ranges. It also covers any additional helper used by Issue 50's named PTY tests that sends a key or assumes a model transition. Issue 35 explicitly requires these tests to run without relying on sleeps: the harnesses currently wait fixed durations to *assume* transitions completed, which can hide ordering bugs and flakes under load.

- Add application-side acknowledgement for each state transition and key-processing boundary the tests wait on — e.g. a test-visible signal when a message has been processed, a load has completed, an overlay has been dismissed, or a state has been entered — exposed through the test-only `vrg_testhooks`-tagged seam mechanism Issue 45 installs (never in the production binary). Add the exact acknowledgement hook names to Issue 45's explicit vrg-consumed hook manifest.
- Each test waits on the handshake for the specific transition it cares about, with a bounded timeout for failure reporting — never on elapsed time as a proxy for progress. `waitForFile`/`waitForAckLines`-style bounded condition polls are permitted; their short sleeps pace checks of an explicit condition and are not settling delays. `cancel_test.go`'s existing send-after-ready pattern is already conforming unless a changed test adds an unacknowledged transition.
- The handshakes must not change production behaviour or timing; they observe it.

See PRD *Testing Decisions* (critical PTY/subprocess tests without sleeps) and Issue 35's verification contract.

### How to verify

- **Manual**: run `go test ./cmd/vrg -count=10` (and under `-race`) → all PTY/subprocess tests pass deterministically with no fixed settling or inter-key delay remaining in any `cmd/vrg` PTY helper.
- **Automated**: the tests themselves — each former settling/inter-key sleep now blocks on a handshake channel/condition with a failure timeout; a grep-level or review check covers `runVrgWithKeys`, `runVrgKillChild`, every `runVrgReplay` trigger callback, `runVrgWithQuit`, and any additional key-sending helper used by Issue 50's named tests. Repeated and race-mode runs demonstrate stability. Bounded condition-poll sleeps are allowed only when each iteration tests the named condition.

### Acceptance criteria

- [ ] Given `runVrgWithKeys`, `runVrgKillChild`, every `runVrgReplay` trigger callback, `runVrgWithQuit`, and any other PTY helper used by Issue 50's named tests, then no key send or model-transition step synchronizes on a fixed settling/inter-key delay; bounded polling is used only for an explicit condition.
- [ ] Given each awaited model transition or key-processing boundary — including overlay dismissal before a following quit — then an application-side acknowledgement exists and the test waits on it.
- [ ] Given a handshake that never arrives, then the test fails on a bounded timeout with a useful message rather than hanging.
- [ ] Given `go test ./cmd/vrg -count=10` and `-race` runs, then the tests pass consistently, including under parallel load.
- [ ] Given the production binary, then handshake instrumentation is absent or inert (test-only, consistent with Issue 45's mechanism).

### User stories addressed

- Testing-infrastructure issue: it hardens the verification of the stories exercised by these PTY tests — principally stories 11, 15, 16, and 22 (cancellation, fatal outcomes, diagnostic replay) — per the PRD's Testing Decisions.

---

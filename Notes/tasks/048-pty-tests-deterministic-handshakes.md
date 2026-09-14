# Tasks for #48: Critical PTY tests wait on application handshakes, not fixed sleeps

Parent issue: #48
Parent PRD: PRD-vrg.md
**Blocked by issues**: #45 — shared-file ordering with #41 on `cmd/vrg/outcome_test.go`: never implement the two concurrently; land Issue #41 completely first, or land this issue first and have Issue #41 adapt to the handshake harness
**Acceptance criteria**: AC1–AC5 → Tasks 1–2
**Manual verification**: Task 4 owns the issue's manual checks.

## Tasks

### 1. Specify the handshake contract

**Type**: RED  
**Output**: Failing tests require application-side acknowledgements for each awaited model transition and key-processing boundary — including a bounded-timeout failure when a handshake never arrives — plus a static check that no PTY helper synchronizes on a fixed settling or inter-key delay.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md` and the build-constraint conventions in `Notes/skills/code-writing/production-code-and-build-constraints.md`.

Begin only after Issue #45 is complete — the handshakes ride its `vrg_testhooks`-tagged seam mechanism, never the production binary. Add failing tests in `cmd/vrg` proving a named acknowledgement exists and fires for a representative awaited transition (e.g. a diagnostic processed into the collection, a load completed, an overlay dismissed, or a state entered), and that a test waiting on a handshake that never arrives fails on a bounded timeout with a useful message rather than hanging. Add a grep-level or review check asserting no `runVrgWithKeys`, `runVrgKillChild`, `runVrgReplay` trigger callback, `runVrgWithQuit`, or other key-sending PTY helper used by Issue #50's named tests retains a fixed settling or inter-key `time.Sleep` — bounded `waitForFile`/`waitForAckLines`-style condition polls whose short sleeps pace checks of an explicit condition remain permitted. The checks fail on the current helpers, which wait fixed durations to assume transitions completed. Keep this task test-only.

---

### 2. Implement the acknowledgement seams and rewrite the helpers

**Type**: GREEN  
**Output**: The handshake tests pass; every PTY helper waits on application-side acknowledgements with bounded timeouts, `go test ./cmd/vrg -count=10` and `-race` runs pass consistently including under parallel load, and the production binary is unaffected.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md` and the build-constraint conventions in `Notes/skills/code-writing/production-code-and-build-constraints.md`.

Add application-side acknowledgement signals for each state transition and key-processing boundary the tests wait on — a message processed, a load completed, an overlay dismissed, a state entered — exposed only through the `vrg_testhooks`-tagged seam mechanism Issue #45 installs, appending the exact acknowledgement hook names to its explicit vrg-consumed hook manifest. The handshakes must observe production behaviour and timing, never change it. Rewrite every key send or assumed application transition in `runVrgWithKeys` and `runVrgKillChild` (`outcome_test.go`), every `runVrgReplay` trigger callback (`replay_test.go`), `runVrgWithQuit` (`search_test.go`), and any additional helper used by Issue #50's named tests — including settle and inter-key sleeps outside the originally cited ranges — so each step blocks on the handshake for the specific transition it cares about with a bounded timeout for failure reporting, never on elapsed time as a proxy for progress; overlay dismissal before a following quit must be acknowledged. `cancel_test.go`'s send-after-ready pattern already conforms unless a changed test adds an unacknowledged transition. If Issue #41 has already landed, adapt its revised `TestStderrContentFixture` and overlay regressions to this harness; if not, note that it must adapt when it lands. If Issue #46's new subprocess tests exist, they use these helpers too. Run `go test ./cmd/vrg -count=10`, `CGO_ENABLED=1 go test -race -count=1 ./cmd/vrg`, and the repository-standard suite.

---

### 3. Document the handshake harness

**Type**: DOCUMENT  
**Output**: Wiki documentation records the acknowledgement seams, the helper contract, and the bounded-poll allowance.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #48 implementation into the appropriate pages under `Notes/wiki`. Document the named acknowledgement hooks and their membership in Issue #45's explicit hook manifest, the rewritten helpers' wait-on-handshake contract with bounded timeouts, the permitted bounded condition-poll sleeps (each iteration checking an explicit condition), and the confirmation that handshakes only observe production timing and are absent or inert in the production binary. Cross-reference Issue #48 and the *Testing Decisions* section of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the handshake-harness walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/048-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/048-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the handshake contract tests and the no-fixed-sleep check, then run the issue's manual scenario: `go test ./cmd/vrg -count=10` and under `-race` → all PTY/subprocess tests pass deterministically with no fixed settling or inter-key delay remaining in any `cmd/vrg` PTY helper. Capture commands, outputs, and exit statuses. Reference Issue #48 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---

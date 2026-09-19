# Tasks for #48: Critical PTY tests wait on application handshakes, not fixed sleeps

Parent issue: #48
Parent PRD: PRD-vrg.md
**Blocked by issues**: #45 — shared-file/harness ordering with #41 and #46 in `cmd/vrg`: never implement overlapping PTY work concurrently; whichever issue lands second adapts to the already-landed tests and helpers. If #41 or #46 has landed, incorporate its tests and any key-sending helpers into the handshake matrix and remove fixed settling/inter-key delays; if #48 lands first, #41/#46 apply their reciprocal adaptation clauses
**Acceptance criteria**: AC1–AC5 → Tasks 1–2
**Manual verification**: Task 3 owns the issue's manual checks.

## Tasks

### 1. Specify the handshake contract

**Type**: RED  
**Output**: A finite helper/action/postcondition/acknowledgement matrix and failing tests require a causally correlated application-side acknowledgement for every awaited model transition and key-processing boundary — including repeated same-kind events, overlay-dismissal-before-quit, and a bounded-timeout failure when a handshake never arrives — plus a static check that no PTY helper synchronizes on a fixed settling or inter-key delay.
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md` and the build-constraint conventions in `Notes/skills/code-writing/production-code-and-build-constraints.md`.

Begin only after Issue #45 is complete — the handshakes ride its `vrg_testhooks`-tagged seam mechanism, never the production binary. First create a finite handshake matrix covering every key send and assumed transition in `runVrgWithKeys`, `runVrgKillChild`, every `runVrgReplay` trigger callback, `runVrgWithQuit`, and every other key-sending helper used by Issue #50's named tests. Each row names the helper/test group, triggering action, exact application-side postcondition, acknowledgement hook/event, and the next action that acknowledgement unlocks. Add failing tests proving every matrix row has an application-side acknowledgement and that acknowledgements are correlated per process and per occurrence — for example with monotonic sequence/event records — so an earlier same-kind event cannot satisfy a later wait. Include dedicated regressions for repeated same-kind events and overlay dismissal acknowledged before a following `q`, plus a wait for an acknowledgement that never arrives failing on a bounded timeout with a useful message rather than hanging. Add a grep-level or review check asserting no covered helper retains a fixed settling or inter-key `time.Sleep` — bounded `waitForFile`/`waitForAckLines`-style condition polls whose short sleeps pace checks of an explicit condition remain permitted. The checks fail on the current helpers, which wait fixed durations to assume transitions completed. Keep this task test-only.

---

### 2. Implement the acknowledgement seams and rewrite the helpers

**Type**: GREEN  
**Output**: The handshake tests pass; every PTY helper waits on application-side acknowledgements with bounded timeouts, `go test ./cmd/vrg -count=10` and `-race` runs pass consistently including under parallel load, and the production binary is unaffected.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md` and the build-constraint conventions in `Notes/skills/code-writing/production-code-and-build-constraints.md`.

Implement every row of the Task 1 handshake matrix with application-side acknowledgement signals for each state transition and key-processing boundary the tests wait on — a message processed, a load completed, an overlay dismissed, a state entered — exposed only through the `vrg_testhooks`-tagged seam mechanism Issue #45 installs. Correlate records per process and per occurrence so helpers wait for the exact postcondition caused by their preceding action rather than consuming stale evidence. Append the exact acknowledgement hook names to Issue #45's explicit vrg-consumed hook manifest, extend its untagged production-artifact boundary test to probe every new name for no behavior and no artifact string, and rerun that test. The handshakes must observe production behaviour and timing, never change it. Rewrite every key send or assumed application transition in `runVrgWithKeys` and `runVrgKillChild` (`outcome_test.go`), every `runVrgReplay` trigger callback (`replay_test.go`), `runVrgWithQuit` (`search_test.go`), and any additional helper used by Issue #50's named tests — including settle and inter-key sleeps outside the originally cited ranges — so each step blocks on the handshake for the specific transition it cares about with a bounded timeout for failure reporting, never on elapsed time as a proxy for progress; overlay dismissal before a following quit must be acknowledged. `cancel_test.go`'s send-after-ready pattern already conforms unless a changed test adds an unacknowledged transition. If Issue #41 has already landed, adapt its revised `TestStderrContentFixture` and overlay regressions to this harness; if not, note that it must adapt when it lands. If Issue #46's new subprocess tests exist, they use these helpers too. Run `go test ./cmd/vrg -count=10`, `CGO_ENABLED=1 go test -race -count=1 ./cmd/vrg`, and the repository-standard suite.

---

### 3. Create the handshake-harness walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/048-04/code-walkthrough`.  
**Depends on**: 2

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/048-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the handshake contract tests and the no-fixed-sleep check, then run the issue's manual scenario: `go test ./cmd/vrg -count=10` and under `-race` → all PTY/subprocess tests pass deterministically with no fixed settling or inter-key delay remaining in any `cmd/vrg` PTY helper. Capture commands, outputs, and exit statuses. Reference Issue #48 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---

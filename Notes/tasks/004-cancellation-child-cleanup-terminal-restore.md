# Tasks for #4: Cancellation (`q` while searching, `ctrl+c` anywhere), child termination/reap, terminal restore, controlled-failure cleanup

Parent issue: #4
Parent PRD: PRD-vrg.md
**Blocked by issues**: #3
**Acceptance criteria**: AC1–AC8 → Tasks 1–2
**Manual verification**: Task 3 owns the issue's manual checks.

## Tasks

### 1. Specify cancellation, cleanup, and terminal restoration

**Type**: RED  
**Output**: Failing model tests cover `q` while searching including gate-held preparation, the `Esc` no-op, and late-completion rejection; the PTY harness with a controllable fake rg asserts exit 130 with child termination and reaping, display and termios restoration, normal-exit reaping, and the injected controlled-failure contract.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issue #3 is complete. Build the subprocess boundary test harness the Testing Decisions of `Notes/PRD-vrg.md` require — a controllable fake `rg` with explicit readiness and completion handshakes run under a controlled terminal/PTY, plus a test-only side channel proving vrg's wait/reap path ran rather than inferring it from a missing PID — and add failing tests for the Issue #4 contracts. At the model level require `q` while searching or result preparation is incomplete (including after rg exit while index preparation is gate-held) to be cancellation, `Esc` during searching to be a no-op, and late search-completion messages after cancellation not to revive the UI. At the PTY level require `q` and `ctrl+c` to exit 130 with the child terminated and reaped, the display-restoration sequence emitted, and PTY termios after exit equal to before launch; a normal exit while rg still runs to leave no orphaned or unreaped child; and an injected controlled failure after the child has signalled ready to reap the child, restore termios, and write a sanitized diagnostic to stderr exactly once, after terminal restoration, exiting 2. Keep this task test-only.

---

### 2. Implement cancellation and the cleanup boundary

**Type**: GREEN  
**Output**: Model and PTY cleanup tests pass; every controlled exit restores the display and input modes, reaps the child, and writes the controlled-failure diagnostic exactly once after restoration.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the cancellation and cleanup paths in `internal/app` and the process boundary in `cmd/vrg`. Make `ctrl+c` in any state and `q` while searching or result preparation is incomplete cancel outstanding work, terminate and reap the rg child, and restore both the display and the PTY input modes before exiting 130 with no further screen; route every ordinary exit through the same cleanup; add the injectable failure hook for controlled application failures and the single post-restoration stderr writer that emits the controlled-failure diagnostic exactly once after restoration and never both directly and through a later replay mechanism; and discard late search completions after cancellation. Issue #3's drainage must end promptly when the child is terminated. Issue #11 generalizes the writer into session replay; do not build the collection here.

---

### 3. Create the cancellation walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/004-04/code-walkthrough`.  
**Depends on**: 2

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/004-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the PTY harness tests: `q` and `ctrl+c` against a blocked fake rg with exit 130, reap evidence, the display-restoration sequence, and termios equality; `q` during gate-held index preparation; and the injected controlled failure with its exactly-once post-restoration diagnostic and exit 2. Include a manual long-search cancellation showing the returned prompt, exit status 130, no orphaned rg, and `stty -a` matching its pre-run state. Reference Issue #4 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---

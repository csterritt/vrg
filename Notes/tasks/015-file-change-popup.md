# Tasks for #15: File-change pop-up with instance-keyed timer

Parent issue: #15
Parent PRD: PRD-vrg.md
**Blocked by issues**: #6, #9, #13
**Acceptance criteria**: AC1–AC6 → Tasks 1–2
**Manual verification**: Task 4 owns the issue's manual checks.

## Tasks

### 1. Specify the file-change pop-up lifecycle

**Type**: RED  
**Output**: Failing model tests driven by instance-keyed injected expiry messages cover fresh timers, stale-instance rejection, dismissal plus action, resize recentering without timer restart, error-overlay cancellation, truncation, and the sink-safety row.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issues #6, #9, and #13 are complete. Add failing model tests in `internal/app` for the Issue #15 contracts and the pop-up bullet of the Colours, overlays, and key precedence section of `Notes/PRD-vrg.md`, driving timers with explicit instance IDs and no sleeps. Require the pop-up to start at selection rather than load completion, with load completion never restarting it; a fresh one-second instance per pop-up whose expiry message dismisses only that instance, so an expiry from a stale instance cannot dismiss a newer pop-up; any key press to dismiss the pop-up and perform its normal action in the same update; centring and left-truncation with a leading `…` computed from the current terminal size at every render, so a resize recentres and re-truncates without dismissing or restarting the timer; an error overlay arriving to cancel the pop-up with no return after dismissal; the single-line safe path routed through the Issue #6 utility with the pop-up added to the sink-safety table; and a long path truncated to fit. Keep this task test-only.

---

### 2. Implement the instance-keyed pop-up

**Type**: GREEN  
**Output**: Pop-up lifecycle tests pass; navigation across a file boundary shows the centred, escaped, truncated path with a fresh timer.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the file-change pop-up in `internal/app` to satisfy Task 1: instance-keyed expiry commands, selection-time start, dismissal-plus-normal-action routing, render-time centring and truncation, error-overlay cancellation without return, and sanitized path text through the shared utility. Help-overlay cancellation and combined precedence testing are owned by Issues #31 and #32.

---

### 3. Document the file-change pop-up

**Type**: DOCUMENT  
**Output**: Wiki documentation records the pop-up lifecycle, instance-keyed timers, and cancellation rules.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #15 implementation and tests into the appropriate pages under `Notes/wiki`. Document the selection-time start independent of load completion, the one-second-or-keypress lifetime with the key performing its normal action, instance-keyed timers with stale-instance rejection, render-time centring and truncation on resize without timer restart, error-overlay cancellation with no return, and the sanitized single-line path. Cross-reference Issue #15 and the Colours, overlays, and key precedence section of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the pop-up walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/015-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/015-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the instance-keyed model tests, then run the binary: `n` across a file boundary showing the centred pop-up and its ~1 s disappearance, a quick second `n` showing the new file's pop-up with the navigation still applied, a resize while shown re-centring it, and a hostile path rendering its escaped form. Reference Issue #15 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---

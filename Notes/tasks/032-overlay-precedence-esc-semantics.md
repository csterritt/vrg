# Tasks for #32: Overlay precedence — error suspends help, append preserves scroll, `Esc`/`q` dismissal semantics

Parent issue: #32
Parent PRD: PRD-vrg.md
**Blocked by issues**: #15, #26, #31
**Acceptance criteria**: AC1–AC7 → Tasks 1–2
**Manual verification**: Task 4 owns the issue's manual checks.

## Tasks

### 1. Specify overlay precedence and dismissal semantics

**Type**: RED  
**Output**: Failing model tests cover error-suspends-help with scroll restoration for both dismissal keys, append-preserving scroll, pop-up cancellation, `Esc` no-ops, the dismissal-outcome table run for `q` and `Esc`, and error-first key routing.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issues #15, #26, and #31 are complete. Add failing model tests in `internal/app` for the Issue #32 contracts and the precedence, `Esc`, and error-overlay bullets of the Colours, overlays, and key precedence section of `Notes/PRD-vrg.md`. Require a new error while help is open at scroll position S to suspend help, and either dismissal key to restore help at S; a second error appended to an overlay the reader has scrolled to position P to keep the reader at P with the new text reachable, generalizing the Issue #26 append-preserving-scroll primitive — already proven for reload re-entry failures — to all appended errors; opening help or an error to cancel any pop-up with no suspended pop-up returning; `Esc` with no overlay open in browsing and no-results to do nothing other than dismissing a pop-up; the dismissal-outcome table run for both `q` and `Esc` as the dismissal key — browse with an error overlay → browsing still running, browse with help → browsing, browse with error-over-help → help restored, empty result with a warning overlay → the no-results screen still running, no-results with help open → closing to no-results still running, fatal with no usable results → exit 2, record-loss with no results → exit 2 — with `Esc` never exiting from a base state and dismissing a fatal no-results overlay with `Esc` terminating with status 2 because there is no underlying state — followed by a second `q` from each still-running state exiting the fixed status or 1 and a second `Esc` leaving it running; and keys to route to the error when both help and error are open. Keep this task test-only.

---

### 2. Implement precedence and dismissal semantics

**Type**: GREEN  
**Output**: Precedence, suspension, append, and dismissal-outcome tests pass; `Esc` never exits from a base state, and dismissing a fatal no-results overlay with `Esc` terminates with status 2 because there is no underlying state.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the precedence stack in `internal/app` to satisfy Task 1: `ctrl+c` over modal error over help over pop-up over base keys, error suspension of help with scroll retention and restoration, appended errors preserving the reader's position by generalizing the Issue #26 append-preserving-scroll primitive to all appended errors, pop-up cancellation by help and error with no return, and the dismissal semantics per the PRD's reconciled rule — `Esc` never exits from a base state, while dismissing a fatal no-results overlay with `Esc` terminates with status 2 because there is no underlying state. The too-small screen's dedicated rule is owned by Issue #33.

---

### 3. Document overlay precedence

**Type**: DOCUMENT  
**Output**: Wiki documentation records the precedence stack, suspension and append rules, and the `Esc`/`q` dismissal table.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #32 implementation and tests into the appropriate pages under `Notes/wiki`. Document the key-precedence stack, error-suspends-help with position restoration, appended errors preserving the reader's scroll, pop-up cancellation without return, and the full dismissal-outcome table for `q` and `Esc` including the fatal-overlay exit-2 route — `Esc` never exits from a base state; dismissing a fatal no-results overlay with `Esc` terminates with status 2 because there is no underlying state — and the no-overlay `Esc` no-op. Cross-reference Issue #32 and the Colours, overlays, and key precedence and Outcome and exit-status contract sections of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the precedence walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/032-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/032-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the suspension, append, and dismissal-outcome model tests, then run the manual cases: `Esc` with no overlay doing nothing, `q` with a browse error overlay closing it and a second `q` exiting with the fixed status, a fake rg exiting 3 with no output showing the fatal overlay where both `Esc` and `q` exit 2, and the gated error-while-help-open case restoring help at its scroll position. Reference Issue #32 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---

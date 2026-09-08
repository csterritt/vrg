# Tasks for #33: "Terminal too small" screen with full state recovery

Parent issue: #33
Parent PRD: PRD-vrg.md
**Blocked by issues**: #32, #24
**Acceptance criteria**: AC1–AC5 → Tasks 1–2
**Manual verification**: Task 4 owns the issue's manual checks.

## Tasks

### 1. Specify the too-small screen and state recovery

**Type**: RED  
**Output**: Failing model tests cover the 20×3 threshold and display, exit semantics per state including overlay-open cases, `Esc` and other-key no-ops, full round-trip restoration including modal stacks and a resize wholly within the too-small state, and pop-up timer continuation.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issues #32 and #24 are complete. Add failing model tests in `internal/app` for the Issue #33 contracts and the minimum-size bullet of the Layout and indicators section of `Notes/PRD-vrg.md`. Require terminals below 20 columns or 3 rows to show centred "Terminal too small" as space permits with only `q` and `ctrl+c` active; `q` to exit 130 while searching, the fixed search-derived status while browsing, 1 on the no-results screen, and 2 with a fatal no-results overlay logically open — and with a browse error overlay logically open to exit the program rather than merely dismiss it; `Esc` and every other key to be no-ops, with a logically open overlay still open after recovery; growth to restore cursor selection, per-file viewport state and logical anchors, the list visibility preference, wrap and colour settings, horizontal offset, and full modal state — which overlay is open, the help and error scroll positions, and any help-suspended-by-error relationship — across round trips with a scrolled help overlay, a scrolled error overlay, and an error-over-help stack, all at their prior positions; a resize wholly within the too-small state — 19×2 → 10×1 → 25×8 — to install no ordinary layout at 10×1, mutate no anchors, and perform no partial modal restoration, with recovery using the final 25×8 dimensions while a scrolled help overlay, an error-over-help stack, and a nontrivial viewport state are preserved without loss; and an active pop-up's timer to continue during too-small with an expiry during it dismissing the pop-up so it is absent after recovery, and the pop-up never displayed on the too-small screen. Keep this task test-only.

---

### 2. Implement the too-small gate

**Type**: GREEN  
**Output**: Threshold, exit, no-op, recovery, and pop-up tests pass; the screen never traps the user behind an invisible modal.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the too-small gate in `internal/app` to satisfy Task 1: the threshold check ahead of normal layout, the centred message, the active-key restriction with `q`'s state-applicable outcome taking precedence over Issue #32's dismissal semantics, full state preservation behind the gate — with resizes wholly within the too-small state keeping the gate installed and deferring recovery to the final dimensions — and pop-up timers continuing without display. All other input is a no-op at this size.

---

### 3. Document the too-small screen

**Type**: DOCUMENT  
**Output**: Wiki documentation records the threshold, active keys, preserved state, and the precedence rule over dismissal semantics.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #33 implementation and tests into the appropriate pages under `Notes/wiki`. Document the 20×3 minimum, the centred message, the `q` and `ctrl+c` exits with their state-applicable statuses, the `Esc` no-op, the complete preserved-state list including modal state and scroll positions, pop-up timer continuation, and the rule that too-small `q` exits rather than dismissing a logically open overlay. Cross-reference Issue #33 and the Layout and indicators section of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the too-small walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/033-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/033-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the threshold, recovery, and exit-semantics model tests, then run the manual cases: opening help, scrolling it, shrinking to 15×2 showing "Terminal too small", enlarging with help reappearing at the same scroll, shrinking again and pressing `q` to exit. Reference Issue #33 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---

# Tasks for #28: Load-completion reveal to the latest selected target through the prepared layout

Parent issue: #28
Parent PRD: PRD-vrg.md
**Blocked by issues**: #15, #17, #19, #21, #23, #27
**Acceptance criteria**: AC1–AC6, AC9–AC10 → Tasks 1–2; AC7–AC8 → Tasks 3–4
**Manual verification**: Task 6 owns the issue's manual checks.

## Tasks

### 1. Specify the two-stage load-completion contract

**Type**: RED  
**Output**: Failing model tests with separately gated loads and layouts cover startup hidden and visible targets, resize between stages, navigation-during-pending intents, out-of-order revision discards, gutter-growth and list-toggle width changes, saved-viewport revisits, marker and cluster targets, and non-current isolation.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issues #15, #17, #19, #21, #23, and #27 are complete. The generic reload-anchor pending intent and its installation-gated commit arrive already tested from Issue #27; these tests build on that seam rather than retroactively supplying it, generalizing it to reveal intents and the full two-stage arbitration. Add failing model tests in `internal/app` with separately gated loads and layout preparation for the Issue #28 contracts and the second File loading bullet plus the Navigation reveal-sequence bullet of `Notes/PRD-vrg.md`. Require load completion for the current file to validate content, establish the new content revision, compute the final gutter width, recompute the text width with gutter growth and Issue #24 list changes participating, and request a prepared layout for the current (path, revision, text width, wrap mode) while performing no row-based decision itself — no visibility test, placement, clamping, or horizontal reveal; the reveal intent to target the latest selected cursor's final display target — the cluster-expanded first-submatch start cell from Issue #21, or the marker cell for a zero-width or terminator-only match from Issue #23 — never a target captured when the load was requested; obsolete layouts to be discarded without consuming or mutating the intent; the commit, when and only when a matching layout installs, to apply visible-target no-scroll, else one-third placement with BOF/EOF clamping, plus horizontal reset then minimal horizontal reveal, all against the installed row model; a startup target hidden from top 0 to land at `floor(h / 3)` and a visible one to keep top 0 with the first `n` advancing to the second stop; a resize between load and layout to discard the old-width layout while the intent survives to commit at the new width; `n`/`p` while the layout is pending to move the cursor immediately with the newest target revealed on commit; gutter growth or list hide/show between the stages to commit against the final text width; a saved-viewport revisit with the target visible to stay and hidden to move; a terminator-only marker target far down to reveal the marker's row with the marker cell painted in run-off-edge mode; a mid-cluster target at column 300 to paint the cluster fully at the right edge; a completion for a non-current file to leave the panel untouched; and the pop-up to be unaffected by either stage. Keep this task test-only.

---

### 2. Implement the two-stage completion

**Type**: GREEN  
**Output**: Two-stage tests pass; the viewport never changes at load-completion time and commits only on a matching installed layout.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the two-stage contract in `internal/app` to satisfy Task 1: stage-one completion limited to validation, revision, gutter, text-width recomputation, and the layout request; the model-carried reveal intent for the latest selection with its final target geometry, generalizing the Issue #27 reload-anchor pending-intent seam into the full two-stage reveal-versus-reload arbitration; the installation-guarded commit applying the Issue #14 and #19 rules against the installed rows; and the starting viewports — top-of-file with horizontal offset zero for first visits including startup, saved per-file viewport for revisits. Stale-entry fallback targets arrive with Issue #29.

---

### 3. Specify reload-intent transitions

**Type**: RED  
**Output**: Failing gated tests cover no-navigation anchor preservation, navigation-during-reload latest-reveal precedence, away-and-back same-file and cross-file entry reveals, and old-revision layout discards.  
**Depends on**: 2

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Add failing gated tests for the Issue #28 reload transitions. Require a reload with no intervening navigation to preserve the prior anchor on commit with no reveal; navigation during the load — including away-and-back within the same file (A→B→A with the match scrolled off-screen before `r`) and across files (A→file B→A) — to replace the reload intent with the entry reveal for the latest selection, so the committed reveal proves navigation intent rather than cursor equality decides; old-revision and new-revision layouts completing out of order to leave the anchor or reveal committed only against the new revision with the late old-revision layout discarded without consuming the intent; and a return to A during the reload to show "Loading…" with the entry reveal applying on commit. Keep this task test-only.

---

### 4. Implement reload-intent transitions

**Type**: GREEN  
**Output**: Reload-intent tests pass with navigation intent, not cursor equality, deciding the commit.  
**Depends on**: 3

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the reload-intent transitions in `internal/app` to satisfy Task 3: intent replacement by any navigation during a load — including away-and-back sequences whose final cursor equals the initial one — the anchor-preservation commit for undisturbed reloads, and revision-ordered installation for out-of-order layouts.

---

### 5. Document the two-stage completion contract

**Type**: DOCUMENT  
**Output**: Wiki documentation records both stages, the intent model, the commit rules, and the reload transitions.  
**Depends on**: 4

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #28 implementation and tests into the appropriate pages under `Notes/wiki`. Document stage one's limited responsibilities, the latest-selection reveal intent with its marker and cluster-expanded target geometry, obsolete-layout discards that never consume intents, the installation-guarded commit with its starting viewports, the startup visible-versus-hidden rules, and the reload transitions where navigation intent rather than cursor equality decides. Cross-reference Issue #28 and the File loading and Navigation sections of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 6. Create the completion walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/028-06/code-walkthrough`.  
**Depends on**: 5

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/028-06/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the gated two-stage and reload-transition tests, then run the manual cases: a slow first file with its first match at line 500 loading to place it about a third down, a first match at line 3 keeping top 0, a reload with an immediate `n` revealing the new stop rather than the old position, a scrolled-off match with `r` then `n` `p` revealed on completion, and a resize during the first file's load landing the reveal correctly for the new size. Reference Issue #28 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---

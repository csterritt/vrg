# Tasks for #27: `r` explicit reload — dropped duplicates, anchor preservation, failure replaces content

Parent issue: #27
Parent PRD: PRD-vrg.md
**Blocked by issues**: #26, #17
**Acceptance criteria**: AC1–AC7 → Tasks 1–2
**Manual verification**: Task 3 owns the issue's manual checks.

## Tasks

### 1. Specify explicit reload

**Type**: RED  
**Output**: Failing gated model tests cover the reload load lifecycle with dropped duplicates, anchor preservation asserted after the matching layout installs, clamping on shrink, failure replacement, the one-stop route, disk-change stability, and revision supersession.

**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issues #26 and #17 are complete. Add failing gated model tests in `internal/app` for the Issue #27 contracts and the reload bullets of the File loading section of `Notes/PRD-vrg.md`. Require `r` to show "Loading…" and issue exactly one reread of the current file without rerunning rg or changing cursor stops; a duplicate `r` or re-entry while that path's load is in flight to be dropped, not queued, with the placeholder's change the only completion signal after which `r` starts a new load; the cursor and logical viewport anchor to be preserved on completion, clamped to new content, asserted after the new revision's matching prepared layout installs rather than against the old revision's layout; a failed reload to replace the old display with "(unreadable)" and show the current-file failure overlay, with a second consecutive failure appending exactly one new occurrence with the reader's position preserved; `r` to work with a one-stop index; no reload to be triggered by simulated disk changes without `r`; the filename row to keep identifying the path; and a reload to produce a new content revision with a gated pre-reload layout released after the reload completes discarded without replacing the reloaded content or its anchor — the Issue #17 revision-superseded test. Keep this task test-only.

---

### 2. Implement explicit reload

**Type**: GREEN  
**Output**: Reload tests pass; reload preserves the anchor through the pending-intent commit owned here and never presents stale content as refreshed.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement `r` in `internal/app` to satisfy Task 1: the reload request with its dropped duplicates, content revisions feeding the Issue #17 layout keying, failure replacement through the Issue #26 overlay component, the one-stop route, and the generic reload-anchor pending intent — preserve the anchor, no reveal — recorded when the reload's load completes and committed only when the new revision's matching prepared layout installs through the existing Issue #17 installation path. This pending-intent seam is owned by this issue so Task 1's anchor-preservation tests pass before Issue #28 begins; Issue #28 later generalizes the same seam into the full two-stage reveal-versus-reload arbitration for all load completions. Validation of original matches against new content is owned by Issue #29.

---

### 3. Create the reload walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/027-04/code-walkthrough`.  
**Depends on**: 2

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/027-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the gated reload and revision-supersession tests, then run the manual cases: opening a file, scrolling, appending lines externally with the display unchanged, `r` showing new content at the same top position, deleting the file and pressing `r` for "(unreadable)", restoring it and pressing `r` for content again, and a single-match search reloading via `r`. Reference Issue #27 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---

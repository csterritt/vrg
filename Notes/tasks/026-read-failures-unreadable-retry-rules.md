# Tasks for #26: Read failures — "(unreadable)", current vs non-current notification, retry rules

Parent issue: #26
Parent PRD: PRD-vrg.md
**Blocked by issues**: #9, #11, #24, #25
**Acceptance criteria**: AC1–AC3, AC7–AC8 → Tasks 1–2; AC4–AC6 → Tasks 3–4
**Manual verification**: Task 6 owns the issue's manual checks.

## Tasks

### 1. Specify read-failure notification and retry rules

**Type**: RED  
**Output**: Failing model tests with an injected failing loader cover current versus non-current notification, later-visit presentation, the same-file-step no-retry rule, entry-from-different-file retries, composed-view robustness, and the new outcome-matrix rows including replay presence.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issues #9, #11, #24, and #25 are complete. Add failing model tests in `internal/app` using an injected failing loader rather than filesystem permissions, for the Issue #26 contracts and the failure and retry bullets of the File loading section of `Notes/PRD-vrg.md`. Require a current-file read failure to show the Issue #9 error overlay and the "(unreadable)" placeholder while its cursor stops are retained and the filename row still identifies the path; a non-current failure to be collected as a diagnostic only with no overlay and no indicator, discovered by visiting that file or through Issue #11 replay; a same-file `n`/`p` step to request no reload while entry from a different file requests exactly one; the composed view to stay well-formed at constrained widths with the truncated safe path per Issue #24's slot rules, the placeholder shown, nothing overflowing, and layout dimensions nonnegative; and the new outcome-matrix rows — every retained file failing to load with fixed status 0 still exiting 0, and a current-file failure with fixed status 2 still exiting 2 — plus the non-current failure diagnostic appearing in the replay. Keep this task test-only.

---

### 2. Implement read-failure states and retry rules

**Type**: GREEN  
**Output**: Notification, retry, composed-view, and outcome-row tests pass.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the read-failure states in `internal/app` to satisfy Task 1: the current-file overlay and placeholder with retained stops, the non-current diagnostic-only collection, the same-file versus cross-file retry distinction, the composed-view robustness through Issue #24's slot, and the outcome-matrix rows proving load failures never change the fixed status. The `r` retry route is owned by Issue #27.

---

### 3. Specify the gated re-entry retry sequence

**Type**: RED  
**Output**: Failing gated model tests cover the immediate prior-failure overlay with "Loading…", exactly one in-flight retry, dismissal without disturbing the load, settlement presentation, and the exactly-one appended occurrence on a second failure.  
**Depends on**: 2

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Add failing gated model tests for the Issue #26 re-entry sequence. Require entering a previously failed file from a different file to show the prior-failure overlay immediately with the panel switching from "(unreadable)" to "Loading…", exactly one retry load starting while the overlay is open, `Esc` dismissing the overlay without disturbing the in-flight load, settlement updating the placeholder to content or "(unreadable)" without waiting for dismissal, a successful retry collecting nothing new with the prior-failure overlay remaining displayed until dismissed, and a second failure appending exactly one new diagnostic occurrence to the open overlay with the reader's scroll position preserved and exactly one new occurrence collected for replay. Include navigation away during the retry settling per Issue #25 and a later re-entry following the same sequence against the new prior state. Keep this task test-only.

---

### 4. Implement the re-entry sequence

**Type**: GREEN  
**Output**: Gated re-entry presentation and append tests pass.  
**Depends on**: 3

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the deterministic re-entry sequence in `internal/app` to satisfy Task 3, reusing the Issue #25 one-load-per-path rule when a load is somehow already in flight and implementing the minimal error-overlay append-preserving-scroll primitive owned by this issue: appending the second failure's single new occurrence to the open overlay without moving the reader's scroll position. Issue #32 later generalizes this primitive to all appended errors; do not wait for Issue #32 or implement its help/precedence semantics here.

---

### 5. Document read failures and retries

**Type**: DOCUMENT  
**Output**: Wiki documentation records failure notification, retry rules, the re-entry sequence, and the fixed-status guarantee.  
**Depends on**: 4

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #26 implementation and tests into the appropriate pages under `Notes/wiki`. Document the current-file overlay and placeholder, the non-current diagnostic-only policy and its mid-session visibility limits, the same-file-step versus cross-file retry distinction, the five-step re-entry sequence with its exactly-one retry and append rules, composed-view robustness, and load failures never changing the fixed exit status including the all-fail and fixed-2 cases. Cross-reference Issue #26 and the File loading, cache, reload, and selection consistency section of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 6. Create the read-failure walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/026-06/code-walkthrough`.  
**Depends on**: 5

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/026-06/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the injected-loader notification and outcome-row tests and the gated re-entry sequence tests, then, on an unprivileged shell, run the manual route against a disposable temporary fixture directory — never a repository file — by copying the two matched files into it, recording the original mode, and running every step under a shell trap that restores the original mode on exit or interruption: making the second matched file unreadable with `chmod 000`, startup showing file 1, `n` into file 2 showing the overlay and "(unreadable)", `Esc`, a same-file `n` with no new overlay, `p` `p` back into file 2 showing the overlay with "Loading…", dismissal, and `q` exiting 0 with the failures listed on stderr. Completing or interrupting the walkthrough must leave no file with altered permissions, and the injected-loader model tests remain the authoritative deterministic verification. Reference Issue #26 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---

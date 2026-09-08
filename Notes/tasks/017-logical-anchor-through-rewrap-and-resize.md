# Tasks for #17: Width-independent logical anchor through rewrap, wrap toggle and resize; lossy EOF clamp; off-UI layout preparation and obsolete-layout isolation

Parent issue: #17
Parent PRD: PRD-vrg.md
**Blocked by issues**: #16
**Acceptance criteria**: AC1–AC5 → Tasks 1–2; AC6–AC11 → Tasks 3–4
**Manual verification**: Task 6 owns the issue's manual checks.

## Tasks

### 1. Specify the logical anchor and lossy EOF clamp

**Type**: RED  
**Output**: Failing Viewport tests cover width round trips, wrap-toggle round trips, anchor replacement by scroll and by reveal, the deliberate EOF-clamp loss, and cursor preservation on resize.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issue #16 is complete. Add failing tests in `internal/viewport` and `internal/app` for the anchor half of Issue #17 and the final Navigation, viewport, and logical anchors bullets of `Notes/PRD-vrg.md`. Require the logical anchor (source line, display-column offset) to be width-independent so a resize or rewrap lands the effective top on the row containing the anchor's text location rather than the row with the same former ordinal; wrap off then on to preserve the logical column when no scroll, reveal, or clamp intervened; user vertical scrolling to replace the anchor with the resulting top row's location; a match reveal that moves the viewport likewise to replace it while a no-scroll reveal does not discard a retained logical column; EOF clamping to pull the effective top upward and update the anchor to that top, with the intentional loss that a subsequent shrink need not restore the old top; and any resize to preserve the cursor selection. Keep this task test-only.

---

### 2. Implement the logical anchor

**Type**: GREEN  
**Output**: Anchor round-trip, replacement, EOF-clamp, and cursor-preservation tests pass.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the logical anchor in `internal/viewport` to satisfy Task 1: anchor retention and restoration through rewrap, wrap toggles, and resizes; anchor replacement by user scrolling and moving reveals; the lossy EOF clamp updating the anchor; and cursor preservation on resize in the App model.

---

### 3. Specify off-UI layout preparation and obsolete-layout isolation

**Type**: RED  
**Output**: Failing gated tests cover every AC6 input during preparation — `ctrl+c`, `q`, `n`/`p`, `w`, and a second resize — pending reveal intents, out-of-order and superseded layouts, cached-file stale-layout navigation with its fast path, and render-cost guards for rows and list entries.  
**Depends on**: 2

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Add failing tests for the preparation half of Issue #17 and the Responsiveness boundaries decisions of `Notes/PRD-vrg.md`. With the layout worker held by a test gate after a resize, require `ctrl+c` to exit 130, `q` to exit with the fixed status, `n` to advance the cursor immediately and `p` to move to the previous stop immediately with the latest pending reveal preserved for whichever stop is newest, `w` to change wrap mode immediately and issue (or replace) the keyed layout request for the new mode without releasing the existing gate, and a second resize to be accepted, all without releasing the gate, and a stop selected while the gate was held to be revealed per the Issue #14 rules once a layout matching the current parameters installs — the pending intent preserved, not lost. Require prepared layouts keyed by (path, content revision, text width, wrap mode) to install only when the key equals the model's current parameters, with out-of-order completions across W1→W2→W3 resizes and rapid wrap toggles discarded and the anchor unaffected throughout; a layout for a file that is no longer current to leave the visible panel and saved per-file state untouched; navigation to a cached file whose installed layout is stale to request a prepared layout for the current parameters and carry the entry-reveal or saved-viewport intent to commit on installation, while a cached file with a matching installed layout commits immediately with no request; and counting-fake render-cost guards proving neither the row provider nor the file-list item provider is called beyond the visible range. Keep this task test-only.

---

### 4. Implement prepared layouts and pending intents

**Type**: GREEN  
**Output**: Gated preparation, isolation, intent, and render-cost tests pass with every AC6 input actionable while a worker is held and obsolete layouts never touching the display or consuming intents.  
**Depends on**: 3

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement prepared-layout jobs completing via messages like file loads, keyed installation guards, the model-carried reveal intent committed with the Issue #14 rules against the installed row model, every AC6 input — `ctrl+c`, `q`, `n`/`p`, `w`, and further resizes — handled without waiting for a held worker, and frame rendering from installed prepared data and the list's visible window. Do not prescribe a particular goroutine or channel design — the contract is where the work happens and how late results are isolated. The revision-supersession test with a real reload lands with Issue #27, but the revision keying must be in place now.

---

### 5. Document the logical anchor and layout preparation

**Type**: DOCUMENT  
**Output**: Wiki documentation records the anchor model, the lossy EOF rule, prepared layouts, installation guards, and pending intents.  
**Depends on**: 4

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #17 implementation and tests into the appropriate pages under `Notes/wiki`. Document the width-independent logical anchor with its replacement rules, the intentionally lossy EOF clamp, off-UI layout preparation keyed by path, content revision, text width, and wrap mode, installation-only-on-match guards with out-of-order discards, the model-carried pending reveal intent and its commit, cached-file stale-layout navigation with its fast path, and the render-cost guarantees. Cross-reference Issue #17 and the Navigation, viewport, and logical anchors and Resources and responsiveness sections of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 6. Create the anchor-and-layout walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/017-06/code-walkthrough`.  
**Depends on**: 5

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/017-06/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the anchor round-trip and EOF-clamp tests, the gated preparation and out-of-order isolation tests, and the cached-file stale-layout tests, then run the binary: scroll partway into a wrapped long line, narrow and widen the terminal with the same text staying at the top, press `w` twice with the same text, scroll to EOF then widen showing the top moving up and staying there, and resize a ~50 MB fixture repeatedly with `ctrl+c` mid-rewrap exiting promptly with 130. Reference Issue #17 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---

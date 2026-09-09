# Tasks for #6: Safe-presentation utility for every output sink

Parent issue: #6
Parent PRD: PRD-vrg.md
**Blocked by issues**: #5
**Acceptance criteria**: AC1–AC8 → Tasks 1–2
**Manual verification**: Task 4 owns the issue's manual checks.

## Tasks

### 1. Specify diagnostic escaping and the shared sink-safety table

**Type**: RED  
**Output**: Failing tests cover diagnostic line preservation, tab expansion, single-lined embedded filenames, the restructured sink-safety table over the five existing sinks (including CLI-help stdout), and unchanged re-runs of the Issue #5 core cases and the Issue #1 CLI output tests against the generalized utility.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issue #5 is complete. Add failing tests for the Issue #6 generalization and the Text, graphemes, and safe presentation sanitization bullets of `Notes/PRD-vrg.md`. Require diagnostics to preserve real line boundaries, expand tabs, escape other controls safely, and embed any filename single-line-escaped first; restructure the Issue #5 hostile fixture set — OSC, CSI, C0, C1, DEL, standalone CR, invalid UTF-8 path bytes, and an embedded filename newline — into a shared, extensible table test covering every sink existing at this point (file-list entry, filename rule, panel content, usage-error stderr, and the Issue #1 generated command-line help on stdout — a sink distinct from the Issue #31 TUI help dialog that adds its own row later), asserting on raw output before any ANSI stripping that no fixture control byte survives, rendering through the no-style composition path so no escape byte may legitimately appear, and additionally with styles enabled asserting the fixture's distinctive payload never appears immediately after an unescaped ESC. Include regression requirements that the Issue #5 core escaping tests, browse sink-safety rows, and Issue #1 CLI output tests (including CLI-help stdout) pass unchanged against the generalized utility, and that the table is structured so later issues add sink rows without duplicating fixtures. Keep this task test-only.

---

### 2. Implement the shared safe-presentation utility

**Type**: GREEN  
**Output**: Diagnostic, sink-safety table, and regression tests pass with the Issue #1 escaper replaced and every existing sink routed through the single utility.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Generalize the Issue #5 core into the single shared utility, unify the minimal Issue #1 escaper into it, and wire every sink existing at this point — file list, filename rule, panel content, usage errors, and the Issue #1 generated command-line help on stdout — through it. Extend it with the diagnostic presentation rules from Task 1, preserving line boundaries, expanding tabs, and single-lining embedded filenames. Do not regress any Issue #5 test or Issue #1 CLI output test, and keep the fixture table extensible for the sinks owned by Issues #9, #11, #15, #31, and #34, which are responsible for adding their own rows.

---

### 3. Document the safe-presentation utility

**Type**: DOCUMENT  
**Output**: Wiki documentation records the canonical escaping contracts per sink class, the shared utility, the sink-safety table, and later-sink ownership.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #6 utility implementation and tests into the appropriate pages under `Notes/wiki`. Document the canonical path and content escaping contracts, the diagnostic rules with single-lined embedded filenames, the replacement of the Issue #1 escaper, the shared sink-safety table with its fixtures and no-style composition method, and the rule that each later issue routes its new sinks through the utility and extends the table. Cross-reference Issue #6 and the Text, graphemes, and safe presentation section of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the safe-presentation walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/006-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/006-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the unit tests for each path and content rule, the diagnostic escaping tests, the shared sink-safety table across every existing sink including CLI-help stdout, and the regression runs of the unchanged Issue #5 cases and Issue #1 CLI output tests, plus the manual checks: the hostile filename and content fixture showing escaped forms with the terminal title unchanged, and a usage error with a control-byte path rendering escaped on stderr with exit 2. Reference Issue #6 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---

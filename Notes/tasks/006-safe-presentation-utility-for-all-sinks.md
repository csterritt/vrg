# Tasks for #6: Safe-presentation utility for every output sink

Parent issue: #6
Parent PRD: PRD-vrg.md
**Blocked by issues**: #5
**Acceptance criteria**: AC1–AC8 → Tasks 1–2

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

### 3. Create the safe-presentation finish marker

**Type**: FINISH MARKER  
**Output**: Finish marker exists at `Notes/finish-markers/006-04/finish-marker.md`.  
**Depends on**: 2

Write `Task 006-04 finished successfully at <time>` to `Notes/finish-markers/006-04/finish-marker.md`, replacing `<time>` with the current UTC timestamp (for example, `date -u +"%Y-%m-%dT%H:%M:%SZ"`). Create the `Notes/finish-markers/006-04/` directory if it does not already exist.

---

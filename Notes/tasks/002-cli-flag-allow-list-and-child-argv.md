# Tasks for #2: CLI flag allow-list, combined shorts, cumulative `-u`, `--`, ordered child argv

Parent issue: #2
Parent PRD: PRD-vrg.md
**Blocked by issues**: #1
**Acceptance criteria**: AC1–AC8 → Tasks 1–2
**Manual verification**: Task 4 owns the issue's manual checks.

## Tasks

### 1. Specify the flag allow-list and protected child argv

**Type**: RED  
**Output**: Failing table-driven tests cover every accepted and rejected flag, combined-short expansion order, cumulative `-u` boundaries across mixed tokens, option placement, `--` handling including the literal `--` pattern rows, dash-leading patterns, the verbatim empty pattern, and the exact child argv.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issue #1 is complete. Both tasks extend `internal/cli`. Add table-driven tests for the Issue #2 contracts and the Invocation and child arguments section of `Notes/PRD-vrg.md`. Cover each allow-listed flag in short and long form (`-i/--ignore-case`, `-S/--smart-case`, `-s/--case-sensitive`, `-w/--word-regexp`, `-x/--line-regexp`, `-F/--fixed-strings`, `--hidden`, `--no-hidden`, `--no-ignore`, `-u/--unrestricted`, `-L/--follow`) forwarded in user order, rejected samples including `-e` and argument-taking options, combined-short expansion order (`-iwF` expanding to `-i -w -F`), cumulative unrestricted counting across short, combined, and long tokens with the boundaries `-u`, `-uu`, `-uuu`, `-u -uu`, `-iu`, `-iuu`, and `-iuuu`, options accepted after the pattern, `--` ending option parsing so dash-leading patterns are protected, the literal `-` pattern, a positional whose bytes are themselves `--` — `vrg -- --` with the default root and `vrg -- -- .` with an explicit root both parsing the second `--` as the verbatim pattern rather than another option terminator, with the child argv containing the mandatory separator followed by a literal `--` pattern — and the empty pattern forwarded verbatim as an empty argv element after `--`. Require the exact child argv `--json --no-config <ordered user flags> -- <pattern> <root>` and exit 2 with a sanitized diagnostic for every rejection. Keep this task test-only.

---

### 2. Implement flag parsing and child argv construction

**Type**: GREEN  
**Output**: Allow-list, combined-short, cumulative `-u`, `--`, and exact-argv tests pass; the stub prints the child argv.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement only enough flag handling in `internal/cli` to satisfy Task 1. Enforce the exact no-argument allow-list, expand combined short flags in their original order, count unrestricted occurrences across all tokens and reject a third, accept options anywhere before `--`, and treat the first two positionals as pattern then root with `--` protecting dash-leading patterns and every token after the first terminator — including one spelled `--` — positional. Build the child argument vector in the mandated order with the mandatory `--json` and `--no-config` internal flags first, preserve user flag order without normalizing contradictory settings, and update the Issue #1 stub to print the constructed argv on success. Do not spawn rg or add any TUI behavior owned by later issues.

---

### 3. Document the flag and argv contract

**Type**: DOCUMENT  
**Output**: Wiki documentation records the allow-list, combined shorts, cumulative `-u`, `--` including the literal-`--` positional rule, and the exact child argv order.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #2 `internal/cli` implementation and tests into the appropriate pages under `Notes/wiki`. Document the no-argument flag allow-list in both forms, combined-short expansion in original order, the cumulative unrestricted count across mixed short, combined, and long tokens with the two-allowed boundary, option placement anywhere before `--`, protection of dash-leading patterns and the literal `-` pattern, the distinction between the first `--` option terminator and a subsequent positional whose bytes are also `--` — `vrg -- --` with the default root and `vrg -- -- .` with an explicit root both parse the second `--` as the verbatim pattern, forwarded in the child argv as a literal `--` element after the mandatory separator — the verbatim empty argv element, rejection of every other option with sanitized diagnostics and exit 2, and the exact child argv with mandatory internal flags first. Cross-reference Issue #2 and the Invocation and child arguments section of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the flag-parsing walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/002-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/002-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the focused `internal/cli` tests and manual runs of the built binary: `vrg -iw foo src` printing `rg --json --no-config -i -w -- foo src`, an option after the pattern accepted and ordered, `-uu` accepted against `-uuu` and `-u -uu` rejected, the combined-short boundaries `-iu`, `-iuu`, and `-iuuu`, `-e` and `--type go` rejected as unsupported flags, `-- -foo` accepted, `- .` with the literal `-` pattern, `-- --` and `-- -- .` with the literal `--` pattern, and `vrg "" .` forwarding the empty pattern verbatim. Reference Issue #2 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---

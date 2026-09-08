## Issue 2: CLI flag allow-list, combined shorts, cumulative `-u`, `--`, ordered child argv

**Type**: AFK
**Blocked by**: Issue 1

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

Extend the CLI module to accept the documented no-argument ripgrep flags and produce the protected child argument vector.

- Allow-list exactly: `-i/--ignore-case`, `-S/--smart-case`, `-s/--case-sensitive`, `-w/--word-regexp`, `-x/--line-regexp`, `-F/--fixed-strings`, `--hidden`, `--no-hidden`, `--no-ignore`, `-u/--unrestricted`, `-L/--follow`. Every other option (including `-e` and argument-taking options) is a usage error.
- Options may appear anywhere before `--`; expand combined short flags (`-iw`) in original order; preserve the user's order in the child argv.
- Count unrestricted occurrences across all tokens (short, combined, long). Zero to two allowed; a third is a usage error (`-uuu`, `-u -uu`, `-u --unrestricted -u`).
- `--` ends option parsing. A dash-leading pattern other than the lone `-` requires `--`; the literal pattern `-` is valid; a pattern spelled `--` supplied after the terminator is the verbatim pattern, not a second terminator — all tokens after the first `--` are positional.
- Child argv order: `--json`, `--no-config`, ordered user flags, `--`, pattern, root.
- The stub from Issue 1 now prints the child argv instead of just pattern/root.

See PRD *Implementation Decisions → Invocation and child arguments* (bullets 2–5, 7) and *Module Design → CLI*.

### How to verify

- **Manual**:
  1. `vrg -iw foo src` prints argv `rg --json --no-config -i -w -- foo src`.
  2. `vrg foo -i src` (option after pattern) is accepted and ordered.
  3. `vrg -uu foo` accepted; `vrg -uuu foo` and `vrg -u -uu foo` exit 2. Combined-short unrestricted: `vrg -iu foo` accepted (one occurrence inside a combined token); `vrg -iuu foo` accepted (two); `vrg -iuuu foo` exits 2 (three).
  4. `vrg -e foo` and `vrg --type go foo` exit 2 with "unsupported flag".
  5. `vrg -- -foo` accepted with pattern `-foo`; `vrg -foo` exits 2; `vrg - .` accepted with pattern `-`; `vrg -- --` and `vrg -- -- .` accepted with the literal pattern `--` (the second `--` is the pattern, not another terminator).
  6. `vrg "" .` prints argv `rg --json --no-config --  .` — the empty pattern is forwarded verbatim as an empty argv element after `--`.
- **Automated**: table tests for every accepted flag (short and long), every rejected sample, combined-flag expansion order, cumulative `-u` across mixed tokens including the combined-short boundary cases `-iu`, `-iuu` (accepted) and `-iuuu` (rejected), `--` behaviour including the literal `--` pattern — `vrg -- --` with the default root and `vrg -- -- .` with an explicit root both parse the second `--` as the verbatim pattern, the child argv containing the mandatory separator followed by a literal `--` pattern — dash-leading patterns, the empty pattern forwarded verbatim as an empty argv element, and the exact child argv including mandatory internal flags first.

### Acceptance criteria

- [ ] Given any allow-listed flag in short or long form, when parsed, then it is forwarded in its original position relative to other user flags.
- [ ] Given a combined short token such as `-iwF`, then it expands to `-i -w -F` in that order.
- [ ] Given two cumulative unrestricted flags in any token mix, then parsing succeeds; given three or more, then exit 2 with a usage diagnostic.
- [ ] Given a non-allow-listed option, then exit 2 and no TUI.
- [ ] Given `--`, then subsequent tokens are positionals even if dash-leading — including a token spelled `--` itself.
- [ ] Given a successful parse, then the child argv is exactly `--json --no-config <user flags…> -- <pattern> <root>`.
- [ ] Given a combined short token containing unrestricted flags such as `-iu` or `-iuu`, then the unrestricted count accumulates per occurrence within the token and the cumulative boundary rules apply (two accepted, three rejected).
- [ ] Given an empty pattern argument, then the child argv forwards it verbatim as an empty element after `--`.

### User stories addressed

- User story 4: documented no-argument flags and combined shorts
- User story 5: options anywhere before `--`, forwarded in order
- User story 6: `--` protects dash-leading patterns
- User story 7: third cumulative `-u` rejected
- User story 8 (part): ripgrep configuration ignored via `--no-config`

---

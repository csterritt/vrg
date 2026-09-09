## Issue 2: CLI flag allow-list, combined shorts, cumulative `-u`, `--`, ordered child argv

**Type**: AFK
**Blocked by**: Issue 1

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

Extend the CLI module to accept the documented no-argument ripgrep flags and produce the protected child argument vector.

- Allow-list exactly: `-i/--ignore-case`, `-S/--smart-case`, `-s/--case-sensitive`, `-w/--word-regexp`, `-x/--line-regexp`, `-F/--fixed-strings`, `--hidden`, `--no-hidden`, `--no-ignore`, `-u/--unrestricted`, `-L/--follow`. These search flags alone are forwarded to the child argv. Local `-h`/`--help` are not search flags: Issue 1's help paths own them, they are never forwarded, and they are never rejected by this allow-list — a help spelling anywhere before `--` still yields the help-only result even when mixed with search flags (for example `vrg -i --help`, `vrg -ih pattern`), because help resolution precedes all allow-list validation. Every other option (including `-e` and argument-taking options) is a usage error. The upstream parser accepts `=`-assignment forms for boolean-valued options (`--ignore-case=false` and `-i=false` parse, delivering `false` to the value); the allow-list is a **no-argument** contract, so these spellings — and `--unrestricted=false`, and help assignment forms such as `--help=false` — must be rejected lexically by the scan, before library permissiveness can broaden the CLI.
- Options may appear anywhere before `--`. Build ordered forwarding on Issue 1's raw-token preflight scan, extended to search flags: scan argv left to right up to the first `--`, consult the shared option-declaration table (the same declarations that configure `mow.cli` and generate its help — not an independently maintained allow-list), record each accepted spelling in encounter order, and expand combined short tokens (`-iw`, `-isi`) left to right within the token. Do not derive order or counts from `VarOpt` callbacks: the library fills option values per container after the whole parse, from a map, so cross-option encounter order is lost (`-i -s -i` is unreconstructible from callbacks) and callback order is nondeterministic. The library remains responsible for the declared syntax and parsing; the scan supplies what it provably cannot preserve — encounter order, exact spellings, and cumulative counts.
- Count unrestricted occurrences across all tokens (short, combined, long) from the scan records, not from callbacks — the library itself does not limit repetitions (`-uuu` parses upstream). Mixed aliases contribute to the same count. Zero to two allowed; a third is a usage error (`-uuu`, `-u -uu`, `-u --unrestricted -u`, `--unrestricted -uu`).
- `--` ends option parsing. A dash-leading pattern other than the lone `-` requires `--`; the literal pattern `-` is valid; a pattern spelled `--` supplied after the terminator is the verbatim pattern, not a second terminator — all tokens after the first `--` are positional.
- Child argv order: `--json`, `--no-config`, ordered user flags, `--`, pattern, root.
- Search flags appear in the generated command-line help: the shared declarations extend Issue 1's help with every allow-listed flag in short and long form, without changing the emission path, help-only precedence, or the Issue 1 output tests.
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
  7. `vrg -i -s -i foo` and `vrg -isi foo` print `rg --json --no-config -i -s -i -- foo .`; `vrg --ignore-case -s -i foo` prints `rg --json --no-config --ignore-case -s -i -- foo .` — encounter order and the spellings as supplied are preserved across repeated and mixed-alias options, including options interleaved with both operands (`vrg foo -i src -s` → `rg --json --no-config -i -s -- foo src`).
  8. `vrg -u --unrestricted foo` is accepted (two occurrences across mixed aliases); `vrg --unrestricted -uu foo` exits 2 (three).
  9. Help regression: `vrg`, `vrg -h`, `vrg --help`, `vrg -i --help`, `vrg -ih foo`, and `vrg foo src --help` all print help on stdout and exit 0 with empty stderr and no child; `vrg -i` (flags only, no pattern, no help) exits 2 with the missing-pattern diagnostic.
  10. Assignment rejection: `vrg --ignore-case=false foo`, `vrg -i=false foo`, `vrg --unrestricted=false foo`, and `vrg --help=false foo` exit 2 with sanitized usage diagnostics; the same strings after `--` are literal operands (`vrg -- --ignore-case=false` parses with that pattern).
  11. Generated help lists every allow-listed flag in short and long form.
- **Automated**: table tests for every accepted flag (short and long), every rejected sample, combined-flag expansion order, cumulative `-u` across mixed tokens including the combined-short boundary cases `-iu`, `-iuu` (accepted) and `-iuuu` (rejected), `--` behaviour including the literal `--` pattern — `vrg -- --` with the default root and `vrg -- -- .` with an explicit root both parse the second `--` as the verbatim pattern, the child argv containing the mandatory separator followed by a literal `--` pattern — dash-leading patterns, the empty pattern forwarded verbatim as an empty argv element, and the exact child argv including mandatory internal flags first. Ordering tests assert the exact child argv for `-i -s -i`, `-isi`, `--ignore-case -s -i`, and options interleaved with both operands (`vrg foo -i src -s`), preserving encounter order and the spellings as supplied — asserted through the resulting child argv (the module's public output), never through `VarOpt` callback invocation order. Cumulative tests include mixed aliases (`-u --unrestricted -u` → three → exit 2; `--unrestricted -uu` → three → exit 2; `-u --unrestricted` → two → accepted). Help regression rows keep every Issue 1 help-only case green with flags present: bare `vrg`, `-h`, `--help`, `-i --help`, `-ih pattern`, and help after a root (`vrg foo src --help`) produce the help-only result with exit 0, empty stderr, no child argv, and no root validation, while `vrg -i` remains an exit-2 missing-pattern error and help-like operands after `--` (for example `vrg -- --help`) stay positional rather than triggering help. Assignment-form rejection rows cover `--ignore-case=false`, `-i=false`, `--unrestricted=false`, and help assignment forms (`--help=false`, `-h=false`) as exit-2 usage errors, with the same strings preserved as positional operands after `--` when arity permits. A help-content test asserts every allow-listed flag appears in the generated help in short and long form, rendered from the same shared declarations used for parsing.

### Acceptance criteria

- [ ] Given any allow-listed flag in short or long form, when parsed, then it is forwarded in its original position relative to other user flags.
- [ ] Given repeated and mixed-alias options such as `-i -s -i`, `-isi`, or `--ignore-case -s -i`, then the child argv preserves the exact encounter order and the spellings as supplied, derived from the ordered token scan rather than library callback order.
- [ ] Given a combined short token such as `-iwF`, then it expands to `-i -w -F` in that order.
- [ ] Given two cumulative unrestricted flags in any token mix, then parsing succeeds; given three or more, then exit 2 with a usage diagnostic.
- [ ] Given a non-allow-listed option that is not a local help spelling, then exit 2 and no TUI; given a local help spelling anywhere before `--`, alone or mixed with search flags, then the Issue 1 help-only result applies and no flag is forwarded.
- [ ] Given an assignment form for a no-argument option (`--ignore-case=false`, `-i=false`, `--unrestricted=false`) or a help option (`--help=false`, `-h=false`), then it is a usage error with exit 2; given the same string after `--`, then it is a positional operand when arity permits.
- [ ] Given `--`, then subsequent tokens are positionals even if dash-leading — including a token spelled `--` itself.
- [ ] Given a successful parse, then the child argv is exactly `--json --no-config <user flags…> -- <pattern> <root>`.
- [ ] Given a combined short token containing unrestricted flags such as `-iu` or `-iuu`, then the unrestricted count accumulates per occurrence within the token and the cumulative boundary rules apply (two accepted, three rejected).
- [ ] Given an empty pattern argument, then the child argv forwards it verbatim as an empty element after `--`.
- [ ] Given the generated command-line help after this issue, then it lists every allow-listed search flag in short and long form from the same shared declarations used for parsing, extending Issue 1's help without changing its emission path or help-only precedence.

### User stories addressed

- User story 4: documented no-argument flags and combined shorts
- User story 5: options anywhere before `--`, forwarded in order
- User story 6: `--` protects dash-leading patterns
- User story 7: third cumulative `-u` rejected
- User story 8 (part): ripgrep configuration ignored via `--no-config`

---

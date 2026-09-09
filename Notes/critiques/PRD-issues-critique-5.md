# PRD issues critique 5 — VRG

## Scope and verdict

Reviewed:

- `Notes/skills/critique-issues/SKILL.md` — the actual skill location; the requested path with an additional `critiques/` directory does not exist.
- `Notes/Ideas.md`.
- `Notes/PRD-vrg.md`, revision 6, in full.
- All 35 files in `Notes/issues/`, in full.
- `Notes/critiques/PRD-issues-critique-4.md` for the disposition of earlier findings.
- The downloaded upstream source of `github.com/jawher/mow.cli v1.2.0`, supplemented by executable probes of its parsing behavior.

This is an **issues-only review**. No PRD, issue, task, or implementation files were changed. There is no `go.mod` or Go implementation in this checkout, so “verify the setup” means verify the proposed setup and upstream-library assumptions, not certify an installed application. Existing tasks were not reviewed or regenerated.

**Verdict: targeted corrections required before implementing Issue 1 and treating the revised CLI contract as propagated downstream.** The dependency selection and most of the scaffold requirements are sound, but the most consequential “verified” library claim in Issue 1 is false: `VarOpt.Set` callbacks do not preserve cross-option order. The suggested help integration also fails some of the issue's own acceptance criteria. Later issues still contain pre-revision-6 assumptions about the allow-list, help, and exit 0.

The rest of the issue set remains strong: no missing feature area or dependency cycle was found. Corrections should be local, not a new set of umbrella issues.

### Severity

- **High:** incorrect implementation guidance can break a central observable contract.
- **Medium:** an acceptance boundary, integration obligation, or PRD discrepancy allows materially inconsistent implementations or tests.
- **Low:** a local documentation, verification, or ownership defect.

## Upstream verification results

`go list -m -json github.com/jawher/mow.cli@latest` resolved to **v1.2.0**, matching Issue 1's pin. Its module checksum is:

```text
h1:e6ViPPy+82A/NFF/cfbq3Lr6q4JHKT9tyHwTCcUQgQw=
```

Probes used a fresh app per process, the proposed spec `[OPTIONS] PATTERN [OPTIONS] [ROOT] [OPTIONS]`, `flag.ContinueOnError`, a local `BoolOpt("h help", ...)`, string positionals, and custom boolean `VarOpt` recorders. The recorders implemented both `flag.Value` and `IsBoolFlag() bool`.

| Invocation / property | Observed upstream behavior | Assessment |
|---|---|---|
| Default error handling | `flag.ExitOnError` | Issue 1 is correct. |
| `ContinueOnError` | Returns rather than exiting for ordinary parse errors/help; still writes parser errors/help to stderr | Exit ownership and output ownership are separate. |
| No arguments with required `PATTERN` | `incorrect usage`, help on stderr | Explicit default-help adaptation is necessary. |
| First-token `--help` | Prints help to stderr, returns nil, skips `Action` | Issue 1 correctly identifies the first-token interception. |
| `foo --help /nonexistent` with declared help option | Successful parse, help value true, **Action runs** | Must handle help before search work in the action/adapter. |
| `-i --help` / `-ih` | `incorrect usage`; help value remains false; Action does not run | Declaring help does not override missing-pattern validation. |
| `foo bar baz --help` | Same parse failure; help remains false | Declaring help does not override excess operands. |
| `--unsupported --help` | Same parse failure | Late help cannot be discovered solely from parsed option values. |
| `foo -i src -s` | Accepted | Proposed spec supports interspersed options. |
| `-- --help`, `-- --`, empty-string pattern, literal `-` pattern | Accepted as positionals | These Issue 1 claims check out. |
| `-i -s -i foo` | Callback log grouped as `i,i,s` or `s,i,i`, not `i,s,i` | Ordered-forwarding claim is false. |
| `-isi foo` | Same cross-option grouping | Combined tokens do not fix ordering. |
| `-uuu foo` | Accepted with three unrestricted callbacks | Adapter must enforce the cumulative limit. |
| `--ignore-case=false foo`, `-i=false foo` | Accepted; custom value receives `false` | No-argument allow-list needs explicit lexical enforcement. |

The probe process itself did not implement VRG exit-status policy: it printed returned errors and state. Its process exit status is not evidence of VRG's required status 0 or 2.

## Findings

### F1 — High: Issue 1 falsely says `VarOpt` preserves original option order

**Affected:**

- `Notes/issues/001-go-scaffold-cli-positionals-and-root.md:32`
- `Notes/issues/002-cli-flag-allow-list-and-child-argv.md:14–18,32–37`

Issue 1 explicitly calls this verified behavior to rely on:

> `VarOpt` with a `flag.Value` records options in their original order across separate and combined tokens.

Upstream does not do this. In `internal/fsm/fsm.go:120–147`, parsing first collects values by option container, then `fillContainers` iterates a `map[*container.Container][]string` and calls `Set` for each container's values. Cross-option encounter order has already been lost; Go map iteration also makes callback order nondeterministic. Callback values do not identify which short/long alias was supplied.

For `-i -s -i foo`, a shared callback log receives either the two `i` callbacks together followed by `s`, or `s` followed by both `i` callbacks. Neither is the required `i,s,i`. This can change ripgrep's case-sensitivity precedence, not merely the formatting of a debug stub.

**Recommended correction:**

- Remove the false verified claim from Issue 1.
- Assign Issue 2 an ordered raw-token scan, or an equivalent order-preserving adapter, that records accepted spellings and expands short clusters left-to-right before library value assignment loses this information.
- Use shared option declarations for library configuration, token validation, and help metadata; do not maintain independently drifting allow-lists.
- Keep `mow.cli` meaningfully responsible for parsing/declared syntax rather than quietly replacing it with an unrelated parser.
- If custom boolean `VarOpt` values are retained, specify `IsBoolFlag() bool`; implementing `flag.Value` alone makes them argument-taking options.

**Ready when:** exact argv tests cover `-i -s -i`, `-isi`, `--ignore-case -s -i`, and options interleaved with both operands. They deterministically preserve encounter order and do not use callback order as evidence. Mixed aliases still contribute correctly to cumulative unrestricted counting.

### F2 — High: the proposed help integration does not satisfy help-anywhere precedence

**Affected:** `Notes/issues/001-go-scaffold-cli-positionals-and-root.md:28,31,43,53,62` and the Issue 2 extension.

The proposed local help boolean solves help after a pattern **only when the full search grammar otherwise parses**. `mow.cli` does not fill option values until the entire FSM parse succeeds. Thus `-i --help`, `-ih` without a pattern, and `foo bar baz --help` fail before the adapter can observe a set help value.

This contradicts Issue 1's acceptance of local help anywhere before `--`, independent of other operands, and the PRD's exception for nonempty missing-pattern invocations when help is requested. The supplied excess-operands manual example, `-h foo bar baz`, tests only the special first-token path and misses the failing symmetric case.

Issue 1 also overgeneralizes:

> after a help request, `Run` returns `nil` without invoking `Action`.

That is true for built-in first-token interception. For the locally declared help option in `foo --help /nonexistent`, **Action does run**. An Action that validates the root or starts search work before checking help violates the no-side-effects requirement.

**Recommended correction:** explicitly resolve local help before required-pattern/arity/root checks and before any search action, using an option-aware preflight or another proven adapter. Respect the first `--`; never find help inside positional tokens after it. Define how combined help is recognized consistently with Issue 2, and retain an explicit help-only result.

**Ready when:** tests cover first-token, later-token, and combined help with missing pattern, excess operands, and invalid root; include `-i --help`, `-ih`, `foo bar baz --help`, and literal help-like operands after `--`. Root validation, child startup, and TUI initialization have failing sentinels on every help-only case. Distinguish built-in interception from local-option Action execution in the guidance.

### F3 — Medium: neither proposed help-emission choice completely owns library output

**Affected:** `Notes/issues/001-go-scaffold-cli-positionals-and-root.md:26–30,55,61–62,70`.

Correctly, Issue 1 notes that the writer is unexported and that help must move to stdout. But its two designs focus on capturing `PrintLongHelp` or rendering help from shared metadata. Neither, as currently described, prevents **`Run` itself** from printing:

- First-token help invokes `PrintLongHelp` inside `Cmd.parse` before returning.
- An FSM parse failure writes `Error: ...` and invokes `PrintHelp` before returning the error.
- `ContinueOnError` changes exit behavior only; it does not suppress either write.

See upstream `commands.go:651–664` and `cli.go`'s package-level `stdErr` initialized from `os.Stderr`. Reassigning the application's `os.Stderr` variable later is not writer injection into the already-initialized library variable.

Rendering safe help after `Run` has printed does not remove the first stderr copy. Capturing only an explicit later `PrintLongHelp` call also misses it. Root validation can already be adapter-owned, but library parse/help output needs its own complete boundary.

**Recommended correction:** resolve the Issue 1 HITL choice into an output strategy covering every library call that can emit, including parse errors and first-token help. A metadata renderer must be paired with a way to prevent or contain the native emissions; it is not sufficient alone. If file-descriptor capture is selected, specify process-wide serialization, concurrent drainage to avoid pipe-capacity deadlocks, and reliable restoration on all controlled paths. Prefer an approach that does not introduce unsynchronized global stderr mutation into a reusable CLI module.

**Ready when:** executable-boundary tests assert exactly one help copy on stdout, empty stderr, and no search work; invalid syntax produces only the selected sanitized diagnostic/usage policy. Tests include failures generated by the library, not just errors caught by root validation.

### F4 — Medium: Issue 2 has not incorporated the new local-help and exact no-argument option contract

**Affected:** `Notes/issues/002-cli-flag-allow-list-and-child-argv.md:14,25–32,39`.

Issue 1 explicitly delegates search-flag declarations and their generated help to Issue 2, but Issue 2 still says:

> Every other option ... is a usage error.

It does not exempt local `-h`/`--help`, mention the chosen `mow.cli` adapter, require search flags to appear in generated command-line help, or require regression tests for help after flags and operands. Its non-allow-listed-option acceptance criterion is likewise unqualified. A literal implementation conflicts with its prerequisite.

The new parser also accepts value syntax not allowed by the PRD's **no-argument** flag list: both `--ignore-case=false foo` and `-i=false foo` succeed upstream. Forwarding these tokens, silently normalizing them to `-i`, or treating them as VRG-only boolean values all depart from the exact contract. This is especially dangerous for `--unrestricted=false` and help-like assignment syntax.

**Recommended correction:** make the distinction explicit: local help options are handled by Issue 1 and never forwarded; the listed search flags alone are forwarded. Require shared declarations to extend the existing command-line help. Reject assignment/value forms for no-argument options before library permissiveness can broaden the CLI.

**Ready when:** Issue 2 tests cover bare/default help regression, `-i --help`, `-ih pattern`, help after a root, `vrg -i` as an exit-2 missing-pattern error, every search flag in generated help, and help-like operands after `--`. Include rejection tests for `--ignore-case=false`, `-i=false`, `--unrestricted=false`, and non-supported help assignment forms, while preserving those same strings as operands after `--` when arity permits.

### F5 — Medium: CLI help is missing from downstream safety, documentation, and final smoke coverage

**Affected:**

- `Notes/issues/006-safe-presentation-utility-for-all-sinks.md:19–20,31`
- `Notes/issues/034-documentation-scale-and-memory-limits.md:17–18,26–27`
- `Notes/issues/035-final-integration-verification.md:16`

Three integration seams still reflect the old CLI:

1. **Issue 6** replaces the Issue 1 escaper but enumerates only browse sinks and usage-error stderr as existing sinks. Generated command-line help on stdout now exists from Issue 1. The reference to Issue 31 help substitutions is not a substitute: Issue 31 is the TUI dialog.
2. **Issue 34** calls its exit table complete but defines 0 only as successful search/browse. Revision 6 adds help-only exit 0. Documentation tests only enumerate Issue 2's search allow-list and compare search-derived outcomes; those checks cannot detect missing local help or the no-argument behavior.
3. **Issue 35** smoke-tests four search outcomes but no help-only path. The inherited full suite includes Issue 1 tests, so this is a narrower verification gap, not total absence of help tests. The closing binary checks should exercise the new no-child/no-TUI startup branch as well.

**Recommended correction:** add CLI-help stdout and its substitutions to Issue 6's existing-sink inventory and regression checks. Extend Issue 34 with bare `vrg`, `-h`/`--help`, stdout/exit-0/no-search behavior, flags-only usage errors, and the distinction from `h`/`?` in the TUI. Have documentation consume the same local/search option declarations used by parsing and generated help. Add a help-only final smoke scenario with rg unavailable and a sentinel fake rg available.

**Ready when:** sanitizer consolidation retains Issue 1's output tests, documentation accounts for both reasons for exit 0, and final binary verification proves help never initializes the TUI or starts rg.

### F6 — Medium: Issue 10's new counter exceptions contradict the authoritative PRD

**Affected:** `Notes/issues/010-record-robustness-malformed-oversized-unknown.md:14–16`; PRD *Result index, records, and stream integrity*.

Two fixes suggested by critique 4 were adopted locally but were not reconciled with the PRD:

- Issue 10 now says an oversized trailing unterminated record is **not additionally counted as malformed**. The PRD explicitly says a trailing unterminated record “is counted as malformed and makes the stream incomplete,” without that exception. Issue 10's preceding malformed-record bullet also still states the unqualified rule.
- Issue 10 now says an unknown type after `summary` is **not additionally counted as unknown**. The PRD requires unknown string types to be skipped, counted separately, and reported. “Does not independently alter the exit status” permits an independent integrity failure; it does not exempt the record from the unknown count.

The eventual status is already fatal in both situations, but collected counts and diagnostics remain observable requirements. The previous critique is not authority to change them silently.

**Recommended correction:** retain the PRD-required classifications and account for integrity separately. For an oversized unterminated record, retain the oversized count, the malformed count required for missing termination, and incomplete integrity; for an unknown post-summary record, retain its unknown count and flag the ordering violation. If exclusive counting is actually desired, request a product-level PRD clarification before encoding that exception in issues.

**Ready when:** the PRD, Issue 9 lifecycle matrix, and Issue 10 counting rules agree, with explicit fixtures for both combinations. Do not preserve a local exception merely because critique 4 recommended it.

### F7 — Medium: Issue 32's dismissal test expects `q` to skip restored help

**Affected:** `Notes/issues/032-overlay-precedence-esc-semantics.md:36`.

The dismissal table correctly includes:

> browse + error-over-help → help restored

It then incorrectly requires:

> a second `q` from each still-running state → the fixed status (0/2) or 1

For the error-over-help row, the second `q` closes the restored help, **not the application**. A third `q` from browsing exits. Implementing the universal assertion would violate the same issue's modal precedence and the PRD's help-restoration contract.

**Recommended correction:** make follow-up key expectations state-specific: error dismissal → restored help; next `q`/`Esc` → browsing; only a subsequent base-state `q` exits.

**Ready when:** the table explicitly checks the full three-key sequence for error-over-help and does not assert that every second `q` exits.

### F8 — Low: Issue 18 derives an invalid gutter-indicator invariant from its pan clamp

**Affected:** `Notes/issues/018-horizontal-panning.md:31,46`; Issues 19–20.

Issue 18 says the `_` gutter marker can never appear on every visible line at once because the widest line always retains a fully painted cluster. That conclusion does not follow: `_` means text is hidden **to the left**, not that no text is painted.

Example: all visible lines contain `abcdefghij`, the text width is 5, and the offset is 3. Every line has visible text `defgh` and hidden-left text `abc`, so every line may correctly show `_` when no match is entirely hidden left. The pan clamp is satisfied.

The acceptance criterion that the widest line “always” has a painted cluster also needs to preserve the documented exception when no cluster fits the text width, and Issue 19's geometric-reveal fallback. Those branches deliberately allow clipping blanks.

**Recommended correction:** remove the inference about `_` and qualify the painted-cell guarantee with the fitting-cluster condition. Do not alter the product owner's visible-lines extent policy.

**Ready when:** a fixture with all visible lines having hidden-left text allows `_` on every line, and narrow synthetic geometry does not conflict with the unpaintable-cluster exception.

### F9 — Low: two stale local instructions remain

**Affected:**

- `Notes/issues/006-safe-presentation-utility-for-all-sinks.md:17`
- `Notes/issues/007-theme-colour-toggle-and-match-styles.md:22`

Issue 5's tab ownership was fixed to point to Issue 16, and Issue 22 now explicitly disclaims it. Issue 6 still groups “tab is expanded structurally and LF / CRLF are line terminators” under an Issue 22 reference and calls that the core already landed in Issue 5. At this stage, Issue 5 only promises the provisional one-cell `→` tab rendering; eight-column stops arrive in Issue 16.

Issue 7's manual instruction says “run `vrg`, press `c`.” Under revision 6, that prints CLI help and exits; it cannot demonstrate theme switching.

**Recommended correction:** separate tab ownership from terminator ownership in Issue 6 and retain the provisional tab rule until Issue 16. Give Issue 7 an explicit search invocation and matching fixture.

**Ready when:** each issue's manual verification is executable against the revised CLI, and the sanitizer consolidation does not accidentally move tab behavior into the wrong issue.

## Coverage and dependency assessment

A mechanical check of the issue metadata established:

- All **35** issue identifiers are unique and all `Blocked by` references resolve.
- The dependency graph is acyclic.
- Every user story **1–85** has explicit individual ownership in Issues 1–34; this does not rely on Issue 35's blanket coverage statement.
- Issue 35 transitively depends on every feature issue.
- Issues **30, 33, and 34** are indeed the terminal feature issues used by Issue 35.

Story coverage is necessary but not sufficient: revised story 3 is nominally assigned to Issue 1, while F1–F5 show why the parser mechanism and downstream acceptance checks still need correction.

| Area | Assessment |
|---|---|
| Go scaffold and dependency choice | Correct library/version selected; actual module creation and remaining version/layout decisions belong to Issue 1. |
| CLI parsing, ordered argv, default help | Not ready as written: F1–F4. |
| CLI-help propagation | Incomplete in safety consolidation, user docs, and final smoke: F5. |
| Child lifecycle and diagnostics | Strong concurrent drainage, controlled-failure cleanup, PTY restoration/reap evidence, and post-restoration replay ownership. |
| Result index and outcomes | Comprehensive schema/lifecycle/outcome coverage; reconcile counter exceptions in F6. |
| Navigation, wrapping, async load/layout | Broadly coherent, including cached-file stale-layout preparation, latest-target intent, and reload-anchor arbitration. |
| Text, encoding, indicators | Detailed coverage; correct the local pan-indicator inference and tab ownership in F8–F9. |
| Modal behavior and too-small recovery | Strong coverage, but the Issue 32 follow-up assertion is wrong: F7. |
| Scope control | No accidental new selection route, streaming browsing, load queue/cancellation, cache eviction, transcoding, or diagnostics-review UI. |

## Disposition of critique 4

| Earlier finding | Current assessment |
|---|---|
| F1: wrong tab owner | Fixed in Issue 5 and explicitly clarified in Issue 22; the analogous Issue 6 wording remains (this review's F9). |
| F2: cached-file stale-layout preparation missing | Resolved in Issues 13 and 17 with explicit request/commit behavior and gated tests. |
| F3: unnecessary Issue 24 dependency on Issue 20 | Resolved; Issue 24 now depends on Issue 17. |
| F4: orphaned match after binary exclusion | Resolved by Issue 9's explicit binary-exclusion precedence. |
| F5: oversized unterminated counting | Made deterministic locally, but the chosen exception contradicts the PRD; reopen as F6 above. |
| F6: unknown type after summary | Same: locally explicit but not PRD-consistent; F6 above. |
| F7: temporary tab placeholder unspecified | Resolved in Issue 5 with the one-cell `→` and deferred cell-position assertions. |

## Recommended revision order

1. Correct Issue 1's upstream claims and choose a proven parsing/help/output adapter (F1–F3).
2. Make Issue 2 explicitly preserve local help, exact no-argument syntax, ordered forwarding, and generated flag help (F4).
3. Propagate the revision-6 help contract into Issues 6, 34, and 35 (F5).
4. Reconcile Issue 10 with the PRD and fix Issue 32's modal follow-up test (F6–F7).
5. Correct the remaining pan-indicator, tab-ownership, and manual-run wording (F8–F9).
6. Recheck the updated issues before propagating their changes to existing tasks through the separate task process.

No application tests were run because there is no application implementation in this checkout. Upstream probes were built and executed in an isolated temporary module outside the repository; dependency and story checks were performed against the documentation. This report certifies neither implementation readiness nor the unreviewed existing tasks.

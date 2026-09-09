## Issue 1: Go scaffold with mow.cli, default help, positionals, and root validation

**Type**: HITL
**Blocked by**: None — can start immediately

### Parent PRD

`Notes/PRD-vrg.md` (revision 6)

### What to build

Bootstrap the greenfield Go project (module, `cmd/vrg` main, package layout for the six modules named in *Module Design*, and pinned dependencies on `github.com/jawher/mow.cli`, Bubble Tea, Bubbles, and Lip Gloss). Build a runnable `vrg` executable whose CLI module uses `mow.cli` for command-line parsing and generated usage/help, delivering the first end-to-end invocation path: help, validated `vrg pattern [root]`, or a usage error.

- **Default and explicit help**: bare `vrg`, `vrg -h`, and `vrg --help` print generated command-line help to stdout and exit 0, with no error diagnostic. These paths do not validate a root, start ripgrep, or initialize the TUI, and work even when ripgrep is unavailable.
- **Help content and ownership**: generated help documents invocation syntax, the required pattern, optional root and its `.` default, and local `-h`/`--help` options. Issue 2 adds the search-flag declarations and their generated help when implementing forwarding. This is command-line help, not the TUI key-binding dialog delivered by Issue 31.
- **Positionals**: first is the pattern, second is the root, root defaults to `.`. Outside help-only paths, missing pattern or more than two positionals is a usage error. A **present but empty** pattern argument is present, not missing: `vrg "" .` parses successfully and retains an empty pattern (forwarding it verbatim in child argv is Issue 2).
- **Local help and option boundaries**: recognize `-h`/`--help` as local options anywhere before `--`; never treat them as search flags. The first `--` ends option parsing; all subsequent tokens are positional, including `-h`, `--help`, and another `--`. A literal pattern `-` is valid. Unsupported options produce usage errors. Search-flag acceptance, combined-short expansion, cumulative `-u`, and ordered child argv remain Issue 2; that extension must preserve these local help paths rather than reject help under its search-flag allow-list.
- **Root validation**: before any TUI starts, accept an existing directory or regular file, including symlinks resolving to either. Reject `-` (stdin), special files, and nonexistent paths. An actual regular file named `-` is addressed as `./-` and accepted.
- **Safe output and errors**: usage errors print a sanitized single-line diagnostic to stderr and exit 2 with no TUI or child. Any additional usage text on error also goes to stderr. Help and diagnostics must obey the PRD's output-safety contract, including substitutions derived from arguments or the executable name. Escape invalid bytes and dangerous controls in offending operands rather than letting library output bypass sanitization. A minimal shared escaper is acceptable here; Issue 6 generalises it.
- **Successful search stub**: for now, a valid search invocation may print a safely escaped representation of the parsed pattern and root and exit 0, enough to prove the slice end-to-end. Distinguish help-only results from successful search parses so the entry point cannot fall through into this stub after printing help. Later issues replace the stub with search and browsing.

#### Integrating `mow.cli`

Pin `github.com/jawher/mow.cli` at `v1.2.0` (the current release). Keep it behind the CLI module's interface: the module's outputs are a help-only result, a parsed search invocation, or a sanitized usage error, exactly as *Module Design → CLI* specifies; callers never see library types. Probing the library shows these defaults conflict with the PRD and must be handled explicitly:

- **Exit ownership**: `cli.App` defaults to `ErrorHandling = flag.ExitOnError`, so `Run` calls `os.Exit` itself (0 for help, 2 for errors) before returning. Set `flag.ContinueOnError` so the CLI module returns a result and the entry point in `cmd/vrg` decides the exit status.
- **No-argument path**: with a required `PATTERN` argument the library treats an empty argv as `incorrect usage`. Detect the empty argument list in the CLI module and return the help-only result before invoking the library parser.
- **Help placement**: the library's built-in `-h`/`--help` interception only triggers when help is the *first* token; `vrg foo --help` is otherwise an arity error. Declare `h help` as a local boolean option and use a spec that permits options between and after positionals (`[OPTIONS] PATTERN [OPTIONS] [ROOT] [OPTIONS]` verified to accept `vrg foo --help /nonexistent`, `vrg -ih foo`, and to stop at `--`). Treat a set help option, or the library's first-token interception, as the help-only result regardless of any other operands.
- **Output destination**: the library writes all help and parser diagnostics to an unexported writer fixed to `os.Stderr`; there is no writer injection point. The PRD requires help on stdout and sanitized output everywhere. The CLI module must therefore produce help text into a caller-supplied `io.Writer` itself rather than letting the library print. Two acceptable designs, chosen under the human decision below: (a) capture the library-rendered help (for example at the file-descriptor level while `PrintLongHelp` runs) and pass it through the escaper; or (b) render help from the same declared spec, argument, and option metadata that configures the library, so the two cannot drift. Either way, nothing external (argv values, `os.Args[0]`) may reach the output unsanitized; using the fixed name `vrg` for `cli.App` is the simplest way to avoid an executable-name substitution.
- **Diagnostic specificity**: the library's parser reports only the bare string `incorrect usage`, with no offending operand. The CLI module must classify failures itself (missing pattern, too many operands, unsupported option, invalid root) and compose the sanitized single-line diagnostic; do not surface the library string as the user-facing message.
- **Help-only versus success**: after a help request, `Run` returns `nil` without invoking `Action`. Represent the outcome with an explicit result kind set from the action/help path rather than inferring it from the returned error.
- **Verified library behaviour to rely on**: `--` ends option parsing natively (`vrg -- --help`, `vrg -- -h`, `vrg -- --`, `vrg - .`, and `vrg "" .` all parse as literal positionals); combined short tokens such as `-ih` expand; and `VarOpt` with a `flag.Value` records options in their original order across separate and combined tokens. Issue 2 will build on these for ordered forwarding and cumulative `-u` counting (the library does not limit repetitions), so the spec and option-declaration approach chosen here must not preclude adding `VarOpt` declarations for the allow-list.

See PRD *Implementation Decisions → Invocation and child arguments*, *Outcome and exit-status contract*, *Module Design → CLI*, and *Testing Decisions → CLI*.

**Human decision required**: confirm module path, package layout (e.g. `internal/cli`, `internal/searchindex`, …), Go version, pinned `mow.cli` / Bubble Tea / Bubbles / Lip Gloss versions, and which help-emission design (capture library output, or render from the shared declaration) satisfies the stdout-and-sanitization contract, before merging. The choice to use `github.com/jawher/mow.cli` and the help behavior are already requirements, not open product decisions.

### How to verify

- **Manual**:
  1. `go build ./...` succeeds; `go test ./...` passes; `go.mod` pins `github.com/jawher/mow.cli v1.2.0` and `go mod verify` is clean. Build a runnable binary with `go build -o ./vrg ./cmd/vrg` and use that binary for the checks below.
  2. Run `./vrg`, `./vrg -h`, and `./vrg --help`, capturing stdout, stderr, and exit status separately. Each prints usage/help to stdout, leaves stderr empty, and exits 0 without a search stub or TUI. Repeat with ripgrep unavailable on `PATH`.
  3. `./vrg foo --help /nonexistent` and `./vrg -h foo bar baz` show help and exit 0, demonstrating help before root validation and arity checks, and help accepted after the pattern.
  4. `./vrg foo` prints the stub success line with root `.` and exits 0.
  5. `./vrg --` (missing pattern) and `./vrg a b c` (excess operands) produce specific stderr usage diagnostics (not the library's bare `incorrect usage`) and exit 2. Bare `./vrg` remains the help-only exception.
  6. `./vrg foo /nonexistent` exits 2; `./vrg foo -` exits 2 with a stdin-rejection message; `./vrg foo /dev/null` exits 2 on systems providing that special file.
  7. `./vrg foo ./go.mod` succeeds; roots that are symlinks to an existing directory or regular file also succeed.
  8. In a temporary directory containing a regular file named `-`, invoke the built binary with `foo ./-`; validation succeeds and exits 0.
  9. `./vrg "" .` succeeds with an empty pattern. `./vrg -- --help`, `./vrg -- -h`, `./vrg -- --`, and `./vrg - .` succeed with those literal patterns rather than showing help.
  10. An unsupported option such as `./vrg --unsupported` produces a sanitized stderr diagnostic naming the option and exits 2.
  11. Invoke the binary with a root operand containing an escape byte (e.g. `./vrg foo "$(printf '/no\033[31msuch')"`); the stderr diagnostic shows an escaped form and no raw control byte reaches the terminal.
- **Automated**:
  - CLI tests cover help-only results for no arguments and explicit local help, generated help content (syntax line, `PATTERN`, `ROOT` with `.` default, `-h`/`--help`), help after the pattern, and help bypassing root validation through an injected validator that fails if called.
  - Table tests cover the default root, directory/regular-file/symlink roots, stdin and special-file rejection, nonexistent roots, arity errors outside help-only paths, the `./-` regular-file root, a present empty pattern, the literal `-` pattern, `--` positional handling (including help-like and literal `--` operands), and unsupported options. Each error case asserts the classified diagnostic kind, not the library's generic message.
  - Output tests cover hostile operand and executable-name substitutions in help, parser errors, root diagnostics, and the success stub. Assert external data cannot emit raw terminal controls or unescaped invalid bytes; legitimate help line breaks remain readable. Assert the CLI module writes help to the supplied writer and nothing to the process stderr on the help path.
  - Subprocess tests execute the built entry point and assert stdout, stderr, and exit status for help, successful parsing, and usage errors. Help works with no rg on `PATH`, emits no TUI/alternate-screen sequences or stub output, and does not invoke a sentinel fake rg when one is available. Test the entry-point dispatch boundary to verify that help never invokes TUI initialization and that exit codes are chosen by `cmd/vrg`, not by the library.

### Acceptance criteria

- [ ] Given a clean checkout and the approved Go toolchain, when the project is built, then `cmd/vrg` produces a runnable executable using pinned `github.com/jawher/mow.cli v1.2.0` for CLI parsing and help, not merely listing it as an unused dependency.
- [ ] Given the library integration, then `ErrorHandling` is `flag.ContinueOnError` and every exit status is chosen by the entry point; the library never calls `os.Exit`.
- [ ] Given no command-line arguments or local `-h`/`--help` anywhere before `--`, when the executable runs, then help goes to stdout and it exits 0 with empty stderr, without root validation, ripgrep startup, TUI initialization, or success-stub output.
- [ ] Given ripgrep is unavailable or a supplied root is invalid, when command-line help is requested, then help still succeeds without validating search prerequisites.
- [ ] Given generated help, then it describes syntax, the required pattern, the optional root and its `.` default, and local help options; Issue 2 extends it with the supported search flags without changing the emission path.
- [ ] Given `vrg pattern`, when the CLI parses, then root defaults to `.`.
- [ ] Given `vrg pattern root` where `root` is an existing directory or regular file, directly or via symlink, then validation succeeds.
- [ ] Given a non-help invocation missing a pattern, more than two positionals, or an unsupported option, then a sanitized single-line usage diagnostic identifying the failure kind is written to stderr and the process exits 2 without starting a child or TUI.
- [ ] Given `--`, then subsequent tokens are positionals, including `-h`, `--help`, and `--`, rather than local help requests or additional option terminators.
- [ ] Given a root of `-`, a special file, or a nonexistent path, then the process exits 2 with a sanitized diagnostic naming the reason.
- [ ] Given external data containing dangerous controls or invalid bytes, then CLI help, diagnostics, and the stub escape that data rather than emitting it raw, including any text the library renders.
- [ ] Given a root of `./-` naming an actual regular file called `-`, then validation succeeds.
- [ ] Given a present but empty pattern argument or a literal `-` pattern, then it is retained as a present positional and parsing succeeds.
- [ ] Given the CLI module's public result type, then help-only, parsed-search, and usage-error outcomes are distinct values; callers do not inspect library errors or strings to tell them apart.

### Definition of done

- [ ] The human decision above is resolved: the reviewer's approval of module path, package layout, Go version, pinned `mow.cli` / Bubble Tea / Bubbles / Lip Gloss versions, and the help-emission design is recorded before merge.
- [ ] Build, CLI tests, and executable-boundary tests pass; the default-help path is demoable independently of Issue 2 and without ripgrep installed.
- [ ] Issue 2 can add allow-listed search flags as additional option declarations on the same `mow.cli` app without changing the spec shape, help emission, or exit handling established here.

### User stories addressed

- User story 1 (invocation slice): accept a directory or regular-file search root
- User story 2: root defaults to `.`
- User story 3: default/explicit help exits 0; invocation mistakes produce stderr diagnostics and exit 2 (search-flag-specific validation follows in Issue 2)
- User story 6 (parsing boundary only): `--` protects dash-leading positionals; child-argv protection follows in Issue 2
- User story 8 (part): stdin roots rejected

---

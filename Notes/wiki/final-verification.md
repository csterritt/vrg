# Final integration verification (Issue #35)

The closing gate delivered by
[Issue #35](../issues/035-final-integration-verification.md): a
clean-checkout verification pass over the fully composed Issues #1–#34
implementation, documented by
[Notes/tasks/035-final-integration-verification.md](../tasks/035-final-integration-verification.md)
against the acceptance criteria AC1–AC5 and the *Testing Decisions*
section of `Notes/PRD-vrg.md`. The evidence run is recorded in the
[035-03 walkthrough](../walkthroughs/035-03/code-walkthrough/walkthrough.md).

## The clean-checkout gate set

The pass ran from a clean worktree checkout of the completed repository
(so no stale artifact could mask a failure):

- `go build ./...` — clean, all packages.
- `go vet ./...` — clean, no diagnostics.
- `go test ./...` — all eight packages pass (`cmd/vrg`,
  `internal/app`, `internal/cli`, `internal/filebuffer`,
  `internal/present`, `internal/searchindex`, `internal/theme`,
  `internal/viewport`).
- Explicit `-count=1` re-runs of the PTY/subprocess packages so every
  critical boundary test executes rather than being satisfied by a
  cached result: `go test -count=1 ./cmd/vrg` (the Issue #4 PTY
  harness — cancellation 130s, ordinary-exit reap, controlled-failure
  exit 2 — plus the Issue #9 fatal-overlay boundary tests and the
  Issue #11 `pty_replay_test.go` suite) and the Issue #3/#4
  subprocess-boundary tests in `./internal/app` (argv/working
  directory, dual-pipe backpressure, cancel-terminate-reap, process
  group, start failure).

## The five smoke outcomes

The final `vrg` binary was smoke-run through
`scripts/smoke.py` — the canonical condition-driven fake-rg PTY
harness, which synchronizes every keypress on an observed rendered
marker and carries no sleeps:

1. **Successful browse** — fake rg emits a complete one-match stream,
   `q` quits, exit 0, stderr empty, PTY reached EOF.
2. **No results** — summary-only stream with a `warn` stderr line:
   warning overlay observed, `Esc` dismisses, the no-results repaint is
   observed, `q` exits 1, and `warn` is replayed to stderr.
3. **Fatal outcome, no usable results** — one malformed record plus
   exit 2: the overlay paints the composed diagnostic (`rg failed:
   exit status 2`, `missing summary`, `1 malformed record skipped`);
   dismissal exits 2 with **both** `q` and `Esc`, and the composed
   diagnostic is replayed to stderr.
4. **Cancellation while searching** — fake rg blocks forever: `q` and
   separately `ctrl+c` both exit 130, the child **and its process
   group** are externally observed gone, the PTY reaches EOF, the
   cursor/alt-screen restoration sequences appear, and the slave
   termios equals its pre-launch value.
5. **Help-only** — bare `vrg`, `-h`, and `--help` each print exactly
   one `Usage:` copy to stdout with empty stderr and exit 0, run with
   a sentinel fake `rg` on `PATH` (and real ripgrep absent) that is
   never invoked — proving the no-child/no-TUI startup branch (see
   [cli-foundation.md](cli-foundation.md)).

## Regressions found and repaired

The first clean pass surfaced one real production regression and one
stale-harness defect:

- **Cancellation hang through a forked payload** (owning contract:
  [Issue #4](cancellation-and-cleanup.md)). The seeded smoke fixture's
  `sh`-scripted rg runs its blocking `sleep` as a **grandchild** rather
  than `exec`'ing it, and `exec.CommandContext`'s default cancel kills
  only the direct PID — the surviving grandchild held the drained pipes
  open, so the collector's drainage wait never ended and the process
  hung after terminal restoration instead of exiting 130. The Go PTY
  suite missed it because `fakeRgBlockScript` `exec`s its sleeper.
  Fix in `internal/app/search.go`: the child now leads its own process
  group (`SysProcAttr.Setpgid`) and `cmd.Cancel` SIGKILLs the group, so
  no descendant can hold the pipes past cancellation. The regression
  test `TestCancelTerminatesChildProcessGroup` in
  `internal/app/subprocess_test.go` adds a `forkblock` fake-rg mode
  (grandchild inherits both pipes) and asserts prompt reap, both pids
  gone, and the group empty. See
  [unit-tests.md](unit-tests.md).
- **Stale smoke-harness markers** (harness repair, not a production
  regression). `scripts/smoke.py` was seeded against pre-renumbering
  issue references and expected the literals `code 2`, `missing summary
  record`, and `ripgrep exited with code 2`; the composed contract the
  focused tests pin renders `rg failed: exit status 2` and `missing
  summary`. The harness markers were aligned to the pinned wording, and
  the no-results gate was retargeted to the observable post-dismissal
  repaint fragment — the centred `No results found` text is painted
  split by the overlay's border cells, so it never appears contiguously
  in the raw stream (all three fragments are asserted instead). No
  behavioral assertion was weakened: exit statuses, child/process-group
  termination, termios restoration, stderr replay, and the sentinel
  fake-rg non-invocation checks are unchanged.

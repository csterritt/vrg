# Tasks for #45: Move test hooks and spin-watchers out of the production binary

Parent issue: #45
Parent PRD: PRD-vrg.md
**Blocked by issues**: none — must land before Issues #46 and #48, which consume this seam
**Acceptance criteria**: AC1, AC4 → Tasks 1–2; AC2 → Tasks 1–2; AC3, AC5 → Task 2
**Manual verification**: Task 4 owns the issue's manual checks.

## Tasks

### 1. Specify the clean production artifact

**Type**: RED  
**Output**: Failing boundary tests prove both halves of the topology: an untagged binary ignores every explicit vrg-consumed hook and contains none of their strings, while a `vrg_testhooks` build can inject every valid/invalid final-model and nil/non-nil-error tuple required by Issue #46 at the executable's actual program-runner call site.
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md` and the build-constraint conventions in `Notes/skills/code-writing/production-code-and-build-constraints.md`.

Add a failing boundary test in `cmd/vrg` that builds an **untagged** production binary (`go build -o <tempdir>/vrg .` into a temp dir), sets each name in the explicit vrg-consumed hook manifest — `VRG_TEST_REAP`, `VRG_TEST_GATE`, `VRG_TEST_FAIL_TRIGGER`, `VRG_TEST_FAIL_DIAGNOSTIC`, `VRG_TEST_DIAGNOSTIC_TRIGGER`, `VRG_TEST_DIAGNOSTIC_TEXT`, `VRG_TEST_COLLECT_ACK`, plus the runner controls `VRG_TEST_RUN_FINAL_MODEL` and `VRG_TEST_RUN_ERROR` (extend the list when Issue #48 adds acknowledgement hooks to the manifest) — and asserts no behavioural change; then inspect the artifact (e.g. `strings`/`bytes.Contains` on the binary) for those names. Derive the probed list only from this explicit manifest — never by grepping every `VRG_TEST_*` occurrence, because fake-rg fixture variables are not vrg behaviour and are renamed `FAKE_RG_*` by Issue #50. The test proves the released artifact is clean, not merely that the tag defaults off; it fails on the current code, where the env-var seams are compiled into the production binary. In the same RED task, add focused tagged-runner boundary tests that build with `vrg_testhooks`, select every tuple Issue #46 needs through `VRG_TEST_RUN_FINAL_MODEL` and `VRG_TEST_RUN_ERROR` — valid model, nil/invalid model, nil error, and non-nil error in the required combinations — and prove the tuple reaches the executable's actual runner return site unchanged. These tests specify only seam reachability and tuple fidelity; Issue #46 remains responsible for shutdown order, replay, cleanup, and exit-status behavior. Keep this task test-only.

---

### 2. Install the build-tagged test-hook topology

**Type**: GREEN  
**Output**: The boundary test passes; `TestMain` builds the binary under test with `-tags vrg_testhooks`, all prior seam-controlled PTY/subprocess behaviours work unchanged against the tagged binary, the dedicated runner seam can produce every Issue #46 return shape at the real `program.Run()` boundary, and `go build`/`go vet`/`go test`/`go test -race` pass in the default configuration.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md` and the build-constraint conventions in `Notes/skills/code-writing/production-code-and-build-constraints.md`.

Split `cmd/vrg/main.go`'s test seams behind two narrow build-constrained boundaries. Add `seams.go` (`//go:build !vrg_testhooks`) returning nil, and `seams_testhooks.go` (`//go:build vrg_testhooks`) reading the option-related `VRG_TEST_*` environment variables and returning the corresponding `app.Option`s and `proc.OnReap` wiring; keep one unconditional common call site such as `opts = append(opts, testSeamOptions(proc)...)`. Add a separate program-runner wrapper, distinct from the app options, around Bubble Tea program construction and `Run()`: the untagged implementation delegates directly to `tea.NewProgram(...).Run()`, and the tagged implementation delegates by default but injects the valid, invalid/nil, error, and nil-error return shapes selected by `VRG_TEST_RUN_FINAL_MODEL`/`VRG_TEST_RUN_ERROR` at the actual executable boundary. Move every hook env-var read, file watcher, trigger loop, and conditional test behaviour into tagged files only — the untagged production build contains none of the manifest strings and no env-var reads beyond legitimate production ones — while inert complementary implementations and their unconditional common call sites are permitted and must perform only direct production delegation or return no options. Change `TestMain` (`cmd/vrg/main_test.go`) to build the binary under test with `go build -tags vrg_testhooks -o binPath .` so `go test ./cmd/vrg`, `go test -race ./cmd/vrg`, and `go test ./...` all exercise the hooked binary automatically. Update tests that relied on the env vars only to the extent needed — their coverage is preserved because the tagged binary exposes the identical seams — and make any polling/watching that genuinely remains in production code cancellable and non-spinning (sleep/backoff or event-driven). Run the full suite in both build variants, including a race/timeout run confirming no remaining watcher spins without cancellation.

---

### 3. Document the test-hook build topology

**Type**: DOCUMENT  
**Output**: Wiki documentation records the `vrg_testhooks` variant, the two seam boundaries, the explicit hook manifest, and the clean-artifact boundary test.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #45 implementation into the appropriate pages under `Notes/wiki`. Document the two build-constrained boundaries (option/process wiring and the program-runner wrapper), the explicit vrg-consumed hook manifest and why fixture-owned variables are excluded, the `TestMain` tagged build, the untagged-artifact boundary test, and the rule that Issues #46 and #48 extend the same mechanism rather than adding production hooks. Cross-reference Issue #45 and the *Outcome and exit-status contract* and *Testing Decisions* sections of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the test-hook-topology walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/045-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/045-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the boundary test and both build variants, then run the issue's manual scenario: build the production binary (`go build ./cmd/vrg`), set each name in the explicit vrg-consumed hook manifest, and show none alter behaviour and `strings` on the artifact finds none of the hook names; then build the tagged binary and exercise `VRG_TEST_RUN_FINAL_MODEL`/`VRG_TEST_RUN_ERROR` to confirm the override reaches an actual `program.Run()` result branch. Capture commands, outputs, and exit statuses. Reference Issue #45 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---

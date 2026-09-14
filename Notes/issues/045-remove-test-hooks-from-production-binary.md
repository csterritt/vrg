## Issue 45: Move test hooks and spin-watchers out of the production binary

**Type**: AFK
**Blocked by**: None — can start immediately; must land before Issues 46 and 48, which consume this seam

### Parent PRD

`Notes/PRD-vrg.md` — audit source: `Notes/critiques/final-audit-vrg.md`, Medium finding 10

### What to build

Remove the test-only seams compiled into the released binary (`cmd/vrg/main.go:76-167`). Environment variables currently make the production binary able to truncate or append arbitrary user-writable files, inject fake diagnostics/failures, hold result preparation indefinitely, and run tight file-watcher polling loops with no sleep or cancellation. These are test-harness capabilities shipping as normal production behaviour — a security and robustness problem, not just untidiness.

**Build topology (selected — implement this one).** The current PTY suite does not execute the `go test` package binary: `TestMain` (`cmd/vrg/main_test.go:19-32`) runs `go build -o <bin> .` and every subprocess test executes that built `vrg`. Code in `_test.go` files therefore cannot reach the binary under test. Use a build-tagged variant instead:

- Production `cmd/vrg` keeps two narrow, build-constrained boundaries:
  - An option/process-wiring call such as `opts = append(opts, testSeamOptions(proc)...)`. `seams.go` (`//go:build !vrg_testhooks`) returns nil; `seams_testhooks.go` (`//go:build vrg_testhooks`) reads the option-related `VRG_TEST_*` environment variables and returns the corresponding `app.Option`s / `proc.OnReap` wiring.
  - A program-runner wrapper, separate from app options, around Bubble Tea program construction and `Run()`. The untagged implementation delegates directly to `tea.NewProgram(...).Run()`. The tagged implementation delegates by default but can inject the valid, invalid/nil, error, and nil-error return shapes required by Issue 46 at the actual executable boundary.
- The explicit vrg-consumed hook manifest is `VRG_TEST_REAP`, `VRG_TEST_GATE`, `VRG_TEST_FAIL_TRIGGER`, `VRG_TEST_FAIL_DIAGNOSTIC`, `VRG_TEST_DIAGNOSTIC_TRIGGER`, `VRG_TEST_DIAGNOSTIC_TEXT`, `VRG_TEST_COLLECT_ACK`, plus the runner controls `VRG_TEST_RUN_FINAL_MODEL` and `VRG_TEST_RUN_ERROR` (whose values select the Issue 46 return shapes). Issue 48 may add named application-acknowledgement hooks to this manifest. Only this manifest — not fixture-owned fake-rg variables — defines production behavior to probe. Every listed hook, file watcher, trigger loop, and conditional test behaviour exists only in tagged files. The untagged production build contains none of these strings and no env-var reads beyond legitimate production ones. Inert complementary implementations and their unconditional common call sites are explicitly permitted; they must perform only the direct production delegation or return no options.
- `TestMain` builds the binary under test with `go build -tags vrg_testhooks -o binPath .`, so `go test ./cmd/vrg`, `go test -race ./cmd/vrg`, and `go test ./...` all exercise the hooked binary automatically. Issue 46's runner controls and Issue 48's handshakes are added through these tagged boundaries.
- A separate production-boundary test builds an **untagged** binary (`go build -o prodBin .` into a temp dir) and probes it: set every name in the explicit vrg-consumed hook manifest (including Issue 46's runner controls and any Issue 48 acknowledgement hooks) and assert no behavioural change; inspect the artifact (e.g. `strings`/`bytes.Contains` on the binary) for those names. Do not derive this list by grepping every `VRG_TEST_*` occurrence, because fake-rg fixture variables are not vrg behavior and are renamed to `FAKE_RG_*` by Issue 50. This test proves the released artifact is clean, not merely that the tag defaults off.
- If any polling/watching genuinely remains in production code, make it cancellable and non-spinning (sleep/backoff or event-driven).
- Tests that relied on these env vars are updated only to the extent needed; their coverage is preserved because the tagged binary exposes the identical seams.

Consult `Notes/skills/code-writing/production-code-and-build-constraints` for the project's build-constraint conventions (use `//go:build`, provide the complementary implementation, test every variant).

### How to verify

- **Manual**: build the production binary (`go build ./cmd/vrg`), set each name in the explicit vrg-consumed hook manifest, and run the binary → none alter behaviour; `strings` on the artifact shows none of those hook names. Build the tagged binary and exercise `VRG_TEST_RUN_FINAL_MODEL`/`VRG_TEST_RUN_ERROR` to confirm the override reaches an actual `program.Run()` result branch.
- **Automated**: the existing PTY/subprocess tests pass against the `vrg_testhooks`-tagged binary built by `TestMain`; runner-shape subprocess tests prove the tagged runner can reach the executable's real `Run()` result branches; the untagged production-binary test probes every name in the explicit vrg-consumed hook manifest and asserts no effect and no corresponding hook strings in the artifact; a race/timeout run confirms no remaining watcher spins without cancellation.

### Acceptance criteria

- [ ] Given the untagged production build, then no name in the explicit vrg-consumed hook manifest can trigger file truncation/appends, injected diagnostics or failures, held preparation, altered `program.Run()` results, acknowledgements, or test-only watcher loops — and none of those hook names, test-control environment reads, watchers, side effects, or conditional test behaviour are present in the artifact. Fake-rg fixture controls are outside this production-behavior manifest regardless of their temporary prefix before Issue 50 renames them. Inert complementary seam implementations and their common call sites are permitted.
- [ ] Given the `vrg_testhooks`-tagged build produced by `TestMain`, then all prior seam-controlled behaviours remain available to subprocess tests, and the dedicated runner seam can produce every Issue 46 return shape at the actual `program.Run()` boundary.
- [ ] Given any remaining production polling, then it is cancellable and bounded (sleep/backoff or event-driven) — no tight spin.
- [ ] Given `go build ./...`, `go vet ./...`, `go test ./...`, and `go test -race ./...` in the default configuration, then they all pass and the production artifact contains no test-control names or behaviour; the inert production delegator/no-op implementations remain allowed.
- [ ] Given the tagged harness, then every test previously using the env-var seams still passes, and Issues 46 and 48 can add runner controls and handshake seams through the same build-constrained mechanism without adding test behaviour to the production variant.

### User stories addressed

- None directly — production-hygiene hardening from the audit; it protects the story 22–23 exit/replay and cleanup contracts by keeping test-only failure injection and unkillable spin loops out of the released binary.

---

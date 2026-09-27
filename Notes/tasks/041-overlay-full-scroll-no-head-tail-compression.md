# Tasks for #41: Error overlays keep every row scrollable — no destructive head/tail compression

Parent issue: #41
Parent PRD: PRD-vrg.md
**Blocked by issues**: none — shared-file/harness ordering with #46 and #48 in `cmd/vrg`: never implement overlapping PTY or `outcome_test.go` work concurrently; whichever issue lands second adapts to the already-landed tests and helpers. If #48 has landed, use its acknowledgement harness with no fixed settling/inter-key delay and add any new key-sending helper to #48's matrix and every new acknowledgement hook it requires to #45's hook manifest; if #41 lands first, #46/#48 apply their reciprocal adaptation clauses
**Acceptance criteria**: AC1–AC6 → Tasks 1–2
**Manual verification**: Task 4 owns the issue's manual checks.

## Tasks

### 1. Specify the complete-scrollable-row contract

**Type**: RED  
**Output**: Failing model-level overlay tests assert the scrollable row set equals the complete wrapped diagnostic, `overlayScroll` clamps to `[0, max(0, rows−height)]`, every row is reachable, appended errors extend the set, and the revised `TestStderrContentFixture` drops simultaneous head/tail rendering while keeping its drainage/completeness checks.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Add failing tests in `internal/app` asserting the model's scrollable overlay rows equal the complete wrapped diagnostic including both first and last markers — no ellipsis row is ever injected into the scrollable set — and that `overlayScroll` clamps exactly to `[0, max(0, rows−maxVisible)]` in both the key handler and the render path, proving tail reachability for arbitrarily large diagnostics (including the ≥ 1 MiB content shape) without sending thousands of PTY keys. Add a moderately oversized fixture whose wrapped row count only slightly exceeds `maxVisible` and assert a bounded row-by-row `down` traversal reaches the final line and an `up` traversal returns to the first; this may be a model test or a PTY test using Issue #48's handshake harness if that has landed. Assert an error appended while an overlay is open extends the scrollable set without moving the reader's position, and keep the existing ignored-key regression that `u`, `d`, page up, and page down have no effect while an error overlay is open — the modal key contract is unchanged. Revise `TestStderrContentFixture` in `cmd/vrg/outcome_test.go` to retain its ≥ 1 MiB stdout/stderr pipe-drainage, complete-stdout, captured-stderr inclusion, and completion assertions while no longer requiring simultaneous head/tail visibility; do not weaken tail verification — move that proof to the complete-row model and bounded traversal assertions. The complete-rows and traversal assertions fail on the current head/tail-compression code. Keep this task test-only.

---

### 2. Keep every wrapped overlay row scrollable

**Type**: GREEN  
**Output**: The overlay tests pass; non-help overlays scroll through the complete wrapped row set with clamping at both ends, render-time clipping at tiny sizes still applies, and help-overlay scrolling is unchanged.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Remove the head-plus-ellipsis-plus-tail compression in `renderOverlay` (`internal/app/app.go`) so the scrollable row set for non-help overlays is the complete wrapped diagnostic — nothing is elided from the model. Clamp `overlayScroll` over the complete set — both where `handleOverlayKey` increments it and where `renderOverlay` derives the visible slice — so every row of a long diagnostic is reachable by `up`/`down`. Keep render-time clipping at tiny terminal sizes (story 83: clipping at render time is fine, removal from the scrollable set is not), keep appended diagnostics extending the scrollable set without resetting scroll, and leave the help overlay's scrolling unchanged. Head/tail summarisation is not reintroduced — if ever wanted it must be an explicit alternate view, which this issue does not require. Run the focused tests plus `go build ./...`, `go vet ./...`, and `go test ./...`.

---

### 3. Document full-scroll overlays

**Type**: DOCUMENT  
**Output**: Wiki documentation records the complete-row scrolling contract and the revised large-stderr fixture semantics.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #41 implementation into the appropriate pages under `Notes/wiki`. Document that non-help overlays keep every wrapped row in the scrollable set with `overlayScroll` clamped to `[0, max(0, rows−height)]`, that render-time clipping at tiny sizes is preserved while model-level elision is forbidden, that this contract supersedes Issue #9's original simultaneous head/tail rendering requirement for the ≥ 1 MiB stderr fixture (drainage and completeness retained), and the unchanged modal key contract. Cross-reference Issue #41 and the *Colours, overlays, and key precedence* section of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the full-scroll-overlay walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/041-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/041-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the model-level complete-rows, clamp, and traversal tests plus the revised `TestStderrContentFixture`, then run the issue's manual scenario: a fatal outcome from a fake rg emitting stderr longer than the overlay's visible height → open the overlay and scroll from the first row to the last with repeated `up`/`down` (confirming `u`/`d`/page keys are ignored), verify every middle row is reachable with no ellipsis substituting for content, dismiss, and confirm scroll clamping at both ends. Capture commands, outputs, and exit statuses. Reference Issue #41 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---

## Issue 49: Tidy dependency manifests — decide the fate of Bubbles and Lip Gloss

**Type**: HITL — the keep-or-remove decision on Bubbles/Lip Gloss is a human call (they are in the PRD's stated stack but currently unimported)
**Blocked by**: None — the decision and tidy can start immediately; see the adoption branch below for the ordering rules that keep a keep-decision from colliding with Issues 38–41

### Parent PRD

`Notes/PRD-vrg.md` — audit source: `Notes/critiques/final-audit-vrg.md`, Low finding 14

### What to build

Resolve the dependency-manifest drift reported by `go mod tidy -diff` (`go.mod:5-27`, `go.sum:1-54`): direct imports marked indirect, required test dependencies absent from the declared graph, and unused requirements on Bubbles and Lip Gloss (`charm.land/bubbles/v2`, `charm.land/lipgloss/v2`) that tidy would remove because the implementation does not import them.

**Decision (HITL)** — exactly one of two bounded outcomes:

- **Removal**: accept that tidy drops both modules, and update every stated-stack reference to match — at minimum the PRD *Further Notes* stated-stack line (`Notes/PRD-vrg.md` *Further Notes*, "using Bubble Tea, Bubbles, and Lip Gloss") and `Notes/wiki/project-overview.md` (the pinned-stack bullet). This issue then closes once the manifests and documentation agree and tidy is clean.
- **Adoption**: record — in this issue or a `Notes/decisions/` entry — the concrete component/API each library will own (e.g. which viewport, list, theme, overlay, or width code it replaces or implements). Then create separately scoped implementation issue(s) for that adoption, each declaring dependencies on the audit-fix issues it touches (Issues 38–41 and any others), so adoption work cannot invalidate in-flight render fixes. Token imports whose sole purpose is retaining a manifest entry are not adoption and are rejected. Under this outcome Issue 49 stays open until the adoption issues land and `go mod tidy -diff` is clean on the final state.

Whichever branch is chosen, commit a clean `go mod tidy` result with correct direct/indirect marking and test dependencies declared. Verify that state directly in this issue with `go mod tidy -diff`. Issue 50 exclusively owns creation of `scripts/verify.sh` and will incorporate this direct check into the persistent closing gate after Issue 49 has closed; Issue 49 neither creates nor partially owns that script.

### How to verify

- **Manual**: run `go mod tidy -diff` on the committed result → empty diff; `go build ./...` and `go test ./...` pass against the tidied graph.
- **Automated**: invoke `go mod tidy -diff` directly and require a clean result; `go mod verify` passes; the build/test suite confirms no missing or unused requirements. Issue 50 later adds this already-green command to the permanent verification routine.

### Acceptance criteria

- [ ] Given the removal decision, then the go.mod/go.sum entries are dropped and the PRD *Further Notes* stated stack and `Notes/wiki/project-overview.md` are updated to match — a finite, reviewable documentation diff.
- [ ] Given the adoption decision, then the concrete component/API each library will own is recorded, separately scoped implementation issues exist with dependencies on the relevant audit fixes, and this issue closes only after they land with tidy clean.
- [ ] Given either decision, then no import exists solely to retain a manifest entry.
- [ ] Given `go mod tidy -diff`, then it reports no drift on the committed manifests.
- [ ] Given the tidied graph, then `go build ./...`, `go vet ./...`, and `go test ./...` all pass.
- [ ] Given this issue closes before Issue 50 starts, then direct `go mod tidy -diff` verification is green and Issue 50 remains the sole owner of creating the standard verification suite that will make this check permanent.

### User stories addressed

- None directly — dependency-manifest hygiene; aligns the declared module graph with the PRD's stated stack and keeps the verification story (story 85's documented-limits spirit; Issue 35's verification gate) reproducible.

---

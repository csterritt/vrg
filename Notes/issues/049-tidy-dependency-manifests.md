## Issue 49: Tidy dependency manifests — remove unused Bubbles and Lip Gloss requirements

**Type**: AFK — removal selected by the product owner on 2026-09-14; Bubbles and Lip Gloss are unused
**Blocked by**: None — the selected removal and tidy can start immediately

### Parent PRD

`Notes/PRD-vrg.md` — audit source: `Notes/critiques/final-audit-vrg.md`, Low finding 14

### What to build

Resolve the dependency-manifest drift reported by `go mod tidy -diff` (`go.mod:5-27`, `go.sum:1-54`): direct imports marked indirect, required test dependencies absent from the declared graph, and unused requirements on Bubbles and Lip Gloss (`charm.land/bubbles/v2`, `charm.land/lipgloss/v2`) that tidy would remove because the implementation does not import them.

**Decision (selected 2026-09-14): Removal.** Bubbles and Lip Gloss are not imported by the implementation and will not be retained through token imports. Let tidy drop both modules, and update every current stated-stack/instruction reference to match — the PRD *Further Notes* stated-stack line,
`Notes/skills/code-writing/styling-tui.md`, and the corresponding entry in `Notes/skills/AGENTS.md`. This issue closes once the manifests and current documentation/instructions agree and tidy is clean.

Commit a clean `go mod tidy` result for the selected removal outcome with correct direct/indirect marking and test dependencies declared. Verify that state directly in this issue with `go mod tidy -diff`. Issue 50 exclusively owns creation of `scripts/verify.sh` and will incorporate this direct check into the persistent closing gate after Issue 49 has closed; Issue 49 neither creates nor partially owns that script.

### How to verify

- **Manual**: run `go mod tidy -diff` on the committed result → empty diff; `go build ./...` and `go test ./...` pass against the tidied graph.
- **Automated**: invoke `go mod tidy -diff` directly and require a clean result; `go mod verify` passes; the build/test suite confirms no missing or unused requirements. Issue 50 later adds this already-green command to the permanent verification routine.

### Acceptance criteria

- [ ] Given the selected removal decision, then the Bubbles/Lip Gloss `go.mod` and `go.sum` entries are dropped and the PRD *Further Notes* stack line, `Notes/skills/code-writing/styling-tui.md`, and its `Notes/skills/AGENTS.md` entry are updated to match.
- [ ] Given the tidied implementation, then no import exists solely to retain a manifest entry.
- [ ] Given `go mod tidy -diff`, then it reports no drift on the committed manifests.
- [ ] Given the tidied graph, then `go build ./...`, `go vet ./...`, and `go test ./...` all pass.
- [ ] Given this issue closes before Issue 50 starts, then direct `go mod tidy -diff` verification is green and Issue 50 remains the sole owner of creating the standard verification suite that will make this check permanent.

### User stories addressed

- None directly — dependency-manifest hygiene; aligns the declared module graph with the PRD's stated stack and keeps the verification story (story 85's documented-limits spirit; Issue 35's verification gate) reproducible.

---

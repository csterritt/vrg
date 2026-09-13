## Issue 49: Tidy dependency manifests — decide the fate of Bubbles and Lip Gloss

**Type**: HITL — the keep-or-remove decision on Bubbles/Lip Gloss is a human call (they are in the PRD's stated stack but currently unimported)
**Blocked by**: None — can start immediately

### Parent PRD

`Notes/PRD-vrg.md` — audit source: `Notes/critiques/final-audit-vrg.md`, Low finding 14

### What to build

Resolve the dependency-manifest drift reported by `go mod tidy -diff` (`go.mod:5-27`, `go.sum:1-54`): direct imports marked indirect, required test dependencies absent from the declared graph, and unused direct requirements on Bubbles and Lip Gloss that tidy would remove because the implementation does not import them.

- **Decision (HITL)**: are Bubbles and Lip Gloss genuinely required? The PRD names them in the stated stack, but the implementation satisfies its requirements without importing them. Either adopt them where intended (making the requirements real) or accept their removal and update the stated-stack documentation accordingly.
- Commit a clean `go mod tidy` result: correct direct/indirect marking, test dependencies declared.
- Add `go mod tidy -diff` (or equivalent cleanliness check) to the verification routine so drift cannot silently return — see Issue 50's suite.

### How to verify

- **Manual**: run `go mod tidy -diff` on the committed result → empty diff; `go build ./...` and `go test ./...` pass against the tidied graph.
- **Automated**: `go mod tidy -diff` run in verification returns clean; `go mod verify` passes; the build/test suite confirms no missing or unused requirements.

### Acceptance criteria

- [ ] Given the Bubbles/Lip Gloss decision, then either the code imports them where intended or the go.mod entries and stated-stack docs are updated to reflect removal.
- [ ] Given `go mod tidy -diff`, then it reports no drift on the committed manifests.
- [ ] Given the tidied graph, then `go build ./...`, `go vet ./...`, and `go test ./...` all pass.
- [ ] Given future changes, then the tidy check is part of the standard verification suite so drift fails fast.

### User stories addressed

- None directly — dependency-manifest hygiene; aligns the declared module graph with the PRD's stated stack and keeps the verification story (story 85's documented-limits spirit; Issue 35's verification gate) reproducible.

---

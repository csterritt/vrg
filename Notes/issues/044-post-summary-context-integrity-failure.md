## Issue 44: Post-`summary` `context` records are integrity failures, not exemptions

**Type**: AFK
**Blocked by**: Issue 36 — AC4 and the outcome-composition verification require Issue 36's composed integrity diagnostic; the parser change itself can be developed first, but this issue cannot close until the diagnostic contract exists

### Parent PRD

`Notes/PRD-vrg.md` — audit source: `Notes/critiques/final-audit-vrg.md`, Medium finding 9

### What to build

Resolve the contradiction between Issue 9's former “`context(P)` in any position” exemption and its summary-is-final row (`internal/searchindex/searchindex.go:487-526`) in favor of the PRD's rule that *any* record after `summary` is a stream-integrity failure. Issue 9's context row is amended to cover only pre-`summary` positions; the implementation currently exempts post-`summary` context records, and the lifecycle test at `internal/searchindex/lifecycle_test.go:90-100` explicitly encodes that superseded expectation.

- Mark every record appearing after `summary` — including `context` — as a stream-integrity failure, uniformly with the other post-summary rows in Issue 9's transition matrix.
- Keep the pre-`summary` `context` semantics unchanged: context payloads remain ignored for matching/lifecycle purposes *before* `summary` — only their post-`summary` position is a violation.
- Correct the contradictory test row so the lifecycle matrix asserts the PRD contract rather than the exemption.

See PRD *Result index, records, and stream integrity* (summary-is-final contract) and Issue 9's lifecycle transition matrix.

### How to verify

- **Manual**: fake rg emitting valid records, then `summary`, then a `context` record, exiting 0 → the outcome is treated as a stream-integrity failure (fatal path per the outcome matrix), not silently accepted.
- **Automated**: the corrected lifecycle matrix row asserts `context`-after-`summary` is an integrity failure; neighbouring rows confirm `context` before `summary` is still ignored for lifecycle purposes, and the outcome composition from Issue 36 reports the after-`summary` violation in the diagnostic.

### Acceptance criteria

- [ ] Given a `context` record after `summary`, then the stream is marked as an integrity failure exactly as for any other post-summary record.
- [ ] Given `context` records before `summary`, then they remain ignored for match/lifecycle semantics as before.
- [ ] Given the lifecycle test matrix, then no row contradicts the summary-is-final contract.
- [ ] Given a post-`summary` `context` violation, then the composed diagnostic (per Issue 36) identifies an after-`summary` record as the cause.

### User stories addressed

- User story 19: missing completion metadata / stream integrity reported
- User story 20: incomplete-stream dispositions applied per the lifecycle matrix

---

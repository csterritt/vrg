## Issue 44: Post-`summary` `context` records are integrity failures, not exemptions

**Type**: AFK
**Blocked by**: Issue 36 — Issue 36 owns the post-summary parser change and contradictory lifecycle-row correction as well as the composed integrity diagnostic; this issue begins from that green boundary and adds dedicated `context` coverage and documentation

### Parent PRD

`Notes/PRD-vrg.md` — audit source: `Notes/critiques/final-audit-vrg.md`, Medium finding 9

### What to build

Resolve the contradiction between Issue 9's former “`context(P)` in any position” exemption and its summary-is-final row (`internal/searchindex/searchindex.go:487-526`) in favor of the PRD's rule that *any* record after `summary` is a stream-integrity failure. Issue 9's context row is amended to cover only pre-`summary` positions. Issue 36 owns removing the implementation exemption and correcting the superseded lifecycle row at `internal/searchindex/lifecycle_test.go:90-100` as part of its post-summary precedence GREEN, so this blocked issue begins from a green suite rather than repeating that parser work.

- Add dedicated regression coverage proving every record appearing after `summary` — including `context` — is a stream-integrity failure uniformly with the other post-summary rows in Issue 9's transition matrix.
- Keep and explicitly cover the pre-`summary` `context` semantics: context payloads remain ignored for matching/lifecycle purposes *before* `summary` — only their post-`summary` position is a violation.
- Verify the structured cause list and Issue 36 outcome composition report exactly the after-`summary` cause for the dedicated `context` case; do not add a second parser change.

See PRD *Result index, records, and stream integrity* (summary-is-final contract) and Issue 9's lifecycle transition matrix.

### How to verify

- **Manual**: fake rg emitting valid records, then `summary`, then a `context` record, exiting 0 → the outcome is treated as a stream-integrity failure (fatal path per the outcome matrix), not silently accepted.
- **Automated**: confirm Issue 36's corrected lifecycle matrix row remains green; dedicated structured-cause coverage asserts `context`-after-`summary` yields exactly the after-`summary` cause, neighbouring rows confirm `context` before `summary` is still ignored for lifecycle purposes, and a focused Issue 36 outcome-composition assertion reports that cause in the complete overlay/replay diagnostic.

### Acceptance criteria

- [ ] Given a `context` record after `summary`, then the stream is marked as an integrity failure exactly as for any other post-summary record.
- [ ] Given `context` records before `summary`, then they remain ignored for match/lifecycle semantics as before.
- [ ] Given the lifecycle test matrix, then no row contradicts the summary-is-final contract.
- [ ] Given a post-`summary` `context` violation, then the composed diagnostic (per Issue 36) identifies an after-`summary` record as the cause.

### User stories addressed

- User story 19: missing completion metadata / stream integrity reported
- User story 20: incomplete-stream dispositions applied per the lifecycle matrix

---

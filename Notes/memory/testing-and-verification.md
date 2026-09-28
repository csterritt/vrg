---
uuid: 1f1bb470-01d4-6fe1-ac35-2f74a0b484e0
created: '2026-09-28T14:15:04Z'
updated: '2026-09-28T14:15:04Z'
title: Testing and verification
summary: Test layout (subprocess boundary, black-box cli, internal) and the scripts/verify.sh
  ten-gate pipeline.
---
# Testing and verification

## Test layout

- `cmd/vrg/main_test.go` — builds the real binary once in `TestMain` and
  asserts at the subprocess boundary: exit statuses, stdout/stderr ownership,
  help/usage shapes, hostile argv0 and operand escaping, no terminal control
  bytes, and that help never execs ripgrep.
- `internal/cli/cli_test.go` — black-box (`package cli_test`) tests of
  `Parse`: help precedence over every error class, token classification,
  root validation via injected `Stat` (a `failStat` sentinel proves
  validation is skipped where required).
- `internal/cli/internal_test.go` — white-box pin: `newApp` must keep
  `flag.ContinueOnError` so the library never exits the process.

## verify.sh — the permanent gate

`scripts/verify.sh` (Issue 50) runs the full post-audit gate set from a clean
checkout, failing on the first non-zero step:

1. `go build ./...`
2. `go vet ./...`
3. `go build -tags vrg_testhooks ./cmd/vrg`
4. `go vet -tags vrg_testhooks ./cmd/vrg`
5. `go test ./... -count=1`
6. `CGO_ENABLED=1 go test -race ./... -count=1`
7. `go test ./cmd/vrg -count=3`
8. `go mod verify`
9. `go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...` (needs network or
   warm caches; reports exit 75 ENVIRONMENT failure when unsatisfiable)
10. `go mod tidy -diff`

The `vrg_testhooks` build tag keeps test hooks out of the production binary
(task 045). The race gate requires a CGO-capable C toolchain.

Sources: `/home/chris/vrg/scripts/verify.sh`,
`/home/chris/vrg/cmd/vrg/main_test.go`,
`/home/chris/vrg/internal/cli/cli_test.go`,
`/home/chris/vrg/internal/cli/internal_test.go`

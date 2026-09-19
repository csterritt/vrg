# Issue #49: tidy dependency manifests — Bubbles/Lip Gloss removal

*2026-09-18T12:13:06Z by Showboat 0.6.1*
<!-- showboat-id: 1641f161-e9f8-41ed-927d-83979ffaef95 -->

Issue #49 resolves the dependency-manifest drift reported by `go mod tidy -diff`: unused requirements on Bubbles and Lip Gloss (`charm.land/bubbles/v2`, `charm.land/lipgloss/v2`) that tidy removes because the implementation imports neither library. The product owner selected **removal** on 2026-09-14 — no token import may be added to retain a manifest entry, the remaining TUI dependency is `charm.land/bubbletea/v2`, and VRG's own `internal/theme` continues to own presentation. See Notes/issues/049-tidy-dependency-manifests.md, Notes/tasks/049-tidy-dependency-manifests.md, and Notes/PRD-vrg.md (*Further Notes*). This is a mechanical no-behaviour-change issue: the walkthrough records the decision, proves both modules are absent from the committed manifests and that no Go source imports them, shows the aligned stated-stack/instruction documents, and captures the issue's verification gates — `go mod tidy -diff`, `go mod verify`, `go build ./...`, `go vet ./...`, and `go test ./...`.

## Bubbles and Lip Gloss are absent from the committed manifests

Neither module path appears in `go.mod` or `go.sum`, and no Go source file imports either — no token import exists to retain a manifest entry.

```bash
grep -n "bubbles\|lipgloss" go.mod go.sum; echo "manifest grep exit: $? (1 = absent)"; grep -rn "charm.land/bubbles\|charm.land/lipgloss" --include="*.go" .; echo "go-source grep exit: $? (1 = no imports)"
```

```output
manifest grep exit: 1 (1 = absent)
go-source grep exit: 1 (1 = no imports)
```

## Stated-stack and instruction documents agree with the manifests

Every current stack claim and prescriptive styling instruction states VRG uses Bubble Tea without Bubbles or Lip Gloss: the PRD *Further Notes* line, [project-overview.md](../../../wiki/project-overview.md), the Scope paragraph in [wiki/AGENTS.md](../../../wiki/AGENTS.md), and [styling-tui.md](../../../skills/code-writing/styling-tui.md). The `code-writing/styling-tui` entry in [skills/AGENTS.md](../../../skills/AGENTS.md) names no removed library. Historical references (Ideas.md, Issue #1/#5/#6, decisions/001, critiques) are preserved unchanged as the record of how the decision evolved.

```bash
grep -n "Bubbles and Lip Gloss" Notes/PRD-vrg.md Notes/wiki/project-overview.md Notes/wiki/AGENTS.md Notes/skills/code-writing/styling-tui.md; echo "---"; grep -n "styling-tui" Notes/skills/AGENTS.md
```

```output
Notes/PRD-vrg.md:343:- Greenfield Go project using Bubble Tea for the TUI and `github.com/jawher/mow.cli` for command-line parsing and generated help. Bubbles and Lip Gloss are intentionally not dependencies because the implementation does not import them. ripgrep 15.x is the reference family; local 15.2.0 checks confirmed default UTF-16 transcoding, UTF-8 BOM removal, CRLF end-position offsets, and cumulative unrestricted semantics.
Notes/wiki/project-overview.md:16:- `charm.land/bubbletea/v2 v2.0.9` for the TUI; Bubbles and Lip Gloss
Notes/wiki/AGENTS.md:62:This wiki covers the vrg project: a Go terminal UI for browsing ripgrep results, using `charm.land/bubbletea/v2` for the TUI and `github.com/jawher/mow.cli` for command-line parsing. Bubbles and Lip Gloss are intentionally not dependencies because the implementation does not import them.
Notes/skills/code-writing/styling-tui.md:8:- Use VRG's existing theme and rendering primitives for terminal presentation. Bubbles and Lip Gloss are intentionally not dependencies; do not add them solely for visual consistency.
---
20:- `code-writing/styling-tui` - You must read this when implementing terminal layout or TUI styling
```

## Verification gates against the committed manifests

The issue's fail-closed check: `go mod tidy -diff` must report no drift — any output fails the task. Then `go mod verify`, `go build ./...`, `go vet ./...`, and `go test ./...`.

```bash
go mod tidy -diff; echo "tidy-diff exit: $? (empty output = no drift)"
```

```output
tidy-diff exit: 0 (empty output = no drift)
```

```bash
go mod verify && go build ./... && go vet ./... && echo "verify+build+vet: all green"
```

```output
all modules verified
verify+build+vet: all green
```

```bash
go test ./... 2>&1 | grep -v "no test files"; echo "test exit: ${PIPESTATUS[0]}"
```

```output
ok  	vrg/cmd/vrg	90.320s
ok  	vrg/internal/app	3.334s
ok  	vrg/internal/cli	(cached)
ok  	vrg/internal/docs	(cached)
ok  	vrg/internal/filebuffer	(cached)
ok  	vrg/internal/safepresentation	(cached)
ok  	vrg/internal/searchindex	(cached)
ok  	vrg/internal/theme	(cached)
ok  	vrg/internal/viewport	(cached)
test exit: 0
```

## Outcome

The removal decision is in effect: `charm.land/bubbles/v2` and `charm.land/lipgloss/v2` are absent from `go.mod` and `go.sum`, no Go source imports them, every current stated-stack and prescriptive reference describes Bubble Tea alone with VRG's own theme/rendering primitives, `go mod tidy -diff` is empty, and verify/build/vet/test are green. Per the issue boundary, `scripts/verify.sh` is not created here — Issue #50 owns it and adopts this already-green `go mod tidy -diff` into the permanent gate after Issue #49 closes.

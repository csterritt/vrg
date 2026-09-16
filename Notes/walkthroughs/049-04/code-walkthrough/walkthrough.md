# Issue #49: tidy dependency manifests — remove unused Bubbles and Lip Gloss requirements

*2026-09-15T23:30:03Z by Showboat 0.6.1*
<!-- showboat-id: 85c757eb-47f2-41dd-8a9c-ebc85097ec59 -->

Walkthrough for Issue #49 (Notes/issues/049-tidy-dependency-manifests.md, Notes/tasks/049-tidy-dependency-manifests.md), a mechanical no-behaviour-change dependency-manifest tidy. The product owner selected removal on 2026-09-14: charm.land/bubbles/v2 and charm.land/lipgloss/v2 are not imported by the implementation and must not be retained — no token import may be added to keep a manifest entry. The remaining TUI dependency is charm.land/bubbletea/v2; VRG's existing theme and rendering primitives continue to own presentation (Notes/PRD-vrg.md *Further Notes* stated stack). Audit source: Notes/critiques/final-audit-vrg.md, Low finding 14. Issue #50 exclusively owns scripts/verify.sh; this issue verifies the committed manifests directly with go mod tidy -diff as the fail-closed check.

## Bubbles and Lip Gloss absent from go.mod and go.sum

```bash
cd /home/chris/vrg && grep -n "bubbles\|lipgloss" go.mod go.sum; echo "grep exit: $? (1 = both absent)"
```

```output
grep exit: 1 (1 = both absent)
```

Neither module appears in either manifest file. No import exists solely to retain a manifest entry — a source scan finds zero references, while charm.land/bubbletea/v2 remains the declared, actually-imported TUI dependency:

```bash
cd /home/chris/vrg && grep -rn "charm.land/bubbles\|charm.land/lipgloss" --include="*.go" . ; echo "source grep exit: $? (1 = no token imports)"; grep -n "bubbletea" go.mod; grep -rn "charm.land/bubbletea" --include="*.go" . | head -5
```

```output
source grep exit: 1 (1 = no token imports)
6:	charm.land/bubbletea/v2 v2.0.9
./internal/app/read_failure_test.go:13:	tea "charm.land/bubbletea/v2"
./internal/app/layout_test.go:9:	tea "charm.land/bubbletea/v2"
./internal/app/too_small_test.go:7:	tea "charm.land/bubbletea/v2"
./internal/app/reveal_horizontal_test.go:7:	tea "charm.land/bubbletea/v2"
./internal/app/scroll_test.go:8:	tea "charm.land/bubbletea/v2"
```

## Stated-stack documents and coding instructions agree

Every current stated-stack or prescriptive reference describes Bubble Tea without Bubbles or Lip Gloss; historical audit/issue/critique references are preserved as decision context:

```bash
cd /home/chris/vrg && grep -n "Bubbles and Lip Gloss" Notes/PRD-vrg.md Notes/wiki/project-overview.md Notes/wiki/AGENTS.md Notes/skills/code-writing/styling-tui.md && grep -n "styling-tui" Notes/skills/AGENTS.md
```

```output
Notes/PRD-vrg.md:343:- Greenfield Go project using Bubble Tea for the TUI and `github.com/jawher/mow.cli` for command-line parsing and generated help. Bubbles and Lip Gloss are intentionally not dependencies because the implementation does not import them. ripgrep 15.x is the reference family; local 15.2.0 checks confirmed default UTF-16 transcoding, UTF-8 BOM removal, CRLF end-position offsets, and cumulative unrestricted semantics.
Notes/wiki/project-overview.md:16:- Bubble Tea v2 pinned for the TUI: `charm.land/bubbletea/v2 v2.0.9`; Bubbles and Lip Gloss are intentionally not dependencies because the implementation does not import them
Notes/wiki/AGENTS.md:62:This wiki covers the vrg project: a Go terminal UI for browsing ripgrep results, using `charm.land/bubbletea/v2` for the TUI and `github.com/jawher/mow.cli` for command-line parsing. Bubbles and Lip Gloss are intentionally not dependencies because the implementation does not import them.
Notes/skills/code-writing/styling-tui.md:8:- Use VRG's existing theme and rendering primitives for terminal presentation. Bubbles and Lip Gloss are intentionally not dependencies; do not add them solely for visual consistency.
20:- `code-writing/styling-tui` - You must read this when implementing terminal layout or TUI styling
```

## Fail-closed check: go mod tidy -diff reports no drift

```bash
cd /home/chris/vrg && out=$(go mod tidy -diff) && if [ -z "$out" ]; then echo "tidy -diff: EMPTY (no drift)"; else echo "$out"; exit 1; fi
```

```output
tidy -diff: EMPTY (no drift)
```

## Verification gates: verify, build, vet, test

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && go test ./... 2>&1 | grep -v "no test files" | sed "s/\tvrg/\tvrg/; s/([0-9.]*s)//g; s/[[:space:]]*$//"
```

```output
all modules verified
ok  	vrg/cmd/vrg	(cached)
ok  	vrg/internal/app	(cached)
ok  	vrg/internal/cli	(cached)
ok  	vrg/internal/docs	(cached)
ok  	vrg/internal/filebuffer	(cached)
ok  	vrg/internal/safepresentation	(cached)
ok  	vrg/internal/searchindex	(cached)
ok  	vrg/internal/theme	(cached)
ok  	vrg/internal/viewport	(cached)
```

All acceptance criteria are met: the removal decision is recorded in the issue and task plan; Bubbles and Lip Gloss are absent from go.mod, go.sum, and every current stack/instruction document; no token import exists; go mod tidy -diff is empty on the committed manifests; and verify/build/vet/test are green. scripts/verify.sh is intentionally not created — Issue #50 exclusively owns it.

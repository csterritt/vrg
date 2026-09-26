# Issue #49: Tidy dependency manifests — remove unused Bubbles and Lip Gloss requirements

*2026-09-25T14:53:54Z by Showboat 0.6.1*
<!-- showboat-id: fd3bef3b-470f-4ec3-8744-901c56c29bd4 -->

Issue #49 (`Notes/tasks/049-tidy-dependency-manifests.md`, parent PRD `Notes/PRD-vrg.md`) tidies the Go dependency manifests. The product owner selected removal on 2026-09-14: the implementation imports neither `charm.land/bubbles/v2` nor `charm.land/lipgloss/v2`, so both requirements must be absent from `go.mod`/`go.sum` and no token import may be added to retain a manifest entry. The remaining TUI dependency is `charm.land/bubbletea/v2`; VRG's `internal/theme` styles and `internal/present` rendering primitives own presentation. This walkthrough proves the libraries are absent from the manifests and the current stack/instruction documents, then records the verification gates against the committed state.

```bash
cd /home/chris/vrg && echo "== go.mod ==" && cat go.mod && echo "== go.sum entries for removed libraries ==" && grep -n "bubbles\|lipgloss" go.mod go.sum; echo "grep exit status: $? (1 = absent from both manifests)"
```

```output
== go.mod ==
module vrg

go 1.27.1

require (
	charm.land/bubbletea/v2 v2.0.9
	github.com/charmbracelet/x/ansi v0.11.8
	github.com/jawher/mow.cli v1.2.0
	golang.org/x/sys v0.47.0
)

require (
	github.com/aymanbagabas/go-udiff v0.4.1 // indirect
	github.com/charmbracelet/colorprofile v0.4.3 // indirect
	github.com/charmbracelet/ultraviolet v0.0.0-20260811164956-006e29f97886 // indirect
	github.com/charmbracelet/x/exp/golden v0.0.0-20250806222409-83e3a29d542f // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/charmbracelet/x/termios v0.1.1 // indirect
	github.com/charmbracelet/x/windows v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.1 // indirect
	github.com/mattn/go-runewidth v0.0.27 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	golang.org/x/exp v0.0.0-20231006140011-7918f672742d // indirect
	golang.org/x/sync v0.22.0 // indirect
)
== go.sum entries for removed libraries ==
grep exit status: 1 (1 = absent from both manifests)
```

No Go source imports either removed library, so nothing exists solely to retain a manifest entry. The current stated-stack and prescriptive documents — the PRD *Further Notes* line, the wiki project overview and schema Scope, and the `code-writing/styling-tui` skill plus its `Notes/skills/AGENTS.md` entry — now state Bubble Tea without Bubbles or Lip Gloss; the only surviving mentions in those documents name the removal itself. Historical records (Issue #1/#5/#6, `Notes/decisions/001`, `Notes/Ideas.md`, early critiques/tasks) keep their original wording because they explain the decision rather than state current requirements.

```bash
cd /home/chris/vrg && echo "== Go imports ==" && grep -rn "bubbles\|lipgloss" --include="*.go" . ; echo "go-import grep exit: $? (1 = no imports)" && echo "== current stack/instruction documents ==" && grep -rn "bubbles\|lipgloss\|Bubbles\|Lip Gloss" Notes/PRD-vrg.md Notes/wiki/project-overview.md Notes/wiki/AGENTS.md Notes/skills/AGENTS.md Notes/skills/code-writing/styling-tui.md
```

```output
== Go imports ==
go-import grep exit: 1 (1 = no imports)
== current stack/instruction documents ==
Notes/PRD-vrg.md:343:- Greenfield Go project using Bubble Tea, with `github.com/jawher/mow.cli` for command-line parsing and generated help. Bubbles and Lip Gloss are not used and were removed from the dependency manifest under Issue #49; VRG's theme and rendering primitives own presentation. ripgrep 15.x is the reference family; local 15.2.0 checks confirmed default UTF-16 transcoding, UTF-8 BOM removal, CRLF end-position offsets, and cumulative unrestricted semantics.
Notes/wiki/project-overview.md:16:- `charm.land/bubbletea/v2 v2.0.9` for the TUI; Bubbles and Lip Gloss
Notes/wiki/AGENTS.md:62:This wiki covers the vrg project: a Go terminal application for browsing ripgrep results using Bubble Tea for the TUI (without Bubbles or Lip Gloss — the `theme`/`present` packages own presentation) and `mow.cli` for command parsing.
Notes/skills/code-writing/styling-tui.md:8:- Bubble Tea is the only TUI library. Bubbles and Lip Gloss are not dependencies (removed under Issue #49); terminal presentation is owned by the `internal/theme` styles and `internal/present` rendering primitives. Do not add token imports of removed libraries.
```

The fail-closed drift check: `go mod tidy -diff` must print nothing — any output would fail the task — followed by `go mod verify` and the build/vet gates.

```bash
cd /home/chris/vrg && out=$(go mod tidy -diff) && echo "go mod tidy -diff output bytes: ${#out}" && go mod verify && go build ./... && go vet ./... && echo "VERIFY+BUILD+VET OK"
```

```output
go mod tidy -diff output bytes: 0
all modules verified
VERIFY+BUILD+VET OK
```

```bash
cd /home/chris/vrg && go test ./... -count=1 | sed -E "s/[[:space:]][0-9.]+s$//"
```

```output
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/present
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
```

All gates green against the committed manifests. Issue #49 is a mechanical no-behaviour-change tidy: `charm.land/bubbles/v2` and `charm.land/lipgloss/v2` are absent from `go.mod` and `go.sum`, no Go source imports them, `go mod tidy -diff` reports zero drift, `go mod verify` passes, and `go build`/`go vet`/`go test ./...` are clean. Stated-stack documents and the `code-writing/styling-tui` skill now prescribe Bubble Tea alone; historical records explaining the decision are preserved. `scripts/verify.sh` was deliberately not created — Issue #50 owns that file and adopts this already-green tidy command into the permanent gate. Sources: `Notes/tasks/049-tidy-dependency-manifests.md`, `Notes/PRD-vrg.md` (*Further Notes*), `go.mod`, `go.sum`, `Notes/wiki/project-overview.md`, `Notes/wiki/AGENTS.md`, `Notes/skills/code-writing/styling-tui.md`, `Notes/skills/AGENTS.md`.

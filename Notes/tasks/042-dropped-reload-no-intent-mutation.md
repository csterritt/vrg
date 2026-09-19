# Tasks for #42: A dropped `r` request must not change revision or reveal intent

Parent issue: #42
Parent PRD: PRD-vrg.md
**Blocked by issues**: none
**Acceptance criteria**: AC1–AC6 → Tasks 1–2
**Manual verification**: Task 3 owns the issue's manual checks.

## Tasks

### 1. Specify atomic reload admission

**Type**: RED  
**Output**: Failing tests assert a dropped `r` during a startup or navigation load leaves revision, intent, and presentation untouched and the in-flight load completes under its original classification, while an accepted `r` applies exactly one revision increment with anchor-preserving intent and navigation re-entry is ungated.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Add failing tests in `internal/app` covering the load-admission boundary. Require: `r` pressed while the startup load is in flight leaves the revision, `IntentReveal`, and presentation exactly as they were, and the load's completion performs the required first-match reveal (story 50) rather than anchor preservation; `r` pressed during a navigation load likewise leaves the pending destination reveal intact per the latest-target rules rather than being replaced by `IntentReloadAnchor` behaviour; an `r` accepted after the previous load finished applies reload flags, "Loading…" presentation, `IntentReloadAnchor`, and exactly one revision increment; rapid repeated `r` presses keep at most one reload in flight per path with the placeholder→content/unreadable completion signal preserved; and navigation re-entry while a duplicate path load is already in flight still updates the selection, placeholder presentation, and `IntentReveal` — the atomic-admission restriction applies only to `handleReload`. The dropped-`r` cases fail on the current code, which marks `reloadingPaths`, switches to `IntentReloadAnchor`, and changes presentation before `startLoad` drops the request. Keep this task test-only.

---

### 2. Make reload admission atomic with state mutation

**Type**: GREEN  
**Output**: The admission tests pass; `handleReload` mutates reload state only when a new load request is actually accepted — single decision point, no intermediate committed state — and the navigation re-entry path is unchanged.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Restructure `handleReload` and `startLoad` in `internal/app/app.go` so the one-load-per-path admission check and the reload-state mutation are a single decision point: check whether a load is already in flight for the current path *before* recording `reloadingPaths`, switching to "Loading…" presentation, or setting `IntentReloadAnchor`, and apply those mutations only when the new load request is actually accepted. A dropped `r` must leave revision, intent, and presentation exactly as they were so the in-flight startup or navigation load completes under its original classification — never misclassified as a reload with an extra revision bump or anchor preservation. Keep the accepted-reload path unchanged — flags, presentation, intent, and the revision bump occur precisely when the new request starts — and do not gate the navigation re-entry path behind successful load admission: re-entry legitimately changes the current selection, placeholder presentation, and `IntentReveal` even when `startLoad` drops a duplicate load. Run the focused tests plus `go build ./...`, `go vet ./...`, and `go test ./...`.

---

### 3. Create the reload-admission walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/042-04/code-walkthrough`.  
**Depends on**: 2

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/042-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the admission tests, then run the issue's manual scenario: with a slow-loading file (test seam or genuinely large file), press `r` while the startup or navigation load is still in flight → nothing visible changes, and when the load completes the destination match is revealed per the normal rules — not anchor-preserved as if a reload had happened. Capture commands, outputs, and exit statuses. Reference Issue #42 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---

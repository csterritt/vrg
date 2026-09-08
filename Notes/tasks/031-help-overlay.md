# Tasks for #31: Help overlay (`h`/`?`) with wrapped, scrollable key bindings

Parent issue: #31
Parent PRD: PRD-vrg.md
**Blocked by issues**: #6, #9, #15
**Acceptance criteria**: AC1–AC8 → Tasks 1–2
**Manual verification**: Task 4 owns the issue's manual checks.

## Tasks

### 1. Specify the help overlay

**Type**: RED  
**Output**: Failing model tests cover opening from browse and no-results, close and ignored keys, `ctrl+c`, scroll bounds, wrapping including unbroken strings, tiny-size clipping, binding-table rendering, pop-up cancellation, and the sink-safety row.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issues #6, #9, and #15 are complete. Add failing model tests in `internal/app` for the Issue #31 contracts and the help bullets of the Colours, overlays, and key precedence section of `Notes/PRD-vrg.md`. Require `h`/`?` to open the modal help from ordinary browsing and the no-results screen, with closing returning to the underlying base state and a subsequent `q` on no-results still exiting 1; while open, `up`/`down` to scroll rendered rows, `q`/`Esc`/`h`/`?` to close, `ctrl+c` to exit 130, and every other key — including `n`/`p`/`w`/`c`/`r` — to be ignored with the state behind unchanged; text to wrap to the interior width including long unbroken strings, with vertical scrolling reaching every row at usable sizes; the overlay to be clipped to the terminal at tiny sizes without a borderless mode and restored on growth, rendering without panic at 25×8; the key-binding list to be defined once as data (binding → description) that the help renderer consumes and documentation tests can iterate, plus a footer slot; opening help to cancel an active Issue #15 pop-up with no return on close; and any substituted text to be routed through the Issue #6 utility with help added to the sink-safety table. Keep this task test-only.

---

### 2. Implement the help overlay and binding table

**Type**: GREEN  
**Output**: Help tests pass with the shared wrapped-scrollable overlay component and the single binding-table data source.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the help overlay in `internal/app` on the wrapped, scrollable overlay component shared with the Issue #9 error overlay, using the Theme's overlay style with base colours and a plain single-line border, the single binding-table data source covering navigation, scrolling, panning, wrap, colour, list toggle, reload, help, and quit/cancel bindings with its footer slot, pop-up cancellation on open, and the key routing from Task 1. Keep the binding table exposed for Issue #34's documentation test.

---

### 3. Document the help overlay

**Type**: DOCUMENT  
**Output**: Wiki documentation records help keys, the shared overlay component, the binding table, and tiny-size clipping.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #31 implementation and tests into the appropriate pages under `Notes/wiki`. Document the `h`/`?` open behavior from browse and no-results, the close and ignored keys with `ctrl+c`, wrapped scrollable text including unbroken strings, tiny-size clipping without a borderless mode, the shared overlay component with the error overlay, pop-up cancellation, the binding table as the single source for rendering and documentation tests, and the footer slot reserved for Issue #34. Cross-reference Issue #31 and the Colours, overlays, and key precedence section of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the help-overlay walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/031-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/031-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the help model tests, then run the manual cases: `?` opening the bordered help, `down` scrolling, `n` doing nothing to the file behind, `Esc` closing, shrinking to 25×8 showing clipped-but-present help, and enlarging restoring the normal layout, plus opening help from the no-results screen and closing back to it with `q` exiting 1. Reference Issue #31 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---

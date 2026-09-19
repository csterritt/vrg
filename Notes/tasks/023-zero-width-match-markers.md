# Tasks for #23: Zero-width match markers

Parent issue: #23
Parent PRD: PRD-vrg.md
**Blocked by issues**: #20, #22
**Acceptance criteria**: AC1–AC5 → Tasks 1–2

## Tasks

### 1. Specify zero-width match markers

**Type**: RED  
**Output**: Failing FileBuffer and Viewport tests cover markers at BOL, inside wide clusters, at EOL, on empty lines, and on LF/CRLF terminators; marker extent and wrap-row behaviour; indicator participation; reveal targeting; and the marker-only-line extent with maximum pan offset 0.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issues #20 and #22 are complete. Add failing tests in `internal/filebuffer` and `internal/viewport` for the Issue #23 contracts and the zero-width bullets of the Text, graphemes, and safe presentation section of `Notes/PRD-vrg.md`. Require a zero-width submatch to render as one inverse-video space at its mapped location, underlined on the current matched line, marking an existing cell without shifting following text; a marker at end of line to extend the effective line width by one cell, so an empty matched line has width one and a marker after a completely full wrap row occupies another row; a position inside a cluster to map to the cluster start with no split wide glyph; a terminator-only `$` match on `hit\r\n` to produce a single marker cell at display column 3 following exactly the same reveal, wrap, clip, horizontal-extent, and indicator rules as any other marker, including counting as entirely hidden for the gutter `*` and right `*`; marker cells to be navigable reveal targets; and markers to participate in horizontal extent and pan clamping so a marker-only line has extent 1 and maximum offset 0 under Issue #18's paintable-boundary maximum. Keep this task test-only.

---

### 2. Implement zero-width markers

**Type**: GREEN  
**Output**: Marker tests pass; markers participate in reveal, wrap, clip, extent, and indicator rules like any other cell.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the zero-width marker in `internal/filebuffer` and `internal/viewport` to satisfy Task 1: the one-cell inverse marker at mapped locations including end-of-line and empty lines, cluster-start mapping, effective-width extension, wrap-row occupation, marker extents feeding Issue #18's maximum, marker cells as reveal targets, and marker visibility feeding Issue #20's indicators. Treat the terminator-only marker as an ordinary marker with no special cases.

---

### 3. Create the finish marker

**Type**: FINISH MARKER  
**Output**: Finish marker exists at `Notes/finish-markers/023-04/finish-marker.md`.  
**Depends on**: 2

Write `Task 023-04 finished successfully at <time>` to `Notes/finish-markers/023-04/finish-marker.md`, replacing `<time>` with the current UTC timestamp (for example, `date -u +"%Y-%m-%dT%H:%M:%SZ"`). Create the `Notes/finish-markers/023-04/` directory if it does not already exist.

---

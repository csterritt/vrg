---
uuid: 1f1bb46f-ff02-6e90-bd90-5ad8c7866637
created: '2026-09-28T14:15:04Z'
updated: '2026-09-28T14:15:04Z'
title: internal package skeletons
summary: 'Declared responsibilities of the five doc-only internal packages: app, filebuffer,
  searchindex, theme, viewport.'
---
# internal package skeletons

Five packages under `internal/` currently contain only a `doc.go` with the
package clause and doc comment — no code. The comments are the authoritative
statement of what each package will own as later issues land:

- `app` — "coordinates lifecycle, state-specific input, asynchronous work,
  cached files, overlays, safe diagnostic replay, cleanup, and frame
  composition." The top-level TUI coordinator.
- `filebuffer` — "loads and classifies one file and turns content plus
  original match data into safe, display-ready source lines and validated
  highlights."
- `searchindex` — "owns parsed ripgrep result data, stream-integrity
  accounting, exclusions, and the circular matched-line cursor." Includes
  binary-file exclusion and malformed/oversized record accounting.
- `theme` — "owns the active colour scheme and styles."
- `viewport` — "owns logical reading position and derives rendered-row
  visibility in wrap and run-off-edge modes." Covers scroll position,
  wrapping, and horizontal panning.

Anything beyond these doc comments is unimplemented; do not assume APIs exist.
The division anticipates the Bubble Tea (charm.land/bubbletea/v2) TUI declared
in `go.mod` as an indirect dependency.

Sources: `/home/chris/vrg/internal/app/doc.go`,
`/home/chris/vrg/internal/filebuffer/doc.go`,
`/home/chris/vrg/internal/searchindex/doc.go`,
`/home/chris/vrg/internal/theme/doc.go`,
`/home/chris/vrg/internal/viewport/doc.go`

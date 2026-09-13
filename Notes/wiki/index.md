# Wiki index

Catalog of all wiki pages for the vrg project.

## Overview

- [project-overview.md](project-overview.md) — what vrg is, stack, layout

## Concepts and architecture

- [cli-foundation.md](cli-foundation.md) — Issue #1 CLI: mow.cli adapter,
  emission-prevention output strategy, shared declarations, preflight,
  help/positional/root/sanitization contracts
- [cli-flag-forwarding.md](cli-flag-forwarding.md) — Issue #2 CLI: search
  flag allow-list, ordered exact-spelling scan records, combined shorts,
  cumulative `-u`, assignment rejection, exact child argv
- [search-collection-path.md](search-collection-path.md) — Issue #3
  (extended by Issue #4): ripgrep subprocess execution, dual-pipe
  drainage, SearchIndex parsing, the searching/summary TUI lifecycle,
  cancellation, child termination/reaping, terminal restoration, and
  controlled-failure cleanup
- [browse-tracer.md](browse-tracer.md) — Issue #5: two-pane browse view
  with file list, filename rule, gutter, inverse-video highlights,
  async file loading with "Loading…" placeholder, safe-presentation
  core for paths and content, and hostile-fixture sink-safety method
- [safe-presentation.md](safe-presentation.md) — Issue #6: shared
  safe-presentation utility for every output sink (path, content,
  diagnostic escaping), unified cli.Escape, shared sink-safety table
  and hostile fixtures, no-style composition method, and future-issue
  sink ownership rule

## Catalogs

- [source-code.md](source-code.md) — all files under `cmd/` and
  `internal/` with purposes
- [unit-tests.md](unit-tests.md) — unit and subprocess-boundary test
  catalog

## Bookkeeping

- [log.md](log.md) — chronological ingest/query/lint record

# Wiki index

Catalog of all wiki pages for the vrg project.

## Overview

- [project-overview.md](project-overview.md) — what vrg is, stack, layout

## Concepts and architecture

- [cli-foundation.md](cli-foundation.md) — Issue #1 CLI: mow.cli adapter,
  emission-prevention output strategy, shared declarations, preflight,
  help/positional/root/sanitization contracts
- [cli-flags-child-argv.md](cli-flags-child-argv.md) — Issue #2 CLI:
  search-flag allow-list, ordered scan records, combined shorts,
  cumulative `-u`, assignment rejection, exact child argv
- [search-collection.md](search-collection.md) — Issue #3 app/search
  path: `rg` spawn with the protected argv from the invocation working
  directory, concurrent dual-pipe drainage, the "Searching…" state
  spanning collection and index preparation, gate/collect-ack test
  seams, interim summary with `q` → exit 0, start-failure exit 2, and
  the SearchIndex record/merge/order/resolution contracts

## Catalogs

- [source-code.md](source-code.md) — all files under `cmd/` and
  `internal/` with purposes
- [unit-tests.md](unit-tests.md) — unit and subprocess-boundary test
  catalog

## Bookkeeping

- [log.md](log.md) — chronological ingest/query/lint record

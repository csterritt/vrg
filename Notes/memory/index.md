---
uuid: 1f1bb46e-4c1a-66ea-bb66-f3481a99d6ac
created: '2026-09-28T14:14:18Z'
updated: '2026-09-28T14:14:18Z'
title: vrg-info memoryfield
summary: Introduction to the memoryfield documenting the vrg Go codebase.
---
# vrg-info

This field documents the **vrg** codebase: a Go terminal application that runs
ripgrep and browses the results in a TUI. Pages record what the source actually
does — structure, contracts, and verification — not the product spec.

The product requirements live in the repository at `Notes/PRD-vrg.md`; planned
work is tracked per-issue in `Notes/issues/` and `Notes/tasks/`. As of
2026-09-28 only the CLI layer and process boundary are implemented; the five
`internal/` domain packages are doc-comment skeletons awaiting their issues.

Sources: `/home/chris/vrg/go.mod`, `/home/chris/vrg/Notes/PRD-vrg.md`

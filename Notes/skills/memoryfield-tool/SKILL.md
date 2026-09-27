---
name: memoryfield-skill
description: >-
  Read, write, search, validate, index, export, and serve memoryfields —
  collections of Markdown memory pages with optional YAML frontmatter and
  optional vector indexes — using the memoryfield-tool CLI. Use when the user
  asks to look up, create, update, search, validate, or export memory pages, or
  mentions "memoryfield", "memory field", "memoryfield-tool", or a
  .memoryfield.zip.
---

# Memoryfields

A memoryfield is a flat collection of Markdown pages with optional YAML
frontmatter and an optional vector index for semantic search. The Markdown
pages are the canonical data; the vector index is derived and can be
regenerated at any time. Fields are designed for search-first access: they can
hold hundreds of pages on any topic, and normally only the relevant few should
enter your context window.

The `memoryfield-tool` CLI reads, writes, searches, validates, and exports
memoryfields. This skill explains how to use it. The complete format
specification lives in `references/SPEC.md`.

## Prerequisite

`memoryfield-tool` must be installed and on PATH. Verify with:

    memoryfield-tool --schema

This prints the full command schema (all commands, flags, defaults, choices)
as JSON. Run it whenever you need exact flags or defaults; do not guess or rely
on memory. If the command is missing, ask the user how they install it
(typically `uv tool install .` from the memoryfield-tool checkout). Do not
proceed without the tool.

`ollama` is a second prerequisite: it serves the embedding model
(`nomic-embed-text-v1.5`) that powers semantic `search` and `index`. Check
with `which ollama`. Without it, `search` still works but falls back to
substring matching, and `index` cannot run. If it is missing or not running,
tell the user rather than trying to install or start it yourself.

## Commands

Run `memoryfield-tool --schema` for the authoritative reference. The essential
commands:

| Command                                                                                  | Purpose                                                                           |
|------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------|
| `catalog [--field NAME] [--sort path\|title\|created\|updated] [--json]`                 | List all pages with frontmatter metadata (enumeration — prefer search, see below) |
| `connect NAME LOCATION [--endpoint-url URL]`                                             | Connect an existing dir or `s3://` as a field                                     |
| `create NAME [--location PATH]`                                                          | Create a new field with an index page                                             |
| `edit [--field NAME] PAGE [--editor CMD]`                                                | Open a page in $EDITOR and write it back                                          |
| `export [--field NAME] [--output PATH]`                                                  | Write a `.memoryfield.zip`                                                        |
| `index [--field NAME]`                                                                   | Build/update the vector index                                                     |
| `new TITLE [--field NAME] [--name PAGE] [--summary S] [--dry-run]`                       | Create a page with generated frontmatter                                          |
| `read [--field NAME] PAGES... [--offset N] [--limit N] [--no-limit] [--no-line-numbers]` | Print pages with line numbers                                                     |
| `search [--field NAME] QUERY [--json]`                                                   | Semantic search over pages (substring fallback)                                   |
| `serve [--port N] [--host H] [--allow-writes] [--open]`                                  | Serve fields over HTTP                                                            |
| `validate [--field NAME]`                                                                | Check a field against the spec (errors exit non-zero)                             |
| `write [--field NAME] PAGE [--force] [--append] [--dry-run] [--title T] [--summary S]`   | Write stdin to a page (auto frontmatter)                                          |

Most commands act on all connected fields unless `--field NAME` limits them;
read/write/new/edit use the single connected field when only one exists and
require `--field` when several do.

## Prefer search over enumeration

A field can hold hundreds of pages on any topic, and enumerating it (with
`catalog`) dumps every page's title and summary into your context window —
usually irrelevant to the task and expensive in tokens. Searching is the
default move: it returns the list of relevant pages directly, ranked, so you
rarely need a listing to find things.

- Start with `search`; `read` only the pages it returns.
- Use `catalog` only when you have a reason to see every page — for example
  the user asks for a listing, or you are validating, exporting, or
  reorganising a field.
- To gauge a field's size without listing it:
  `memoryfield-tool catalog --json | jq 'length'`.

## Workflow: reading

1. `memoryfield-tool search "query"` — semantic search (unless ollama is
   unavailable).  This is the default way to find pages.
2. `memoryfield-tool read page.md` — print a page with line numbers.

## Workflow: writing

1. `memoryfield-tool new "Title" --summary "One sentence"` — create a page;
   the filename is slugified and frontmatter (uuid, created, updated, title)
   is generated.
2. `echo 'body' | memoryfield-tool write page.md --title "Title"` — write a
   page from stdin, auto-filling missing frontmatter.
3. `memoryfield-tool write --force page.md` — overwrite, preserving stored
   uuid/created/updated/title.
4. `memoryfield-tool validate --field NAME` — confirm the field conforms.

## Page format rules

When writing pages, follow the spec (full detail in `references/SPEC.md`):

- Filenames: `^[a-z0-9]+(?:-[a-z0-9]+)*\.md$` — lowercase ASCII letters,
  digits, hyphens; e.g. `carbon-fibre.md`. `write`/`new` enforce this.
- Pages are UTF-8 Markdown, flat at the field root; subdirectories are never
  pages.
- Frontmatter (all optional per spec): `title`, `uuid`, `summary`, `created`,
  `updated`. Datetimes MUST be quoted strings (e.g.
  `created: '2026-03-01T09:00:00Z'`) or YAML 1.1 parsers coerce them to
  datetime objects.
- Pages SHOULD NOT exceed 8192 bytes; split longer pages, giving each new page
  a fresh `uuid` and preserving sources.
- Pages SHOULD include sources and citations.
- `index.md` is an optional introduction, NOT a catalogue of pages; use
  `listing.md` for a catalogue.

## Error handling

- `memoryfield-tool` not found — ask the user how to install it; do not
  proceed.
- `no memoryfields connected (run connect)` — run `connect`/`create` or ask
  the user for a location.
- `multiple memoryfields connected — specify --field` — pass `--field NAME`.
- `uuid conflict on ...` — the body's uuid differs from the stored one; keep
  the stored uuid when replacing a page.
- `file exists: ... (use --force ...)` — pass `--force` to overwrite or
  `--append` to append.
- `not valid UTF-8` / empty body — fix the input before writing.
- `embedding unavailable (is ollama running?)` — search falls back to
  substring matching; check `which ollama`, then run `index` once ollama is
  available.
- `validate` errors (exit non-zero) must be fixed; warnings may be left.

## Spec reference

`references/SPEC.md` is the complete memoryfield format specification
(container rules, frontmatter, vector index schema, transports, HTTP/PUT
semantics, security). Load it when you need full normative detail. If the file
is missing (some installers copy only SKILL.md), fetch the canonical version
from https://github.com/calpaterson/memoryfield-spec/blob/main/SPEC.md

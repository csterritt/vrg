## Issue 47: Read-failure diagnostics single-line embedded filenames

**Type**: AFK
**Blocked by**: None — can start immediately

### Parent PRD

`Notes/PRD-vrg.md` — audit source: `Notes/critiques/final-audit-vrg.md`, Medium finding 12

### What to build

Fix diagnostic construction for file-load failures (`internal/filebuffer/filebuffer.go:111-119`, `internal/app/app.go:1291-1304`). `os.ReadFile` returns a `*PathError` whose message embeds the raw filename, and the app passes `err.Error()` straight to `EscapeDiagnostic`, which preserves newlines as diagnostic boundaries. A filename containing a newline therefore becomes *multiple* diagnostic lines — the escaping sanitizes control bytes but does not preserve the diagnostic's single-line structure. A hostile or unusual filename can split one error into injected-looking extra lines.

- Build file-load diagnostics from a separately `EscapePath`-escaped path plus a sanitized reason that does not repeat the raw path — e.g. unwrap the `PathError` to its `Err` for the reason rather than reusing `err.Error()`.
- One failed read produces exactly one diagnostic line, whatever bytes the filename contains.
- Apply the same construction to every file-load diagnostic site (initial load, reload, retry).

See PRD *Text, graphemes, and safe presentation* (sanitizers) and *File loading, cache, reload, and selection consistency* (read-failure diagnostics).

### How to verify

- **Manual**: create files named with embedded newline, tab, invalid UTF-8, and ESC bytes; arrange a read failure (e.g. permission removal after indexing) → each failure surfaces as exactly one diagnostic line with the path escaped inline, in both the overlay and the exit stderr replay.
- **Automated**: tests driving real load failures for filenames containing newline, tab, invalid UTF-8, and ESC, asserting the produced diagnostic is a single line containing the `EscapePath`-escaped filename and a sanitized reason with no raw path repetition; assert the same holds through the overlay row set and the replay buffer.

### Acceptance criteria

- [ ] Given a filename containing a newline and a failing read, then the diagnostic is exactly one line — never split into multiple diagnostic rows.
- [ ] Given filenames containing tab, invalid UTF-8, or ESC bytes, then the embedded path appears in `EscapePath`-escaped form within the single-line diagnostic.
- [ ] Given the diagnostic text, then the reason portion does not repeat the raw unescaped path (no `PathError.Error()` passthrough).
- [ ] Given read failures from initial load, `r` reload, and retry paths, then all use the same single-line construction.
- [ ] Given the exit replay, then the same single-line diagnostic appears verbatim — replay introduces no re-splitting.

### User stories addressed

- User story 25: original filename bytes retained for opening; invalid bytes visibly escaped
- User story 75: dangerous controls visibly escaped in every output sink
- User story 38: current-file read failure shows an error overlay and "(unreadable)"
- User story 22: collected diagnostics replayed to stderr after terminal restoration

---

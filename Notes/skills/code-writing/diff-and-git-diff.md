---
name: diff-and-git-diff
description: How to use the 'sem diff', etc. commands to diff files and git history. Use whenever diffing two files or examining git history.
---

## Commands

### sem diff
Entity-level diff. Shows which functions/classes were added, modified, deleted, or renamed. Distinguishes cosmetic changes (whitespace/formatting) from structural changes (logic).

```
sem diff                          # working changes
sem diff --staged                 # staged only
sem diff --commit abc1234         # specific commit
sem diff --from HEAD~5 --to HEAD  # commit range
sem diff file1.ts file2.ts       # compare two files (no git needed)
sem diff --format json            # JSON output for agents/CI
sem diff --format plain           # git-status style
sem diff --format markdown        # markdown tables
sem diff --stdin --format json    # read file changes from stdin
sem diff --file-exts .py .rs      # filter by extension
sem diff -v                       # verbose inline content diffs
```

### sem blame
Entity-level blame. Who last modified each function/class, not each line.

```
sem blame src/auth.ts
sem blame src/auth.ts --json
```

### sem graph
Cross-file entity dependency graph. Shows what each function calls and what calls it.

```
sem graph
sem graph --entity validateToken
sem graph --file-exts .py
sem graph --format json
sem graph --no-default-excludes
```

### sem impact
Transitive impact analysis. If this entity changes, what else is affected? BFS through dependency graph.

```
sem impact validateToken
sem impact validateToken --json
sem impact validateToken --file-exts .py
sem impact validateToken --no-default-excludes
```

## JSON Output

```
sem diff --format json
```

Returns:
```json
{
  "summary": { "fileCount": 2, "added": 1, "modified": 1, "deleted": 1, "moved": 0, "renamed": 0, "reordered": 0, "orphan": 0, "total": 3 },
  "changes": [
    {
      "entityId": "src/auth.ts::function::validateToken",
      "changeType": "added",
      "entityType": "function",
      "entityName": "validateToken",
      "startLine": 12,
      "endLine": 18,
      "oldStartLine": null,
      "oldEndLine": null,
      "filePath": "src/auth.ts"
    }
  ]
}
```

The named change-type buckets (added, modified, deleted, moved, renamed, reordered) always sum to total. Orphan is metadata for module-level changes already included in those buckets.

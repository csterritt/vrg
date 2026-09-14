---
name: styling-tui
description: Rules for terminal layout and styling in VRG.
---

## Terminal styling

- Use VRG's existing theme and rendering primitives for terminal presentation. Bubbles and Lip Gloss are intentionally not dependencies; do not add them solely for visual consistency.
- Derive all widths and heights from the current terminal dimensions and the exact layout arithmetic in the PRD. Account for borders, padding, and the global footer explicitly.
- Keep layout calculations separate from content rendering so arithmetic can be tested directly with table-driven tests.
- Centralize repeated styles and semantic states. Do not scatter ANSI escape sequences through model or rendering code.
- Treat strings as terminal cells, not bytes or runes, when measuring, truncating, padding, or scrolling. Preserve valid UTF-8 and avoid splitting visible glyphs.
- Apply truncation only where the PRD permits it. Keep borders exclusive, rows complete, and focused controls visible.
- Do not rely on color alone to communicate focus, errors, warnings, loading, cancellation, or terminal states. Output must remain understandable with limited or disabled color.
- Test required terminal sizes and edge cases, including 80x24, 100x30, 160x50, below-minimum restoration, long content, empty results, and oversized values.

## References

- [Bubble Tea](https://github.com/charmbracelet/bubbletea)

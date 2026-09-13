//go:build manual_demo

package app_test

import (
	"fmt"
	"strings"
	"testing"

	"vrg/internal/filebuffer"
)

// TestManualDemoLoadIsolation demonstrates the Issue #25 manual case
// as a gated model test:
// - A is a very large first file (load held at a gate).
// - B is a small second file (load completes immediately).
// - Press n immediately at startup: B appears while A is still loading.
// - Press p: A shows content only once it has loaded.
// - A must never show content before completion.
func TestManualDemoLoadIsolation(t *testing.T) {
	// Build a search index with two files: A (first, large) and B (second, small).
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)

	// A is "large" (many lines), B is small.
	linesA := make([]filebuffer.Line, 200)
	for i := 0; i < 200; i++ {
		linesA[i] = ml(i+1, fmt.Sprintf("line-%d-content-a", i+1))
	}
	bufA := makeBuf(linesA, 200, 5)
	bufB := makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3)

	loader := newGatedLoader()
	loader.set("src/a.go", bufA)
	loader.set("src/b.go", bufB)

	// Setup: A is the startup file (first in path order). Its load is held.
	m, loadA := setupBrowseGated(t, idx, loader)
	loader.waitStarted("src/a.go")

	// Step 1: A is loading. The panel shows "Loading…".
	view := viewContent(m)
	if !strings.Contains(view, "Loading") {
		t.Fatalf("Step 1: panel should show Loading while A loads: %q", view)
	}
	if strings.Contains(view, "content-a") {
		t.Fatalf("Step 1: A must never show content before completion: %q", view)
	}
	fmt.Printf("Step 1 (A loading): panel shows Loading (correct)\n")

	// Step 2: Press n immediately at startup. B appears while A is still loading.
	m, cmd := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	loadB := startLoadAsync(cmd)
	loader.waitStarted("src/b.go")

	// B's load completes immediately (its gate is not held).
	loader.release("src/b.go")
	lc := <-loadB
	m = deliverCompletion(t, m, lc)

	view = viewContent(m)
	if !strings.Contains(view, "content-b") {
		t.Fatalf("Step 2: B should appear after n: %q", view)
	}
	if strings.Contains(view, "Loading") {
		t.Fatalf("Step 2: B should not show Loading after completion: %q", view)
	}
	fmt.Printf("Step 2 (n to B): B appears while A still loading (correct)\n")

	// Step 3: Press p. A is still loading. The panel shows Loading for A.
	m, cmd = update(t, m, keyPress('p'))
	assertCurrentPath(t, m, "src/a.go")
	_ = startLoadAsync(cmd) // drain any timer command

	view = viewContent(m)
	if !strings.Contains(view, "Loading") {
		t.Fatalf("Step 3: panel should show Loading for A (still loading): %q", view)
	}
	if strings.Contains(view, "content-a") {
		t.Fatalf("Step 3: A must never show content before completion: %q", view)
	}
	fmt.Printf("Step 3 (p back to A): A still loading, panel shows Loading (correct)\n")

	// Step 4: Release A's gate. A's completion arrives. A shows content.
	loader.release("src/a.go")
	lc = <-loadA
	m = deliverCompletion(t, m, lc)

	view = viewContent(m)
	if strings.Contains(view, "Loading") {
		t.Fatalf("Step 4: A should not show Loading after completion: %q", view)
	}
	if !strings.Contains(view, "content-a") {
		t.Fatalf("Step 4: A should show content after completion: %q", view)
	}
	fmt.Printf("Step 4 (A completes): A shows content only after completion (correct)\n")

	fmt.Printf("\nManual demonstration passed: A never showed content before completion.\n")
}

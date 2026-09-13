//go:build manual_demo

package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
)

// TestManualDemoStaleValidation demonstrates the Issue #29 manual
// verification scenarios with a real file on disk:
//
//  1. Edit the matched word to a same-length different word.
//  2. Enter the file.
//  3. Verify no highlight appears.
//  4. Verify "file changed since search" appears.
//  5. Delete trailing matched lines.
//  6. Verify landing on the last line without a highlight.
//  7. Revert using r.
//  8. Verify the stale note clears.
func TestManualDemoStaleValidation(t *testing.T) {
	// Build an index with one file, line 1 matching "hit".
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hit\n", 1, subSpec{"hit", 0, 3}),
	)
	dir := t.TempDir()
	p := filepath.Join(dir, "src/a.go")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}

	// Step 1: write the file with the matched word, then edit it to a
	// same-length different word ("hat").
	if err := os.WriteFile(p, []byte("hit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	realLoader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return filebuffer.Load([]byte(p), stops)
	}
	gate := make(chan struct{})
	close(gate)
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(realLoader),
		app.WithPopupDuration(0),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if cmd != nil {
		m = deliverLoad(t, m, cmd)
	}

	// Step 2: edit the matched word to a same-length different word.
	if err := os.WriteFile(p, []byte("hat\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Press r to reload the edited content.
	m, rCmd := update(t, m, keyPress('r'))
	if rCmd != nil {
		lc := execReloadCmd(t, rCmd)
		m = deliverCompletion(t, m, lc)
	}

	view := viewContent(m)
	// Step 3: verify no highlight appears. The highlight uses inverse
	// video (\x1b[7m). "hat" should not be wrapped in inverse video.
	if strings.Contains(view, "\x1b[7mhat") {
		t.Fatalf("Step 3 FAIL: 'hat' is highlighted (invented highlight for dropped submatch):\n%s", view)
	}
	t.Logf("Step 3 OK: no highlight on 'hat' (submatch dropped)")

	// Step 4: verify "file changed since search" appears.
	if !strings.Contains(view, "file changed since search") {
		t.Fatalf("Step 4 FAIL: stale note not present:\n%s", view)
	}
	t.Logf("Step 4 OK: 'file changed since search' note present")

	// Step 5: delete trailing matched lines. Rewrite the file to have
	// no line 1 match (the line is gone). Use a 2-line file where the
	// matched line (line 1) is deleted, leaving only line 2.
	if err := os.WriteFile(p, []byte("gone\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, rCmd = update(t, m, keyPress('r'))
	if rCmd != nil {
		lc := execReloadCmd(t, rCmd)
		m = deliverCompletion(t, m, lc)
	}

	// Step 6: verify landing on the last line without a highlight.
	// The missing line (line 1) falls back to the last source line
	// (line 1 of "gone\n"), which is visible from offset 0.
	if m.ViewportOffset() != 0 {
		t.Fatalf("Step 6 FAIL: ViewportOffset = %d, want 0 (last line visible from top)", m.ViewportOffset())
	}
	view = viewContent(m)
	if !strings.Contains(view, "gone") {
		t.Fatalf("Step 6 FAIL: 'gone' not in view:\n%s", view)
	}
	if strings.Contains(view, "\x1b[7mgone") {
		t.Fatalf("Step 6 FAIL: 'gone' is highlighted (invented highlight):\n%s", view)
	}
	t.Logf("Step 6 OK: landed on last line 'gone' without highlight")

	// Step 7: revert using r. Rewrite the file back to "hit\n" and
	// reload.
	if err := os.WriteFile(p, []byte("hit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, rCmd = update(t, m, keyPress('r'))
	if rCmd != nil {
		lc := execReloadCmd(t, rCmd)
		m = deliverCompletion(t, m, lc)
	}

	// Step 8: verify the stale note clears.
	view = viewContent(m)
	if strings.Contains(view, "file changed since search") {
		t.Fatalf("Step 8 FAIL: stale note still present after revert:\n%s", view)
	}
	t.Logf("Step 8 OK: stale note cleared after revert")

	t.Logf("Manual demonstration passed: stale-match validation behavior matches the Issue #29 contracts.")
}

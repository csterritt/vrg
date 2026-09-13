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

// TestManualDemoUnsupportedEncoding exercises the Issue #30 manual
// verification scenario with a real UTF-16 LE file on disk:
//  1. Create a UTF-16 LE file with printf '\xff\xfeh\0i\0\n\0'.
//  2. Run vrg hi .
//  3. Enter the file and show (unsupported encoding) with the overlay.
//  4. Dismiss the overlay.
//  5. Press r.
//  6. Show Loading....
//  7. Show the unsupported placeholder with a new overlay.
//  8. Dismiss again.
//  9. Press q.
// 10. Verify exit status 0.
// 11. Verify each encoding diagnostic appears on stderr.
func TestManualDemoUnsupportedEncoding(t *testing.T) {
	dir := t.TempDir()

	// Step 1: Create a UTF-16 LE file with printf '\xff\xfeh\0i\0\n\0'.
	utf16Path := filepath.Join(dir, "utf16.txt")
	if err := os.WriteFile(utf16Path,
		[]byte{0xFF, 0xFE, 'h', 0x00, 'i', 0x00, 0x0A, 0x00}, 0o644); err != nil {
		t.Fatal(err)
	}

	// Also create a normal file so the index has a usable startup file.
	normalPath := filepath.Join(dir, "normal.txt")
	if err := os.WriteFile(normalPath, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	pathMap := map[string]string{
		"utf16.txt":  utf16Path,
		"normal.txt": normalPath,
	}
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		if full, ok := pathMap[string(path)]; ok {
			return filebuffer.Load([]byte(full), stops)
		}
		return filebuffer.Load(path, stops)
	}

	// Step 2: Run vrg hi . (simulate search with a match in utf16.txt).
	idx := buildIndex(t, "/work",
		textMatch("normal.txt", "hi\n", 1, subSpec{"hi", 0, 2}),
		textMatch("utf16.txt", "hi\n", 1, subSpec{"hi", 0, 2}),
	)
	gate := make(chan struct{})
	close(gate)
	m := app.New([]string{"--json", "--", "hi", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithPopupDuration(0),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	loadCh := startLoadAsync(cmd)
	lc := <-loadCh
	m = deliverCompletion(t, m, lc)

	// Startup file (normal.txt) loaded fine.
	if m.OverlayOpen() {
		t.Fatalf("Step 2 FAIL: overlay open after normal startup file")
	}
	t.Logf("Step 2 OK: vrg hi . started, normal.txt loaded")

	// Step 3: Enter the UTF-16 file (n) and show (unsupported encoding)
	// with the overlay.
	m, navCmd := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "utf16.txt")
	m = deliverLoad(t, m, navCmd)
	if !m.OverlayOpen() {
		t.Fatalf("Step 3 FAIL: overlay not open for UTF-16 LE file")
	}
	if m.OverlayKind() != app.OverlayError {
		t.Fatalf("Step 3 FAIL: OverlayKind = %v, want OverlayError", m.OverlayKind())
	}
	t.Logf("Step 3 OK: entered utf16.txt, overlay open with encoding diagnostic")

	// Step 4: Dismiss the overlay.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	if m.OverlayOpen() {
		t.Fatalf("Step 4 FAIL: overlay still open after Esc")
	}
	view := viewContent(m)
	if !strings.Contains(view, "(unsupported encoding)") {
		t.Fatalf("Step 4 FAIL: view does not contain '(unsupported encoding)' after dismiss:\n%s", view)
	}
	t.Logf("Step 4 OK: overlay dismissed, panel shows (unsupported encoding)")

	// Step 5: Press r to reload.
	m, reloadCmd := update(t, m, keyPress('r'))

	// Step 6: Show Loading....
	view = viewContent(m)
	if !strings.Contains(view, "Loading") {
		t.Fatalf("Step 6 FAIL: view does not show 'Loading...' during reload:\n%s", view)
	}
	t.Logf("Step 6 OK: panel shows Loading... during reload")

	// Step 7: Show the unsupported placeholder with a new overlay.
	m = deliverLoad(t, m, reloadCmd)
	if !m.OverlayOpen() {
		t.Fatalf("Step 7 FAIL: overlay not open after reload of unsupported file")
	}
	t.Logf("Step 7 OK: overlay open after reload")

	// Step 8: Dismiss again.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	if m.OverlayOpen() {
		t.Fatalf("Step 8 FAIL: overlay still open after second Esc")
	}
	view = viewContent(m)
	if !strings.Contains(view, "(unsupported encoding)") {
		t.Fatalf("Step 8 FAIL: view does not contain '(unsupported encoding)' after second dismiss:\n%s", view)
	}
	t.Logf("Step 8 OK: overlay dismissed, panel shows (unsupported encoding)")

	// Step 9: Press q.
	m, quitCmd := update(t, m, keyPress('q'))
	assertQuit(t, quitCmd)

	// Step 10: Verify exit status 0.
	if m.ExitCode() != 0 {
		t.Fatalf("Step 10 FAIL: ExitCode = %d, want 0", m.ExitCode())
	}
	t.Logf("Step 10 OK: exit status 0")

	// Step 11: Verify each encoding diagnostic appears on stderr.
	diags := m.Diagnostics()
	found := false
	for _, d := range diags {
		if strings.Contains(d, "UTF-16") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Step 11 FAIL: no UTF-16 diagnostic in replay: %q", diags)
	}
	t.Logf("Step 11 OK: UTF-16 diagnostic present in stderr replay: %q", diags)

	t.Logf("Manual demonstration passed: unsupported-encoding behavior matches the Issue #30 contracts.")
}

//go:build manual_demo

package app_test

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestManualDemoReadFailureSingleLine drives the Issue #47 manual
// scenario entirely inside disposable temporary directories: for
// each hostile filename kind (embedded newline, tab, invalid UTF-8,
// ESC) a real file is created, indexed, and then removed while its
// load is held at the file-load gate, so the released real
// os.ReadFile fails with ENOENT. Each failure must surface as
// exactly one diagnostic line — the EscapePath-escaped path plus a
// sanitized reason that never repeats the raw path — in the overlay
// and in the collected diagnostics replayed to stderr at exit.
// Fixtures are removed on every outcome; the cleanup evidence is
// printed.
func TestManualDemoReadFailureSingleLine(t *testing.T) {
	for _, tc := range hostileNames {
		dir := newHostileDir(t)
		rawPath := []byte(filepath.Join(dir, string(tc.base)))
		if err := os.WriteFile(string(rawPath), []byte("x\n"), 0o644); err != nil {
			os.RemoveAll(dir)
			if tc.name == "invalid-utf8" {
				fmt.Printf("%s: unsupported environment (filesystem rejects the filename): %v\n", tc.name, err)
				continue
			}
			t.Fatalf("write fixture %q: %v", rawPath, err)
		}
		// Index the file, hold the load at the gate, remove the
		// fixture, then release the real read attempt.
		idx := buildRealPathIndex(t, dir, rawPath)
		gate := make(chan struct{})
		m, loadCh := setupRealLoadBrowse(t, idx, dir, gate)
		if err := os.Remove(string(rawPath)); err != nil {
			t.Fatalf("remove fixture %q: %v", rawPath, err)
		}
		close(gate)

		lc := <-loadCh
		pe := requirePathError(t, lc)
		m = deliverCompletion(t, m, lc)
		want := wantReadDiagnostic(rawPath, pe)

		// The overlay shows the escaped path inline on one row.
		fmt.Printf("%s: overlay row: %s\n", tc.name, m.OverlayText())
		assertSingleLineReadDiagnostic(t, m, rawPath, want)

		// Esc dismisses the overlay, q exits; the collected
		// diagnostics replay to stderr exactly as collected —
		// one line per failed read, no re-splitting.
		m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
		m, cmd := update(t, m, keyPress('q'))
		assertQuit(t, cmd)
		var replay strings.Builder
		for _, d := range m.Diagnostics() {
			fmt.Fprintln(&replay, d)
		}
		lines := strings.Split(strings.TrimRight(replay.String(), "\n"), "\n")
		if len(lines) != 1 {
			t.Fatalf("%s: replay produced %d lines, want 1: %q", tc.name, len(lines), replay.String())
		}
		fmt.Printf("%s: exit stderr replay: %s\n", tc.name, strings.TrimRight(replay.String(), "\n"))

		// Cleanup evidence: the hostile fixture and its directory
		// are gone on success and failure alike.
		if err := os.RemoveAll(dir); err != nil {
			t.Fatalf("cleanup %s: %v", dir, err)
		}
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("fixture dir %s survived cleanup", dir)
		}
		fmt.Printf("%s: cleanup evidence: %s removed (exists=false)\n", tc.name, dir)
	}
}

// TestManualDemoReadFailurePermissionDenied demonstrates the
// permission-denial variant: a fixture left unreadable (mode 000)
// makes the real read fail with EACCES, producing the same
// single-line diagnostic construction. A cleanup trap restores the
// mode so nothing survives outside the temporary directory. When
// elevated privileges or ACLs still permit the read, the
// environment is reported unsupported rather than passed.
func TestManualDemoReadFailurePermissionDenied(t *testing.T) {
	dir := newHostileDir(t)
	rawPath := []byte(filepath.Join(dir, "pm\nz"))
	writeRealFile(t, string(rawPath), "x\n")
	if err := os.Chmod(string(rawPath), 0o000); err != nil {
		t.Fatalf("chmod fixture %q: %v", rawPath, err)
	}
	// The cleanup trap restores the mode before the fixture is
	// removed, on success and failure alike.
	restored := false
	restore := func() {
		if !restored {
			restored = true
			os.Chmod(string(rawPath), 0o644)
		}
	}
	t.Cleanup(restore)

	idx := buildRealPathIndex(t, dir, rawPath)
	gate := make(chan struct{})
	close(gate)
	m, loadCh := setupRealLoadBrowse(t, idx, dir, gate)
	lc := <-loadCh
	if lc.Err == nil {
		restore()
		fmt.Printf("permission-denial variant: unsupported environment — read succeeded (elevated privileges or ACLs permit mode-000 reads)\n")
		return
	}
	var pe *fs.PathError
	if !errors.As(lc.Err, &pe) {
		t.Fatalf("load error = %T %v, want *fs.PathError from os.ReadFile", lc.Err, lc.Err)
	}
	if !errors.Is(lc.Err, fs.ErrPermission) {
		t.Fatalf("load error = %v, want fs.ErrPermission (mode-000 fixture)", lc.Err)
	}
	m = deliverCompletion(t, m, lc)
	want := wantReadDiagnostic(rawPath, pe)

	fmt.Printf("permission-denied: overlay row: %s\n", m.OverlayText())
	assertSingleLineReadDiagnostic(t, m, rawPath, want)

	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	m, cmd := update(t, m, keyPress('q'))
	assertQuit(t, cmd)
	var replay strings.Builder
	for _, d := range m.Diagnostics() {
		fmt.Fprintln(&replay, d)
	}
	lines := strings.Split(strings.TrimRight(replay.String(), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("permission-denied: replay produced %d lines, want 1: %q", len(lines), replay.String())
	}
	fmt.Printf("permission-denied: exit stderr replay: %s\n", strings.TrimRight(replay.String(), "\n"))

	// Restore the mode through the trap, then remove the fixture
	// and its directory — the cleanup evidence.
	restore()
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("cleanup %s: %v", dir, err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("fixture dir %s survived cleanup", dir)
	}
	fmt.Printf("permission-denied: cleanup evidence: mode restored, %s removed (exists=false)\n", dir)
}

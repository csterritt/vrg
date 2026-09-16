package app_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Issue #39: static guard for the shared grapheme/cell helper ---
//
// All display geometry must go through the shared ANSI-aware
// grapheme/cell helper. To keep it the single decoding point, the
// mechanical predicate is: across every non-test production .go file
// under internal/ and cmd/, the symbol utf8.DecodeRuneInString may
// appear only in internal/safepresentation/cellwidth.go. Any other
// occurrence — including in files and packages added later — fails
// this test.

// repoRoot walks up from dir to the repository root containing go.mod.
func repoRoot(t *testing.T, dir string) string {
	t.Helper()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate repository root (go.mod)")
		}
		dir = parent
	}
}

// TestDecodeRuneInStringOnlyInSharedCellHelper scans every non-test
// production .go file under internal/ and cmd/ and fails when
// utf8.DecodeRuneInString appears outside the allow-listed shared
// helper file (Issue #39).
func TestDecodeRuneInStringOnlyInSharedCellHelper(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	root := repoRoot(t, cwd)
	allowed := filepath.Join("internal", "safepresentation", "cellwidth.go")
	if _, err := os.Stat(filepath.Join(root, allowed)); err != nil {
		t.Fatalf("allow-listed shared helper %s is missing: %v", allowed, err)
	}
	var violations []string
	for _, top := range []string{"internal", "cmd"} {
		base := filepath.Join(root, top)
		err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if rel, _ := filepath.Rel(root, path); strings.Contains(string(data), "utf8.DecodeRuneInString") && rel != allowed {
				violations = append(violations, rel)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", top, err)
		}
	}
	if len(violations) > 0 {
		t.Fatalf("utf8.DecodeRuneInString found outside %s (display geometry must use the shared grapheme/cell helper): %v", allowed, violations)
	}
}

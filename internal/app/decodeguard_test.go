package app

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cellWidthHelper is the sole production file allowed to call
// utf8.DecodeRuneInString: the shared ANSI-aware grapheme/cell helper's
// implementation. Every other display-geometry consumer measures and
// truncates through that helper, and unrelated decoding uses for range
// or another standard-library primitive — so no width or truncation
// loop anywhere else can revert to one rune equals one cell.
const cellWidthHelper = "internal/safepresentation/cellwidth.go"

// The mechanical guard behind Issue 39's shared-cell-policy contract:
// scanning every non-test production .go file under internal/ and cmd/,
// utf8.DecodeRuneInString may appear only inside cellWidthHelper — an
// occurrence in any other file, including a newly added package, fails.
// Test files are excluded: test-only decoder utilities are free to
// decode however they like.
func TestDecodeRuneInStringAllowList(t *testing.T) {
	root := filepath.Join("..", "..")
	var offenders []string
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir),
			func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() || !strings.HasSuffix(path, ".go") ||
					strings.HasSuffix(path, "_test.go") {
					return nil
				}
				rel, err := filepath.Rel(root, path)
				if err != nil {
					return err
				}
				if filepath.ToSlash(rel) == cellWidthHelper {
					return nil
				}
				b, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				if strings.Contains(string(b), "utf8.DecodeRuneInString") {
					offenders = append(offenders, filepath.ToSlash(rel))
				}
				return nil
			})
		if err != nil {
			t.Fatalf("scanning %s: %v", dir, err)
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("utf8.DecodeRuneInString outside %s: %v", cellWidthHelper, offenders)
	}
}

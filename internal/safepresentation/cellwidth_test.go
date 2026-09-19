package safepresentation

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Issue #39: one shared ANSI-aware grapheme/cell helper — every
// display-width and truncation consumer measures through CellWidth,
// which applies the uniseg grapheme-cluster policy and ignores ANSI
// escape sequences the theme wraps around content.

// CellWidth measures terminal cells by grapheme cluster — never
// bytes, never runes: a two-cell CJK glyph, a base-plus-combining
// cluster, and an emoji ZWJ sequence each measure exactly their
// painted cells.
func TestCellWidthGraphemePolicy(t *testing.T) {
	for _, c := range []struct {
		name string
		in   string
		want int
	}{
		{"empty", "", 0},
		{"ascii", "abc", 3},
		{"cjk wide glyph", "文", 2},
		{"cjk inside text", "ab文cd", 6},
		{"combining cluster", "é", 1},
		{"wide combining cluster", "文́", 2},
		{"emoji zwj sequence", "👨‍👩‍👧", 2},
	} {
		if got := CellWidth(c.in); got != c.want {
			t.Errorf("%s: CellWidth(%q) = %d, want %d", c.name, c.in, got, c.want)
		}
	}
}

// CellWidth is ANSI-aware: the SGR runs the theme wraps around
// content occupy no cells, so a styled string measures exactly its
// content. Consumers may therefore measure either the unstyled text
// or the emitted styled row and get the same cell count.
func TestCellWidthIgnoresANSI(t *testing.T) {
	for _, c := range []struct {
		name string
		in   string
		want int
	}{
		{"sgr wrapped ascii", "\x1b[37;40mhello\x1b[0m", 5},
		{"inverse run inside text", "ab\x1b[30;47mc\x1b[37;40;24md", 4},
		{"styled wide glyph", "\x1b[30;47;4m文\x1b[37;40;24m", 2},
		{"styled combining cluster", "\x1b[30;47mé\x1b[0m", 1},
		{"styled zwj sequence", "\x1b[30;47m👨‍👩‍👧\x1b[0m", 2},
		{"reset only", "\x1b[0m", 0},
	} {
		if got := CellWidth(c.in); got != c.want {
			t.Errorf("%s: CellWidth(%q) = %d, want %d", c.name, c.in, got, c.want)
		}
	}
}

// The Issue #39 mechanical predicate: across every non-test
// production .go file under internal/ and cmd/, the symbol
// utf8.DecodeRuneInString appears only in
// internal/safepresentation/cellwidth.go — the shared grapheme/cell
// helper. Any other occurrence fails, including one in a newly added
// package or file, so no display-geometry code can route around the
// shared policy with its own rune-decoding width or truncation loop.
// Test-only decoder utilities are excluded from the scan.
func TestDecodeRuneInStringAllowList(t *testing.T) {
	const symbol = "utf8.DecodeRuneInString"
	allowed := map[string]bool{
		"internal/safepresentation/cellwidth.go": true,
	}
	for _, root := range []string{"../../internal", "../../cmd"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") ||
				strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel := filepath.ToSlash(strings.TrimPrefix(path, "../../"))
			if strings.Contains(string(data), symbol) && !allowed[rel] {
				t.Errorf("%s contains %s — rune decoding belongs only in internal/safepresentation/cellwidth.go", rel, symbol)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("scanning %s: %v", root, err)
		}
	}
}

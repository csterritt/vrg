package app

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// The shared grapheme/cell helper is the single authority for display
// geometry, and the policy is enforced mechanically: across every
// non-test production .go file under internal/ and cmd/, the symbol
// utf8.DecodeRuneInString may occur only in
// internal/present/cellwidth.go — the helper's implementation file.
// An occurrence anywhere else fails, including one in a newly added
// package or file, so no display-geometry consumer can re-implement a
// rune-decoding width or truncation loop that evades the shared
// policy. Test-only decoder utilities are excluded from the scan.
func TestDecodeRuneInStringOnlyInCellHelper(t *testing.T) {
	const allow = "internal/present/cellwidth.go"
	root := filepath.Join("..", "..")
	fset := token.NewFileSet()
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir),
			func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() || !strings.HasSuffix(d.Name(), ".go") ||
					strings.HasSuffix(d.Name(), "_test.go") {
					return nil
				}
				f, err := parser.ParseFile(fset, p, nil, 0)
				if err != nil {
					return err
				}
				found := false
				ast.Inspect(f, func(n ast.Node) bool {
					sel, ok := n.(*ast.SelectorExpr)
					if !ok || sel.Sel.Name != "DecodeRuneInString" {
						return true
					}
					if id, ok := sel.X.(*ast.Ident); ok && id.Name == "utf8" {
						found = true
					}
					return true
				})
				if found {
					rel, err := filepath.Rel(root, p)
					if err != nil {
						return err
					}
					if filepath.ToSlash(rel) != allow {
						t.Errorf("%s calls utf8.DecodeRuneInString — "+
							"permitted only in %s", rel, allow)
					}
				}
				return nil
			})
		if err != nil {
			t.Fatalf("scanning %s: %v", dir, err)
		}
	}
}

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readREADME returns the repository-root README.md — the single
// user-facing documentation artifact Issue #34 keeps synchronized with
// the CLI's shared option declarations.
func readREADME(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatalf("README.md: %v", err)
	}
	return string(b)
}

// The README's flag and option documentation cannot drift from
// parsing: iterating the shared optionDecls must find every declared
// spelling — the allow-listed search flags and the local help options
// alike — as its exact no-argument spelling, plus its description and
// generated option line, in the README.
func TestREADMEDocumentsEveryDeclaredOption(t *testing.T) {
	readme := readREADME(t)
	for i := range optionDecls {
		d := &optionDecls[i]
		var names string
		var spellings []string
		if d.short != 0 {
			names = "-" + string(d.short)
			spellings = append(spellings, names)
		}
		if d.long != "" {
			if names != "" {
				names += ", "
			}
			names += "--" + d.long
			spellings = append(spellings, "--"+d.long)
		}
		if line := "  " + names + "\t" + d.desc; !strings.Contains(readme, line) {
			t.Errorf("README lacks the generated option line %q", line)
		}
		for _, s := range spellings {
			if !strings.Contains(readme, s) {
				t.Errorf("README lacks option spelling %q", s)
			}
		}
	}
}

// The README carries the complete generated command-line help
// verbatim — the same text Parse writes for bare vrg and -h/--help —
// so the usage, argument, and option documentation is the shared
// declarations' own output rather than a copy that can drift.
func TestREADMECarriesGeneratedHelp(t *testing.T) {
	readme := readREADME(t)
	if help := HelpText(); !strings.Contains(readme, help) {
		t.Fatalf("README lacks the generated help block:\n%s", help)
	}
}

// The README documents the invocation contract the preflight enforces:
// the synopsis, options anywhere before --, the flags-only
// missing-pattern usage error, and the lexical rejection of
// =-assignment spellings for the no-argument options.
func TestREADMEInvocationContract(t *testing.T) {
	readme := readREADME(t)
	for _, token := range []string{
		"vrg [flags] pattern [root]",
		"--",
		"--ignore-case=false",
		"usage error",
	} {
		if !strings.Contains(readme, token) {
			t.Errorf("README invocation documentation lacks %q", token)
		}
	}
}

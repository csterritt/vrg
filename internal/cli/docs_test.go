package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// readREADME returns the repository's committed user-facing README —
// the single documentation artifact Issue 34 keeps synchronized with
// the implementation.
func readREADME(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	return string(b)
}

// The README's option documentation is asserted against the shared
// option declarations — the same declarations that configure the
// parser and generate help — so it cannot drift: every declared
// spelling, including the local help options, and every declared
// description appears in the exact no-argument spellings.
func TestReadmeDocumentsDeclaredOptions(t *testing.T) {
	readme := readREADME(t)
	for _, d := range optionDecls {
		if d.short != 0 {
			if want := "`-" + string(d.short) + "`"; !strings.Contains(readme, want) {
				t.Errorf("README lacks option spelling %s", want)
			}
		}
		if d.long != "" {
			if want := "`--" + d.long + "`"; !strings.Contains(readme, want) {
				t.Errorf("README lacks option spelling %s", want)
			}
		}
		if !strings.Contains(readme, d.desc) {
			t.Errorf("README lacks option description %q", d.desc)
		}
	}
}

// The README documents the option contract's semantics: options
// anywhere before the -- terminator, combined short expansion, the
// cumulative unrestricted limit, ordered forwarding, and the
// no-argument contract's rejection of every =-assignment spelling.
func TestReadmeFlagSemantics(t *testing.T) {
	readme := readREADME(t)
	for _, tok := range []string{
		"`--`",             // the option terminator
		"`-iw`", "`-i -w`", // combined shorts expand in order
		"at most twice",         // cumulative unrestricted limit
		"encounter order",       // ordered forwarding
		"`--ignore-case=false`", // a rejected search-flag assignment
		"`-h=false`",            // a rejected help-option assignment
		"usage error",
	} {
		if !strings.Contains(readme, tok) {
			t.Errorf("README lacks %q for the option contract", tok)
		}
	}
}

// The README's invocation documentation agrees with the parser: bare
// vrg and -h/--help are the documented help-only paths — command-line
// help on stdout, exit 0, no search and no TUI — and the documented
// flags-only invocation vrg -i is the missing-pattern usage error.
func TestReadmeInvocationDocsAgreeWithParser(t *testing.T) {
	readme := readREADME(t)
	for _, args := range [][]string{nil, {"-h"}, {"--help"}} {
		var out bytes.Buffer
		res := Parse(args, &out, Env{})
		if res.Kind != KindHelp || !strings.HasPrefix(out.String(), "Usage:") {
			t.Fatalf("Parse(%q) = kind %v with output %q, want the documented help-only result",
				args, res.Kind, out.String())
		}
	}
	res := Parse([]string{"-i"}, &bytes.Buffer{}, Env{})
	if res.Kind != KindUsageError || res.ErrorKind != ErrMissingPattern {
		t.Fatalf("Parse(-i) = kind %v err %v, want the documented missing-pattern usage error",
			res.Kind, res.ErrorKind)
	}
	for _, tok := range []string{
		"no arguments", "`-h`", "`--help`", "stdout", "exit 0",
		"`vrg -i`", "exit 2",
	} {
		if !strings.Contains(readme, tok) {
			t.Errorf("README lacks %q for the invocation contract", tok)
		}
	}
}

// The README identifies ripgrep 15.x as the reference family and states
// that VRG supplies --no-config so ripgrep configuration files are
// never honoured — and the child argv really carries it.
func TestReadmeRipgrepReference(t *testing.T) {
	readme := readREADME(t)
	for _, tok := range []string{"ripgrep 15.x", "`--no-config`", "configuration files", "never honoured"} {
		if !strings.Contains(readme, tok) {
			t.Errorf("README lacks %q for the ripgrep reference", tok)
		}
	}
	res := Parse([]string{"p"}, &bytes.Buffer{}, Env{})
	if res.Kind != KindSearch {
		t.Fatalf("Parse(p) = kind %v, want KindSearch", res.Kind)
	}
	if !slices.Contains(res.Args, "--no-config") || !slices.Contains(res.Args, "--json") {
		t.Fatalf("child argv %q lacks the documented --json --no-config", res.Args)
	}
}

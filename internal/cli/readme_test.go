package cli

import (
	"os"
	"strings"
	"testing"
)

// readmeText reads the repository README — the single user-facing
// documentation artifact (Issue #34).
func readmeText(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatalf("README.md: %v", err)
	}
	return string(b)
}

// Every declared option — the local help option and each allow-listed
// search flag — appears in the README's flag table as a row in its
// exact no-argument spellings, asserted against the shared option
// declarations so the documentation cannot drift from parsing.
func TestReadmeFlagTable(t *testing.T) {
	readme := readmeText(t)
	for _, d := range optionDecls {
		var names []string
		if d.short != 0 {
			names = append(names, "-"+string(d.short))
		}
		if d.long != "" {
			names = append(names, "--"+d.long)
		}
		row := "| " + strings.Join(names, ", ") + " | " + d.desc + " |"
		if !strings.Contains(readme, row) {
			t.Fatalf("README flag table missing row %q", row)
		}
	}
}

// The README documents the help-only exit-0 path — bare vrg and
// -h/--help printing command-line help on stdout with exit 0, no
// search, no TUI — as distinct from the TUI's h/? help dialog, and
// the flags-only usage error (vrg -i with no pattern → exit 2).
func TestReadmeHelpOnlyPath(t *testing.T) {
	readme := readmeText(t)
	for _, tok := range []string{
		"no arguments", "-h", "--help", "stdout", "exit 0",
		"h / ?", "vrg -i", "exit 2",
	} {
		if !strings.Contains(readme, tok) {
			t.Fatalf("README missing help-path token %q", tok)
		}
	}
}

// The README identifies ripgrep 15.x as the reference family and
// states that VRG supplies --no-config so ripgrep configuration files
// are never honoured.
func TestReadmeRipgrepFamily(t *testing.T) {
	readme := readmeText(t)
	for _, tok := range []string{"15.x", "--no-config", "configuration"} {
		if !strings.Contains(readme, tok) {
			t.Fatalf("README missing ripgrep token %q", tok)
		}
	}
}

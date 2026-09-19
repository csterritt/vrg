package docs

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

// The README's scale-and-memory-limits section is generated text: the
// committed file must carry the shared source's rendering verbatim, so
// the section cannot drift from the source the help footer renders
// from.
func TestReadmeLimitsSection(t *testing.T) {
	readme := readmeText(t)
	if !strings.Contains(readme, Limits()) {
		t.Fatalf("README limits section out of date; want it to contain:\n%s", Limits())
	}
}

// The help-overlay footer renders the same statements the README
// carries: every required statement and scale item appears verbatim,
// so deleting one from the shared source fails the suite.
func TestFooterCarriesEveryStatement(t *testing.T) {
	footer := Footer()
	for _, stmt := range Statements {
		if !strings.Contains(footer, stmt) {
			t.Fatalf("help footer missing statement %q:\n%s", stmt, footer)
		}
	}
	for _, item := range ScaleItems {
		if !strings.Contains(footer, item) {
			t.Fatalf("help footer missing scale item %q:\n%s", item, footer)
		}
	}
}

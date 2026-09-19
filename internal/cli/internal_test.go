package cli

import (
	"flag"
	"strings"
	"testing"
)

// The library application must run with flag.ContinueOnError so it never
// calls os.Exit: every exit status belongs to cmd/vrg.
func TestAppUsesContinueOnError(t *testing.T) {
	app := newApp()
	if app.ErrorHandling != flag.ContinueOnError {
		t.Fatalf("app.ErrorHandling = %v, want flag.ContinueOnError", app.ErrorHandling)
	}
}

// Generated help is driven by the shared option declarations: every
// declared option appears in every spelling it declares.
func TestHelpRendersEveryDeclaredOption(t *testing.T) {
	help := renderHelp()
	for _, d := range optionDecls {
		if d.short != 0 && !strings.Contains(help, "-"+string(d.short)) {
			t.Errorf("generated help missing short spelling -%c:\n%s", d.short, help)
		}
		if d.long != "" && !strings.Contains(help, "--"+d.long) {
			t.Errorf("generated help missing long spelling --%s:\n%s", d.long, help)
		}
	}
}

// The shared declarations drive the raw-token scan: every declared
// search-flag spelling is accepted and recorded verbatim; the help
// declaration is a help request, never a forwarded flag.
func TestScanRecordsEveryDeclaredFlag(t *testing.T) {
	for _, d := range optionDecls {
		if d.help {
			continue
		}
		if d.short != 0 {
			spelling := "-" + string(d.short)
			p := scanArgs([]string{spelling, "x"})
			if p.badOption != "" || len(p.flags) != 1 || p.flags[0] != spelling {
				t.Fatalf("scanArgs(%q) = badOption %q flags %q, want recorded %q", spelling, p.badOption, p.flags, spelling)
			}
		}
		if d.long != "" {
			spelling := "--" + d.long
			p := scanArgs([]string{spelling, "x"})
			if p.badOption != "" || len(p.flags) != 1 || p.flags[0] != spelling {
				t.Fatalf("scanArgs(%q) = badOption %q flags %q, want recorded %q", spelling, p.badOption, p.flags, spelling)
			}
		}
	}
	for _, spelling := range []string{"-h", "--help"} {
		if p := scanArgs([]string{spelling, "x"}); !p.help {
			t.Fatalf("scanArgs(%q) did not classify the declared help option as a help request", spelling)
		}
	}
}

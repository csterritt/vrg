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

// optionDecls is the single declaration source for generated help,
// raw-token scanning, and parser configuration. Iterating it must find
// every declared option rendered in generated help under both of its
// spellings, and the ordered scan must accept every forwarded spelling
// verbatim — there is no separately maintained allow-list to drift.
func TestSharedDeclarationsDriveHelpAndScan(t *testing.T) {
	help := renderHelp()
	for i := range optionDecls {
		d := &optionDecls[i]
		var names string
		if d.short != 0 {
			names = "-" + string(d.short)
		}
		if d.long != "" {
			if names != "" {
				names += ", "
			}
			names += "--" + d.long
		}
		if !strings.Contains(help, "  "+names+"\t") {
			t.Errorf("generated help lacks the option line for %q", names)
		}
		if !d.forward {
			continue
		}
		var spellings []string
		if d.short != 0 {
			spellings = append(spellings, "-"+string(d.short))
		}
		if d.long != "" {
			spellings = append(spellings, "--"+d.long)
		}
		for _, spelling := range spellings {
			p := scanArgs([]string{spelling, "pat"})
			if p.help || p.errTok != "" || len(p.flags) != 1 || p.flags[0] != spelling {
				t.Errorf("scanArgs(%q) = %+v, want the spelling recorded verbatim", []string{spelling, "pat"}, p)
			}
			if got := p.positionals; len(got) != 1 || got[0] != "pat" {
				t.Errorf("scanArgs(%q).positionals = %q, want [pat]", []string{spelling, "pat"}, got)
			}
		}
	}
}

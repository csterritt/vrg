package cli_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"

	"vrg/internal/cli"
)

// failStat is a sentinel proving root validation never runs on paths that
// must not reach it: help-only results and non-root usage errors.
func failStat(t *testing.T) func(string) (fs.FileInfo, error) {
	t.Helper()
	return func(name string) (fs.FileInfo, error) {
		t.Fatalf("root validation invoked on %q; this path must not validate a root", name)
		return nil, nil
	}
}

// recordStat wraps os.Stat and records the roots it was asked to validate.
func recordStat(calls *[]string) func(string) (fs.FileInfo, error) {
	return func(name string) (fs.FileInfo, error) {
		*calls = append(*calls, name)
		return os.Stat(name)
	}
}

func parse(t *testing.T, args []string, stat func(string) (fs.FileInfo, error)) (cli.Result, string) {
	t.Helper()
	var out bytes.Buffer
	res := cli.Parse(args, &out, cli.Env{Stat: stat})
	return res, out.String()
}

// assertOneHelpCopy requires out to carry exactly one generated help
// document and nothing else: the Usage line appears once at the start and
// the stub marker never appears.
func assertOneHelpCopy(t *testing.T, out string) {
	t.Helper()
	if !strings.HasPrefix(out, "Usage:") {
		t.Fatalf("help output does not start with a Usage line: %q", out)
	}
	if n := strings.Count(out, "Usage:"); n != 1 {
		t.Fatalf("help output contains %d Usage lines, want exactly one", n)
	}
	if strings.Contains(out, "search stub:") {
		t.Fatalf("help output contains success-stub text: %q", out)
	}
	if strings.ContainsRune(out, '\x1b') || strings.ContainsRune(out, '\x9b') {
		t.Fatalf("help output contains raw control bytes: %q", out)
	}
}

// Every help-only row: help wins over missing-pattern, excess-operand,
// unsupported-option, and invalid-root errors, over option-looking tokens,
// and over operand positions.
func TestHelpOnlyResults(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"bare invocation", nil},
		{"empty argv", []string{}},
		{"first-token short help", []string{"-h"}},
		{"first-token long help", []string{"--help"}},
		{"help after pattern", []string{"foo", "--help"}},
		{"help after pattern short", []string{"foo", "-h"}},
		{"help before would-be invalid root", []string{"foo", "--help", "/nonexistent"}},
		{"help after would-be invalid root", []string{"foo", "/nonexistent", "--help"}},
		{"help after excess operands", []string{"foo", "bar", "baz", "--help"}},
		{"help after unsupported option", []string{"--unsupported", "--help"}},
		{"help between unsupported options", []string{"--bogus", "-h", "--bogus2"}},
		{"flag-preceded help without pattern", []string{"-i", "--help"}},
		{"search flags then help", []string{"-i", "-w", "--help"}},
		{"search flags, pattern, then help", []string{"-i", "foo", "-h"}},
		{"help after a root", []string{"foo", "someroot", "--help"}},
		{"combined short help", []string{"-ih"}},
		{"combined short help first", []string{"-hi"}},
		{"combined short help with pattern", []string{"-ih", "foo"}},
		{"combined short help with unknowns", []string{"-xh"}},
		{"help beats unsupported option", []string{"-e", "--help"}},
		{"help beats cumulative unrestricted limit", []string{"-uuu", "--help"}},
		{"help beats assignment rejection", []string{"--help=false", "--help"}},
		{"help token joined to letters", []string{"-help"}},
		{"help before option terminator", []string{"-h", "--"}},
		{"help before terminator and operand", []string{"--help", "--", "x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, out := parse(t, tc.args, failStat(t))
			if res.Kind != cli.KindHelp {
				t.Fatalf("Parse(%q).Kind = %v, want KindHelp", tc.args, res.Kind)
			}
			if res.Pattern != "" || res.Root != "" || res.ChildArgs != nil {
				t.Fatalf("help-only result carries search fields: %+v", res)
			}
			assertOneHelpCopy(t, out)
		})
	}
}

// Generated help documents the invocation syntax, the required pattern,
// the optional root and its default, and the local help options. The text
// is rendered from the same declarations that configure the parser.
func TestGeneratedHelpContent(t *testing.T) {
	_, out := parse(t, []string{"--help"}, failStat(t))
	for _, want := range []string{
		"Usage: vrg",
		"PATTERN",
		"ROOT",
		`(default ".")`,
		"-h",
		"--help",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("generated help missing %q:\n%s", want, out)
		}
	}
	// The syntax line shows the required pattern and optional root.
	if !strings.Contains(out, "PATTERN [ROOT]") {
		t.Errorf("generated help syntax line does not show PATTERN [ROOT]:\n%s", out)
	}
}

// Every allow-listed no-argument flag is a contract without values: the
// scan rejects = assignment spellings lexically, for search flags and the
// help option alike, before the library's permissive boolean parsing can
// see them. None are help requests, none reach root validation, and none
// produce a child argv.
func TestOptionAssignmentFormsRejected(t *testing.T) {
	for _, arg := range []string{
		"--ignore-case=false", "-i=false",
		"--ignore-case=true", "-i=true", "-i=1",
		"--unrestricted=false", "--unrestricted=true", "-u=0",
		"--hidden=true", "--fixed-strings=off", "-S=",
		"--help=false", "-h=false", "--help=true", "-h=true",
		"--help=", "-h=", "--ignore-case=", "--help=maybe",
	} {
		res, out := parse(t, []string{"foo", arg}, failStat(t))
		if res.Kind != cli.KindUsageError || res.ErrorKind != cli.ErrUnsupportedOption {
			t.Fatalf("Parse(foo %q) = kind %v err %v, want unsupported-option usage error", arg, res.Kind, res.ErrorKind)
		}
		if res.ChildArgs != nil {
			t.Fatalf("Parse(foo %q) produced a child argv on a rejected spelling: %q", arg, res.ChildArgs)
		}
		if out != "" {
			t.Fatalf("Parse(foo %q) emitted help text for a rejected spelling: %q", arg, out)
		}
		assertSanitizedLine(t, res.Diagnostic)
	}
}

// The same assignment-shaped bytes are ordinary operands after the first
// -- when arity permits.
func TestAssignmentSpellingsAfterTerminator(t *testing.T) {
	for _, arg := range []string{
		"--ignore-case=false", "-i=false", "--unrestricted=false",
		"--help=false", "-h=false",
	} {
		res, _ := parse(t, []string{"--", arg}, os.Stat)
		if res.Kind != cli.KindSearch || res.Pattern != arg {
			t.Fatalf(`Parse(["--", %q]) = kind %v pattern %q, want a search with that pattern`, arg, res.Kind, res.Pattern)
		}
	}
}

func TestPositionalsAndRoot(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	subdir := filepath.Join(dir, "sub")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	linkDir := filepath.Join(dir, "linkdir")
	if err := os.Symlink(subdir, linkDir); err != nil {
		t.Fatal(err)
	}
	linkFile := filepath.Join(dir, "linkfile")
	if err := os.Symlink(file, linkFile); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(dir, "fifo")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "missing")

	cases := []struct {
		name        string
		args        []string
		wantKind    cli.Kind
		wantErr     cli.ErrorKind
		wantPattern string
		wantRoot    string
	}{
		{"pattern only defaults root", []string{"foo"}, cli.KindSearch, 0, "foo", "."},
		{"explicit directory root", []string{"foo", subdir}, cli.KindSearch, 0, "foo", subdir},
		{"regular file root", []string{"foo", file}, cli.KindSearch, 0, "foo", file},
		{"symlink to directory", []string{"foo", linkDir}, cli.KindSearch, 0, "foo", linkDir},
		{"symlink to regular file", []string{"foo", linkFile}, cli.KindSearch, 0, "foo", linkFile},
		{"empty pattern is present", []string{"", "."}, cli.KindSearch, 0, "", "."},
		{"literal dash pattern", []string{"-", "."}, cli.KindSearch, 0, "-", "."},
		{"dash pattern via terminator", []string{"--", "-"}, cli.KindSearch, 0, "-", "."},
		{"option terminator only", []string{"--"}, cli.KindUsageError, cli.ErrMissingPattern, "", ""},
		{"excess operands", []string{"a", "b", "c"}, cli.KindUsageError, cli.ErrExcessOperand, "", ""},
		{"excess operand after terminator", []string{"a", "b", "--", "c"}, cli.KindUsageError, cli.ErrExcessOperand, "", ""},
		{"unsupported long option", []string{"--unsupported"}, cli.KindUsageError, cli.ErrUnsupportedOption, "", ""},
		{"unsupported short option", []string{"-e"}, cli.KindUsageError, cli.ErrUnsupportedOption, "", ""},
		{"unsupported option after pattern", []string{"foo", "-e"}, cli.KindUsageError, cli.ErrUnsupportedOption, "", ""},
		{"argument-taking option rejected", []string{"--type", "go", "foo"}, cli.KindUsageError, cli.ErrUnsupportedOption, "", ""},
		{"argument-taking short rejected", []string{"-t", "go", "foo"}, cli.KindUsageError, cli.ErrUnsupportedOption, "", ""},
		{"undeclared combined letters rejected", []string{"-foo"}, cli.KindUsageError, cli.ErrUnsupportedOption, "", ""},
		{"dash-leading pattern needs terminator", []string{"-foo", "."}, cli.KindUsageError, cli.ErrUnsupportedOption, "", ""},
		{"invalid help assignment value", []string{"foo", "--help=maybe"}, cli.KindUsageError, cli.ErrUnsupportedOption, "", ""},
		{"empty help assignment value", []string{"foo", "--help="}, cli.KindUsageError, cli.ErrUnsupportedOption, "", ""},
		{"nonexistent root", []string{"foo", missing}, cli.KindUsageError, cli.ErrInvalidRoot, "", ""},
		{"stdin root", []string{"foo", "-"}, cli.KindUsageError, cli.ErrInvalidRoot, "", ""},
		{"special file root", []string{"foo", fifo}, cli.KindUsageError, cli.ErrInvalidRoot, "", ""},
		{"device root", []string{"foo", "/dev/null"}, cli.KindUsageError, cli.ErrInvalidRoot, "", ""},
		{"dash-leading operand after terminator", []string{"foo", "--", "-h"}, cli.KindUsageError, cli.ErrInvalidRoot, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, out := parse(t, tc.args, os.Stat)
			if res.Kind != tc.wantKind {
				t.Fatalf("Parse(%q).Kind = %v, want %v (diagnostic %q)", tc.args, res.Kind, tc.wantKind, res.Diagnostic)
			}
			if out != "" {
				t.Fatalf("Parse(%q) wrote %q to the help writer on a non-help path", tc.args, out)
			}
			switch tc.wantKind {
			case cli.KindSearch:
				if res.Pattern != tc.wantPattern || res.Root != tc.wantRoot {
					t.Fatalf("Parse(%q) = pattern %q root %q, want %q %q", tc.args, res.Pattern, res.Root, tc.wantPattern, tc.wantRoot)
				}
				assertChildArgs(t, res, []string{"--json", "--no-config", "--", tc.wantPattern, tc.wantRoot})
			case cli.KindUsageError:
				if res.ErrorKind != tc.wantErr {
					t.Fatalf("Parse(%q).ErrorKind = %v, want %v", tc.args, res.ErrorKind, tc.wantErr)
				}
				assertSanitizedLine(t, res.Diagnostic)
			}
		})
	}
}

// Non-root usage errors must be classified before root validation runs.
func TestUsageErrorsPrecedeRootValidation(t *testing.T) {
	for _, args := range [][]string{
		{"--"},
		{"a", "b", "c"},
		{"--unsupported"},
		{"foo", "-e"},
		{"foo", "--ignore-case=false"},
		{"-uuu", "foo"},
	} {
		res, _ := parse(t, args, failStat(t))
		if res.Kind != cli.KindUsageError {
			t.Fatalf("Parse(%q).Kind = %v, want KindUsageError", args, res.Kind)
		}
	}
}

// A regular file literally named "-" is reachable as "./-" and validates.
func TestDashFileRoot(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "-"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	res, _ := parse(t, []string{"foo", "./-"}, os.Stat)
	if res.Kind != cli.KindSearch {
		t.Fatalf(`Parse(["foo", "./-"]).Kind = %v, want KindSearch (diagnostic %q)`, res.Kind, res.Diagnostic)
	}
	if res.Root != "./-" {
		t.Fatalf("root = %q, want %q", res.Root, "./-")
	}
}

// Help-like tokens after the first -- are positional operands, never help
// requests.
func TestHelpLikeTokensAfterTerminator(t *testing.T) {
	cases := []struct {
		args        []string
		wantPattern string
		wantRoot    string
	}{
		{[]string{"--", "--help"}, "--help", "."},
		{[]string{"--", "-h"}, "-h", "."},
		{[]string{"--", "--"}, "--", "."},
		{[]string{"--", "-h", "."}, "-h", "."},
	}
	for _, tc := range cases {
		var calls []string
		res, out := parse(t, tc.args, recordStat(&calls))
		if res.Kind != cli.KindSearch {
			t.Fatalf("Parse(%q).Kind = %v, want KindSearch (diagnostic %q)", tc.args, res.Kind, res.Diagnostic)
		}
		if res.Pattern != tc.wantPattern || res.Root != tc.wantRoot {
			t.Fatalf("Parse(%q) = pattern %q root %q, want %q %q", tc.args, res.Pattern, res.Root, tc.wantPattern, tc.wantRoot)
		}
		if out != "" {
			t.Fatalf("Parse(%q) emitted help for a post-terminator token: %q", tc.args, out)
		}
	}
}

// Usage diagnostics are classified, single-line, and sanitized; hostile
// operand bytes can never reach the diagnostic raw.
func TestUsageDiagnosticSafety(t *testing.T) {
	cases := []struct {
		name         string
		args         []string
		wantErr      cli.ErrorKind
		wantContains string
	}{
		{"escape bytes in option", []string{"--bad\x1b[31mopt"}, cli.ErrUnsupportedOption, "^["},
		{"invalid utf8 in option", []string{"--bad\xff"}, cli.ErrUnsupportedOption, `\xff`},
		{"escape bytes in root", []string{"foo", "/no\x1b[31msuch"}, cli.ErrInvalidRoot, "^["},
		{"newline in root", []string{"foo", "no\nsuch"}, cli.ErrInvalidRoot, `\n`},
		{"tab in excess operand", []string{"a", "b", "c\td"}, cli.ErrExcessOperand, `\t`},
		{"backslash in operand", []string{"a", "b", "c\\d"}, cli.ErrExcessOperand, `\\`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, _ := parse(t, tc.args, os.Stat)
			if res.Kind != cli.KindUsageError || res.ErrorKind != tc.wantErr {
				t.Fatalf("Parse(%q) = kind %v err %v, want usage error %v", tc.args, res.Kind, res.ErrorKind, tc.wantErr)
			}
			assertSanitizedLine(t, res.Diagnostic)
			if !strings.Contains(res.Diagnostic, tc.wantContains) {
				t.Fatalf("diagnostic %q does not show the escaped form %q", res.Diagnostic, tc.wantContains)
			}
		})
	}
}

// assertChildArgs requires the exact protected child argument vector:
// --json --no-config, the ordered expanded user flags, --, pattern, root.
func assertChildArgs(t *testing.T, res cli.Result, want []string) {
	t.Helper()
	if res.Kind != cli.KindSearch {
		t.Fatalf("result kind = %v, want KindSearch (diagnostic %q)", res.Kind, res.Diagnostic)
	}
	if !slices.Equal(res.ChildArgs, want) {
		t.Fatalf("child argv = %q, want %q", res.ChildArgs, want)
	}
}

// Every allow-listed no-argument search flag is accepted in both spellings
// and forwarded verbatim, ahead of --, the pattern, and the root.
func TestSearchFlagsForwarded(t *testing.T) {
	for _, flag := range []string{
		"-i", "--ignore-case",
		"-S", "--smart-case",
		"-s", "--case-sensitive",
		"-w", "--word-regexp",
		"-x", "--line-regexp",
		"-F", "--fixed-strings",
		"--hidden", "--no-hidden", "--no-ignore",
		"-u", "--unrestricted",
		"-L", "--follow",
	} {
		res, out := parse(t, []string{flag, "foo"}, os.Stat)
		if out != "" {
			t.Fatalf("Parse(%q) wrote to the help writer on a search path: %q", flag, out)
		}
		assertChildArgs(t, res, []string{"--json", "--no-config", flag, "--", "foo", "."})
	}
}

// Encounter order and supplied spellings survive to the child argv for
// repeated, combined, mixed-alias, and operand-interleaved options. Order
// comes from the ordered scan records, never from library value
// assignment, and contradictory flags are forwarded without normalization.
func TestOrderedChildArgs(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{"no flags", []string{"foo"}, []string{"--json", "--no-config", "--", "foo", "."}},
		{"repeated and interleaved shorts", []string{"-i", "-s", "-i", "foo"}, []string{"--json", "--no-config", "-i", "-s", "-i", "--", "foo", "."}},
		{"combined shorts expand in order", []string{"-isi", "foo"}, []string{"--json", "--no-config", "-i", "-s", "-i", "--", "foo", "."}},
		{"combined token -iwF", []string{"-iwF", "foo"}, []string{"--json", "--no-config", "-i", "-w", "-F", "--", "foo", "."}},
		{"mixed long and short aliases", []string{"--ignore-case", "-s", "-i", "foo"}, []string{"--json", "--no-config", "--ignore-case", "-s", "-i", "--", "foo", "."}},
		{"options interleaved with both operands", []string{"foo", "-i", dir, "-s"}, []string{"--json", "--no-config", "-i", "-s", "--", "foo", dir}},
		{"options before and between operands", []string{"-w", "foo", "-F", dir}, []string{"--json", "--no-config", "-w", "-F", "--", "foo", dir}},
		{"contradictory flags not normalized", []string{"--hidden", "--no-hidden", "foo"}, []string{"--json", "--no-config", "--hidden", "--no-hidden", "--", "foo", "."}},
		{"same flag repeated", []string{"-i", "-i", "foo"}, []string{"--json", "--no-config", "-i", "-i", "--", "foo", "."}},
		{"flags after both operands", []string{"foo", ".", "-s", "-w"}, []string{"--json", "--no-config", "-s", "-w", "--", "foo", "."}},
		{"empty pattern", []string{"", "."}, []string{"--json", "--no-config", "--", "", "."}},
		{"literal dash pattern", []string{"-", "."}, []string{"--json", "--no-config", "--", "-", "."}},
		{"dash-leading pattern after terminator", []string{"--", "-foo"}, []string{"--json", "--no-config", "--", "-foo", "."}},
		{"second terminator is the pattern", []string{"--", "--"}, []string{"--json", "--no-config", "--", "--", "."}},
		{"second terminator with explicit root", []string{"--", "--", dir}, []string{"--json", "--no-config", "--", "--", dir}},
		{"flags before terminator", []string{"-i", "--", "-x"}, []string{"--json", "--no-config", "-i", "--", "-x", "."}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, out := parse(t, tc.args, os.Stat)
			if out != "" {
				t.Fatalf("Parse(%q) wrote to the help writer on a search path: %q", tc.args, out)
			}
			assertChildArgs(t, res, tc.want)
		})
	}
}

// Unrestricted occurrences are counted cumulatively across separate,
// combined, short, and long spellings: two pass, a third is a classified
// usage error with no root validation and no child argv.
func TestUnrestrictedLimit(t *testing.T) {
	accepted := [][]string{
		{"-u", "foo"},
		{"-uu", "foo"},
		{"-iu", "foo"},
		{"-iuu", "foo"},
		{"-u", "--unrestricted", "foo"},
		{"--unrestricted", "-u", "foo"},
		{"-u", "-u", "foo"},
	}
	for _, args := range accepted {
		res, out := parse(t, args, os.Stat)
		if out != "" {
			t.Fatalf("Parse(%q) wrote to the help writer: %q", args, out)
		}
		if res.Kind != cli.KindSearch {
			t.Fatalf("Parse(%q).Kind = %v, want KindSearch (diagnostic %q)", args, res.Kind, res.Diagnostic)
		}
	}

	rejected := [][]string{
		{"-uuu", "foo"},
		{"-u", "-uu", "foo"},
		{"-iuuu", "foo"},
		{"-u", "--unrestricted", "-u", "foo"},
		{"--unrestricted", "-uu", "foo"},
		{"-u", "-u", "-u", "foo"},
		{"--unrestricted", "--unrestricted", "--unrestricted", "foo"},
	}
	for _, args := range rejected {
		res, out := parse(t, args, failStat(t))
		if res.Kind != cli.KindUsageError || res.ErrorKind != cli.ErrExcessUnrestricted {
			t.Fatalf("Parse(%q) = kind %v err %v, want excess-unrestricted usage error", args, res.Kind, res.ErrorKind)
		}
		if res.ChildArgs != nil {
			t.Fatalf("Parse(%q) produced a child argv past the unrestricted limit: %q", args, res.ChildArgs)
		}
		if out != "" {
			t.Fatalf("Parse(%q) emitted help text: %q", args, out)
		}
		assertSanitizedLine(t, res.Diagnostic)
	}
}

// Flags without a pattern remain the Issue 1 missing-pattern usage error.
func TestFlagsOnlyMissingPattern(t *testing.T) {
	for _, args := range [][]string{
		{"-i"},
		{"-iw"},
		{"--hidden"},
		{"-u", "-u"},
		{"--ignore-case", "--follow"},
	} {
		res, out := parse(t, args, failStat(t))
		if res.Kind != cli.KindUsageError || res.ErrorKind != cli.ErrMissingPattern {
			t.Fatalf("Parse(%q) = kind %v err %v, want missing-pattern usage error", args, res.Kind, res.ErrorKind)
		}
		if res.ChildArgs != nil {
			t.Fatalf("Parse(%q) produced a child argv without a pattern: %q", args, res.ChildArgs)
		}
		if out != "" {
			t.Fatalf("Parse(%q) emitted help text: %q", args, out)
		}
	}
}

// Generated help lists every allow-listed search flag in short and long
// form alongside the Issue 1 syntax, positional, and local-help entries.
func TestGeneratedHelpListsSearchFlags(t *testing.T) {
	for _, spelling := range []string{
		"-i", "--ignore-case",
		"-S", "--smart-case",
		"-s", "--case-sensitive",
		"-w", "--word-regexp",
		"-x", "--line-regexp",
		"-F", "--fixed-strings",
		"--hidden", "--no-hidden", "--no-ignore",
		"-u", "--unrestricted",
		"-L", "--follow",
		"-h", "--help",
	} {
		if !strings.Contains(cli.HelpText(), spelling) {
			t.Errorf("generated help missing %q:\n%s", spelling, cli.HelpText())
		}
	}
}

func assertSanitizedLine(t *testing.T, diag string) {
	t.Helper()
	if diag == "" {
		t.Fatal("usage error carries an empty diagnostic")
	}
	if strings.ContainsAny(diag, "\n\r\x1b\x9b") {
		t.Fatalf("diagnostic contains raw line or control bytes: %q", diag)
	}
	for i := 0; i < len(diag); i++ {
		if diag[i] < 0x20 || diag[i] == 0x7f {
			t.Fatalf("diagnostic contains raw control byte 0x%02x: %q", diag[i], diag)
		}
	}
}

// The classified diagnostics name the failure specifically; the library's
// generic "incorrect usage" must never surface.
func TestDiagnosticsAreSpecific(t *testing.T) {
	for _, args := range [][]string{
		{"--"},
		{"a", "b", "c"},
		{"--unsupported"},
		{"foo", "/nonexistent"},
	} {
		res, _ := parse(t, args, os.Stat)
		if res.Kind != cli.KindUsageError {
			t.Fatalf("Parse(%q).Kind = %v, want KindUsageError", args, res.Kind)
		}
		if strings.Contains(res.Diagnostic, "incorrect usage") {
			t.Fatalf("Parse(%q) surfaced the library diagnostic: %q", args, res.Diagnostic)
		}
	}
}

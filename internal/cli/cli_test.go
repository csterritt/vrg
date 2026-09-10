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
		{"combined short help", []string{"-ih"}},
		{"combined short help first", []string{"-hi"}},
		{"combined short help with unknowns", []string{"-xh"}},
		{"help token joined to letters", []string{"-help"}},
		{"help before option terminator", []string{"-h", "--"}},
		{"help before terminator and operand", []string{"--help", "--", "x"}},
		{"combined short help with pattern", []string{"-ih", "foo"}},
		{"long search flag then help", []string{"--ignore-case", "--help"}},
		{"combined search flags then help", []string{"-iwF", "-h"}},
		{"help between pattern and flag", []string{"foo", "-i", "--help"}},
		{"help after two operands", []string{"foo", "src", "--help"}},
		{"help after unrestricted overrun", []string{"-uuu", "--help"}},
		{"help after rejected assignment", []string{"--ignore-case=false", "--help"}},
		{"help after rejected short option", []string{"-e", "--help"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, out := parse(t, tc.args, failStat(t))
			if res.Kind != cli.KindHelp {
				t.Fatalf("Parse(%q).Kind = %v, want KindHelp", tc.args, res.Kind)
			}
			if res.Pattern != "" || res.Root != "" || len(res.ChildArgs) != 0 {
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

// The library seam: a help value set through a successful parse (the
// assignment spelling, which the raw-token preflight does not classify as
// a help request) must still produce the help-only result before root
// validation or search work. A bare --help=true without a pattern is not a
// help request at all and stays a missing-pattern usage error.
func TestParsedLocalHelpValue(t *testing.T) {
	for _, args := range [][]string{
		{"foo", "--help=true"},
		{"foo", "-h=true"},
		{"--help=true", "foo"},
	} {
		res, out := parse(t, args, failStat(t))
		if res.Kind != cli.KindHelp {
			t.Fatalf("Parse(%q).Kind = %v, want KindHelp from the parsed help value", args, res.Kind)
		}
		assertOneHelpCopy(t, out)
	}

	res, _ := parse(t, []string{"--help=true"}, failStat(t))
	if res.Kind != cli.KindUsageError || res.ErrorKind != cli.ErrMissingPattern {
		t.Fatalf(`Parse(["--help=true"]).Kind = %v (%v), want missing-pattern usage error`, res.Kind, res.ErrorKind)
	}
}

// Assignment spellings that disable help are not help requests. They are
// rejected lexically as usage errors before root validation, distinct
// from the parsed --help=true seam above: the no-argument contract does
// not widen just because the library would parse a boolean value.
func TestHelpAssignmentSpellingsAreNotHelpRequests(t *testing.T) {
	for _, arg := range []string{"--help=false", "-h=false"} {
		t.Run(arg, func(t *testing.T) {
			res, out := parse(t, []string{"foo", arg}, failStat(t))
			if res.Kind == cli.KindHelp {
				t.Fatalf("Parse(%q) produced the help-only result for a help-disabling spelling", []string{"foo", arg})
			}
			if res.Kind != cli.KindUsageError || res.ErrorKind != cli.ErrUnsupportedOption {
				t.Fatalf("Parse(%q) = kind %v err %v, want an unsupported-option usage error", []string{"foo", arg}, res.Kind, res.ErrorKind)
			}
			if strings.Contains(out, "Usage:") {
				t.Fatalf("help text emitted for %q: %q", arg, out)
			}
			assertSanitizedLine(t, res.Diagnostic)
		})
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
		{"-uuu", "foo"},
		{"--ignore-case=false", "foo"},
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

// wantChildArgs builds the required child argument vector: the mandatory
// internal flags, the user's flag spellings in encounter order, the
// terminator, then pattern and root.
func wantChildArgs(flags []string, pattern, root string) []string {
	argv := []string{"--json", "--no-config"}
	argv = append(argv, flags...)
	return append(argv, "--", pattern, root)
}

// assertSearchArgv requires a successful parse whose public child argv —
// the module's forwarding contract — is exactly want.
func assertSearchArgv(t *testing.T, args []string, res cli.Result, want []string) {
	t.Helper()
	if res.Kind != cli.KindSearch {
		t.Fatalf("Parse(%q).Kind = %v, want KindSearch (diagnostic %q)", args, res.Kind, res.Diagnostic)
	}
	if !slices.Equal(res.ChildArgs, want) {
		t.Fatalf("Parse(%q).ChildArgs = %q, want %q", args, res.ChildArgs, want)
	}
}

// assertRejectedOption requires a classified unsupported-option usage
// error with a sanitized single-line diagnostic, no help output, and no
// root validation.
func assertRejectedOption(t *testing.T, args []string) {
	t.Helper()
	res, out := parse(t, args, failStat(t))
	if res.Kind != cli.KindUsageError || res.ErrorKind != cli.ErrUnsupportedOption {
		t.Fatalf("Parse(%q) = kind %v err %v, want unsupported-option usage error", args, res.Kind, res.ErrorKind)
	}
	if out != "" {
		t.Fatalf("Parse(%q) wrote %q to the help writer on a non-help path", args, out)
	}
	assertSanitizedLine(t, res.Diagnostic)
}

// Every allow-listed no-argument search flag is accepted in its short and
// long form and forwarded verbatim to the child argv between the
// mandatory internal flags and the terminator.
func TestSearchFlagSpellingsForwarded(t *testing.T) {
	for _, flag := range []string{
		"-i", "--ignore-case",
		"-S", "--smart-case",
		"-s", "--case-sensitive",
		"-w", "--word-regexp",
		"-x", "--line-regexp",
		"-F", "--fixed-strings",
		"--hidden",
		"--no-hidden",
		"--no-ignore",
		"-u", "--unrestricted",
		"-L", "--follow",
	} {
		t.Run(flag, func(t *testing.T) {
			args := []string{flag, "foo"}
			res, out := parse(t, args, os.Stat)
			assertSearchArgv(t, args, res, wantChildArgs([]string{flag}, "foo", "."))
			if res.Pattern != "foo" || res.Root != "." {
				t.Fatalf("Parse(%q) = pattern %q root %q, want %q %q", args, res.Pattern, res.Root, "foo", ".")
			}
			if out != "" {
				t.Fatalf("Parse(%q) wrote %q to the help writer on a non-help path", args, out)
			}
		})
	}
}

// Forwarding preserves encounter order and the supplied spelling —
// derived from the ordered scan records, never from library callback
// order — including repetitions, combined-short expansion, mixed aliases,
// options interleaved with both operands, and contradictory flags vrg
// deliberately does not normalize.
func TestChildArgvPreservesEncounterOrderAndSpelling(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name        string
		args        []string
		wantFlags   []string
		wantPattern string
		wantRoot    string
	}{
		{"repeated shorts", []string{"-i", "-s", "-i", "foo"}, []string{"-i", "-s", "-i"}, "foo", "."},
		{"combined shorts expand in order", []string{"-isi", "foo"}, []string{"-i", "-s", "-i"}, "foo", "."},
		{"mixed long and short aliases", []string{"--ignore-case", "-s", "-i", "foo"}, []string{"--ignore-case", "-s", "-i"}, "foo", "."},
		{"combined across option kinds", []string{"-iwF", "foo"}, []string{"-i", "-w", "-F"}, "foo", "."},
		{"options interleaved with both operands", []string{"foo", "-i", dir, "-s"}, []string{"-i", "-s"}, "foo", dir},
		{"options before between and after operands", []string{"-s", "foo", "-i", dir, "-w"}, []string{"-s", "-i", "-w"}, "foo", dir},
		{"unrestricted mixed aliases", []string{"-u", "--unrestricted", "foo"}, []string{"-u", "--unrestricted"}, "foo", "."},
		{"contradictory flags not normalized", []string{"-i", "-s", "-S", "foo"}, []string{"-i", "-s", "-S"}, "foo", "."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, _ := parse(t, tc.args, os.Stat)
			assertSearchArgv(t, tc.args, res, wantChildArgs(tc.wantFlags, tc.wantPattern, tc.wantRoot))
		})
	}
}

// Unrestricted occurrences accumulate across short, long, and combined
// spellings from the scan records: zero to two are forwarded, a third in
// any token mix is a usage error before root validation.
func TestCumulativeUnrestrictedBoundary(t *testing.T) {
	accepted := []struct {
		name      string
		args      []string
		wantFlags []string
	}{
		{"single short", []string{"-u", "foo"}, []string{"-u"}},
		{"combined pair", []string{"-uu", "foo"}, []string{"-u", "-u"}},
		{"one inside combined token", []string{"-iu", "foo"}, []string{"-i", "-u"}},
		{"two inside combined token", []string{"-iuu", "foo"}, []string{"-i", "-u", "-u"}},
		{"short then long", []string{"-u", "--unrestricted", "foo"}, []string{"-u", "--unrestricted"}},
		{"long then combined", []string{"--unrestricted", "-iu", "foo"}, []string{"--unrestricted", "-i", "-u"}},
	}
	for _, tc := range accepted {
		t.Run("accepted "+tc.name, func(t *testing.T) {
			res, _ := parse(t, tc.args, os.Stat)
			assertSearchArgv(t, tc.args, res, wantChildArgs(tc.wantFlags, "foo", "."))
		})
	}

	rejected := [][]string{
		{"-uuu", "foo"},
		{"-u", "-uu", "foo"},
		{"-iuuu", "foo"},
		{"-u", "--unrestricted", "-u", "foo"},
		{"--unrestricted", "-uu", "foo"},
		{"-u", "-u", "-u", "foo"},
		{"-uuu"}, // the overrun classifies before arity
	}
	for _, args := range rejected {
		assertRejectedOption(t, args)
	}
}

// Every option outside the allow-list is a usage error, including -e and
// argument-taking options in any spelling.
func TestUnsupportedOptionsRejected(t *testing.T) {
	for _, args := range [][]string{
		{"-e", "foo"},
		{"foo", "-e"},
		{"-e"},
		{"-foo"},
		{"--type", "go", "foo"},
		{"--type=go", "foo"},
		{"-t", "go", "foo"},
		{"--max-count", "3", "foo"},
		{"--max-count=3", "foo"},
		{"-m", "3", "foo"},
		{"-g", "*.go", "foo"},
		{"--glob", "*.go", "foo"},
		{"-A", "2", "foo"},
		{"-A2", "foo"},
		{"--vimgrep", "foo"},
		{"--no-config", "foo"},
	} {
		assertRejectedOption(t, args)
	}
}

// Boolean assignment spellings for no-argument options are rejected
// lexically — the allow-list contract does not widen just because the
// library would parse a bool value. Truthy help assignments are the sole
// exception: they parse into the help value (TestParsedLocalHelpValue).
func TestAssignmentSpellingsRejected(t *testing.T) {
	for _, args := range [][]string{
		{"--ignore-case=false", "foo"},
		{"--ignore-case=true", "foo"},
		{"-i=false", "foo"},
		{"-i=true", "foo"},
		{"--unrestricted=false", "foo"},
		{"--unrestricted=0", "foo"},
		{"--hidden=true", "foo"},
		{"--no-ignore=1", "foo"},
		{"--help=false", "foo"},
		{"--help=0", "foo"},
		{"-h=false", "foo"},
		{"foo", "--ignore-case=false"},
		{"-i=false"},
	} {
		assertRejectedOption(t, args)
	}
}

// After the first -- the same bytes are positional operands whenever
// arity permits; they are never option spellings.
func TestAssignmentSpellingsPositionalAfterTerminator(t *testing.T) {
	cases := []struct {
		args        []string
		wantPattern string
		wantRoot    string
	}{
		{[]string{"--", "--ignore-case=false"}, "--ignore-case=false", "."},
		{[]string{"--", "-i=false", "."}, "-i=false", "."},
		{[]string{"--", "--unrestricted=false", "."}, "--unrestricted=false", "."},
		{[]string{"--", "--help=false", "."}, "--help=false", "."},
		{[]string{"--", "-h=false", "."}, "-h=false", "."},
	}
	for _, tc := range cases {
		res, _ := parse(t, tc.args, os.Stat)
		assertSearchArgv(t, tc.args, res, wantChildArgs(nil, tc.wantPattern, tc.wantRoot))
	}
}

// The child argv carries operand forms verbatim after the terminator:
// the empty pattern as an empty element, the literal -, protected
// dash-leading patterns, and a second -- as the pattern itself in both
// root forms.
func TestChildArgvOperandForms(t *testing.T) {
	cases := []struct {
		name        string
		args        []string
		wantFlags   []string
		wantPattern string
		wantRoot    string
	}{
		{"empty pattern forwarded verbatim", []string{"", "."}, nil, "", "."},
		{"literal dash pattern", []string{"-", "."}, nil, "-", "."},
		{"dash-leading pattern after terminator", []string{"--", "-foo"}, nil, "-foo", "."},
		{"dash-leading pattern with flags", []string{"-i", "--", "-foo"}, []string{"-i"}, "-foo", "."},
		{"literal -- pattern default root", []string{"--", "--"}, nil, "--", "."},
		{"literal -- pattern explicit root", []string{"--", "--", "."}, nil, "--", "."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, _ := parse(t, tc.args, os.Stat)
			assertSearchArgv(t, tc.args, res, wantChildArgs(tc.wantFlags, tc.wantPattern, tc.wantRoot))
		})
	}
}

// Search flags without a pattern remain a missing-pattern usage error,
// classified before root validation.
func TestFlagsOnlyMissingPattern(t *testing.T) {
	for _, args := range [][]string{
		{"-i"},
		{"-i", "-s"},
		{"--hidden"},
		{"-iwF"},
		{"-u", "--unrestricted"},
	} {
		res, out := parse(t, args, failStat(t))
		if res.Kind != cli.KindUsageError || res.ErrorKind != cli.ErrMissingPattern {
			t.Fatalf("Parse(%q) = kind %v err %v, want missing-pattern usage error", args, res.Kind, res.ErrorKind)
		}
		if out != "" {
			t.Fatalf("Parse(%q) wrote %q to the help writer on a non-help path", args, out)
		}
		assertSanitizedLine(t, res.Diagnostic)
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

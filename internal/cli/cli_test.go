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
		{"combined short help then pattern", []string{"-ih", "foo"}},
		{"combined short help first", []string{"-hi"}},
		{"combined short help with unknowns", []string{"-xh"}},
		{"mixed search flags then help", []string{"-i", "-s", "--help"}},
		{"help between search flags", []string{"-i", "-h", "-s"}},
		{"help beats excess unrestricted", []string{"-uuu", "--help"}},
		{"help beats unsupported option", []string{"-e", "--help"}},
		{"help after valid root", []string{"foo", ".", "--help"}},
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
			if res.Pattern != "" || res.Root != "" || len(res.ChildArgv) != 0 {
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

// Issue 2 makes the CLI a strict no-argument contract: every "="
// assignment spelling for a declared option — search flag or help — is
// rejected lexically by the preflight scan before the permissive library
// parser can accept it. The result is an unsupported-option usage error,
// never help and never a search, and root validation never runs.
func TestAssignmentSpellingsAreUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"foo", "--help=true"},
		{"foo", "-h=true"},
		{"--help=true", "foo"},
		{"--help=true"},
		{"foo", "--help=false"},
		{"foo", "-h=false"},
		{"--help=false", "foo"},
		{"foo", "--ignore-case=false"},
		{"--ignore-case=false", "foo"},
		{"foo", "-i=false"},
		{"foo", "--unrestricted=false"},
		{"foo", "--smart-case="},
	} {
		res, out := parse(t, args, failStat(t))
		if res.Kind != cli.KindUsageError || res.ErrorKind != cli.ErrUnsupportedOption {
			t.Fatalf("Parse(%q) = kind %v err %v, want unsupported-option usage error", args, res.Kind, res.ErrorKind)
		}
		if strings.Contains(out, "Usage:") {
			t.Fatalf("Parse(%q) emitted help for an assignment spelling: %q", args, out)
		}
		if len(res.ChildArgv) != 0 {
			t.Fatalf("Parse(%q) produced child argv for an assignment spelling: %q", args, res.ChildArgv)
		}
		assertSanitizedLine(t, res.Diagnostic)
	}
}

// The identical bytes are ordinary positional operands after the first
// --: assignment spellings, help spellings, and other dash-leading tokens
// are protected when arity permits.
func TestAssignmentSpellingsArePositionalAfterTerminator(t *testing.T) {
	cases := []struct {
		args []string
		argv []string
	}{
		{[]string{"--", "--ignore-case=false"}, wantArgv("--ignore-case=false", ".")},
		{[]string{"--", "-i=false"}, wantArgv("-i=false", ".")},
		{[]string{"--", "-i=false", "."}, wantArgv("-i=false", ".")},
		{[]string{"--", "--unrestricted=false"}, wantArgv("--unrestricted=false", ".")},
		{[]string{"--", "--help=false"}, wantArgv("--help=false", ".")},
		{[]string{"--", "-h=false"}, wantArgv("-h=false", ".")},
		{[]string{"--", "--help=true"}, wantArgv("--help=true", ".")},
	}
	for _, tc := range cases {
		assertChildArgv(t, tc.args, tc.argv)
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
		{"unsupported short option", []string{"-z"}, cli.KindUsageError, cli.ErrUnsupportedOption, "", ""},
		{"unsupported option after pattern", []string{"foo", "-z"}, cli.KindUsageError, cli.ErrUnsupportedOption, "", ""},
		{"search flag without pattern", []string{"-i"}, cli.KindUsageError, cli.ErrMissingPattern, "", ""},
		{"repeated flag without pattern", []string{"-u", "-u"}, cli.KindUsageError, cli.ErrMissingPattern, "", ""},
		{"combined flags without pattern", []string{"-iw"}, cli.KindUsageError, cli.ErrMissingPattern, "", ""},
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
		{"foo", "-z"},
		{"-uuu", "foo"},
		{"foo", "--help=false"},
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
		{[]string{"--", "--", "."}, "--", "."},
		{[]string{"--", "-i"}, "-i", "."},
		{[]string{"--", "-u", "."}, "-u", "."},
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

// wantArgv builds the expected child argument vector for a search: the
// mandatory internal flags, the user's flags in encounter order, the
// terminator, and the pattern and root operands.
func wantArgv(pattern, root string, flags ...string) []string {
	argv := []string{"--json", "--no-config"}
	argv = append(argv, flags...)
	return append(argv, "--", pattern, root)
}

// assertChildArgv requires a search result whose public child argument
// vector — the module's contract with the eventual rg subprocess — is
// exactly want, element for element.
func assertChildArgv(t *testing.T, args []string, want []string) {
	t.Helper()
	res, out := parse(t, args, os.Stat)
	if res.Kind != cli.KindSearch {
		t.Fatalf("Parse(%q).Kind = %v, want KindSearch (diagnostic %q)", args, res.Kind, res.Diagnostic)
	}
	if out != "" {
		t.Fatalf("Parse(%q) wrote %q to the help writer on a search path", args, out)
	}
	if !slices.Equal(res.ChildArgv, want) {
		t.Fatalf("Parse(%q).ChildArgv = %q, want %q", args, res.ChildArgv, want)
	}
}

// Every allow-listed flag forwards its supplied spelling to the child,
// in both short and long form.
func TestAcceptedSearchFlags(t *testing.T) {
	cases := []struct {
		name string
		args []string
		argv []string
	}{
		{"short -i", []string{"-i", "foo"}, wantArgv("foo", ".", "-i")},
		{"long --ignore-case", []string{"--ignore-case", "foo"}, wantArgv("foo", ".", "--ignore-case")},
		{"short -S", []string{"-S", "foo"}, wantArgv("foo", ".", "-S")},
		{"long --smart-case", []string{"--smart-case", "foo"}, wantArgv("foo", ".", "--smart-case")},
		{"short -s", []string{"-s", "foo"}, wantArgv("foo", ".", "-s")},
		{"long --case-sensitive", []string{"--case-sensitive", "foo"}, wantArgv("foo", ".", "--case-sensitive")},
		{"short -w", []string{"-w", "foo"}, wantArgv("foo", ".", "-w")},
		{"long --word-regexp", []string{"--word-regexp", "foo"}, wantArgv("foo", ".", "--word-regexp")},
		{"short -x", []string{"-x", "foo"}, wantArgv("foo", ".", "-x")},
		{"long --line-regexp", []string{"--line-regexp", "foo"}, wantArgv("foo", ".", "--line-regexp")},
		{"short -F", []string{"-F", "foo"}, wantArgv("foo", ".", "-F")},
		{"long --fixed-strings", []string{"--fixed-strings", "foo"}, wantArgv("foo", ".", "--fixed-strings")},
		{"long-only --hidden", []string{"--hidden", "foo"}, wantArgv("foo", ".", "--hidden")},
		{"long-only --no-hidden", []string{"--no-hidden", "foo"}, wantArgv("foo", ".", "--no-hidden")},
		{"long-only --no-ignore", []string{"--no-ignore", "foo"}, wantArgv("foo", ".", "--no-ignore")},
		{"short -u", []string{"-u", "foo"}, wantArgv("foo", ".", "-u")},
		{"long --unrestricted", []string{"--unrestricted", "foo"}, wantArgv("foo", ".", "--unrestricted")},
		{"short -L", []string{"-L", "foo"}, wantArgv("foo", ".", "-L")},
		{"long --follow", []string{"--follow", "foo"}, wantArgv("foo", ".", "--follow")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertChildArgv(t, tc.args, tc.argv)
		})
	}
}

// Encounter order and the exact supplied spellings survive into the
// child argv for repeated, combined, mixed-alias, and operand-interleaved
// options. Combined shorts expand left to right; uncombined flags keep
// the short or long form the user typed. Contradictory flags are never
// normalized.
func TestFlagEncounterOrder(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name string
		args []string
		argv []string
	}{
		{"repeated mixed shorts", []string{"-i", "-s", "-i", "foo"}, wantArgv("foo", ".", "-i", "-s", "-i")},
		{"combined shorts", []string{"-isi", "foo"}, wantArgv("foo", ".", "-i", "-s", "-i")},
		{"long then shorts", []string{"--ignore-case", "-s", "-i", "foo"}, wantArgv("foo", ".", "--ignore-case", "-s", "-i")},
		{"combined -iwF expands left to right", []string{"-iwF", "foo"}, wantArgv("foo", ".", "-i", "-w", "-F")},
		{"combined flags then both operands", []string{"-iw", "foo", dir}, wantArgv("foo", dir, "-i", "-w")},
		{"option between operands", []string{"foo", "-i", dir}, wantArgv("foo", dir, "-i")},
		{"options interleaved with both operands", []string{"foo", "-i", dir, "-s"}, wantArgv("foo", dir, "-i", "-s")},
		{"flag after both operands", []string{"foo", dir, "-i"}, wantArgv("foo", dir, "-i")},
		{"contradictory flags preserved", []string{"-i", "-s", "-S", "foo"}, wantArgv("foo", ".", "-i", "-s", "-S")},
		{"hidden pair preserved", []string{"--hidden", "--no-hidden", "foo"}, wantArgv("foo", ".", "--hidden", "--no-hidden")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertChildArgv(t, tc.args, tc.argv)
		})
	}
}

// Unrestricted occurrences accumulate across all spellings — separate
// tokens, combined shorts, and the long alias. Zero to two are accepted;
// the third is a sanitized exit-2 usage error with no root validation,
// child, or TUI.
func TestUnrestrictedCumulativeLimit(t *testing.T) {
	accepted := []struct {
		name string
		args []string
		argv []string
	}{
		{"single short", []string{"-u", "foo"}, wantArgv("foo", ".", "-u")},
		{"combined double", []string{"-uu", "foo"}, wantArgv("foo", ".", "-u", "-u")},
		{"one inside combined", []string{"-iu", "foo"}, wantArgv("foo", ".", "-i", "-u")},
		{"two inside combined", []string{"-iuu", "foo"}, wantArgv("foo", ".", "-i", "-u", "-u")},
		{"short then long", []string{"-u", "--unrestricted", "foo"}, wantArgv("foo", ".", "-u", "--unrestricted")},
		{"two longs", []string{"--unrestricted", "--unrestricted", "foo"}, wantArgv("foo", ".", "--unrestricted", "--unrestricted")},
	}
	for _, tc := range accepted {
		t.Run("accepted/"+tc.name, func(t *testing.T) {
			assertChildArgv(t, tc.args, tc.argv)
		})
	}

	rejected := []struct {
		name string
		args []string
	}{
		{"combined triple", []string{"-uuu", "foo"}},
		{"short then combined double", []string{"-u", "-uu", "foo"}},
		{"three inside combined", []string{"-iuuu", "foo"}},
		{"short long short", []string{"-u", "--unrestricted", "-u", "foo"}},
		{"long then combined double", []string{"--unrestricted", "-uu", "foo"}},
		{"three separate shorts", []string{"-u", "-u", "-u", "foo"}},
		{"three longs", []string{"--unrestricted", "--unrestricted", "--unrestricted", "foo"}},
	}
	for _, tc := range rejected {
		t.Run("rejected/"+tc.name, func(t *testing.T) {
			res, out := parse(t, tc.args, failStat(t))
			if res.Kind != cli.KindUsageError || res.ErrorKind != cli.ErrExcessUnrestricted {
				t.Fatalf("Parse(%q) = kind %v err %v, want excess-unrestricted usage error", tc.args, res.Kind, res.ErrorKind)
			}
			if out != "" {
				t.Fatalf("Parse(%q) wrote %q to the help writer on a usage error", tc.args, out)
			}
			if len(res.ChildArgv) != 0 {
				t.Fatalf("Parse(%q) produced child argv on a usage error: %q", tc.args, res.ChildArgv)
			}
			assertSanitizedLine(t, res.Diagnostic)
		})
	}
}

// Every option outside the allow-list — unknown spellings, the explicitly
// excluded -e, argument-taking options, and combined tokens containing an
// undeclared letter — is a classified usage error, never forwarded.
func TestRejectedOptions(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"-e before pattern", []string{"-e", "foo"}},
		{"-e after pattern", []string{"foo", "-e"}},
		{"argument-taking --type", []string{"--type", "go", "foo"}},
		{"argument-taking --type=", []string{"--type=go", "foo"}},
		{"argument-taking -t", []string{"-t", "go", "foo"}},
		{"context flag -A", []string{"-A", "2", "foo"}},
		{"context flag --context", []string{"--context", "2", "foo"}},
		{"glob --glob", []string{"--glob", "*.go", "foo"}},
		{"combined with undeclared letter", []string{"-iz", "foo"}},
		{"single undeclared letter", []string{"-z", "foo"}},
		{"unknown long", []string{"--multiline", "foo"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, out := parse(t, tc.args, failStat(t))
			if res.Kind != cli.KindUsageError || res.ErrorKind != cli.ErrUnsupportedOption {
				t.Fatalf("Parse(%q) = kind %v err %v, want unsupported-option usage error", tc.args, res.Kind, res.ErrorKind)
			}
			if out != "" {
				t.Fatalf("Parse(%q) wrote %q to the help writer on a usage error", tc.args, out)
			}
			assertSanitizedLine(t, res.Diagnostic)
		})
	}
}

// The first -- ends option recognition: dash-leading patterns need it,
// the lone - is a pattern without it, a second -- is the verbatim
// pattern, and the empty pattern forwards as an empty argv element.
func TestTerminatorAndProtectedPatterns(t *testing.T) {
	dir := t.TempDir()
	accepted := []struct {
		name string
		args []string
		argv []string
	}{
		{"dash-leading pattern via terminator", []string{"--", "-foo"}, wantArgv("-foo", ".")},
		{"dash-leading pattern with root", []string{"--", "-foo", dir}, wantArgv("-foo", dir)},
		{"literal dash pattern", []string{"-", "."}, wantArgv("-", ".")},
		{"literal dash via terminator", []string{"--", "-"}, wantArgv("-", ".")},
		{"second terminator is the pattern", []string{"--", "--"}, wantArgv("--", ".")},
		{"second terminator with explicit root", []string{"--", "--", dir}, wantArgv("--", dir)},
		{"empty pattern forwards empty element", []string{"", "."}, wantArgv("", ".")},
		{"empty pattern with flags", []string{"-i", "", dir}, wantArgv("", dir, "-i")},
		{"flag-like pattern via terminator", []string{"--", "-i"}, wantArgv("-i", ".")},
		{"option after terminator is root", []string{"foo", "--", dir}, wantArgv("foo", dir)},
	}
	for _, tc := range accepted {
		t.Run("accepted/"+tc.name, func(t *testing.T) {
			assertChildArgv(t, tc.args, tc.argv)
		})
	}

	// Without the terminator a dash-leading pattern is an option token.
	res, _ := parse(t, []string{"-foo"}, failStat(t))
	if res.Kind != cli.KindUsageError || res.ErrorKind != cli.ErrUnsupportedOption {
		t.Fatalf(`Parse(["-foo"]) = kind %v err %v, want unsupported-option usage error`, res.Kind, res.ErrorKind)
	}
	assertSanitizedLine(t, res.Diagnostic)
}

// Generated help lists every allow-listed flag in short and long form,
// rendered from the same declarations that drive scanning and parsing.
func TestGeneratedHelpListsSearchFlags(t *testing.T) {
	_, out := parse(t, []string{"--help"}, failStat(t))
	for _, names := range []string{
		"-h, --help",
		"-i, --ignore-case",
		"-S, --smart-case",
		"-s, --case-sensitive",
		"-w, --word-regexp",
		"-x, --line-regexp",
		"-F, --fixed-strings",
		"--hidden",
		"--no-hidden",
		"--no-ignore",
		"-u, --unrestricted",
		"-L, --follow",
	} {
		if !strings.Contains(out, "  "+names+"\t") {
			t.Errorf("generated help missing option line %q:\n%s", names, out)
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

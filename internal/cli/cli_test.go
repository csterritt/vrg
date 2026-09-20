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
		{"long search flag then help", []string{"--ignore-case", "-h", "foo"}},
		{"combined short help", []string{"-ih"}},
		{"combined short help first", []string{"-hi"}},
		{"combined short help with unknowns", []string{"-xh"}},
		{"combined short help with pattern", []string{"-ih", "pattern"}},
		{"help after flag and root", []string{"-i", "foo", ".", "--help"}},
		{"third unrestricted then help", []string{"-uuu", "--help"}},
		{"rejected assignment then help", []string{"--ignore-case=false", "--help"}},
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
			if res.Pattern != "" || res.Root != "" {
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
		"-i",
		"--ignore-case",
		"-S",
		"--smart-case",
		"-s",
		"--case-sensitive",
		"-w",
		"--word-regexp",
		"-x",
		"--line-regexp",
		"-F",
		"--fixed-strings",
		"--hidden",
		"--no-hidden",
		"--no-ignore",
		"-u",
		"--unrestricted",
		"-L",
		"--follow",
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

// Every "=" assignment spelling is rejected lexically: the allow-list is
// a no-argument contract and the help option has no assignment form
// either, so the library's permissive boolean assignment parsing never
// applies. These tokens are classified before root validation and are
// never treated as help requests.
func TestAssignmentFormsRejected(t *testing.T) {
	for _, arg := range []string{
		"--ignore-case=false",
		"-i=false",
		"--unrestricted=false",
		"--help=false",
		"-h=false",
		"--help=true",
		"-h=true",
		"--hidden=yes",
		"-L=1",
		"--ignore-case=",
	} {
		t.Run(arg, func(t *testing.T) {
			res, out := parse(t, []string{"foo", arg}, failStat(t))
			if res.Kind == cli.KindHelp {
				t.Fatalf("Parse(%q) produced the help-only result for an assignment spelling", []string{"foo", arg})
			}
			if res.Kind != cli.KindUsageError || res.ErrorKind != cli.ErrUnsupportedOption {
				t.Fatalf("Parse(%q) = kind %v err %v, want unsupported-option usage error",
					[]string{"foo", arg}, res.Kind, res.ErrorKind)
			}
			assertSanitizedLine(t, res.Diagnostic)
			if out != "" {
				t.Fatalf("Parse(%q) wrote %q to the help writer for an assignment spelling", []string{"foo", arg}, out)
			}
		})
	}
}

// The same assignment bytes are ordinary positional operands after the
// first -- when arity permits.
func TestAssignmentSpellingsAfterTerminator(t *testing.T) {
	cases := []struct {
		args        []string
		wantPattern string
		wantRoot    string
	}{
		{[]string{"--", "--ignore-case=false"}, "--ignore-case=false", "."},
		{[]string{"--", "-i=false", "."}, "-i=false", "."},
		{[]string{"--", "--help=false"}, "--help=false", "."},
	}
	for _, tc := range cases {
		res, out := parse(t, tc.args, os.Stat)
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

	// In the root position the assignment bytes are validated as a root,
	// which proves the token stayed positional rather than being parsed
	// as an option.
	var calls []string
	res, _ := parse(t, []string{"foo", "--", "--unrestricted=false"}, recordStat(&calls))
	if res.Kind != cli.KindUsageError || res.ErrorKind != cli.ErrInvalidRoot {
		t.Fatalf(`Parse(["foo", "--", "--unrestricted=false"]) = kind %v err %v, want invalid-root usage error`,
			res.Kind, res.ErrorKind)
	}
	if len(calls) != 1 || calls[0] != "--unrestricted=false" {
		t.Fatalf("root validation saw %v, want [--unrestricted=false]", calls)
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
		{"flags only no pattern", []string{"-i"}, cli.KindUsageError, cli.ErrMissingPattern, "", ""},
		{"unsupported long option", []string{"--unsupported"}, cli.KindUsageError, cli.ErrUnsupportedOption, "", ""},
		{"unsupported short option", []string{"-z"}, cli.KindUsageError, cli.ErrUnsupportedOption, "", ""},
		{"unsupported option after pattern", []string{"foo", "-e"}, cli.KindUsageError, cli.ErrUnsupportedOption, "", ""},
		{"dash-leading pattern without terminator", []string{"-foo"}, cli.KindUsageError, cli.ErrUnsupportedOption, "", ""},
		{"undeclared letter in combined token", []string{"-iz", "foo"}, cli.KindUsageError, cli.ErrUnsupportedOption, "", ""},
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
		{[]string{"--", "--", "."}, "--", "."},
		{[]string{"--", "-h", "."}, "-h", "."},
		{[]string{"--", "--ignore-case=false"}, "--ignore-case=false", "."},
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

// The classified diagnostics name the failure specifically; the library's
// generic "incorrect usage" must never surface.
func TestDiagnosticsAreSpecific(t *testing.T) {
	for _, args := range [][]string{
		{"--"},
		{"a", "b", "c"},
		{"--unsupported"},
		{"foo", "/nonexistent"},
		{"-uuu", "foo"},
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

// Every allow-listed search flag, in every spelling it supports, reaches
// the child argument vector, and every non-allow-listed option — including
// -e, argument-taking options, and undeclared letters inside combined
// tokens — is a usage error.
func TestSearchFlagAllowList(t *testing.T) {
	accepted := []string{
		"-i", "--ignore-case",
		"-S", "--smart-case",
		"-s", "--case-sensitive",
		"-w", "--word-regexp",
		"-x", "--line-regexp",
		"-F", "--fixed-strings",
		"--hidden", "--no-hidden", "--no-ignore",
		"-u", "--unrestricted",
		"-L", "--follow",
	}
	for _, flag := range accepted {
		t.Run(flag, func(t *testing.T) {
			res, _ := parse(t, []string{flag, "foo"}, os.Stat)
			if res.Kind != cli.KindSearch {
				t.Fatalf("Parse(%q).Kind = %v, want KindSearch (diagnostic %q)",
					[]string{flag, "foo"}, res.Kind, res.Diagnostic)
			}
			want := []string{"--json", "--no-config", flag, "--", "foo", "."}
			if !slices.Equal(res.Args, want) {
				t.Fatalf("Parse(%q).Args = %q, want %q", []string{flag, "foo"}, res.Args, want)
			}
		})
	}

	rejected := []string{
		"-e", "-t", "-m", "-f", "-g", "-z", "-9",
		"--type", "--type=go", "--glob", "--glob=x", "--sort", "--max-depth",
		"--vimgrep", "--no", "--i", "---",
		"-ifoo", "-Sz",
	}
	for _, flag := range rejected {
		t.Run("reject "+flag, func(t *testing.T) {
			res, _ := parse(t, []string{flag, "foo"}, failStat(t))
			if res.Kind != cli.KindUsageError || res.ErrorKind != cli.ErrUnsupportedOption {
				t.Fatalf("Parse(%q) = kind %v err %v, want unsupported-option usage error",
					[]string{flag, "foo"}, res.Kind, res.ErrorKind)
			}
		})
	}
}

// The child argument vector is exactly
// "--json --no-config <ordered expanded user flags> -- <pattern> <root>".
// Forwarding order and spellings come from the ordered scan records:
// encounter order survives for repeated, mixed-alias, combined, and
// operand-interleaved flags, and contradictory flags are never
// normalized.
func TestChildArgv(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{"no flags", []string{"foo"}, []string{"--json", "--no-config", "--", "foo", "."}},
		{"repeated shorts", []string{"-i", "-s", "-i", "foo"},
			[]string{"--json", "--no-config", "-i", "-s", "-i", "--", "foo", "."}},
		{"combined shorts expand in order", []string{"-isi", "foo"},
			[]string{"--json", "--no-config", "-i", "-s", "-i", "--", "foo", "."}},
		{"combined mixed case", []string{"-iwF", "foo"},
			[]string{"--json", "--no-config", "-i", "-w", "-F", "--", "foo", "."}},
		{"mixed spellings", []string{"--ignore-case", "-s", "-i", "foo"},
			[]string{"--json", "--no-config", "--ignore-case", "-s", "-i", "--", "foo", "."}},
		{"options interleaved with operands", []string{"foo", "-i", dir, "-s"},
			[]string{"--json", "--no-config", "-i", "-s", "--", "foo", dir}},
		{"option after both operands", []string{"foo", dir, "--hidden"},
			[]string{"--json", "--no-config", "--hidden", "--", "foo", dir}},
		{"contradictory flags not normalized", []string{"-i", "-s", "-S", "foo"},
			[]string{"--json", "--no-config", "-i", "-s", "-S", "--", "foo", "."}},
		{"unrestricted combined", []string{"-uu", "foo"},
			[]string{"--json", "--no-config", "-u", "-u", "--", "foo", "."}},
		{"unrestricted mixed aliases", []string{"-u", "--unrestricted", "foo"},
			[]string{"--json", "--no-config", "-u", "--unrestricted", "--", "foo", "."}},
		{"unrestricted inside combined", []string{"-iu", "foo"},
			[]string{"--json", "--no-config", "-i", "-u", "--", "foo", "."}},
		{"empty pattern verbatim", []string{"", "."},
			[]string{"--json", "--no-config", "--", "", "."}},
		{"literal dash pattern", []string{"-", "."},
			[]string{"--json", "--no-config", "--", "-", "."}},
		{"protected dash-leading pattern", []string{"--", "-foo"},
			[]string{"--json", "--no-config", "--", "-foo", "."}},
		{"literal -- pattern", []string{"--", "--"},
			[]string{"--json", "--no-config", "--", "--", "."}},
		{"literal -- pattern explicit root", []string{"--", "--", "."},
			[]string{"--json", "--no-config", "--", "--", "."}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, out := parse(t, tc.args, os.Stat)
			if res.Kind != cli.KindSearch {
				t.Fatalf("Parse(%q).Kind = %v, want KindSearch (diagnostic %q)", tc.args, res.Kind, res.Diagnostic)
			}
			if !slices.Equal(res.Args, tc.want) {
				t.Fatalf("Parse(%q).Args = %q, want %q", tc.args, res.Args, tc.want)
			}
			if out != "" {
				t.Fatalf("Parse(%q) wrote %q to the help writer on a search path", tc.args, out)
			}
		})
	}
}

// Unrestricted occurrences count cumulatively across every token and
// alias mix — separate, combined, short, and long spellings — from the
// ordered scan records. Two are accepted; the third is a sanitized
// usage error with no root validation and no child argv.
func TestUnrestrictedLimit(t *testing.T) {
	accepted := [][]string{
		{"-u", "foo"},
		{"--unrestricted", "foo"},
		{"-uu", "foo"},
		{"-iu", "foo"},
		{"-iuu", "foo"},
		{"-u", "--unrestricted", "foo"},
		{"--unrestricted", "-u", "foo"},
		{"--unrestricted", "--unrestricted", "foo"},
	}
	for _, args := range accepted {
		res, _ := parse(t, args, os.Stat)
		if res.Kind != cli.KindSearch {
			t.Fatalf("Parse(%q).Kind = %v, want KindSearch (diagnostic %q)", args, res.Kind, res.Diagnostic)
		}
	}

	rejected := [][]string{
		{"-uuu", "foo"},
		{"-u", "-u", "-u", "foo"},
		{"-u", "-uu", "foo"},
		{"-iuuu", "foo"},
		{"-u", "--unrestricted", "-u", "foo"},
		{"--unrestricted", "-uu", "foo"},
		{"-u", "foo", "-uu"},
	}
	for _, args := range rejected {
		res, out := parse(t, args, failStat(t))
		if res.Kind != cli.KindUsageError {
			t.Fatalf("Parse(%q).Kind = %v, want KindUsageError", args, res.Kind)
		}
		assertSanitizedLine(t, res.Diagnostic)
		if !strings.Contains(res.Diagnostic, "unrestricted") {
			t.Fatalf("Parse(%q) diagnostic %q does not name the unrestricted option", args, res.Diagnostic)
		}
		if len(res.Args) != 0 {
			t.Fatalf("Parse(%q) carried child argv %q on a usage error", args, res.Args)
		}
		if out != "" {
			t.Fatalf("Parse(%q) wrote %q to the help writer on a usage error", args, out)
		}
	}

	// The first usage error in argv order wins when both kinds appear.
	res, _ := parse(t, []string{"-uuu", "-e", "foo"}, failStat(t))
	if res.Kind != cli.KindUsageError || !strings.Contains(res.Diagnostic, "unrestricted") {
		t.Fatalf(`Parse(["-uuu", "-e", "foo"]) diagnostic %q does not report the earlier excess`, res.Diagnostic)
	}
	res, _ = parse(t, []string{"-e", "-uuu", "foo"}, failStat(t))
	if res.Kind != cli.KindUsageError || res.ErrorKind != cli.ErrUnsupportedOption {
		t.Fatalf(`Parse(["-e", "-uuu", "foo"]) = kind %v err %v, want unsupported-option usage error`,
			res.Kind, res.ErrorKind)
	}
}

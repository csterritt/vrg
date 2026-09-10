package cli_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
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

// Assignment spellings that disable help are not help requests. They must
// not produce the help-only result; the eventual exit status is Issue 2's
// to change, so only the not-help contract is pinned.
func TestHelpAssignmentSpellingsAreNotHelpRequests(t *testing.T) {
	for _, arg := range []string{"--help=false", "-h=false"} {
		t.Run(arg, func(t *testing.T) {
			var calls []string
			res, out := parse(t, []string{"foo", arg}, recordStat(&calls))
			if res.Kind == cli.KindHelp {
				t.Fatalf("Parse(%q) produced the help-only result for a help-disabling spelling", []string{"foo", arg})
			}
			if strings.Contains(out, "Usage:") {
				t.Fatalf("help text emitted for %q: %q", arg, out)
			}
			// The normal parse path ran: root validation was reached for the
			// defaulted root, which a help path would have skipped.
			if len(calls) == 0 {
				t.Fatalf("root validation never ran for %q; invocation was treated as help-only", arg)
			}
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
		{"unsupported short option", []string{"-x"}, cli.KindUsageError, cli.ErrUnsupportedOption, "", ""},
		{"unsupported option after pattern", []string{"foo", "-x"}, cli.KindUsageError, cli.ErrUnsupportedOption, "", ""},
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
		{"foo", "-x"},
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

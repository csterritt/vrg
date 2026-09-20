// Package cli owns command-line parsing and generated help for vrg.
//
// Parsing is delegated to github.com/jawher/mow.cli behind this package's
// interface; callers see only explicit result kinds, never library types.
// The package renders help itself from the same declarations that
// configure the library, and an ordered raw-token preflight resolves every
// help request and validates every token before the library runs, so no
// library emission path (first-token help, parse-failure diagnostics) can
// ever fire.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	mowcli "github.com/jawher/mow.cli"

	"vrg/internal/safepresentation"
)

// appName is the fixed executable name used for the mow.cli application so
// that os.Args[0] can never reach generated output as a substitution.
const appName = "vrg"

const appDesc = "Search with ripgrep and browse the results in a terminal UI."

// Kind identifies which contract a Parse result follows.
type Kind int

const (
	// KindHelp is the help-only result: generated help was written to the
	// writer supplied to Parse. No root validation, child startup, TUI
	// initialization, or search work happened.
	KindHelp Kind = iota
	// KindSearch is a validated search invocation; Pattern and Root are set.
	KindSearch
	// KindUsageError is a classified usage error; Diagnostic is a sanitized
	// single line destined for stderr.
	KindUsageError
)

// ErrorKind classifies the cause of a KindUsageError result.
type ErrorKind int

const (
	// ErrMissingPattern means no positional pattern operand was supplied
	// outside a help-only path.
	ErrMissingPattern ErrorKind = iota
	// ErrExcessOperand means more than two positional operands were
	// supplied.
	ErrExcessOperand
	// ErrUnsupportedOption means a pre-terminator dash token is not a
	// supported option spelling.
	ErrUnsupportedOption
	// ErrExcessUnrestricted means the cumulative -u/--unrestricted count
	// across all spellings exceeded the limit of two.
	ErrExcessUnrestricted
	// ErrInvalidRoot means the root operand is not an existing directory
	// or regular file.
	ErrInvalidRoot
)

// Result is the CLI module's output contract. Exactly one interpretation
// applies per Kind; the entry point alone maps kinds to streams and exit
// statuses.
type Result struct {
	Kind Kind
	// Pattern, Root, and Args are set only for KindSearch. Root is the
	// validated operand, "." when omitted. Args is the exact protected
	// argument vector for the rg child, excluding the program name:
	// "--json", "--no-config", the user's flags in encounter order with
	// the supplied spellings preserved, "--", pattern, root.
	Pattern string
	Root    string
	Args    []string
	// ErrorKind and Diagnostic are set only for KindUsageError. Diagnostic
	// is a sanitized single line without a trailing newline.
	ErrorKind  ErrorKind
	Diagnostic string
}

// Env injects the environment dependencies Parse needs.
type Env struct {
	// Stat validates the root operand; nil means os.Stat. A stat that
	// follows symlinks (as os.Stat does) accepts links to directories and
	// regular files.
	Stat func(name string) (fs.FileInfo, error)
}

// optionDecl describes one command-line option. optionDecls is the single
// declaration source for mow.cli configuration, raw-token recognition in
// the preflight scan, and generated help.
type optionDecl struct {
	short byte   // short option letter, as in -h; 0 for long-only
	long  string // long option name, as in --help; "" for short-only
	help  bool   // local help option: its spellings are help requests
	// unrestricted marks the options whose occurrences count toward the
	// cumulative limit of two; only -u/--unrestricted carries it.
	unrestricted bool
	desc         string
}

// argDecl describes one positional argument for the parser spec and
// generated help.
type argDecl struct {
	name       string
	optional   bool
	defaultVal string // shown in help and used as the declared default
	desc       string
}

var optionDecls = []optionDecl{
	{short: 'h', long: "help", help: true, desc: "Show command-line help and exit."},
	{short: 'i', long: "ignore-case", desc: "Search case-insensitively."},
	{short: 'S', long: "smart-case", desc: "Search case-insensitively unless the pattern contains uppercase."},
	{short: 's', long: "case-sensitive", desc: "Search case-sensitively."},
	{short: 'w', long: "word-regexp", desc: "Match the pattern against whole words only."},
	{short: 'x', long: "line-regexp", desc: "Match the pattern against whole lines only."},
	{short: 'F', long: "fixed-strings", desc: "Treat the pattern as a literal string."},
	{long: "hidden", desc: "Search hidden files and directories."},
	{long: "no-hidden", desc: "Do not search hidden files and directories."},
	{long: "no-ignore", desc: "Do not respect ignore files."},
	{short: 'u', long: "unrestricted", unrestricted: true, desc: "Reduce ignore restrictions; may be given at most twice."},
	{short: 'L', long: "follow", desc: "Follow symbolic links while searching."},
}

var argDecls = []argDecl{
	{name: "PATTERN", desc: "The ripgrep search pattern."},
	{name: "ROOT", optional: true, defaultVal: ".", desc: "Search root: a directory or regular file."},
}

// name renders the option's mow.cli declaration name, e.g. "h help".
func (d optionDecl) name() string {
	parts := make([]string, 0, 2)
	if d.short != 0 {
		parts = append(parts, string(d.short))
	}
	if d.long != "" {
		parts = append(parts, d.long)
	}
	return strings.Join(parts, " ")
}

// spec builds the mow.cli spec string from the argument declarations,
// permitting options before, between, and after positionals:
// "[OPTIONS] PATTERN [OPTIONS] [ROOT] [OPTIONS]".
func spec() string {
	var b strings.Builder
	b.WriteString("[OPTIONS]")
	for _, a := range argDecls {
		name := a.name
		if a.optional {
			name = "[" + name + "]"
		}
		b.WriteString(" " + name + " [OPTIONS]")
	}
	return b.String()
}

// newApp constructs the mow.cli application behind the adapter boundary.
// flag.ContinueOnError keeps exit-status ownership in cmd/vrg: the library
// returns errors instead of calling os.Exit.
func newApp() *mowcli.Cli {
	app := mowcli.App(appName, appDesc)
	app.ErrorHandling = flag.ContinueOnError
	return app
}

// Parse classifies args and, for KindSearch, carries the validated
// invocation. Generated help is written to out on KindHelp results;
// nothing is written for other kinds and no library output can reach the
// process streams. All exit-status and stderr decisions belong to the
// caller.
func Parse(args []string, out io.Writer, env Env) Result {
	if len(args) == 0 {
		io.WriteString(out, safepresentation.EscapeDiagnostic(renderHelp()))
		return Result{Kind: KindHelp}
	}

	p := scanArgs(args)
	if p.help {
		io.WriteString(out, safepresentation.EscapeDiagnostic(renderHelp()))
		return Result{Kind: KindHelp}
	}
	// Scan errors are reported in argv order.
	if p.badOption != "" && (p.excessU == "" || p.badIndex < p.excessIndex) {
		return usageErrorf(ErrUnsupportedOption, "unsupported option %s", safepresentation.EscapePath([]byte(p.badOption)))
	}
	if p.excessU != "" {
		return usageErrorf(ErrExcessUnrestricted,
			"too many unrestricted options at %s: at most two -u/--unrestricted are allowed", safepresentation.EscapePath([]byte(p.excessU)))
	}
	switch {
	case len(p.positionals) == 0:
		return usageErrorf(ErrMissingPattern, "missing required argument PATTERN")
	case len(p.positionals) > 2:
		return usageErrorf(ErrExcessOperand, "unexpected extra operand %s", safepresentation.EscapePath([]byte(p.positionals[2])))
	}

	// The preflight has resolved every help request and rejected every
	// token the library could fail on, so Run can neither emit nor error;
	// only the declared syntax work remains for it. The parsed values are
	// not used for forwarding: encounter order and cumulative counts come
	// solely from the ordered scan records, which the library cannot
	// preserve.
	var pattern, root string
	values := make([]bool, len(optionDecls))
	app := newApp()
	app.Spec = spec()
	for i, d := range optionDecls {
		app.BoolOptPtr(&values[i], d.name(), false, d.desc)
	}
	app.StringArgPtr(&pattern, "PATTERN", "", argDecls[0].desc)
	app.StringArgPtr(&root, "ROOT", argDecls[1].defaultVal, argDecls[1].desc)
	// Action must be non-nil: with a nil action the library prints help.
	app.Action = func() {}
	if err := app.Run(append([]string{appName}, args...)); err != nil {
		// Unreachable while the preflight stays aligned with the declared
		// grammar; never surface the library's own message.
		return usageErrorf(ErrUnsupportedOption, "invalid arguments")
	}
	// Defensive seam: every spelling the library could use to set a help
	// value is already a help request or a rejected assignment in the
	// preflight, so this cannot fire while the two stay aligned. If it
	// ever did, the help-only result still wins before root validation
	// or search work.
	for i, d := range optionDecls {
		if d.help && values[i] {
			io.WriteString(out, safepresentation.EscapeDiagnostic(renderHelp()))
			return Result{Kind: KindHelp}
		}
	}

	stat := env.Stat
	if stat == nil {
		stat = os.Stat
	}
	if res, bad := checkRoot(stat, root); bad {
		return res
	}
	return Result{Kind: KindSearch, Pattern: pattern, Root: root, Args: childArgs(p.flags, pattern, root)}
}

// childArgs builds the protected rg argument vector from the ordered scan
// records: the mandatory internal flags, then each user-supplied flag in
// encounter order, then the operands behind the -- terminator.
func childArgs(flags []scanFlag, pattern, root string) []string {
	args := make([]string, 0, len(flags)+5)
	args = append(args, "--json", "--no-config")
	for _, f := range flags {
		args = append(args, f.arg)
	}
	return append(args, "--", pattern, root)
}

// checkRoot validates the parsed root operand: an existing directory or
// regular file, including symlinks resolving to either. "-" is stdin and
// always rejected; a real file named "-" is addressed as "./-".
func checkRoot(stat func(string) (fs.FileInfo, error), root string) (Result, bool) {
	if root == "-" {
		return usageErrorf(ErrInvalidRoot, `invalid root "-": standard input is not a searchable root`), true
	}
	fi, err := stat(root)
	if err != nil {
		reason := "cannot be accessed"
		if errors.Is(err, fs.ErrNotExist) {
			reason = "does not exist"
		}
		return usageErrorf(ErrInvalidRoot, "invalid root %s: %s", safepresentation.EscapePath([]byte(root)), reason), true
	}
	if !fi.IsDir() && !fi.Mode().IsRegular() {
		return usageErrorf(ErrInvalidRoot, "invalid root %s: not a directory or regular file", safepresentation.EscapePath([]byte(root))), true
	}
	return Result{}, false
}

func usageErrorf(kind ErrorKind, format string, args ...any) Result {
	return Result{
		Kind:       KindUsageError,
		ErrorKind:  kind,
		Diagnostic: appName + ": " + fmt.Sprintf(format, args...),
	}
}

// preflight is the product of the single ordered raw-token scan over argv.
type preflight struct {
	help      bool   // a help request appeared before the first --
	badOption string // first unsupported option token, "" if none
	badIndex  int    // argv index of badOption
	// excessU is the first token carrying a third unrestricted occurrence,
	// "" if the cumulative count stayed within the limit of two.
	excessU     string
	excessIndex int        // argv index of excessU
	flags       []scanFlag // accepted search flags in encounter order
	positionals []string   // positional operands in order
}

// scanFlag records one accepted search-flag occurrence: arg is the exact
// child argv element preserving the supplied spelling, decl identifies the
// option for cumulative counting.
type scanFlag struct {
	arg  string
	decl *optionDecl
}

// scanArgs scans argv left to right, stopping option recognition at the
// first "--" (later tokens are all positional, including -h, --help, and
// another --). A help request wins over every other classification and
// ends the scan. Every other pre-terminator dash token is classified
// against the shared declarations: accepted spellings are recorded in
// encounter order, combined shorts expand left to right, "=" assignment
// forms are rejected outright, and unrestricted occurrences accumulate
// toward their limit.
func scanArgs(args []string) (p preflight) {
	positional := false
	unrestricted := 0
	for i, tok := range args {
		switch {
		case positional:
			p.positionals = append(p.positionals, tok)
		case tok == "--":
			positional = true
		case isHelpToken(tok):
			p.help = true
			return p
		case !isOptionToken(tok):
			p.positionals = append(p.positionals, tok)
		default:
			flags, ok := expandOptionToken(tok)
			if !ok {
				if p.badOption == "" {
					p.badOption, p.badIndex = tok, i
				}
				continue
			}
			for _, f := range flags {
				if f.decl.unrestricted {
					unrestricted++
					if unrestricted > 2 && p.excessU == "" {
						p.excessU, p.excessIndex = tok, i
					}
				}
				p.flags = append(p.flags, f)
			}
		}
	}
	return p
}

// expandOptionToken classifies a pre-terminator option token — not "-",
// "--", or a help request — into the ordered flag records it contributes.
// ok is false when the token is not an exact spelling of a declared
// no-argument option: unknown names, undeclared letters in a combined
// token, and every "=" assignment form all reject the whole token.
func expandOptionToken(tok string) (flags []scanFlag, ok bool) {
	if strings.HasPrefix(tok, "--") {
		for i := range optionDecls {
			if d := &optionDecls[i]; !d.help && d.long != "" && tok == "--"+d.long {
				return []scanFlag{{arg: tok, decl: d}}, true
			}
		}
		return nil, false
	}
	if len(tok) == 2 {
		if d := shortDecl(tok[1]); d != nil {
			return []scanFlag{{arg: tok, decl: d}}, true
		}
		return nil, false
	}
	// A combined short token expands left to right; each letter becomes
	// its own child argv element with the short spelling.
	for i := 1; i < len(tok); i++ {
		d := shortDecl(tok[i])
		if d == nil {
			return nil, false
		}
		flags = append(flags, scanFlag{arg: "-" + string(tok[i]), decl: d})
	}
	return flags, true
}

// shortDecl returns the declaration for a non-help option's short letter,
// or nil when no declared search flag uses it.
func shortDecl(c byte) *optionDecl {
	for i := range optionDecls {
		if d := &optionDecls[i]; !d.help && d.short == c {
			return d
		}
	}
	return nil
}

// isHelpToken reports whether tok is a local help request: a literal
// spelling of a declared help option, or a combined short token of ASCII
// letters containing the help option's short letter. Assignment spellings
// such as -h=false and --help=false are not help requests.
func isHelpToken(tok string) bool {
	for _, d := range optionDecls {
		if !d.help {
			continue
		}
		if d.short != 0 && tok == "-"+string(d.short) {
			return true
		}
		if d.long != "" && tok == "--"+d.long {
			return true
		}
		if d.short == 0 || len(tok) < 3 || tok[0] != '-' || tok[1] == '-' {
			continue
		}
		has := false
		letters := true
		for i := 1; i < len(tok); i++ {
			c := tok[i]
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
				letters = false
				break
			}
			has = has || c == d.short
		}
		if letters && has {
			return true
		}
	}
	return false
}

// isOptionToken reports whether tok is an option-position token: a dash
// token other than the lone "-" (a positional) or the "--" terminator.
func isOptionToken(tok string) bool {
	return strings.HasPrefix(tok, "-") && tok != "-" && tok != "--"
}

// HelpText returns the generated command-line help — the same text Parse
// writes to the help writer for KindHelp results, rendered through the
// shared safe-presentation utility. The entry point appends it to
// usage-error diagnostics on stderr.
func HelpText() string {
	return safepresentation.EscapeDiagnostic(renderHelp())
}

// renderHelp generates the command-line help from the shared
// declarations. It contains no external substitutions: the application
// name and every description string are fixed. The raw text still passes
// through safepresentation.EscapeDiagnostic at emission, so the stdout
// sink's safety contract does not depend on the declarations staying
// fixed.
func renderHelp() string {
	var b strings.Builder
	b.WriteString("Usage: " + appName + " [OPTIONS]")
	for _, a := range argDecls {
		name := a.name
		if a.optional {
			name = "[" + name + "]"
		}
		b.WriteString(" " + name)
	}
	b.WriteString("\n\n" + appDesc + "\n\n")

	b.WriteString("Arguments:\n")
	for _, a := range argDecls {
		line := "  " + a.name + "\t" + a.desc
		if a.defaultVal != "" {
			line += " (default \"" + a.defaultVal + "\")"
		}
		b.WriteString(line + "\n")
	}

	b.WriteString("\nOptions:\n")
	for _, d := range optionDecls {
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
		b.WriteString("  " + names + "\t" + d.desc + "\n")
	}
	return b.String()
}

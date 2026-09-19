// Package cli owns command-line parsing and generated help for vrg.
//
// Parsing is delegated to github.com/jawher/mow.cli behind this package's
// interface; callers see only explicit result kinds, never library types.
// The package renders help itself from the same declarations that
// configure the library, and an ordered raw-token preflight resolves every
// help request, validates every token, and records accepted search-flag
// spellings in encounter order before the library runs, so no library
// emission path (first-token help, parse-failure diagnostics) can ever
// fire and no information the library cannot preserve (cross-option
// order, exact spellings, cumulative counts) is ever needed from it.
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
	// ErrInvalidRoot means the root operand is not an existing directory
	// or regular file.
	ErrInvalidRoot
	// ErrExcessUnrestricted means the cumulative -u/--unrestricted count
	// across all spellings exceeded two occurrences.
	ErrExcessUnrestricted
)

// Result is the CLI module's output contract. Exactly one interpretation
// applies per Kind; the entry point alone maps kinds to streams and exit
// statuses.
type Result struct {
	Kind Kind
	// Pattern, Root, and ChildArgs are set only for KindSearch. Root is
	// the validated operand, "." when omitted. ChildArgs is the protected
	// ripgrep argument vector (excluding the "rg" program name): the
	// mandatory internal flags --json --no-config, the user flags in
	// encounter order and supplied spelling, then --, the pattern, and
	// the root — all forwarded verbatim.
	Pattern   string
	Root      string
	ChildArgs []string
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
// the preflight scan, ordered search-flag records for the child argv, and
// generated help — no independently maintained allow-list exists.
type optionDecl struct {
	short        byte   // short option letter, as in -h; 0 for long-only
	long         string // long option name, as in --help; "" for short-only
	help         bool   // local help option: its spellings are help requests, never forwarded, never allow-list-rejected
	unrestricted bool   // occurrences count toward the two-occurrence unrestricted cap
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

// optionDecls lists every accepted command-line spelling: the local help
// option plus the allow-listed no-argument ripgrep search flags. Every
// other pre-terminator option token — including -e, argument-taking
// options, and = assignment spellings — is a usage error.
var optionDecls = []optionDecl{
	{short: 'h', long: "help", help: true, desc: "Show command-line help and exit."},
	{short: 'i', long: "ignore-case", desc: "Case-insensitive search."},
	{short: 'S', long: "smart-case", desc: "Smart case search."},
	{short: 's', long: "case-sensitive", desc: "Case-sensitive search."},
	{short: 'w', long: "word-regexp", desc: "Match whole words only."},
	{short: 'x', long: "line-regexp", desc: "Match whole lines only."},
	{short: 'F', long: "fixed-strings", desc: "Treat the pattern as a literal string."},
	{long: "hidden", desc: "Search hidden files and directories."},
	{long: "no-hidden", desc: "Do not search hidden files and directories."},
	{long: "no-ignore", desc: "Do not respect ignore files."},
	{short: 'u', long: "unrestricted", unrestricted: true, desc: "Reduce filtering; may be supplied at most twice."},
	{short: 'L', long: "follow", desc: "Follow symbolic links."},
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
		io.WriteString(out, renderHelp())
		return Result{Kind: KindHelp}
	}

	p := scanArgs(args)
	if p.help {
		io.WriteString(out, renderHelp())
		return Result{Kind: KindHelp}
	}
	// The scan reports the first problem in argv order; only one of these
	// is ever set.
	if p.excessUnrestricted {
		return usageErrorf(ErrExcessUnrestricted, "too many unrestricted options: -u/--unrestricted may appear at most twice")
	}
	if p.badOption != "" {
		return usageErrorf(ErrUnsupportedOption, "unsupported option %s", safepresentation.EscapePath([]byte(p.badOption)))
	}
	switch {
	case len(p.positionals) == 0:
		return usageErrorf(ErrMissingPattern, "missing required argument PATTERN")
	case len(p.positionals) > 2:
		return usageErrorf(ErrExcessOperand, "unexpected extra operand %s", safepresentation.EscapePath([]byte(p.positionals[2])))
	}

	// The preflight has resolved every help request and rejected every
	// token the library could fail on, so Run can neither emit nor error;
	// only the declared syntax work remains for it.
	var helpOpt bool
	var pattern, root string
	app := newApp()
	app.Spec = spec()
	for _, d := range optionDecls {
		// The help option keeps a bound value as defense in depth; the
		// search flags are declared so the library accepts and parses
		// them, but their values are never consulted — the ordered scan
		// records alone drive forwarding.
		if d.help {
			app.BoolOptPtr(&helpOpt, d.name(), false, d.desc)
		} else {
			app.BoolOpt(d.name(), false, d.desc)
		}
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
	// A parsed local-help value yields the help-only result before root
	// validation or any search work. Unreachable while the scan rejects
	// every assignment spelling and intercepts every help token; kept as
	// a safety net for the adapter boundary.
	if helpOpt {
		io.WriteString(out, renderHelp())
		return Result{Kind: KindHelp}
	}

	stat := env.Stat
	if stat == nil {
		stat = os.Stat
	}
	if res, bad := checkRoot(stat, root); bad {
		return res
	}
	child := make([]string, 0, len(p.flags)+5)
	child = append(child, "--json", "--no-config")
	child = append(child, p.flags...)
	child = append(child, "--", pattern, root)
	return Result{Kind: KindSearch, Pattern: pattern, Root: root, ChildArgs: child}
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

// usageErrorf builds a classified usage error. Operands arrive already
// single-line-escaped through the shared utility; the composed
// diagnostic passes through it too, so the sink-facing contract —
// sanitized, single-line — holds whatever the format carries.
func usageErrorf(kind ErrorKind, format string, args ...any) Result {
	return Result{
		Kind:       KindUsageError,
		ErrorKind:  kind,
		Diagnostic: safepresentation.EscapeDiagnostic(appName + ": " + fmt.Sprintf(format, args...)),
	}
}

// preflight is the product of the single ordered raw-token scan over argv.
type preflight struct {
	help               bool     // a help request appeared before the first --
	badOption          string   // first unsupported option token, "" if none
	excessUnrestricted bool     // a third -u/--unrestricted occurrence was scanned
	unrestricted       int      // cumulative -u/--unrestricted count across all spellings
	positionals        []string // positional operands in order
	flags              []string // accepted search-flag spellings in encounter order
}

// scanArgs scans argv left to right, stopping option recognition at the
// first "--" (later tokens are all positional, including -h, --help, and
// another --). A help request wins over every other classification and
// ends the scan. Accepted search flags are recorded in encounter order —
// the scan supplies what library value assignment provably cannot
// preserve (cross-option order, exact spellings, cumulative counts), so
// forwarding never reads parser output for the flags.
func scanArgs(args []string) (p preflight) {
	positional := false
	for _, tok := range args {
		switch {
		case positional:
			p.positionals = append(p.positionals, tok)
		case tok == "--":
			positional = true
		case isHelpToken(tok):
			p.help = true
			return p
		case isOptionToken(tok):
			p.scanOption(tok)
		default:
			p.positionals = append(p.positionals, tok)
		}
	}
	return p
}

// scanOption validates one pre-terminator option token against the shared
// declarations and records its accepted spellings in encounter order.
// Long tokens keep the supplied spelling; combined short tokens expand
// left to right into canonical single-letter spellings. The no-argument
// contract rejects every = assignment spelling lexically, before the
// library's permissive boolean parsing can see it. The first problem in
// argv order claims the usage-error slot; scanning continues so a later
// help request still takes precedence.
func (p *preflight) scanOption(tok string) {
	reject := func() {
		if p.badOption == "" && !p.excessUnrestricted {
			p.badOption = tok
		}
	}
	if strings.Contains(tok, "=") {
		reject()
		return
	}
	var spellings []string
	if strings.HasPrefix(tok, "--") {
		if d := longDecl(tok[2:]); d != nil && !d.help {
			spellings = []string{tok}
			if d.unrestricted {
				p.noteUnrestricted()
			}
		} else {
			reject()
			return
		}
	} else {
		for i := 1; i < len(tok); i++ {
			d := shortDecl(tok[i])
			if d == nil || d.help {
				reject()
				return
			}
			spellings = append(spellings, "-"+string(tok[i]))
			if d.unrestricted {
				p.noteUnrestricted()
			}
		}
	}
	p.flags = append(p.flags, spellings...)
}

// noteUnrestricted records one -u/--unrestricted occurrence; the third
// claims the usage-error slot unless an earlier token already did.
func (p *preflight) noteUnrestricted() {
	p.unrestricted++
	if p.unrestricted > 2 && p.badOption == "" {
		p.excessUnrestricted = true
	}
}

// shortDecl returns the declaration for a single-letter option spelling,
// or nil when no declared option has that letter.
func shortDecl(c byte) *optionDecl {
	for i := range optionDecls {
		if optionDecls[i].short != 0 && optionDecls[i].short == c {
			return &optionDecls[i]
		}
	}
	return nil
}

// longDecl returns the declaration for a long option name (without the
// leading --), or nil when no declared option has that name.
func longDecl(name string) *optionDecl {
	for i := range optionDecls {
		if optionDecls[i].long != "" && optionDecls[i].long == name {
			return &optionDecls[i]
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
// writes to the help writer for KindHelp results. The entry point appends
// it to usage-error diagnostics on stderr.
func HelpText() string {
	return renderHelp()
}

// renderHelp generates the command-line help from the shared
// declarations. It contains no external substitutions: the application
// name and every description string are fixed. The generated text still
// passes through the shared utility like every sink — real line
// boundaries are preserved and the layout tabs expand to columns.
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
	return safepresentation.EscapeDiagnostic(b.String())
}

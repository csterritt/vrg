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
	"strconv"
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
)

// Result is the CLI module's output contract. Exactly one interpretation
// applies per Kind; the entry point alone maps kinds to streams and exit
// statuses.
type Result struct {
	Kind Kind
	// Pattern, Root, and ChildArgs are set only for KindSearch. Root is
	// the validated operand, "." when omitted. ChildArgs is the exact
	// ripgrep argument vector after the program name: the mandatory
	// internal flags, the user's flag spellings in encounter order, "--",
	// pattern, root.
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
// the preflight scan, and generated help.
type optionDecl struct {
	short  byte   // short option letter, as in -h; 0 for long-only
	long   string // long option name, as in --help; "" for short-only
	help   bool   // local help option: its spellings are help requests, never forwarded
	search bool   // allow-listed no-argument search flag: forwarded to the child argv
	// unrestricted counts toward the cumulative -u/--unrestricted limit:
	// two occurrences allowed, a third is a usage error.
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
	{short: 'i', long: "ignore-case", search: true, desc: "Case insensitive search."},
	{short: 'S', long: "smart-case", search: true, desc: "Case insensitive unless the pattern has uppercase."},
	{short: 's', long: "case-sensitive", search: true, desc: "Case sensitive search."},
	{short: 'w', long: "word-regexp", search: true, desc: "Match whole words only."},
	{short: 'x', long: "line-regexp", search: true, desc: "Match whole lines only."},
	{short: 'F', long: "fixed-strings", search: true, desc: "Treat the pattern as a literal string."},
	{long: "hidden", search: true, desc: "Search hidden files and directories."},
	{long: "no-hidden", search: true, desc: "Do not search hidden files and directories."},
	{long: "no-ignore", search: true, desc: "Do not respect ignore files."},
	{short: 'u', long: "unrestricted", search: true, unrestricted: true, desc: "Loosen search restrictions; may be given twice."},
	{short: 'L', long: "follow", search: true, desc: "Follow symbolic links."},
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
	if p.badOption != "" {
		return usageErrorf(ErrUnsupportedOption, "unsupported option %s", Escape(p.badOption))
	}
	if p.unrestricted > 2 {
		return usageErrorf(ErrUnsupportedOption, "option -u/--unrestricted may be used at most twice")
	}
	switch {
	case len(p.positionals) == 0:
		return usageErrorf(ErrMissingPattern, "missing required argument PATTERN")
	case len(p.positionals) > 2:
		return usageErrorf(ErrExcessOperand, "unexpected extra operand %s", Escape(p.positionals[2]))
	}

	// The preflight has resolved every help request and rejected every
	// token the library could fail on, so Run can neither emit nor error;
	// only the declared syntax work remains for it.
	var helpOpt bool
	var searchOpt bool
	var pattern, root string
	app := newApp()
	app.Spec = spec()
	for _, d := range optionDecls {
		// Search-flag values are never read: forwarding order, exact
		// spellings, and cumulative counts come solely from the ordered
		// scan records, so all of them share one sink.
		if d.help {
			app.BoolOptPtr(&helpOpt, d.name(), false, d.desc)
		} else {
			app.BoolOptPtr(&searchOpt, d.name(), false, d.desc)
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
	// A local help value set through a successful parse (an assignment
	// spelling such as --help=true) yields the same help-only result,
	// before root validation or any search work.
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
	childArgs := make([]string, 0, len(p.flags)+5)
	childArgs = append(childArgs, "--json", "--no-config")
	childArgs = append(childArgs, p.flags...)
	childArgs = append(childArgs, "--", pattern, root)
	return Result{Kind: KindSearch, Pattern: pattern, Root: root, ChildArgs: childArgs}
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
		return usageErrorf(ErrInvalidRoot, "invalid root %s: %s", Escape(root), reason), true
	}
	if !fi.IsDir() && !fi.Mode().IsRegular() {
		return usageErrorf(ErrInvalidRoot, "invalid root %s: not a directory or regular file", Escape(root)), true
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
	help         bool     // a help request appeared before the first --
	badOption    string   // first unsupported option token, "" if none
	flags        []string // accepted search-flag spellings in encounter order
	unrestricted int      // cumulative -u/--unrestricted occurrences
	positionals  []string // positional operands in order
}

// reject records the first unsupported option token; scanning continues
// because a later help request still wins over the error.
func (p *preflight) reject(tok string) {
	if p.badOption == "" {
		p.badOption = tok
	}
}

// record appends an accepted search-flag spelling and counts cumulative
// unrestricted occurrences.
func (p *preflight) record(d *optionDecl, spelling string) {
	p.flags = append(p.flags, spelling)
	if d.unrestricted {
		p.unrestricted++
	}
}

// scanArgs scans argv left to right, stopping option recognition at the
// first "--" (later tokens are all positional, including -h, --help, and
// another --). A help request wins over every other classification and
// ends the scan. Accepted search flags are recorded in encounter order
// with the exact spelling supplied — combined short tokens expand left to
// right — because library option values provably cannot preserve that
// order across different options.
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

// scanOption classifies one pre-terminator dash token that is neither the
// terminator, the lone "-", nor a help request. Accepted spellings are
// recorded verbatim; anything else is rejected lexically, including the
// = assignment forms the library would otherwise parse for boolean
// options — the allow-list is a no-argument contract.
func (p *preflight) scanOption(tok string) {
	if strings.HasPrefix(tok, "--") {
		name, value, assigned := strings.Cut(tok[2:], "=")
		d := longDecl(name)
		switch {
		case d == nil:
			p.reject(tok)
		case assigned:
			// The sole surviving assignment is a truthy value for the
			// local help option: the library parses it into the help
			// value. Every other = spelling is rejected lexically.
			if !d.help || !isTruthyBool(value) {
				p.reject(tok)
			}
		case !d.search:
			// Unreachable for --help: isHelpToken intercepts it first.
			p.reject(tok)
		default:
			p.record(d, tok)
		}
		return
	}
	// A lone short assignment form -x=value: only a truthy help value
	// passes through to the library parse; every other spelling is
	// rejected. An = anywhere else in the token fails the expansion below.
	if len(tok) > 2 && tok[2] == '=' {
		if d := shortDecl(tok[1]); d == nil || !d.help || !isTruthyBool(tok[3:]) {
			p.reject(tok)
		}
		return
	}
	// Combined or lone short token: expand left to right; every letter
	// must be a declared search flag.
	for i := 1; i < len(tok); i++ {
		d := shortDecl(tok[i])
		if d == nil || !d.search {
			p.reject(tok)
			return
		}
		p.record(d, "-"+string(tok[i]))
	}
}

// longDecl returns the declaration for a long option name, or nil.
func longDecl(name string) *optionDecl {
	for i := range optionDecls {
		if optionDecls[i].long != "" && optionDecls[i].long == name {
			return &optionDecls[i]
		}
	}
	return nil
}

// shortDecl returns the declaration for a short option letter, or nil.
func shortDecl(c byte) *optionDecl {
	for i := range optionDecls {
		if optionDecls[i].short != 0 && optionDecls[i].short == c {
			return &optionDecls[i]
		}
	}
	return nil
}

// isTruthyBool reports whether s is a boolean literal the library would
// parse as true; the help option's pass-through seam matches what the
// library accepts.
func isTruthyBool(s string) bool {
	v, err := strconv.ParseBool(s)
	return err == nil && v
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

// OptionDecl is the exported form of one command-line option declaration,
// for documentation tests (Issue #34) that assert the README and help
// footer list every allow-listed flag and local help option from the same
// shared declarations that configure mow.cli and validate the raw-token
// scan.
type OptionDecl struct {
	Short        byte   // short option letter, as in -h; 0 for long-only
	Long         string // long option name, as in --help; "" for short-only
	Help         bool   // local help option: never forwarded to ripgrep
	Search       bool   // allow-listed no-argument search flag
	Unrestricted bool   // counts toward the cumulative -u limit
	Desc         string
}

// OptionDecls returns the shared option declarations — the single source
// for mow.cli configuration, raw-token recognition, generated help, and
// Issue #34 documentation synchronization. Documentation tests iterate
// this slice to verify the README lists every allow-listed flag and
// local help option in short and long form with the exact no-argument
// spellings.
func OptionDecls() []OptionDecl {
	out := make([]OptionDecl, 0, len(optionDecls))
	for _, d := range optionDecls {
		out = append(out, OptionDecl{
			Short:        d.short,
			Long:         d.long,
			Help:         d.help,
			Search:       d.search,
			Unrestricted: d.unrestricted,
			Desc:         d.desc,
		})
	}
	return out
}

// ArgDecls returns the shared positional argument declarations — the
// single source for the parser spec, generated help, and Issue #34
// documentation synchronization.
type ArgDecl struct {
	Name       string
	Optional   bool
	DefaultVal string
	Desc       string
}

// ArgDecls returns the shared positional argument declarations.
func ArgDecls() []ArgDecl {
	out := make([]ArgDecl, 0, len(argDecls))
	for _, a := range argDecls {
		out = append(out, ArgDecl{
			Name:       a.name,
			Optional:   a.optional,
			DefaultVal: a.defaultVal,
			Desc:       a.desc,
		})
	}
	return out
}

// renderHelp generates the command-line help from the shared declarations.
// It contains no external substitutions: the application name and every
// description string are fixed.
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
		b.WriteString("  " + d.spellings() + "\t" + d.desc + "\n")
	}
	return b.String()
}

// spellings renders the option's user-facing forms for generated help,
// e.g. "-h, --help" or "--hidden".
func (d optionDecl) spellings() string {
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
	return names
}

// Escape renders external data safe for a single-line diagnostic or stub
// substitution. It delegates to safepresentation.EscapePath, the shared
// safe-presentation utility for path-style single-line escaping:
// backslashes double; newline, carriage return, and tab become \n, \r,
// \t; other C0 controls and DEL use caret notation; C1 controls use \u
// escapes; invalid UTF-8 bytes use \xNN. Printable text passes through.
func Escape(s string) string {
	return safepresentation.EscapePath([]byte(s)).Text
}

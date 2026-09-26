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
	"unicode/utf8"

	mowcli "github.com/jawher/mow.cli"
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
	// ErrExcessUnrestricted means more than two cumulative unrestricted
	// (-u/--unrestricted) occurrences were supplied before the first --.
	ErrExcessUnrestricted
)

// Result is the CLI module's output contract. Exactly one interpretation
// applies per Kind; the entry point alone maps kinds to streams and exit
// statuses.
type Result struct {
	Kind Kind
	// Pattern, Root, and ChildArgv are set only for KindSearch. Root is
	// the validated operand, "." when omitted. ChildArgv is the exact
	// argument vector for the rg child process, argv[0] excluded:
	// "--json", "--no-config", the user's flags in encounter order with
	// the spellings supplied, "--", the pattern, and the root.
	Pattern   string
	Root      string
	ChildArgv []string
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
	short        byte   // short option letter, as in -h; 0 for long-only
	long         string // long option name, as in --help; "" for short-only
	help         bool   // local help option: its spellings are help requests
	forward      bool   // allow-listed search flag forwarded to the child argv
	unrestricted bool   // occurrences count toward the cumulative two-flag cap
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
	{short: 'i', long: "ignore-case", forward: true, desc: "Case-insensitive search."},
	{short: 'S', long: "smart-case", forward: true, desc: "Case-insensitive unless the pattern contains uppercase."},
	{short: 's', long: "case-sensitive", forward: true, desc: "Case-sensitive search."},
	{short: 'w', long: "word-regexp", forward: true, desc: "Match whole words only."},
	{short: 'x', long: "line-regexp", forward: true, desc: "Match whole lines only."},
	{short: 'F', long: "fixed-strings", forward: true, desc: "Treat the pattern as a literal string."},
	{long: "hidden", forward: true, desc: "Search hidden files and directories."},
	{long: "no-hidden", forward: true, desc: "Do not search hidden files and directories."},
	{long: "no-ignore", forward: true, desc: "Do not respect ignore files."},
	{short: 'u', long: "unrestricted", forward: true, unrestricted: true, desc: "Reduce ignore-file and hidden-file filtering; may be given at most twice."},
	{short: 'L', long: "follow", forward: true, desc: "Follow symbolic links."},
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
	if p.errTok != "" {
		if p.errKind == ErrExcessUnrestricted {
			return usageErrorf(ErrExcessUnrestricted, "unrestricted option given more than twice: %s", Escape(p.errTok))
		}
		return usageErrorf(ErrUnsupportedOption, "unsupported option %s", Escape(p.errTok))
	}
	switch {
	case len(p.positionals) == 0:
		return usageErrorf(ErrMissingPattern, "missing required argument PATTERN")
	case len(p.positionals) > 2:
		return usageErrorf(ErrExcessOperand, "unexpected extra operand %s", Escape(p.positionals[2]))
	}

	// The preflight has resolved every help request and rejected every
	// token the library could fail on, so Run can neither emit nor error;
	// only the declared syntax work remains for it. Flag values are not
	// read back: forwarding order and cumulative counts come solely from
	// the ordered scan records, because the library fills option values
	// per container after the parse and loses cross-option order.
	var helpOpt *bool
	var pattern, root string
	app := newApp()
	app.Spec = spec()
	for _, d := range optionDecls {
		v := app.BoolOpt(d.name(), false, d.desc)
		if d.help {
			helpOpt = v
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
	// validation or search work. The preflight's lexical rejection of
	// every "=" assignment spelling makes this unreachable; the check
	// stays so no library behavior can turn an option into a silent
	// search.
	if helpOpt != nil && *helpOpt {
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
	childArgv := make([]string, 0, len(p.flags)+5)
	childArgv = append(childArgv, "--json", "--no-config")
	childArgv = append(childArgv, p.flags...)
	childArgv = append(childArgv, "--", pattern, root)
	return Result{Kind: KindSearch, Pattern: pattern, Root: root, ChildArgv: childArgv}
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
	help         bool      // a help request appeared before the first --
	errTok       string    // first failing option token, "" if none
	errKind      ErrorKind // classification of errTok
	positionals  []string  // positional operands in order
	flags        []string  // accepted search-flag spellings in encounter order
	unrestricted int       // cumulative unrestricted occurrences across spellings
}

// scanArgs scans argv left to right, stopping option recognition at the
// first "--" (later tokens are all positional, including -h, --help, and
// another --). A help request wins over every other classification and
// ends the scan. Accepted search-flag spellings are recorded in encounter
// order, expanding combined short tokens left to right, so forwarding
// order and cumulative unrestricted counts come from this scan rather
// than from the library's unordered per-option values.
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

// scanOption validates one pre-terminator dash token against the shared
// declarations and records its accepted spellings in encounter order,
// counting unrestricted occurrences as it goes. The first failing token —
// an unrecognized spelling or the token containing the third unrestricted
// occurrence — is kept; scanning continues so a later help request can
// still win.
func (p *preflight) scanOption(tok string) {
	uses, ok := expandOption(tok)
	if !ok {
		if p.errTok == "" {
			p.errKind = ErrUnsupportedOption
			p.errTok = tok
		}
		return
	}
	for _, u := range uses {
		p.flags = append(p.flags, u.spelling)
		if u.decl.unrestricted {
			p.unrestricted++
			if p.unrestricted > 2 && p.errTok == "" {
				p.errKind = ErrExcessUnrestricted
				p.errTok = tok
			}
		}
	}
}

// flagUse is one accepted option occurrence: the shared declaration it
// matched and the exact spelling to forward to the child.
type flagUse struct {
	decl     *optionDecl
	spelling string
}

// expandOption resolves a pre-terminator dash token to its accepted
// spellings against the shared declarations. A literal long flag forwards
// its supplied spelling; a short token of one or more declared letters
// expands left to right, so -iwF is -i -w -F. Every other token fails:
// unknown names, undeclared letters, and every "=" assignment spelling,
// which the no-argument contract rejects lexically.
func expandOption(tok string) ([]flagUse, bool) {
	if strings.HasPrefix(tok, "--") {
		for i := range optionDecls {
			if d := &optionDecls[i]; d.forward && d.long == tok[2:] {
				return []flagUse{{d, tok}}, true
			}
		}
		return nil, false
	}
	var uses []flagUse
	for i := 1; i < len(tok); i++ {
		c := tok[i]
		var d *optionDecl
		for j := range optionDecls {
			if optionDecls[j].forward && optionDecls[j].short != 0 && optionDecls[j].short == c {
				d = &optionDecls[j]
				break
			}
		}
		if d == nil {
			return nil, false
		}
		uses = append(uses, flagUse{d, "-" + string(c)})
	}
	return uses, true
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

// Escape renders external data safe for a single-line diagnostic or stub
// substitution. Backslashes double; newline, carriage return, and tab
// become \n, \r, \t; other C0 controls and DEL use caret notation; C1
// controls use \u escapes; invalid UTF-8 bytes use \xNN. Printable text
// passes through. This is the minimal Issue 1 escaper; Issue 6 generalizes
// safe presentation for every sink.
func Escape(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			switch {
			case c == '\\':
				b.WriteString(`\\`)
			case c == '\n':
				b.WriteString(`\n`)
			case c == '\r':
				b.WriteString(`\r`)
			case c == '\t':
				b.WriteString(`\t`)
			case c < 0x20:
				b.WriteByte('^')
				b.WriteByte(c + '@')
			case c == 0x7f:
				b.WriteString(`^?`)
			default:
				b.WriteByte(c)
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			fmt.Fprintf(&b, `\x%02x`, c)
			i++
			continue
		}
		if r >= 0x80 && r < 0xa0 {
			fmt.Fprintf(&b, `\u%04x`, r)
		} else {
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	return b.String()
}

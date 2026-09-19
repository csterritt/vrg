// Command vrg runs ripgrep and browses the results in a terminal UI.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"vrg/internal/cli"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the process boundary: it alone chooses exit statuses and output
// streams. The CLI module returns an explicit result kind; the library
// never exits the process and never writes to either stream.
func run(args []string, stdout, stderr io.Writer) int {
	res := cli.Parse(args, stdout, cli.Env{Stat: os.Stat})
	switch res.Kind {
	case cli.KindHelp:
		return 0
	case cli.KindSearch:
		// Interim stub proving the end-to-end slice; later issues replace
		// it with the search and the TUI. It prints the exact protected
		// child argv, one space-separated element at a time so an empty
		// element stays visible as an empty field.
		var b strings.Builder
		b.WriteString("search stub: argv=rg")
		for _, a := range res.ChildArgs {
			b.WriteString(" " + cli.Escape(a))
		}
		fmt.Fprintln(stdout, b.String())
		return 0
	default:
		fmt.Fprintln(stderr, res.Diagnostic)
		fmt.Fprintf(stderr, "\n%s", cli.HelpText())
		return 2
	}
}

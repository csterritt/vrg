// Command vrg runs ripgrep and browses the results in a terminal UI.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"vrg/internal/app"
	"vrg/internal/cli"
	"vrg/internal/safepresentation"
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
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "vrg: cannot determine working directory: %s\n", safepresentation.EscapePath([]byte(err.Error())))
			return 2
		}
		return app.Run(context.Background(), app.Config{
			Args:       res.ChildArgs,
			Dir:        cwd,
			Err:        stderr,
			RunProgram: runProgram,
		}, testSeamOptions()...)
	default:
		fmt.Fprintln(stderr, res.Diagnostic)
		fmt.Fprintf(stderr, "\n%s", cli.HelpText())
		return 2
	}
}

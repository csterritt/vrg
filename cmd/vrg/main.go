// Command vrg runs ripgrep and browses the results in a terminal UI.
package main

import (
	"fmt"
	"io"
	"os"

	"vrg/internal/app"
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
		// stop releases the file watchers the tagged test seams start.
		stop := make(chan struct{})
		defer close(stop)
		return app.Run(res.Args, searchEnv(stderr, stop))
	default:
		fmt.Fprintln(stderr, res.Diagnostic)
		fmt.Fprintf(stderr, "\n%s", cli.HelpText())
		return 2
	}
}

// searchEnv builds the app environment for a search. The two
// unconditional call sites are the build-tagged boundaries:
// runTeaProgram is the program-runner seam — the production variant
// delegates directly to the constructed program's Run — and
// testSeamEnv wires the VRG_TEST_* subprocess seams, present only in
// the vrg_testhooks build. stop releases any file watchers the seams
// started.
func searchEnv(stderr io.Writer, stop <-chan struct{}) app.Env {
	env := app.Env{Stderr: stderr, Runner: runTeaProgram}
	return testSeamEnv(env, stop)
}

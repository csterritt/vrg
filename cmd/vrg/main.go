// Command vrg runs ripgrep and browses the results in a terminal UI.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

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
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "vrg: cannot determine working directory: %s\n", cli.Escape(err.Error()))
			return 2
		}
		return app.Run(context.Background(), app.Config{
			Args: res.ChildArgs,
			Dir:  cwd,
			Err:  stderr,
		}, testSeamOptions()...)
	default:
		fmt.Fprintln(stderr, res.Diagnostic)
		fmt.Fprintf(stderr, "\n%s", cli.HelpText())
		return 2
	}
}

// testSeamOptions wires the VRG_TEST_* test hooks as app options.
// VRG_TEST_GATE=<file> holds index preparation while the file exists;
// VRG_TEST_COLLECT_ACK=<file> records a line once the child's output has
// been fully collected, so tests can observe that rg exited while
// preparation is still held.
func testSeamOptions() []app.Option {
	var opts []app.Option
	if p := os.Getenv("VRG_TEST_COLLECT_ACK"); p != "" {
		opts = append(opts, app.WithCollectAck(func() { appendLine(p, "collected\n") }))
	}
	if p := os.Getenv("VRG_TEST_GATE"); p != "" {
		opts = append(opts, app.WithGate(func() { waitFileGone(p) }))
	}
	return opts
}

func appendLine(path, line string) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	f.WriteString(line)
	f.Close()
}

// waitFileGone polls until path no longer exists.
func waitFileGone(path string) {
	for {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

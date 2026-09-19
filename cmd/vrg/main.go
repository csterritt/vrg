// Command vrg runs ripgrep and browses the results in a terminal UI.
package main

import (
	"context"
	"errors"
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
// preparation is still held. VRG_TEST_LOAD_GATE=<file> holds each file
// load — read and decode/map together — while the file exists.
// VRG_TEST_REAP=<file> records the child's reaped wait status once per
// process — the side channel proving vrg's wait/reap path ran.
// VRG_TEST_FAIL=<file> injects a controlled failure once the file exists.
func testSeamOptions() []app.Option {
	var opts []app.Option
	if p := os.Getenv("VRG_TEST_COLLECT_ACK"); p != "" {
		opts = append(opts, app.WithCollectAck(func() { appendLine(p, "collected\n") }))
	}
	if p := os.Getenv("VRG_TEST_GATE"); p != "" {
		opts = append(opts, app.WithGate(func() { waitFileGone(p) }))
	}
	if p := os.Getenv("VRG_TEST_LOAD_GATE"); p != "" {
		opts = append(opts, app.WithLoadGate(func() { waitFileGone(p) }))
	}
	if p := os.Getenv("VRG_TEST_REAP"); p != "" {
		opts = append(opts, app.WithReapReport(func(res app.Result) {
			appendLine(p, fmt.Sprintf("reaped code=%d err=%v\n", res.Code, res.Err))
		}))
	}
	if p := os.Getenv("VRG_TEST_FAIL"); p != "" {
		opts = append(opts, app.WithFailFunc(func() error {
			waitFileExists(p)
			return errors.New("injected test failure \x1b[7m")
		}))
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

// waitFileExists polls until path exists.
func waitFileExists(path string) {
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

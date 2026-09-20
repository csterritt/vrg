// Command vrg runs ripgrep and browses the results in a terminal UI.
package main

import (
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
		// stop releases the file watchers below when the run ends.
		stop := make(chan struct{})
		defer close(stop)
		return app.Run(res.Args, searchEnv(stderr, stop))
	default:
		fmt.Fprintln(stderr, res.Diagnostic)
		fmt.Fprintf(stderr, "\n%s", cli.HelpText())
		return 2
	}
}

// searchEnv builds the app environment for a search. The VRG_TEST_*
// variables wire the subprocess-boundary test seams — a preparation
// gate, a reap side channel, a diagnostic-collection acknowledgement
// side channel, and a controlled-failure trigger — and are inert when
// unset. stop releases any file watchers the seams started.
func searchEnv(stderr io.Writer, stop <-chan struct{}) app.Env {
	env := app.Env{Stderr: stderr}
	if path := os.Getenv("VRG_TEST_GATE"); path != "" {
		env.Gate = fileTrigger(path, stop)
	}
	if path := os.Getenv("VRG_TEST_REAP"); path != "" {
		env.OnReap = func(err error) {
			status := "exit status 0"
			if err != nil {
				status = err.Error()
			}
			_ = os.WriteFile(path, []byte(status+"\n"), 0o644)
		}
	}
	if path := os.Getenv("VRG_TEST_COLLECT_ACK"); path != "" {
		env.OnCollect = func(line string) {
			f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
			if err == nil {
				_, _ = f.WriteString(line + "\n")
				_ = f.Close()
			}
		}
	}
	if path := os.Getenv("VRG_TEST_FAIL_TRIGGER"); path != "" {
		env.Fail = fileTrigger(path, stop)
		env.FailDiagnostic = os.Getenv("VRG_TEST_FAIL_DIAGNOSTIC")
	}
	return env
}

// fileTrigger returns a channel that closes once path exists, polled on
// a paced ticker rather than a spin. The watcher exits without closing
// the channel when stop closes.
func fileTrigger(path string, stop <-chan struct{}) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		t := time.NewTicker(5 * time.Millisecond)
		defer t.Stop()
		for {
			if _, err := os.Stat(path); err == nil {
				close(ch)
				return
			}
			select {
			case <-stop:
				return
			case <-t.C:
			}
		}
	}()
	return ch
}

package app

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	"vrg/internal/searchindex"
)

// Config describes one search invocation and the process/test seams the
// collector honors.
type Config struct {
	// Rg is the ripgrep executable: a name resolved through PATH ("rg")
	// or an explicit path. Empty means "rg".
	Rg string
	// Argv is the protected child argument vector — cli.Result.ChildArgv,
	// argv[0] excluded.
	Argv []string
	// Workdir is the invocation working directory: the child's working
	// directory and the base for resolving relative result paths.
	Workdir string
	// Drained, when non-nil, is closed once the child has exited and both
	// pipes are fully drained, before PrepareGate is awaited. Tests use
	// it to prove the searching state spans post-exit processing.
	Drained chan struct{}
	// PrepareGate, when non-nil, is awaited after the child has exited
	// and both pipes are drained and before index preparation begins.
	// Tests hold it to keep the app in the searching state independently
	// of rg exit. Cancellation abandons the wait.
	PrepareGate <-chan struct{}
	// ReapReport, when non-nil, receives one line holding the child's
	// reaped wait status immediately after Wait returns. It is the
	// subprocess-boundary harness's evidence that vrg's reap path ran,
	// rather than inferring reaping from a missing pid.
	ReapReport io.Writer
	// DiagAck, when non-nil, receives one acknowledgement line for
	// every diagnostic the model processes into the session collection.
	// The PTY harness waits on it before sending an exit key: it proves
	// application-side processing, where a child-side write handshake
	// would prove only that bytes reached the pipe.
	DiagAck io.Writer
	// DiagInject, when non-nil, delivers additional diagnostic lines to
	// the model through the same channel as live child stderr — the
	// test-only injection seam behind the tagged binary's diagnostic
	// trigger. Delivery ends when the channel closes or the session is
	// cancelled.
	DiagInject <-chan string
}

// searchDoneMsg delivers the finished collection to the model: the
// prepared index, the buffered child stderr, and the child's wait error
// (nil for a clean exit). The model's outcome decision classifies all
// three independently — see decideOutcome in overlay.go. The stderr
// bytes still serve display; the session collection took them
// line-by-line through diagMsg while the search ran.
type searchDoneMsg struct {
	index   *searchindex.Index
	stderr  []byte
	waitErr error
}

// diagMsg carries one raw child-stderr line collected while the search
// runs, so a diagnostic lands in the session collection even when the
// child never exits. Processing the message — escaping the line and
// appending it — is the moment the diagnostic counts as collected.
type diagMsg struct {
	line string
}

// Session is one running search: the spawned child, the collector
// draining and reaping it, and the model that reports progress. The
// process boundary terminates the child through Cancel and confirms the
// reap through Reaped on every exit path.
type Session struct {
	model  Model
	cancel context.CancelFunc
	reaped chan struct{}
}

// Model returns the Bubble Tea model for the session.
func (s *Session) Model() Model { return s.model }

// Cancel terminates the rg child and abandons outstanding collection
// work. It is idempotent.
func (s *Session) Cancel() { s.cancel() }

// Reaped is closed once the child has exited, both pipes have drained,
// and Wait has returned — that is, once the child has been reaped.
func (s *Session) Reaped() <-chan struct{} { return s.reaped }

// Start spawns the rg child for cfg and returns the session with
// collection already running off the UI path. A spawn failure is
// returned synchronously — before any TUI — so the caller can diagnose
// it and exit without entering the program.
func Start(ctx context.Context, cfg Config) (*Session, error) {
	rg := cfg.Rg
	if rg == "" {
		rg = "rg"
	}
	ctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(ctx, rg, cfg.Argv...)
	cmd.Dir = cfg.Workdir
	// The child leads its own process group so cancellation kills the
	// whole tree: a scripted rg forks its payload rather than exec'ing
	// it, and a surviving grandchild would hold the drained pipes open
	// and stall the reap forever.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("cannot start rg: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("cannot start rg: %w", err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("cannot start rg: %w", err)
	}
	done := make(chan searchDoneMsg, 1)
	// diags carries stderr lines to the model as they are read. It is
	// unbuffered: every send pairs with the model's event wait, so the
	// completion cannot overtake a diagnostic on the way to Update.
	diags := make(chan string)
	if cfg.DiagInject != nil {
		go injectDiags(ctx, cfg.DiagInject, diags)
	}
	reaped := make(chan struct{})
	go collect(ctx, cmd, stdout, stderr, cfg, reaped, done, diags)
	m := newModel(done, cancel)
	m.diagCh = diags
	m.diagAck = cfg.DiagAck
	return &Session{model: m, cancel: cancel, reaped: reaped}, nil
}

// collect drains the child's stdout and stderr concurrently for its
// whole lifetime — neither pipe can fill and block rg — then waits for
// exit, reports the reaped status, honors the drained signal and
// preparation gate, prepares the index, and delivers the result. Each
// stderr line is additionally forwarded to the model as it is read, so
// diagnostics reach the session collection even when the child stays
// running. Killing the child closes both pipes, so drainage ends
// promptly rather than waiting for further output. Cancellation
// abandons the gate and skips index preparation entirely.
func collect(ctx context.Context, cmd *exec.Cmd, stdout, stderr io.Reader, cfg Config, reaped chan<- struct{}, done chan<- searchDoneMsg, diags chan<- string) {
	var outBuf, errBuf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _, _ = io.Copy(&outBuf, stdout) }()
	go func() {
		defer wg.Done()
		// stderr is also delivered to the model line-by-line as it is
		// read: a diagnostic reaches the session collection even if the
		// child then blocks forever. Cancellation stops the delivery —
		// the model has quit and no one is collecting — while drainage
		// continues to the pipe's end.
		live := true
		r := bufio.NewReader(stderr)
		for {
			line, rerr := r.ReadString('\n')
			if line != "" {
				errBuf.WriteString(line)
				if live {
					line = strings.TrimSuffix(line, "\n")
					line = strings.TrimSuffix(line, "\r")
					select {
					case diags <- line:
					case <-ctx.Done():
						live = false
					}
				}
			}
			if rerr != nil {
				return
			}
		}
	}()
	wg.Wait()
	waitErr := cmd.Wait()

	if cfg.ReapReport != nil {
		fmt.Fprintf(cfg.ReapReport, "%s\n", cmd.ProcessState)
	}
	close(reaped)
	if cfg.Drained != nil {
		close(cfg.Drained)
	}

	if cfg.PrepareGate != nil && ctx.Err() == nil {
		select {
		case <-cfg.PrepareGate:
		case <-ctx.Done():
		}
	}
	var ix *searchindex.Index
	if ctx.Err() == nil {
		ix = prepareIndex(cfg.Workdir, outBuf.Bytes())
	}
	done <- searchDoneMsg{
		index:   ix,
		stderr:  errBuf.Bytes(),
		waitErr: waitErr,
	}
}

// injectDiags forwards externally injected diagnostic lines onto the
// model's diag channel under the same cancellation rule as live stderr
// delivery: a line a cancelled model will never read is dropped rather
// than blocking the injector.
func injectDiags(ctx context.Context, in <-chan string, diags chan<- string) {
	for line := range in {
		select {
		case diags <- line:
		case <-ctx.Done():
			return
		}
	}
}

// prepareIndex feeds the collected record stream through the index and
// prepares it: decoding, record skipping and counting, lifecycle
// validation, and binary exclusion all happen inside Feed.
func prepareIndex(workdir string, stream []byte) *searchindex.Index {
	ix := searchindex.New(workdir)
	ix.Feed(stream)
	ix.Prepare()
	return ix
}

package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"

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
}

// searchDoneMsg delivers the finished collection to the model: the
// prepared index, the buffered child stderr, and the child's wait error
// (nil for a clean exit). Stderr classification and process-outcome
// effects are Issue #9's; here they are cargo.
type searchDoneMsg struct {
	index   *searchindex.Index
	stderr  []byte
	waitErr error
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
	reaped := make(chan struct{})
	go collect(ctx, cmd, stdout, stderr, cfg, reaped, done)
	return &Session{model: newModel(done, cancel), cancel: cancel, reaped: reaped}, nil
}

// collect drains the child's stdout and stderr concurrently for its
// whole lifetime — neither pipe can fill and block rg — then waits for
// exit, reports the reaped status, honors the drained signal and
// preparation gate, prepares the index, and delivers the result.
// Killing the child closes both pipes, so drainage ends promptly rather
// than waiting for further output. Cancellation abandons the gate and
// skips index preparation entirely.
func collect(ctx context.Context, cmd *exec.Cmd, stdout, stderr io.Reader, cfg Config, reaped chan<- struct{}, done chan<- searchDoneMsg) {
	var outBuf, errBuf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _, _ = io.Copy(&outBuf, stdout) }()
	go func() { defer wg.Done(); _, _ = io.Copy(&errBuf, stderr) }()
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

// prepareIndex decodes the collected record stream into the index and
// prepares it. Malformed records are skipped; their counting is Issue
// #10's and lifecycle integrity is Issue #9's.
func prepareIndex(workdir string, stream []byte) *searchindex.Index {
	ix := searchindex.New(workdir)
	for _, line := range bytes.Split(stream, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if rec, err := searchindex.DecodeRecord(line); err == nil {
			ix.Add(rec)
		}
	}
	ix.Prepare()
	return ix
}

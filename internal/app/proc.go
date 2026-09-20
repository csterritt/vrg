package app

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"sync"
	"syscall"

	"vrg/internal/searchindex"
)

// Child is the process seam: a started rg child with piped output.
// Stdout and Stderr must be drained concurrently for the child's whole
// lifetime so neither pipe can fill and block it; Wait reaps it.
// Terminate kills the child and its process group; it is a no-op for an
// already-exited child and safe to call from any state.
type Child interface {
	Stdout() io.Reader
	Stderr() io.Reader
	Terminate()
	Wait() error
}

// Starter launches the rg child with the protected argv from workdir.
type Starter func(argv []string, workdir string) (Child, error)

// execChild adapts a running *exec.Cmd to the Child seam.
type execChild struct {
	cmd    *exec.Cmd
	stdout io.ReadCloser
	stderr io.ReadCloser
}

func (c *execChild) Stdout() io.Reader { return c.stdout }
func (c *execChild) Stderr() io.Reader { return c.stderr }
func (c *execChild) Wait() error       { return c.cmd.Wait() }

// Terminate kills the child's whole process group, so subprocesses the
// child spawned do not outlive it. The child is its own group leader
// (Setpgid at spawn), so the negative pid cannot reach vrg's group.
// A group with no members left — including the common case where the
// child already exited — makes the kill a harmless ESRCH.
func (c *execChild) Terminate() {
	if c.cmd.Process != nil {
		_ = syscall.Kill(-c.cmd.Process.Pid, syscall.SIGKILL)
	}
}

// execStarter is the production Starter: rg resolved on PATH, run from
// the invocation working directory in its own process group, with both
// output streams piped.
func execStarter(argv []string, workdir string) (Child, error) {
	cmd := exec.Command("rg", argv...)
	cmd.Dir = workdir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdout.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &execChild{cmd: cmd, stdout: stdout, stderr: stderr}, nil
}

// reaper wraps a Child so the single underlying Wait is shared between
// collection and the exit cleanup path: the first caller reaps the
// process and every caller observes the same wait status. onReap, when
// set, observes that status once — the test seam proving vrg's reap
// path ran rather than inferring it from a missing PID.
type reaper struct {
	child   Child
	onReap  func(error)
	mu      sync.Mutex
	reaped  bool
	waitErr error
}

func (p *reaper) Stdout() io.Reader { return p.child.Stdout() }
func (p *reaper) Stderr() io.Reader { return p.child.Stderr() }
func (p *reaper) Terminate()        { p.child.Terminate() }

// Wait reaps the child on first call and reports the recorded status to
// every later call.
func (p *reaper) Wait() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.reaped {
		p.waitErr = p.child.Wait()
		p.reaped = true
		if p.onReap != nil {
			p.onReap(p.waitErr)
		}
	}
	return p.waitErr
}

// searchResult is the product of one collected search, delivered to the
// model as a message: the prepared index, the stream-integrity result —
// assessed separately from process success — the captured stderr bytes
// (unclassified; the outcome decision owns classification), and the
// child's wait status.
type searchResult struct {
	index     *searchindex.Index
	integrity searchindex.Integrity
	stderr    []byte
	err       error
}

// collect drains both pipes concurrently for the whole child lifetime —
// stdout records feed the index builder, which applies the lifecycle
// matrix; stderr bytes are buffered — waits for the child, then prepares
// the index and its integrity result. Neither pipe can fill and block
// rg, and terminating the child closes both pipes so drainage ends
// promptly. A non-nil gate is awaited between child exit and index
// preparation so tests can hold preparation independently of rg exit;
// ctx cancellation releases a held gate.
func collect(ctx context.Context, child Child, workdir string, gate <-chan struct{}) searchResult {
	b := searchindex.NewBuilder(workdir)
	var stderr bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&stderr, child.Stderr())
	}()
	b.Consume(child.Stdout())
	wg.Wait()
	err := child.Wait()
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
		}
	}
	index, integrity := b.Finish()
	return searchResult{index: index, integrity: integrity, stderr: stderr.Bytes(), err: err}
}

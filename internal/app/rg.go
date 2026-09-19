package app

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os/exec"
	"sync"
)

// Result is a finished child's fully collected output.
type Result struct {
	// Stdout is the complete JSON event stream; Stderr is everything the
	// child wrote to its diagnostic stream.
	Stdout []byte
	Stderr []byte
	// Code is the child's exit code, or -1 when no exit code applies
	// (for example signal death). Err is the Wait error, if any.
	Code int
	Err  error
}

// Child is a started search child whose stdout and stderr are drained
// concurrently for its whole lifetime, so neither pipe can fill and block
// it. Wait returns the finished result once the child has exited and both
// pipes are fully collected; it is safe to call from several goroutines.
// Terminate kills a still-running child so Wait returns promptly; on an
// already-exited child it is a no-op. Diags streams the drained stderr
// lines as they arrive — each value one line with its terminator
// retained — closing at stderr EOF, so a diagnostic can be collected
// while the child still runs; it may be nil for a child without
// incremental diagnostics, in which case Wait's Result.Stderr is the
// only stderr route.
type Child interface {
	Wait() Result
	Terminate()
	Diags() <-chan string
}

// StartFunc spawns the search child. A non-nil error is a start failure:
// no child is running and nothing has been collected.
type StartFunc func(ctx context.Context, args []string, dir string) (Child, error)

// proc is the production Child: a running rg process.
type proc struct {
	cmd  *exec.Cmd
	done chan struct{}
	res  Result

	// Incremental stderr diagnostics: the drainage goroutine appends
	// each drained line to pending under mu and signals notify (cap 1,
	// coalescing); a feeder started lazily by Diags forwards the queue
	// onto diags in order and closes it once the queue empties after
	// stderr EOF. Queueing keeps drainage — and therefore Wait —
	// independent of any consumer's pace.
	mu        sync.Mutex
	pending   []string
	stderrEOF bool
	notify    chan struct{}
	diags     chan string
	feedOnce  sync.Once
}

// spawn starts rg with args (the protected vector, excluding the program
// name) running from dir — the invocation working directory. Both output
// pipes drain concurrently from the first spawn for the child's whole
// lifetime; drainage reads to EOF before Wait so a blocked pipe can never
// stall the child, and child termination ends drainage promptly.
func spawn(ctx context.Context, args []string, dir string) (Child, error) {
	cmd := exec.CommandContext(ctx, "rg", args...)
	cmd.Dir = dir
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	p := &proc{
		cmd:    cmd,
		done:   make(chan struct{}),
		notify: make(chan struct{}, 1),
		diags:  make(chan string),
	}
	go p.collect(cmd, stdout, stderr)
	return p, nil
}

// collect drains both pipes until the child exits, then reaps it and
// publishes the finished Result. Terminating the child closes the pipes,
// ending the copies promptly.
func (p *proc) collect(cmd *exec.Cmd, stdout, stderr io.Reader) {
	var outBuf, errBuf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); io.Copy(&outBuf, stdout) }() //nolint:errcheck
	go func() { defer wg.Done(); p.drainStderr(&errBuf, stderr) }()
	wg.Wait()
	err := cmd.Wait()
	code := -1
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	}
	p.res = Result{Stdout: outBuf.Bytes(), Stderr: errBuf.Bytes(), Code: code, Err: err}
	close(p.done)
}

// drainStderr copies stderr into buf line by line, queueing each drained
// line — terminator retained — for incremental delivery so a diagnostic
// can be collected while the child still runs. Queue appends never
// block, so a consumer's pace can never throttle the copy.
func (p *proc) drainStderr(buf *bytes.Buffer, r io.Reader) {
	defer func() {
		p.mu.Lock()
		p.stderrEOF = true
		p.mu.Unlock()
		select {
		case p.notify <- struct{}{}:
		default:
		}
	}()
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		if line != "" {
			buf.WriteString(line)
			p.mu.Lock()
			p.pending = append(p.pending, line)
			p.mu.Unlock()
			select {
			case p.notify <- struct{}{}:
			default:
			}
		}
		if err != nil {
			return
		}
	}
}

// Diags returns the incremental stderr line stream, starting the feeder
// on first call.
func (p *proc) Diags() <-chan string {
	p.feedOnce.Do(func() { go p.feedDiags() })
	return p.diags
}

// feedDiags forwards queued stderr lines onto the diags channel in drain
// order, closing it once the queue empties after stderr EOF. It may
// block on a slow or absent consumer — harmless: neither the copy nor
// Wait depends on it.
func (p *proc) feedDiags() {
	for {
		p.mu.Lock()
		if len(p.pending) > 0 {
			line := p.pending[0]
			p.pending = p.pending[1:]
			p.mu.Unlock()
			p.diags <- line
			continue
		}
		eof := p.stderrEOF
		p.mu.Unlock()
		if eof {
			close(p.diags)
			return
		}
		<-p.notify
	}
}

// Wait returns the child's collected result once it has exited and both
// pipes are fully drained.
func (p *proc) Wait() Result {
	<-p.done
	return p.res
}

// Terminate kills the child so collection and Wait end promptly; it is a
// no-op once the child has exited.
func (p *proc) Terminate() {
	if p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
}

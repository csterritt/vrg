package app

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os/exec"
	"sync"

	"vrg/internal/searchindex"
)

// Child is the process seam: a started rg child with piped output.
// Stdout and Stderr must be drained concurrently for the child's whole
// lifetime so neither pipe can fill and block it; Wait reaps it.
type Child interface {
	Stdout() io.Reader
	Stderr() io.Reader
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

// execStarter is the production Starter: rg resolved on PATH, run from
// the invocation working directory, with both output streams piped.
func execStarter(argv []string, workdir string) (Child, error) {
	cmd := exec.Command("rg", argv...)
	cmd.Dir = workdir
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

// searchResult is the product of one collected search, delivered to the
// model as a message: the prepared index, the captured stderr bytes
// (unclassified; Issue 9 owns classification), and the child's wait
// status.
type searchResult struct {
	index  *searchindex.Index
	stderr []byte
	err    error
}

// collect drains both pipes concurrently for the whole child lifetime —
// stdout records feed the index, stderr bytes are buffered — waits for
// the child, then prepares the index. Neither pipe can fill and block
// rg, and terminating the child closes both pipes so drainage ends
// promptly. A non-nil gate is awaited between child exit and index
// preparation so tests can hold preparation independently of rg exit.
func collect(ctx context.Context, child Child, workdir string, gate <-chan struct{}) searchResult {
	index := searchindex.New(workdir)
	var stderr bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&stderr, child.Stderr())
	}()
	drainRecords(child.Stdout(), index)
	wg.Wait()
	err := child.Wait()
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
		}
	}
	index.Finish()
	return searchResult{index: index, stderr: stderr.Bytes(), err: err}
}

// drainRecords feeds every newline-terminated record to the index.
// Records failing per-record validation are skipped — skip/count
// accounting is Issue 10's. A trailing unterminated fragment is dropped
// here; Issue 10 counts it malformed.
func drainRecords(r io.Reader, index *searchindex.Index) {
	br := bufio.NewReader(r)
	for {
		rec, err := br.ReadBytes('\n')
		if err != nil {
			return
		}
		_ = index.Add(rec[:len(rec)-1])
	}
}

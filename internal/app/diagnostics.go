package app

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/safepresentation"
)

// stderrMsg is one drained line of the child's stderr, delivered to the
// model as it is read so a diagnostic reaches the session collection
// even while the child is still running. raw is the raw line bytes
// including the newline terminator; the child's final unterminated
// chunk arrives without one.
type stderrMsg struct{ raw string }

// diagnostics is the session diagnostic collection: every diagnostic
// line the model has processed, in collection order and independent of
// what an overlay displayed. It is shared across the model's value
// copies and with Run's cleanup boundary, so lines appended inside the
// Update loop and a controlled failure's diagnostic land in one ordered
// collection. The collection is never persisted: replay to stderr is
// its only output.
type diagnostics struct {
	mu    sync.Mutex
	lines []string
	// emit delivers a diagnostic message into the running program's
	// Update loop — the production runner wires it to Program.Send.
	// Collection happens in Update, so a message still in flight when
	// the exit decision runs is never collected. A nil emit still
	// drains the child pipe, which must never fill and block rg.
	emit func(tea.Msg)
	// onAdd, when set, fires once per collected line — the test seam
	// acknowledging that a diagnostic has been processed into the
	// collection.
	onAdd func(string)
}

// add appends one already-escaped diagnostic line to the collection and
// fires the collection acknowledgement.
func (d *diagnostics) add(line string) {
	d.mu.Lock()
	d.lines = append(d.lines, line)
	onAdd := d.onAdd
	d.mu.Unlock()
	if onAdd != nil {
		onAdd(line)
	}
}

// addAll appends each line in order.
func (d *diagnostics) addAll(lines []string) {
	for _, l := range lines {
		d.add(l)
	}
}

// snapshot copies the collected lines in collection order.
func (d *diagnostics) snapshot() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.lines...)
}

// replay writes every collected diagnostic to w — one line each, in
// collection order. It is the single post-restoration stderr writer
// shared by ordinary quits, cancellation, and controlled failures, so
// each collected occurrence reaches stderr exactly once.
func (d *diagnostics) replay(w io.Writer) {
	for _, line := range d.snapshot() {
		fmt.Fprintln(w, line)
	}
}

// escapeDiagnosticLines renders one raw stderr line into collection
// lines through the safe-presentation diagnostic policy: the real line
// boundary is preserved as a collection boundary and embedded controls
// are escaped. A blank line carries no diagnostic and yields none.
func escapeDiagnosticLines(raw string) []string {
	esc := strings.TrimSuffix(safepresentation.EscapeDiagnostic(raw), "\n")
	if esc == "" {
		return nil
	}
	return strings.Split(esc, "\n")
}

// drainStderr reads r line by line — a trailing unterminated chunk is
// delivered too — and hands each raw line to send for delivery to the
// model's diagnostic collection. A nil send still drains: the pipe must
// never fill and block the child.
func drainStderr(r io.Reader, send func(tea.Msg)) {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 && send != nil {
			send(stderrMsg{raw: string(line)})
		}
		if err != nil {
			return
		}
	}
}

package app

import (
	"fmt"
	"sync"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/safepresentation"
)

// Event is one acknowledgement record emitted by the model's
// acknowledgement log: a per-process monotonic sequence number, the
// event kind, and a single-line detail. The log exists so tests can
// wait on the exact occurrence an action caused — a record with a
// sequence number beyond the wait's mark is causally the action's
// result, and an earlier same-kind record can never satisfy it.
type Event struct {
	Seq    uint64
	Kind   string
	Detail string
}

// events is the per-run acknowledgement log, shared across the model's
// value copies like the diagnostic collection. emit, when set by Run
// from the environment, receives each record at the moment the
// transition it names commits — inside the Update that processed its
// message. The log observes transitions; it never changes them, and
// the untagged build wires no emit so the records cost nothing.
type events struct {
	mu   sync.Mutex
	seq  uint64
	emit func(Event)
}

// armed reports whether the log emits: the hook is wired only in the
// test-seam build, so production Update calls pay a nil check.
func (e *events) armed() bool {
	if e == nil {
		return false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.emit != nil
}

// record appends one acknowledgement record with the next per-process
// sequence number.
func (e *events) record(kind, detail string) {
	e.mu.Lock()
	e.seq++
	ev := Event{Seq: e.seq, Kind: kind, Detail: detail}
	emit := e.emit
	e.mu.Unlock()
	if emit != nil {
		emit(ev)
	}
}

// noteUpdate emits the acknowledgement records for one processed
// Update message: a msg record for the message itself, a key record
// for a processed key press, a load or layout record for an admitted
// completion, and a record for each transition the message committed —
// a state entered, the diagnostic overlay or help opened or dismissed,
// a pop-up opened or dismissed. loadOK carries the pre-update
// admission check for a loadResult, whose request entry the update
// consumes. All records land after the update returns, so each
// acknowledges a fully committed transition.
func (e *events) noteUpdate(prev, next Model, msg tea.Msg, loadOK bool) {
	if !e.armed() {
		return
	}
	e.record("msg", fmt.Sprintf("%T", msg))
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		e.record("key", msg.String())
	case loadResult:
		if loadOK {
			res := "ok"
			if msg.err != nil {
				res = "fail"
			}
			e.record("load", res+" "+safepresentation.EscapePath(msg.path))
		}
	case layoutResult:
		// The result installed only while its key still named the
		// current parameters; the post-update buffer entry is the
		// proof.
		if next.buffers[msg.key.Path] == msg.model {
			e.record("layout", safepresentation.EscapePath([]byte(msg.key.Path)))
		}
	}
	if prev.state != next.state {
		e.record("state", next.state.String())
	}
	if prev.overlay == nil && next.overlay != nil {
		e.record("overlay", "open")
	} else if prev.overlay != nil && next.overlay == nil {
		e.record("overlay", "dismissed")
	}
	if prev.help == nil && next.help != nil {
		e.record("help", "open")
	} else if prev.help != nil && next.help == nil {
		e.record("help", "dismissed")
	}
	if prev.popup == nil && next.popup != nil {
		e.record("popup", "open")
	} else if prev.popup != nil && next.popup == nil {
		e.record("popup", "dismissed")
	}
}

// String names the lifecycle state in acknowledgement records.
func (s state) String() string {
	switch s {
	case stateSearching:
		return "searching"
	case stateBrowse:
		return "browse"
	case stateNoResults:
		return "noresults"
	case stateFatal:
		return "fatal"
	case stateCancelled:
		return "cancelled"
	}
	return "unknown"
}

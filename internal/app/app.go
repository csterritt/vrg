package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
	"vrg/internal/theme"
	"vrg/internal/viewport"
)

// Config is the validated search invocation plus the process's I/O
// wiring.
type Config struct {
	// Args is the protected ripgrep argument vector, excluding the "rg"
	// program name.
	Args []string
	// Dir is the invocation working directory: rg runs there and
	// relative result paths resolve against it.
	Dir string
	// Err receives diagnostics such as the start-failure message; nil
	// means os.Stderr.
	Err io.Writer
	// Start spawns the search child; nil means spawn rg from PATH.
	Start StartFunc
}

// Option configures optional seams.
type Option func(*options)

type options struct {
	// gate, when set, runs after the child's stream is fully collected
	// and before index preparation — the hold proving the searching
	// state covers post-exit processing.
	gate func()
	// collectAck, when set, runs once collection completes, before gate.
	collectAck func()
	// reap, when set, is called once per process with the child's reaped
	// wait status — the side channel proving vrg's wait/reap path ran
	// rather than inferring reaping from a missing pid.
	reap func(Result)
	// fail, when set, is the injectable controlled-failure hook: a
	// non-nil return is a controlled application failure.
	fail func() error
	// loadGate, when set, runs inside each file-load command before the
	// read — the hold proving "Loading…" spans the whole load: disk read
	// plus decode and byte→cell mapping, all off the update path.
	loadGate func()
}

// WithGate holds index preparation until fn returns.
func WithGate(fn func()) Option { return func(o *options) { o.gate = fn } }

// WithCollectAck runs fn once the child's output is fully collected.
func WithCollectAck(fn func()) Option { return func(o *options) { o.collectAck = fn } }

// WithReapReport calls fn once with the child's reaped wait status.
func WithReapReport(fn func(Result)) Option { return func(o *options) { o.reap = fn } }

// WithFailFunc installs the controlled-failure hook: a non-nil return
// from fn fails the application under vrg's control.
func WithFailFunc(fn func() error) Option { return func(o *options) { o.fail = fn } }

// WithLoadGate holds each file load — read and decode/map together —
// until fn returns.
func WithLoadGate(fn func()) Option { return func(o *options) { o.loadGate = fn } }

type state int

const (
	stateSearching state = iota
	stateBrowse
)

// model is the Bubble Tea model: "Searching…" while collection and index
// preparation run off the update path, then the two-pane browse view.
type model struct {
	cfg   Config
	opts  options
	child Child

	state         state
	width, height int
	status        int
	// quitting marks that a controlled exit is underway; messages
	// arriving after it — including late search and load completions —
	// are discarded so they cannot revive the UI.
	quitting bool
	// failErr is a controlled application failure reported through the
	// single post-restoration stderr writer in Run.
	failErr error

	// Browse state. idx is the prepared search index; cur is the current
	// file's index into idx.Files (the first stop's file — Issue #13
	// owns navigation). Files load asynchronously: loading marks the
	// in-flight raw paths, bufs caches prepared buffers, and failed
	// records read failures, all keyed by the raw path bytes — never by
	// an escaped display form.
	idx     *searchindex.Index
	cur     int
	loading map[string]bool
	bufs    map[string]*filebuffer.Buffer
	failed  map[string]bool
	vp      viewport.Viewport
	theme   theme.Theme
}

func newModel(cfg Config, opts options, child Child) *model {
	return &model{
		cfg:     cfg,
		opts:    opts,
		child:   child,
		state:   stateSearching,
		loading: map[string]bool{},
		bufs:    map[string]*filebuffer.Buffer{},
		failed:  map[string]bool{},
		theme:   theme.Dark(),
	}
}

// searchDoneMsg carries the collected result and prepared index from the
// collection command into Update.
type searchDoneMsg struct {
	res Result
	idx *searchindex.Index
}

// failMsg carries a controlled application failure from the injected
// failure hook into Update.
type failMsg struct{ err error }

// Init starts collection and, when the failure hook is installed, the
// hook itself. The collection command blocks on the child, runs the
// collection acknowledgement and the preparation gate, then builds the
// index — all off the UI update path so the model stays responsive.
func (m *model) Init() tea.Cmd {
	child := m.child
	opts := m.opts
	dir := m.cfg.Dir
	collect := func() tea.Msg {
		res := child.Wait()
		if opts.collectAck != nil {
			opts.collectAck()
		}
		if opts.gate != nil {
			opts.gate()
		}
		return searchDoneMsg{res: res, idx: searchindex.Build(res.Stdout, dir)}
	}
	if opts.fail == nil {
		return collect
	}
	fail := opts.fail
	return tea.Batch(collect, func() tea.Msg {
		return failMsg{err: fail()}
	})
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.quitting {
		// A controlled exit is underway: discard everything still in
		// flight so late work cannot revive the UI.
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case searchDoneMsg:
		m.state = stateBrowse
		m.idx = msg.idx
		return m, m.startLoad()
	case fileLoadedMsg:
		key := string(msg.path)
		delete(m.loading, key)
		if msg.err != nil {
			m.failed[key] = true
		} else {
			m.bufs[key] = msg.buf
		}
	case failMsg:
		if msg.err == nil {
			return m, nil
		}
		m.failErr = msg.err
		m.quitting = true
		return m, m.quitCmd()
	case tea.KeyPressMsg:
		switch {
		case msg.Keystroke() == "ctrl+c":
			// ctrl+c has global precedence: cancellation in every state.
			m.status = 130
			m.quitting = true
			return m, m.quitCmd()
		case msg.Text == "q" && m.state == stateSearching:
			// q while searching — including gate-held index
			// preparation after rg has exited — is cancellation.
			m.status = 130
			m.quitting = true
			return m, m.quitCmd()
		case msg.Text == "c":
			// c toggles the colour scheme between dark and light for
			// the session; nothing persists.
			m.theme = m.theme.Toggled()
		case msg.Text == "q" && m.state == stateBrowse:
			// q in ordinary browsing exits with the fixed
			// search-derived status through the same cleanup path.
			m.status = 0
			m.quitting = true
			return m, m.quitCmd()
		}
	}
	return m, nil
}

// quitCmd is the single cleanup path every controlled exit routes
// through: terminate and reap the child, report the reaped status, then
// quit so the program restores the terminal.
func (m *model) quitCmd() tea.Cmd {
	child, report := m.child, m.opts.reap
	return func() tea.Msg {
		reapChild(child, report)
		return tea.QuitMsg{}
	}
}

// reapChild terminates a still-running child and waits for it, ending
// Issue #3's drainage promptly because termination closes its pipes.
// Wait is idempotent, so running it again for an already-finished or
// already-reaped child is harmless.
func reapChild(c Child, report func(Result)) {
	c.Terminate()
	res := c.Wait()
	if report != nil {
		report(res)
	}
}

func (m *model) View() tea.View {
	s := center("Searching…", m.width, m.height)
	if m.state == stateBrowse {
		s = m.browseView()
	}
	v := tea.NewView(m.theme.Base(s))
	v.AltScreen = true
	return v
}

// Run is the whole search lifecycle: spawn the child, show "Searching…"
// until the stream is collected and the index prepared, then the
// two-pane browse view. A start failure prints a sanitized diagnostic to
// cfg.Err and
// returns exit 2 without entering the TUI. Every controlled exit —
// ordinary, cancellation, or a controlled application failure —
// terminates and reaps the child and restores the terminal; a controlled
// failure additionally writes its sanitized diagnostic exactly once,
// after restoration, through writeFailureDiag.
func Run(ctx context.Context, cfg Config, opts ...Option) int {
	errOut := cfg.Err
	if errOut == nil {
		errOut = os.Stderr
	}
	start := cfg.Start
	if start == nil {
		start = spawn
	}
	child, err := start(ctx, cfg.Args, cfg.Dir)
	if err != nil {
		fmt.Fprintf(errOut, "vrg: cannot start ripgrep: %s\n", safepresentation.EscapePath([]byte(err.Error())))
		return 2
	}
	var o options
	for _, fn := range opts {
		fn(&o)
	}
	if o.reap != nil {
		// Both the model's quit command and the post-program safety net
		// run the wait/reap path; the report must fire exactly once.
		var once sync.Once
		report := o.reap
		o.reap = func(r Result) { once.Do(func() { report(r) }) }
	}
	m := newModel(cfg, o, child)
	final, err := tea.NewProgram(m, tea.WithContext(ctx)).Run()
	// Exits that bypass the model — interrupt, program error, a caught
	// panic — still owe the child termination and reaping.
	reapChild(child, o.reap)
	if err != nil {
		if errors.Is(err, tea.ErrInterrupted) {
			return 130
		}
		writeFailureDiag(errOut, err)
		return 2
	}
	if fm, ok := final.(*model); ok {
		if fm.failErr != nil {
			writeFailureDiag(errOut, fm.failErr)
			return 2
		}
		return fm.status
	}
	return 2
}

// writeFailureDiag is the single post-restoration stderr writer: it
// emits one sanitized controlled-failure diagnostic, and only runs after
// the program has returned and the terminal is restored.
func writeFailureDiag(w io.Writer, err error) {
	fmt.Fprintf(w, "vrg: %s\n", safepresentation.EscapePath([]byte(err.Error())))
}

// center pads s into a w×h field, centered horizontally and vertically.
// Unknown dimensions degrade to the bare string.
func center(s string, w, h int) string {
	if w <= 0 || h <= 0 {
		return s
	}
	pad := 0
	if n := utf8.RuneCountInString(s); n < w {
		pad = (w - n) / 2
	}
	top := 0
	if h > 1 {
		top = (h - 1) / 2
	}
	return strings.Repeat("\n", top) + strings.Repeat(" ", pad) + s
}

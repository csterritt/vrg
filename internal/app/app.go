package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/cli"
	"vrg/internal/searchindex"
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
}

// WithGate holds index preparation until fn returns.
func WithGate(fn func()) Option { return func(o *options) { o.gate = fn } }

// WithCollectAck runs fn once the child's output is fully collected.
func WithCollectAck(fn func()) Option { return func(o *options) { o.collectAck = fn } }

type state int

const (
	stateSearching state = iota
	stateSummary
)

// model is the Bubble Tea model: "Searching…" while collection and index
// preparation run off the update path, then the interim summary.
type model struct {
	cfg   Config
	opts  options
	child Child

	state          state
	width, height  int
	files, matched int
	status         int
}

func newModel(cfg Config, opts options, child Child) *model {
	return &model{cfg: cfg, opts: opts, child: child, state: stateSearching}
}

// searchDoneMsg carries the collected result and prepared index from the
// collection command into Update.
type searchDoneMsg struct {
	res Result
	idx *searchindex.Index
}

// Init starts collection: the command blocks on the child, runs the
// collection acknowledgement and the preparation gate, then builds the
// index — all off the UI update path so the model stays responsive.
func (m *model) Init() tea.Cmd {
	child := m.child
	opts := m.opts
	dir := m.cfg.Dir
	return func() tea.Msg {
		res := child.Wait()
		if opts.collectAck != nil {
			opts.collectAck()
		}
		if opts.gate != nil {
			opts.gate()
		}
		return searchDoneMsg{res: res, idx: searchindex.Build(res.Stdout, dir)}
	}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case searchDoneMsg:
		m.state = stateSummary
		m.files = len(msg.idx.Files)
		for _, f := range msg.idx.Files {
			m.matched += len(f.Stops)
		}
	case tea.KeyPressMsg:
		// Cancellation and its statuses are Issue #4's; while searching,
		// q is inert.
		if m.state == stateSummary && msg.Text == "q" {
			m.status = 0
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *model) View() tea.View {
	var s string
	if m.state == stateSummary {
		s = fmt.Sprintf("%d files, %d matched lines", m.files, m.matched)
	} else {
		s = "Searching…"
	}
	return tea.NewView(center(s, m.width, m.height))
}

// Run is the whole search lifecycle: spawn the child, show "Searching…"
// until the stream is collected and the index prepared, then the interim
// summary. A start failure prints a sanitized diagnostic to cfg.Err and
// returns exit 2 without entering the TUI.
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
		fmt.Fprintf(errOut, "vrg: cannot start ripgrep: %s\n", cli.Escape(err.Error()))
		return 2
	}
	var o options
	for _, fn := range opts {
		fn(&o)
	}
	m := newModel(cfg, o, child)
	final, err := tea.NewProgram(m, tea.WithContext(ctx)).Run()
	if err != nil {
		fmt.Fprintf(errOut, "vrg: %s\n", cli.Escape(err.Error()))
		return 2
	}
	if fm, ok := final.(*model); ok {
		return fm.status
	}
	return 2
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

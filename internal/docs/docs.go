// Package docs is the shared structured source for vrg's user-facing
// documentation (Issue #34): the scale, record-limit, and memory
// statements that both the repository README and the TUI help-overlay
// footer render, so neither sink can drift from the other. The
// documentation synchronization tests assert each statement lands
// verbatim in both sinks.
package docs

import "strings"

const (
	// ScaleStatement introduces the scale examples: each stands alone,
	// never a simultaneous capacity guarantee.
	ScaleStatement = "The documented scale examples are independent, not simultaneous capacity guarantees."
	// AssumptionStatement qualifies the ~50 MB example's content:
	// UTF-8 (or near-UTF-8) with ordinary line lengths. A single very
	// long line that ripgrep must emit as base64 bytes expands by
	// roughly a third in the JSON stream and can push one match record
	// over the limit even when the file itself is under 50 MB.
	AssumptionStatement = "The ~50 MB file example assumes UTF-8 (or near-UTF-8) content with ordinary line lengths: a single very long line that ripgrep must emit as base64 bytes expands by roughly a third in the JSON stream and can push one match record over the 64 MiB limit even when the file itself is under 50 MB."
	// RecordLimitStatement states the per-record bound and the
	// oversized-record disposition: skipped and counted, never
	// truncated, the path named when recoverable.
	RecordLimitStatement = "Every JSON record is limited to 64 MiB: an oversized record is skipped and counted, never silently truncated, and the oversized-record diagnostic names the affected file's path when it can be recovered."
	// MemoryStatement states the retention and termination limits:
	// session-long buffers, no eviction, no aggregate bound, no
	// reliable OOM recovery, no guaranteed terminal cleanup under
	// forced termination.
	MemoryStatement = "Loaded file buffers are retained for the whole session: no eviction, no aggregate memory bound, and no reliable OOM recovery; large searches or many visited files may exhaust memory and terminate the process, and forced termination cannot guarantee terminal cleanup."
)

// ScaleItems are the three independent scale examples ScaleStatement
// introduces; each is a standalone example, not a combined capacity.
var ScaleItems = []string{
	"about 10,000 matched files",
	"about 100,000 matched lines",
	"individual files around 50 MB",
}

// Statements is the ordered required-statement list the documentation
// synchronization tests iterate: each statement must survive verbatim
// in every sink, so deleting one fails the suite.
var Statements = []string{
	ScaleStatement,
	AssumptionStatement,
	RecordLimitStatement,
	MemoryStatement,
}

// Footer renders the TUI help-overlay footer note from the shared
// source: the scale statement and items followed by the assumption,
// record-limit, and memory statements as one compact paragraph under
// the binding table. The help renderer treats the note as substituted
// text and routes it through the safe-presentation diagnostic escaper
// like every runtime string.
func Footer() string {
	items := make([]string, len(ScaleItems))
	copy(items, ScaleItems)
	items[len(items)-1] = "and " + items[len(items)-1]
	return "Scale and memory limits — " + ScaleStatement + " They are " +
		strings.Join(items, ", ") + ". " +
		AssumptionStatement + " " + RecordLimitStatement + " " + MemoryStatement
}

// Limits renders the README's scale-and-memory-limits section body
// from the same shared source: the committed README must carry this
// text verbatim — the synchronization test asserts it — so the section
// cannot drift from the source or from the help footer.
func Limits() string {
	var b strings.Builder
	b.WriteString(ScaleStatement + "\n\n")
	for _, item := range ScaleItems {
		b.WriteString("- " + item + "\n")
	}
	b.WriteString("\n" + AssumptionStatement + "\n\n" +
		RecordLimitStatement + "\n\n" + MemoryStatement + "\n")
	return b.String()
}

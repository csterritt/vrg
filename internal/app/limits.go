package app

import "strings"

// scaleLimitNote is the single structured source for the scale,
// record-limit, and memory documentation: the help overlay footer
// renders it and the README carries it verbatim, with the Issue 34
// documentation test asserting every statement in both sinks so
// neither can drift from the other.
var scaleLimitNote = []string{
	"Scale examples: approximately 10,000 matched files, 100,000 matched lines, or individual files around 50 MB. The examples are independent, not simultaneous capacity guarantees.",
	"The 50 MB file example assumes UTF-8 content with ordinary line lengths. Lines ripgrep must emit as base64 bytes expand by roughly a third, so a single match record can exceed the 64 MiB record limit even when the file itself is under 50 MB.",
	"An oversized record is skipped and counted; the diagnostic names the path when it was parsed before the limit was reached.",
	"Loaded buffers are retained for the session: there is no eviction, no aggregate memory bound, no reliable OOM recovery, and no guaranteed terminal cleanup under forced termination.",
}

// helpFooterNote renders the shared scale-and-limits statements as the
// help overlay footer's note.
func helpFooterNote() string {
	return strings.Join(scaleLimitNote, "\n")
}

package app_test

import (
	"os"
	"strings"
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
	"vrg/internal/viewport"
)

// --- Grapheme cluster highlight expansion through indicators (Issue #21) ---

// makeExpandedBufLine creates a filebuffer.Line whose ByteCells and
// Highlights are already expanded to grapheme-cluster boundaries (the
// post-Issue-#21 form FileBuffer produces). Clusters come from the
// shared policy.
func makeExpandedBufLine(num int, raw string, highlights ...[2]int) filebuffer.Line {
	d := safepresentation.EscapeContent([]byte(raw))
	return filebuffer.Line{
		Number:     num,
		Display:    d.Text,
		ByteCells:  d.ByteCells,
		Highlights: highlights,
		Clusters:   safepresentation.GraphemeClusters(d.Text),
	}
}

// TestIndicatorMidClusterMatchHiddenLeft verifies that a match starting
// mid-cluster, where the actual cluster start is hidden left, counts as
// hidden-left and produces a left `*` indicator (Issue #21). The
// submatch covers only the combining mark bytes; the highlight expands
// to the whole é cluster. When the cluster start is hidden left, the
// indicator must show `*`.
func TestIndicatorMidClusterMatchHiddenLeft(t *testing.T) {
	// Build a line: 50 ASCII chars + "e\u0301" (é, 1 cell) + 200 ASCII.
	// The é cluster is at cell 50. The highlight expands to [50, 51).
	// Pan right by 51: the é cluster (cell 50) is hidden left. The
	// indicator must show `*` (match entirely hidden left).
	prefix := strings.Repeat("a", 50)
	suffix := strings.Repeat("a", 200)
	raw := prefix + "e\u0301" + suffix + "\n"
	// Submatch on the combining mark only: bytes 51-53 (0xCC 0x81).
	// After expansion, the highlight covers the whole é cluster [50, 51).
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", raw, 1, subSpec{"\u0301", 51, 53}),
	)
	// Build the line with the expanded highlight [50, 51).
	line := makeExpandedBufLine(1, raw, [2]int{50, 51})
	buf := makeBuf([]filebuffer.Line{line}, 1, 3)
	m := setupBrowseIndicatorMode(t, idx, buf, viewport.WrapOff)
	m = panRight(t, m, 51)
	view := viewContent(m)
	if !strings.Contains(view, "1* ") {
		t.Fatalf("view does not contain left '*' (mid-cluster match, cluster start hidden left): %q", view)
	}
}

// TestIndicatorMidClusterMatchVisibleNoStar verifies that a match
// starting mid-cluster, where the expanded cluster is fully visible,
// produces no left `*` indicator (Issue #21). The submatch covers only
// the combining mark; the highlight expands to the whole cluster; the
// cluster is visible, so no hidden-match indicator.
func TestIndicatorMidClusterMatchVisibleNoStar(t *testing.T) {
	// é cluster at cell 50. Pan right by 49: the cluster [50, 51) is
	// within the window [49, 49+76). No hidden-match indicator.
	prefix := strings.Repeat("a", 50)
	suffix := strings.Repeat("a", 200)
	raw := prefix + "e\u0301" + suffix + "\n"
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", raw, 1, subSpec{"\u0301", 51, 53}),
	)
	line := makeExpandedBufLine(1, raw, [2]int{50, 51})
	buf := makeBuf([]filebuffer.Line{line}, 1, 3)
	m := setupBrowseIndicatorMode(t, idx, buf, viewport.WrapOff)
	m = panRight(t, m, 49)
	view := viewContent(m)
	if strings.Contains(view, "1* ") {
		t.Fatalf("view contains left '*' (expanded cluster visible, no hidden match): %q", view)
	}
}

// TestIndicatorWideClusterMatchHiddenLeft verifies that a match on a
// wide cluster, where the cluster start is hidden left, counts as
// hidden-left and produces a left `*` indicator (Issue #21).
func TestIndicatorWideClusterMatchHiddenLeft(t *testing.T) {
	// 50 ASCII chars + "中" (2 cells) + 200 ASCII. The 中 cluster is at
	// cells [50, 52). Pan right by 51: the cluster start (cell 50) is
	// hidden left. The highlight expands to [50, 52). The indicator
	// must show `*`.
	prefix := strings.Repeat("a", 50)
	suffix := strings.Repeat("a", 200)
	raw := prefix + "中" + suffix + "\n"
	// Submatch on the last byte of 中 (byte 52): partially covers the
	// wide cluster. After expansion, highlight [50, 52).
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", raw, 1, subSpec{"\xad", 52, 53}),
	)
	line := makeExpandedBufLine(1, raw, [2]int{50, 52})
	buf := makeBuf([]filebuffer.Line{line}, 1, 3)
	m := setupBrowseIndicatorMode(t, idx, buf, viewport.WrapOff)
	m = panRight(t, m, 51)
	view := viewContent(m)
	if !strings.Contains(view, "1* ") {
		t.Fatalf("view does not contain left '*' (wide cluster match, cluster start hidden left): %q", view)
	}
}

// TestIndicatorWideClusterMatchPartiallyVisibleNoStar verifies that a
// match on a wide cluster, where the cluster is partially visible (the
// cluster start is visible but the cluster straddles the right edge),
// does not produce a right `*` indicator because the cluster has a
// non-blank visible cell (Issue #21). The highlight expands to the whole
// wide cluster; a partially visible cluster (not split) counts as
// visible.
func TestIndicatorWideClusterMatchPartiallyVisibleNoStar(t *testing.T) {
	// 50 ASCII chars + "中" (2 cells) + 200 ASCII. The 中 cluster is at
	// cells [50, 52). Pan right by 50: window [50, 50+76). The cluster
	// [50, 52) is fully visible. No hidden-match indicator.
	prefix := strings.Repeat("a", 50)
	suffix := strings.Repeat("a", 200)
	raw := prefix + "中" + suffix + "\n"
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", raw, 1, subSpec{"\xad", 52, 53}),
	)
	line := makeExpandedBufLine(1, raw, [2]int{50, 52})
	buf := makeBuf([]filebuffer.Line{line}, 1, 3)
	m := setupBrowseIndicatorMode(t, idx, buf, viewport.WrapOff)
	m = panRight(t, m, 50)
	view := viewContent(m)
	if strings.Contains(view, "1* ") {
		t.Fatalf("view contains left '*' (wide cluster fully visible): %q", view)
	}
}

// TestIndicatorMidClusterMatchFromLoad verifies the end-to-end path:
// FileBuffer.Load expands a combining-only submatch to the whole base
// cluster, and the indicator consumes the expanded span. When the
// cluster start is hidden left, the indicator must show `*`. This test
// uses filebuffer.Load (the production path) rather than a pre-expanded
// line, so it exercises the Issue #21 expansion implementation.
func TestIndicatorMidClusterMatchFromLoad(t *testing.T) {
	prefix := strings.Repeat("a", 50)
	suffix := strings.Repeat("a", 200)
	raw := prefix + "e\u0301" + suffix + "\n"
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", raw, 1, subSpec{"\u0301", 51, 53}),
	)
	// Use the production filebuffer.Load path so the expansion runs.
	dir := t.TempDir()
	p := dir + "/src/a.go"
	if err := os.MkdirAll(dir+"/src", 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(raw), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	var stops []searchindex.Stop
	for _, s := range idx.Stops() {
		if string(s.RawPath) == "src/a.go" {
			stops = append(stops, s)
		}
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	buf.GutterWidth = 3
	m := setupBrowseIndicatorMode(t, idx, buf, viewport.WrapOff)
	m = panRight(t, m, 51)
	view := viewContent(m)
	if !strings.Contains(view, "1* ") {
		t.Fatalf("view does not contain left '*' (mid-cluster match from Load, cluster start hidden left): %q", view)
	}
}

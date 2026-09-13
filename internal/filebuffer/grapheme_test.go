package filebuffer_test

import (
	"os"
	"path/filepath"
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
)

// --- Grapheme cluster tests (Issue #16) ---

// TestGraphemeClustersASCII verifies that ASCII text produces one
// 1-cell cluster per character. The shared grapheme segmentation policy
// is the source FileBuffer exposes and Viewport consumes.
func TestGraphemeClustersASCII(t *testing.T) {
	clusters := safepresentation.GraphemeClusters("abc")
	if len(clusters) != 3 {
		t.Fatalf("clusters = %d, want 3", len(clusters))
	}
	for i, c := range clusters {
		if c.Width != 1 {
			t.Fatalf("cluster %d Width = %d, want 1", i, c.Width)
		}
	}
	if clusters[0].StartByte != 0 || clusters[0].EndByte != 1 {
		t.Fatalf("cluster 0 = [%d, %d), want [0, 1)", clusters[0].StartByte, clusters[0].EndByte)
	}
	if clusters[2].StartByte != 2 || clusters[2].EndByte != 3 {
		t.Fatalf("cluster 2 = [%d, %d), want [2, 3)", clusters[2].StartByte, clusters[2].EndByte)
	}
}

// TestGraphemeClustersWide verifies that wide characters (East Asian
// Wide/Fullwidth) produce 2-cell clusters. The cell-width policy must
// count CJK ideographs and fullwidth characters as two terminal cells.
func TestGraphemeClustersWide(t *testing.T) {
	// U+4E2D (中) is a CJK ideograph, East Asian Wide = 2 cells.
	clusters := safepresentation.GraphemeClusters("中")
	if len(clusters) != 1 {
		t.Fatalf("clusters = %d, want 1", len(clusters))
	}
	if clusters[0].Width != 2 {
		t.Fatalf("wide cluster Width = %d, want 2", clusters[0].Width)
	}
}

// TestGraphemeClustersCombining verifies that a combining mark stays
// attached to its base character as one cluster. The cluster width
// equals the base character's width (the combining mark adds no
// advance). This is the grapheme-boundary wrapping contract: a
// combining mark must not be separated from its base.
func TestGraphemeClustersCombining(t *testing.T) {
	// U+0065 (e) + U+0301 (combining acute) = é as two code points,
	// one grapheme cluster, width 1.
	clusters := safepresentation.GraphemeClusters("e\u0301")
	if len(clusters) != 1 {
		t.Fatalf("clusters = %d, want 1 (base + combining = one cluster)", len(clusters))
	}
	if clusters[0].Width != 1 {
		t.Fatalf("combining cluster Width = %d, want 1 (base width, combining adds no advance)", clusters[0].Width)
	}
}

// TestGraphemeClustersWideWithCombining verifies that a wide character
// with a combining mark forms one 2-cell cluster.
func TestGraphemeClustersWideWithCombining(t *testing.T) {
	// U+4E2D (中) + U+0301 (combining acute) = one cluster, width 2.
	clusters := safepresentation.GraphemeClusters("中\u0301")
	if len(clusters) != 1 {
		t.Fatalf("clusters = %d, want 1 (wide base + combining = one cluster)", len(clusters))
	}
	if clusters[0].Width != 2 {
		t.Fatalf("wide+combining cluster Width = %d, want 2", clusters[0].Width)
	}
}

// TestGraphemeClustersMixed verifies a mix of ASCII, wide, and
// combining content produces correct cluster boundaries and widths.
func TestGraphemeClustersMixed(t *testing.T) {
	// "a中b\u0301" = a (1 cell), 中 (2 cells), b+combining (1 cell) = 3 clusters
	display := "a中b\u0301"
	clusters := safepresentation.GraphemeClusters(display)
	if len(clusters) != 3 {
		t.Fatalf("clusters = %d, want 3", len(clusters))
	}
	want := []int{1, 2, 1}
	for i, w := range want {
		if clusters[i].Width != w {
			t.Fatalf("cluster %d Width = %d, want %d", i, clusters[i].Width, w)
		}
	}
}

// --- Tab expansion tests (Issue #16) ---

// TestTabExpansionEightColumnStops verifies that tabs expand to the
// next multiple of 8 source-display columns, replacing Issue #5's
// provisional → placeholder. A tab at column 1 expands to 7 spaces
// (reaching column 8); a tab at column 0 expands to 8 spaces.
func TestTabExpansionEightColumnStops(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "tabs.txt")
	if err := os.WriteFile(p, []byte("a\tb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	// "a" at column 0, tab at column 1 expands to 7 spaces (columns 1-7),
	// "b" at column 8. Display: "a       b" (a + 7 spaces + b).
	want := "a       b"
	if buf.Lines[0].Display != want {
		t.Fatalf("Display = %q, want %q (tab expanded to 8-column stop)", buf.Lines[0].Display, want)
	}
}

// TestTabExpansionAtColumnZero verifies that a tab at column 0 expands
// to 8 spaces (the full first tab stop).
func TestTabExpansionAtColumnZero(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "tabs.txt")
	if err := os.WriteFile(p, []byte("\tx\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	// Tab at column 0 expands to 8 spaces, "x" at column 8.
	want := "        x"
	if buf.Lines[0].Display != want {
		t.Fatalf("Display = %q, want %q (tab at col 0 expands to 8 spaces)", buf.Lines[0].Display, want)
	}
}

// TestTabExpansionByteCells verifies that the tab byte maps to the
// correct display cell range. A tab at column 1 maps to cells [1, 8).
func TestTabExpansionByteCells(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "tabs.txt")
	if err := os.WriteFile(p, []byte("a\tb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	bc := buf.Lines[0].ByteCells
	// Raw bytes: 'a' (0), '\t' (1), 'b' (2), '\n' (3).
	// 'a' → [0, 1), '\t' → [1, 8), 'b' → [8, 9), '\n' → [9, 9).
	if len(bc) != 4 {
		t.Fatalf("ByteCells len = %d, want 4", len(bc))
	}
	if bc[0] != [2]int{0, 1} {
		t.Fatalf("byte 0 ('a') = %v, want [0, 1)", bc[0])
	}
	if bc[1] != [2]int{1, 8} {
		t.Fatalf("byte 1 ('\\t') = %v, want [1, 8) (tab expands to 7 cells)", bc[1])
	}
	if bc[2] != [2]int{8, 9} {
		t.Fatalf("byte 2 ('b') = %v, want [8, 9)", bc[2])
	}
}

// TestTabExpansionMultipleTabs verifies consecutive tabs expand to
// successive 8-column stops.
func TestTabExpansionMultipleTabs(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "tabs.txt")
	if err := os.WriteFile(p, []byte("\t\tx\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	// First tab: col 0 → 8 spaces (col 8). Second tab: col 8 → 8 spaces
	// (col 16). "x" at col 16. Display: 16 spaces + "x".
	want := "                x"
	if buf.Lines[0].Display != want {
		t.Fatalf("Display = %q, want %q (two tabs to 16 spaces + x)", buf.Lines[0].Display, want)
	}
}

// TestTabExpansionIndependentOfGutter verifies that tab stops are
// independent of the gutter: they count from the start of the line
// content (column 0), not from the gutter position.
func TestTabExpansionIndependentOfGutter(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "tabs.txt")
	if err := os.WriteFile(p, []byte("x\t\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	// "x" at col 0, tab at col 1 → 7 spaces (col 8). Display: "x       ".
	want := "x       "
	if buf.Lines[0].Display != want {
		t.Fatalf("Display = %q, want %q", buf.Lines[0].Display, want)
	}
}

// --- Load populates clusters (Issue #16) ---

// TestLoadPopulatesClusters verifies that Load populates Line.Clusters
// using the shared grapheme segmentation policy. Viewport consumes
// these clusters for wrapping without re-deriving.
func TestLoadPopulatesClusters(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(p, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(buf.Lines[0].Clusters) == 0 {
		t.Fatal("Line.Clusters is empty, want populated by Load")
	}
	if len(buf.Lines[0].Clusters) != 5 {
		t.Fatalf("Clusters len = %d, want 5 (one per ASCII char)", len(buf.Lines[0].Clusters))
	}
}

// TestLoadPopulatesClustersWide verifies that Load populates clusters
// with correct widths for wide characters.
func TestLoadPopulatesClustersWide(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(p, []byte("中\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	clusters := buf.Lines[0].Clusters
	if len(clusters) != 1 {
		t.Fatalf("Clusters len = %d, want 1", len(clusters))
	}
	if clusters[0].Width != 2 {
		t.Fatalf("wide cluster Width = %d, want 2", clusters[0].Width)
	}
}

// TestLoadPopulatesClustersCombining verifies that Load populates
// clusters that keep combining marks with their base.
func TestLoadPopulatesClustersCombining(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(p, []byte("e\u0301\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	clusters := buf.Lines[0].Clusters
	if len(clusters) != 1 {
		t.Fatalf("Clusters len = %d, want 1 (base + combining = one cluster)", len(clusters))
	}
	if clusters[0].Width != 1 {
		t.Fatalf("combining cluster Width = %d, want 1", clusters[0].Width)
	}
}

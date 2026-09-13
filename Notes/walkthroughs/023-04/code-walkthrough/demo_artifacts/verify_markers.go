// verify_markers.go: load a file through the production filebuffer.Load
// path with a zero-width submatch at the given byte position and print
// the display text, highlights, byte cells, and cluster widths so the
// Issue #23 marker contracts can be inspected.
package main

import (
	"fmt"
	"os"

	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: verify_markers <path> <bytePos>")
		os.Exit(2)
	}
	path := os.Args[1]
	var bytePos int
	fmt.Sscanf(os.Args[2], "%d", &bytePos)
	stops := []searchindex.Stop{
		{
			RawPath:    []byte(path),
			LineNumber: 1,
			Line:       nil,
			Submatches: []searchindex.Submatch{{Match: []byte{}, Start: bytePos, End: bytePos}},
		},
	}
	buf, err := filebuffer.Load([]byte(path), stops)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Load error:", err)
		os.Exit(1)
	}
	for _, line := range buf.Lines {
		fmt.Printf("line %d display=%q\n", line.Number, line.Display)
		fmt.Printf("  highlights=%v\n", line.Highlights)
		width := 0
		for _, c := range line.Clusters {
			width += c.Width
		}
		fmt.Printf("  cluster_count=%d cluster_width=%d\n", len(line.Clusters), width)
		for i, c := range line.Clusters {
			fmt.Printf("    cluster[%d] bytes=[%d,%d) width=%d\n", i, c.StartByte, c.EndByte, c.Width)
		}
	}
}

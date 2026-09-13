package filebuffer

import (
	"fmt"
	"os"

	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
)

// Buffer is a loaded, decoded, and mapped file ready for display. The
// completion message carries a fully prepared buffer so Update does no
// full-file work.
type Buffer struct {
	Lines       []Line
	LineCount   int
	GutterWidth int
}

// Line is one display-ready source line.
type Line struct {
	// Number is the 1-based source line number.
	Number int
	// Display is the escaped display text (no line terminator).
	Display string
	// ByteCells maps each original byte index to its [start, end) display
	// cell range.
	ByteCells [][2]int
	// Highlights are the display cell ranges to render in inverse video.
	Highlights [][2]int
}

// Load reads, decodes, and maps a file's bytes into a display-ready
// buffer. The stops provide the match data for this file (filtered by
// the caller to the file's raw path). The returned buffer is fully
// prepared so the caller's Update does no full-file work.
func Load(path []byte, stops []searchindex.Stop) (*Buffer, error) {
	data, err := os.ReadFile(string(path))
	if err != nil {
		return nil, err
	}

	rawLines := splitLines(data)
	lineCount := len(rawLines)
	gw := gutterWidth(lineCount)

	stopsByLine := make(map[int][]searchindex.Stop)
	for _, s := range stops {
		stopsByLine[s.LineNumber] = append(stopsByLine[s.LineNumber], s)
	}

	lines := make([]Line, 0, lineCount)
	for i, rawLine := range rawLines {
		lineNum := i + 1
		d := safepresentation.EscapeContent(rawLine)

		var highlights [][2]int
		for _, s := range stopsByLine[lineNum] {
			for _, sm := range s.Submatches {
				if sm.Start >= sm.End {
					continue
				}
				if sm.Start >= len(d.ByteCells) {
					continue
				}
				end := sm.End - 1
				if end >= len(d.ByteCells) {
					end = len(d.ByteCells) - 1
				}
				hl := [2]int{d.ByteCells[sm.Start][0], d.ByteCells[end][1]}
				if hl[0] < hl[1] {
					highlights = append(highlights, hl)
				}
			}
		}

		lines = append(lines, Line{
			Number:     lineNum,
			Display:    d.Text,
			ByteCells:  d.ByteCells,
			Highlights: highlights,
		})
	}

	return &Buffer{
		Lines:       lines,
		LineCount:   lineCount,
		GutterWidth: gw,
	}, nil
}

// splitLines splits file bytes into lines, each including its terminator.
// LF and CRLF are line terminators; a standalone CR is not. A trailing
// terminator does not produce an extra empty line.
func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i := 0; i < len(data); {
		if data[i] == '\n' {
			lines = append(lines, data[start:i+1])
			start = i + 1
			i++
		} else if data[i] == '\r' && i+1 < len(data) && data[i+1] == '\n' {
			lines = append(lines, data[start:i+2])
			start = i + 2
			i += 2
		} else {
			i++
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}

// gutterWidth returns the digit count of the largest line number plus
// two spaces, with a minimum of one digit.
func gutterWidth(lineCount int) int {
	digits := 1
	if lineCount > 0 {
		digits = len(fmt.Sprintf("%d", lineCount))
	}
	return digits + 2
}

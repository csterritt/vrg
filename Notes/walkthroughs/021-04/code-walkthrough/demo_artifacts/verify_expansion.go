//go:build ignore

package main

import (
	"bytes"
	"fmt"
	"os"
	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
)

func main() {
	path := os.Args[1]
	pattern := os.Args[2]
	content, _ := os.ReadFile(path)
	lines := splitLines(content)
	var stops []searchindex.Stop
	for i, line := range lines {
		subs := findSubmatches(line, []byte(pattern))
		if len(subs) == 0 {
			continue
		}
		stops = append(stops, searchindex.Stop{
			RawPath:    []byte(path),
			Path:       []byte(path),
			LineNumber: i + 1,
			Line:       line,
			Submatches: subs,
		})
	}
	if len(stops) == 0 {
		fmt.Println("no matches")
		return
	}
	buf, err := filebuffer.Load([]byte(path), stops)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load error: %v\n", err)
		os.Exit(1)
	}
	for _, line := range buf.Lines {
		if len(line.Highlights) == 0 {
			continue
		}
		fmt.Printf("line %d: display=%q\n", line.Number, line.Display)
		fmt.Printf("  highlights=%v\n", line.Highlights)
		fmt.Printf("  clusters=%v\n", line.Clusters)
	}
}

func splitLines(content []byte) [][]byte {
	var lines [][]byte
	for len(content) > 0 {
		idx := bytes.IndexByte(content, '\n')
		if idx < 0 {
			lines = append(lines, content)
			break
		}
		lines = append(lines, content[:idx+1])
		content = content[idx+1:]
	}
	return lines
}

func findSubmatches(content, pattern []byte) []searchindex.Submatch {
	var subs []searchindex.Submatch
	start := 0
	for {
		idx := indexOf(content[start:], pattern)
		if idx < 0 {
			break
		}
		s := start + idx
		e := s + len(pattern)
		subs = append(subs, searchindex.Submatch{Match: pattern, Start: s, End: e})
		start = e
	}
	return subs
}

func indexOf(haystack, needle []byte) int {
	if len(needle) == 0 || len(needle) > len(haystack) {
		return -1
	}
	for i := 0; i <= len(haystack)-len(needle); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

package safepresentation

import (
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

// CellWidth returns the terminal-cell width of s under the shared
// uniseg grapheme policy — the single ANSI-aware measure every
// display-geometry consumer routes through (Issue #39). ANSI CSI
// sequences — the SGR runs the theme wraps around content — occupy no
// cells and are skipped, so the measure of a styled string is the
// measure of its content. The escapers never emit a raw ESC, so no
// legitimate display string opens a sequence accidentally.
func CellWidth(s string) (w int) {
	rest := s
	state := -1
	for len(rest) > 0 {
		if n := csiPrefix(rest); n > 0 {
			rest = rest[n:]
			state = -1
			continue
		}
		var cw int
		_, rest, cw, state = uniseg.FirstGraphemeClusterInString(rest, state)
		w += cw
	}
	return w
}

// csiPrefix is the length of the ANSI CSI sequence opening s — ESC [
// then parameter and intermediate bytes closed by one final byte in
// 0x40–0x7E — or zero when s does not open one. An unterminated
// sequence consumes the rest of the string.
func csiPrefix(s string) int {
	if len(s) < 2 || s[0] != 0x1b || s[1] != '[' {
		return 0
	}
	i := 2
	for i < len(s) && (s[i] < 0x40 || s[i] > 0x7e) {
		i++
	}
	if i < len(s) {
		i++
	}
	return i
}

// decodeRune decodes the first rune in b, returning utf8.RuneError with
// size 1 for an invalid byte. Keeping the decoder in this one file is
// the Issue #39 boundary: display-geometry code elsewhere never decodes
// runes itself.
func decodeRune(b []byte) (rune, int) { return utf8.DecodeRune(b) }

// decodeRuneInString is the string form of decodeRune.
func decodeRuneInString(s string) (rune, int) { return utf8.DecodeRuneInString(s) }

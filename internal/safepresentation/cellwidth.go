package safepresentation

import (
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

// CellWidth returns the terminal-cell width of s under the shared
// uniseg policy.
func CellWidth(s string) int { return uniseg.StringWidth(s) }

// decodeRune decodes the first rune in b, returning utf8.RuneError with
// size 1 for an invalid byte. Keeping the decoder in this one file is
// the Issue #39 boundary: display-geometry code elsewhere never decodes
// runes itself.
func decodeRune(b []byte) (rune, int) { return utf8.DecodeRune(b) }

// decodeRuneInString is the string form of decodeRune.
func decodeRuneInString(s string) (rune, int) { return utf8.DecodeRuneInString(s) }

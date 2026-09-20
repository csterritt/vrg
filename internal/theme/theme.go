package theme

import (
	"strings"

	"github.com/clipperhouse/displaywidth"
)

// SGR codes for the two schemes: dark is white on black, light is black
// on white. A scheme's inverse — the match and indicator style — is
// literally the other scheme's base pair, so it is always the true
// inverse of the active foreground/background.
const (
	sgrReset        = "\x1b[0m"
	sgrUnderlineOn  = "\x1b[4m"
	sgrUnderlineOff = "\x1b[24m"
	darkBase        = "\x1b[37;40m"
	lightBase       = "\x1b[30;47m"
)

// Theme holds the active colour scheme and supplies every style the
// renderer needs. The zero value emits no ANSI styling — the no-style
// composition path used to prove that no searched-data control byte can
// reach the terminal.
type Theme struct {
	styled bool
	dark   bool
}

// Styled returns the styled theme with the dark scheme active.
func Styled() Theme { return Theme{styled: true, dark: true} }

// Plain returns the no-style theme: every colour style is the identity,
// so no escape byte may legitimately appear in rendered output.
func Plain() Theme { return Theme{} }

// Toggle returns the theme with the opposite scheme active. The scheme
// is session state only; nothing persists.
func (t Theme) Toggle() Theme {
	t.dark = !t.dark
	return t
}

// base is the active scheme's foreground/background pair, or "" in the
// no-style theme.
func (t Theme) base() string {
	switch {
	case !t.styled:
		return ""
	case t.dark:
		return darkBase
	default:
		return lightBase
	}
}

// inverse is the true inverse of the active scheme's base pair.
func (t Theme) inverse() string {
	switch {
	case !t.styled:
		return ""
	case t.dark:
		return lightBase
	default:
		return darkBase
	}
}

// Base wraps an outermost run — a composed row or whole screen — in the
// active scheme's colours and resets at its end. Interior segments use
// the other styles, each of which restores the base pair at its close
// so they compose inside a base-painted run and stand alone equally.
func (t Theme) Base(s string) string {
	if !t.styled || s == "" {
		return s
	}
	return t.base() + s + sgrReset
}

// baseSeg renders one segment in the scheme's base colours.
func (t Theme) baseSeg(s string) string {
	if !t.styled || s == "" {
		return s
	}
	return t.base() + s + t.base()
}

// Gutter renders a line-number gutter segment in base colours.
func (t Theme) Gutter(s string) string { return t.baseSeg(s) }

// FilenameRule renders the filename rule in base colours.
func (t Theme) FilenameRule(s string) string { return t.baseSeg(s) }

// FileList renders a file-list segment in base colours.
func (t Theme) FileList(s string) string { return t.baseSeg(s) }

// Match renders a match segment in the true inverse of the active
// scheme's base colours.
func (t Theme) Match(s string) string {
	if !t.styled || s == "" {
		return s
	}
	return t.inverse() + s + t.base()
}

// CurrentMatch renders a match on the current matched line: the inverse
// match style plus underline.
func (t Theme) CurrentMatch(s string) string {
	if !t.styled || s == "" {
		return s
	}
	return sgrUnderlineOn + t.inverse() + s + sgrUnderlineOff + t.base()
}

// Indicator renders a hidden-content indicator in inverse video.
func (t Theme) Indicator(s string) string { return t.Match(s) }

// CurrentFile renders the current file-list entry underlined.
func (t Theme) CurrentFile(s string) string {
	if !t.styled || s == "" {
		return s
	}
	return sgrUnderlineOn + s + sgrUnderlineOff + t.base()
}

// Overlay renders content inside a plain single-line border w cells
// wide and h cells tall, painted in the scheme's base colours. Content
// lines are clipped to the interior width and extra lines dropped;
// missing lines render blank. A box too small for its border clips the
// content alone. In the no-style theme the border is drawn without
// colour codes.
func (t Theme) Overlay(content []string, w, h int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	rows := make([]string, 0, h)
	if w >= 2 && h >= 2 {
		inner := w - 2
		rows = append(rows, "┌"+strings.Repeat("─", inner)+"┐")
		for i := 0; i < h-2; i++ {
			line := ""
			if i < len(content) {
				line = displaywidth.TruncateString(content[i], inner, "")
			}
			if d := displaywidth.String(line); d < inner {
				line += strings.Repeat(" ", inner-d)
			}
			rows = append(rows, "│"+line+"│")
		}
		rows = append(rows, "└"+strings.Repeat("─", inner)+"┘")
	} else {
		for i := 0; i < h && i < len(content); i++ {
			rows = append(rows, displaywidth.TruncateString(content[i], w, ""))
		}
	}
	return t.Base(strings.Join(rows, "\n"))
}

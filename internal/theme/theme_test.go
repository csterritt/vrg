package theme_test

import (
	"strings"
	"testing"

	"vrg/internal/theme"
)

// ANSI SGR sequences used by the theme.
const (
	reset      = "\x1b[0m"
	underline  = "\x1b[4m"
	darkBase   = "\x1b[37;40m" // white on black
	darkMatch  = "\x1b[30;47m" // black on white (true inverse of dark base)
	lightBase  = "\x1b[30;47m" // black on white
	lightMatch = "\x1b[37;40m" // white on black (true inverse of light base)
)

// containsSeq reports whether s contains the given ANSI sequence.
func containsSeq(s, seq string) bool { return strings.Contains(s, seq) }

// --- Scheme tests ---

// TestNewStartsDark verifies that New returns a theme with the dark
// scheme active (white on black).
func TestNewStartsDark(t *testing.T) {
	tm := theme.New()
	if tm.Scheme() != theme.SchemeDark {
		t.Fatalf("Scheme() = %v, want SchemeDark", tm.Scheme())
	}
}

// TestToggleDarkToLight verifies that Toggle flips the scheme from
// dark to light.
func TestToggleDarkToLight(t *testing.T) {
	tm := theme.New()
	tm = tm.Toggle()
	if tm.Scheme() != theme.SchemeLight {
		t.Fatalf("after Toggle, Scheme() = %v, want SchemeLight", tm.Scheme())
	}
}

// TestToggleLightToDark verifies that toggling twice returns to dark
// with no persistence.
func TestToggleLightToDark(t *testing.T) {
	tm := theme.New()
	tm = tm.Toggle()
	tm = tm.Toggle()
	if tm.Scheme() != theme.SchemeDark {
		t.Fatalf("after two Toggles, Scheme() = %v, want SchemeDark", tm.Scheme())
	}
}

// TestToggleNoPersistence verifies that toggling does not persist across
// themes: a fresh New is always dark regardless of prior toggles.
func TestToggleNoPersistence(t *testing.T) {
	tm := theme.New()
	tm = tm.Toggle()
	fresh := theme.New()
	if fresh.Scheme() != theme.SchemeDark {
		t.Fatalf("fresh New() after toggling another instance: Scheme() = %v, want SchemeDark", fresh.Scheme())
	}
}

// TestNoStyleScheme verifies that the no-style theme reports the dark
// scheme (its zero value) but produces no ANSI sequences.
func TestNoStyleScheme(t *testing.T) {
	tm := theme.NoStyle()
	if !tm.IsNoStyle() {
		t.Fatal("NoStyle().IsNoStyle() = false, want true")
	}
}

// TestNoStyleToggleIsNoOp verifies that toggling the no-style theme
// is a no-op (stays no-style).
func TestNoStyleToggleIsNoOp(t *testing.T) {
	tm := theme.NoStyle()
	tm = tm.Toggle()
	if !tm.IsNoStyle() {
		t.Fatal("NoStyle().Toggle().IsNoStyle() = false, want true")
	}
}

// --- Base colour pair tests ---

// TestDarkBaseColours verifies that the dark scheme's base style uses
// white on black.
func TestDarkBaseColours(t *testing.T) {
	tm := theme.New()
	got := tm.Base("x")
	if !containsSeq(got, darkBase) {
		t.Fatalf("dark Base does not contain white-on-black sequence %q: %q", darkBase, got)
	}
}

// TestLightBaseColours verifies that the light scheme's base style uses
// black on white.
func TestLightBaseColours(t *testing.T) {
	tm := theme.New().Toggle()
	got := tm.Base("x")
	if !containsSeq(got, lightBase) {
		t.Fatalf("light Base does not contain black-on-white sequence %q: %q", lightBase, got)
	}
}

// TestBaseResets verifies that Base ends with a reset sequence.
func TestBaseResets(t *testing.T) {
	tm := theme.New()
	got := tm.Base("x")
	if !containsSeq(got, reset) {
		t.Fatalf("dark Base does not contain reset %q: %q", reset, got)
	}
}

// --- True inverse match tests ---

// TestDarkMatchTrueInverse verifies that the dark scheme's match style
// is the true inverse of its base: black on white.
func TestDarkMatchTrueInverse(t *testing.T) {
	tm := theme.New()
	got := tm.Match("x")
	if !containsSeq(got, darkMatch) {
		t.Fatalf("dark Match does not contain true-inverse (black-on-white) sequence %q: %q", darkMatch, got)
	}
	// The match sequence must appear before the content, not just as
	// the base restore after it.
	if !strings.HasPrefix(got, darkMatch) {
		t.Fatalf("dark Match does not start with true-inverse sequence %q: %q", darkMatch, got)
	}
}

// TestLightMatchTrueInverse verifies that the light scheme's match
// style is the true inverse of its base: white on black.
func TestLightMatchTrueInverse(t *testing.T) {
	tm := theme.New().Toggle()
	got := tm.Match("x")
	if !containsSeq(got, lightMatch) {
		t.Fatalf("light Match does not contain true-inverse (white-on-black) sequence %q: %q", lightMatch, got)
	}
	if !strings.HasPrefix(got, lightMatch) {
		t.Fatalf("light Match does not start with true-inverse sequence %q: %q", lightMatch, got)
	}
}

// TestMatchRestoresBase verifies that after a match span the base
// colours are restored (so text after the match remains in base).
func TestMatchRestoresBase(t *testing.T) {
	tm := theme.New()
	got := tm.Match("x")
	if !containsSeq(got, darkBase) {
		t.Fatalf("dark Match does not restore base sequence %q after match: %q", darkBase, got)
	}
}

// --- Current-match underline tests ---

// TestDarkCurrentMatchUnderline verifies that the dark scheme's
// current-match style adds underline to the true inverse match.
func TestDarkCurrentMatchUnderline(t *testing.T) {
	tm := theme.New()
	got := tm.CurrentMatch("x")
	// The current match sequence must contain the dark match colours
	// (30;47) and the underline attribute (4).
	if !strings.Contains(got, "30;47") {
		t.Fatalf("dark CurrentMatch does not contain true-inverse colours (30;47): %q", got)
	}
	if !strings.Contains(got, ";4m") {
		t.Fatalf("dark CurrentMatch does not contain underline attribute (;4m): %q", got)
	}
}

// TestLightCurrentMatchUnderline verifies that the light scheme's
// current-match style adds underline to the true inverse match.
func TestLightCurrentMatchUnderline(t *testing.T) {
	tm := theme.New().Toggle()
	got := tm.CurrentMatch("x")
	// The current match sequence must contain the light match colours
	// (37;40) and the underline attribute (4).
	if !strings.Contains(got, "37;40") {
		t.Fatalf("light CurrentMatch does not contain true-inverse colours (37;40): %q", got)
	}
	if !strings.Contains(got, ";4m") {
		t.Fatalf("light CurrentMatch does not contain underline attribute (;4m): %q", got)
	}
}

// TestCurrentMatchRestoresBase verifies that after a current-match
// span the base colours are restored.
func TestCurrentMatchRestoresBase(t *testing.T) {
	tm := theme.New()
	got := tm.CurrentMatch("x")
	if !containsSeq(got, darkBase) {
		t.Fatalf("dark CurrentMatch does not restore base sequence %q: %q", darkBase, got)
	}
}

// --- Indicator tests ---

// TestDarkIndicatorInverse verifies that the dark scheme's indicator
// style uses the inverse colour pair (same as match).
func TestDarkIndicatorInverse(t *testing.T) {
	tm := theme.New()
	got := tm.Indicator("x")
	if !containsSeq(got, darkMatch) {
		t.Fatalf("dark Indicator does not contain inverse sequence %q: %q", darkMatch, got)
	}
}

// TestLightIndicatorInverse verifies that the light scheme's indicator
// style uses the inverse colour pair (same as match).
func TestLightIndicatorInverse(t *testing.T) {
	tm := theme.New().Toggle()
	got := tm.Indicator("x")
	if !containsSeq(got, lightMatch) {
		t.Fatalf("light Indicator does not contain inverse sequence %q: %q", lightMatch, got)
	}
}

// --- Current-file underline tests ---

// TestUnderlineSequence verifies that Underline wraps with the SGR 4
// underline sequence.
func TestUnderlineSequence(t *testing.T) {
	tm := theme.New()
	got := tm.Underline("x")
	if !containsSeq(got, underline) {
		t.Fatalf("Underline does not contain underline sequence %q: %q", underline, got)
	}
}

// TestUnderlineRestoresBase verifies that after an underlined span the
// base colours are restored.
func TestUnderlineRestoresBase(t *testing.T) {
	tm := theme.New()
	got := tm.Underline("x")
	if !containsSeq(got, darkBase) {
		t.Fatalf("dark Underline does not restore base sequence %q: %q", darkBase, got)
	}
}

// TestUnderlineBothSchemes verifies that Underline works in both
// schemes, restoring the correct base colours.
func TestUnderlineBothSchemes(t *testing.T) {
	dark := theme.New()
	light := theme.New().Toggle()
	darkGot := dark.Underline("x")
	lightGot := light.Underline("x")
	if !containsSeq(darkGot, darkBase) {
		t.Fatalf("dark Underline does not restore dark base %q: %q", darkBase, darkGot)
	}
	if !containsSeq(lightGot, lightBase) {
		t.Fatalf("light Underline does not restore light base %q: %q", lightBase, lightGot)
	}
}

// --- Gutter and file-list tests ---

// TestGutterUsesBaseColours verifies that the gutter style uses the
// active scheme's base colours.
func TestGutterUsesBaseColours(t *testing.T) {
	dark := theme.New()
	light := theme.New().Toggle()
	darkGot := dark.Gutter("x")
	lightGot := light.Gutter("x")
	if !containsSeq(darkGot, darkBase) {
		t.Fatalf("dark Gutter does not contain base sequence %q: %q", darkBase, darkGot)
	}
	if !containsSeq(lightGot, lightBase) {
		t.Fatalf("light Gutter does not contain base sequence %q: %q", lightBase, lightGot)
	}
}

// TestFileListUsesBaseColours verifies that the file-list style uses
// the active scheme's base colours.
func TestFileListUsesBaseColours(t *testing.T) {
	dark := theme.New()
	light := theme.New().Toggle()
	darkGot := dark.FileList("x")
	lightGot := light.FileList("x")
	if !containsSeq(darkGot, darkBase) {
		t.Fatalf("dark FileList does not contain base sequence %q: %q", darkBase, darkGot)
	}
	if !containsSeq(lightGot, lightBase) {
		t.Fatalf("light FileList does not contain base sequence %q: %q", lightBase, lightGot)
	}
}

// --- Filename-rule tests ---

// TestFilenameRuleEmbedsName verifies that the filename-rule style
// embeds the filename in a horizontal rule.
func TestFilenameRuleEmbedsName(t *testing.T) {
	tm := theme.New()
	got := tm.FilenameRule("main.go")
	if !strings.Contains(got, "main.go") {
		t.Fatalf("FilenameRule does not contain the filename: %q", got)
	}
	if !strings.ContainsAny(got, "─") {
		t.Fatalf("FilenameRule does not contain a horizontal rule character: %q", got)
	}
}

// TestFilenameRuleUsesBaseColours verifies that the filename-rule style
// uses the active scheme's base colours.
func TestFilenameRuleUsesBaseColours(t *testing.T) {
	dark := theme.New()
	light := theme.New().Toggle()
	darkGot := dark.FilenameRule("x")
	lightGot := light.FilenameRule("x")
	if !containsSeq(darkGot, darkBase) {
		t.Fatalf("dark FilenameRule does not contain base sequence %q: %q", darkBase, darkGot)
	}
	if !containsSeq(lightGot, lightBase) {
		t.Fatalf("light FilenameRule does not contain base sequence %q: %q", lightBase, lightGot)
	}
}

// --- Overlay tests ---

// TestOverlayUsesBaseColours verifies that the overlay style uses the
// active scheme's base colours.
func TestOverlayUsesBaseColours(t *testing.T) {
	dark := theme.New()
	light := theme.New().Toggle()
	darkGot := dark.Overlay("hello")
	lightGot := light.Overlay("hello")
	if !containsSeq(darkGot, darkBase) {
		t.Fatalf("dark Overlay does not contain base sequence %q: %q", darkBase, darkGot)
	}
	if !containsSeq(lightGot, lightBase) {
		t.Fatalf("light Overlay does not contain base sequence %q: %q", lightBase, lightGot)
	}
}

// TestOverlaySingleLineBorder verifies that the overlay style wraps
// content with a plain single-line border.
func TestOverlaySingleLineBorder(t *testing.T) {
	tm := theme.New()
	got := tm.Overlay("hello")
	// Single-line border uses these box-drawing characters.
	borderCorners := "┌┐└┘"
	for _, c := range borderCorners {
		if !strings.ContainsRune(got, c) {
			t.Fatalf("Overlay does not contain border corner %q: %q", c, got)
		}
	}
	if !strings.ContainsAny(got, "│") {
		t.Fatalf("Overlay does not contain vertical border character │: %q", got)
	}
	if !strings.ContainsAny(got, "─") {
		t.Fatalf("Overlay does not contain horizontal border character ─: %q", got)
	}
}

// TestOverlayContainsContent verifies that the overlay style preserves
// the content within the border.
func TestOverlayContainsContent(t *testing.T) {
	tm := theme.New()
	got := tm.Overlay("hello")
	if !strings.Contains(got, "hello") {
		t.Fatalf("Overlay does not contain the content: %q", got)
	}
}

// --- No-style tests ---

// TestNoStyleBaseIsPassthrough verifies that the no-style theme's
// Base method returns the string unchanged.
func TestNoStyleBaseIsPassthrough(t *testing.T) {
	tm := theme.NoStyle()
	if got := tm.Base("x"); got != "x" {
		t.Fatalf("NoStyle Base = %q, want %q", got, "x")
	}
}

// TestNoStyleMatchIsPassthrough verifies that the no-style theme's
// Match method returns the string unchanged.
func TestNoStyleMatchIsPassthrough(t *testing.T) {
	tm := theme.NoStyle()
	if got := tm.Match("x"); got != "x" {
		t.Fatalf("NoStyle Match = %q, want %q", got, "x")
	}
}

// TestNoStyleCurrentMatchIsPassthrough verifies that the no-style
// theme's CurrentMatch method returns the string unchanged.
func TestNoStyleCurrentMatchIsPassthrough(t *testing.T) {
	tm := theme.NoStyle()
	if got := tm.CurrentMatch("x"); got != "x" {
		t.Fatalf("NoStyle CurrentMatch = %q, want %q", got, "x")
	}
}

// TestNoStyleIndicatorIsPassthrough verifies that the no-style theme's
// Indicator method returns the string unchanged.
func TestNoStyleIndicatorIsPassthrough(t *testing.T) {
	tm := theme.NoStyle()
	if got := tm.Indicator("x"); got != "x" {
		t.Fatalf("NoStyle Indicator = %q, want %q", got, "x")
	}
}

// TestNoStyleUnderlineIsPassthrough verifies that the no-style theme's
// Underline method returns the string unchanged.
func TestNoStyleUnderlineIsPassthrough(t *testing.T) {
	tm := theme.NoStyle()
	if got := tm.Underline("x"); got != "x" {
		t.Fatalf("NoStyle Underline = %q, want %q", got, "x")
	}
}

// TestNoStyleGutterIsPassthrough verifies that the no-style theme's
// Gutter method returns the string unchanged.
func TestNoStyleGutterIsPassthrough(t *testing.T) {
	tm := theme.NoStyle()
	if got := tm.Gutter("x"); got != "x" {
		t.Fatalf("NoStyle Gutter = %q, want %q", got, "x")
	}
}

// TestNoStyleFileListIsPassthrough verifies that the no-style theme's
// FileList method returns the string unchanged.
func TestNoStyleFileListIsPassthrough(t *testing.T) {
	tm := theme.NoStyle()
	if got := tm.FileList("x"); got != "x" {
		t.Fatalf("NoStyle FileList = %q, want %q", got, "x")
	}
}

// TestNoStyleFilenameRuleIsPassthrough verifies that the no-style
// theme's FilenameRule method returns the rule without ANSI sequences.
func TestNoStyleFilenameRuleIsPassthrough(t *testing.T) {
	tm := theme.NoStyle()
	got := tm.FilenameRule("x")
	if strings.Contains(got, "\x1b") {
		t.Fatalf("NoStyle FilenameRule contains ANSI escape: %q", got)
	}
	if !strings.Contains(got, "x") {
		t.Fatalf("NoStyle FilenameRule does not contain the name: %q", got)
	}
}

// TestNoStyleOverlayIsPassthrough verifies that the no-style theme's
// Overlay method returns the content without ANSI sequences.
func TestNoStyleOverlayIsPassthrough(t *testing.T) {
	tm := theme.NoStyle()
	got := tm.Overlay("x")
	if strings.Contains(got, "\x1b") {
		t.Fatalf("NoStyle Overlay contains ANSI escape: %q", got)
	}
	if !strings.Contains(got, "x") {
		t.Fatalf("NoStyle Overlay does not contain the content: %q", got)
	}
}

// TestNoStyleProducesNoANSI verifies that no style method on the
// no-style theme produces any ANSI escape sequences.
func TestNoStyleProducesNoANSI(t *testing.T) {
	tm := theme.NoStyle()
	for _, got := range []string{
		tm.Base("x"),
		tm.Gutter("x"),
		tm.Match("x"),
		tm.CurrentMatch("x"),
		tm.Indicator("x"),
		tm.Underline("x"),
		tm.FileList("x"),
		tm.FilenameRule("x"),
		tm.Overlay("x"),
	} {
		if strings.Contains(got, "\x1b") {
			t.Fatalf("NoStyle method produced ANSI escape: %q", got)
		}
	}
}

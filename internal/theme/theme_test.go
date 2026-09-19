package theme_test

import (
	"testing"

	"vrg/internal/theme"
)

// The schemes' base colour pairs: dark is white on black, light is
// black on white (PRD: "Dark scheme is initially white on black; light
// is black on white").
func TestSchemeColourPairs(t *testing.T) {
	cases := []struct {
		name  string
		theme theme.Theme
		want  string
	}{
		{"dark", theme.Dark(), "\x1b[37;40m"},
		{"light", theme.Light(), "\x1b[30;47m"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.theme.Base("x")
			want := tc.want + "x\x1b[0m"
			if got != want {
				t.Fatalf("%s Base = %q, want %q", tc.name, got, want)
			}
		})
	}
}

// Toggled flips dark↔light and back. It is a pure value transform —
// the receiver is unchanged — and carries no persistence: nothing is
// written anywhere, the new scheme lives only in the model's theme
// field. The no-style theme has no scheme to flip.
func TestToggleFlipsSchemes(t *testing.T) {
	dark := theme.Dark()
	if got := dark.Toggled(); got != theme.Light() {
		t.Fatalf("Dark().Toggled() = %+v, want light", got)
	}
	if got := theme.Light().Toggled(); got != theme.Dark() {
		t.Fatalf("Light().Toggled() = %+v, want dark", got)
	}
	if dark != theme.Dark() {
		t.Fatal("Toggled mutated its receiver")
	}
	if got := theme.Plain().Toggled(); got != theme.Plain() {
		t.Fatalf("Plain().Toggled() = %+v, want the unchanged no-style theme", got)
	}
}

// The match style is the true inverse of the active scheme's base
// colours — the scheme's background becomes the match's foreground and
// vice versa, so a dark-scheme match is black on white and a
// light-scheme match is white on black: each equal to the other
// scheme's base pair.
func TestMatchIsTrueInverse(t *testing.T) {
	cases := []struct {
		name       string
		theme      theme.Theme
		matchOn    string
		baseResume string
	}{
		{"dark", theme.Dark(), "\x1b[30;47m", "\x1b[37;40;24m"},
		{"light", theme.Light(), "\x1b[37;40m", "\x1b[30;47;24m"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.theme.Match("hit")
			want := tc.matchOn + "hit" + tc.baseResume
			if got != want {
				t.Fatalf("%s Match = %q, want %q", tc.name, got, want)
			}
		})
	}
}

// Matches on the current matched line keep the true-inverse colours and
// add underline — the only distinction between a current-line match and
// an ordinary one in either scheme.
func TestCurrentMatchUnderlines(t *testing.T) {
	cases := []struct {
		name       string
		theme      theme.Theme
		matchOn    string
		baseResume string
	}{
		{"dark", theme.Dark(), "\x1b[30;47;4m", "\x1b[37;40;24m"},
		{"light", theme.Light(), "\x1b[37;40;4m", "\x1b[30;47;24m"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.theme.CurrentMatch("hit")
			want := tc.matchOn + "hit" + tc.baseResume
			if got != want {
				t.Fatalf("%s CurrentMatch = %q, want %q", tc.name, got, want)
			}
		})
	}
}

// Indicators use the inverse style — the same colours as a match.
func TestIndicatorIsInverse(t *testing.T) {
	for _, th := range []theme.Theme{theme.Dark(), theme.Light()} {
		if got, want := th.Indicator("*"), th.Match("*"); got != want {
			t.Fatalf("Indicator = %q, want the match style %q", got, want)
		}
	}
}

// The current file-list entry is underlined in both schemes; an
// ordinary file-list entry carries only the base colours.
func TestCurrentFileUnderlined(t *testing.T) {
	cases := []struct {
		name       string
		theme      theme.Theme
		baseOn     string
		entryOn    string
		baseResume string
	}{
		{"dark", theme.Dark(), "\x1b[37;40m", "\x1b[37;40;4m", "\x1b[37;40;24m"},
		{"light", theme.Light(), "\x1b[30;47m", "\x1b[30;47;4m", "\x1b[30;47;24m"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.theme.CurrentFile("a.go")
			want := tc.entryOn + "a.go" + tc.baseResume
			if got != want {
				t.Fatalf("%s CurrentFile = %q, want %q", tc.name, got, want)
			}
			// FileList is the base-colour style without underline.
			got = tc.theme.FileList("a.go")
			want = tc.baseOn + "a.go" + tc.baseResume
			if got != want {
				t.Fatalf("%s FileList = %q, want %q", tc.name, got, want)
			}
		})
	}
}

// Gutter, filename-rule, and file-list styles all render in the
// scheme's base colours.
func TestBaseColourStyles(t *testing.T) {
	cases := []struct {
		name       string
		theme      theme.Theme
		baseOn     string
		baseResume string
	}{
		{"dark", theme.Dark(), "\x1b[37;40m", "\x1b[37;40;24m"},
		{"light", theme.Light(), "\x1b[30;47m", "\x1b[30;47;24m"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			checks := []struct{ name, in, got string }{
				{"Gutter", "1  ", tc.theme.Gutter("1  ")},
				{"FilenameRule", "─ a.go ──", tc.theme.FilenameRule("─ a.go ──")},
				{"FileList", "a.go", tc.theme.FileList("a.go")},
			}
			for _, c := range checks {
				if want := tc.baseOn + c.in + tc.baseResume; c.got != want {
					t.Fatalf("%s = %q, want %q", c.name, c.got, want)
				}
			}
		})
	}
}

// The overlay style frames its rows in a plain single-line border —
// ─, │, and the corners — all in the scheme's base colours, with short
// rows padded so the right border aligns.
func TestOverlayBorderBaseColours(t *testing.T) {
	got := theme.Dark().Overlay([]string{"hello", "hi"})
	want := []string{
		"\x1b[37;40m┌───────┐\x1b[37;40;24m",
		"\x1b[37;40m│ hello │\x1b[37;40;24m",
		"\x1b[37;40m│ hi    │\x1b[37;40;24m",
		"\x1b[37;40m└───────┘\x1b[37;40;24m",
	}
	if len(got) != len(want) {
		t.Fatalf("Overlay rows = %d, want %d: %q", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Overlay row %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// On the no-style composition path every decorator is the identity —
// no escape byte may legitimately appear — except Overlay, which still
// draws its border (structure, not styling).
func TestPlainNoStyle(t *testing.T) {
	p := theme.Plain()
	for name, got := range map[string]string{
		"Base":         p.Base("x"),
		"Gutter":       p.Gutter("x"),
		"Match":        p.Match("x"),
		"CurrentMatch": p.CurrentMatch("x"),
		"Indicator":    p.Indicator("x"),
		"FilenameRule": p.FilenameRule("x"),
		"FileList":     p.FileList("x"),
		"CurrentFile":  p.CurrentFile("x"),
	} {
		if got != "x" {
			t.Fatalf("Plain.%s = %q, want the input unchanged", name, got)
		}
	}
	got := p.Overlay([]string{"x"})
	want := []string{"┌───┐", "│ x │", "└───┘"}
	if len(got) != len(want) {
		t.Fatalf("Plain.Overlay rows = %d, want %d: %q", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Plain.Overlay row %d = %q, want %q", i, got[i], want[i])
		}
	}
}

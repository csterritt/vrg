package theme

import (
	"strings"
	"testing"
)

// The dark scheme — initially active — is white on black.
func TestDarkSchemeIsInitiallyActive(t *testing.T) {
	th := Styled()
	if got, want := th.Base("x"), "\x1b[37;40mx\x1b[0m"; got != want {
		t.Fatalf("Styled().Base = %q, want %q (white on black)", got, want)
	}
}

// c's toggle flips between the schemes and back, in memory only.
func TestToggleFlipsBetweenSchemes(t *testing.T) {
	dark, light := "\x1b[37;40m", "\x1b[30;47m"
	th := Styled()
	if got := th.Toggle().Base("x"); !strings.HasPrefix(got, light) {
		t.Fatalf("after one toggle Base = %q, want it to start with %q (black on white)", got, light)
	}
	if got := th.Toggle().Toggle().Base("x"); !strings.HasPrefix(got, dark) {
		t.Fatalf("after two toggles Base = %q, want it to start with %q (back to dark)", got, dark)
	}
}

// Every supplied style in both schemes: the segment opens with its
// colours and closes back at the scheme's base, so styled runs compose
// inside a base-painted row.
func TestSchemeColourPairs(t *testing.T) {
	for _, tc := range []struct {
		name      string
		th        Theme
		base, inv string
	}{
		{"dark", Styled(), "\x1b[37;40m", "\x1b[30;47m"},
		{"light", Styled().Toggle(), "\x1b[30;47m", "\x1b[37;40m"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// A scheme's match style is the other scheme's base: the
			// true inverse of the active foreground/background pair.
			if got, want := tc.th.Match("m"), tc.inv+"m"+tc.base; got != want {
				t.Fatalf("Match = %q, want %q", got, want)
			}
			if got, want := tc.th.Indicator("*"), tc.inv+"*"+tc.base; got != want {
				t.Fatalf("Indicator = %q, want %q (inverse)", got, want)
			}
			for name, style := range map[string]func(string) string{
				"Gutter":       tc.th.Gutter,
				"FilenameRule": tc.th.FilenameRule,
				"FileList":     tc.th.FileList,
			} {
				if got, want := style("s"), tc.base+"s"+tc.base; got != want {
					t.Fatalf("%s = %q, want %q (base colours)", name, got, want)
				}
			}
		})
	}
}

// Matches on the current matched line add underline to the inverse
// match style, in both schemes.
func TestCurrentMatchAddsUnderline(t *testing.T) {
	for _, tc := range []struct {
		name      string
		th        Theme
		base, inv string
	}{
		{"dark", Styled(), "\x1b[37;40m", "\x1b[30;47m"},
		{"light", Styled().Toggle(), "\x1b[30;47m", "\x1b[37;40m"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, want := tc.th.CurrentMatch("m"), "\x1b[4m"+tc.inv+"m\x1b[24m"+tc.base; got != want {
				t.Fatalf("CurrentMatch = %q, want %q (inverse plus underline)", got, want)
			}
		})
	}
}

// The current file-list entry is underlined in both schemes.
func TestCurrentFileUnderlined(t *testing.T) {
	for _, tc := range []struct {
		name string
		th   Theme
		base string
	}{
		{"dark", Styled(), "\x1b[37;40m"},
		{"light", Styled().Toggle(), "\x1b[30;47m"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, want := tc.th.CurrentFile("a.txt"), "\x1b[4ma.txt\x1b[24m"+tc.base; got != want {
				t.Fatalf("CurrentFile = %q, want %q", got, want)
			}
		})
	}
}

// The overlay style paints a plain single-line border in the scheme's
// base colours, content clipped and padded to the interior.
func TestOverlayBaseColoursSingleLineBorder(t *testing.T) {
	for _, tc := range []struct {
		name string
		th   Theme
		base string
	}{
		{"dark", Styled(), "\x1b[37;40m"},
		{"light", Styled().Toggle(), "\x1b[30;47m"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := tc.base + "┌────┐\n│hi  │\n└────┘" + "\x1b[0m"
			if got := tc.th.Overlay([]string{"hi"}, 6, 3); got != want {
				t.Fatalf("Overlay = %q, want %q", got, want)
			}
		})
	}
}

// The overlay's interior sizing is measured in terminal cells on
// grapheme-cluster boundaries: wide and combining content pads and
// clips so the borders stay aligned, and a cluster split by the
// interior edge drops whole rather than drawing a half cell.
func TestOverlayMeasuresCellsForWideAndCombiningContent(t *testing.T) {
	th := Plain()
	want := "┌────┐\n│世界│\n│éx  │\n└────┘"
	if got := th.Overlay([]string{"世界", "éx"}, 6, 4); got != want {
		t.Fatalf("Overlay = %q, want %q", got, want)
	}
	// The trailing x does not fit; the wide clusters paint whole.
	want = "┌────┐\n│世界│\n└────┘"
	if got := th.Overlay([]string{"世界x"}, 6, 3); got != want {
		t.Fatalf("Overlay = %q, want %q — x clipped, the wide clusters whole", got, want)
	}
}

// The overlay measures its content ANSI-aware: embedded escape
// sequences occupy no cells, borders still align, and a zero-width
// trailing style reset survives interior truncation instead of being
// cut away.
func TestOverlayIgnoresANSIWhenSizing(t *testing.T) {
	th := Plain()
	want := "┌────┐\n│\x1b[31mhi\x1b[0m  │\n└────┘"
	if got := th.Overlay([]string{"\x1b[31mhi\x1b[0m"}, 6, 3); got != want {
		t.Fatalf("Overlay = %q, want %q — ANSI sequences are zero cells", got, want)
	}
	want = "┌──┐\n│\x1b[31mhe\x1b[0m│\n└──┘"
	if got := th.Overlay([]string{"\x1b[31mhello\x1b[0m"}, 4, 3); got != want {
		t.Fatalf("Overlay = %q, want %q — the reset survives truncation", got, want)
	}
}

// The no-style composition path emits no ANSI styling at all; the
// overlay keeps its structural border without colour codes.
func TestPlainEmitsNoANSI(t *testing.T) {
	th := Plain()
	for name, got := range map[string]string{
		"Base":         th.Base("x"),
		"Gutter":       th.Gutter("x"),
		"Match":        th.Match("x"),
		"CurrentMatch": th.CurrentMatch("x"),
		"Indicator":    th.Indicator("x"),
		"FilenameRule": th.FilenameRule("x"),
		"FileList":     th.FileList("x"),
		"CurrentFile":  th.CurrentFile("x"),
	} {
		if got != "x" {
			t.Fatalf("Plain().%s = %q, want %q", name, got, "x")
		}
	}
	if got := th.Overlay([]string{"hi"}, 6, 3); strings.Contains(got, "\x1b") {
		t.Fatalf("Plain().Overlay contains an escape: %q", got)
	}
	if want := "┌────┐\n│hi  │\n└────┘"; th.Overlay([]string{"hi"}, 6, 3) != want {
		t.Fatalf("Plain().Overlay = %q, want %q", th.Overlay([]string{"hi"}, 6, 3), want)
	}
}

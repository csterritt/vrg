package theme

import (
	"strings"
	"testing"
)

// The dark scheme is initially active: white on black. Plain exists for
// the no-style composition path and is not a scheme.
func TestDarkSchemeIsInitiallyActive(t *testing.T) {
	th := Dark()
	if th.Light() {
		t.Fatal("Dark() reports the light scheme")
	}
	if got, want := th.Base("x"), "\x1b[37;40mx\x1b[0m"; got != want {
		t.Fatalf("dark Base = %q, want %q (white on black)", got, want)
	}
}

// Both schemes' base foreground/background pairs.
func TestSchemeColourPairs(t *testing.T) {
	for _, tc := range []struct {
		name string
		th   Theme
		want string
	}{
		{"dark", Dark(), "\x1b[37;40mx\x1b[0m"},
		{"light", Dark().Toggle(), "\x1b[30;47mx\x1b[0m"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.th.Base("x"); got != tc.want {
				t.Fatalf("%s Base = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

// The toggle flips the active scheme dark → light → dark. It is
// in-memory state only: nothing is persisted anywhere.
func TestToggleRoundTrip(t *testing.T) {
	th := Dark()
	th = th.Toggle()
	if !th.Light() {
		t.Fatal("after one Toggle the scheme is not light")
	}
	if got, want := th.Base("x"), "\x1b[30;47mx\x1b[0m"; got != want {
		t.Fatalf("light Base = %q, want %q (black on white)", got, want)
	}
	th = th.Toggle()
	if th.Light() {
		t.Fatal("after two Toggles the scheme is not dark again")
	}
	if got, want := th.Base("x"), "\x1b[37;40mx\x1b[0m"; got != want {
		t.Fatalf("restored dark Base = %q, want %q", got, want)
	}
}

// A match renders in the true inverse of the active scheme's base pair
// and restores the base pair afterwards. In dark that is black on
// white; in light, white on black.
func TestMatchIsTrueInverse(t *testing.T) {
	for _, tc := range []struct {
		name string
		th   Theme
		want string
	}{
		{"dark", Dark(), "\x1b[30;47mhit\x1b[37;40m"},
		{"light", Dark().Toggle(), "\x1b[37;40mhit\x1b[30;47m"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.th.Match("hit"); got != tc.want {
				t.Fatalf("%s Match = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

// A match on the current matched line adds underline to the inverse
// pair, and restores both underline and base colours afterwards.
func TestCurrentMatchUnderline(t *testing.T) {
	for _, tc := range []struct {
		name string
		th   Theme
		want string
	}{
		{"dark", Dark(), "\x1b[30;47;4mhit\x1b[24;37;40m"},
		{"light", Dark().Toggle(), "\x1b[37;40;4mhit\x1b[24;30;47m"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.th.CurrentMatch("hit"); got != tc.want {
				t.Fatalf("%s CurrentMatch = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

// The current file-list entry is underlined in both schemes; underline
// is an attribute, so the ambient base colours are untouched.
func TestCurrentFileUnderline(t *testing.T) {
	for _, tc := range []struct {
		name string
		th   Theme
		want string
	}{
		{"dark", Dark(), "\x1b[4mf.txt\x1b[24m"},
		{"light", Dark().Toggle(), "\x1b[4mf.txt\x1b[24m"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.th.CurrentFile("f.txt"); got != tc.want {
				t.Fatalf("%s CurrentFile = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

// Indicators render in the true inverse of the active scheme.
func TestIndicatorInverse(t *testing.T) {
	for _, tc := range []struct {
		name string
		th   Theme
		want string
	}{
		{"dark", Dark(), "\x1b[30;47m*\x1b[37;40m"},
		{"light", Dark().Toggle(), "\x1b[37;40m*\x1b[30;47m"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.th.Indicator("*"); got != tc.want {
				t.Fatalf("%s Indicator = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

// Gutter, file-list, and filename-rule text render in the active
// scheme's base colours.
func TestBaseColourStyles(t *testing.T) {
	for _, tc := range []struct {
		name string
		th   Theme
		pair string
	}{
		{"dark", Dark(), "37;40"},
		{"light", Dark().Toggle(), "30;47"},
	} {
		on, off := "\x1b["+tc.pair+"m", "\x1b["+tc.pair+"m"
		for _, st := range []struct {
			name string
			fn   func(string) string
		}{
			{"Gutter", tc.th.Gutter},
			{"FileList", tc.th.FileList},
			{"FilenameRule", tc.th.FilenameRule},
		} {
			if got, want := st.fn("s"), on+"s"+off; got != want {
				t.Fatalf("%s %s = %q, want %q", tc.name, st.name, got, want)
			}
		}
	}
}

// Overlays use the base colours and a plain single-line border around
// their content.
func TestOverlayStyle(t *testing.T) {
	th := Dark()
	got := th.Overlay([]string{"hello", "wider line"})
	want := "\x1b[37;40m" +
		"┌──────────┐\n" +
		"│hello     │\n" +
		"│wider line│\n" +
		"└──────────┘" +
		"\x1b[37;40m"
	if got != want {
		t.Fatalf("dark Overlay = %q, want %q", got, want)
	}
	light := Dark().Toggle().Overlay([]string{"x"})
	if !strings.HasPrefix(light, "\x1b[30;47m") {
		t.Fatalf("light Overlay lacks base colours: %q", light)
	}
	if !strings.Contains(light, "│x│") {
		t.Fatalf("light Overlay lacks the single-line border: %q", light)
	}
}

// Plain is the no-style composition path: every style is the identity
// and no escape byte may legitimately appear.
func TestPlainIsIdentity(t *testing.T) {
	th := Plain()
	for _, st := range []struct {
		name string
		fn   func(string) string
	}{
		{"Base", th.Base},
		{"Gutter", th.Gutter},
		{"FileList", th.FileList},
		{"FilenameRule", th.FilenameRule},
		{"Match", th.Match},
		{"CurrentMatch", th.CurrentMatch},
		{"Indicator", th.Indicator},
		{"CurrentFile", th.CurrentFile},
	} {
		if got := st.fn("s"); got != "s" {
			t.Fatalf("Plain %s = %q, want identity", st.name, got)
		}
	}
	if got := th.Overlay([]string{"x"}); strings.Contains(got, "\x1b") {
		t.Fatalf("Plain Overlay emits an escape byte: %q", got)
	}
}

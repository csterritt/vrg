package filebuffer_test

import (
	"path/filepath"
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
)

// A file opening with a UTF-16 or UTF-32 BOM has an encoding the panel
// does not present: rg searched its transcoded view, so neither the raw
// bytes nor the recorded offsets can be shown faithfully. The buffer
// reports the detected encoding name and carries no lines, highlights,
// or markers — the caller's placeholder and diagnostic stand in for
// content. Longer BOMs are checked before the shorter ones they
// subsume, so FF FE 00 00 classifies as UTF-32 LE rather than UTF-16
// LE.
func TestUnsupportedEncodingBOMs(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		want    string
	}{
		{"UTF-16 LE", "\xff\xfeh\x00i\x00t\x00\n\x00", "UTF-16 LE"},
		{"UTF-16 BE", "\xfe\xff\x00h\x00i\x00t\x00\n", "UTF-16 BE"},
		{"UTF-32 LE", "\xff\xfe\x00\x00h\x00\x00\x00i\x00\x00\x00", "UTF-32 LE"},
		{"UTF-32 BE", "\x00\x00\xfe\xff\x00\x00\x00h\x00\x00\x00i", "UTF-32 BE"},
		// The overlap ordering: FF FE 00 00 opens with the UTF-16 LE
		// prefix but is the UTF-32 LE signature — the longer BOM wins.
		{"UTF-32 LE subsumes the UTF-16 LE prefix", "\xff\xfe\x00\x00", "UTF-32 LE"},
		{"bare UTF-16 LE BOM", "\xff\xfe", "UTF-16 LE"},
		{"bare UTF-16 BE BOM", "\xfe\xff", "UTF-16 BE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeFile(t, filepath.Join(t.TempDir(), "f.bin"), tc.content)
			buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
				subStop(1, sub(0, 3, "hit")),
			})
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := buf.Unsupported(); got != tc.want {
				t.Fatalf("Unsupported() = %q, want %q", got, tc.want)
			}
			if got := buf.LineCount(); got != 0 {
				t.Fatalf("LineCount = %d, want 0 — no file text", got)
			}
			if got := buf.Highlights(0); len(got) != 0 {
				t.Fatalf("Highlights(0) = %+v, want none", got)
			}
			if got := buf.Markers(0); len(got) != 0 {
				t.Fatalf("Markers(0) = %v, want none", got)
			}
			if line, cell := buf.TargetCell(subStop(1, sub(0, 3, "hit"))); line != 0 || cell != 0 {
				t.Fatalf("TargetCell = (%d, %d), want (0, 0)", line, cell)
			}
		})
	}
}

// Stale-match validation never runs against encoded bytes: recorded
// submatches that cannot match the raw UTF-16 content — validation
// would drop every one — leave the buffer unmarked stale. The
// placeholder is an encoding verdict, not a change since the search.
func TestUnsupportedEncodingSkipsStaleValidation(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.bin"),
		"\xff\xfeh\x00i\x00t\x00\n\x00")
	buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
		subStop(1, sub(0, 3, "hit")),
		subStop(9, sub(0, 4, "zzzz")),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if buf.Unsupported() == "" {
		t.Fatal("a UTF-16 BOM file is not unsupported")
	}
	if buf.Stale() {
		t.Fatal("stale-match validation ran against the encoded bytes")
	}
}

// Bytes that are not a UTF-16/32 signature stay ordinary files: a
// UTF-8 BOM is supported and invisible, and near-miss or lone signature
// bytes are ordinary content.
func TestNonBOMEncodingsAreNotMisclassified(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		lines   int
		first   string
	}{
		{"UTF-8 BOM is supported and invisible", "\xef\xbb\xbfhit\n", 1, "hit"},
		{"plain UTF-8", "hit\n", 1, "hit"},
		{"a lone FF byte is content", "\xffabc\n", 1, "\uFFFDabc"},
		{"a UTF-32 BE prefix without FF is content", "\x00\x00\xfe\x00hit\n", 1, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), tc.content)
			buf, err := filebuffer.Load([]byte(path), nil)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := buf.Unsupported(); got != "" {
				t.Fatalf("Unsupported() = %q, want supported", got)
			}
			if got := buf.LineCount(); got != tc.lines {
				t.Fatalf("LineCount = %d, want %d", got, tc.lines)
			}
			if tc.first != "" {
				if got := text(buf.Cells(0)); got != tc.first {
					t.Fatalf("line 0 = %q, want %q", got, tc.first)
				}
			}
		})
	}
}

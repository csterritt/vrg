package filebuffer_test

import (
	"testing"
)

// The UTF-16/32 fixture encodings — each file opens with its
// byte-order mark.
const (
	// encUTF16LE is the UTF-16 LE encoding of "hi\n", FF FE included.
	encUTF16LE = "\xff\xfeh\x00i\x00\n\x00"
	// encUTF16BE is the UTF-16 BE encoding of "hi\n".
	encUTF16BE = "\xfe\xff\x00h\x00i\x00\n"
	// encUTF32LE is the UTF-32 LE encoding of "hi": its four-byte
	// signature opens with UTF-16 LE's two-byte BOM — the overlap the
	// ordering must resolve.
	encUTF32LE = "\xff\xfe\x00\x00h\x00\x00\x00i\x00\x00\x00"
	// encUTF32BE is the UTF-32 BE encoding of "hi".
	encUTF32BE = "\x00\x00\xfe\xff\x00\x00\x00h\x00\x00\x00i"
)

// Each UTF-16/32 byte-order mark classifies the file as its named
// unsupported encoding (Issue #30): the buffer carries no lines, no
// highlights, and no stale state — the panel presents the placeholder
// and the app reports the diagnostic.
func TestUnsupportedBOMsClassified(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"UTF-16 LE", encUTF16LE, "UTF-16 LE"},
		{"UTF-16 BE", encUTF16BE, "UTF-16 BE"},
		{"UTF-32 LE", encUTF32LE, "UTF-32 LE"},
		{"UTF-32 BE", encUTF32BE, "UTF-32 BE"},
		// A two-byte file that is nothing but the BOM still classifies.
		{"UTF-16 LE signature only", "\xff\xfe", "UTF-16 LE"},
		// A third zero byte alone does not make UTF-32 LE.
		{"UTF-16 LE with a NUL third byte", "\xff\xfe\x00x", "UTF-16 LE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := load(t, writeFile(t, tc.content), stop(1, [2]int{0, 2}))
			if got := b.Unsupported(); got != tc.want {
				t.Fatalf("Unsupported() = %q, want %q", got, tc.want)
			}
			if b.LineCount() != 0 || len(b.Lines()) != 0 {
				t.Fatalf("lines = %d/%d, want no file text for an unsupported encoding",
					b.LineCount(), len(b.Lines()))
			}
			if b.Stale() {
				t.Fatal("an unsupported file marked stale — validation must not run on its bytes")
			}
		})
	}
}

// The overlap ordering: FF FE 00 00 opens with UTF-16 LE's two-byte
// signature, so the longer UTF-32 LE BOM is checked first and wins.
func TestUTF32LECheckedBeforeUTF16LE(t *testing.T) {
	b := load(t, writeFile(t, "\xff\xfe\x00\x00x\x00\x00\x00"))
	if got := b.Unsupported(); got != "UTF-32 LE" {
		t.Fatalf("Unsupported() = %q for FF FE 00 00, want %q — the longer BOM wins",
			got, "UTF-32 LE")
	}
}

// A leading UTF-8 BOM is a supported signature — Issue #22's
// coordinate adjustment owns it — and is never misclassified as
// unsupported; neither is ordinary content, a truncated signature, or
// signature bytes away from the file's start.
func TestSupportedSignaturesNotUnsupported(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"UTF-8 BOM", "\xef\xbb\xbfhit\n"},
		{"plain text", "hi\n"},
		{"empty file", ""},
		{"lone FF", "\xff"},
		{"lone FE", "\xfe"},
		{"truncated UTF-32 BE", "\x00\x00\xfe"},
		{"FF FE away from the start", "a\xff\xfe\n"},
		{"UTF-8 BOM on a later line", "a\n\xef\xbb\xbfz\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := load(t, writeFile(t, tc.content))
			if got := b.Unsupported(); got != "" {
				t.Fatalf("Unsupported() = %q, want supported", got)
			}
		})
	}
	// The UTF-8 BOM file decodes normally: the mark stays invisible.
	if l := load(t, writeFile(t, "\xef\xbb\xbfhit\n")).Lines()[0]; l.Text != "hit" {
		t.Fatalf("UTF-8 BOM line text = %q, want %q", l.Text, "hit")
	}
}

// Stale validation never runs on the raw encoded bytes (Issue #30):
// the recorded submatches are in rg's transcoded line coordinates,
// which cannot byte-equal a raw range, so the Issue #29 check would
// drop every one. The unsupported buffer skips it entirely.
func TestUnsupportedSkipsStaleValidation(t *testing.T) {
	// rg reports the transcoded line "hi\n" with submatch "hi" —
	// bytes that appear nowhere in the raw UTF-16 file.
	st := stopSubs(1, sub(0, 2, "hi"))
	b := load(t, writeFile(t, encUTF16LE), st)
	if b.Stale() {
		t.Fatal("stale validation ran against unsupported-encoding bytes")
	}
	if got := b.Lines(); len(got) != 0 {
		t.Fatalf("lines = %d, want none — no file text, no highlights", len(got))
	}
}

package app

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestMain doubles the test binary as the fake rg child: when
// VRG_FAKE_RG is set the re-executed binary runs fakeRgMain before the
// testing package can parse the child argv as test flags.
func TestMain(m *testing.M) {
	if mode := os.Getenv("VRG_FAKE_RG"); mode != "" {
		os.Exit(fakeRgMain(mode))
	}
	os.Exit(m.Run())
}

// echoStream is a complete, well-formed rg JSON stream: one file, one
// matched line, and a summary.
const echoStream = `{"type":"begin","data":{"path":{"text":"f.txt"}}}` + "\n" +
	`{"type":"match","data":{"path":{"text":"f.txt"},"lines":{"text":"hello\n"},"line_number":1,"submatches":[{"match":{"text":"hello"},"start":0,"end":5}]}}` + "\n" +
	`{"type":"end","data":{"path":{"text":"f.txt"},"binary_offset":null}}` + "\n" +
	`{"type":"summary","data":{}}`

// floodRecords is the number of match records the flood fixture emits,
// interleaved with the stderr flood.
const floodRecords = 20

// fakeRgMain is the fake rg. It observes the environment keys
// VRG_FAKE_RG (mode) and VRG_FAKE_DIR (artifact directory).
func fakeRgMain(mode string) int {
	dir := os.Getenv("VRG_FAKE_DIR")
	switch mode {
	case "echo":
		if dir != "" {
			os.WriteFile(filepath.Join(dir, "argv"),
				[]byte(strings.Join(os.Args[1:], "\n")), 0o644)
			if wd, err := os.Getwd(); err == nil {
				os.WriteFile(filepath.Join(dir, "cwd"), []byte(wd), 0o644)
			}
		}
		fmt.Fprint(os.Stdout, echoStream+"\n")
		return 0
	case "flood":
		// Interleave a large stderr stream with valid stdout records:
		// well over pipe capacity on stderr forces the parent to drain
		// both pipes concurrently or deadlock.
		chunk := bytes.Repeat([]byte("e"), 1<<16)
		for i := 0; i < floodRecords; i++ {
			if _, err := os.Stderr.Write(chunk); err != nil {
				return 3
			}
			fmt.Fprintf(os.Stdout,
				`{"type":"match","data":{"path":{"text":"f%d.txt"},"lines":{"text":"hit\n"},"line_number":%d,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`+"\n",
				i%2, i+1)
		}
		fmt.Fprintln(os.Stdout, `{"type":"summary","data":{}}`)
		// Handshake: both pipes are fully written before exit, so the
		// marker file proves the child finished writing rather than
		// being cut off.
		if dir != "" {
			os.WriteFile(filepath.Join(dir, "writes-done"), []byte("done"), 0o644)
		}
		return 0
	}
	return 2
}

// fakeRg returns the test binary path for use as Config.Rg.
func fakeRg() string { return os.Args[0] }

// The child receives exactly the protected argv and runs in the
// invocation working directory.
func TestChildArgvAndWorkingDirectory(t *testing.T) {
	work := t.TempDir()
	out := t.TempDir()
	t.Setenv("VRG_FAKE_RG", "echo")
	t.Setenv("VRG_FAKE_DIR", out)

	argv := []string{"--json", "--no-config", "-i", "-w", "--", "pat tern", "sub dir"}
	m, err := Start(context.Background(), Config{
		Rg:      fakeRg(),
		Argv:    argv,
		Workdir: work,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	awaitMsg(t, m, 10*time.Second)

	gotArgv, err := os.ReadFile(filepath.Join(out, "argv"))
	if err != nil {
		t.Fatalf("fake rg did not capture argv: %v", err)
	}
	if got := strings.Split(string(gotArgv), "\n"); !slices.Equal(got, argv) {
		t.Fatalf("child argv = %q, want %q", got, argv)
	}
	gotCwd, err := os.ReadFile(filepath.Join(out, "cwd"))
	if err != nil {
		t.Fatalf("fake rg did not capture cwd: %v", err)
	}
	wantCwd, _ := filepath.EvalSymlinks(work)
	gotCwdEval, _ := filepath.EvalSymlinks(string(gotCwd))
	if gotCwdEval != wantCwd {
		t.Fatalf("child working directory = %q, want %q", gotCwd, wantCwd)
	}
}

// A fake rg writing well over pipe capacity to stderr interleaved with a
// valid stdout stream neither deadlocks vrg nor loses stdout records:
// collection completes, every emitted match lands in the index, the
// stderr bytes are all captured, and the child's post-write handshake
// proves it finished writing both pipes before exiting.
func TestDualPipeBackpressure(t *testing.T) {
	work := t.TempDir()
	out := t.TempDir()
	t.Setenv("VRG_FAKE_RG", "flood")
	t.Setenv("VRG_FAKE_DIR", out)

	m, err := Start(context.Background(), Config{
		Rg:      fakeRg(),
		Argv:    []string{"--json", "--no-config", "--", "x", "."},
		Workdir: work,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	msg := awaitMsg(t, m, 30*time.Second)
	res, ok := msg.(searchDoneMsg)
	if !ok {
		t.Fatalf("completion message = %T, want searchDoneMsg", msg)
	}
	if res.waitErr != nil {
		t.Fatalf("child wait error: %v", res.waitErr)
	}
	if len(res.stderr) < 1<<20 {
		t.Fatalf("captured stderr = %d bytes, want at least 1 MiB", len(res.stderr))
	}
	if res.index == nil || res.index.LineCount() != floodRecords || res.index.FileCount() != 2 {
		var lines, files int
		if res.index != nil {
			lines, files = res.index.LineCount(), res.index.FileCount()
		}
		t.Fatalf("index = %d stops over %d files, want %d stops over 2 files", lines, files, floodRecords)
	}
	if _, err := os.Stat(filepath.Join(out, "writes-done")); err != nil {
		t.Fatalf("child handshake missing: %v", err)
	}
}

// A child that cannot be started surfaces a sanitized start error from
// Start before any UI work.
func TestStartFailure(t *testing.T) {
	for _, rg := range []string{
		filepath.Join(t.TempDir(), "no-such-rg"), // explicit missing path
		"vrg-definitely-not-a-real-rg",           // PATH lookup failure
	} {
		_, err := Start(context.Background(), Config{
			Rg:      rg,
			Argv:    []string{"--json", "--no-config", "--", "x", "."},
			Workdir: t.TempDir(),
		})
		if err == nil {
			t.Fatalf("Start(Rg=%q) succeeded, want start failure", rg)
		}
		for i := 0; i < len(err.Error()); i++ {
			if c := err.Error()[i]; c < 0x20 || c == 0x7f {
				t.Fatalf("start diagnostic carries raw control byte 0x%02x: %q", c, err.Error())
			}
		}
	}
}

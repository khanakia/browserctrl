//go:build darwin || linux

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/khanakia/browserctrl/browser"
	"github.com/khanakia/browserctrl/browser/browsertest"
)

// fakeBinary writes a shell script that exits with code and echoes its args,
// standing in for browserctrl so the harness is tested without building it.
func fakeBinary(t *testing.T, code int) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "fake")
	script := "#!/bin/sh\necho \"$@\"\nexit " + strconv.Itoa(code) + "\n"
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRun(t *testing.T) {
	t.Parallel()
	t.Run("usage error", func(t *testing.T) {
		// Pins the too-few-args arm.
		t.Parallel()
		var so, se bytes.Buffer
		if code := run([]string{"docfixture"}, &so, &se); code != exitUsage || !strings.Contains(se.String(), "usage:") {
			t.Errorf("exit %d err %q", code, se.String())
		}
	})
	t.Run("builds fixture, holds locks, passes args and exit code through", func(t *testing.T) {
		// Pins the happy path: root created on first use, --root appended,
		// child's exit code returned verbatim.
		t.Parallel()
		root := filepath.Join(t.TempDir(), "root")
		var so, se bytes.Buffer
		code := run([]string{"docfixture", root, fakeBinary(t, 0), "list", "--json"}, &so, &se)
		if code != 0 {
			t.Fatalf("exit %d err %q", code, se.String())
		}
		if got := strings.TrimSpace(so.String()); got != "list --json --root "+root {
			t.Errorf("child args = %q", got)
		}
		for _, dir := range runningProfiles {
			if _, err := os.Stat(browsertest.LockPath(root, dir, string(browser.ExtClaudeCode))); err != nil {
				t.Errorf("fixture missing %s: %v", dir, err)
			}
		}
		// Second run reuses the root (no rebuild) and propagates exit 2.
		if code := run([]string{"docfixture", root, fakeBinary(t, 2), "find", "chrome"}, &so, &se); code != 2 {
			t.Errorf("exit %d, want child's 2", code)
		}
	})
	t.Run("build failure when root cannot be created", func(t *testing.T) {
		// Pins the browsertest.Build error arm: parent is a file.
		t.Parallel()
		parent := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(parent, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		var so, se bytes.Buffer
		if code := run([]string{"docfixture", filepath.Join(parent, "root"), fakeBinary(t, 0)}, &so, &se); code != exitFailure || !strings.Contains(se.String(), "build fixture") {
			t.Errorf("exit %d err %q", code, se.String())
		}
	})
	t.Run("lock failure when root exists without the fixture", func(t *testing.T) {
		// Pins holdLocks' open-error arm: an existing but empty root skips
		// Build, so the LOCK files are absent.
		t.Parallel()
		root := t.TempDir()
		var so, se bytes.Buffer
		if code := run([]string{"docfixture", root, fakeBinary(t, 0)}, &so, &se); code != exitFailure || !strings.Contains(se.String(), "open LOCK") {
			t.Errorf("exit %d err %q", code, se.String())
		}
	})
	t.Run("lock failure when another holder has it", func(t *testing.T) {
		// Pins holdLocks' flock-error arm and its partial-release path.
		t.Parallel()
		root := filepath.Join(t.TempDir(), "root")
		if err := browsertest.Build(root, fixtureProfiles...); err != nil {
			t.Fatal(err)
		}
		f, err := os.OpenFile(browsertest.LockPath(root, runningProfiles[1], string(browser.ExtClaudeCode)), os.O_RDWR, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = f.Close() }()
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			t.Fatal(err)
		}
		var so, se bytes.Buffer
		if code := run([]string{"docfixture", root, fakeBinary(t, 0)}, &so, &se); code != exitFailure || !strings.Contains(se.String(), "flock") {
			t.Errorf("exit %d err %q", code, se.String())
		}
	})
	t.Run("binary missing", func(t *testing.T) {
		// Pins the non-ExitError arm of cmd.Run (exec failure).
		t.Parallel()
		root := filepath.Join(t.TempDir(), "root")
		var so, se bytes.Buffer
		if code := run([]string{"docfixture", root, filepath.Join(t.TempDir(), "nope")}, &so, &se); code != exitFailure || !strings.Contains(se.String(), "run browserctrl") {
			t.Errorf("exit %d err %q", code, se.String())
		}
	})
}

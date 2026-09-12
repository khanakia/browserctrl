package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/khanakia/browserctrl/browser"
)

// errWriter fails on the first write; pins every "return err" arm that only
// fires when stdout is broken (closed pipe, full disk).
type errWriter struct{}

var errSink = errors.New("sink closed")

func (errWriter) Write([]byte) (int, error) { return 0, errSink }

// corruptRoot is a user-data root whose Local State is unparsable, which is
// the one condition that makes browser.Scan itself fail.
func corruptRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Local State"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// Pins the scan-error arm of both verbs (RunE returns the error → exit 1).
func TestRun_ScanErrorExits1(t *testing.T) {
	t.Parallel()
	root := corruptRoot(t)
	for _, args := range [][]string{{"list"}, {"find", "x"}} {
		var so, se bytes.Buffer
		code := run(append(args, "--"+flagRoot, root), &so, &se)
		if code != exitFailure || so.Len() != 0 || !strings.Contains(se.String(), "Local State") {
			t.Errorf("%v: exit %d out %q err %q", args, code, so.String(), se.String())
		}
	}
}

// Pins writeTable's error arms: header write failure, and the extra ERROR
// cell for an entry whose store could not be read.
func TestWriteTable(t *testing.T) {
	t.Parallel()
	t.Run("header write failure", func(t *testing.T) {
		t.Parallel()
		if err := writeTable(errWriter{}, []browser.Entry{{DeviceID: "x"}}); !errors.Is(err, errSink) {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("empty-list hint write failure", func(t *testing.T) {
		t.Parallel()
		if err := writeTable(errWriter{}, nil); !errors.Is(err, errSink) {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("error row gets an ERROR cell", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		err := writeTable(&buf, []browser.Entry{{ProfileDir: "Default", State: browser.RunStateIdle, Error: "boom"}})
		if err != nil || !strings.Contains(buf.String(), "ERROR: boom") {
			t.Errorf("err %v out %q", err, buf.String())
		}
	})
}

// Pins writeRow's write-error arm on its own.
func TestWriteRow_WriteError(t *testing.T) {
	t.Parallel()
	if err := writeRow(errWriter{}, "a", "b"); !errors.Is(err, errSink) {
		t.Errorf("err = %v", err)
	}
}

// Pins writeJSON's encoder error arm via a failing writer.
func TestWriteJSON_WriteError(t *testing.T) {
	t.Parallel()
	if err := writeJSON(errWriter{}, browser.Entry{}); err == nil {
		t.Error("want error from failing writer")
	}
}

// Pins resolveVersion's link-time override arm. Not parallel: it mutates the
// package-level version variable.
func TestResolveVersion_LinkerStamped(t *testing.T) {
	old := version
	t.Cleanup(func() { version = old })
	version = "v9.9.9"
	if got := resolveVersion(); got != "v9.9.9" {
		t.Errorf("got %q", got)
	}
	// Under `go test` the build info's Main.Version is empty or "(devel)",
	// so the default arm must return versionDev.
	version = versionDev
	if got := resolveVersion(); got != versionDev {
		t.Errorf("got %q, want %q under go test", got, versionDev)
	}
}

// Pins the --version flag end to end through cobra.
func TestRun_VersionFlag(t *testing.T) {
	t.Parallel()
	var so, se bytes.Buffer
	if code := run([]string{"--version"}, &so, &se); code != exitOK || !strings.Contains(so.String(), "browserctrl version") {
		t.Errorf("exit %d out %q err %q", code, so.String(), se.String())
	}
}

// Pins the find-with-json-on-stdout-failure arm: the id write error surfaces
// as exit 1 rather than being swallowed.
func TestRun_FindStdoutBroken(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// A minimal root: Default profile with a connected store.
	dir := filepath.Join(root, "Default", "Local Extension Settings", string(browser.ExtClaudeCode))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Local State"), []byte(`{"profile":{"info_cache":{"Default":{"name":"a","user_name":"a@x.io"}}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	writeStore(t, dir, map[string]any{"bridgeDeviceId": "id-a"})
	var se bytes.Buffer
	if code := run([]string{"find", "a", "--" + flagRoot, root}, errWriter{}, &se); code != exitFailure {
		t.Errorf("exit %d, err %q", code, se.String())
	}
	if code := run([]string{"list", "--" + flagRoot, root}, errWriter{}, &se); code != exitFailure {
		t.Errorf("list: exit %d, err %q", code, se.String())
	}
	if code := run([]string{"list", "--json", "--" + flagRoot, root}, errWriter{}, &se); code != exitFailure {
		t.Errorf("list --json: exit %d, err %q", code, se.String())
	}
	if code := run([]string{"find", "a", "--json", "--" + flagRoot, root}, errWriter{}, &se); code != exitFailure {
		t.Errorf("find --json: exit %d, err %q", code, se.String())
	}
}

package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ubgo/buildinfo"

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

// Pins releaseVersion: stamped and module versions pass through, an absent
// version and Go's untagged pseudo-version both map to dev so the skills
// command serves the live directory instead of fetching a bundle.
func TestReleaseVersion(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		info buildinfo.Info
		want string
	}{
		{"ldflags stamp", buildinfo.Info{Version: "v0.1.0", Source: buildinfo.SourceLdflags}, "v0.1.0"},
		{"go install module version", buildinfo.Info{Version: "v0.2.3", Source: buildinfo.SourceModule}, "v0.2.3"},
		{"pseudo-version from go build", buildinfo.Info{Version: "v0.0.0-20260912061725-90d9f421ae81+dirty", Source: buildinfo.SourceModule}, buildinfo.DevVersion},
		{"no version", buildinfo.Info{Version: buildinfo.DevVersion, Source: buildinfo.SourceUnknown}, buildinfo.DevVersion},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := releaseVersion(tc.info); got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}

// Pins versionString: commit and dirty rendering, and no double "dirty" when
// Go already encoded +dirty in the version.
func TestVersionString(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		info buildinfo.Info
		want string
	}{
		{"no commit", buildinfo.Info{Version: "dev", Commit: buildinfo.Unknown}, "dev"},
		{"commit clean", buildinfo.Info{Version: "v0.1.0", Commit: "90d9f421ae81abcdef"}, "v0.1.0 (90d9f42)"},
		{"commit dirty stamped", buildinfo.Info{Version: "v0.1.0", Commit: "90d9f42", Modified: true}, "v0.1.0 (90d9f42) dirty"},
		{"pseudo already +dirty", buildinfo.Info{Version: "v0.0.0-2026+dirty", Commit: "90d9f42", Modified: true}, "v0.0.0-2026+dirty (90d9f42)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := versionString(tc.info); got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}

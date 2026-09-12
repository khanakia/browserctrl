//go:build darwin || linux

package browsertest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Pins Build's Local State write-error arm: the profile directory already
// exists (so MkdirAll is a no-op) but the root itself is read-only, so the
// final WriteFile of `Local State` is what fails.
func TestBuild_LocalStateWriteError(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("running as root; read-only dir cannot be enforced")
	}
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "Default"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o755) })
	err := Build(root, ProfileSpec{Dir: "Default", Name: "x"})
	if err == nil || !strings.Contains(err.Error(), "write Local State") {
		t.Errorf("err = %v", err)
	}
}

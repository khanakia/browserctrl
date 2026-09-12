package browser

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// These tests mutate HOME / TMPDIR via t.Setenv, so none of them may call
// t.Parallel(); Go enforces that.

// Pins ReadProfiles' read-error arm (not "missing", a real read failure):
// a `Local State` that is a directory cannot be read as a file.
func TestReadProfiles_LocalStateIsDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, localStateFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadProfiles(root); err == nil {
		t.Error("want error when Local State is a directory")
	}
}

// Pins candidateRoots' home-dir error arm and DefaultRoots' propagation of
// it: with HOME unset, os.UserHomeDir fails on darwin and linux.
func TestDefaultRoots_NoHome(t *testing.T) {
	t.Setenv("HOME", "")
	if _, err := candidateRoots(currentGOOS()); err == nil {
		t.Error("candidateRoots: want error with HOME unset")
	}
	if _, err := DefaultRoots(); err == nil {
		t.Error("DefaultRoots: want error with HOME unset")
	}
}

// Pins DefaultRoots' existence filter: a HOME with no browsers installed
// yields an empty (not nil-error) root list, and Scan with nil Roots then
// returns no entries. This is the arm a fresh CI box exercises.
func TestScan_DefaultRootsEmptyHome(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")
	roots, err := DefaultRoots()
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 0 {
		t.Fatalf("expected no roots under an empty HOME, got %+v", roots)
	}
	got, err := Scan(context.Background(), Options{})
	if err != nil || got != nil {
		t.Errorf("got %v, %v", got, err)
	}
}

// Pins Scan's DefaultRoots error arm (nil Roots + unresolvable HOME).
func TestScan_DefaultRootsError(t *testing.T) {
	t.Setenv("HOME", "")
	if _, err := Scan(context.Background(), Options{}); err == nil {
		t.Error("want error propagated from DefaultRoots")
	}
}

// Pins Scan's ctx check between stores: a pre-cancelled context aborts
// before the first store is read.
func TestScan_CancelledContext(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, defaultProfileDir), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Scan(ctx, Options{Roots: []Root{{KindChrome, root}}})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

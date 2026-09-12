//go:build darwin || linux

package browser

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestProbeLock(t *testing.T) {
	t.Parallel()
	t.Run("missing file is unknown", func(t *testing.T) {
		t.Parallel()
		if got := probeLock(filepath.Join(t.TempDir(), levelDBLockFile)); got != RunStateUnknown {
			t.Errorf("got %s, want %s", got, RunStateUnknown)
		}
	})
	t.Run("free lock is idle", func(t *testing.T) {
		t.Parallel()
		p := filepath.Join(t.TempDir(), levelDBLockFile)
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if got := probeLock(p); got != RunStateIdle {
			t.Errorf("got %s, want %s", got, RunStateIdle)
		}
		// Probe must not leave the lock held (run-twice rule).
		if got := probeLock(p); got != RunStateIdle {
			t.Errorf("second probe got %s, want %s", got, RunStateIdle)
		}
	})
	t.Run("held lock is running", func(t *testing.T) {
		t.Parallel()
		p := filepath.Join(t.TempDir(), levelDBLockFile)
		holder, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = holder.Close() }()
		// Same flock the Chromium LevelDB env takes. A separate fd in the same
		// process still conflicts under flock (locks are per open-file, not
		// per process), so this faithfully simulates a foreign holder.
		if err := syscall.Flock(int(holder.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			t.Fatal(err)
		}
		if got := probeLock(p); got != RunStateRunning {
			t.Errorf("got %s, want %s", got, RunStateRunning)
		}
		if err := syscall.Flock(int(holder.Fd()), syscall.LOCK_UN); err != nil {
			t.Fatal(err)
		}
		// Inverse: releasing flips it back to idle.
		if got := probeLock(p); got != RunStateIdle {
			t.Errorf("after unlock got %s, want %s", got, RunStateIdle)
		}
	})
}

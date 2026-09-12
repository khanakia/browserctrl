//go:build darwin || linux

package browser

import (
	"errors"
	"os"
	"syscall"
)

// probeLock reports whether another process holds the LevelDB LOCK at path.
//
// Why flock and not fcntl: Chromium's LevelDB env (`env_chromium`) locks via
// `base::File::Lock`, which on POSIX is `flock(LOCK_EX)`. On macOS flock and
// fcntl locks interoperate, but on Linux they are independent — so an fcntl
// F_GETLK probe would always report "free" there. flock is the only probe
// that is correct on both.
//
// Shared, non-blocking: the browser holds LOCK_EX, so a LOCK_SH|LOCK_NB
// attempt fails with EWOULDBLOCK exactly when the browser is running, while
// two concurrent probes (parallel scans in one process — flock is per open
// file description, not per process) never block each other. An earlier
// LOCK_EX probe did, and mis-sorted entries under `go test -race`. On success
// we unlock immediately; the probe never keeps the file locked.
func probeLock(path string) RunState {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return RunStateUnknown
	}
	// Documented optional cleanup: the handle was never written to, so Close
	// has nothing to flush and its error cannot change the probe result.
	defer func() { _ = f.Close() }()
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_SH|syscall.LOCK_NB)
	switch {
	case err == nil:
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) // Close releases it anyway
		return RunStateIdle
	case errors.Is(err, syscall.EWOULDBLOCK):
		return RunStateRunning
	default:
		return RunStateUnknown
	}
}

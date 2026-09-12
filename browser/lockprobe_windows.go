//go:build windows

package browser

// probeLock is not implemented on Windows: Chromium locks the LevelDB LOCK
// file with LockFileEx, and probing that without acquiring it needs a Win32
// call this package has not verified. Returning RunStateUnknown (never Idle)
// keeps the "careless path is the correct path" rule — a caller filtering on
// Running simply sees nothing rather than a wrong answer.
func probeLock(_ string) RunState {
	return RunStateUnknown
}

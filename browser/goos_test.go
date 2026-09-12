package browser

import "runtime"

// currentGOOS is a seam so tests read the same value production code does.
func currentGOOS() string { return runtime.GOOS }

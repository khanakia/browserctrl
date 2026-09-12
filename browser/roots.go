package browser

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Root is one browser family's user-data directory (the folder that contains
// `Local State` and the profile sub-directories).
type Root struct {
	Kind Kind   `json:"kind"`
	Path string `json:"path"`
}

// DefaultRoots returns the well-known user-data roots for the current OS.
//
// Why it exists: the scanner is deliberately root-agnostic (any Chromium user
// data dir works), so this is the single place OS-specific paths live. Roots
// that do not exist on disk are filtered out here so callers only iterate
// installed browsers.
//
// Verification status: the macOS table was checked against a real machine;
// the Linux and Windows tables come from the browsers' documented defaults
// and are covered only by the cross-compile gate, not by a live run.
func DefaultRoots() ([]Root, error) {
	all, err := candidateRoots(runtime.GOOS)
	if err != nil {
		return nil, err
	}
	present := make([]Root, 0, len(all))
	for _, r := range all {
		if st, statErr := os.Stat(r.Path); statErr == nil && st.IsDir() {
			present = append(present, r)
		}
	}
	return present, nil
}

// candidateRoots builds the full per-OS path table without touching disk.
// Split from DefaultRoots so tests can assert the table shape for every OS.
func candidateRoots(goos string) ([]Root, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir: %w", err)
	}
	switch goos {
	case "darwin":
		as := filepath.Join(home, "Library", "Application Support")
		return []Root{
			{KindChrome, filepath.Join(as, "Google", "Chrome")},
			{KindChromeBeta, filepath.Join(as, "Google", "Chrome Beta")},
			{KindChromeCanary, filepath.Join(as, "Google", "Chrome Canary")},
			{KindChromeDev, filepath.Join(as, "Google", "Chrome Dev")},
			{KindChromium, filepath.Join(as, "Chromium")},
			{KindBrave, filepath.Join(as, "BraveSoftware", "Brave-Browser")},
			{KindEdge, filepath.Join(as, "Microsoft Edge")},
			{KindArc, filepath.Join(as, "Arc", "User Data")},
			{KindVivaldi, filepath.Join(as, "Vivaldi")},
			{KindOpera, filepath.Join(as, "com.operasoftware.Opera")},
			{KindOperaGX, filepath.Join(as, "com.operasoftware.OperaGX")},
		}, nil
	case "linux":
		cfg := os.Getenv("XDG_CONFIG_HOME")
		if cfg == "" {
			cfg = filepath.Join(home, ".config")
		}
		return []Root{
			{KindChrome, filepath.Join(cfg, "google-chrome")},
			{KindChromeBeta, filepath.Join(cfg, "google-chrome-beta")},
			{KindChromeDev, filepath.Join(cfg, "google-chrome-unstable")},
			{KindChromium, filepath.Join(cfg, "chromium")},
			{KindBrave, filepath.Join(cfg, "BraveSoftware", "Brave-Browser")},
			{KindEdge, filepath.Join(cfg, "microsoft-edge")},
			{KindVivaldi, filepath.Join(cfg, "vivaldi")},
			{KindOpera, filepath.Join(cfg, "opera")},
		}, nil
	case "windows":
		local := os.Getenv("LOCALAPPDATA")
		roaming := os.Getenv("APPDATA")
		if local == "" || roaming == "" {
			return nil, fmt.Errorf("LOCALAPPDATA/APPDATA not set")
		}
		return []Root{
			{KindChrome, filepath.Join(local, "Google", "Chrome", "User Data")},
			{KindChromeBeta, filepath.Join(local, "Google", "Chrome Beta", "User Data")},
			{KindChromeCanary, filepath.Join(local, "Google", "Chrome SxS", "User Data")},
			{KindChromeDev, filepath.Join(local, "Google", "Chrome Dev", "User Data")},
			{KindChromium, filepath.Join(local, "Chromium", "User Data")},
			{KindBrave, filepath.Join(local, "BraveSoftware", "Brave-Browser", "User Data")},
			{KindEdge, filepath.Join(local, "Microsoft", "Edge", "User Data")},
			{KindVivaldi, filepath.Join(local, "Vivaldi", "User Data")},
			{KindOpera, filepath.Join(roaming, "Opera Software", "Opera Stable")},
			{KindOperaGX, filepath.Join(roaming, "Opera Software", "Opera GX Stable")},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported OS %q", goos)
	}
}

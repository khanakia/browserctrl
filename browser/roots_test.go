package browser

import (
	"path/filepath"
	"testing"
)

// The per-OS tables must be non-empty, absolute, and use only known Kinds.
// This pins the table shape for OSes we cannot run on in CI.
func TestCandidateRoots(t *testing.T) {
	t.Setenv("LOCALAPPDATA", `C:\Users\x\AppData\Local`)
	t.Setenv("APPDATA", `C:\Users\x\AppData\Roaming`)
	known := map[Kind]bool{}
	for _, k := range KindValues {
		known[k] = true
	}
	for _, goos := range []string{"darwin", "linux", "windows"} {
		roots, err := candidateRoots(goos)
		if err != nil {
			t.Fatalf("%s: %v", goos, err)
		}
		if len(roots) == 0 {
			t.Errorf("%s: empty table", goos)
		}
		seen := map[Kind]bool{}
		for _, r := range roots {
			if !known[r.Kind] {
				t.Errorf("%s: unknown kind %q", goos, r.Kind)
			}
			if seen[r.Kind] {
				t.Errorf("%s: duplicate kind %q", goos, r.Kind)
			}
			seen[r.Kind] = true
			if r.Kind == KindCustom {
				t.Errorf("%s: KindCustom must never appear in the default table", goos)
			}
			if goos != "windows" && !filepath.IsAbs(r.Path) {
				t.Errorf("%s: relative path %q", goos, r.Path)
			}
		}
		if !seen[KindChrome] {
			t.Errorf("%s: table lacks chrome", goos)
		}
	}
	if _, err := candidateRoots("plan9"); err == nil {
		t.Error("unsupported OS must error")
	}
}

func TestCandidateRoots_WindowsNeedsEnv(t *testing.T) {
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("APPDATA", "")
	if _, err := candidateRoots("windows"); err == nil {
		t.Error("want error when LOCALAPPDATA/APPDATA unset")
	}
}

// DefaultRoots only returns directories that exist; on any machine the result
// must be a subset of the candidate table.
func TestDefaultRoots_SubsetOfCandidates(t *testing.T) {
	t.Parallel()
	got, err := DefaultRoots()
	if err != nil {
		t.Fatal(err)
	}
	all, err := candidateRoots(currentGOOS())
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{}
	for _, r := range all {
		allowed[r.Path] = true
	}
	for _, r := range got {
		if !allowed[r.Path] {
			t.Errorf("DefaultRoots returned %q which is not in the candidate table", r.Path)
		}
	}
}

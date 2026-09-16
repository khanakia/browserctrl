//go:build darwin || linux

package browser

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/khanakia/browserctrl/browser/browsertest"
)

// A profile with no Claude extension still has to report running/idle
// correctly, otherwise `list --profiles` cannot answer "my profile is open
// but missing" — the question the flag exists for. The signal is the
// profile's own localStorage LevelDB LOCK, held here exactly as Chromium
// holds it.
func TestScan_IncludeAllProfiles_RunStateFromProfileLock(t *testing.T) {
	t.Parallel()
	root := browsertest.Root(t,
		browsertest.ProfileSpec{Dir: "Profile 26", Name: "no-ext", Email: "hi@x.io"},
	)
	opts := Options{Roots: []Root{{KindChrome, root}}, Extensions: []ExtensionID{ExtClaudeCode}, IncludeAllProfiles: true}

	got, err := Scan(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].State != RunStateIdle {
		t.Fatalf("closed profile: want one idle entry, got %+v", got)
	}

	holder, err := os.OpenFile(browsertest.ProfileLockPath(root, "Profile 26"), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Close() }()
	if err := syscall.Flock(int(holder.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	got, err = Scan(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].State != RunStateRunning {
		t.Fatalf("open profile: want one running entry, got %+v", got)
	}
	// Inverse: releasing the lock flips it back, so the probe is reading the
	// lock and not some cached or path-derived guess.
	if err := syscall.Flock(int(holder.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	if got, err = Scan(context.Background(), opts); err != nil || got[0].State != RunStateIdle {
		t.Fatalf("after unlock: got %+v, %v", got, err)
	}
}

// A profile directory Chromium has never opened has no localStorage LevelDB
// at all: the honest answer is unknown, never idle.
func TestScan_IncludeAllProfiles_NoProfileLevelDBIsUnknown(t *testing.T) {
	t.Parallel()
	root := browsertest.Root(t, browsertest.ProfileSpec{Dir: "Profile 1", Name: "bare", Email: "b@x.io"})
	if err := os.RemoveAll(filepath.Join(root, "Profile 1", localStorageDir)); err != nil {
		t.Fatal(err)
	}
	got, err := Scan(context.Background(), Options{Roots: []Root{{KindChrome, root}}, Extensions: []ExtensionID{ExtClaudeCode}, IncludeAllProfiles: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].State != RunStateUnknown {
		t.Fatalf("want one unknown entry, got %+v", got)
	}
}

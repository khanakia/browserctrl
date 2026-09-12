// Package browsertest builds fake Chromium user-data roots on disk so the
// scanner (and any program embedding it) can be tested without a browser.
//
// Why a separate exported package: both the `browser` package tests and the
// CLI's end-to-end tests need the same fixture, and a third-party importer of
// `browser` will too. Keeping it here avoids three copies of LevelDB set-up.
//
// Two entry points: Build writes a root anywhere and returns an error (usable
// from ordinary programs — e.g. generating documentation output); Root wraps
// Build for tests with a t.TempDir() and t.Fatal on failure.
package browsertest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/syndtr/goleveldb/leveldb"
)

// Chromium layout names, duplicated from the browser package on purpose: the
// fixture must produce what Chromium produces, independently of what the
// scanner expects, so a typo in either side fails a test instead of both
// silently agreeing.
const (
	localStateFile       = "Local State"
	extensionSettingsDir = "Local Extension Settings"
	lockFile             = "LOCK"
)

// Storage keys the Claude extension writes (see browser/constants.go).
const (
	KeyBridgeDeviceID    = "bridgeDeviceId"
	KeyBridgeDisplayName = "bridgeDisplayName"
	KeyMcpConnected      = "mcpConnected"
)

// Directory and file modes for the fixture. Chromium itself creates 0700
// profile dirs; the fixture is looser because tests read it from a different
// working directory and nothing in it is secret.
const (
	fixtureDirMode  = 0o755
	fixtureFileMode = 0o644
)

// ProfileSpec describes one profile to materialise under a root.
type ProfileSpec struct {
	// Dir is the profile directory name ("Default", "Profile 5").
	Dir string
	// Name / Email land in `Local State` info_cache. Empty values are written
	// as empty strings, mirroring a signed-out profile.
	Name  string
	Email string
	// Stores maps extension id → key/value pairs to write into that
	// extension's LevelDB. Values are JSON-encoded by the fixture, exactly as
	// Chromium does. A nil map means "extension not installed".
	Stores map[string]map[string]any
}

// Build materialises profiles under root (created if missing).
//
// `Local State` is written only when at least one profile has a Name or
// Email, so callers can also exercise the no-Local-State fallback (Opera
// never populates info_cache) by passing profiles with both empty.
func Build(root string, profiles ...ProfileSpec) error {
	infoCache := map[string]map[string]string{}
	writeLocalState := false
	for _, p := range profiles {
		if p.Name != "" || p.Email != "" {
			writeLocalState = true
		}
		infoCache[p.Dir] = map[string]string{"name": p.Name, "user_name": p.Email}
		if err := os.MkdirAll(filepath.Join(root, p.Dir), fixtureDirMode); err != nil {
			return fmt.Errorf("mkdir profile %s: %w", p.Dir, err)
		}
		for ext, kv := range p.Stores {
			if err := BuildStore(filepath.Join(root, p.Dir, extensionSettingsDir, ext), kv); err != nil {
				return err
			}
		}
	}
	if !writeLocalState {
		return nil
	}
	ls := map[string]any{"profile": map[string]any{"info_cache": infoCache}}
	raw, err := json.Marshal(ls)
	if err != nil {
		return fmt.Errorf("marshal Local State: %w", err)
	}
	if err := os.WriteFile(filepath.Join(root, localStateFile), raw, fixtureFileMode); err != nil {
		return fmt.Errorf("write Local State: %w", err)
	}
	return nil
}

// BuildStore creates a LevelDB at dir holding kv (values JSON-encoded) and
// closes it, leaving a LOCK file behind like a real closed Chromium store.
func BuildStore(dir string, kv map[string]any) (err error) {
	db, err := leveldb.OpenFile(dir, nil)
	if err != nil {
		return fmt.Errorf("create leveldb %s: %w", dir, err)
	}
	defer func() {
		if cErr := db.Close(); cErr != nil && err == nil {
			err = fmt.Errorf("close leveldb %s: %w", dir, cErr)
		}
	}()
	for k, v := range kv {
		raw, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("marshal %s: %w", k, err)
		}
		if err := db.Put([]byte(k), raw, nil); err != nil {
			return fmt.Errorf("put %s: %w", k, err)
		}
	}
	// goleveldb always creates LOCK on open, but the probe target must exist
	// even if that implementation detail changes — write it if absent.
	lock := filepath.Join(dir, lockFile)
	if _, statErr := os.Stat(lock); statErr != nil {
		if err := os.WriteFile(lock, nil, fixtureFileMode); err != nil {
			return fmt.Errorf("write LOCK: %w", err)
		}
	}
	return nil
}

// Root is Build for tests: a fresh t.TempDir() root, t.Fatal on failure.
// Takes testing.TB so it works from benchmarks and fuzz targets too.
func Root(t testing.TB, profiles ...ProfileSpec) string {
	t.Helper()
	root := t.TempDir()
	if err := Build(root, profiles...); err != nil {
		t.Fatal(err)
	}
	return root
}

// WriteStore is BuildStore for tests: t.Fatal on failure.
func WriteStore(t testing.TB, dir string, kv map[string]any) {
	t.Helper()
	if err := BuildStore(dir, kv); err != nil {
		t.Fatal(err)
	}
}

// LockPath returns the LOCK file path for an extension store under root.
func LockPath(root, profileDir, ext string) string {
	return filepath.Join(root, profileDir, extensionSettingsDir, ext, lockFile)
}

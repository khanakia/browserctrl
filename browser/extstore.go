package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

// ExtensionRecord is what the Claude extension has persisted for one profile.
type ExtensionRecord struct {
	// DeviceID is `bridgeDeviceId` — the exact string the Claude Code MCP
	// reports as `deviceId` in `list_connected_browsers`. Empty when the
	// extension is installed but has never connected.
	DeviceID string `json:"deviceId"`
	// DisplayName is `bridgeDisplayName` — the label the user typed in the
	// extension's "name this browser" prompt. Empty if never set.
	DisplayName string `json:"displayName"`
	// McpConnected is `mcpConnected` — true once the extension has completed
	// an MCP handshake at some point (sticky; see storageKeyMcpConnected).
	McpConnected bool `json:"mcpConnected"`
	// AccountUUID is `accountUuid` — the Claude account signed in to this
	// profile's extension. Empty when the extension has never been signed in.
	// The account is NOT derivable from the profile's Google/Microsoft email:
	// on a real machine a profile signed in to Chrome as one person was signed
	// in to Claude as another (2026-09-25).
	AccountUUID string `json:"accountUuid"`
	// OrgUUID is the `uuid` inside the `tokenOrg` object — the organization
	// the current token belongs to. Empty when signed out.
	OrgUUID string `json:"orgUuid"`
}

// tokenOrg is the shape of storageKeyTokenOrg. Only uuid is read; hybrid is
// declared so the decode fails loudly if the shape ever changes.
type tokenOrg struct {
	Hybrid bool   `json:"hybrid"`
	UUID   string `json:"uuid"`
}

// ErrNoExtensionStore is returned when the extension's LevelDB directory does
// not exist for that profile, i.e. the extension is not installed there.
var ErrNoExtensionStore = errors.New("browser: extension store not present")

// ReadExtensionStore reads DeviceID + DisplayName from an extension's
// chrome.storage.local LevelDB at dir.
//
// Why a copy: a running browser holds the LevelDB lock, and LevelDB (both
// Chromium's and goleveldb) refuses to open a locked DB. The directory is a
// handful of KB, so we snapshot it into a temp dir and open the snapshot
// read-only. The snapshot may be mid-compaction if the browser is writing at
// that instant — the caller gets an error for that one profile and can retry.
//
// Value decoding: Chromium stores chrome.storage values JSON-encoded, so a
// string arrives as `"…"` including quotes; we json-decode into string. A key
// that is absent yields an empty field, not an error (fresh install).
func ReadExtensionStore(ctx context.Context, dir string) (rec ExtensionRecord, err error) {
	if st, statErr := os.Stat(dir); statErr != nil || !st.IsDir() {
		return rec, ErrNoExtensionStore
	}
	tmp, err := os.MkdirTemp("", "browserctrl-ldb-*")
	if err != nil {
		return rec, fmt.Errorf("mkdtemp: %w", err)
	}
	defer func() {
		// Cleanup failure is non-fatal for the read but must not be silent.
		if rmErr := os.RemoveAll(tmp); rmErr != nil && err == nil {
			err = fmt.Errorf("remove snapshot %s: %w", tmp, rmErr)
		}
	}()
	if err := snapshotDir(ctx, dir, tmp); err != nil {
		return rec, fmt.Errorf("snapshot %s: %w", dir, err)
	}
	db, err := leveldb.OpenFile(tmp, &opt.Options{ReadOnly: true, ErrorIfMissing: true})
	if err != nil {
		return rec, fmt.Errorf("open leveldb snapshot: %w", err)
	}
	defer func() {
		if cErr := db.Close(); cErr != nil && err == nil {
			err = fmt.Errorf("close leveldb: %w", cErr)
		}
	}()
	if rec.DeviceID, err = getJSONString(db, storageKeyBridgeDeviceID); err != nil {
		return rec, err
	}
	if rec.DisplayName, err = getJSONString(db, storageKeyBridgeDisplayName); err != nil {
		return rec, err
	}
	if rec.McpConnected, err = getJSONBool(db, storageKeyMcpConnected); err != nil {
		return rec, err
	}
	if rec.AccountUUID, err = getJSONString(db, storageKeyAccountUUID); err != nil {
		return rec, err
	}
	org, err := getJSONValue[tokenOrg](db, storageKeyTokenOrg)
	if err != nil {
		return rec, err
	}
	rec.OrgUUID = org.UUID
	return rec, nil
}

// getJSONString fetches key and JSON-decodes it as a string. Missing key →
// "" with nil error (documented fallback). A value that is not a JSON string
// is an error: it means the extension changed its storage shape and the
// caller must not silently show garbage.
func getJSONString(db *leveldb.DB, key string) (string, error) {
	raw, err := db.Get([]byte(key), nil)
	if errors.Is(err, leveldb.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get %s: %w", key, err)
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", fmt.Errorf("decode %s (%q): %w", key, raw, err)
	}
	return s, nil
}

// getJSONValue is getJSONString for a structured value. Generic rather than
// an `any` out-parameter so the call site keeps its static type and no value
// is ever boxed. Missing key → the zero T with a nil error, which is the
// documented fallback for a profile that has never been signed in.
func getJSONValue[T any](db *leveldb.DB, key string) (T, error) {
	var out T
	raw, err := db.Get([]byte(key), nil)
	if errors.Is(err, leveldb.ErrNotFound) {
		return out, nil
	}
	if err != nil {
		return out, fmt.Errorf("get %s: %w", key, err)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("decode %s (%q): %w", key, raw, err)
	}
	return out, nil
}

// getJSONBool is getJSONString for a JSON boolean; missing key → false.
func getJSONBool(db *leveldb.DB, key string) (bool, error) {
	raw, err := db.Get([]byte(key), nil)
	if errors.Is(err, leveldb.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("get %s: %w", key, err)
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err != nil {
		return false, fmt.Errorf("decode %s (%q): %w", key, raw, err)
	}
	return b, nil
}

// snapshotDir copies the regular files of src (flat, LevelDB has no subdirs)
// into dst. The LOCK file is skipped: it is an empty flock target and copying
// it would be harmless but pointless. Honors ctx between files.
func snapshotDir(ctx context.Context, src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !e.Type().IsRegular() || e.Name() == levelDBLockFile {
			continue
		}
		if err := copyFile(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// copyFile is a plain byte copy; named returns exist only so the deferred
// Close error can be surfaced (a dropped flush error would hide a truncated
// snapshot and make the LevelDB open fail with a confusing message).
func copyFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if cErr := in.Close(); cErr != nil && err == nil {
			err = cErr
		}
	}()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if cErr := out.Close(); cErr != nil && err == nil {
			err = cErr
		}
	}()
	_, err = io.Copy(out, in)
	return err
}

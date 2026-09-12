//go:build darwin || linux

package browser

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"

	"github.com/khanakia/browserctrl/browser/browsertest"
)

// skipIfRoot: permission-based failures cannot be provoked as root.
func skipIfRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("running as root; chmod-based failure cannot be provoked")
	}
}

// Pins the mkdtemp error arm: TMPDIR pointing nowhere makes os.MkdirTemp fail
// before anything is copied. Not parallel: it mutates the environment.
func TestReadExtensionStore_TempDirUnavailable(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "store")
	browsertest.WriteStore(t, dir, map[string]any{browsertest.KeyBridgeDeviceID: "abc"})
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "does-not-exist"))
	_, err := ReadExtensionStore(context.Background(), dir)
	if err == nil || !strings.Contains(err.Error(), "mkdtemp") {
		t.Errorf("err = %v, want mkdtemp failure", err)
	}
}

// Pins the snapshot error arm via snapshotDir → os.ReadDir failing on an
// unreadable directory.
func TestReadExtensionStore_UnreadableDir(t *testing.T) {
	t.Parallel()
	skipIfRoot(t)
	dir := filepath.Join(t.TempDir(), "store")
	browsertest.WriteStore(t, dir, map[string]any{browsertest.KeyBridgeDeviceID: "abc"})
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	_, err := ReadExtensionStore(context.Background(), dir)
	if err == nil || !strings.Contains(err.Error(), "snapshot") {
		t.Errorf("err = %v, want snapshot failure", err)
	}
}

// Pins the copyFile open-error arm: one unreadable file inside an otherwise
// readable store makes the snapshot fail on that file.
func TestReadExtensionStore_UnreadableFile(t *testing.T) {
	t.Parallel()
	skipIfRoot(t)
	dir := filepath.Join(t.TempDir(), "store")
	browsertest.WriteStore(t, dir, map[string]any{browsertest.KeyBridgeDeviceID: "abc"})
	current := filepath.Join(dir, "CURRENT")
	if err := os.Chmod(current, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(current, 0o644) })
	_, err := ReadExtensionStore(context.Background(), dir)
	if err == nil || !strings.Contains(err.Error(), "snapshot") {
		t.Errorf("err = %v, want snapshot failure", err)
	}
}

// Pins the leveldb-open error arm: a store whose CURRENT names a manifest
// that does not exist cannot be opened, even read-only.
func TestReadExtensionStore_CorruptManifestPointer(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "store")
	browsertest.WriteStore(t, dir, map[string]any{browsertest.KeyBridgeDeviceID: "abc"})
	if err := os.WriteFile(filepath.Join(dir, "CURRENT"), []byte("MANIFEST-999999\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ReadExtensionStore(context.Background(), dir)
	if err == nil || !strings.Contains(err.Error(), "open leveldb snapshot") {
		t.Errorf("err = %v, want open failure", err)
	}
}

// Pins the db.Get error arm (an error other than not-found): a compacted
// table whose data block bytes are corrupted fails the block checksum on
// read, which goleveldb reports from Get rather than from Open.
func TestReadExtensionStore_CorruptTableBlock(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "store")
	db, err := leveldb.OpenFile(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Enough distinct keys that the table has a real data block, then force
	// a compaction so the data lives in an .ldb rather than the journal.
	for i := 0; i < 64; i++ {
		if err := db.Put([]byte(strings.Repeat("k", i+1)), []byte(`"v"`), nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Put([]byte(storageKeyBridgeDeviceID), []byte(`"abc"`), nil); err != nil {
		t.Fatal(err)
	}
	if err := db.CompactRange(util.Range{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	ldbs, err := filepath.Glob(filepath.Join(dir, "*.ldb"))
	if err != nil || len(ldbs) == 0 {
		t.Fatalf("no .ldb after compaction: %v", err)
	}
	raw, err := os.ReadFile(ldbs[0])
	if err != nil {
		t.Fatal(err)
	}
	// Flip bytes in the first quarter of the file: inside the data block,
	// well away from the footer/index the opener validates.
	for i := 8; i < len(raw)/4; i++ {
		raw[i] ^= 0xFF
	}
	if err := os.WriteFile(ldbs[0], raw, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = ReadExtensionStore(context.Background(), dir)
	if err == nil {
		t.Skip("goleveldb served the corrupted block without error; Get-error arm not provoked on this build")
	}
	if !strings.Contains(err.Error(), "get "+storageKeyBridgeDeviceID) && !strings.Contains(err.Error(), "open leveldb snapshot") {
		t.Errorf("unexpected error shape: %v", err)
	}
}

// Pins that a non-regular entry inside the store (a stray directory) is
// skipped rather than copied or treated as an error.
func TestSnapshotDir_SkipsSubdirectories(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "store")
	browsertest.WriteStore(t, dir, map[string]any{browsertest.KeyBridgeDeviceID: "abc"})
	if err := os.Mkdir(filepath.Join(dir, "lost"), 0o755); err != nil {
		t.Fatal(err)
	}
	rec, err := ReadExtensionStore(context.Background(), dir)
	if err != nil || rec.DeviceID != "abc" {
		t.Errorf("rec %+v err %v", rec, err)
	}
}

// Pins copyFile's create-error arm directly: destination directory missing.
func TestCopyFile_DestinationMissing(t *testing.T) {
	t.Parallel()
	src := filepath.Join(t.TempDir(), "src")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(src, filepath.Join(t.TempDir(), "no-such-dir", "dst")); err == nil {
		t.Error("want error when destination directory does not exist")
	}
}

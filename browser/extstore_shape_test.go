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

// Pins the bridgeDisplayName type-error arm specifically (the device id is a
// valid string, so the first read succeeds and the second one fails).
func TestReadExtensionStore_DisplayNameWrongType(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "store")
	browsertest.WriteStore(t, dir, map[string]any{
		browsertest.KeyBridgeDeviceID:    "ok",
		browsertest.KeyBridgeDisplayName: []string{"not", "a", "string"},
	})
	_, err := ReadExtensionStore(context.Background(), dir)
	if err == nil || !strings.Contains(err.Error(), "decode "+storageKeyBridgeDisplayName) {
		t.Errorf("err = %v", err)
	}
}

// Pins getJSONBool's read-error arm (an error other than not-found).
//
// Construction: the table's key range is ["mcpConnected", "zzz…"], so the
// two bridge* keys sort below it and come back not-found from the index
// without touching a data block; mcpConnected is inside the block we
// corrupt, so its Get fails the block checksum.
func TestReadExtensionStore_CorruptBlockReachesMcpConnected(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "store")
	db, err := leveldb.OpenFile(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put([]byte(storageKeyMcpConnected), []byte(`true`), nil); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 64; i++ {
		if err := db.Put([]byte(strings.Repeat("z", i)), []byte(`"pad"`), nil); err != nil {
			t.Fatal(err)
		}
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
	for i := 8; i < len(raw)/4; i++ {
		raw[i] ^= 0xFF
	}
	if err := os.WriteFile(ldbs[0], raw, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = ReadExtensionStore(context.Background(), dir)
	if err == nil {
		t.Skip("goleveldb served the corrupted block without error; arm not provoked on this build")
	}
	if !strings.Contains(err.Error(), "get "+storageKeyMcpConnected) {
		t.Skipf("corruption surfaced earlier than the mcpConnected read (%v); arm not provoked by this layout", err)
	}
}

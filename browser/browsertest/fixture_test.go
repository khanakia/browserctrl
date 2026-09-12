package browsertest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/syndtr/goleveldb/leveldb"
)

const testExt = "fcoeoabgfenejglbffodgkkbkcdhcgfn"

// Pins the happy path end to end: Local State content, store contents,
// JSON encoding of values, LOCK presence — i.e. what the scanner will read.
func TestBuild_WritesChromiumLayout(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	err := Build(root,
		ProfileSpec{Dir: "Default", Name: "Aman", Email: "a@x.io", Stores: map[string]map[string]any{
			testExt: {KeyBridgeDeviceID: "id-1", KeyMcpConnected: true},
		}},
		ProfileSpec{Dir: "Profile 4"},
	)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, localStateFile))
	if err != nil {
		t.Fatal(err)
	}
	var ls struct {
		Profile struct {
			InfoCache map[string]map[string]string `json:"info_cache"`
		} `json:"profile"`
	}
	if err := json.Unmarshal(raw, &ls); err != nil {
		t.Fatal(err)
	}
	if got := ls.Profile.InfoCache["Default"]["user_name"]; got != "a@x.io" {
		t.Errorf("user_name = %q", got)
	}
	if _, ok := ls.Profile.InfoCache["Profile 4"]; !ok {
		t.Error("Profile 4 missing from info_cache")
	}
	db, err := leveldb.OpenFile(filepath.Join(root, "Default", extensionSettingsDir, testExt), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	v, err := db.Get([]byte(KeyBridgeDeviceID), nil)
	if err != nil || string(v) != `"id-1"` {
		t.Errorf("stored value = %q, %v (want JSON-quoted string)", v, err)
	}
	if _, err := os.Stat(LockPath(root, "Default", testExt)); err != nil {
		t.Errorf("LOCK missing: %v", err)
	}
}

// Pins the documented fallback: no Name/Email anywhere → no Local State.
func TestBuild_NoLocalStateWhenAnonymous(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := Build(root, ProfileSpec{Dir: "Default"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, localStateFile)); !os.IsNotExist(err) {
		t.Errorf("Local State should not exist, stat err = %v", err)
	}
}

// Pins the mkdir error arm: a root that is a regular file cannot hold profiles.
func TestBuild_ErrorWhenRootIsFile(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(root, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	err := Build(root, ProfileSpec{Dir: "Default"})
	if err == nil || !strings.Contains(err.Error(), "mkdir profile") {
		t.Errorf("err = %v", err)
	}
}

// Pins that a BuildStore failure propagates out of Build unchanged.
func TestBuild_PropagatesStoreError(t *testing.T) {
	t.Parallel()
	err := Build(t.TempDir(), ProfileSpec{Dir: "Default", Stores: map[string]map[string]any{
		testExt: {"bad": make(chan int)},
	}})
	if err == nil || !strings.Contains(err.Error(), "marshal bad") {
		t.Errorf("err = %v", err)
	}
}

func TestBuildStore_Errors(t *testing.T) {
	t.Parallel()
	t.Run("cannot create leveldb under a file", func(t *testing.T) {
		// Pins the leveldb.OpenFile error arm.
		t.Parallel()
		f := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(f, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := BuildStore(filepath.Join(f, "store"), nil); err == nil || !strings.Contains(err.Error(), "create leveldb") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("unmarshalable value", func(t *testing.T) {
		// Pins the json.Marshal error arm (channels cannot be encoded).
		t.Parallel()
		err := BuildStore(filepath.Join(t.TempDir(), "s"), map[string]any{"k": make(chan int)})
		if err == nil || !strings.Contains(err.Error(), "marshal k") {
			t.Errorf("err = %v", err)
		}
	})
}

// fakeTB records Fatal instead of stopping the goroutine, so the t.Fatal
// arms of Root/WriteStore can be pinned without failing the real test.
type fakeTB struct {
	testing.TB // nil; only the methods below are ever called
	tmp        string
	fatal      []string
}

func (f *fakeTB) Helper()           {}
func (f *fakeTB) TempDir() string   { return f.tmp }
func (f *fakeTB) Fatal(args ...any) { f.fatal = append(f.fatal, fmt.Sprint(args...)) }

func TestRootAndWriteStore_FatalOnError(t *testing.T) {
	t.Parallel()
	// Root: TempDir returns a path that is a file → Build fails → Fatal.
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	ft := &fakeTB{tmp: file}
	if got := Root(ft, ProfileSpec{Dir: "Default"}); got != file || len(ft.fatal) != 1 {
		t.Errorf("Root returned %q, fatal calls %d", got, len(ft.fatal))
	}
	// WriteStore: unmarshalable value → Fatal.
	ft2 := &fakeTB{}
	WriteStore(ft2, filepath.Join(t.TempDir(), "s"), map[string]any{"k": make(chan int)})
	if len(ft2.fatal) != 1 {
		t.Errorf("WriteStore fatal calls = %d", len(ft2.fatal))
	}
}

func TestRootAndWriteStore_HappyPath(t *testing.T) {
	t.Parallel()
	root := Root(t, ProfileSpec{Dir: "Default", Name: "x"})
	if _, err := os.Stat(filepath.Join(root, localStateFile)); err != nil {
		t.Error(err)
	}
	dir := filepath.Join(t.TempDir(), "s")
	WriteStore(t, dir, map[string]any{KeyBridgeDeviceID: "z"})
	if _, err := os.Stat(filepath.Join(dir, lockFile)); err != nil {
		t.Error(err)
	}
}

func TestLockPath(t *testing.T) {
	t.Parallel()
	got := LockPath("/r", "Profile 5", testExt)
	want := filepath.Join("/r", "Profile 5", extensionSettingsDir, testExt, lockFile)
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

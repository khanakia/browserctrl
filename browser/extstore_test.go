package browser

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/khanakia/browserctrl/browser/browsertest"
)

func TestReadExtensionStore(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		kv      map[string]any // nil → directory absent
		want    ExtensionRecord
		wantErr error
		wantAny bool // any error acceptable (shape mismatch)
	}{
		{
			name: "all keys present",
			kv: map[string]any{
				browsertest.KeyBridgeDeviceID:    "ce9a8e06-61d3-4e7b-894b-13fd92987212",
				browsertest.KeyBridgeDisplayName: "chrome1",
				browsertest.KeyMcpConnected:      true,
				"unrelated":                      map[string]int{"x": 1},
			},
			want: ExtensionRecord{DeviceID: "ce9a8e06-61d3-4e7b-894b-13fd92987212", DisplayName: "chrome1", McpConnected: true},
		},
		{
			name: "fresh install has no keys",
			kv:   map[string]any{"selectedModel": "claude-sonnet-5"},
			want: ExtensionRecord{},
		},
		{name: "empty store", kv: map[string]any{}, want: ExtensionRecord{}},
		{name: "store dir absent", kv: nil, wantErr: ErrNoExtensionStore},
		{
			name:    "device id is not a string",
			kv:      map[string]any{browsertest.KeyBridgeDeviceID: 42},
			wantAny: true,
		},
		{
			name:    "mcpConnected is not a bool",
			kv:      map[string]any{browsertest.KeyMcpConnected: "yes"},
			wantAny: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Join(t.TempDir(), "store")
			if tc.kv != nil {
				browsertest.WriteStore(t, dir, tc.kv)
			}
			got, err := ReadExtensionStore(context.Background(), dir)
			switch {
			case tc.wantAny:
				if err == nil {
					t.Fatal("want error, got nil")
				}
				return
			case tc.wantErr != nil:
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			case err != nil:
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

// Reading twice must give the same answer and leave no snapshot behind
// (run-twice rule): the source dir is never mutated.
func TestReadExtensionStore_Idempotent(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "store")
	browsertest.WriteStore(t, dir, map[string]any{browsertest.KeyBridgeDeviceID: "abc"})
	first, err := ReadExtensionStore(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ReadExtensionStore(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || first.DeviceID != "abc" {
		t.Errorf("first %+v second %+v", first, second)
	}
}

func TestReadExtensionStore_CancelledContext(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "store")
	browsertest.WriteStore(t, dir, map[string]any{browsertest.KeyBridgeDeviceID: "abc"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ReadExtensionStore(ctx, dir); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

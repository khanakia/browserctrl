package browser

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/khanakia/browserctrl/browser/browsertest"
)

func TestScan(t *testing.T) {
	t.Parallel()
	ext := string(ExtClaudeCode)
	chrome := browsertest.Root(t,
		browsertest.ProfileSpec{Dir: "Default", Name: "Aman", Email: "me@x.io", Stores: map[string]map[string]any{
			ext: {browsertest.KeyBridgeDeviceID: "id-default", browsertest.KeyBridgeDisplayName: "chrome1", browsertest.KeyMcpConnected: true},
		}},
		browsertest.ProfileSpec{Dir: "Profile 5", Name: "work", Email: "w@x.io", Stores: map[string]map[string]any{
			ext: {browsertest.KeyBridgeDeviceID: "id-p5"},
			// Desktop-app extension in the same profile: only reported with --all.
			string(ExtClaudeDesktop): {browsertest.KeyBridgeDeviceID: "id-p5-desktop"},
		}},
		browsertest.ProfileSpec{Dir: "Profile 9", Name: "no-ext", Email: "n@x.io"},
	)
	// A root with no Local State at all (Opera-style) still yields Default.
	opera := browsertest.Root(t, browsertest.ProfileSpec{Dir: "Default", Stores: map[string]map[string]any{
		ext: {browsertest.KeyBridgeDeviceID: "id-opera"},
	}})
	roots := []Root{{KindOpera, opera}, {KindChrome, chrome}}

	t.Run("default extension set = Claude Code only, sorted by kind then dir", func(t *testing.T) {
		t.Parallel()
		got, err := Scan(context.Background(), Options{Roots: roots, Extensions: []ExtensionID{ExtClaudeCode}})
		if err != nil {
			t.Fatal(err)
		}
		want := []Entry{
			{DeviceID: "id-default", DisplayName: "chrome1", McpConnected: true, State: RunStateIdle, Browser: KindChrome, BrowserPath: chrome, ProfileDir: "Default", ProfileName: "Aman", Email: "me@x.io", Extension: ExtClaudeCode, Installed: true},
			{DeviceID: "id-p5", State: RunStateIdle, Browser: KindChrome, BrowserPath: chrome, ProfileDir: "Profile 5", ProfileName: "work", Email: "w@x.io", Extension: ExtClaudeCode, Installed: true},
			{DeviceID: "id-opera", State: RunStateIdle, Browser: KindOpera, BrowserPath: opera, ProfileDir: "Default", Extension: ExtClaudeCode, Installed: true},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got  %+v\nwant %+v", got, want)
		}
	})
	t.Run("nil Extensions scans every known id", func(t *testing.T) {
		t.Parallel()
		got, err := Scan(context.Background(), Options{Roots: roots})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 4 {
			t.Fatalf("want 4 entries, got %d: %+v", len(got), got)
		}
		if got[2].Extension != ExtClaudeDesktop || got[2].DeviceID != "id-p5-desktop" {
			t.Errorf("desktop ext entry misplaced: %+v", got[2])
		}
	})
	// The "why is my profile missing?" case: Profile 9 has no Claude
	// extension store, so it is absent by default and present, flagged
	// Installed=false with no device id, under IncludeAllProfiles.
	t.Run("IncludeAllProfiles adds profiles that have no extension store", func(t *testing.T) {
		t.Parallel()
		opts := Options{Roots: roots, Extensions: []ExtensionID{ExtClaudeCode}, IncludeAllProfiles: true}
		got, err := Scan(context.Background(), opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 4 {
			t.Fatalf("want 4 entries (3 installed + 1 profile without the extension), got %d: %+v", len(got), got)
		}
		var uninstalled []Entry
		for _, e := range got {
			if !e.Installed {
				uninstalled = append(uninstalled, e)
			}
		}
		if len(uninstalled) != 1 {
			t.Fatalf("want exactly 1 Installed=false entry, got %+v", uninstalled)
		}
		want := Entry{State: RunStateIdle, Browser: KindChrome, BrowserPath: chrome, ProfileDir: "Profile 9", ProfileName: "no-ext", Email: "n@x.io"}
		if !reflect.DeepEqual(uninstalled[0], want) {
			t.Errorf("got  %+v\nwant %+v", uninstalled[0], want)
		}
	})
	t.Run("IncludeAllProfiles does not duplicate a profile that has any store", func(t *testing.T) {
		t.Parallel()
		// Profile 5 holds two extension stores; with --all it must yield two
		// rows and no extra "not installed" row.
		got, err := Scan(context.Background(), Options{Roots: roots, IncludeAllProfiles: true})
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range got {
			if e.ProfileDir == "Profile 5" && !e.Installed {
				t.Errorf("Profile 5 has stores but got an uninstalled row: %+v", e)
			}
		}
		if len(got) != 5 {
			t.Errorf("want 5 entries (4 stores + Profile 9), got %d: %+v", len(got), got)
		}
	})
	t.Run("unreadable store is reported inline, scan continues", func(t *testing.T) {
		t.Parallel()
		bad := browsertest.Root(t,
			browsertest.ProfileSpec{Dir: "Default", Name: "a", Stores: map[string]map[string]any{ext: {browsertest.KeyBridgeDeviceID: 7}}},
			browsertest.ProfileSpec{Dir: "Profile 1", Name: "b", Stores: map[string]map[string]any{ext: {browsertest.KeyBridgeDeviceID: "ok"}}},
		)
		got, err := Scan(context.Background(), Options{Roots: []Root{{KindChrome, bad}}, Extensions: []ExtensionID{ExtClaudeCode}})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[0].Error == "" || got[0].DeviceID != "" || got[1].DeviceID != "ok" {
			t.Errorf("got %+v", got)
		}
	})
	t.Run("corrupt Local State aborts", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, localStateFile), []byte("{"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Scan(context.Background(), Options{Roots: []Root{{KindChrome, root}}}); err == nil {
			t.Error("want error")
		}
	})
	t.Run("empty roots yields no entries and no error", func(t *testing.T) {
		t.Parallel()
		got, err := Scan(context.Background(), Options{Roots: []Root{}})
		if err != nil || got != nil {
			t.Errorf("got %v, %v", got, err)
		}
	})
}

func TestSortEntries_RunningFirst(t *testing.T) {
	t.Parallel()
	es := []Entry{
		{DeviceID: "u", State: RunStateUnknown, Browser: KindArc},
		{DeviceID: "i", State: RunStateIdle, Browser: KindArc},
		{DeviceID: "r2", State: RunStateRunning, Browser: KindVivaldi},
		{DeviceID: "r1", State: RunStateRunning, Browser: KindChrome, ProfileDir: "Profile 2"},
		{DeviceID: "r0", State: RunStateRunning, Browser: KindChrome, ProfileDir: "Default"},
	}
	sortEntries(es)
	if got, want := ids(es), []string{"r0", "r1", "r2", "i", "u"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// Every RunState must have a sort rank, else a new state would silently sort
// as 0 (= running).
func TestRunStateRankCoversAllValues(t *testing.T) {
	t.Parallel()
	for _, s := range RunStateValues {
		if _, ok := runStateRank[s]; !ok {
			t.Errorf("runStateRank missing %s", s)
		}
	}
}

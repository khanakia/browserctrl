package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/khanakia/browserctrl/browser"
	"github.com/khanakia/browserctrl/browser/browsertest"
)

func TestPickOne(t *testing.T) {
	t.Parallel()
	run := browser.Entry{DeviceID: "run", State: browser.RunStateRunning}
	idle := browser.Entry{DeviceID: "idle", State: browser.RunStateIdle}
	idle2 := browser.Entry{DeviceID: "idle2", State: browser.RunStateIdle}
	noID := browser.Entry{State: browser.RunStateRunning}
	for _, tc := range []struct {
		name    string
		in      []browser.Entry
		want    string
		wantErr error
	}{
		{"none", nil, "", errNoMatch},
		{"only entries without ids", []browser.Entry{noID}, "", errNoMatch},
		{"single", []browser.Entry{idle}, "idle", nil},
		{"single with id beside id-less", []browser.Entry{noID, idle}, "idle", nil},
		{"several, one running wins", []browser.Entry{idle, run, idle2}, "run", nil},
		{"several idle is ambiguous", []browser.Entry{idle, idle2}, "", errAmbiguous},
		{"several running is ambiguous", []browser.Entry{run, {DeviceID: "run2", State: browser.RunStateRunning}}, "", errAmbiguous},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := pickOne(tc.in)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if got.DeviceID != tc.want {
				t.Errorf("got %q, want %q", got.DeviceID, tc.want)
			}
		})
	}
}

// End-to-end through cobra with a fixture root, checking stdout, stderr and
// exit codes for every subcommand shape.
func TestRun_EndToEnd(t *testing.T) {
	t.Parallel()
	ext := string(browser.ExtClaudeCode)
	root := browsertest.Root(t,
		browsertest.ProfileSpec{Dir: "Default", Name: "Aman", Email: "me@x.io", Stores: map[string]map[string]any{
			ext: {browsertest.KeyBridgeDeviceID: "id-default", browsertest.KeyBridgeDisplayName: "chrome1", browsertest.KeyMcpConnected: true},
		}},
		browsertest.ProfileSpec{Dir: "Profile 23", Name: "legable", Email: "a@legable.co", Stores: map[string]map[string]any{
			ext: {browsertest.KeyBridgeDeviceID: "id-legable"},
		}},
		browsertest.ProfileSpec{Dir: "Profile 4", Name: "fresh", Email: "f@x.io", Stores: map[string]map[string]any{
			ext: {},
		}},
	)
	exec := func(args ...string) (code int, out, errOut string) {
		var so, se bytes.Buffer
		code = run(append(args, "--"+flagRoot, root), &so, &se)
		return code, so.String(), se.String()
	}

	t.Run("list table", func(t *testing.T) {
		t.Parallel()
		code, out, _ := exec("list")
		if code != exitOK {
			t.Fatalf("exit %d", code)
		}
		for _, want := range []string{"STATE", "id-default", "id-legable", "chrome1", "a@legable.co", "custom", placeholderEmpty} {
			if !strings.Contains(out, want) {
				t.Errorf("table lacks %q:\n%s", want, out)
			}
		}
	})
	t.Run("list json", func(t *testing.T) {
		t.Parallel()
		code, out, _ := exec("list", "--"+flagJSON)
		if code != exitOK {
			t.Fatalf("exit %d", code)
		}
		var got []browser.Entry
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatal(err)
		}
		if len(got) != 3 || got[0].Browser != browser.KindCustom {
			t.Errorf("got %+v", got)
		}
	})
	t.Run("list --all widens to the desktop extension ids", func(t *testing.T) {
		// Pins the --all arm: a store under the desktop-app extension id is
		// invisible by default and appears with --all.
		t.Parallel()
		wide := browsertest.Root(t,
			browsertest.ProfileSpec{Dir: "Default", Name: "d", Email: "d@x.io", Stores: map[string]map[string]any{
				string(browser.ExtClaudeDesktop): {browsertest.KeyBridgeDeviceID: "id-desktop"},
			}},
		)
		var so, se bytes.Buffer
		if code := run([]string{"list", "--" + flagRoot, wide}, &so, &se); code != exitOK || strings.Contains(so.String(), "id-desktop") {
			t.Errorf("without --all: exit %d out %q", code, so.String())
		}
		so.Reset()
		if code := run([]string{"list", "--" + flagAll, "--" + flagRoot, wide}, &so, &se); code != exitOK || !strings.Contains(so.String(), "id-desktop") {
			t.Errorf("with --all: exit %d out %q", code, so.String())
		}
	})
	t.Run("list running filters idle fixture out", func(t *testing.T) {
		t.Parallel()
		code, out, _ := exec("list", "--"+flagRunning)
		if code != exitOK || !strings.Contains(out, "no Claude extension stores found") {
			t.Errorf("exit %d out %q", code, out)
		}
	})
	t.Run("find unique", func(t *testing.T) {
		t.Parallel()
		code, out, _ := exec("find", "legable")
		if code != exitOK || strings.TrimSpace(out) != "id-legable" {
			t.Errorf("exit %d out %q", code, out)
		}
	})
	t.Run("find json", func(t *testing.T) {
		t.Parallel()
		code, out, _ := exec("find", "chrome1", "--"+flagJSON)
		if code != exitOK {
			t.Fatalf("exit %d", code)
		}
		var got browser.Entry
		if err := json.Unmarshal([]byte(out), &got); err != nil || got.DeviceID != "id-default" {
			t.Errorf("got %+v err %v", got, err)
		}
	})
	t.Run("find no match exits 1", func(t *testing.T) {
		t.Parallel()
		code, out, errOut := exec("find", "firefox")
		if code != exitFailure || out != "" || !strings.Contains(errOut, errNoMatch.Error()) {
			t.Errorf("exit %d out %q err %q", code, out, errOut)
		}
	})
	t.Run("find ignores id-less matches when resolving", func(t *testing.T) {
		t.Parallel()
		// "x.io" matches Default (has id) and fresh (no id) → unique.
		code, out, _ := exec("find", "x.io")
		if code != exitOK || strings.TrimSpace(out) != "id-default" {
			t.Errorf("exit %d out %q", code, out)
		}
	})
	t.Run("find ambiguous exits 2 and lists candidates", func(t *testing.T) {
		t.Parallel()
		code, out, errOut := exec("find", "custom") // browser kind: matches all three
		if code != exitAmbiguous || out != "" || !strings.Contains(errOut, "id-default") || !strings.Contains(errOut, "id-legable") {
			t.Errorf("exit %d out %q err %q", code, out, errOut)
		}
	})
	t.Run("find without terms is a usage error", func(t *testing.T) {
		t.Parallel()
		if code, _, _ := exec("find"); code != exitFailure {
			t.Errorf("exit %d", code)
		}
	})
}

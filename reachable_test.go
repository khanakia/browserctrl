package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/khanakia/browserctrl/browser"
	"github.com/khanakia/browserctrl/browser/browsertest"
)

// signInSession points this process at a Claude Code profile signed in as
// uuid/email, the way CLAUDE_CONFIG_DIR does for a real session.
func signInSession(t *testing.T, uuid, email string) {
	t.Helper()
	dir := t.TempDir()
	body := `{"oauthAccount":{"accountUuid":"` + uuid + `","emailAddress":"` + email + `"}}`
	if err := os.WriteFile(filepath.Join(dir, ".claude.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
}

// --reachable keeps only browsers on the account the session runs as: the
// ones whose ids select_browser will accept. Not parallel — every subtest
// sets the session through the environment.
func TestReachable(t *testing.T) {
	ext := string(browser.ExtClaudeCode)
	root := browsertest.Root(t,
		browsertest.ProfileSpec{Dir: "Default", Name: "mine", Email: "me@x.io", Stores: map[string]map[string]any{
			ext: {browsertest.KeyBridgeDeviceID: "id-mine", browsertest.KeyAccountUUID: "acct-session"},
		}},
		browsertest.ProfileSpec{Dir: "Profile 2", Name: "theirs", Email: "t@x.io", Stores: map[string]map[string]any{
			ext: {browsertest.KeyBridgeDeviceID: "id-theirs", browsertest.KeyAccountUUID: "acct-other"},
		}},
		browsertest.ProfileSpec{Dir: "Profile 3", Name: "signedout", Email: "s@x.io", Stores: map[string]map[string]any{
			ext: {browsertest.KeyBridgeDeviceID: "id-signedout"},
		}},
	)
	exec := func(args ...string) (code int, out, errOut string) {
		var so, se bytes.Buffer
		code = run(append(args, "--"+flagRoot, root), &so, &se)
		return code, so.String(), se.String()
	}

	t.Run("list keeps only the session's account", func(t *testing.T) {
		signInSession(t, "acct-session", "me@claude.io")
		code, out, _ := exec("list", "--"+flagReachable)
		if code != exitOK || !strings.Contains(out, "id-mine") {
			t.Fatalf("exit %d out:\n%s", code, out)
		}
		for _, leak := range []string{"id-theirs", "id-signedout"} {
			if strings.Contains(out, leak) {
				t.Errorf("%s is not on the session account but was listed:\n%s", leak, out)
			}
		}
	})
	t.Run("the same command answers differently for another session", func(t *testing.T) {
		// Reachability is a fact about the asking session, so switching the
		// profile must switch the answer.
		signInSession(t, "acct-other", "them@claude.io")
		code, out, _ := exec("list", "--"+flagReachable)
		if code != exitOK || !strings.Contains(out, "id-theirs") || strings.Contains(out, "id-mine") {
			t.Errorf("exit %d out:\n%s", code, out)
		}
	})
	t.Run("json carries only reachable entries", func(t *testing.T) {
		signInSession(t, "acct-session", "me@claude.io")
		code, out, _ := exec("list", "--"+flagReachable, "--"+flagJSON)
		if code != exitOK {
			t.Fatalf("exit %d", code)
		}
		var got []browser.Entry
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].DeviceID != "id-mine" {
			t.Errorf("got %+v", got)
		}
	})
	t.Run("find refuses an id the session cannot reach", func(t *testing.T) {
		signInSession(t, "acct-session", "me@claude.io")
		if code, out, _ := exec("find", "theirs", "--"+flagReachable); code != exitFailure || out != "" {
			t.Errorf("exit %d out %q, want no match", code, out)
		}
		if code, out, _ := exec("find", "mine", "--"+flagReachable); code != exitOK || strings.TrimSpace(out) != "id-mine" {
			t.Errorf("exit %d out %q", code, out)
		}
	})
	t.Run("no browser on the session account says so by name", func(t *testing.T) {
		// The generic "is the extension installed?" hint would mislead: it
		// is installed, just signed in elsewhere.
		signInSession(t, "acct-nobody", "lonely@claude.io")
		code, out, _ := exec("list", "--"+flagReachable)
		if code != exitOK || !strings.Contains(out, "lonely@claude.io") || strings.Contains(out, emptyHintStores) {
			t.Errorf("exit %d out %q", code, out)
		}
	})
	t.Run("an account with no email is named by uuid", func(t *testing.T) {
		signInSession(t, "acct-nobody", "")
		code, out, _ := exec("list", "--"+flagReachable)
		if code != exitOK || !strings.Contains(out, "acct-nobody") {
			t.Errorf("exit %d out %q", code, out)
		}
	})
	t.Run("an unknown session account is an error, not an empty list", func(t *testing.T) {
		// "Nothing reachable" and "cannot tell" need different actions; an
		// agent given an empty list would wrongly conclude the former.
		t.Setenv("HOME", t.TempDir())
		t.Setenv("CLAUDE_CONFIG_DIR", "")
		code, out, errOut := exec("list", "--"+flagReachable)
		if code != exitFailure || out != "" || !strings.Contains(errOut, flagReachable) {
			t.Errorf("exit %d out %q err %q", code, out, errOut)
		}
	})
	t.Run("no home at all is the same error", func(t *testing.T) {
		t.Setenv("HOME", "")
		t.Setenv("CLAUDE_CONFIG_DIR", "")
		if code, _, errOut := exec("list", "--"+flagReachable); code != exitFailure || !strings.Contains(errOut, flagReachable) {
			t.Errorf("exit %d err %q", code, errOut)
		}
	})
	t.Run("without the flag nothing is filtered", func(t *testing.T) {
		signInSession(t, "acct-session", "me@claude.io")
		code, out, _ := exec("list")
		if code != exitOK || !strings.Contains(out, "id-theirs") || !strings.Contains(out, "id-mine") {
			t.Errorf("exit %d out:\n%s", code, out)
		}
	})
}

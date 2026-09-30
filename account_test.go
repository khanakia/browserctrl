package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/khanakia/browserctrl/browser"
)

func TestAccountAliases(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		in      []string
		want    map[string]string
		wantErr bool
	}{
		{name: "none", in: nil, want: map[string]string{}},
		{name: "one pair", in: []string{"u-1=work"}, want: map[string]string{"u-1": "work"}},
		{name: "several", in: []string{"u-1=work", "u-2=personal"}, want: map[string]string{"u-1": "work", "u-2": "personal"}},
		// A label may contain "=" (an email-ish label, a query string); only
		// the FIRST separator splits, so the label survives intact.
		{name: "label keeps later separators", in: []string{"u-1=a=b"}, want: map[string]string{"u-1": "a=b"}},
		{name: "last wins for a repeated uuid", in: []string{"u-1=first", "u-1=second"}, want: map[string]string{"u-1": "second"}},
		{name: "no separator", in: []string{"u-1"}, wantErr: true},
		{name: "empty uuid", in: []string{"=work"}, wantErr: true},
		{name: "empty label", in: []string{"u-1="}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := accountAliases(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got %v", got)
				}
				// The message must name the flag and the offending value, or
				// the user cannot tell which of several pairs was wrong.
				if !strings.Contains(err.Error(), flagAccountAlias) {
					t.Errorf("error does not name the flag: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNewAccountLabeler(t *testing.T) {
	t.Parallel()
	known := map[string]string{"u-personal": "me@x.io", "u-work": "me@work.io"}
	aliases := map[string]string{"u-other": "client account"}
	entries := []browser.Entry{
		{AccountUUID: "u-personal"}, {AccountUUID: "u-work"}, {AccountUUID: "u-other"}, {AccountUUID: "u-third"}, {},
	}
	label := newAccountLabeler(known, aliases, entries)

	for _, tc := range []struct {
		name string
		in   browser.Entry
		want string
	}{
		{"signed out shows nothing", browser.Entry{}, ""},
		// Every account some profile is signed in as gets its email — not
		// just the profile the current shell happens to select.
		{"a known account shows its email", browser.Entry{AccountUUID: "u-personal"}, "me@x.io"},
		{"a second known account shows its email too", browser.Entry{AccountUUID: "u-work"}, "me@work.io"},
		{"aliased account shows the label", browser.Entry{AccountUUID: "u-other"}, "client account"},
		// The only unnamed account in the set: no number to disambiguate
		// against, so numbering it would be noise.
		{"the one unnamed account is just other", browser.Entry{AccountUUID: "u-third"}, labelOtherAccount},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := label(tc.in); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("several unnamed accounts are numbered in listing order", func(t *testing.T) {
		t.Parallel()
		es := []browser.Entry{{AccountUUID: "u-b"}, {AccountUUID: "u-a"}, {AccountUUID: "u-b"}}
		l := newAccountLabeler(nil, nil, es)
		if got, want := l(es[0]), labelOtherAccount+" 1"; got != want {
			t.Errorf("first seen: got %q, want %q", got, want)
		}
		if got, want := l(es[1]), labelOtherAccount+" 2"; got != want {
			t.Errorf("second seen: got %q, want %q", got, want)
		}
		// Same account twice must keep the same number, or the table would
		// imply more accounts than exist.
		if got, want := l(es[2]), labelOtherAccount+" 1"; got != want {
			t.Errorf("repeat: got %q, want %q", got, want)
		}
	})
	t.Run("a known account does not consume a number", func(t *testing.T) {
		t.Parallel()
		// With one named and one unnamed account, the unnamed one is the only
		// "other" and must not be called "other account 2".
		es := []browser.Entry{{AccountUUID: "u-personal"}, {AccountUUID: "u-x"}}
		l := newAccountLabeler(known, nil, es)
		if got := l(es[1]); got != labelOtherAccount {
			t.Errorf("got %q, want %q", got, labelOtherAccount)
		}
	})
	t.Run("an unknown account not in the set still labels", func(t *testing.T) {
		t.Parallel()
		// Defensive: the labeler must never return an empty cell for a
		// signed-in profile just because it was not in the numbering pass.
		l := newAccountLabeler(known, nil, nil)
		if got := l(browser.Entry{AccountUUID: "u-surprise"}); got != labelOtherAccount {
			t.Errorf("got %q, want %q", got, labelOtherAccount)
		}
	})
	t.Run("an alias never overrides a known email", func(t *testing.T) {
		t.Parallel()
		// A stale alias must not rename an account the machine can prove.
		l := newAccountLabeler(known, map[string]string{"u-personal": "WRONG"}, nil)
		if got := l(browser.Entry{AccountUUID: "u-personal"}); got != "me@x.io" {
			t.Errorf("got %q, want the known email", got)
		}
	})
	t.Run("a known account with no email falls back rather than printing blank", func(t *testing.T) {
		t.Parallel()
		l := newAccountLabeler(map[string]string{"u-noemail": ""}, nil, nil)
		if got := l(browser.Entry{AccountUUID: "u-noemail"}); got != labelOtherAccount {
			t.Errorf("got %q, want %q", got, labelOtherAccount)
		}
	})
}

// knownAccounts must degrade to "no names" rather than fail: a browser
// inventory is still useful on a machine where Claude Code is absent or
// logged out, it just cannot name any account for free.
func TestKnownAccounts_EmptyWhenNothingIsSignedIn(t *testing.T) {
	// Not parallel: mutates the environment.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	if got := knownAccounts(); len(got) != 0 {
		t.Errorf("got %v, want no accounts", got)
	}
}

// And with no home at all, the candidate paths cannot even be built.
func TestKnownAccounts_NoHome(t *testing.T) {
	// Not parallel: mutates the environment.
	t.Setenv("HOME", "")
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	if got := knownAccounts(); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

// The regression this exists for: the label must not depend on which shell
// asked. A profile directory next to the default one names its account even
// when CLAUDE_CONFIG_DIR is unset, exactly as a plain terminal runs it.
func TestKnownAccounts_FindsSiblingProfilesWithoutTheEnvVar(t *testing.T) {
	// Not parallel: mutates the environment.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	write := func(path, uuid, email string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		body := `{"oauthAccount":{"accountUuid":"` + uuid + `","emailAddress":"` + email + `"}}`
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(home, ".claude.json"), "u-personal", "me@x.io")
	write(filepath.Join(home, ".claude-work", ".claude.json"), "u-work", "me@work.io")

	got := knownAccounts()
	want := map[string]string{"u-personal": "me@x.io", "u-work": "me@work.io"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// A filter must never rename an account: `list` and `list --running` label
// the same browser identically, because labels are computed over the whole
// scan rather than the rows being printed.
func TestAccountLabels_AreStableUnderFiltering(t *testing.T) {
	t.Parallel()
	all := []browser.Entry{
		{AccountUUID: "u-a", State: browser.RunStateIdle},
		{AccountUUID: "u-b", State: browser.RunStateRunning},
	}
	running := browser.OnlyRunning(all)

	full := newAccountLabeler(nil, nil, all)
	filtered := newAccountLabeler(nil, nil, all)
	if got, want := filtered(running[0]), full(all[1]); got != want {
		t.Errorf("filtered listing says %q, full listing says %q", got, want)
	}
	if full(all[0]) == full(all[1]) {
		t.Errorf("two different accounts share the label %q", full(all[0]))
	}
}

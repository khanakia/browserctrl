package main

import (
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
	session := browser.ClaudeAccount{UUID: "u-session", Email: "me@x.io"}
	aliases := map[string]string{"u-other": "work account"}
	entries := []browser.Entry{
		{AccountUUID: "u-session"}, {AccountUUID: "u-other"}, {AccountUUID: "u-third"}, {},
	}
	label := newAccountLabeler(session, aliases, entries)

	for _, tc := range []struct {
		name string
		in   browser.Entry
		want string
	}{
		{"signed out shows nothing", browser.Entry{}, ""},
		{"session account shows its email", browser.Entry{AccountUUID: "u-session"}, "me@x.io"},
		{"aliased account shows the label", browser.Entry{AccountUUID: "u-other"}, "work account"},
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
		l := newAccountLabeler(browser.ClaudeAccount{}, nil, es)
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
	t.Run("an unknown account not in the set still labels", func(t *testing.T) {
		t.Parallel()
		// Defensive: the labeler must never return an empty cell for a
		// signed-in profile just because it was not in the numbering pass.
		l := newAccountLabeler(session, nil, nil)
		if got := l(browser.Entry{AccountUUID: "u-surprise"}); got != labelOtherAccount {
			t.Errorf("got %q, want %q", got, labelOtherAccount)
		}
	})

	t.Run("an alias never overrides the session account", func(t *testing.T) {
		t.Parallel()
		// Otherwise a stale alias could rename the one account we can prove,
		// which is the only trustworthy label in the table.
		l := newAccountLabeler(session, map[string]string{"u-session": "WRONG"}, nil)
		if got := l(browser.Entry{AccountUUID: "u-session"}); got != "me@x.io" {
			t.Errorf("got %q, want the session email", got)
		}
	})
	t.Run("no signed-in account falls back to uuids", func(t *testing.T) {
		t.Parallel()
		l := newAccountLabeler(browser.ClaudeAccount{}, nil, nil)
		if got := l(browser.Entry{AccountUUID: "u-short"}); got != labelOtherAccount {
			t.Errorf("got %q, want %q", got, labelOtherAccount)
		}
	})
	t.Run("session account without an email falls back to its uuid", func(t *testing.T) {
		t.Parallel()
		// An account we can identify but cannot name is no better than any
		// other uuid — it must not print an empty cell.
		l := newAccountLabeler(browser.ClaudeAccount{UUID: "u-short"}, nil, nil)
		if got := l(browser.Entry{AccountUUID: "u-short"}); got != labelOtherAccount {
			t.Errorf("got %q, want %q", got, labelOtherAccount)
		}
	})
}

// sessionAccount must degrade to the zero account rather than fail: a browser
// inventory is still useful on a machine where Claude Code is absent or
// logged out, it just cannot name any account for free.
func TestSessionAccount_FallsBackWhenNothingIsSignedIn(t *testing.T) {
	// Not parallel: mutates HOME.
	t.Setenv("HOME", t.TempDir())
	if got := sessionAccount(); got != (browser.ClaudeAccount{}) {
		t.Errorf("got %+v, want the zero account", got)
	}
}

// And with no home at all, the path cannot even be built.
func TestSessionAccount_NoHome(t *testing.T) {
	// Not parallel: mutates HOME.
	t.Setenv("HOME", "")
	if got := sessionAccount(); got != (browser.ClaudeAccount{}) {
		t.Errorf("got %+v, want the zero account", got)
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

	full := newAccountLabeler(browser.ClaudeAccount{}, nil, all)
	filtered := newAccountLabeler(browser.ClaudeAccount{}, nil, all)
	if got, want := filtered(running[0]), full(all[1]); got != want {
		t.Errorf("filtered listing says %q, full listing says %q", got, want)
	}
	if full(all[0]) == full(all[1]) {
		t.Errorf("two different accounts share the label %q", full(all[0]))
	}
}

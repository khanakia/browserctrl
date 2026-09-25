package browser

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// The config holds a lot more than the account; the reader must pick the one
// object out and ignore the rest without choking on it.
func TestReadClaudeAccount(t *testing.T) {
	t.Parallel()
	write := func(t *testing.T, body string) string {
		t.Helper()
		p := filepath.Join(t.TempDir(), claudeConfigFile)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	t.Run("reads the signed-in account past unrelated config", func(t *testing.T) {
		t.Parallel()
		p := write(t, `{"projects":{"a":["/x"]},"oauthAccount":{"accountUuid":"u-1","emailAddress":"me@x.io","displayName":"Aman","fullName":"Aman B","organizationUuid":"o-1","organizationName":"My Org"},"tipsHistory":{"n":3}}`)
		got, err := ReadClaudeAccount(p)
		if err != nil {
			t.Fatal(err)
		}
		want := ClaudeAccount{UUID: "u-1", Email: "me@x.io", DisplayName: "Aman", FullName: "Aman B", OrgUUID: "o-1", OrgName: "My Org"}
		if got != want {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})
	t.Run("missing file is ErrNoClaudeAccount", func(t *testing.T) {
		t.Parallel()
		_, err := ReadClaudeAccount(filepath.Join(t.TempDir(), "absent.json"))
		if !errors.Is(err, ErrNoClaudeAccount) {
			t.Errorf("got %v, want ErrNoClaudeAccount", err)
		}
	})
	t.Run("config with no account is ErrNoClaudeAccount", func(t *testing.T) {
		t.Parallel()
		if _, err := ReadClaudeAccount(write(t, `{"projects":{}}`)); !errors.Is(err, ErrNoClaudeAccount) {
			t.Errorf("got %v, want ErrNoClaudeAccount", err)
		}
	})
	t.Run("account without a uuid is ErrNoClaudeAccount", func(t *testing.T) {
		t.Parallel()
		if _, err := ReadClaudeAccount(write(t, `{"oauthAccount":{"emailAddress":"me@x.io"}}`)); !errors.Is(err, ErrNoClaudeAccount) {
			t.Errorf("got %v, want ErrNoClaudeAccount", err)
		}
	})
	t.Run("malformed JSON is a real error, not a shrug", func(t *testing.T) {
		t.Parallel()
		_, err := ReadClaudeAccount(write(t, `{`))
		if err == nil || errors.Is(err, ErrNoClaudeAccount) {
			t.Errorf("got %v, want a parse error", err)
		}
	})
	t.Run("malformed oauthAccount is a real error", func(t *testing.T) {
		t.Parallel()
		_, err := ReadClaudeAccount(write(t, `{"oauthAccount":"nope"}`))
		if err == nil || errors.Is(err, ErrNoClaudeAccount) {
			t.Errorf("got %v, want a parse error", err)
		}
	})
}

// A path that exists but cannot be read as a file (here: a directory) must
// surface as a real error, not as "nobody is signed in" — the two lead the
// caller to different conclusions.
func TestReadClaudeAccount_UnreadablePathIsAnError(t *testing.T) {
	t.Parallel()
	_, err := ReadClaudeAccount(t.TempDir())
	if err == nil || errors.Is(err, ErrNoClaudeAccount) {
		t.Errorf("got %v, want a read error", err)
	}
}

func TestDefaultClaudeConfigPath(t *testing.T) {
	// Not parallel: mutates HOME.
	home := t.TempDir()
	t.Setenv("HOME", home)
	got, err := DefaultClaudeConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, claudeConfigFile); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// With no home directory there is no config to point at; the error must be
// returned rather than a bare filename that would resolve against the cwd.
func TestDefaultClaudeConfigPath_NoHome(t *testing.T) {
	// Not parallel: mutates HOME.
	t.Setenv("HOME", "")
	if got, err := DefaultClaudeConfigPath(); err == nil {
		t.Errorf("got %q, want an error", got)
	}
}

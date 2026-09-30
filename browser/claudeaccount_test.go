package browser

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
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

// A machine with several Claude Code profiles selects one with
// CLAUDE_CONFIG_DIR, and each holds its own signed-in account. Ignoring the
// variable reads the wrong profile's account and mislabels every browser.
func TestDefaultClaudeConfigPath_HonoursConfigDirEnv(t *testing.T) {
	// Not parallel: mutates the environment.
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv(claudeConfigDirEnv, dir)
	got, err := DefaultClaudeConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, claudeConfigFile); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// An empty value is not a directory; it must fall back to $HOME rather than
// resolve to a bare ".claude.json" against the working directory.
func TestDefaultClaudeConfigPath_EmptyConfigDirFallsBackToHome(t *testing.T) {
	// Not parallel: mutates the environment.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(claudeConfigDirEnv, "")
	got, err := DefaultClaudeConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, claudeConfigFile); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDefaultClaudeConfigPath(t *testing.T) {
	// Not parallel: mutates the environment.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(claudeConfigDirEnv, "")
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
	// Not parallel: mutates the environment.
	t.Setenv("HOME", "")
	t.Setenv(claudeConfigDirEnv, "")
	if got, err := DefaultClaudeConfigPath(); err == nil {
		t.Errorf("got %q, want an error", got)
	}
}

// writeConfig drops a minimal Claude Code config naming one account.
func writeConfig(t *testing.T, path, uuid, email string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"oauthAccount":{"accountUuid":"` + uuid + `","emailAddress":"` + email + `"}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestClaudeConfigPaths(t *testing.T) {
	// Not parallel: every subtest mutates the environment.
	t.Run("home config plus every sibling profile dir, env unset", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv(claudeConfigDirEnv, "")
		for _, d := range []string{".claude", ".claude-work"} {
			if err := os.MkdirAll(filepath.Join(home, d), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		got, err := ClaudeConfigPaths()
		if err != nil {
			t.Fatal(err)
		}
		want := []string{
			filepath.Join(home, claudeConfigFile),
			filepath.Join(home, ".claude", claudeConfigFile),
			filepath.Join(home, ".claude-work", claudeConfigFile),
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got  %v\nwant %v", got, want)
		}
	})
	t.Run("files matching the profile pattern are not treated as profile dirs", func(t *testing.T) {
		// ~/.claude.json is a FILE that matches ".claude*". Treating it as a
		// directory yields ~/.claude.json/.claude.json, which can only fail
		// with "not a directory" and showed up as a spurious read error.
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv(claudeConfigDirEnv, "")
		writeConfig(t, filepath.Join(home, claudeConfigFile), "u-1", "one@x.io")
		if err := os.WriteFile(filepath.Join(home, ".claude.json.backup"), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := ClaudeConfigPaths()
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{filepath.Join(home, claudeConfigFile)}; !reflect.DeepEqual(got, want) {
			t.Errorf("got  %v\nwant %v", got, want)
		}
		// And reading them is clean: one account, no error.
		accts, err := ReadClaudeAccounts(got)
		if err != nil || len(accts) != 1 {
			t.Errorf("got %+v, %v", accts, err)
		}
	})
	t.Run("a home path with glob metacharacters still works", func(t *testing.T) {
		// "[" makes a filepath.Glob pattern malformed; discovery must not
		// depend on the home path being pattern-safe.
		home := filepath.Join(t.TempDir(), "we[ird")
		work := filepath.Join(home, ".claude-work")
		if err := os.MkdirAll(work, 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("HOME", home)
		t.Setenv(claudeConfigDirEnv, "")
		got, err := ClaudeConfigPaths()
		if err != nil {
			t.Fatal(err)
		}
		want := []string{filepath.Join(home, claudeConfigFile), filepath.Join(work, claudeConfigFile)}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got  %v\nwant %v", got, want)
		}
	})
	t.Run("a symlinked profile directory is followed", func(t *testing.T) {
		home, real := t.TempDir(), t.TempDir()
		if err := os.Symlink(real, filepath.Join(home, ".claude-work")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		t.Setenv("HOME", home)
		t.Setenv(claudeConfigDirEnv, "")
		got, err := ClaudeConfigPaths()
		if err != nil {
			t.Fatal(err)
		}
		want := []string{filepath.Join(home, claudeConfigFile), filepath.Join(home, ".claude-work", claudeConfigFile)}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got  %v\nwant %v", got, want)
		}
	})
	t.Run("a home that does not exist still yields the explicit candidates", func(t *testing.T) {
		home := filepath.Join(t.TempDir(), "gone")
		t.Setenv("HOME", home)
		t.Setenv(claudeConfigDirEnv, "")
		got, err := ClaudeConfigPaths()
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{filepath.Join(home, claudeConfigFile)}; !reflect.DeepEqual(got, want) {
			t.Errorf("got  %v\nwant %v", got, want)
		}
	})
	t.Run("the env-selected config comes first", func(t *testing.T) {
		home, elsewhere := t.TempDir(), t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv(claudeConfigDirEnv, elsewhere)
		got, err := ClaudeConfigPaths()
		if err != nil {
			t.Fatal(err)
		}
		if len(got) == 0 || got[0] != filepath.Join(elsewhere, claudeConfigFile) {
			t.Errorf("got %v, want the CLAUDE_CONFIG_DIR config first", got)
		}
	})
	t.Run("an env dir that is also a sibling is listed once", func(t *testing.T) {
		home := t.TempDir()
		work := filepath.Join(home, ".claude-work")
		if err := os.MkdirAll(work, 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("HOME", home)
		t.Setenv(claudeConfigDirEnv, work)
		got, err := ClaudeConfigPaths()
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, p := range got {
			if p == filepath.Join(work, claudeConfigFile) {
				n++
			}
		}
		if n != 1 {
			t.Errorf("work config listed %d times in %v", n, got)
		}
	})
	t.Run("no home is an error", func(t *testing.T) {
		t.Setenv("HOME", "")
		t.Setenv(claudeConfigDirEnv, "")
		if got, err := ClaudeConfigPaths(); err == nil {
			t.Errorf("got %v, want an error", got)
		}
	})
}

func TestReadClaudeAccounts(t *testing.T) {
	t.Parallel()
	t.Run("one account per profile, absent paths skipped", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		a, b := filepath.Join(dir, "a.json"), filepath.Join(dir, "b.json")
		writeConfig(t, a, "u-1", "one@x.io")
		writeConfig(t, b, "u-2", "two@x.io")
		got, err := ReadClaudeAccounts([]string{a, filepath.Join(dir, "absent.json"), b})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[0].Email != "one@x.io" || got[1].Email != "two@x.io" {
			t.Errorf("got %+v", got)
		}
	})
	t.Run("two profiles on the same account yield it once", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		a, b := filepath.Join(dir, "a.json"), filepath.Join(dir, "b.json")
		writeConfig(t, a, "u-1", "one@x.io")
		writeConfig(t, b, "u-1", "one@x.io")
		got, err := ReadClaudeAccounts([]string{a, b})
		if err != nil || len(got) != 1 {
			t.Errorf("got %+v, %v", got, err)
		}
	})
	t.Run("a broken profile is reported without losing the others", func(t *testing.T) {
		t.Parallel()
		// One unparseable config must not un-name every other account: the
		// good one is still returned, alongside an error naming the bad one.
		dir := t.TempDir()
		good, bad := filepath.Join(dir, "good.json"), filepath.Join(dir, "bad.json")
		writeConfig(t, good, "u-1", "one@x.io")
		if err := os.WriteFile(bad, []byte("{"), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := ReadClaudeAccounts([]string{bad, good})
		if err == nil {
			t.Error("want an error for the broken config")
		}
		if len(got) != 1 || got[0].UUID != "u-1" {
			t.Errorf("got %+v, want the good account", got)
		}
	})
	t.Run("no paths, no accounts, no error", func(t *testing.T) {
		t.Parallel()
		if got, err := ReadClaudeAccounts(nil); err != nil || got != nil {
			t.Errorf("got %v, %v", got, err)
		}
	})
}

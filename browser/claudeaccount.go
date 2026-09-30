package browser

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ClaudeAccount names a Claude account uuid.
//
// Why it exists: Entry.AccountUUID decides which browsers a Claude Code
// session can reach, but a uuid is unreadable. Claude Code records the
// account it is signed in as — uuid, email, display name, organization — in
// its own config file, and that is the ONLY place on this machine where a
// Claude account uuid is paired with a human-readable identity.
//
// Invariant: one config names exactly ONE account — the one that Claude Code
// profile is signed in as right now. A machine with several profiles
// (CLAUDE_CONFIG_DIR) therefore names several accounts, one per config; see
// ClaudeConfigPaths. An account no profile is signed in as cannot be named
// at all: verified 2026-09-25 that its uuid appears in the extension stores,
// in claude.ai's own site data (`__qk_hint_account_uuid`, `ccd-sync-owner`)
// and in past session transcripts, but its email is stored nowhere. Callers
// that need to name such an account must be given the label by the user.
type ClaudeAccount struct {
	// UUID matches Entry.AccountUUID for a profile signed in to this account.
	UUID string `json:"accountUuid"`
	// Email is the address the account signs in with; the label worth showing.
	Email string `json:"emailAddress"`
	// DisplayName / FullName are the profile names Claude shows, either may be
	// empty depending on how the account was set up.
	DisplayName string `json:"displayName"`
	FullName    string `json:"fullName"`
	// OrgUUID matches Entry.OrgUUID; OrgName is its human-readable name.
	OrgUUID string `json:"organizationUuid"`
	OrgName string `json:"organizationName"`
}

// claudeConfigFile is Claude Code's config, oauthAccountField is the object
// inside it holding the signed-in account, and claudeConfigDirEnv is the
// variable Claude Code honours to relocate its whole config directory. All
// three are Claude Code's layout, not ours: read-only, and absent on a
// machine without Claude Code.
const (
	claudeConfigFile   = ".claude.json"
	oauthAccountField  = "oauthAccount"
	claudeConfigDirEnv = "CLAUDE_CONFIG_DIR"
	// claudeProfileDirPrefix is what the directories people point
	// CLAUDE_CONFIG_DIR at start with when they keep several Claude Code
	// profiles: siblings of the default ~/.claude named ~/.claude-work,
	// ~/.claude-acme and so on. It is a convention, not a rule — a profile
	// kept elsewhere is simply not discovered, and its account stays unnamed.
	claudeProfileDirPrefix = ".claude"
)

// ErrNoClaudeAccount reports that the config exists but names no account —
// a machine where Claude Code has never signed in. Distinct from a read
// error so a caller can degrade to showing uuids instead of failing.
var ErrNoClaudeAccount = fmt.Errorf("browser: no signed-in Claude account in %s", claudeConfigFile)

// DefaultClaudeConfigPath is $CLAUDE_CONFIG_DIR/.claude.json when that
// variable is set, else ~/.claude.json.
//
// Why the variable matters: someone who keeps more than one Claude Code
// profile on a machine switches between them with CLAUDE_CONFIG_DIR, and each
// directory holds its OWN signed-in account. Reading ~/.claude.json
// unconditionally compares browsers against whatever account the default
// profile last used, which silently mislabels every row as belonging to some
// other account — observed on a machine running a work profile while the home
// config still named the personal login (2026-09-30).
//
// Why a function and not a constant: the environment is read at call time, so
// a caller (or a test) can point somewhere else by passing its own path to
// ReadClaudeAccount instead of the package reaching for $HOME on its own.
func DefaultClaudeConfigPath() (string, error) {
	if dir := os.Getenv(claudeConfigDirEnv); dir != "" {
		return filepath.Join(dir, claudeConfigFile), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, claudeConfigFile), nil
}

// ClaudeConfigPaths lists every Claude Code config worth reading, most
// specific first: the one CLAUDE_CONFIG_DIR selects, the default
// ~/.claude.json, then each sibling profile directory's config. Paths are
// returned whether or not they exist; ReadClaudeAccounts skips the absent.
//
// Why more than one: which account a browser is signed in to is a fact about
// the browser, not about the shell asking. Resolving only the current
// process's profile made the answer depend on an environment variable, so
// the same command named an account inside a Claude Code session and printed
// "other account" for the very same browser from a plain terminal a moment
// later (2026-09-30). Every profile on the machine knows one account by
// name; reading all of them names every account that can be named.
func ClaudeConfigPaths() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("home dir: %w", err)
	}
	var paths []string
	if dir := os.Getenv(claudeConfigDirEnv); dir != "" {
		paths = append(paths, filepath.Join(dir, claudeConfigFile))
	}
	paths = append(paths, filepath.Join(home, claudeConfigFile))
	// A directory listing rather than filepath.Glob: the pattern would embed
	// $HOME, and a home path containing a glob metacharacter ("[") is a
	// malformed pattern. An unreadable home just means no sibling profiles
	// are discovered — the two explicit candidates above still stand.
	entries, err := os.ReadDir(home)
	if err != nil {
		return dedupe(paths), nil
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), claudeProfileDirPrefix) {
			continue
		}
		dir := filepath.Join(home, e.Name())
		// The prefix also matches FILES — ~/.claude.json itself, its backups
		// — and a config "inside" a file is a path that can only ever fail
		// with ENOTDIR, surfacing as a bogus read error. os.Stat rather than
		// the entry's own type so a SYMLINKED profile directory counts.
		if st, statErr := os.Stat(dir); statErr != nil || !st.IsDir() {
			continue
		}
		paths = append(paths, filepath.Join(dir, claudeConfigFile))
	}
	return dedupe(paths), nil
}

// dedupe keeps the first occurrence of each path, preserving order, so a
// CLAUDE_CONFIG_DIR that is also a sibling profile is read once.
func dedupe(paths []string) []string {
	seen := make(map[string]bool, len(paths))
	out := paths[:0:0]
	for _, p := range paths {
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// ReadClaudeAccounts reads every config in paths and returns the distinct
// accounts found, in path order.
//
// A path with no account (absent file, never signed in) is skipped silently:
// that is the normal state of most candidates. A config that exists but
// cannot be read or parsed is reported in the returned error WITHOUT
// discarding the accounts that did load — one broken profile must not
// un-name every other account. Callers that only want labels can use the
// slice and ignore the error.
func ReadClaudeAccounts(paths []string) ([]ClaudeAccount, error) {
	var (
		out  []ClaudeAccount
		errs []error
	)
	seen := make(map[string]bool, len(paths))
	for _, p := range paths {
		acct, err := ReadClaudeAccount(p)
		switch {
		case errors.Is(err, ErrNoClaudeAccount):
			continue
		case err != nil:
			errs = append(errs, err)
			continue
		}
		if seen[acct.UUID] {
			continue
		}
		seen[acct.UUID] = true
		out = append(out, acct)
	}
	return out, errors.Join(errs...)
}

// ReadClaudeAccount parses path and returns the account Claude Code is signed
// in as. A missing file or a config with no account yields
// ErrNoClaudeAccount; malformed JSON is a real error, because silently
// pretending nobody is signed in would hide a broken config.
//
// The file holds far more than the account (project history, caches); only
// the oauthAccount object is decoded, and nothing is ever written back.
func ReadClaudeAccount(path string) (ClaudeAccount, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ClaudeAccount{}, ErrNoClaudeAccount
	}
	if err != nil {
		return ClaudeAccount{}, fmt.Errorf("read %s: %w", path, err)
	}
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return ClaudeAccount{}, fmt.Errorf("parse %s: %w", path, err)
	}
	blob, ok := cfg[oauthAccountField]
	if !ok {
		return ClaudeAccount{}, ErrNoClaudeAccount
	}
	var acct ClaudeAccount
	if err := json.Unmarshal(blob, &acct); err != nil {
		return ClaudeAccount{}, fmt.Errorf("parse %s.%s: %w", path, oauthAccountField, err)
	}
	if acct.UUID == "" {
		return ClaudeAccount{}, ErrNoClaudeAccount
	}
	return acct, nil
}

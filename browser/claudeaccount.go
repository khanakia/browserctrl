package browser

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ClaudeAccount names a Claude account uuid.
//
// Why it exists: Entry.AccountUUID decides which browsers a Claude Code
// session can reach, but a uuid is unreadable. Claude Code records the
// account it is signed in as — uuid, email, display name, organization — in
// its own config file, and that is the ONLY place on this machine where a
// Claude account uuid is paired with a human-readable identity.
//
// Invariant: this resolves exactly ONE account, the one Claude Code is signed
// in as right now. Verified on a machine with two accounts in use
// (2026-09-25): the second account's uuid appears in the extension stores, in
// claude.ai's own site data (`__qk_hint_account_uuid`, `ccd-sync-owner`) and
// in past session transcripts, but no email for it is stored anywhere. Callers
// that need to name another account must be given the label by the user.
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

// claudeConfigFile is Claude Code's config, and oauthAccountField is the
// object inside it holding the signed-in account. Both are Claude Code's
// layout, not ours: read-only, and absent on a machine without Claude Code.
const (
	claudeConfigFile  = ".claude.json"
	oauthAccountField = "oauthAccount"
)

// ErrNoClaudeAccount reports that the config exists but names no account —
// a machine where Claude Code has never signed in. Distinct from a read
// error so a caller can degrade to showing uuids instead of failing.
var ErrNoClaudeAccount = fmt.Errorf("browser: no signed-in Claude account in %s", claudeConfigFile)

// DefaultClaudeConfigPath is ~/.claude.json.
//
// Why a function and not a constant: $HOME is resolved at call time, so a
// caller (or a test) can point somewhere else by passing its own path to
// ReadClaudeAccount instead of the package reaching for $HOME on its own.
func DefaultClaudeConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, claudeConfigFile), nil
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

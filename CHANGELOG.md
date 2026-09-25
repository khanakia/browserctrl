# Changelog

All notable changes to **browserctrl** are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
### Changed
### Deprecated
### Removed
### Fixed
### Security

## [0.1.2] - 2026-09-25

### Added
- `CLAUDE ACCOUNT` column on `browserctrl list` — which Claude account each browser's extension is signed in to, read from the store's `accountUuid`. This is what decides reachability: `list_connected_browsers` only reports browsers on the session's own account, so a browser on another account can be open, connected and still rejected by `select_browser`. The account Claude Code is signed in as is shown as its email (from its own config); any other is shown as `other account`, numbered only when there are several, because Claude stores no email for them anywhere on disk — verified against the extension store, claude.ai's site data, Claude Code's config and backups, and past transcripts. Labels are computed over the whole scan, so a filter such as `--running` never renames an account.
- `--account-alias <uuid>=<label>` on `list` (repeatable) to name the accounts that cannot be named from disk. A pair without `=` is a hard error, not a silently ignored argument.
- `browser.Entry.AccountUUID` / `.OrgUUID` (JSON `accountUuid` / `orgUuid`), `browser.ClaudeAccount` + `browser.ReadClaudeAccount` + `browser.DefaultClaudeConfigPath` + `browser.ErrNoClaudeAccount`, and `browsertest.KeyAccountUUID` / `KeyTokenOrg`. The org uuid comes from `tokenOrg`, not `lastActiveOrgHint`, which goes stale after an account switch (seen disagreeing on a real profile).

### Changed
- An "every command at a glance" index at the top of the command reference: every verb, every flag and every exit code in two tables.
- Documented that `running` does not mean selectable: a device id resolves only when the extension is installed, the account matches the session's, and the extension's bridge is live at that moment — and nothing on disk records the third, so a retry is sometimes the whole fix. New README FAQ entry, a "Claude accounts" section in the command reference, a recipe for grouping browsers by account, and a section in the agent skill for when `select_browser` rejects a correct id.

### Deprecated
### Removed
### Fixed
### Security

## [0.1.1] - 2026-09-16

### Added
- `browserctrl list --profiles` — also list profiles that have **no** Claude extension installed, with an `EXTENSION` column (`installed` / `not installed`). A profile without the extension has no device id, is invisible to `list_connected_browsers` and `select_browser`, and was previously omitted from `list` with no explanation; this flag makes "my profile is open but missing" answerable. Running/idle for such a profile is read from the profile's own localStorage LevelDB `LOCK`, so its state is as real as an installed profile's.
- `browser.Options.IncludeAllProfiles` and `browser.Entry.Installed` (JSON `installed`) backing that flag, plus `browsertest.ProfileLockPath` for simulating a running profile that has no extension store. `browsertest.Build` now creates each profile's localStorage LevelDB, as Chromium does.

### Changed
- Documented throughout that the device id is minted by the Claude extension per profile and exists nowhere else: new README FAQ entry for a profile missing from `list`, a `list --profiles` section in the command reference, a recipe telling the three empty-id cases apart, and a "when the user says a browser is missing" section in the agent skill.
- `list --help` gained a long description spelling out the same precondition.
- The extension prerequisite is now stated at the top of the README, the docs index and `llms.txt` instead of only in the FAQ: the device id is minted by the Claude in Chrome extension per profile, so a profile without it has no id and is listed by nothing — `browserctrl` or the MCP.

### Deprecated
### Removed
### Fixed
### Security

## [0.1.0] - 2026-09-12

### Added
- `browserctrl list` — every Chromium profile with the Claude extension, with device id, running/idle state, sticky MCP-connected flag, browser kind, profile directory, profile name, signed-in email and display name; `--json`, `--running`, `--all`, repeatable `--root`.
- `browserctrl find <term>...` — resolve free text to exactly one device id; exit `1` on no match, `2` on ambiguity with candidates on stderr; prefers the single running match.
- Go package `github.com/khanakia/browserctrl/browser` (`Scan`, `Match`, `OnlyRunning`, `ReadProfiles`, `ReadExtensionStore`, `DefaultRoots`, typed `Kind` / `ExtensionID` / `RunState` constants) and `browser/browsertest` (`Build`, `BuildStore`, `Root`, `WriteStore`, `LockPath`) for browser-free tests.
- Well-known user-data roots for Chrome, Chrome Beta/Canary/Dev, Chromium, Brave, Edge, Arc, Vivaldi, Opera and Opera GX on macOS, Linux and Windows (macOS verified live; Linux/Windows compile-and-shape tested).
- Running/idle detection via a shared non-blocking `flock` on the extension LevelDB `LOCK`; `unknown` on Windows.
- Markdown lint in `docs_test.go`: relative links, GitHub anchor slugs, no `---` rules, no hard-wrapped prose.
- `internal/docfixture` + `task docs:capture` so every terminal block in the docs is real captured output.
- `browserctrl skills` (voltkit/skillcmd): `list`, `get`, `path`, `check`, `version`, `refresh` over the published `skills/browserctrl-core/SKILL.md`, always matched to the binary's version; dev builds serve the checkout's `skills/` directory.
- Release pipeline via volt: `package main` at the repo root so one bare `vX.Y.Z` stream serves `go install`, archives, checksums, the skills bundle, `install.sh` / `install.ps1` and the Homebrew formula (`khanakia/homebrew-tap`); `.volt.yml` at the root, `internal/.volt.yml` never released; manual-dispatch `release.yml` and `ci.yml`; `task volt:ci` / `volt:doctor` / `volt:status` / `volt:gen` / `volt:release:snapshot` / `volt:release`.
- `--version` backed by `ubgo/buildinfo`: volt's ldflags stamp, else the `go install` module version, else Go's pseudo-version with short commit and dirty marker.

### Fixed
- Concurrent scans in one process reported idle profiles as running because the probe took an exclusive lock; the probe now takes a shared lock. Caught by `go test -race` with parallel subtests.

<!--
Release process:
  1. Move the relevant [Unreleased] entries under a new version heading below.
  2. Date it: ## [1.2.0] - YYYY-MM-DD
  3. Tag the release (e.g. v1.2.0) and update the link refs at the bottom.
-->

[Unreleased]: https://github.com/khanakia/browserctrl/compare/v0.1.2...main
[0.1.2]: https://github.com/khanakia/browserctrl/releases/tag/v0.1.2
[0.1.1]: https://github.com/khanakia/browserctrl/releases/tag/v0.1.1
[0.1.0]: https://github.com/khanakia/browserctrl/releases/tag/v0.1.0

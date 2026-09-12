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

[Unreleased]: https://github.com/khanakia/browserctrl/compare/v0.1.0...main
[0.1.0]: https://github.com/khanakia/browserctrl/releases/tag/v0.1.0

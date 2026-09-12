# browserctrl documentation

`browserctrl` maps Claude-in-Chrome device ids to the Chromium browser, profile, signed-in email and display name behind them, says which profiles are open right now, and resolves a nickname to an id. It is a CLI and a Go library (`github.com/khanakia/browserctrl/browser`); the [repository README](../README.md) is the landing page.

## Install

```sh
go install github.com/khanakia/browserctrl/cmd/browserctrl@latest
```

Or from a clone: `task build` produces `bin/browserctrl`, `task install` puts it on `$GOPATH/bin`.

## Guides

| Guide | Covers |
|---|---|
| [Command reference](commands.md) | **every verb** — `list` · `find` · `completion` · `--version` — every flag, real output, exit codes, the JSON shape |
| [Recipes](recipes.md) | Claude Code integration end to end, shell and jq workflows, custom `--user-data-dir` roots, naming browsers, using the fixture package in your own tests |
| [Go API](go-api.md) | importing `browser` and `browser/browsertest` — `Scan`, `Match`, `OnlyRunning`, `ReadProfiles`, `ReadExtensionStore`, `DefaultRoots`, fixtures |

## Design promises (hold everywhere)

- **Read-only.** The tool never opens a live browser store. Each extension LevelDB is copied to a temp dir, opened read-only, and the copy is deleted before the command returns. `Local State` is read, never written.
- **Never a wrong "idle".** The running probe answers `running`, `idle`, or `unknown`; on a platform or file where it cannot tell (Windows, or an unreadable `LOCK`) it says `unknown`, and `--running` / `OnlyRunning` exclude `unknown` rather than guess.
- **One bad profile never hides the others.** A store that cannot be read (for example a snapshot taken mid-compaction) is reported inline in its own row with an `error` field; the scan continues.
- **Stable machine interface.** `--json` emits the same `Entry` objects on every verb; exit codes are `0` success, `1` failure or no match, `2` ambiguous match (candidates on stderr).
- **No network, no flags, no daemon.** Nothing leaves the machine and the browser is never relaunched with `--remote-debugging-port`.

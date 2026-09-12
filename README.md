<h1 align="center">browserctrl</h1>
<p align="center"><strong>CLI + Go library — which browser is that device id? Answered from disk, in one command.</strong></p>
<p align="center">Maps Claude-in-Chrome device ids to the Chromium browser, profile and signed-in email behind them, shows which profiles are open right now, and resolves a nickname to an id.</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-Apache--2.0-2ea44f" alt="License: Apache-2.0"></a>
  <img src="https://img.shields.io/badge/go-1.26%2B-00ADD8" alt="Go 1.26 or later">
  <img src="https://img.shields.io/badge/platforms-macOS%20%C2%B7%20Linux%20%C2%B7%20Windows-555" alt="Runs on macOS, Linux and Windows (macOS verified live)">
  <img src="https://img.shields.io/badge/read--only-no%20network%2C%20no%20browser%20flags-2ea44f" alt="Read-only: no network, no browser flags">
  <img src="https://img.shields.io/badge/coverage-97.2%25%20lib%20%C2%B7%2097.3%25%20cli%20%C2%B7%2096.2%25%20merged-2ea44f" alt="Statement coverage: 97.2% browser package, 97.3% CLI, 96.2% merged across every package">
</p>

`browserctrl` is an open-source, local-first browser identity resolver for AI coding agents: it tells you which Chromium browser profile is behind each Claude-in-Chrome **device id** — running or not — so Claude Code's `claude-in-chrome` MCP can `select_browser` the right window first time instead of prompting in every browser. It reads Chrome, Edge, Brave, Vivaldi, Opera, Arc and Chromium profile stores read-only, needs no `--remote-debugging-port`, sends nothing anywhere, and ships as a CLI (`browserctrl`) plus an importable Go package (`github.com/khanakia/browserctrl/browser`) with a fixture package for tests.

```sh
go install github.com/khanakia/browserctrl/cmd/browserctrl@latest
```

```go
import "github.com/khanakia/browserctrl/browser"
```

**Two ways in:**

| | You want | Start here |
|---|---|---|
| 🖥️ **`browserctrl` CLI** | see every profile with the Claude extension, which are open right now, and get the id for "my work chrome" without clicking through prompts | [Usage](#usage) → [command reference](docs/commands.md) → [recipes](docs/recipes.md) |
| 📦 **Go library** | scan Chromium user-data roots from your own tool, match entries, build test fixtures | [Library use](#library-use) → [Go API](docs/go-api.md) |

**Contents:** [Why browserctrl?](#why-browserctrl) · [Install](#install) · [Usage](#usage) · [Using it from Claude Code](#using-it-from-claude-code) · [Agent skill](#agent-skill) · [How it works](#how-it-works) · [Library use](#library-use) · [Testing](#testing) · [Development](#development) · [Docs](#docs) · [FAQ](#faq)

## Why browserctrl?

With more than one Chromium browser or profile open, the Claude Code `claude-in-chrome` MCP can only report `list_connected_browsers` like this:

```json
[
  { "deviceId": "ce9a8e06-61d3-4e7b-894b-13fd92987212", "name": "Browser 1" },
  { "deviceId": "1056c3fd-8eab-42e9-a9fd-7df3b88394c6", "name": "Browser 2" },
  { "deviceId": "30c7e2d1-b0da-4e21-bed8-27c8cc755346", "name": "Browser 3" }
]
```

The names are not stable between listings, so the only way to pick the right window was `switch_browser`: a confirmation prompt fires in **every** connected extension and you click the one you meant. Every session. `browserctrl` reads the answer from disk instead:

```
$ browserctrl list
STATE    MCP  DEVICE ID                             BROWSER  PROFILE     NAME     EMAIL              DISPLAY NAME
running  yes  ce9a8e06-61d3-4e7b-894b-13fd92987212  custom   Default     Aman     aman@example.com   chrome-main
running  yes  2aa533d3-d03f-4dd8-b664-f59b8930eccc  custom   Profile 5   Work     aman@work.example  work-chrome
idle     yes  f836694e-b2f0-4e5b-93e4-ff946c6183ad  custom   Profile 23  legable  aman@legable.co    aman-legable
idle     no   -                                     custom   Profile 4   Fresh    fresh@example.com  -

$ browserctrl find legable
f836694e-b2f0-4e5b-93e4-ff946c6183ad
```

Output captured against the repo's doc fixture (`task docs:capture`), which is why the browser column reads `custom`; on a real machine it says `chrome`, `vivaldi`, `edge`, and so on.

How the jobs compare with the other ways of answering "which browser is that id?":

| | **browserctrl** | MCP `switch_browser` prompt | By hand (extension popup / `chrome://version`) | DevTools protocol tools |
|---|---|---|---|---|
| Map a device id to browser + profile + email | ✅ one command, from disk | ❌ click in each window | ⚠️ open every profile, read it off | ❌ extension storage is not exposed |
| Tell which profiles are open right now | ✅ `STATE` column, via the LevelDB lock | ⚠️ connected ones only, unstable labels | ❌ | ⚠️ only with `--remote-debugging-port` |
| Include profiles whose browser is closed | ✅ `idle` rows | ❌ | ❌ | ❌ |
| Scriptable (`--json`, exit codes, nickname → id) | ✅ | ❌ tool call + human click | ❌ | ✅ |
| Works without relaunching the browser with flags | ✅ read-only, no flags | ✅ | ✅ | ❌ |
| Chrome, Edge, Brave, Vivaldi, Opera, Arc, Chromium in one list | ✅ | ✅ whatever is connected | ⚠️ one at a time | ⚠️ per instance |

If you want to drive the page itself, the MCP already does that; if you want to know **which window the id is** and hand the agent the right one first time, that is `browserctrl`.

## Install

```sh
go install github.com/khanakia/browserctrl/cmd/browserctrl@latest
```

Or from a checkout: `task install` (binary into `$GOPATH/bin`) or `task build` (`./bin/browserctrl`). Requires Go 1.26 or later; no runtime dependencies, no browser flags, no network.

Releases are cut by [volt](https://github.com/khanakia/voltkit): each `browserctrl/vX.Y.Z` tag ships cross-compiled archives with a `checksums.txt`, a Homebrew formula in `khanakia/homebrew-tap` (`brew install khanakia/tap/browserctrl` once the first release exists), and the skills bundle the [agent skill](#agent-skill) section describes. `browserctrl --version` prints the stamped version and commit; a source build prints Go's pseudo-version instead so you can still tell which commit you are running.

## Usage

- `browserctrl list` — every profile with the extension, running ones first. `--json` for machine output, `--running` to keep only open profiles, `--all` to also scan the Claude desktop-app extension ids, `--root <dir>` (repeatable) to scan a custom `--user-data-dir` instead of the well-known install locations.
- `browserctrl find <term>...` — prints exactly one device id. Every term must match (case-insensitive substring) one of display name, profile name, email, profile dir, browser kind, device id. If several match but exactly one is running, that one wins. Exit 1 = no match, exit 2 = ambiguous (candidates on stderr).
- `browserctrl skills` — list, print and freshness-check the agent skill this binary ships (`list`, `get`, `path`, `check`, `version`, `refresh`).
- `browserctrl --version`, `browserctrl completion <shell>`.

Every flag, with real output for every verb, is in the [command reference](docs/commands.md); end-to-end workflows are in [recipes](docs/recipes.md).

## Using it from Claude Code

Add this to your global `CLAUDE.md` so the agent never asks you to click through the confirmation prompt again:

```
# Chrome browser selection
Before any claude-in-chrome action, run `browserctrl list --running --json` and pick the deviceId whose profileName / email / displayName matches what I asked for (e.g. "my legable chrome" → the entry with email aman@legable.co). Call select_browser with that id directly. Only fall back to switch_browser if the list is empty or the match is ambiguous.
```

The [recipes page](docs/recipes.md#claude-code-pick-the-browser-without-a-prompt) has the longer version, including how to give each browser a memorable display name.

## Agent skill

The rule above also ships as a [SKILL.md](skills/browserctrl-core/SKILL.md) — the open agent-skill format — so any harness can install it instead of you pasting it into `CLAUDE.md`:

```sh
npx skills add khanakia/browserctrl        # install into your agent (skills.sh)
browserctrl skills                          # what this binary ships
browserctrl skills check <installed-dir>    # exit 0 current, exit 1 stale
```

```
$ browserctrl skills
  browserctrl-core  Pick the right Chrome/Chromium browser for the claude-in-chrome MCP without a click-through prompt.

install for agents:  npx skills add khanakia/browserctrl
```

The skill is served by the binary itself (`skills get browserctrl-core`) and is always the version that matches the installed binary: release builds fetch `skills_<version>.tar.gz` from their own release once and cache it, source builds serve the repo's `skills/` directory. That is [voltkit/skillcmd](https://github.com/khanakia/voltkit/tree/main/skillcmd); the wiring in `cmd/browserctrl/skills_gen.go` is generated by `volt gen skills`.

## How it works

Each Chromium profile keeps the extension's `chrome.storage.local` in `<profile>/Local Extension Settings/<extension-id>/` as a LevelDB. The Claude extension writes `bridgeDeviceId` (the id the MCP reports), `bridgeDisplayName` (the name you typed when connecting) and `mcpConnected` there. `Local State` at the browser's user-data root maps profile directories to names and signed-in emails. A running browser holds an `flock` on the LevelDB `LOCK` file, which is the "running" signal. The LevelDB is snapshotted to a temp dir and opened read-only, so the tool never touches the live store.

Verified on macOS against Chrome, Vivaldi, Opera and Edge. The Linux and Windows path tables come from the browsers' documented defaults and are only covered by the cross-compile gate (`task cross`); the running/idle probe returns `unknown` on Windows.

## Library use

The `browser` package is importable on its own: `browser.Scan(ctx, browser.Options{})` returns `[]browser.Entry`; `browser.Match` and `browser.OnlyRunning` do the filtering; `browser/browsertest` builds fake user-data roots for your own tests.

```go
entries, err := browser.Scan(ctx, browser.Options{})
if err != nil {
	return err
}
for _, e := range browser.OnlyRunning(browser.Match(entries, "legable")) {
	fmt.Println(e.DeviceID, e.Browser, e.ProfileName, e.Email)
}
```

Full surface, with the contracts each function keeps, is in the [Go API page](docs/go-api.md).

## Testing

| Package | Statement coverage (own tests) |
|---|---|
| `browser` (library) | 97.2% |
| `browser/browsertest` (fixtures) | 89.1% |
| `cmd/browserctrl` (CLI) | 97.3% |
| `internal/docfixture` (doc harness) | 97.3% |
| **merged, every package** | **96.2% (358/372 statements)** |

Re-measure with `task cover`; `task test:uncovered` lists what is left per function. The numbers above are the measured ones, not rounded claims. The suite needs no browser: every scenario runs against fake user-data roots built by `browser/browsertest`, "running" is simulated by holding the same `flock` Chromium holds, and `go test -race` with parallel subtests is what caught the exclusive-lock probe bug.

The uncovered remainder is **14 statements**, each named rather than hand-waved:

- **Two `os.Exit` wrappers** — `main()` in `cmd/browserctrl` and in `internal/docfixture`. Both delegate to a `run()` that is tested end to end; the wrapper itself cannot run inside `go test`.
- **Six deferred-`Close` / cleanup error arms** — `db.Close`, `in.Close`, `out.Close` and `os.RemoveAll` in the store reader, plus `db.Close` in the fixture builder. They surface a flush or unlink failure instead of dropping it; provoking one needs fault injection below `os`, which the suite deliberately does not do.
- **Two tabwriter row-write arms** in the table renderer. `text/tabwriter` buffers any line containing a tab until `Flush`, so a broken stdout is reported by `Flush` (covered), never by the per-row write.
- **One `flock` default arm** in the running probe: an error other than `EWOULDBLOCK`, e.g. a filesystem that does not support `flock`. It maps to `unknown`, never to `idle`.
- **Three fixture-builder defensive arms**: `json.Marshal` of a `map[string]string` (cannot fail), `db.Put` on a freshly opened LevelDB (no error source without disk faults), and re-creating `LOCK` when LevelDB did not (it always does; the arm guards a future implementation change).

## Development

`task check` is the gate: gofmt, vet, staticcheck + golangci-lint, `go test -race` (which includes the markdown lint in `docs_test.go`), cross-compile for linux/windows/darwin, and a `go mod tidy` diff check. `task volt:ci` runs volt's equivalent gate plus its skills-frontmatter lint. `task docs:capture -- <args>` regenerates any terminal output shown in the docs against the fixture in `internal/docfixture`.

Releases: `task volt:release:snapshot` builds every platform into `dist/` and publishes nothing; `task volt:release:cli -- --bump patch` tags `browserctrl/vX.Y.Z` and publishes archives, checksums, the skills bundle and (with `HOMEBREW_TAP_GITHUB_TOKEN`) the brew formula; `task volt:release:lib -- --bump patch` tags the Go module bare `vX.Y.Z`, which is what `go install …@latest` resolves. Configuration lives in `cmd/browserctrl/.volt.yml`. Nothing releases on push: `.github/workflows/release.yml` is manual-dispatch by volt's design, and `ci.yml` is manual-dispatch here because the repository is private.

## Docs

| Page | Covers |
|---|---|
| [Command reference](docs/commands.md) | every verb and flag, real output, exit codes, JSON shape |
| [Recipes](docs/recipes.md) | Claude Code integration, shell + jq workflows, custom user-data dirs, testing with the fixture package |
| [Go API](docs/go-api.md) | `browser` and `browser/browsertest` — types, functions, guarantees |
| [Docs index](docs/README.md) | all of the above plus the design promises |

## FAQ

**Does browserctrl send anything to the cloud or need an API key?** No. It reads files under your browsers' user-data directories and prints what it finds. There is no network code in the binary, no account, no telemetry. The only third-party dependency is a LevelDB reader.

**Does it modify my Chrome profile, or lock it?** No. Each extension store is copied to a temp directory, opened read-only from the copy, and the copy is deleted before exit. The running/idle probe takes a *shared* non-blocking `flock` on the `LOCK` file and releases it immediately; it cannot block the browser or another scan.

**Why does Claude Code show "Browser 1 / 2 / 3", and how does this fix it?** The `claude-in-chrome` MCP only knows each connected extension's device id and assigns display labels per listing, so they shift. `browserctrl` joins the same device id (`bridgeDeviceId` in the extension's storage) with the profile's name and email from Chromium's `Local State`, so the agent can pick by "my work chrome" and call `select_browser` directly instead of `switch_browser`'s click-in-every-window prompt.

**Does it work when the browser is closed?** Yes. Idle profiles are listed with their ids too (`STATE idle`), which is how you see everything the extension is installed in. Only a `running` profile can actually be selected by the MCP, so pass `--running` when the id is going straight into `select_browser`.

**Which browsers and OSes are supported?** Any Chromium-based browser whose profile layout is standard: Chrome (stable/beta/canary/dev), Chromium, Brave, Edge, Arc, Vivaldi, Opera and Opera GX are scanned by default, and `--root` handles anything launched with `--user-data-dir`. macOS is verified against real installs; the Linux and Windows path tables are compiled and shape-tested in the gate but have not been run on a live machine, and the running probe returns `unknown` on Windows rather than guess.

**How is this different from just using `switch_browser`?** `switch_browser` asks a human to click Connect in the right window, every time, and cannot tell you which window is which. `browserctrl` answers from disk in milliseconds, works for closed profiles, is scriptable (`--json`, exit codes), and lets you resolve a nickname to an id with `find`. Keep `switch_browser` as the fallback for the ambiguous case.

**What does `mcpConnected` mean if a profile is idle?** It is a sticky flag the extension sets after its first successful MCP handshake and does not clear on exit. `idle` + `MCP yes` means "has connected before, not open now"; `MCP no` with an empty device id means the extension is installed but has never connected.

**Can I use it from Go, and test without a browser?** Yes. `browser.Scan`, `browser.Match` and `browser.OnlyRunning` are the whole surface the CLI uses, and `browser/browsertest` writes fake user-data roots with real LevelDB stores so your tests need no browser. See the [Go API page](docs/go-api.md).

<sub>browserctrl — open-source, local-first CLI and Go library that resolves Claude-in-Chrome / Claude Code MCP device ids to Chromium browser profiles (Chrome, Edge, Brave, Vivaldi, Opera, Arc, Chromium) by reading extension LevelDB storage and `Local State` read-only. No cloud, no API key, no browser flags. Apache-2.0.</sub>

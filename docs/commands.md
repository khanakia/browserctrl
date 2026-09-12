# Command reference

Every verb, every flag, with output captured by `task docs:capture` against the fixture in `internal/docfixture` (four profiles, two of them held open the way a running browser holds them). That is why the `BROWSER` column and `browser` field read `custom` here: the fixture is passed with `--root`, and roots given explicitly are labelled `custom`. On a real machine they read `chrome`, `chrome-beta`, `chrome-canary`, `chrome-dev`, `chromium`, `brave`, `edge`, `arc`, `vivaldi`, `opera`, or `opera-gx`.

**Contents:** [Global flags](#global-flags) · [Shared flags](#shared-flags) · [list](#list) · [find](#find) · [completion](#completion) · [Exit codes](#exit-codes) · [JSON shape](#json-shape) · [Table columns](#table-columns)

## Global flags

```
$ browserctrl --help
List Claude-connected Chromium browser profiles and their device ids

Usage:
  browserctrl [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  find        Print the device id of the single profile matching all terms
  help        Help about any command
  list        Show every profile with the Claude extension, running ones first

Flags:
  -h, --help      help for browserctrl
  -v, --version   version for browserctrl

Use "browserctrl [command] --help" for more information about a command.
```

```
$ browserctrl --version
browserctrl version dev
```

`dev` is what a source build prints. A `go install …@vX.Y.Z` build prints that module version; a release binary prints whatever was stamped with `-ldflags "-X main.version=…"`.

## Shared flags

`list` and `find` take the same four flags, so a query behaves identically whichever verb runs it.

| Flag | Effect |
|---|---|
| `--json` | Emit JSON instead of the table. `list` prints an array of entries, `find` prints the single matched entry. |
| `--running` | Keep only profiles whose state is `running`. `unknown` is excluded on purpose (see [Table columns](#table-columns)). |
| `--all` | Also scan the two extension ids allowed by the Claude desktop app's native-host manifest, not just the Claude Code extension. Entries carry the `extension` field so they can be told apart. |
| `--root <dir>` | Scan this Chromium user-data directory **instead of** the well-known install locations. Repeatable. Entries are labelled `browser: custom`. Use it for `--user-data-dir` profiles such as Chrome for Testing or a Playwright persistent context. |

## list

Show every profile that has the Claude extension installed, running ones first, then by browser kind, then by profile directory.

```
$ browserctrl list --help
Show every profile with the Claude extension, running ones first

Usage:
  browserctrl list [flags]

Examples:
  browserctrl list
  browserctrl list --running --json

Flags:
      --all                scan the Claude desktop-app extension ids too, not just Claude Code's
  -h, --help               help for list
      --json               emit JSON instead of a table
      --root stringArray   scan this Chromium user-data dir instead of the well-known ones (repeatable)
      --running            only profiles currently open in a running browser
```

```
$ browserctrl list
STATE    MCP  DEVICE ID                             BROWSER  PROFILE     NAME     EMAIL              DISPLAY NAME
running  yes  ce9a8e06-61d3-4e7b-894b-13fd92987212  custom   Default     Aman     aman@example.com   chrome-main
running  yes  2aa533d3-d03f-4dd8-b664-f59b8930eccc  custom   Profile 5   Work     aman@work.example  work-chrome
idle     yes  f836694e-b2f0-4e5b-93e4-ff946c6183ad  custom   Profile 23  legable  aman@legable.co    aman-legable
idle     no   -                                     custom   Profile 4   Fresh    fresh@example.com  -
```

The last row is a profile where the extension is installed but has never connected: no device id yet, `MCP no`. It is listed so you can see the extension is there; `find` will never pick it.

```
$ browserctrl list --running
STATE    MCP  DEVICE ID                             BROWSER  PROFILE    NAME  EMAIL              DISPLAY NAME
running  yes  ce9a8e06-61d3-4e7b-894b-13fd92987212  custom   Default    Aman  aman@example.com   chrome-main
running  yes  2aa533d3-d03f-4dd8-b664-f59b8930eccc  custom   Profile 5  Work  aman@work.example  work-chrome
```

```
$ browserctrl list --running --json
[
  {
    "deviceId": "ce9a8e06-61d3-4e7b-894b-13fd92987212",
    "displayName": "chrome-main",
    "state": "running",
    "mcpConnected": true,
    "browser": "custom",
    "browserPath": ".docs-fixture",
    "profileDir": "Default",
    "profileName": "Aman",
    "email": "aman@example.com",
    "extension": "fcoeoabgfenejglbffodgkkbkcdhcgfn"
  },
  {
    "deviceId": "2aa533d3-d03f-4dd8-b664-f59b8930eccc",
    "displayName": "work-chrome",
    "state": "running",
    "mcpConnected": true,
    "browser": "custom",
    "browserPath": ".docs-fixture",
    "profileDir": "Profile 5",
    "profileName": "Work",
    "email": "aman@work.example",
    "extension": "fcoeoabgfenejglbffodgkkbkcdhcgfn"
  }
]
```

When nothing matches (no extension anywhere, or `--running` on a machine with every browser closed) the table form prints one hint line and exits 0:

```
no Claude extension stores found (is the extension installed in any profile?)
```

The JSON form prints `null` in that case (an empty Go slice), so scripts should treat `null` and `[]` alike.

## find

Resolve free text to exactly one device id and print it, nothing else, so it can be captured: `id=$(browserctrl find legable)`.

```
$ browserctrl find --help
Every term must match (case-insensitive substring) one of: display name,
profile name, email, profile dir, browser kind, device id. When several
profiles match but exactly one is running, that one wins. Otherwise the
candidates are listed on stderr and the exit code is 2.

Usage:
  browserctrl find <term>... [flags]

Examples:
  browserctrl find legable
  browserctrl find vivaldi work
  DEVICE=$(browserctrl find analyzify)

Flags:
      --all                scan the Claude desktop-app extension ids too, not just Claude Code's
  -h, --help               help for find
      --json               emit JSON instead of a table
      --root stringArray   scan this Chromium user-data dir instead of the well-known ones (repeatable)
      --running            only profiles currently open in a running browser
```

Matching rules, in order:

1. The query is split on whitespace. **Every** term must appear, case-insensitively, somewhere in the entry's display name, profile name, email, profile directory, browser kind, or device id. Terms may hit different fields (`work chrome` → display name `work-chrome`, or profile name `Work` + browser `chrome`).
2. Entries with no device id are dropped: an empty id is useless to `select_browser`.
3. One entry left → print its id, exit 0.
4. Several left, exactly one of them `running` → print that one, exit 0. The window that is open is almost always the one meant.
5. Otherwise exit 2 and list the candidates on stderr.

```
$ browserctrl find legable
f836694e-b2f0-4e5b-93e4-ff946c6183ad
```

```
$ browserctrl find work
2aa533d3-d03f-4dd8-b664-f59b8930eccc
```

Rule 4 in action: `example.com` matches the `Default` profile (email `aman@example.com`) and `Profile 4` (`fresh@example.com`), but `Profile 4` has no device id, so the match is unique:

```
$ browserctrl find example.com
ce9a8e06-61d3-4e7b-894b-13fd92987212
```

Ambiguous (both `chrome-main` and `work-chrome` contain `chrome` and both are running), exit 2:

```
$ browserctrl find chrome
error: query matches more than one browser
STATE    MCP  DEVICE ID                             BROWSER  PROFILE    NAME  EMAIL              DISPLAY NAME
running  yes  ce9a8e06-61d3-4e7b-894b-13fd92987212  custom   Default    Aman  aman@example.com   chrome-main
running  yes  2aa533d3-d03f-4dd8-b664-f59b8930eccc  custom   Profile 5  Work  aman@work.example  work-chrome
```

Everything above goes to stderr; stdout stays empty so `$(…)` captures nothing rather than garbage. Add a term to disambiguate: `browserctrl find chrome main`.

No match, exit 1:

```
$ browserctrl find firefox
error: no browser matches the query
```

With `--json` the whole entry is printed instead of the bare id:

```
$ browserctrl find legable --json
{
  "deviceId": "f836694e-b2f0-4e5b-93e4-ff946c6183ad",
  "displayName": "aman-legable",
  "state": "idle",
  "mcpConnected": true,
  "browser": "custom",
  "browserPath": ".docs-fixture",
  "profileDir": "Profile 23",
  "profileName": "legable",
  "email": "aman@legable.co",
  "extension": "fcoeoabgfenejglbffodgkkbkcdhcgfn"
}
```

Note the entry is `idle`: `find` does not require a running browser unless you pass `--running`. Pass it when the id is going straight into `select_browser`, which can only connect to an open window.

## completion

Standard cobra shell completion. `browserctrl completion bash|zsh|fish|powershell` prints the script; `browserctrl completion --help` shows the per-shell install lines.

## Exit codes

| Code | Meaning | Emitted by |
|---|---|---|
| `0` | success (including an empty `list`) | all verbs |
| `1` | failure: scan error, unreadable `Local State`, usage error, or `find` with no match | all verbs |
| `2` | `find` matched more than one entry with a device id and could not tie-break on `running`; candidates are on stderr | `find` |

`1` and `2` are distinct so a script can decide to ask the user on `2` rather than treat it as a hard failure.

## JSON shape

Both verbs emit the same object; `list` wraps it in an array, `find` prints one. Every field is always present except `error`, which appears only when that store could not be read.

| Field | Type | Meaning |
|---|---|---|
| `deviceId` | string | `bridgeDeviceId` from the extension store — the value `select_browser` takes. Empty when the extension has never connected. |
| `displayName` | string | `bridgeDisplayName` — the name typed in the extension's "name this browser" prompt. Empty if never set. |
| `state` | `"running"` \| `"idle"` \| `"unknown"` | Whether the profile is open right now, from the LevelDB `LOCK`. `unknown` on Windows or when the lock file could not be opened. |
| `mcpConnected` | bool | The extension has completed an MCP handshake at some point. Sticky: it is not cleared when the browser exits, so combine with `state` for "connected now". |
| `browser` | string | Browser family (`chrome`, `edge`, `vivaldi`, …) or `custom` for `--root` dirs. |
| `browserPath` | string | The user-data root that was scanned. |
| `profileDir` | string | Profile directory name inside the root (`Default`, `Profile 5`). The stable key. |
| `profileName` | string | Profile label from `Local State`. Empty for browsers that do not populate it (Opera). |
| `email` | string | Signed-in account from `Local State`, empty if signed out. |
| `extension` | string | Which extension id this store belongs to. Only differs from the Claude Code id under `--all`. |
| `error` | string, optional | Why this store could not be read. `deviceId` and `displayName` are empty when set. |

## Table columns

`STATE` · `MCP` (`yes`/`no` = `mcpConnected`) · `DEVICE ID` · `BROWSER` · `PROFILE` (directory) · `NAME` · `EMAIL` · `DISPLAY NAME`. Empty cells show `-` so columns stay aligned and a missing value cannot be mistaken for a rendering glitch. A row whose store failed to read gets an extra trailing cell `ERROR: <reason>`.

Sort order is `running` → `idle` → `unknown`, then browser kind alphabetically, then profile directory. Running entries come first because they are the ones an agent can connect to.

# Command reference

Every verb, every flag, with output captured by `task docs:capture` against the fixture in `internal/docfixture` (four profiles, two of them held open the way a running browser holds them). That is why the `BROWSER` column and `browser` field read `custom` here: the fixture is passed with `--root`, and roots given explicitly are labelled `custom`. On a real machine they read `chrome`, `chrome-beta`, `chrome-canary`, `chrome-dev`, `chromium`, `brave`, `edge`, `arc`, `vivaldi`, `opera`, or `opera-gx`.

**Contents:** [Every command at a glance](#every-command-at-a-glance) · [Global flags](#global-flags) · [Shared flags](#shared-flags) · [list](#list) · [list --profiles](#list---profiles) · [Claude accounts](#claude-accounts) · [find](#find) · [skills](#skills) · [completion](#completion) · [Exit codes](#exit-codes) · [JSON shape](#json-shape) · [Table columns](#table-columns)

## Every command at a glance

| Command | What it does | Exits |
|---|---|---|
| `browserctrl list` | one row per profile with the Claude extension: device id, running/idle, MCP flag, browser, profile, name, email, display name, [Claude account](#claude-accounts) | `0` |
| `browserctrl list --profiles` | the same plus profiles with **no** extension, marked `not installed` — see [list --profiles](#list---profiles) | `0` |
| `browserctrl find <term>...` | the device id of the single profile matching every term | `0` hit · `1` none · `2` ambiguous |
| `browserctrl skills <list, get, path, check, version, refresh>` | the agent skill this binary ships — see [skills](#skills) | `0` · `1` stale/error |
| `browserctrl completion <bash, zsh, fish, powershell>` | shell completion script — see [completion](#completion) | `0` |
| `browserctrl --version` | version, commit and dirty marker — see [Global flags](#global-flags) | `0` |
| `browserctrl help [command]` | cobra's built-in help | `0` |

| Flag | On | Effect |
|---|---|---|
| `--json` | `list`, `find` | JSON instead of the table: an array for `list`, one object for `find` — see [JSON shape](#json-shape) |
| `--running` | `list`, `find` | only profiles open right now; `unknown` is excluded deliberately |
| `--all` | `list`, `find` | also scan the two Claude desktop-app extension ids |
| `--root <dir>` | `list`, `find` | scan this user-data dir **instead of** the well-known ones; repeatable |
| `--profiles` | `list` | include profiles without the extension |
| `--account-alias <uuid>=<label>` | `list` | name a Claude account in the `CLAUDE ACCOUNT` column; repeatable |

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
  skills      List and retrieve this tool's bundled agent skills

Flags:
  -h, --help      help for browserctrl
  -v, --version   version for browserctrl

Use "browserctrl [command] --help" for more information about a command.
```

`--version` comes from [ubgo/buildinfo](https://github.com/ubgo/buildinfo) and has three shapes depending on how the binary was produced. A `go build` from a checkout (what `task build` does) embeds Go's pseudo-version, the short commit, and `+dirty` when the tree had uncommitted changes:

```
$ browserctrl --version
browserctrl version v0.0.0-20260912061725-90d9f421ae81+dirty (90d9f42)
```

A volt release build is stamped with the tag (`browserctrl version v0.1.0 (90d9f42)`), and `go install …@vX.Y.Z` reports that module version. `go run` embeds no VCS data and prints `browserctrl version dev`.

## Shared flags

`list` and `find` take the same four flags, so a query behaves identically whichever verb runs it. `list` takes one more of its own, `--profiles`.

| Flag | Effect |
|---|---|
| `--json` | Emit JSON instead of the table. `list` prints an array of entries, `find` prints the single matched entry. |
| `--running` | Keep only profiles whose state is `running`. `unknown` is excluded on purpose (see [Table columns](#table-columns)). |
| `--all` | Also scan the two extension ids allowed by the Claude desktop app's native-host manifest, not just the Claude Code extension. Entries carry the `extension` field so they can be told apart. |
| `--root <dir>` | Scan this Chromium user-data directory **instead of** the well-known install locations. Repeatable. Entries are labelled `browser: custom`. Use it for `--user-data-dir` profiles such as Chrome for Testing or a Playwright persistent context. |
| `--profiles` (`list` only) | Also list profiles that have **no** Claude extension installed, so a missing profile is visible instead of silently absent. See [list --profiles](#list---profiles). Not on `find`: such a profile has no device id to resolve. |
| `--account-alias <uuid>=<label>` (`list` only) | Give a Claude account uuid a readable name in the `CLAUDE ACCOUNT` column. Repeatable. See [Claude accounts](#claude-accounts) for why only one account can be named without it. A pair without `=` is an error, not a silently ignored argument. |

## list

Show every profile that has the Claude extension installed, running ones first, then by browser kind, then by profile directory. A profile where the extension is **not** installed has no device id anywhere on disk, so it is omitted entirely — pass [`--profiles`](#list---profiles) to see those too.

```
$ browserctrl list --help
Lists one row per profile that has a Claude extension store on disk, which
is the only place a device id exists. A profile where the extension is not
installed has no id, cannot be selected by the MCP, and is not listed unless
you pass --profiles.

Usage:
  browserctrl list [flags]

Examples:
  browserctrl list
  browserctrl list --running --json
  browserctrl list --profiles
  browserctrl list --account-alias 2f7c1b90-...=work@example.com

Flags:
      --account-alias stringArray   name a Claude account: <account-uuid>=<label> (repeatable)
      --all                         scan the Claude desktop-app extension ids too, not just Claude Code's
  -h, --help                        help for list
      --json                        emit JSON instead of a table
      --profiles                    also list profiles that do NOT have the Claude extension installed
      --root stringArray            scan this Chromium user-data dir instead of the well-known ones (repeatable)
      --running                     only profiles currently open in a running browser
```

```
$ browserctrl list
STATE    MCP  DEVICE ID                             BROWSER  PROFILE     NAME     EMAIL              DISPLAY NAME  CLAUDE ACCOUNT
running  yes  ce9a8e06-61d3-4e7b-894b-13fd92987212  custom   Default     Aman     aman@example.com   chrome-main   other account 1
running  yes  2aa533d3-d03f-4dd8-b664-f59b8930eccc  custom   Profile 5   Work     aman@work.example  work-chrome   other account 1
idle     yes  f836694e-b2f0-4e5b-93e4-ff946c6183ad  custom   Profile 23  legable  aman@legable.co    aman-legable  other account 2
idle     no   -                                     custom   Profile 4   Fresh    fresh@example.com  -             -
```

The last row is a profile where the extension is installed but has never connected: no device id yet, `MCP no`. It is listed so you can see the extension is there; `find` will never pick it.

```
$ browserctrl list --running
STATE    MCP  DEVICE ID                             BROWSER  PROFILE    NAME  EMAIL              DISPLAY NAME  CLAUDE ACCOUNT
running  yes  ce9a8e06-61d3-4e7b-894b-13fd92987212  custom   Default    Aman  aman@example.com   chrome-main   other account 1
running  yes  2aa533d3-d03f-4dd8-b664-f59b8930eccc  custom   Profile 5  Work  aman@work.example  work-chrome   other account 1
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
    "accountUuid": "11111111-1111-4111-8111-111111111111",
    "orgUuid": "1111aaaa-1111-4111-8111-111111111111",
    "extension": "fcoeoabgfenejglbffodgkkbkcdhcgfn",
    "installed": true
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
    "accountUuid": "11111111-1111-4111-8111-111111111111",
    "orgUuid": "1111aaaa-1111-4111-8111-111111111111",
    "extension": "fcoeoabgfenejglbffodgkkbkcdhcgfn",
    "installed": true
  }
]
```

When nothing matches (no extension anywhere, or `--running` on a machine with every browser closed) the table form prints one hint line and exits 0:

```
no Claude extension stores found (is the extension installed in any profile?)
```

The JSON form prints `null` in that case (an empty Go slice), so scripts should treat `null` and `[]` alike.

## list --profiles

Answers "why is my profile not in the list?". Chrome installs extensions **per profile**, and the device id is created by the Claude extension on its first run in that profile. A profile the extension was never installed in therefore has no id, is invisible to `list_connected_browsers` and `select_browser`, and — without this flag — is not printed at all, whether or not it is open right now.

`--profiles` adds one row per such profile and an `EXTENSION` column saying which is which:

```
$ browserctrl list --profiles
STATE    EXTENSION      MCP  DEVICE ID                             BROWSER  PROFILE     NAME      EMAIL                 DISPLAY NAME  CLAUDE ACCOUNT
running  installed      yes  ce9a8e06-61d3-4e7b-894b-13fd92987212  custom   Default     Aman      aman@example.com      chrome-main   other account 1
running  not installed  no   -                                     custom   Profile 26  Personal  personal@example.com  -             -
running  installed      yes  2aa533d3-d03f-4dd8-b664-f59b8930eccc  custom   Profile 5   Work      aman@work.example     work-chrome   other account 1
idle     installed      yes  f836694e-b2f0-4e5b-93e4-ff946c6183ad  custom   Profile 23  legable   aman@legable.co       aman-legable  other account 2
idle     installed      no   -                                     custom   Profile 4   Fresh     fresh@example.com     -             -
```

`Profile 26` is the case to recognise: open right now, a real signed-in profile, and completely unusable from Claude Code until the extension is installed in it. Note the two different empty-id rows — `Profile 26` has no extension, `Profile 4` has one that has never connected — which is why the JSON carries an explicit `installed` field rather than leaving you to infer it from an empty `deviceId`.

The `STATE` of a row without the extension is read from that profile's own localStorage LevelDB lock instead of the extension store's, so running/idle is just as real. A profile directory Chromium has never opened has neither lock and reports `unknown`.

`--profiles` composes with the other flags: `--running --profiles` shows only open profiles including the ones missing the extension, and `--json` adds the same rows to the array.

## Claude accounts

The `CLAUDE ACCOUNT` column answers a question the device id cannot: **which browsers this session can reach at all.** `list_connected_browsers` only reports browsers signed in to the *same Claude account* as the session asking, so a browser on another account is invisible to that session no matter how open it is, and `select_browser` rejects its id with "No connected browser has deviceId …".

It reads in plain words rather than uuids:

| Cell | Meaning |
|---|---|
| an email address | the account Claude Code is signed in as right now — this session can reach these browsers |
| `other account` (numbered `1`, `2`, … when there are several) | a different Claude account; this session cannot reach it, whatever the state column says |
| a label you chose | an account you named with `--account-alias` |
| `-` | the extension is installed but signed out (or, under `--profiles`, not installed at all) |

```
$ browserctrl list --account-alias 22222222-2222-4222-8222-222222222222=work@example.com
STATE    MCP  DEVICE ID                             BROWSER  PROFILE     NAME     EMAIL              DISPLAY NAME  CLAUDE ACCOUNT
running  yes  ce9a8e06-61d3-4e7b-894b-13fd92987212  custom   Default     Aman     aman@example.com   chrome-main   other account
running  yes  2aa533d3-d03f-4dd8-b664-f59b8930eccc  custom   Profile 5   Work     aman@work.example  work-chrome   other account
idle     yes  f836694e-b2f0-4e5b-93e4-ff946c6183ad  custom   Profile 23  legable  aman@legable.co    aman-legable  work@example.com
idle     no   -                                     custom   Profile 4   Fresh    fresh@example.com  -             -
```

**Why only one account gets an email.** Claude stores no email in the browser — only an account uuid. That was checked against the extension's own storage, claude.ai's site data inside the profile (`__qk_hint_account_uuid`, `ccd-sync-owner` and its analytics payloads, which carry `account_uuid` and `organization_uuid` and nothing else), Claude Code's config and every backup of it, and past session transcripts. The one place a uuid is paired with an email is Claude Code's config for the account it is signed in as, which is where the email cell comes from. Naming any other account is therefore something only you can do, with `--account-alias <uuid>=<label>`; take the uuid from `--json` (`accountUuid`) and put the flag in a shell alias.

Labels are computed over the whole scan, not the rows being printed, so `--running` and `--profiles` never rename an account.

**The `EMAIL` column is a different thing.** That is the Google/Microsoft account of the *browser profile*, from `Local State`. The two are unrelated: a profile signed in to Chrome as one person can be signed in to Claude as another, which is exactly the shape that makes a mismatch hard to spot.

So a device id is usable when three things hold, and browserctrl can verify only the first two from disk: the extension is installed in that profile, the account matches the session's, and the extension's bridge is live at that moment. The third is knowable only from `list_connected_browsers`.

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
STATE    MCP  DEVICE ID                             BROWSER  PROFILE    NAME  EMAIL              DISPLAY NAME  CLAUDE ACCOUNT
running  yes  ce9a8e06-61d3-4e7b-894b-13fd92987212  custom   Default    Aman  aman@example.com   chrome-main   other account 1
running  yes  2aa533d3-d03f-4dd8-b664-f59b8930eccc  custom   Profile 5  Work  aman@work.example  work-chrome   other account 1
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
  "accountUuid": "22222222-2222-4222-8222-222222222222",
  "orgUuid": "2222bbbb-2222-4222-8222-222222222222",
  "extension": "fcoeoabgfenejglbffodgkkbkcdhcgfn",
  "installed": true
}
```

Note the entry is `idle`: `find` does not require a running browser unless you pass `--running`. Pass it when the id is going straight into `select_browser`, which can only connect to an open window.

## skills

Serves the agent skill(s) this binary ships, always at the version that matches the binary. Provided by [voltkit/skillcmd](https://github.com/khanakia/voltkit/tree/main/skillcmd); the wiring is `skills_gen.go` at the repo root, generated by `volt gen skills`.

```
$ browserctrl skills --help
Serves browserctrl's agent skills (SKILL.md format), always matched to this
binary's version (dev): content is fetched once per version from the
project's release and cached; a binary upgrade re-syncs automatically.

Install into an agent harness with skills.sh:  npx skills add khanakia/browserctrl
Verify an installed copy:                      browserctrl skills check <dir>

Usage:
  browserctrl skills [flags]
  browserctrl skills [command]

Available Commands:
  check       Is an installed copy of a skill current for this binary?
  get         Print a skill's full content
  list        List available skills
  path        Print the filesystem path of the skills (or one skill)
  refresh     Re-download this version's skills bundle (publish-recovery; never automatic)
  version     Binary version, canonical skills hash, and the serving source

Flags:
  -h, --help   help for skills

Use "browserctrl skills [command] --help" for more information about a command.
```

Where the content comes from, first hit wins: the `BROWSERCTRL_SKILLS_DIR` environment variable; for a dev build (version `dev`, which includes any `go build` from a checkout) the nearest `skills/` directory walking up from the working directory; otherwise the per-version cache under `os.UserCacheDir()/browserctrl/skills/<version>/`, filled once from the `<version>` release's `skills_<version>.tar.gz` and checksum-verified. Every command says which source served.

```
$ browserctrl skills
  browserctrl-core  Pick the right Chrome/Chromium browser for the claude-in-chrome MCP without a click-through prompt.

install for agents:  npx skills add khanakia/browserctrl
```

```
$ browserctrl skills version
browserctrl dev
skills_hash: d07cbd067bc84cf303b290d082aafc6f897399488956f5b47f3e3ef3b6bc1879
source: live (/path/to/checkout/skills)
```

```
$ browserctrl skills check skills/browserctrl-core
browserctrl-core  current
```

`skills check <dir>` exits 0 when the installed copy matches this binary's skill byte for byte (hidden files such as `.DS_Store` ignored) and 1 when it is stale — the freshness loop the SKILL.md itself tells an agent to run. `skills get browserctrl-core` prints the full SKILL.md (`--json`, `--full` for supporting files); `skills path` prints the directory being served; `skills refresh` drops and re-fetches the cache for a release build. The `source:` path above is shortened; it is the absolute path of the checkout's `skills/` directory.

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
| `accountUuid` | string | The Claude account the extension is signed in as (`accountUuid` in its storage). Empty when signed out or not installed. Decides which sessions can reach this browser — see [Claude accounts](#claude-accounts). |
| `orgUuid` | string | The organization of the current token (`tokenOrg.uuid`). Empty when signed out. Read from `tokenOrg`, not `lastActiveOrgHint`: the hint goes stale after an account switch and the two are known to disagree on a real profile. |
| `extension` | string | Which extension id this store belongs to. Only differs from the Claude Code id under `--all`. Empty when `installed` is false. |
| `installed` | bool | Whether a Claude extension store exists for this profile. Always `true` except for the extra rows `list --profiles` adds. False means the profile can never yield a device id until the extension is installed in it — distinct from an installed extension that has simply never connected (`installed: true` with an empty `deviceId`). |
| `error` | string, optional | Why this store could not be read. `deviceId` and `displayName` are empty when set. |

## Table columns

`STATE` · `MCP` (`yes`/`no` = `mcpConnected`) · `DEVICE ID` · `BROWSER` · `PROFILE` (directory) · `NAME` · `EMAIL` · `DISPLAY NAME` · `CLAUDE ACCOUNT`. Empty cells show `-` so columns stay aligned and a missing value cannot be mistaken for a rendering glitch. A row whose store failed to read gets an extra trailing cell `ERROR: <reason>`. `list --profiles` inserts one more column after `STATE`: `EXTENSION`, either `installed` or `not installed`.

Sort order is `running` → `idle` → `unknown`, then browser kind alphabetically, then profile directory. Running entries come first because they are the ones an agent can connect to.

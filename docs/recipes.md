# Recipes

End-to-end workflows. Command output here was captured with `task docs:capture` against the doc fixture, so `browser` reads `custom`; substitute your real browser kinds.

**Contents:** [Claude Code: pick the browser without a prompt](#claude-code-pick-the-browser-without-a-prompt) · [Install the rule as an agent skill](#install-the-rule-as-an-agent-skill) · [Give every browser a name you will remember](#give-every-browser-a-name-you-will-remember) · [Which "Browser N" is which?](#which-browser-n-is-which) · [Which Claude account is each browser signed into?](#which-claude-account-is-each-browser-signed-into) · [A profile is missing from the list](#a-profile-is-missing-from-the-list) · [Shell: capture an id safely](#shell-capture-an-id-safely) · [jq cookbook](#jq-cookbook) · [Custom user-data dirs](#custom-user-data-dirs) · [Test your own tooling with the fixture package](#test-your-own-tooling-with-the-fixture-package)

## Claude Code: pick the browser without a prompt

The problem: with several browsers connected, Claude Code's `claude-in-chrome` MCP must either guess or send a "click Connect in the browser you want" prompt to every extension. The fix is one rule in your global `CLAUDE.md` (`~/.claude/CLAUDE.md`) that tells the agent to look the id up first:

```
# Chrome browser selection
Before any claude-in-chrome action, run `browserctrl list --running --json` and pick the deviceId whose profileName / email / displayName matches what I asked for (e.g. "my legable chrome" → the entry with email aman@legable.co). Call select_browser with that id directly. Only fall back to switch_browser if the list is empty or the match is ambiguous.
```

What the agent then sees, and why each field is there:

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

- `--running` because `select_browser` can only attach to an open window; idle profiles would be noise.
- `email` is the most reliable human key: profile names are often auto-generated (`Profile 2`, `Person 1`), emails are not.
- `displayName` is the name you gave the extension; once every browser has one (next recipe) the agent can match on that alone.

You can phrase requests as "use my work chrome" or "the legable profile" and the agent resolves them from this list. If you prefer to keep the agent's job even smaller, have it run `browserctrl find <your words> --running` and use whatever id comes back, falling back to the prompt only on exit code 2.

## Install the rule as an agent skill

The CLAUDE.md rule above is also published as `skills/browserctrl-core/SKILL.md`, so a harness that supports the SKILL.md format can install it once instead of every project carrying the paragraph:

```sh
npx skills add khanakia/browserctrl
```

The binary is the source of truth for that file. After upgrading `browserctrl`, ask it whether the installed copy still matches:

```
$ browserctrl skills check <dir-where-the-harness-put-SKILL.md>
browserctrl-core  current
```

Exit 0 means current; exit 1 means stale, in which case `browserctrl skills get browserctrl-core` prints the version that matches the binary and the skill's own header tells the agent to prefer that output. Release builds fetch their skill bundle from the matching `vX.Y.Z` release once and cache it; a source build serves the checkout's `skills/` directory, so editing the skill needs no rebuild.

## Give every browser a name you will remember

The `DISPLAY NAME` column is `bridgeDisplayName`, which the Claude extension stores when you name a browser during `switch_browser` (the confirmation screen has a name field). Name each one once — `work-chrome`, `personal`, `client-x` — and from then on:

```
$ browserctrl find work
2aa533d3-d03f-4dd8-b664-f59b8930eccc
```

`find` matches display names alongside emails and profile names, so a short unique word per browser is enough. Profiles that have never been named show `-`; `list` still identifies them by email and profile directory.

## Which "Browser N" is which?

When the MCP reports `Browser 1 / 2 / 3` and you need to know which window each is, match ids:

```
$ browserctrl list --running
STATE    MCP  DEVICE ID                             BROWSER  PROFILE    NAME  EMAIL              DISPLAY NAME
running  yes  ce9a8e06-61d3-4e7b-894b-13fd92987212  custom   Default    Aman  aman@example.com   chrome-main
running  yes  2aa533d3-d03f-4dd8-b664-f59b8930eccc  custom   Profile 5  Work  aman@work.example  work-chrome
```

The `DEVICE ID` column is byte-for-byte the `deviceId` in `list_connected_browsers`. The `name` there (`Browser 1`) is assigned per listing and can change between calls; the id does not, so always match on the id.

If a connected browser does **not** appear in `list --running`, it is using a profile in a location the tool does not scan by default — pass its user-data dir with `--root` (next recipes) or open an issue naming the browser.

## A profile is missing from the list

The symptom: a Chrome profile you have open right now, signed in and working, does not appear in `browserctrl list` at all — not even as `idle`.

The cause is almost always that the Claude extension is not installed in **that** profile. Chrome scopes extensions per profile, and the device id is minted by the extension the first time it runs in one, so a profile without the extension has no id to report, no store for browserctrl to read, and no presence in the MCP's `list_connected_browsers` either. `list` omits it because a row with nothing selectable in it is noise; `--profiles` is the switch that turns that omission into an explicit answer:

```sh
browserctrl list --profiles
```

Look for the profile with `EXTENSION: not installed`. Its `STATE` is still real (read from the profile's own localStorage lock), so a row reading `running  not installed` is exactly the confusing case: the window is open, and Claude Code still cannot see it. Install Claude in Chrome from that window, then re-run `browserctrl list` — the row appears as soon as the extension has written its id.

Two nearby cases worth telling apart:

| Symptom | Meaning | Fix |
|---|---|---|
| Profile absent from `list`, shown by `--profiles` as `not installed` | no extension in that profile | install the extension in that window |
| Listed with an empty `DEVICE ID` and `MCP no` | extension installed, never connected | open the extension in that window and connect it once |
| Listed with an id but `STATE idle` | extension fine, browser closed | open the browser; `select_browser` needs a running profile |

```sh
# scriptable version: which profiles have no extension?
browserctrl list --profiles --json | jq -r '.[] | select(.installed | not) | "\(.browser)\t\(.profileDir)\t\(.email)"'
```

## Which Claude account is each browser signed into?

When several Claude accounts are in play, a correct device id can still fail with "No connected browser has deviceId …" — `list_connected_browsers` only reports browsers on the session's own account. The `CLAUDE ACCOUNT` column groups them:

```sh
browserctrl list
```

The account Claude Code is signed in as appears as its email; every other one appears as `other account`, because Claude stores no email for them in the browser — only a uuid. If you want real names, take the uuids from `--json` and name them once:

```sh
alias bl='browserctrl list --account-alias 2f7c1b90-...=work@example.com --account-alias 8ad41c02-...=personal@example.com'
```

Group them from JSON without aliases at all:

```sh
browserctrl list --json | jq -r 'group_by(.accountUuid)[] | "\(.[0].accountUuid // "signed out"): \(map(.profileName) | join(", "))"'
```

Do not read the `email` column as the Claude account — it is the browser profile's Google/Microsoft account from `Local State`, and the two routinely differ on the same profile.

## Shell: capture an id safely

`find` prints the id alone on stdout and everything else on stderr, and it uses distinct exit codes, so it composes:

```sh
if id=$(browserctrl find legable --running); then
  echo "selecting $id"
else
  case $? in
    1) echo "no running browser matches" ;;
    2) echo "more than one matches; add a term (candidates were printed above)" ;;
  esac
fi
```

Under `set -e` remember that `$(…)` inside an `if` does not abort the script; outside one it will, which is usually what you want for a hard-required id.

Ambiguity looks like this and leaves stdout empty, so `$id` stays unset rather than holding half a table:

```
$ browserctrl find chrome
error: query matches more than one browser
STATE    MCP  DEVICE ID                             BROWSER  PROFILE    NAME  EMAIL              DISPLAY NAME
running  yes  ce9a8e06-61d3-4e7b-894b-13fd92987212  custom   Default    Aman  aman@example.com   chrome-main
running  yes  2aa533d3-d03f-4dd8-b664-f59b8930eccc  custom   Profile 5  Work  aman@work.example  work-chrome
```

## jq cookbook

All of these read `browserctrl list --json` (or `--running --json`).

```sh
# id of the profile signed in as a given email
browserctrl list --json | jq -r '.[] | select(.email == "aman@legable.co") | .deviceId'

# one line per running browser: id, kind, display name
browserctrl list --running --json | jq -r '.[] | "\(.deviceId)  \(.browser)  \(.displayName)"'

# profiles where the extension is installed but has never connected
browserctrl list --json | jq -r '.[] | select(.deviceId == "") | "\(.browser) \(.profileDir) \(.email)"'

# how many browsers are open right now
browserctrl list --running --json | jq 'length'

# any store that failed to read (mid-compaction snapshot etc.)
browserctrl list --json | jq -r '.[] | select(.error != null) | "\(.profileDir): \(.error)"'
```

`list --json` prints `null` when there are no entries; `jq 'length'` handles that (`0`), `.[]` does too (no output). A `find --json` result is a single object, not an array.

## Custom user-data dirs

Anything started with `--user-data-dir` (Chrome for Testing, a Playwright or Puppeteer persistent context, a kiosk profile, Chromium built from source) lives outside the well-known roots. Point the tool at it:

```sh
browserctrl list --root ~/chrome-for-testing-profile --root ./e2e/.chromium
```

`--root` replaces the default roots rather than adding to them, so the output is exactly those dirs, labelled `browser: custom`. Combine with `find` as usual:

```sh
browserctrl find e2e --root ./e2e/.chromium
```

Well-known roots per OS, for reference:

| OS | Roots scanned by default |
|---|---|
| macOS | `~/Library/Application Support/` + `Google/Chrome`, `Google/Chrome Beta`, `Google/Chrome Canary`, `Google/Chrome Dev`, `Chromium`, `BraveSoftware/Brave-Browser`, `Microsoft Edge`, `Arc/User Data`, `Vivaldi`, `com.operasoftware.Opera`, `com.operasoftware.OperaGX` |
| Linux | `$XDG_CONFIG_HOME` (default `~/.config`) + `google-chrome`, `google-chrome-beta`, `google-chrome-unstable`, `chromium`, `BraveSoftware/Brave-Browser`, `microsoft-edge`, `vivaldi`, `opera` |
| Windows | `%LOCALAPPDATA%` + `Google\Chrome\User Data`, `Google\Chrome Beta\User Data`, `Google\Chrome SxS\User Data`, `Google\Chrome Dev\User Data`, `Chromium\User Data`, `BraveSoftware\Brave-Browser\User Data`, `Microsoft\Edge\User Data`, `Vivaldi\User Data`; `%APPDATA%` + `Opera Software\Opera Stable`, `Opera Software\Opera GX Stable` |

Only roots that exist are scanned; a missing browser costs nothing. macOS is the verified table; Linux and Windows are compiled and unit-tested for shape but have not been run against a live machine.

## Test your own tooling with the fixture package

If you build on the library, `browser/browsertest` writes a fake user-data root with real LevelDB stores so your tests need no browser:

```go
import (
	"context"
	"testing"

	"github.com/khanakia/browserctrl/browser"
	"github.com/khanakia/browserctrl/browser/browsertest"
)

func TestPicksWorkChrome(t *testing.T) {
	root := browsertest.Root(t,
		browsertest.ProfileSpec{Dir: "Default", Name: "Aman", Email: "aman@example.com", Stores: map[string]map[string]any{
			string(browser.ExtClaudeCode): {browsertest.KeyBridgeDeviceID: "id-main", browsertest.KeyBridgeDisplayName: "chrome-main"},
		}},
		browsertest.ProfileSpec{Dir: "Profile 5", Name: "Work", Email: "aman@work.example", Stores: map[string]map[string]any{
			string(browser.ExtClaudeCode): {browsertest.KeyBridgeDeviceID: "id-work", browsertest.KeyBridgeDisplayName: "work-chrome"},
		}},
	)
	entries, err := browser.Scan(context.Background(), browser.Options{
		Roots: []browser.Root{{Kind: browser.KindCustom, Path: root}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := browser.Match(entries, "work")
	if len(got) != 1 || got[0].DeviceID != "id-work" {
		t.Fatalf("got %+v", got)
	}
}
```

To simulate a *running* profile, hold `flock(LOCK_EX)` on `browsertest.LockPath(root, "Profile 5", ext)` for the duration of the test — that is exactly what `internal/docfixture` does to produce the `running` rows in these docs. `browsertest.Build` is the same thing without `testing.T`, for programs rather than tests.

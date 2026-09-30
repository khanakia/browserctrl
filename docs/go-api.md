# Go API

`github.com/khanakia/browserctrl/browser` is the library the CLI is a thin front-end over. It has no dependency on cobra or on the CLI; the only third-party dependency is `github.com/syndtr/goleveldb` for reading the extension stores. Full godoc: `go doc -all github.com/khanakia/browserctrl/browser`.

**Contents:** [Scan](#scan) · [Entry](#entry) · [Filtering](#filtering) · [Claude accounts](#claude-accounts) · [Lower-level readers](#lower-level-readers) · [Constants](#constants) · [browsertest](#browsertest) · [Guarantees](#guarantees)

## Scan

```go
func Scan(ctx context.Context, opts Options) ([]Entry, error)

type Options struct {
	Roots              []Root        // nil → DefaultRoots()
	Extensions         []ExtensionID // nil → ExtensionValues (all known Claude extension ids)
	IncludeAllProfiles bool          // also return profiles with NO extension store, as Installed=false entries
}

type Root struct {
	Kind Kind
	Path string
}
```

Walks every root → profile → extension store and returns one `Entry` per store found, sorted running → idle → unknown, then by browser kind, then by profile directory. The zero `Options` scans the well-known roots for every known extension id; the CLI passes `Extensions: []ExtensionID{ExtClaudeCode}` unless `--all` is given.

`IncludeAllProfiles` adds one `Installed: false` entry for every profile in which none of `Extensions` was found, instead of omitting it; this is what `list --profiles` uses. Its `State` is probed on the profile's own localStorage LevelDB lock, since there is no extension store to probe, and a profile Chromium has never opened reports `RunStateUnknown`.

Errors: a root whose `Local State` cannot be parsed aborts the scan (real corruption). A single unreadable extension store does **not** — it is recorded in `Entry.Error` and the scan continues. Cancelling `ctx` aborts between files.

```go
entries, err := browser.Scan(ctx, browser.Options{})
```

## Entry

```go
type Entry struct {
	DeviceID     string      // bridgeDeviceId — what select_browser takes; "" if never connected
	DisplayName  string      // bridgeDisplayName — user-given name; "" if never set
	State        RunState    // RunStateRunning | RunStateIdle | RunStateUnknown
	McpConnected bool        // sticky "has completed an MCP handshake at some point"
	AccountUUID  string      // the Claude account the extension is signed in as; "" if signed out
	OrgUUID      string      // the organization of its current token (from tokenOrg); "" if signed out
	Browser      Kind        // KindChrome, KindVivaldi, … or KindCustom for caller-supplied roots
	BrowserPath  string      // the user-data root scanned
	ProfileDir   string      // "Default", "Profile 5" — the stable key
	ProfileName  string      // from Local State; "" for browsers that do not populate it
	Email        string      // from Local State; "" if signed out
	Extension    ExtensionID // which extension id this store belongs to; "" when Installed is false
	Installed    bool        // false only for the extra entries IncludeAllProfiles adds
	Error        string      // set when the store exists but could not be read; omitted from JSON when empty
}
```

JSON tags match the CLI's `--json` output field for field (`deviceId`, `displayName`, `state`, `mcpConnected`, `accountUuid`, `orgUuid`, `browser`, `browserPath`, `profileDir`, `profileName`, `email`, `extension`, `installed`, `error`), so a program can consume either.

`Installed` is an explicit field because an empty `DeviceID` already means something else: an installed extension that has never connected. `AccountUUID` decides reachability — the MCP pairs a session only with browsers on the session's own Claude account — and it is unrelated to `Email`, which is the browser profile's Google or Microsoft account.

## Filtering

```go
func Match(entries []Entry, query string) []Entry
func OnlyRunning(entries []Entry) []Entry
func OnlyAccount(entries []Entry, uuid string) []Entry
```

`Match` splits `query` on whitespace and keeps entries where **every** term is a case-insensitive substring of at least one of `DisplayName`, `ProfileName`, `Email`, `ProfileDir`, `Browser`, `DeviceID`. An empty query returns the input unchanged. It is substring, not fuzzy, on purpose: silently picking the wrong browser is worse than asking for one more character.

`OnlyRunning` keeps `State == RunStateRunning`. `RunStateUnknown` is excluded: "could not tell" must never be presented as "connected".

`OnlyAccount` keeps entries whose `AccountUUID` equals `uuid`: the browsers a session running as that account can reach. An empty `uuid` returns `nil` rather than matching every signed-out entry. It is necessary, not sufficient — the extension's bridge must also be live at the moment of the call, which nothing on disk records.

The CLI's `find` tie-break (drop id-less entries, then prefer the single running one) is CLI policy in the root `main` package, not library behaviour — compose it yourself if you want the same rule:

```go
hits := browser.Match(entries, "work chrome")
if running := browser.OnlyRunning(hits); len(running) == 1 {
	hits = running
}
```

## Claude accounts

```go
type ClaudeAccount struct {
	UUID        string // matches Entry.AccountUUID
	Email       string
	DisplayName string
	FullName    string
	OrgUUID     string // matches Entry.OrgUUID
	OrgName     string
}

func DefaultClaudeConfigPath() (string, error)
func ClaudeConfigPaths() ([]string, error)
func ReadClaudeAccount(path string) (ClaudeAccount, error)
func ReadClaudeAccounts(paths []string) ([]ClaudeAccount, error)

var ErrNoClaudeAccount error
```

An extension store holds only an account uuid. The one place a uuid is paired with an email is a Claude Code config (`.claude.json` → `oauthAccount`), and one config names exactly one account: the one that Claude Code profile is signed in as. These functions read it; nothing is ever written back, and every path is a parameter, so the package never reaches for `$HOME` behind the caller's back.

- `DefaultClaudeConfigPath` is the config of the **current** profile: `$CLAUDE_CONFIG_DIR/.claude.json` when that variable is set, else `~/.claude.json`. Use it to answer "which account is this session?".
- `ClaudeConfigPaths` lists every config worth reading, most specific first: the `CLAUDE_CONFIG_DIR` one, `~/.claude.json`, then each sibling directory of the home whose name starts with `.claude` (`~/.claude-work/.claude.json` and so on). Files matching that prefix are skipped, symlinked directories are followed, and paths are returned whether or not they exist. Use it to name every account on the machine.
- `ReadClaudeAccount` returns `ErrNoClaudeAccount` for a missing file, a config with no account, or an account without a uuid; malformed JSON is a real error.
- `ReadClaudeAccounts` returns the distinct accounts across `paths`, skipping those with no account. A config that exists but cannot be read is reported in the returned error **without** discarding the accounts that did load, so a caller that only wants labels can use the slice and ignore the error.

```go
// which browsers can the session in this shell reach?
path, err := browser.DefaultClaudeConfigPath()
if err != nil {
	return err
}
session, err := browser.ReadClaudeAccount(path)
if err != nil {
	return err
}
reachable := browser.OnlyAccount(browser.OnlyRunning(entries), session.UUID)
```

An account no profile is signed in as cannot be named at all: its email is stored nowhere on disk.

## Lower-level readers

```go
func DefaultRoots() ([]Root, error)
func ReadProfiles(root string) ([]Profile, error)
func ReadExtensionStore(ctx context.Context, dir string) (ExtensionRecord, error)

type Profile struct{ Dir, Name, Email string }
type ExtensionRecord struct {
	DeviceID     string
	DisplayName  string
	McpConnected bool
	AccountUUID  string
	OrgUUID      string
}

var ErrNoExtensionStore = errors.New("browser: extension store not present")
```

- `DefaultRoots` returns the per-OS well-known roots **that exist on disk**. Unsupported OS → error.
- `ReadProfiles` parses `<root>/Local State`. Missing file or empty `info_cache` → falls back to `[{Dir: "Default"}]` if that directory exists, else `nil`. Malformed JSON → error.
- `ReadExtensionStore` snapshots the LevelDB at `dir` into a temp directory, opens the copy read-only, reads its keys (`bridgeDeviceId`, `bridgeDisplayName`, `mcpConnected`, `accountUuid`, and the `uuid` inside the `tokenOrg` object), and removes the snapshot. `ErrNoExtensionStore` when `dir` does not exist (extension not installed); an error when the copy cannot be opened (for instance a snapshot taken mid-compaction) or a value has an unexpected JSON type. Missing keys are not errors: a fresh install yields the zero record.

## Constants

Every closed set is typed; there are no bare strings to compare against.

| Type | Values | Iteration list |
|---|---|---|
| `Kind` | `KindChrome` `KindChromeBeta` `KindChromeCanary` `KindChromeDev` `KindChromium` `KindBrave` `KindEdge` `KindArc` `KindVivaldi` `KindOpera` `KindOperaGX` `KindCustom` | `KindValues` |
| `ExtensionID` | `ExtClaudeCode` (the Claude Code extension, verified) · `ExtClaudeDesktop` · `ExtClaudeDesktopAlt` (the two other ids in the Claude desktop app's native-host manifest, layout assumed) | `ExtensionValues` |
| `RunState` | `RunStateRunning` `RunStateIdle` `RunStateUnknown` | `RunStateValues` |

## browsertest

`github.com/khanakia/browserctrl/browser/browsertest` writes fake Chromium roots with real LevelDB stores so code built on `browser` can be tested without a browser.

```go
type ProfileSpec struct {
	Dir    string                    // profile directory name
	Name   string                    // Local State name (optional)
	Email  string                    // Local State user_name (optional)
	Stores map[string]map[string]any // extension id → key/value; values JSON-encoded like Chromium does
}

func Build(root string, profiles ...ProfileSpec) error          // any program
func BuildStore(dir string, kv map[string]any) error            // one LevelDB
func Root(t testing.TB, profiles ...ProfileSpec) string          // Build into t.TempDir(), t.Fatal on error
func WriteStore(t testing.TB, dir string, kv map[string]any)      // BuildStore, t.Fatal on error
func LockPath(root, profileDir, ext string) string               // an extension store's LOCK, for simulating "running"
func ProfileLockPath(root, profileDir string) string             // the profile's own LOCK, for a running profile with no extension

const (
	KeyBridgeDeviceID    = "bridgeDeviceId"
	KeyBridgeDisplayName = "bridgeDisplayName"
	KeyMcpConnected      = "mcpConnected"
	KeyAccountUUID       = "accountUuid"
	KeyTokenOrg          = "tokenOrg" // an OBJECT: map[string]any{"hybrid": false, "uuid": "…"}
)
```

`Build` also creates each profile's localStorage LevelDB, as Chromium does for every profile it opens; that is the lock `ProfileLockPath` points at and the one `IncludeAllProfiles` probes.

`Local State` is written only if at least one profile has a `Name` or `Email`, so a spec with both empty exercises the no-Local-State fallback (Opera-style). The layout constants are deliberately duplicated from `browser` rather than imported: the fixture must produce what Chromium produces, independent of what the scanner expects, so a typo on either side fails a test instead of both agreeing.

A worked example is in [recipes](recipes.md#test-your-own-tooling-with-the-fixture-package).

## Guarantees

- **Read-only.** Nothing under a user-data root is ever written or locked for longer than a non-blocking probe. Extension stores are copied before being opened. Claude Code configs are read and never written.
- **No hidden inputs.** Every path is a parameter. The only functions that consult the environment are the ones named for it (`DefaultRoots`, `DefaultClaudeConfigPath`, `ClaudeConfigPaths`), and each returns paths for the caller to pass on.
- **Concurrency-safe probing.** The running probe takes a *shared* non-blocking `flock`; concurrent scans in one process do not interfere with each other (an exclusive probe did, and was caught by the race-detector run).
- **Typed closed sets.** `Kind`, `ExtensionID`, `RunState` are typed with canonical `*Values` slices; a new value must be added there, and a test asserts the sort-rank map covers every `RunState`.
- **Contexts honoured.** `Scan` and `ReadExtensionStore` check `ctx` between files.
- **Platform honesty.** Path tables exist for darwin, linux and windows and are cross-compiled in the gate; only darwin has been run against real browsers. The Windows `probeLock` returns `RunStateUnknown` rather than guessing.

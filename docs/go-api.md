# Go API

`github.com/khanakia/browserctrl/browser` is the library the CLI is a thin front-end over. It has no dependency on cobra or on the CLI; the only third-party dependency is `github.com/syndtr/goleveldb` for reading the extension stores. Full godoc: `go doc -all github.com/khanakia/browserctrl/browser`.

**Contents:** [Scan](#scan) · [Entry](#entry) · [Filtering](#filtering) · [Lower-level readers](#lower-level-readers) · [Constants](#constants) · [browsertest](#browsertest) · [Guarantees](#guarantees)

## Scan

```go
func Scan(ctx context.Context, opts Options) ([]Entry, error)

type Options struct {
	Roots      []Root        // nil → DefaultRoots()
	Extensions []ExtensionID // nil → ExtensionValues (all known Claude extension ids)
}

type Root struct {
	Kind Kind
	Path string
}
```

Walks every root → profile → extension store and returns one `Entry` per store found, sorted running → idle → unknown, then by browser kind, then by profile directory. The zero `Options` scans the well-known roots for every known extension id; the CLI passes `Extensions: []ExtensionID{ExtClaudeCode}` unless `--all` is given.

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
	Browser      Kind        // KindChrome, KindVivaldi, … or KindCustom for caller-supplied roots
	BrowserPath  string      // the user-data root scanned
	ProfileDir   string      // "Default", "Profile 5" — the stable key
	ProfileName  string      // from Local State; "" for browsers that do not populate it
	Email        string      // from Local State; "" if signed out
	Extension    ExtensionID // which extension id this store belongs to
	Error        string      // set when the store exists but could not be read; omitted from JSON when empty
}
```

JSON tags match the CLI's `--json` output field for field (`deviceId`, `displayName`, `state`, `mcpConnected`, `browser`, `browserPath`, `profileDir`, `profileName`, `email`, `extension`, `error`), so a program can consume either.

## Filtering

```go
func Match(entries []Entry, query string) []Entry
func OnlyRunning(entries []Entry) []Entry
```

`Match` splits `query` on whitespace and keeps entries where **every** term is a case-insensitive substring of at least one of `DisplayName`, `ProfileName`, `Email`, `ProfileDir`, `Browser`, `DeviceID`. An empty query returns the input unchanged. It is substring, not fuzzy, on purpose: silently picking the wrong browser is worse than asking for one more character.

`OnlyRunning` keeps `State == RunStateRunning`. `RunStateUnknown` is excluded: "could not tell" must never be presented as "connected".

The CLI's `find` tie-break (drop id-less entries, then prefer the single running one) is CLI policy in the root `main` package, not library behaviour — compose it yourself if you want the same rule:

```go
hits := browser.Match(entries, "work chrome")
if running := browser.OnlyRunning(hits); len(running) == 1 {
	hits = running
}
```

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
}

var ErrNoExtensionStore = errors.New("browser: extension store not present")
```

- `DefaultRoots` returns the per-OS well-known roots **that exist on disk**. Unsupported OS → error.
- `ReadProfiles` parses `<root>/Local State`. Missing file or empty `info_cache` → falls back to `[{Dir: "Default"}]` if that directory exists, else `nil`. Malformed JSON → error.
- `ReadExtensionStore` snapshots the LevelDB at `dir` into a temp directory, opens the copy read-only, reads the three keys, and removes the snapshot. `ErrNoExtensionStore` when `dir` does not exist (extension not installed); an error when the copy cannot be opened (for instance a snapshot taken mid-compaction) or a value has an unexpected JSON type. Missing keys are not errors: a fresh install yields the zero record.

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
func LockPath(root, profileDir, ext string) string               // the LOCK file, for simulating "running"

const (
	KeyBridgeDeviceID    = "bridgeDeviceId"
	KeyBridgeDisplayName = "bridgeDisplayName"
	KeyMcpConnected      = "mcpConnected"
)
```

`Local State` is written only if at least one profile has a `Name` or `Email`, so a spec with both empty exercises the no-Local-State fallback (Opera-style). The layout constants are deliberately duplicated from `browser` rather than imported: the fixture must produce what Chromium produces, independent of what the scanner expects, so a typo on either side fails a test instead of both agreeing.

A worked example is in [recipes](recipes.md#test-your-own-tooling-with-the-fixture-package).

## Guarantees

- **Read-only.** Nothing under a user-data root is ever written or locked for longer than a non-blocking probe. Extension stores are copied before being opened.
- **Concurrency-safe probing.** The running probe takes a *shared* non-blocking `flock`; concurrent scans in one process do not interfere with each other (an exclusive probe did, and was caught by the race-detector run).
- **Typed closed sets.** `Kind`, `ExtensionID`, `RunState` are typed with canonical `*Values` slices; a new value must be added there, and a test asserts the sort-rank map covers every `RunState`.
- **Contexts honoured.** `Scan` and `ReadExtensionStore` check `ctx` between files.
- **Platform honesty.** Path tables exist for darwin, linux and windows and are cross-compiled in the gate; only darwin has been run against real browsers. The Windows `probeLock` returns `RunStateUnknown` rather than guessing.

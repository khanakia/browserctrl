// Package browser discovers Chromium-family browser profiles that have a
// Claude browser extension installed and reports the extension's stable
// device id per profile.
//
// Why it exists: when several Chromium browsers/profiles are open, the Claude
// Code MCP tool `list_connected_browsers` only shows opaque ids and unstable
// labels ("Browser 1", "Browser 2"…). The id → "which window is that?" mapping
// lives on disk: each profile's `Local Extension Settings/<ext-id>/` LevelDB
// stores the extension's `bridgeDeviceId` (the id the MCP reports) and the
// user-chosen `bridgeDisplayName`. Joining that with `Local State` (profile name +
// signed-in email) gives a human-readable answer with zero clicking.
//
// This file holds every closed-set value the package uses. Rule: NO bare
// strings for anything with a closed set of choices — typo = compile error.
package browser

// Kind identifies a Chromium-based browser family. Each family has its own
// user-data root per OS (see roots.go); the profile layout inside the root is
// shared Chromium code, so one scanner handles all of them.
type Kind string

const (
	KindChrome       Kind = "chrome"
	KindChromeBeta   Kind = "chrome-beta"
	KindChromeCanary Kind = "chrome-canary"
	KindChromeDev    Kind = "chrome-dev"
	KindChromium     Kind = "chromium"
	KindBrave        Kind = "brave"
	KindEdge         Kind = "edge"
	KindArc          Kind = "arc"
	KindVivaldi      Kind = "vivaldi"
	KindOpera        Kind = "opera"
	KindOperaGX      Kind = "opera-gx"
	// KindCustom labels a root the caller passed explicitly (e.g. a
	// `--user-data-dir` Chrome for Testing profile) rather than a well-known
	// install location.
	KindCustom Kind = "custom"
)

// KindValues is the canonical iteration order — drives the roots table and
// the CLI's `--browser` filter validation. Reading and writing share this.
var KindValues = []Kind{
	KindChrome, KindChromeBeta, KindChromeCanary, KindChromeDev, KindChromium,
	KindBrave, KindEdge, KindArc, KindVivaldi, KindOpera, KindOperaGX, KindCustom,
}

// ExtensionID is a Chrome Web Store / unpacked extension id (32 chars a–p).
type ExtensionID string

const (
	// ExtClaudeCode is the "Claude in Chrome" extension that Claude Code's
	// `claude-in-chrome` MCP server talks to. Its id is pinned in the native
	// messaging host manifest Claude Code writes
	// (`com.anthropic.claude_code_browser_extension.json`). This is the one
	// verified to store `bridgeDeviceId` + `bridgeDisplayName` + `mcpConnected`.
	ExtClaudeCode ExtensionID = "fcoeoabgfenejglbffodgkkbkcdhcgfn"
	// ExtClaudeDesktop and ExtClaudeDesktopAlt are the other two ids allowed
	// by the Claude desktop app's native-host manifest
	// (`com.anthropic.claude_browser_extension.json`). Their storage layout is
	// assumed identical but is NOT verified — entries from them are labelled
	// with the extension id so a reader can tell them apart.
	ExtClaudeDesktop    ExtensionID = "dihbgbndebgnbjfmelmegjepbnkhlgni"
	ExtClaudeDesktopAlt ExtensionID = "dngcpimnedloihjnnfngkgjoidhnaolf"
)

// ExtensionValues is the default scan set. Order = report order when one
// profile has several of them installed.
var ExtensionValues = []ExtensionID{ExtClaudeCode, ExtClaudeDesktop, ExtClaudeDesktopAlt}

// RunState says whether a profile is currently open in a running browser.
type RunState string

const (
	// RunStateRunning: the profile's extension LevelDB lock is held → the
	// browser has that profile open with the extension loaded, i.e. it is
	// (or can be) connected to the MCP right now.
	RunStateRunning RunState = "running"
	// RunStateIdle: the lock is free → profile exists but is not open.
	RunStateIdle RunState = "idle"
	// RunStateUnknown: lock probing is unsupported on this OS or the LOCK
	// file could not be opened. Never treat as idle.
	RunStateUnknown RunState = "unknown"
)

// RunStateValues is the canonical iteration order (used by tests + CLI help).
var RunStateValues = []RunState{RunStateRunning, RunStateIdle, RunStateUnknown}

// Chromium on-disk layout names. These are Chromium source constants, kept
// verbatim; they are identical across every Kind above.
const (
	// localStateFile sits at the user-data root and holds `profile.info_cache`.
	localStateFile = "Local State"
	// extensionSettingsDir is the per-profile directory holding one LevelDB per
	// extension keyed by extension id (chrome.storage.local backing store).
	extensionSettingsDir = "Local Extension Settings"
	// levelDBLockFile is the file LevelDB flock()s while the DB is open. A held
	// lock is the cheapest reliable "is this profile running" signal.
	levelDBLockFile = "LOCK"
	// defaultProfileDir is the profile Chromium creates first; used as a
	// fallback when `Local State` has no info_cache (fresh install).
	defaultProfileDir = "Default"
	// localStorageDir / localStorageLevelDBDir locate the profile's DOM
	// localStorage LevelDB (`<profile>/Local Storage/leveldb`). Chromium opens
	// it for every profile it has open, whatever extensions are installed, so
	// its LOCK is the running signal for a profile that has no Claude
	// extension store to probe. Verified against a live Chrome on 2026-09-16:
	// it reported held/free identically to the extension store's LOCK for
	// every profile that had both.
	localStorageDir        = "Local Storage"
	localStorageLevelDBDir = "leveldb"
)

// chrome.storage.local keys the Claude extension writes. Values are JSON
// encoded by Chromium (a string value is stored as `"..."` with quotes).
const (
	storageKeyBridgeDeviceID    = "bridgeDeviceId"
	storageKeyBridgeDisplayName = "bridgeDisplayName"
	// storageKeyMcpConnected is a bool the extension flips when its MCP bridge
	// handshake succeeded. It is NOT cleared on browser exit, so it means
	// "has connected at some point", not "connected right now"; combine with
	// RunState for the live answer.
	storageKeyMcpConnected = "mcpConnected"
)

package browser

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
)

// Entry is one (browser, profile, extension) triple that has a Claude
// extension store on disk — the unit the user chooses between. With
// Options.IncludeAllProfiles it is also used for a profile that has no such
// store, flagged by Installed=false and carrying no device id or extension.
type Entry struct {
	// DeviceID is the id to pass to the MCP's select_browser. May be empty
	// (extension installed, never connected) — see ExtensionRecord.
	DeviceID string `json:"deviceId"`
	// DisplayName is the user-given name from the extension, if any.
	DisplayName string `json:"displayName"`
	// State is Running when the profile is open right now (lock held).
	State RunState `json:"state"`
	// McpConnected: the extension has completed an MCP handshake at least
	// once (sticky flag). Running && McpConnected ≈ "selectable right now".
	McpConnected bool `json:"mcpConnected"`
	// Browser is the family (chrome, vivaldi, …); BrowserPath its data root.
	Browser     Kind   `json:"browser"`
	BrowserPath string `json:"browserPath"`
	// ProfileDir / ProfileName / Email come from `Local State`.
	ProfileDir  string `json:"profileDir"`
	ProfileName string `json:"profileName"`
	Email       string `json:"email"`
	// Extension is which Claude extension id this store belongs to. Empty
	// on an Installed=false entry — there is no store to attribute.
	Extension ExtensionID `json:"extension"`
	// Installed is false only for the profile rows Scan adds when
	// Options.IncludeAllProfiles is set: the profile exists in `Local State`
	// but has no Claude extension store, so it can never yield a device id
	// and the MCP cannot see it at all. Every store-backed entry is true.
	//
	// Why an explicit field rather than "DeviceID == \"\"": an installed but
	// never-connected extension also has an empty id, and the two cases need
	// different advice (wait for a handshake vs install the extension).
	Installed bool `json:"installed"`
	// Error is set (and DeviceID/DisplayName left empty) when the store
	// exists but could not be read — e.g. a snapshot taken mid-compaction.
	// Kept on the entry instead of aborting so one bad profile never hides
	// the others. Empty string means the read succeeded.
	Error string `json:"error,omitempty"`
}

// Options configures Scan. Zero value = DefaultRoots + ExtensionValues.
type Options struct {
	// Roots to scan; nil → DefaultRoots().
	Roots []Root
	// Extensions to look for in each profile; nil → ExtensionValues.
	Extensions []ExtensionID
	// IncludeAllProfiles adds one Installed=false entry for every profile in
	// which none of Extensions was found, so a caller can show that the
	// profile exists and explain its absence instead of silently omitting it.
	// Off by default: the common question is "which ids can I select?", and
	// a profile with no extension answers it with noise.
	IncludeAllProfiles bool
}

// Scan walks every root → profile → extension store and returns one Entry per
// store found, sorted Running first, then by browser kind, then profile dir.
// With Options.IncludeAllProfiles, a profile where no store was found still
// yields one Installed=false Entry instead of being omitted.
//
// Why sorted that way: the running entries are the ones the user can actually
// connect to, so they belong at the top of any listing.
//
// Errors: a root whose `Local State` cannot be parsed aborts the scan (that is
// a real corruption, not a per-profile hiccup). A single unreadable extension
// store is recorded in Entry.Error and the scan continues.
func Scan(ctx context.Context, opts Options) ([]Entry, error) {
	roots := opts.Roots
	if roots == nil {
		var err error
		if roots, err = DefaultRoots(); err != nil {
			return nil, err
		}
	}
	exts := opts.Extensions
	if exts == nil {
		exts = ExtensionValues
	}
	var out []Entry
	for _, root := range roots {
		profiles, err := ReadProfiles(root.Path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", root.Kind, err)
		}
		for _, p := range profiles {
			found := false
			for _, ext := range exts {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				e, ok := scanOne(ctx, root, p, ext)
				if ok {
					out = append(out, e)
					found = true
				}
			}
			if !found && opts.IncludeAllProfiles {
				out = append(out, uninstalledEntry(root, p))
			}
		}
	}
	sortEntries(out)
	return out, nil
}

// scanOne builds the Entry for one profile+extension; ok=false when the
// extension is simply not installed there.
func scanOne(ctx context.Context, root Root, p Profile, ext ExtensionID) (Entry, bool) {
	dir := filepath.Join(root.Path, p.Dir, extensionSettingsDir, string(ext))
	rec, err := ReadExtensionStore(ctx, dir)
	if errors.Is(err, ErrNoExtensionStore) {
		return Entry{}, false
	}
	e := Entry{
		DeviceID:     rec.DeviceID,
		DisplayName:  rec.DisplayName,
		McpConnected: rec.McpConnected,
		State:        probeLock(filepath.Join(dir, levelDBLockFile)),
		Browser:      root.Kind,
		BrowserPath:  root.Path,
		ProfileDir:   p.Dir,
		ProfileName:  p.Name,
		Email:        p.Email,
		Extension:    ext,
		Installed:    true,
	}
	if err != nil {
		e.Error = err.Error()
	}
	return e, true
}

// uninstalledEntry describes a profile that has no Claude extension store.
//
// State still comes from a lock probe, just a different file: with no
// extension store there is no extension LOCK, so the profile's own
// localStorage LevelDB LOCK answers "is this profile open right now". A
// profile Chromium has never opened has neither, and probeLock reports
// unknown — which is the honest answer, not idle.
func uninstalledEntry(root Root, p Profile) Entry {
	return Entry{
		State:       probeLock(filepath.Join(root.Path, p.Dir, localStorageDir, localStorageLevelDBDir, levelDBLockFile)),
		Browser:     root.Kind,
		BrowserPath: root.Path,
		ProfileDir:  p.Dir,
		ProfileName: p.Name,
		Email:       p.Email,
	}
}

// runStateRank orders Running < Idle < Unknown for sorting.
var runStateRank = map[RunState]int{RunStateRunning: 0, RunStateIdle: 1, RunStateUnknown: 2}

func sortEntries(es []Entry) {
	sort.SliceStable(es, func(i, j int) bool {
		a, b := es[i], es[j]
		if runStateRank[a.State] != runStateRank[b.State] {
			return runStateRank[a.State] < runStateRank[b.State]
		}
		if a.Browser != b.Browser {
			return a.Browser < b.Browser
		}
		return a.ProfileDir < b.ProfileDir
	})
}

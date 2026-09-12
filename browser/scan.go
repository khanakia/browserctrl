package browser

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
)

// Entry is one (browser, profile, extension) triple that has a Claude
// extension store on disk — the unit the user chooses between.
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
	// Extension is which Claude extension id this store belongs to.
	Extension ExtensionID `json:"extension"`
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
}

// Scan walks every root → profile → extension store and returns one Entry per
// store found, sorted Running first, then by browser kind, then profile dir.
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
			for _, ext := range exts {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				e, ok := scanOne(ctx, root, p, ext)
				if ok {
					out = append(out, e)
				}
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
	}
	if err != nil {
		e.Error = err.Error()
	}
	return e, true
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

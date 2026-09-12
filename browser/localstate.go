package browser

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Profile is one Chromium profile as described by the root's `Local State`.
type Profile struct {
	// Dir is the profile directory name relative to the root ("Default",
	// "Profile 23"). It is the stable key; Name/Email are user-editable.
	Dir string `json:"dir"`
	// Name is the local profile label shown in Chromium's profile picker.
	Name string `json:"name"`
	// Email is the signed-in Google/Microsoft account, empty if signed out.
	Email string `json:"email"`
}

// localState mirrors only the slice of Chromium's `Local State` JSON we read.
// Every other key is ignored on purpose — the file is large and churns.
type localState struct {
	Profile struct {
		InfoCache map[string]localStateProfile `json:"info_cache"`
	} `json:"profile"`
}

// localStateProfile: the `profile.info_cache.<dir>` object. Chromium field
// names kept verbatim (wire format), translated into Profile on the way out.
type localStateProfile struct {
	Name     string `json:"name"`
	UserName string `json:"user_name"`
}

// ReadProfiles lists the profiles under a user-data root.
//
// Why it exists: profile dirs on disk are opaque ("Profile 23"); only `Local
// State` maps them to the name + email the user recognises.
//
// Fallback: when `Local State` is missing or has an empty info_cache (fresh
// install, or a browser like Opera that never populates it) the function
// returns the "Default" directory if it exists, with empty Name/Email, so the
// scanner still finds the extension store. Result is sorted by Dir.
func ReadProfiles(root string) ([]Profile, error) {
	raw, err := os.ReadFile(filepath.Join(root, localStateFile))
	switch {
	case errors.Is(err, os.ErrNotExist):
		return defaultProfileFallback(root), nil
	case err != nil:
		return nil, fmt.Errorf("read %s: %w", localStateFile, err)
	}
	profiles, err := parseLocalState(raw)
	if err != nil {
		return nil, fmt.Errorf("parse %s in %s: %w", localStateFile, root, err)
	}
	if len(profiles) == 0 {
		return defaultProfileFallback(root), nil
	}
	return profiles, nil
}

// parseLocalState decodes the JSON and returns profiles sorted by Dir.
// Separated from file I/O so it can be table-tested with fixtures.
func parseLocalState(raw []byte) ([]Profile, error) {
	var ls localState
	if err := json.Unmarshal(raw, &ls); err != nil {
		return nil, err
	}
	out := make([]Profile, 0, len(ls.Profile.InfoCache))
	for dir, p := range ls.Profile.InfoCache {
		out = append(out, Profile{Dir: dir, Name: p.Name, Email: p.UserName})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Dir < out[j].Dir })
	return out, nil
}

// defaultProfileFallback returns [Default] when that directory exists, else
// nil. Never errors: an absent Default simply means "no profiles".
func defaultProfileFallback(root string) []Profile {
	if st, err := os.Stat(filepath.Join(root, defaultProfileDir)); err == nil && st.IsDir() {
		return []Profile{{Dir: defaultProfileDir}}
	}
	return nil
}

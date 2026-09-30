package browser

import "strings"

// Match filters entries by a free-text query so a user (or an agent) can say
// "legable chrome" instead of pasting a UUID.
//
// Semantics: the query is split on whitespace; EVERY term must appear
// (case-insensitive substring) in at least one of the entry's identifying
// fields — DisplayName, ProfileName, Email, ProfileDir, Browser, DeviceID.
// Terms may hit different fields ("legable" → email, "chrome" → browser).
// An empty query returns every entry unchanged.
//
// Why substring and not fuzzy: the fields are short and the failure mode of
// fuzzy matching (picking the wrong browser silently) is worse than asking
// the user to type one more character.
func Match(entries []Entry, query string) []Entry {
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return entries
	}
	var out []Entry
	for _, e := range entries {
		if matchesAll(e, terms) {
			out = append(out, e)
		}
	}
	return out
}

func matchesAll(e Entry, terms []string) bool {
	hay := strings.ToLower(strings.Join([]string{
		e.DisplayName, e.ProfileName, e.Email, e.ProfileDir, string(e.Browser), e.DeviceID,
	}, "\x00"))
	for _, t := range terms {
		if !strings.Contains(hay, t) {
			return false
		}
	}
	return true
}

// OnlyRunning keeps entries whose State is Running. Unknown is excluded on
// purpose: "we could not tell" must never be presented as "connected".
func OnlyRunning(entries []Entry) []Entry {
	var out []Entry
	for _, e := range entries {
		if e.State == RunStateRunning {
			out = append(out, e)
		}
	}
	return out
}

// OnlyAccount keeps entries whose extension is signed in to the Claude
// account uuid — the browsers a session running as that account can reach.
//
// Why it exists: the MCP pairs a session only with browsers on its own
// account, so an id from any other entry is rejected however open that
// browser is. An empty uuid matches nothing, on purpose: "which account?"
// left unanswered must not be read as "every signed-out profile".
//
// Necessary, not sufficient: the extension's bridge must also be live at the
// moment of the call, and nothing on disk records that.
func OnlyAccount(entries []Entry, uuid string) []Entry {
	if uuid == "" {
		return nil
	}
	var out []Entry
	for _, e := range entries {
		if e.AccountUUID == uuid {
			out = append(out, e)
		}
	}
	return out
}

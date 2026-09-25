package main

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/khanakia/browserctrl/browser"
)

// Table column headers, in display order. Kept as one slice so the header
// row and the data rows cannot drift apart.
var tableHeader = []string{"STATE", "MCP", "DEVICE ID", "BROWSER", "PROFILE", "NAME", "EMAIL", "DISPLAY NAME", columnAccount}

// columnAccount holds the Claude account the extension is signed in as — the
// column that answers "why can this session not see that browser?", since the
// MCP only reports browsers on its own account.
const columnAccount = "CLAUDE ACCOUNT"

// profilesTableHeader is tableHeader plus the EXTENSION column, used by
// `list --profiles` where rows without the extension are included and the
// reason a row has no device id has to be visible.
var profilesTableHeader = append([]string{tableHeader[0], columnExtension}, tableHeader[1:]...)

// columnExtension and its cells: the answer to "why does this profile have no
// device id?" — not installed means the MCP cannot see the profile at all.
const (
	columnExtension  = "EXTENSION"
	cellExtInstalled = "installed"
	cellExtMissing   = "not installed"
)

// MCP column cells: "yes" = extension has completed an MCP handshake before.
const (
	cellMcpYes = "yes"
	cellMcpNo  = "no"
)

// placeholderEmpty is what an empty cell shows so columns stay aligned and a
// reader can tell "no value" from a rendering glitch.
const placeholderEmpty = "-"

// tabwriter layout: min cell width, tab width, padding, pad char.
const (
	tableMinWidth = 0
	tableTabWidth = 0
	tablePadding  = 2
	tablePadChar  = ' '
)

// accountLabeler turns an entry's Claude account uuid into something a human
// can read. Injected rather than computed here because the mapping comes from
// outside the scan (Claude Code's config, plus user-given aliases) and the
// renderer must stay a pure formatter.
type accountLabeler func(browser.Entry) string

// writeTable renders entries as an aligned text table. An empty slice prints
// only a hint line — an empty table with just a header reads like a bug.
func writeTable(w io.Writer, entries []browser.Entry, label accountLabeler) error {
	return renderTable(w, entries, false, label)
}

// writeProfilesTable is writeTable for `list --profiles`: same rows plus the
// EXTENSION column, and an empty-slice hint that talks about profiles rather
// than extension stores (in this mode "nothing found" means no profile at
// all, which is a different problem).
func writeProfilesTable(w io.Writer, entries []browser.Entry, label accountLabeler) error {
	return renderTable(w, entries, true, label)
}

// emptyHint lines, one per mode — see writeTable / writeProfilesTable.
const (
	emptyHintStores   = "no Claude extension stores found (is the extension installed in any profile?)"
	emptyHintProfiles = "no browser profiles found (is any Chromium browser installed, or is --root wrong?)"
)

// renderTable is the shared body. showExtension adds the EXTENSION column;
// the header and the row builder read from the same switch so they cannot
// drift apart.
func renderTable(w io.Writer, entries []browser.Entry, showExtension bool, label accountLabeler) error {
	header, hint := tableHeader, emptyHintStores
	if showExtension {
		header, hint = profilesTableHeader, emptyHintProfiles
	}
	if len(entries) == 0 {
		_, err := fmt.Fprintln(w, hint)
		return err
	}
	tw := tabwriter.NewWriter(w, tableMinWidth, tableTabWidth, tablePadding, tablePadChar, 0)
	if err := writeRow(tw, header...); err != nil {
		return err
	}
	for _, e := range entries {
		row := []string{string(e.State)}
		if showExtension {
			row = append(row, installedCell(e.Installed))
		}
		row = append(row,
			yesNo(e.McpConnected),
			orPlaceholder(e.DeviceID),
			string(e.Browser),
			e.ProfileDir,
			orPlaceholder(e.ProfileName),
			orPlaceholder(e.Email),
			orPlaceholder(e.DisplayName),
			orPlaceholder(label(e)),
		)
		if e.Error != "" {
			// Surface the per-entry read failure inline rather than hiding it.
			row = append(row, "ERROR: "+e.Error)
		}
		if err := writeRow(tw, row...); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func writeRow(w io.Writer, cells ...string) error {
	for i, c := range cells {
		sep := "\t"
		if i == len(cells)-1 {
			sep = "\n"
		}
		if _, err := fmt.Fprint(w, c, sep); err != nil {
			return err
		}
	}
	return nil
}

func installedCell(installed bool) string {
	if installed {
		return cellExtInstalled
	}
	return cellExtMissing
}

func yesNo(b bool) string {
	if b {
		return cellMcpYes
	}
	return cellMcpNo
}

func orPlaceholder(s string) string {
	if s == "" {
		return placeholderEmpty
	}
	return s
}

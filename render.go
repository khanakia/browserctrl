package main

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/khanakia/browserctrl/browser"
)

// Table column headers, in display order. Kept as one slice so the header
// row and the data rows cannot drift apart.
var tableHeader = []string{"STATE", "MCP", "DEVICE ID", "BROWSER", "PROFILE", "NAME", "EMAIL", "DISPLAY NAME"}

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

// writeTable renders entries as an aligned text table. An empty slice prints
// only a hint line — an empty table with just a header reads like a bug.
func writeTable(w io.Writer, entries []browser.Entry) error {
	if len(entries) == 0 {
		_, err := fmt.Fprintln(w, "no Claude extension stores found (is the extension installed in any profile?)")
		return err
	}
	tw := tabwriter.NewWriter(w, tableMinWidth, tableTabWidth, tablePadding, tablePadChar, 0)
	if err := writeRow(tw, tableHeader...); err != nil {
		return err
	}
	for _, e := range entries {
		row := []string{
			string(e.State),
			yesNo(e.McpConnected),
			orPlaceholder(e.DeviceID),
			string(e.Browser),
			e.ProfileDir,
			orPlaceholder(e.ProfileName),
			orPlaceholder(e.Email),
			orPlaceholder(e.DisplayName),
		}
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

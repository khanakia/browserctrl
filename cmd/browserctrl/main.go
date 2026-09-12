// Command browserctrl lists Chromium browser profiles that have the Claude
// browser extension installed, together with the extension's device id — the
// value the Claude Code `claude-in-chrome` MCP needs for `select_browser`.
//
// Why it exists: with several browsers/profiles open, Claude Code can only
// show "Browser 1 / 2 / 3" and asks the user to click through a confirmation
// screen in each. `browserctrl list` answers "which id is my legable Chrome?"
// from disk in one shot; `browserctrl find legable` prints just that id so an
// agent can pipe it straight into the tool call.
//
// Exit codes (closed set, see exitCode constants): 0 ok, 1 error / no match,
// 2 ambiguous match.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"

	"github.com/khanakia/browserctrl/browser"
)

// Exit codes. 2 (ambiguous) is distinct from 1 so a script can decide to
// prompt the user rather than treat it as a hard failure.
const (
	exitOK        = 0
	exitFailure   = 1
	exitAmbiguous = 2
)

// Flag names — shared between subcommands so `--json` / `--running` behave
// identically everywhere.
const (
	flagJSON    = "json"
	flagRunning = "running"
	flagAll     = "all"
	flagRoot    = "root"
)

// version is the value printed by `browserctrl --version`. Overridable at
// link time (`-ldflags "-X main.version=v1.2.3"`) for release builds; when
// left as versionDev, resolveVersion falls back to the module version Go
// embeds for `go install pkg@vX.Y.Z` builds.
var version = versionDev

// versionDev marks a build with no release version stamped in.
const versionDev = "dev"

// Sentinel errors mapped to exit codes in main.
var (
	errNoMatch   = errors.New("no browser matches the query")
	errAmbiguous = errors.New("query matches more than one browser")
)

// listFlags is the parsed flag set shared by `list` and `find`.
type listFlags struct {
	json    bool
	running bool
	all     bool
	// roots, when non-empty, REPLACES the well-known install locations with
	// the given user-data dirs (labelled browser=custom).
	roots []string
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is main without the process exit so tests can drive it end to end.
func run(args []string, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	root := newRootCmd(stdout, stderr)
	root.SetArgs(args)
	err := root.ExecuteContext(ctx)
	switch {
	case err == nil:
		return exitOK
	case errors.Is(err, errAmbiguous):
		return exitAmbiguous
	default:
		// stderr write failure has no further reporting channel; the exit
		// code already carries the outcome.
		_, _ = fmt.Fprintln(stderr, "error:", err)
		return exitFailure
	}
}

func newRootCmd(stdout, stderr io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:           "browserctrl",
		Short:         "List Claude-connected Chromium browser profiles and their device ids",
		Version:       resolveVersion(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.AddCommand(newListCmd(), newFindCmd())
	return root
}

func addListFlags(cmd *cobra.Command, f *listFlags) {
	cmd.Flags().BoolVar(&f.json, flagJSON, false, "emit JSON instead of a table")
	cmd.Flags().BoolVar(&f.running, flagRunning, false, "only profiles currently open in a running browser")
	cmd.Flags().BoolVar(&f.all, flagAll, false, "scan the Claude desktop-app extension ids too, not just Claude Code's")
	cmd.Flags().StringArrayVar(&f.roots, flagRoot, nil, "scan this Chromium user-data dir instead of the well-known ones (repeatable)")
}

func newListCmd() *cobra.Command {
	var f listFlags
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show every profile with the Claude extension, running ones first",
		Example: `  browserctrl list
  browserctrl list --running --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			entries, err := scan(cmd.Context(), f)
			if err != nil {
				return err
			}
			if f.json {
				return writeJSON(cmd.OutOrStdout(), entries)
			}
			return writeTable(cmd.OutOrStdout(), entries)
		},
	}
	addListFlags(cmd, &f)
	return cmd
}

func newFindCmd() *cobra.Command {
	var f listFlags
	cmd := &cobra.Command{
		Use:   "find <term>...",
		Short: "Print the device id of the single profile matching all terms",
		Long: `Every term must match (case-insensitive substring) one of: display name,
profile name, email, profile dir, browser kind, device id. When several
profiles match but exactly one is running, that one wins. Otherwise the
candidates are listed on stderr and the exit code is 2.`,
		Example: `  browserctrl find legable
  browserctrl find vivaldi work
  DEVICE=$(browserctrl find analyzify)`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			entries, err := scan(cmd.Context(), f)
			if err != nil {
				return err
			}
			matches := browser.Match(entries, strings.Join(args, " "))
			hit, err := pickOne(matches)
			if err != nil {
				if errors.Is(err, errAmbiguous) {
					// Best-effort diagnostics on stderr; the ambiguity error is
					// the result, so a failure to print candidates is not promoted.
					_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "error:", err)
					_ = writeTable(cmd.ErrOrStderr(), matches)
				}
				return err
			}
			if f.json {
				return writeJSON(cmd.OutOrStdout(), hit)
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), hit.DeviceID)
			return err
		},
	}
	addListFlags(cmd, &f)
	return cmd
}

// scan runs browser.Scan with the flag-derived options and applies the
// --running filter. --all widens the extension set to every known id.
func scan(ctx context.Context, f listFlags) ([]browser.Entry, error) {
	opts := browser.Options{Extensions: []browser.ExtensionID{browser.ExtClaudeCode}}
	if f.all {
		opts.Extensions = browser.ExtensionValues
	}
	for _, r := range f.roots {
		opts.Roots = append(opts.Roots, browser.Root{Kind: browser.KindCustom, Path: r})
	}
	entries, err := browser.Scan(ctx, opts)
	if err != nil {
		return nil, err
	}
	if f.running {
		entries = browser.OnlyRunning(entries)
	}
	return entries, nil
}

// pickOne resolves a match set to exactly one entry.
//
// Tie-break: several matches but exactly one Running → that one (the user
// almost always means the window that is open). Entries without a DeviceID
// are never picked — an empty id is useless to select_browser.
func pickOne(matches []browser.Entry) (browser.Entry, error) {
	var withID []browser.Entry
	for _, m := range matches {
		if m.DeviceID != "" {
			withID = append(withID, m)
		}
	}
	switch len(withID) {
	case 0:
		return browser.Entry{}, errNoMatch
	case 1:
		return withID[0], nil
	}
	if running := browser.OnlyRunning(withID); len(running) == 1 {
		return running[0], nil
	}
	return browser.Entry{}, errAmbiguous
}

// resolveVersion returns the linker-stamped version, else the module version
// recorded by `go install …@vX.Y.Z`, else versionDev. Never errors: a version
// string is informational and must not stop the tool from running.
func resolveVersion() string {
	if version != versionDev {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return versionDev
}

// writeJSON pretty-prints v. Generic rather than `any`-typed so the call
// site's static type is preserved (no boxing through an untyped parameter).
func writeJSON[T []browser.Entry | browser.Entry](w io.Writer, v T) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

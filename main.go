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
	"strings"

	"github.com/spf13/cobra"
	"github.com/ubgo/buildinfo"

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
	// flagProfiles is on `list` only: `find` resolves a device id, and a
	// profile without an extension has none to resolve.
	flagProfiles = "profiles"
	// flagAccountAlias names a Claude account uuid that this machine cannot
	// name by itself (see accountAliases).
	flagAccountAlias = "account-alias"
)

// version is the binary's release version, used by the generated
// skills_gen.go (volt gen skills) to pick the release tag whose skills bundle
// matches this exact binary. buildinfo.DevVersion switches the skills command
// to serving the working tree's skills/ directory instead of fetching.
var version = releaseVersion(buildinfo.Get())

// pseudoVersionPrefix is what Go stamps into Main.Version for a `go build`
// from a git checkout with no tag (e.g. v0.0.0-20260912061725-90d9f421ae81
// +dirty). It is a real-looking version that no release ever carries.
const pseudoVersionPrefix = "v0.0.0-"

// releaseVersion maps build provenance to the version the skills bundle is
// keyed by: a volt ldflags stamp or a `go install …@vX.Y.Z` module version
// is returned as is; anything else — no version at all, or Go's untagged
// pseudo-version — is buildinfo.DevVersion.
//
// Why: without this, a plain `go build` in the repo produced a binary whose
// `skills` command tried to download a bundle for a pseudo-version and 404ed
// (seen 2026-09-12); only `go run`, which embeds no VCS data, said "dev".
func releaseVersion(info buildinfo.Info) string {
	if !info.HasVersion() || strings.HasPrefix(info.Version, pseudoVersionPrefix) {
		return buildinfo.DevVersion
	}
	return info.Version
}

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
	// profiles widens the listing to profiles with no Claude extension, which
	// are otherwise invisible — the "why is my profile missing?" answer.
	profiles bool
	// aliases are `<uuid>=<label>` pairs naming Claude accounts, in the order
	// given; see accountAliases for why they are needed.
	aliases []string
}

// accountAliases parses `--account-alias <uuid>=<label>` into uuid → label.
//
// Why the flag exists: only ONE Claude account can be named from disk — the
// one Claude Code is signed in as (browser.ReadClaudeAccount). Every other
// account exists on this machine as a bare uuid, in the extension store, in
// claude.ai's site data and in past transcripts, with its email stored
// nowhere (verified 2026-09-25). So naming a second account is something only
// the user can supply, once, from their shell profile or a wrapper.
//
// A pair without "=" is an error rather than a silently ignored argument: a
// typo here would otherwise show up as an unexplained uuid in the table.
func accountAliases(pairs []string) (map[string]string, error) {
	out := make(map[string]string, len(pairs))
	for _, p := range pairs {
		uuid, label, ok := strings.Cut(p, aliasSeparator)
		if !ok || uuid == "" || label == "" {
			return nil, fmt.Errorf("--%s %q: want <account-uuid>%s<label>", flagAccountAlias, p, aliasSeparator)
		}
		out[uuid] = label
	}
	return out, nil
}

// aliasSeparator splits an --account-alias pair. labelOtherAccount is what an
// account that cannot be named is called: it is not this session's account,
// which is the only fact that changes what the reader should do.
const (
	aliasSeparator    = "="
	labelOtherAccount = "other account"
)

// newAccountLabeler builds the column's formatter.
//
// Why not just print the uuid: a Claude account uuid tells the reader
// nothing, and the account's EMAIL cannot be recovered for any account but
// the signed-in one — Claude stores no email in the browser at all (checked
// 2026-09-25 against the extension store, claude.ai's own site data,
// Claude Code's config and its backups, and past transcripts). So the column
// answers the question the uuid was standing in for: can THIS session reach
// that browser? Preference order: the signed-in account's email, a label the
// user gave, otherwise "other account" — numbered only when there are
// several, so the common two-account case reads as plain English.
//
// entries is the set about to be rendered; numbering follows their order so
// the same listing always labels the same account the same way.
func newAccountLabeler(session browser.ClaudeAccount, aliases map[string]string, entries []browser.Entry) accountLabeler {
	others := otherAccountNumbers(session.UUID, aliases, entries)
	return func(e browser.Entry) string {
		switch {
		case e.AccountUUID == "":
			return ""
		case session.UUID != "" && e.AccountUUID == session.UUID && session.Email != "":
			return session.Email
		}
		if label, ok := aliases[e.AccountUUID]; ok {
			return label
		}
		if n, ok := others[e.AccountUUID]; ok && n > 0 {
			return fmt.Sprintf("%s %d", labelOtherAccount, n)
		}
		return labelOtherAccount
	}
}

// otherAccountNumbers assigns 1..N to the accounts that are neither the
// signed-in one nor aliased, in the order they appear. A single such account
// maps to 0, meaning "do not number it": "other account" beats "other
// account 1" when there is nothing to tell it apart from.
func otherAccountNumbers(sessionUUID string, aliases map[string]string, entries []browser.Entry) map[string]int {
	order := make([]string, 0, len(entries))
	seen := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.AccountUUID == "" || e.AccountUUID == sessionUUID || seen[e.AccountUUID] {
			continue
		}
		if _, aliased := aliases[e.AccountUUID]; aliased {
			continue
		}
		seen[e.AccountUUID] = true
		order = append(order, e.AccountUUID)
	}
	out := make(map[string]int, len(order))
	for i, u := range order {
		if len(order) == 1 {
			out[u] = 0
			break
		}
		out[u] = i + 1
	}
	return out
}

// sessionAccount reads the account Claude Code is signed in as. Any failure
// degrades to the zero account: the listing still works, accounts just show
// as uuids. That is the whole point of the fallback — a browser inventory
// must not fail because Claude Code is missing or logged out.
func sessionAccount() browser.ClaudeAccount {
	path, err := browser.DefaultClaudeConfigPath()
	if err != nil {
		return browser.ClaudeAccount{}
	}
	acct, err := browser.ReadClaudeAccount(path)
	if err != nil {
		return browser.ClaudeAccount{}
	}
	return acct
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
		Version:       versionString(buildinfo.Get()),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.AddCommand(newListCmd(), newFindCmd(), newSkillsCommand())
	return root
}

// versionString renders `--version`: the raw buildinfo version (so a
// pseudo-version still identifies the exact source build) plus, when the
// build carries a real commit, its short hash and a dirty marker. The marker
// is skipped when Go already encoded it as a "+dirty" suffix.
func versionString(info buildinfo.Info) string {
	if !info.HasCommit() {
		return info.Version
	}
	s := info.Version + " (" + shortCommit(info.Commit) + ")"
	if info.Modified && !strings.Contains(info.Version, dirtySuffix) {
		s += " " + dirtyMarker
	}
	return s
}

// dirtySuffix is Go's own uncommitted-tree marker inside a pseudo-version;
// dirtyMarker is ours for stamped versions that carry no such suffix.
const (
	dirtySuffix = "+dirty"
	dirtyMarker = "dirty"
)

// shortCommitLen is git's conventional abbreviated hash length.
const shortCommitLen = 7

func shortCommit(c string) string {
	if len(c) > shortCommitLen {
		return c[:shortCommitLen]
	}
	return c
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
		Long: `Lists one row per profile that has a Claude extension store on disk, which
is the only place a device id exists. A profile where the extension is not
installed has no id, cannot be selected by the MCP, and is not listed unless
you pass --profiles.`,
		Example: `  browserctrl list
  browserctrl list --running --json
  browserctrl list --profiles
  browserctrl list --account-alias 2f7c1b90-...=work@example.com`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			entries, all, err := scan(cmd.Context(), f)
			if err != nil {
				return err
			}
			if f.json {
				return writeJSON(cmd.OutOrStdout(), entries)
			}
			aliases, err := accountAliases(f.aliases)
			if err != nil {
				return err
			}
			label := newAccountLabeler(sessionAccount(), aliases, all)
			if f.profiles {
				return writeProfilesTable(cmd.OutOrStdout(), entries, label)
			}
			return writeTable(cmd.OutOrStdout(), entries, label)
		},
	}
	addListFlags(cmd, &f)
	cmd.Flags().BoolVar(&f.profiles, flagProfiles, false, "also list profiles that do NOT have the Claude extension installed")
	cmd.Flags().StringArrayVar(&f.aliases, flagAccountAlias, nil, "name a Claude account: <account-uuid>=<label> (repeatable)")
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
			entries, all, err := scan(cmd.Context(), f)
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
					_ = writeTable(cmd.ErrOrStderr(), matches, newAccountLabeler(sessionAccount(), nil, all))
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
//
// It returns the filtered entries AND the unfiltered scan. The second value
// is what account labels are computed from: numbering accounts over the rows
// being printed would let `--running` rename an account between two commands
// on the same machine, which is worse than no label at all.
func scan(ctx context.Context, f listFlags) (entries, all []browser.Entry, err error) {
	opts := browser.Options{Extensions: []browser.ExtensionID{browser.ExtClaudeCode}}
	if f.all {
		opts.Extensions = browser.ExtensionValues
	}
	opts.IncludeAllProfiles = f.profiles
	for _, r := range f.roots {
		opts.Roots = append(opts.Roots, browser.Root{Kind: browser.KindCustom, Path: r})
	}
	all, err = browser.Scan(ctx, opts)
	if err != nil {
		return nil, nil, err
	}
	entries = all
	if f.running {
		entries = browser.OnlyRunning(all)
	}
	return entries, all, nil
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

// writeJSON pretty-prints v. Generic rather than `any`-typed so the call
// site's static type is preserved (no boxing through an untyped parameter).
func writeJSON[T []browser.Entry | browser.Entry](w io.Writer, v T) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

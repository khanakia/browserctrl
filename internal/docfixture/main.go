//go:build darwin || linux

// Command docfixture produces the terminal output shown in the docs.
//
// Why it exists: every command example in README.md and docs/ must be real
// captured output, never typed by hand. This tool builds a deterministic fake
// Chromium root (browsertest.Build), holds the LevelDB locks that a running
// browser holds (an extension store's, and for a profile with no extension
// the profile's own localStorage store), and then runs browserctrl against
// that root so the capture shows genuine running/idle rows. Re-run it after
// any output change and paste the result (`task docs:capture`).
//
// Usage: docfixture <root> <browserctrl-binary> [browserctrl args...]
// The root is created on first use and reused afterwards so ids are stable.
// Exit code = browserctrl's exit code, so error examples can be captured too.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"

	"github.com/khanakia/browserctrl/browser"
	"github.com/khanakia/browserctrl/browser/browsertest"
)

// Exit codes of this tool itself (browserctrl's are passed through).
const (
	exitUsage   = 2
	exitFailure = 1
)

// minArgs: program, root, binary.
const minArgs = 3

// The fixture. Device ids are arbitrary UUIDs kept constant so the docs
// never churn; names/emails are example-domain placeholders.
var fixtureProfiles = []browsertest.ProfileSpec{
	{Dir: "Default", Name: "Aman", Email: "aman@example.com", Stores: map[string]map[string]any{
		string(browser.ExtClaudeCode): {browsertest.KeyBridgeDeviceID: "ce9a8e06-61d3-4e7b-894b-13fd92987212", browsertest.KeyBridgeDisplayName: "chrome-main", browsertest.KeyMcpConnected: true},
	}},
	{Dir: "Profile 5", Name: "Work", Email: "aman@work.example", Stores: map[string]map[string]any{
		string(browser.ExtClaudeCode): {browsertest.KeyBridgeDeviceID: "2aa533d3-d03f-4dd8-b664-f59b8930eccc", browsertest.KeyBridgeDisplayName: "work-chrome", browsertest.KeyMcpConnected: true},
	}},
	{Dir: "Profile 23", Name: "legable", Email: "aman@legable.co", Stores: map[string]map[string]any{
		string(browser.ExtClaudeCode): {browsertest.KeyBridgeDeviceID: "f836694e-b2f0-4e5b-93e4-ff946c6183ad", browsertest.KeyBridgeDisplayName: "aman-legable", browsertest.KeyMcpConnected: true},
	}},
	// Extension installed but never connected: no bridge keys at all.
	{Dir: "Profile 4", Name: "Fresh", Email: "fresh@example.com", Stores: map[string]map[string]any{
		string(browser.ExtClaudeCode): {"selectedModel": "claude-sonnet-5"},
	}},
	// No extension at all: open right now, yet invisible to `list` and to the
	// MCP. Only `list --profiles` shows it — the case the docs must explain.
	{Dir: "Profile 26", Name: "Personal", Email: "personal@example.com"},
}

// runningProfiles are the fixture profiles whose extension-store LOCK this
// tool holds while browserctrl runs, so they report as running.
var runningProfiles = []string{"Default", "Profile 5"}

// runningProfilesWithoutExtension are held by their profile-level
// localStorage LOCK instead — the only running signal a profile with no
// extension store has. Each must be a fixture profile with no Stores.
var runningProfilesWithoutExtension = []string{"Profile 26"}

func main() {
	os.Exit(run(os.Args, os.Stdout, os.Stderr))
}

// run is main without os.Exit so tests can drive it; stdout/stderr are
// io.Writer because exec.Cmd accepts any writer, not just files.
func run(args []string, stdout, stderr io.Writer) int {
	// stderr writes on failure paths: a failed diagnostic has no further
	// channel to report on; the exit code carries the outcome.
	if len(args) < minArgs {
		_, _ = fmt.Fprintln(stderr, "usage: docfixture <root> <browserctrl-binary> [args...]")
		return exitUsage
	}
	root, bin, rest := args[1], args[2], args[3:]
	if _, err := os.Stat(root); err != nil {
		if err := browsertest.Build(root, fixtureProfiles...); err != nil {
			_, _ = fmt.Fprintln(stderr, "build fixture:", err)
			return exitFailure
		}
	}
	release, err := holdLocks(root)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return exitFailure
	}
	defer release()

	cmd := exec.Command(bin, append(rest, "--root", root)...)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err = cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exit):
		return exit.ExitCode()
	default:
		_, _ = fmt.Fprintln(stderr, "run browserctrl:", err)
		return exitFailure
	}
}

// holdLocks takes flock(LOCK_EX) on each running profile's LOCK — the same
// lock Chromium's LevelDB env takes — and returns a func that releases them.
func holdLocks(root string) (release func(), err error) {
	var files []*os.File
	release = func() {
		for _, f := range files {
			_ = f.Close() // closing drops the flock; nothing was written
		}
	}
	paths := map[string]string{}
	for _, dir := range runningProfiles {
		paths[dir] = browsertest.LockPath(root, dir, string(browser.ExtClaudeCode))
	}
	for _, dir := range runningProfilesWithoutExtension {
		paths[dir] = browsertest.ProfileLockPath(root, dir)
	}
	for dir, path := range paths {
		f, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			release()
			return nil, fmt.Errorf("open LOCK for %s: %w", dir, err)
		}
		files = append(files, f)
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			release()
			return nil, fmt.Errorf("flock %s: %w", dir, err)
		}
	}
	return release, nil
}

---
name: browserctrl-core
description: Pick the right Chrome/Chromium browser for the claude-in-chrome MCP without a click-through prompt. Use whenever more than one browser is connected, when list_connected_browsers shows opaque "Browser 1/2/3" names, when the user says "use my work chrome" / "the legable profile" / "which browser is that id", or before calling select_browser.
---

> **Freshness:** run `browserctrl skills check <dir-of-this-file>` — the directory this SKILL.md lives in. If STALE, ignore this file's body and follow `browserctrl skills get browserctrl-core` instead; that output always matches the installed binary.

# browserctrl — pick the browser without asking

`browserctrl` maps every Claude-in-Chrome **device id** to the Chromium browser, profile directory, profile name, signed-in email and user-given display name behind it, and says which profiles are **open right now**. It reads the extension's on-disk storage and Chromium's `Local State`; it never touches the browser, never needs `--remote-debugging-port`, and sends nothing anywhere.

## The rule

Before any `claude-in-chrome` action when more than one browser may be connected:

1. Run `browserctrl list --running --json`.
2. Pick the entry whose `email`, `profileName`, `displayName` or `browser` matches what the user asked for ("my work chrome" → `displayName: work-chrome` or `email: aman@work.example`).
3. Call `select_browser` with that entry's `deviceId`.
4. Fall back to `switch_browser` (the click-in-every-window prompt) **only** if the list is empty or nothing matches unambiguously.

The `deviceId` field is byte-for-byte the id `list_connected_browsers` reports; its `name` there ("Browser 2") is assigned per listing and is not stable, so never match on it.

## Commands

```sh
browserctrl list                    # table: STATE MCP DEVICE ID BROWSER PROFILE NAME EMAIL DISPLAY NAME
browserctrl list --running --json   # only open profiles, as JSON — what an agent should read
browserctrl find <terms...>         # prints exactly one device id; every term must match some field
browserctrl find work --running     # same, restricted to open browsers
browserctrl list --root <dir>       # scan a --user-data-dir profile (Chrome for Testing, Playwright)
browserctrl list --all              # include the Claude desktop-app extension ids
```

Exit codes: `0` ok, `1` failure or no match, `2` ambiguous (candidates printed on stderr, stdout empty). On `2`, add a term — `browserctrl find chrome main` — or fall back to the prompt.

## Reading an entry

| Field | Meaning |
|---|---|
| `deviceId` | pass to `select_browser`; empty means the extension has never connected in that profile |
| `state` | `running` = open now (can be selected); `idle` = closed; `unknown` = could not tell (Windows) — never treat as idle |
| `mcpConnected` | sticky "has completed an MCP handshake before"; combine with `state` for "connected now" |
| `browser` | `chrome`, `edge`, `brave`, `vivaldi`, `opera`, `arc`, `chromium`, … or `custom` for `--root` |
| `profileDir` / `profileName` / `email` | from Chromium's `Local State`; email is the most reliable human key |
| `displayName` | the name the user typed when connecting the extension; ask the user to name each browser once, then match on it |

## Worked example

User: "use my legable chrome and open the dashboard".

```sh
$ browserctrl find legable --running
f836694e-b2f0-4e5b-93e4-ff946c6183ad
```

→ `select_browser(deviceId: "f836694e-b2f0-4e5b-93e4-ff946c6183ad")`, then proceed. If the exit code is `1`, tell the user that profile is not open (it will appear as `idle` in `browserctrl list`) rather than guessing another window.

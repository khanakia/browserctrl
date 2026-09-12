# Contributing to browserctrl

Thanks for your interest in improving **browserctrl**. This guide covers how to get set up, the rules the code follows, and what a good pull request looks like.

By participating, you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md).

## Getting started

```sh
git clone https://github.com/khanakia/browserctrl.git
cd browserctrl
task --list        # every task is self-documenting
task check         # the full gate; must be green before you open a PR
```

Requirements: Go 1.26+, [Task](https://taskfile.dev), and for `task lint` both `staticcheck` and `golangci-lint` on your `PATH` (`go install honnef.co/go/tools/cmd/staticcheck@latest`, `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`).

## The workflow

| Task | What it does |
|---|---|
| `task build` / `task install` | build `bin/browserctrl` / install to `$GOPATH/bin` |
| `task run -- list` / `task find -- legable` | run the CLI from source |
| `task test` | `go test -race ./...` — includes the markdown lint in `docs_test.go` |
| `task cover` | statement coverage for the whole module, the numbers the README quotes |
| `task test:uncovered` | every function below 100%, so you can see what a change left uncovered |
| `task cross` | cross-compile linux/amd64, linux/arm64, windows/amd64, darwin/arm64 |
| `task docs:capture -- <args>` | run the CLI against the doc fixture and print real output to paste into the docs |
| `task check` | fmt check → vet → lint → test → cross → `go mod tidy -diff` |
| `task volt:ci` | `volt ci --all`: volt's gate with its embedded golangci config plus the SKILL.md frontmatter lint |
| `task volt:status` / `task volt:doctor` | release streams and their next version / is the repo releasable (tools, auth, remote) |
| `task volt:release:snapshot` | build every platform into `dist/` with checksums, publish nothing — the release rehearsal |
| `task volt:gen` / `task volt:gen -- skills` | regenerate volt's hash-guarded files (workflows + install scripts / the skills wiring) |

CI is never automated here: `ci.yml` and `release.yml` are `workflow_dispatch` only, by decision, not by budget. Run the gate on a clean runner when you want a second opinion — Actions tab → ci → Run workflow, or `gh workflow run ci.yml --repo khanakia/browserctrl --ref main`. `task check` on your machine **is** the gate; say in the PR that it passed.

## Releasing

Releases are made by [volt](https://github.com/khanakia/voltkit) and are always a deliberate, manual act — never on push, never by tag. `package main` sits at the repo root next to the `browser` library, so there is exactly one stream: a bare `vX.Y.Z` tag that `go install github.com/khanakia/browserctrl@latest` and library importers resolve **and** that carries the release assets — cross-compiled archives + `checksums.txt`, `skills_<version>.tar.gz`, and the Homebrew formula in `khanakia/homebrew-tap` when `HOMEBREW_TAP_GITHUB_TOKEN` is set (skipped loudly otherwise).

Order: bump `CHANGELOG.md` (move `[Unreleased]` under the version), commit, `task check`, `task volt:release:snapshot` to rehearse, then `task volt:release -- vX.Y.Z` (or `-- --bump patch|minor|major`). volt refuses a dirty tree, verifies the stamped version inside the produced binary, and re-reads what it published rather than trusting exit codes; a half-finished release is recovered with `volt release --from-tag vX.Y.Z`, which is also what the manual `release.yml` workflow runs. Never reuse a version the Go checksum database has seen — a burned version is skipped forward, not re-tagged.

Generated files carry a `volt:hash` header and are refused (with a diff) rather than overwritten when hand-edited: `.github/workflows/*.yml`, `install.sh`, `install.ps1`, `skills_gen.go`. `ci.yml` is intentionally hand-edited to `workflow_dispatch`; keep that when regenerating (`volt gen` will report it refused — expected).

## Rules the code follows

These are enforced by review and, where possible, by tests. A PR that breaks one will be asked to change, however small.

- **Read-only, always.** Nothing under a browser's user-data directory is written. Extension stores are snapshotted to a temp dir before being opened; the running probe takes a *shared* non-blocking `flock` and releases it immediately. If a change needs to write, it is the wrong change.
- **Never a wrong "idle".** The probe returns `running`, `idle`, or `unknown`. When it cannot tell (Windows, unreadable `LOCK`) it says `unknown`, and every "running" filter excludes `unknown`. Do not collapse `unknown` into `idle` to make a table look tidier.
- **One bad profile never hides the others.** A store that fails to read is reported on its own row with `Error` set; the scan continues. Only a corrupt `Local State` aborts.
- **No bare strings for closed sets.** Browser kinds, extension ids, run states, storage keys, flag names, exit codes and column headers are named constants in `browser/constants.go` or at the top of the file that owns them, each with a `*Values` slice where iteration matters. `grep` your diff for string literals compared with `==` or used as `case` labels before you push.
- **No lint or type suppressions.** No `//nolint`, no loosened assertions, no skipped tests. Fix the cause. The only tolerated `_ =` is a documented optional-cleanup path (closing a read-only handle in a probe, writing diagnostics to stderr on an already-failing path), with a comment saying why.
- **Every exported symbol has a why-and-invariant doc comment.** Not "gets the profiles" but why the function exists and what a caller must not assume. Multi-shape values are documented at the declaration.
- **Platform claims are proven by the gate, not by prose.** Path tables for Linux and Windows exist in `browser/roots.go`; `task cross` compiles them and `roots_test.go` checks their shape. If you add a platform-specific branch, add it to the cross-compile list in `Taskfile.yml` and say in the README what has actually been run.
- **Doc output is captured, never typed.** Every terminal block in `README.md` and `docs/` comes from `task docs:capture` against `internal/docfixture`. If your change alters output, re-capture and paste; do not hand-edit the block.
- **Markdown is linted by `go test`.** `docs_test.go` fails on hard-wrapped prose (a paragraph is one physical line), `---` horizontal rules, broken relative links, and anchors that do not match GitHub's heading slugs. New pages must be added to its `docFiles` list to be covered.
- **Tests pin behaviour.** New logic ships with a test; a bug fix ships with the test that would have caught it. Table-driven for more than two cases, `t.Parallel()` where independent, fixtures via `browser/browsertest` rather than hand-built LevelDBs.

## Coverage, and the statements that are not covered

`task cover` prints per-package coverage (each package's own tests) and the merged total across every package; the README quotes those numbers verbatim. The bar for a PR is: every **reachable** arm you add or touch has a test, and the test's comment names the arm it pins (`// Pins the mkdtemp error arm: …`). Coverage is measured, not targeted — a pinning test that documents *why* an arm exists is the deliverable, the percentage is a by-product.

Fourteen statements are knowingly uncovered and enumerated in the README's Testing section: two `os.Exit` wrappers, six deferred-Close/cleanup error arms, two tabwriter per-row write arms that `Flush` reports instead, the `flock` default arm, and three defensive arms in the fixture builder. If you find a way to reach one of them from a test without fault injection below `os`, add the test and delete it from the list. If you add a new unreachable arm, add it to the list with its reason — an unexplained gap is treated as a missing test.

Useful while iterating:

```sh
task test:uncovered        # functions below 100%, from the merged profile
task test:cover:html       # open the annotated source in a browser
```

## Branches & commits

- Branch off `main`. Use a short descriptive branch name (`fix/...`, `feat/...`, `docs/...`).
- Write commits in **Conventional Commit** style: `type(scope): description` (`feat`, `fix`, `docs`, `refactor`, `test`, `chore`).
- Keep commits focused; one logical change per commit where practical.

## Pull request checklist

- [ ] `task check` is green locally (fmt, vet, lint, race tests including the doc lint, cross-compile, tidy).
- [ ] New behaviour has tests; a bug fix has the test that reproduces it.
- [ ] Every new exported symbol has a doc comment saying why it exists and what must not be assumed.
- [ ] No new string literals for closed-set values; no suppressions.
- [ ] Any changed CLI output was re-captured with `task docs:capture` and pasted into the docs.
- [ ] `CHANGELOG.md` has an entry under `[Unreleased]` for user-facing changes.
- [ ] No unrelated files or formatting churn.

## Changelog

User-facing changes go under `[Unreleased]` in [CHANGELOG.md](CHANGELOG.md), following [Keep a Changelog](https://keepachangelog.com/).

## Questions

Open a [discussion or issue](https://github.com/khanakia/browserctrl/issues). We're happy to help you land your first contribution.

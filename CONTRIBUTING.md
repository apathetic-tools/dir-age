<!-- CONTRIBUTING.md (markdown) -->

# Contributing to dir-age

Thanks for taking a look. This is a small, single-purpose CLI tool, so the
bar for contributing is low, but so is the appetite for scope creep — see
[Project philosophy](#project-philosophy) before proposing new flags or
config surface.

Please also read the [Code of Conduct](CODE_OF_CONDUCT.md).

## Prerequisites

- [Go](https://go.dev/) 1.26 or later.
- [just](https://github.com/casey/just) (optional, but the commands below
  assume it).
- [Node.js](https://nodejs.org/) 24+ (optional) — only needed for local git
  hook conveniences (commit message linting, plus mirrors of `just fmt`/
  `just check`); none of it is required to open a PR, see
  [Commit messages](#commit-messages) below. [mise](https://mise.jdx.dev/)
  users can just run `mise install` (a `.node-version`/`mise.toml` pin is
  checked in). Node 24's bundled npm (11.16+) also enforces the
  `min-release-age` setting in [.npmrc](../.npmrc), which refuses to install
  a dependency version published less than 7 days ago — expect `npm install`
  to reject a version bump that's too fresh; wait a few days or pin an
  older version.

## Tooling

Three separate tools show up in this repo; they don't overlap, each has one
job:

| Tool   | Job                                              | Contributors run it? |
| ------ | ------------------------------------------------ | --------------------- |
| `just` | Go dev tasks — build/run/test/fmt/vet/check       | Yes, day-to-day.       |
| `mise` | Pins the Node version for the release tooling below | Optional; `.node-version` works with fnm/nvm/asdf too. |
| `npm`  | Hosts the release-automation packages (semantic-release, commitlint, husky) | Rarely directly — mostly runs via git hooks and CI. |

`mise` is runtime-pinning only, not a task runner — it has no tasks defined
in this repo, and dev tasks live in the justfile, not `mise.toml`.

## Getting started

```
git clone https://github.com/apathetic-tools/dir-age.git
cd dir-age
just build   # builds to build/dir-age(.exe)
just run .   # builds, then runs against the given path
just test    # runs the test suite
```

Without `just`, the equivalent plain commands are `go build -o dir-age .`,
`./dir-age .`, and `go test ./...`.

Before opening a PR, make sure both of these are clean:

```
go test ./...
go vet ./...
```

## Project layout

- [main.go](main.go) — CLI entry point and the core `analyze` walk that
  aggregates file birth/modified times into a result.
- [ignore.go](ignore.go) — resolution of the skip list: built-in defaults,
  `.dir-age-ignore` file parsing, and the upward/cascading search logic.
- [pause_windows.go](pause_windows.go) / [pause_other.go](pause_other.go) —
  the "press Enter to exit" behavior on Windows when launched by double-click
  or drag-and-drop (detected via console ownership), a no-op elsewhere.
- `main_test.go` / `ignore_test.go` — tests. All filesystem fixtures use
  `t.TempDir()` (never the repo itself). File times are supplied
  deterministically via the `timesForPath` seam in main.go rather than real
  birth-time behavior, which varies by OS/filesystem and would otherwise make
  tests flaky or platform-dependent.

## Testing conventions

- Never write test fixtures into the repo — use `t.TempDir()`.
- If a test needs specific file times, override `timesForPath` (see
  `withStubTimes` in main_test.go) instead of relying on real filesystem
  birth-time support, which isn't available or consistent everywhere.
- Cover both "found at this precedence level" and "correctly falls through
  when absent" for anything involving the ignore-file resolution order
  (scanned path → its ancestors → beside the executable → built-in
  defaults), plus that nested overrides stay scoped to their own subtree.

## Project philosophy

This tool is meant to be zero-config with sensible defaults. Before adding a
new flag or config file format, consider whether the built-in defaults or
the existing `.dir-age-ignore` mechanism already cover it — a bug fix or a
better default is usually preferable to a new knob.

## Commit messages

Commit messages (specifically each PR's squashed/merged commit on `main`)
must follow [Conventional Commits](https://www.conventionalcommits.org/),
e.g. `fix: handle missing birth time on network shares` or
`feat: add --foo flag`. This is what drives releases — see
[Releases](#releases) below — so the type matters:

- `fix:` — patch release.
- `feat:` — minor release.
- `feat!:`, `fix!:`, or a `BREAKING CHANGE:` footer — major release.
- `chore:`, `docs:`, `test:`, `refactor:`, `ci:` — no release triggered.

CI lints your **PR title** against this on every PR (see
[.github/workflows/commitlint.yml](../.github/workflows/commitlint.yml)) —
that's what actually needs to conform, since it becomes the squash-merge
commit message on `main` that semantic-release reads. This runs
regardless of whether you have Node installed locally, so `npm` is never
required just to open a PR.

If you do have Node installed (see [Prerequisites](#prerequisites)), running
`npm install` once after cloning additionally enables a `commit-msg` git
hook that lints every local commit message as you write it via
[commitlint](https://commitlint.js.org/) — a convenience, not something
your PR depends on.

`npm install` also enables two Go-only git hooks (no Node involved in
running them): `pre-commit` runs `just fmt`, and `pre-push` runs
`just check` (fmt + vet + test). Both fall back to the plain `go`/`gofmt`
equivalents if `just` isn't installed.

## Releases

Releases are fully automated with
[semantic-release](https://semantic-release.gitbook.io/): every push to
`main` inspects the commit messages since the last release, and if any
warrant one, it tags a new version, generates `CHANGELOG.md`, and publishes
a GitHub Release with cross-compiled binaries attached
(see [.releaserc.json](../.releaserc.json) and
[scripts/build-release-assets.sh](../scripts/build-release-assets.sh)).
There's no manual tagging or version bumping — just merge commits with the
right type.

## Submitting changes

1. Open an issue first for anything beyond a small fix, so we can agree on
   the approach before you invest time in it.
2. Keep PRs focused — one change per PR.
3. Make sure `go test ./...` and `go vet ./...` pass.
4. Describe *why* the change is needed, not just what it does.
5. Use a [Conventional Commits](https://www.conventionalcommits.org/) type
   for the PR title/commit message — see [Commit messages](#commit-messages).

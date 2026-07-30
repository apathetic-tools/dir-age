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

## Submitting changes

1. Open an issue first for anything beyond a small fix, so we can agree on
   the approach before you invest time in it.
2. Keep PRs focused — one change per PR.
3. Make sure `go test ./...` and `go vet ./...` pass.
4. Describe *why* the change is needed, not just what it does.

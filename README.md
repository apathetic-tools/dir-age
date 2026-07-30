<!-- README.md (markdown) -->

# dir-age

Estimates when a directory's contents were created and last updated, based on
the files inside it — not the folder's own timestamp (which usually just
reflects whenever it was last copied or checked out).

Drag a folder onto the executable, or run it from the command line.

## Usage

```
dir-age <path> [path...]
```

Example:

```
$ dir-age ./my-project
my-project
  likely created: 2024-03-02 09:15:41
  last updated:   2026-07-30 13:14:08
```

- **likely created** — the earliest file birth time found (falls back to
  modified time on filesystems that don't track birth time).
- **last updated** — the latest file modified time found.

Common noise is skipped automatically: version control directories
(`.git`, `.svn`, `.hg`, `.bzr`), dependency folders (`node_modules`, `vendor`,
`.venv`, ...), and build output (`build`, `dist`, `target`, ...).

To customize what's skipped, add a `.dir-age-ignore` file (one directory name
per line, `#` comments allowed) either in the directory you're scanning (or
any of its parents — the nearest one found wins) or next to the `dir-age`
executable for a personal default. A `.dir-age-ignore` found in a
subdirectory further overrides the rules for that subdirectory alone, the
same way `.gitignore` cascades.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, testing, and
how to submit changes.

## License

[MIT](LICENSE)

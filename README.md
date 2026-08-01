<!-- README.md (markdown) -->

# dir-age

Estimates when a directory's contents were created and last updated, based on
the files inside it — not the folder's own timestamp (which usually just
reflects whenever it was last copied or checked out).

Drag a folder onto the executable, or run it from the command line.

---

***[Issues](https://github.com/apathetic-tools/dir-age/issues)* are only for bugs with the intended functionality of an existing implemented feature, for everything else use *[Discussions](https://github.com/apathetic-tools/dir-age/discussions)*, including feature requests, installation and program support.**

---

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

Common noise is skipped when estimating these, using a set of sane defaults
provided out of the box — see [docs/dir-age-ignore.md](docs/dir-age-ignore.md)
for the defaults and how to customize them with a `.dir-age-ignore` file.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, testing, and
how to submit changes.

## License

[MIT](LICENSE)

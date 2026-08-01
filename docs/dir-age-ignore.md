<!-- dir-age-ignore.md (markdown) -->

# `.dir-age-ignore`

Common noise is skipped when estimating created/last-updated times: version
control directories (`.git`, `.svn`, `.hg`, `.bzr`), dependency folders
(`node_modules`, `vendor`, `.venv`, ...), and build output (`build`, `dist`,
`target`, ...). These sane defaults apply automatically — you only need a
`.dir-age-ignore` file if you want to customize them.

To customize what's skipped, add a `.dir-age-ignore` file either in the
directory you're scanning (or any of its parents — the nearest one found
wins) or next to the `dir-age` executable for a personal default. It uses
the same syntax as a `.gitignore` file:

- one pattern per line; blank lines and `#` comments are ignored.
- `name` or `*.log` matches that name/glob at any depth.
- `/name` or `dir/sub` (a `/` anywhere but the end) anchors the pattern to
  the directory the `.dir-age-ignore` file is in, rather than matching at
  every depth.
- a trailing `/` (e.g. `build/`) matches directories only, never files.
- `**` matches zero or more path segments; `?` and `[...]` work as in shell
  globs.
- a leading `!` re-includes something an earlier pattern excluded.

A `.dir-age-ignore` found in a subdirectory further overrides the rules for
that subdirectory alone, the same way `.gitignore` cascades — though unlike
`.gitignore`, each file fully replaces the inherited rules rather than
adding to them.

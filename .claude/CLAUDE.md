<!-- .claude/CLAUDE.md (markdown) -->

# CLAUDE.md

## Verifying changes

Use `just` targets, not separate `go vet`, `go test`, `gofmt`:

- `just fmt` — check formatting (fails if any file needs `gofmt`)
- `just vet` — run `go vet`
- `just test` — run test suite
- `just check` — run all above; use before calling change done

See [justfile](../justfile) for exact commands each target runs.

<!-- CLAUDE.md (markdown) -->

# CLAUDE.md

## Verifying changes

Use the `just` targets instead of running `go vet`, `go test`, or `gofmt`
separately:

- `just fmt` — check formatting (fails if any file needs `gofmt`)
- `just vet` — run `go vet`
- `just test` — run the test suite
- `just check` — run all of the above; use this before considering a change done

See [justfile](../justfile) for the exact commands each target runs.
# Contributing

Contributions are welcome — new examples, bug fixes, and improvements to existing ones.

## Requirements

- Go 1.26+
- [`goimports`](https://pkg.go.dev/golang.org/x/tools/cmd/goimports)
- [`golangci-lint`](https://golangci-lint.run/welcome/install/) — install via `brew install golangci-lint` or from the official install page (avoid `curl | sh` in production environments)

## Setup

```bash
cp .env.example .env
# Add TRONGRID_API_KEY and TRON_PRIVATE_KEY to .env
```

## Development workflow

```bash
make fmt      # format with goimports
make lint     # run golangci-lint
make build    # compile all examples
make check    # fmt-check + lint + build (runs in CI)
```

## Adding a new example

1. Create `examples/{name}/main.go` as a `package main`
2. Use shared helpers from `utils/` (connection, formatting, signing)
3. Add a `run/{name}` target to the `Makefile`
4. Keep the example focused on a single SDK feature
5. Support a `-dryrun` flag for write operations

## Guidelines

- Use `log.Fatal` for unrecoverable errors in examples
- Load credentials from `.env` via the `utils` package (auto-loaded via `init()`)
- Never hardcode private keys or API keys
- Write operations must support `-dryrun` (sign but do not broadcast)
- Always pass a `feeLimit` when calling `TriggerContract`

## Pull requests

- Run `make check` before opening a PR
- Keep each PR focused on one example or fix
- Include example output in the PR description if adding a new example

## Reporting issues

Open a GitHub issue with:
- Go version (`go version`)
- The command you ran
- The full error output

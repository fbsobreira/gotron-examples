# GoTRON SDK Examples

## Project Overview

Example applications demonstrating [gotron-sdk](https://github.com/fbsobreira/gotron-sdk) usage for TRON blockchain interaction.

## Structure

```
go.mod                  # Module root — uses gotron-sdk@master via replace directive
utils/                  # Shared helper utilities (connection, formatting, etc.)
examples/{name}/        # Each example is a standalone main package
mcp-feedback.md         # Feedback/issues found while using GoTRON MCP server
```

## Dependencies

- **Go 1.26**
- **gotron-sdk**: always use `@master` branch (replace directive in go.mod)
- TRON mainnet gRPC endpoint: `grpc.trongrid.io:50051`

## Conventions

- Each example lives in `examples/{name}/main.go`
- Examples should be self-contained and runnable with `go run ./examples/{name}`
- Use shared utilities from `utils/` package for common operations (gRPC connection, address formatting)
- Keep examples minimal and focused on one SDK feature
- Use `log.Fatal` for unrecoverable errors in examples

## GoTRON MCP Server

This project uses a GoTRON MCP server (`https://mcp.gotron.sh/mcp`) for TRON blockchain queries during development. See `.mcp.json` for the configured endpoint. Log any MCP issues or improvement suggestions in `mcp-feedback.md`.

## Build & Run

```bash
# Run a specific example
go run ./examples/{name}

# Tidy dependencies after adding new imports
go mod tidy
```

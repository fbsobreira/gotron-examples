# gotron-examples

[![CI](https://github.com/fbsobreira/gotron-examples/actions/workflows/ci.yml/badge.svg)](https://github.com/fbsobreira/gotron-examples/actions/workflows/ci.yml)

Example applications demonstrating [gotron-sdk](https://github.com/fbsobreira/gotron-sdk) usage for TRON blockchain interaction.

## Examples

| Example | Description |
|---|---|
| [`address`](examples/address/) | Derive TRON address from a private key |
| [`justlend-energy`](examples/justlend-energy/) | Rent and return energy via JustLend DAO |
| [`sunswap`](examples/sunswap/) | Quote and execute token swaps on SunSwap V2 |

## Requirements

- Go 1.26+
- A TronGrid API key ([trongrid.io](https://www.trongrid.io/))
- A TRON private key (for write operations)

## Setup

```bash
git clone https://github.com/fbsobreira/gotron-examples
cd gotron-examples
cp .env.example .env
# Edit .env and fill in TRONGRID_API_KEY and TRON_PRIVATE_KEY
```

## Running examples

```bash
# Print address from private key
go run ./examples/address

# SunSwap: get price quote (1 TRX → USDT)
go run ./examples/sunswap -action quote

# SunSwap: get pair contract address
go run ./examples/sunswap -action pair

# SunSwap: simulate swap (no signing)
go run ./examples/sunswap -action simulate-swap -from <address>

# SunSwap: execute swap (dryrun — sign but do not broadcast)
go run ./examples/sunswap -action swap -amount 1000000 -dryrun

# SunSwap: execute swap (live)
go run ./examples/sunswap -action swap -amount 1000000

# JustLend: show rental rates
go run ./examples/justlend-energy -action rates -receiver <address> -amount 100

# JustLend: show active rental info
go run ./examples/justlend-energy -action info -receiver <address> -amount 1

# JustLend: rent energy (dryrun)
go run ./examples/justlend-energy -action rent -receiver <address> -amount 100 -dryrun

# JustLend: rent energy (live)
go run ./examples/justlend-energy -action rent -receiver <address> -amount 100

# JustLend: return energy (live)
go run ./examples/justlend-energy -action return -receiver <address> -amount 100
```

Or use Makefile shortcuts:

```bash
make run/address
make run/sunswap ARGS="-action quote"
make run/justlend-energy ARGS="-action rates -receiver <address> -amount 100"
```

## Development

```bash
make fmt      # format with goimports
make lint     # run golangci-lint
make build    # compile all examples
make check    # fmt-check + lint + build
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution guidelines.

## License

[MIT](LICENSE)

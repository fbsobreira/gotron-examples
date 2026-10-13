# gotron-examples

[![CI](https://github.com/fbsobreira/gotron-examples/actions/workflows/ci.yml/badge.svg)](https://github.com/fbsobreira/gotron-examples/actions/workflows/ci.yml)

Example applications demonstrating [gotron-sdk](https://github.com/fbsobreira/gotron-sdk) usage for TRON blockchain interaction.

## Examples

| Example | Description |
|---|---|
| [`address`](examples/address/) | Derive TRON address from a private key |
| [`justlend-energy`](examples/justlend-energy/) | Rent and return energy via JustLend DAO |
| [`sunswap`](examples/sunswap/) | Quote and execute token swaps on SunSwap V2 |
| [`pq-capabilities`](examples/pq-capabilities/) | Show which post-quantum signature schemes a node accepts (experimental) |
| [`pq-keys`](examples/pq-keys/) | Generate a post-quantum key and save it as an encrypted keystore (experimental) |
| [`pq-transfer`](examples/pq-transfer/) | Send TRX on Nile signed with an FN-DSA-512 key (experimental) |
| [`pq-migrate`](examples/pq-migrate/) | Authorize a post-quantum key on an existing ECDSA account (experimental) |

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

## Post-quantum signatures (TIP-899)

> **Experimental.** TIP-899 is a draft and is live only on the **Nile testnet**. Mainnet nodes do not
> accept post-quantum signatures. The wire format, sizes and activation can change before the TIP is final.
> The PQ examples default to Nile (`grpc.nile.trongrid.io:50051`).

TIP-899 adds a `pq_auth_sig` field to transactions, next to the ECDSA `signature`. It carries the PQ public
key and the signature. A PQ address is `0x41 || Keccak-256(public_key)[12:]`, the same recipe as ECDSA, so it
looks like any other `T...` address. See the SDK's
[post-quantum guide](https://github.com/fbsobreira/gotron-sdk/blob/master/docs/post-quantum.md) for how it works.

| Scheme | Public key | Signature | On Nile | Build |
|---|---|---|---|---|
| FN-DSA-512 (Falcon) | 896 B | 617–667 B | **enabled** | needs `-tags falcon`, CGO and liboqs |
| ML-DSA-44 | 1,312 B | 2,420 B | disabled | pure Go |

FN-DSA-512 signing and verifying need [liboqs](https://openquantumsafe.org/liboqs/):

```bash
brew install liboqs pkgconf
CGO_ENABLED=1 go run -tags falcon ./examples/<name>
```

Without the tag, the examples stop with a message explaining this.

```bash
# Which schemes do Nile and mainnet accept? (pure Go, no keys)
go run ./examples/pq-capabilities

# Generate a key and write an encrypted, wallet-cli compatible keystore.
# The passphrase is read from PQ_KEYSTORE_PASSWORD or prompted for.
go run ./examples/pq-keys                                   # ML-DSA-44 (disabled on Nile)
CGO_ENABLED=1 go run -tags falcon ./examples/pq-keys -scheme fn-dsa-512 -out pq-keystore-fn.json

# The keystore uses the wallet-cli PQ format, so tronctl can import and sign with it.
# Signing with FN-DSA-512 needs the tronctl-pq build (see below); the default tronctl can only import it.
tronctl keys import-ks pq-keystore-fn.json <name>
tronctl -s <name> ...

# Fund the printed address from the Nile faucet (https://nileex.io/join/getJoinPage), then send TRX.
# Each PQ transfer burns about 1.7 TRX of bandwidth unless the account has staked bandwidth.
CGO_ENABLED=1 go run -tags falcon ./examples/pq-transfer -keystore pq-keystore-fn.json -to <address> -amount 1000000 -dryrun
CGO_ENABLED=1 go run -tags falcon ./examples/pq-transfer -keystore pq-keystore-fn.json -to <address> -amount 1000000

# Authorize the PQ key on an existing ECDSA account (TRON_PRIVATE_KEY signs the update).
# The keystore proves you hold the PQ key and tells its scheme; a scheme the node rejects is refused.
# Dry run by default; -confirm signs and broadcasts. Nile charges a 100 TRX permission-update fee.
go run ./examples/pq-migrate -keystore pq-keystore-fn.json                       # either key can sign
go run ./examples/pq-migrate -keystore pq-keystore-fn.json -confirm

# After a migration, sign for the old account with the PQ key.
CGO_ENABLED=1 go run -tags falcon ./examples/pq-transfer -keystore pq-keystore-fn.json -from <account> -to <address>

# Then tighten: both keys required, or PQ only.
go run ./examples/pq-migrate -keystore pq-keystore-fn.json -mode pq-only -confirm
```

Keystores hold secret keys: keep them out of version control (`pq-keystore*.json` is in `.gitignore`).
Never pass a private key or passphrase as a command-line argument.

To use these keystores from the command line, since v0.27.0, download the
`tronctl-pq_<version>_<os>_<arch>` archive. It has liboqs linked in, so nothing else needs installing. The
default `tronctl` (release archives, Homebrew, `go install`) can import FN-DSA-512 keys but cannot create or sign
with them. See [tronctl builds with FN-DSA-512](https://github.com/fbsobreira/gotron-sdk/blob/master/docs/post-quantum.md#tronctl-builds-with-fn-dsa-512).
Go programs, these examples included, still need `-tags falcon`, CGO and liboqs for FN-DSA-512.

### Migrating an account

There is no "claim" step: a PQ key has its own address, so it gets authority over an existing account only
when that account's current key rewrites the account's permissions. `pq-migrate` sets the owner permission
and one active permission to the same keys:

| `-mode` | Keys, threshold | ECDSA alone can sign? | Use |
|---|---|---|---|
| `either` (default) | ECDSA 1 + PQ 1, threshold 1 | **yes** | Staging: test PQ signing with no lockout risk. **No security gain yet** |
| `both` | ECDSA 1 + PQ 1, threshold 2 | no | Hybrid: an attacker must break both schemes |
| `pq-only` | PQ 1, threshold 1 | no | Strongest. The ECDSA key loses all authority |

With `both` or `pq-only`, losing the PQ key, or governance switching its scheme off, **locks the account**.
So `pq-migrate` accepts those modes only once the PQ key is already in the owner permission: run `either`
first, send a PQ-signed transaction with `pq-transfer -from`, then tighten. After `both`, every transaction
also needs an ECDSA co-signature, which `pq-transfer` does not add (use a multi-sign tool such as wallet-cli).
The example refuses witness accounts (their witness permission takes exactly one key) and accounts with more
than one active permission. Any other key on the active permission is removed, and the dry run says so.
The active permission keeps its name and operations. An account with no stored permissions (created before
multi-sign, at genesis or by a contract) has no operations to copy, and no API gives the network's defaults, so
pass them with `-operations <hex bitmap>`, copied from an account created recently on the same network.

The alternative is to create a new PQ account and move the assets to it. That gives a **different address**, and
every exchange, DApp and contract allowance has to learn it.

## Development

```bash
make fmt      # format with goimports
make lint     # run golangci-lint
make build    # compile all examples
make build-falcon  # compile with FN-DSA-512 support (needs liboqs)
make check    # fmt-check + lint + build
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution guidelines.

## License

[MIT](LICENSE)

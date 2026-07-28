# Chain Utils

Standalone Go scripts for estimating Polygon PoS block timings and hardfork
activation heights on Bor and Heimdall.

The hardfork calculators are intended to be used in pairs:

1. Run the matching average block time calculator against the target network.
2. Choose the average window you want to schedule from.
3. Pass that average block time and the target UTC timestamp into the matching
   hardfork block calculator.

For the inverse calculation—estimating when a known Heimdall height will be
reached—pass that same measured average to the Heimdall block-time estimator.

Run each script directly with `go run <script>.go`. The scripts are independent
`package main` files, so do not run `go run .`.

## Scripts

| Script | Purpose |
| --- | --- |
| `bor_average_blocktime_calculator.go` | Calculates Bor average block time over configurable lookback windows. |
| `bor_hf_block_calculator.go` | Predicts the Bor block height for a target UTC timestamp. |
| `bor_block_time_estimator.go` | Predicts when Bor will reach a target block height. |
| `heimdall_average_blocktime_calculator.go` | Calculates Heimdall average block time over configurable lookback windows. |
| `heimdall_hf_block_calculator.go` | Predicts the Heimdall block height for a target UTC timestamp. |
| `heimdall_block_time_estimator.go` | Predicts when Heimdall will reach a target block height. |

Every script supports:

```bash
go run <script>.go help
go run <script>.go -help
```

## Bor Average Block Time

For mainnet Bor, set `BOR_MAINNET_RPC` to an authenticated JSON-RPC endpoint
before running the examples below.

```bash
go run bor_average_blocktime_calculator.go \
  -rpc=$BOR_MAINNET_RPC \
  -lookbacks=40000,280000,560000,1120000
```

Common options:

| Option | Description | Default |
| --- | --- | --- |
| `-rpc` | Required. Bor JSON-RPC endpoint for the network being scheduled. | none |
| `-lookbacks` | Comma-separated block distances from the latest block. | `40000,280000,560000,1120000` |
| `-timeout` | HTTP request timeout. | `20s` |

Amoy example:

```bash
go run bor_average_blocktime_calculator.go \
  -rpc=https://rpc-amoy.polygon.technology \
  -lookbacks=10000,50000,100000
```

## Bor Hardfork Block

```bash
go run bor_hf_block_calculator.go \
  -rpc=$BOR_MAINNET_RPC \
  -target=2026-06-01T14:00:00Z \
  -avg=2.156
```

Required options:

| Option | Description |
| --- | --- |
| `-rpc` | Bor JSON-RPC endpoint for the network being scheduled. |
| `-target` | Target UTC timestamp in RFC3339 or RFC3339Nano format. |
| `-avg` | Average Bor block time in seconds. Use a value from `bor_average_blocktime_calculator.go`. |

Common options:

| Option | Description | Default |
| --- | --- | --- |
| `-timeout` | HTTP request timeout. | `20s` |

Amoy example:

```bash
go run bor_hf_block_calculator.go \
  -rpc=https://rpc-amoy.polygon.technology \
  -target=2026-06-01T14:00:00Z \
  -avg=2.1
```

## Bor Block-Time Estimator

Use this for the inverse of the hardfork calculator: provide a known target
height and the average block time measured by the Bor average calculator.

```bash
go run bor_block_time_estimator.go \
  -rpc=$BOR_MAINNET_RPC \
  -target=80000000 \
  -avg=2.156
```

Required options:

| Option | Description |
| --- | --- |
| `-rpc` | Bor JSON-RPC endpoint for the network being scheduled. |
| `-target` | Target Bor block height. Commas are accepted for readability. |
| `-avg` | Average Bor block time in seconds. Use a value from `bor_average_blocktime_calculator.go`. |

Common options:

| Option | Description | Default |
| --- | --- | --- |
| `-timeout` | HTTP request timeout. | `20s` |

## Heimdall Average Block Time

```bash
go run heimdall_average_blocktime_calculator.go \
  -rpc=https://tendermint-api.polygon.technology \
  -lookbacks=10000,100000,1000000,1500000
```

Common options:

| Option | Description | Default |
| --- | --- | --- |
| `-rpc` | Required. Heimdall Tendermint RPC base URL for the network being scheduled. | none |
| `-base` | Alias for `-rpc`, kept for older invocations. | empty |
| `-lookbacks` | Comma-separated block distances from the latest block. | `10000,100000,1000000,1500000` |
| `-timeout` | HTTP request timeout. | `15s` |

Amoy example:

```bash
go run heimdall_average_blocktime_calculator.go \
  -rpc=https://tendermint-api-amoy.polygon.technology \
  -lookbacks=10000,50000,100000
```

## Heimdall Hardfork Block

```bash
go run heimdall_hf_block_calculator.go \
  -rpc=https://tendermint-api.polygon.technology \
  -target=2026-06-01T14:00:00Z \
  -avg=1.30
```

Required options:

| Option | Description |
| --- | --- |
| `-rpc` | Heimdall Tendermint RPC base URL for the network being scheduled. |
| `-target` | Target UTC timestamp in RFC3339 or RFC3339Nano format. |
| `-avg` | Average Heimdall block time in seconds. Use a value from `heimdall_average_blocktime_calculator.go`. |

Common options:

| Option | Description | Default |
| --- | --- | --- |
| `-base` | Alias for `-rpc`, kept for older invocations. | empty |
| `-timeout` | HTTP request timeout. | `15s` |

Amoy example:

```bash
go run heimdall_hf_block_calculator.go \
  -rpc=https://tendermint-api-amoy.polygon.technology \
  -target=2026-06-01T14:00:00Z \
  -avg=1.25
```

## Heimdall Block-Time Estimator

Use this for the inverse of the hardfork calculator: provide a known target
height and the average block time measured by the Heimdall average calculator.

```bash
go run heimdall_block_time_estimator.go \
  -rpc=https://tendermint-api.polygon.technology \
  -target=50185000 \
  -avg=1.30
```

Required options:

| Option | Description |
| --- | --- |
| `-rpc` | Heimdall Tendermint RPC base URL for the network being scheduled. |
| `-target` | Target Heimdall block height. Commas are accepted for readability. |
| `-avg` | Average Heimdall block time in seconds. Use a value from `heimdall_average_blocktime_calculator.go`. |

Common options:

| Option | Description | Default |
| --- | --- | --- |
| `-base` | Alias for `-rpc`, kept for older invocations. | empty |
| `-timeout` | HTTP request timeout. | `15s` |

## Scheduling Notes

- Always use the endpoint for the same network you are scheduling.
- Use UTC timestamps with an explicit `Z` suffix, for example
  `2026-06-01T14:00:00Z`.
- Re-run the average calculation close to the final proposal time. Block times
  drift with network conditions.
- Compare multiple lookback windows before choosing the `-avg` value. Short
  windows react faster; long windows smooth out transient variance.
- Treat the predicted block as an estimate until it is cross-checked against
  current network state and release-specific hardfork requirements.

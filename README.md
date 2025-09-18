# 🛠️ Chain Utils

A personal collection of blockchain infrastructure scripts and utilities — built for developers working on validator nodes, chain monitoring, or custom on-chain tooling.

---

## 🔍 What's Inside

| Script | Description |
|--------|-------------|
| `bor_average_blocktime_calculator.go` | Calculates the average block time over the last 40k, 280k, 560k, and 1.12M blocks on the Bor chain. Useful for chain health monitoring and block production analysis. |
| `bor_hf_block_calculator.go` | Predicts the block height corresponding to a future target UTC time given an assumed average block time for the Bor chain (e.g. planning for hardforks or upgrades). |
| `heimdall_average_blocktime_calculator.go` | Calculates the average block time over the last 10k, 100k, 1M, and 1.5M blocks. Useful for chain health monitoring and block production analysis. |
| `heimdall_hf_block_calculator.go`        | Predicts the block height corresponding to a future target UTC time given an assumed average block time (e.g. planning for hardforks or upgrades). |

---

## 🚀 Quick Start

### Example 1: Calculate Bor Average Block Times

```bash
go run bor_average_blocktime_calculator.go
```

This script
- Fetches the latest block height and timestamp from Bor RPC
- For each lookback (40k, 280k, 560k, 1.12M blocks), fetches a past block
- Prints elapsed time (days/hours/minutes/seconds) and average block time in seconds


### Example 2: Predict Bor Block Height at a Future Time

```bash
go run bor_hf_block_calculator.go
```

This script
- Fetches the latest block height and timestamp from Bor RPC
- Uses a configurable target UTC timestamp and average block time
- Calculates how many blocks fit in the delta between now and target
- Prints the predicted block height and time delta


### Example 3: Calculate Heimdall Average Block Times

```bash
go run heimdall_average_blocktime_calculator.go
```

This script
- Fetches the latest block height and timestamp from Heimdall APIs
- For each lookback (10k, 100k, 1M, 1.5M blocks), fetches a past block
- Prints elapsed time (days/hours/minutes/seconds) and average block time in seconds


### Example 4: Predict Heimdall Block Height at a Future Time

```bash
go run heimdall_hf_block_calculator.go
```

This script
- Fetches the latest block height and timestamp from Heimdall APIs
- Uses a hardcoded target UTC timestamp and an average block time (in seconds)
- Calculates how many blocks fit in the delta between now and target
- Prints the predicted block height and time delta

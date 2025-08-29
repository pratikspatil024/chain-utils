# 🛠️ Chain Utils

A personal collection of blockchain infrastructure scripts and utilities — built for developers working on validator nodes, chain monitoring, or custom on-chain tooling.

---

## 🔍 What's Inside

| Script | Description |
|--------|-------------|
| `heimdall_block_time_estimator.go` | Estimates the future mining time of a specific block on the Heimdall chain using public APIs. |
| `heimdall_average_blocktime_calculator.go` | Calculates the average block time over the last 10k, 100k, 1M, and 1.5M blocks. Useful for chain health monitoring and block production analysis. |
| `heimdall_hf_block_calculator.go`        | Predicts the block height corresponding to a future target UTC time given an assumed average block time (e.g. planning for hardforks or upgrades). |
| (More coming idk when...) | Planning to add Bor tools, validator metrics fetchers, and health checkers. |

---

## 🚀 Quick Start

### Example 1: Estimate Heimdall Block ETA

```bash
go run heimdall_block_time_estimator.go
```

This script
- Fetches the latest block height and timestamp from Heimdall APIs
- Calculates average block time over the past 2000 blocks
- Predicts when a future block (e.g. 8788500) will be mined


### Example 2: Calculate Average Block Times

```bash
go run heimdall_average_blocktime_calculator.go
```

This script
- Fetches the latest block height and timestamp
- For each lookback (10k, 100k, 1M, 1.5M blocks), fetches a past block
- Prints elapsed time (days/hours/minutes/seconds) and average block time in seconds


### Example 3: Predict Block Height at a Future Time

```bash
go run heimdall_hf_block_calculator.go
```

This script
- Fetches the latest block height and timestamp
- Uses a hardcoded target UTC timestamp and an average block time (in seconds)
- Calculates how many blocks fit in the delta between now and target
- Prints the predicted block height and time delta

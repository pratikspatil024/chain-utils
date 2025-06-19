# 🛠️ Chain Utils

A personal collection of blockchain infrastructure scripts and utilities — built for developers working on validator nodes, chain monitoring, or custom on-chain tooling.

---

## 🔍 What's Inside

| Script | Description |
|--------|-------------|
| `heimdall_block_time_estimator.go` | Estimates the future mining time of a specific block on the Heimdall chain using public APIs. |
| (More coming idk when...) | Planning to add Bor tools, validator metrics fetchers, and health checkers. |

---

## 🚀 Quick Start

### Example: Estimate Heimdall Block ETA

```bash
go run heimdall_block_time_estimator.go
```

This script
- Fetches the latest block height and timestamp from Heimdall APIs
- Calculates average block time over the past 2000 blocks
- Predicts when a future block (e.g. 8788500) will be mined

